/*
 * Copyright (c) 2019-2021. Abstrium SAS <team (at) pydio.com>
 * This file is part of Pydio Cells.
 *
 * Pydio Cells is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * Pydio Cells is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with Pydio Cells.  If not, see <http://www.gnu.org/licenses/>.
 *
 * The latest code can be found at <https://pydio.com>.
 */

package core

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/pydio/cells/v4/common"
	grpc2 "github.com/pydio/cells/v4/common/client/grpc"
	"github.com/pydio/cells/v4/common/log"
	"github.com/pydio/cells/v4/common/nodes"
	"github.com/pydio/cells/v4/common/nodes/abstract"
	"github.com/pydio/cells/v4/common/nodes/models"
	"github.com/pydio/cells/v4/common/proto/encryption"
	"github.com/pydio/cells/v4/common/proto/object"
	"github.com/pydio/cells/v4/common/proto/tree"
	"github.com/pydio/cells/v4/common/service/context/metadata"
	"github.com/pydio/cells/v4/common/service/errors"
	"github.com/pydio/cells/v4/common/utils/uuid"
)

func WithFlatInterceptor() nodes.Option {
	return func(options *nodes.RouterOptions) {
		options.Wrappers = append(options.Wrappers, &FlatStorageHandler{})
	}
}

var keyClient encryption.NodeKeyManagerClient

// FlatStorageHandler intercepts request to a flat-storage
type FlatStorageHandler struct {
	abstract.Handler
}

func (f *FlatStorageHandler) Adapt(h nodes.Handler, options nodes.RouterOptions) nodes.Handler {
	f.Next = h
	f.ClientsPool = options.Pool
	return f
}

// CreateNode creates directly in index, but make sure not to override
func (f *FlatStorageHandler) CreateNode(ctx context.Context, in *tree.CreateNodeRequest, opts ...grpc.CallOption) (*tree.CreateNodeResponse, error) {
	if !nodes.IsFlatStorage(ctx, "in") || in.GetNode().IsLeaf() {
		return f.Next.CreateNode(ctx, in, opts...)
	}
	if r, e := f.Next.ReadNode(ctx, &tree.ReadNodeRequest{Node: &tree.Node{Path: in.GetNode().GetPath()}}); e == nil && r.GetNode() != nil {
		rNode := r.GetNode()
		if rNode.IsLeaf() {
			return nil, errors.Forbidden("cannot.override.file", "trying to create a folder on top of an existing file")
		}
		rNode.MustSetMeta(common.MetaFlagIndexed, true)
		return &tree.CreateNodeResponse{Node: rNode}, nil
	}
	cResp, cErr := f.ClientsPool.GetTreeClientWrite().CreateNode(ctx, in, opts...)
	if cErr == nil && cResp.GetNode() != nil {
		cResp.GetNode().MustSetMeta(common.MetaFlagIndexed, true)
	}
	return cResp, cErr
}

func (f *FlatStorageHandler) DeleteNode(ctx context.Context, in *tree.DeleteNodeRequest, opts ...grpc.CallOption) (*tree.DeleteNodeResponse, error) {
	isFlat := nodes.IsFlatStorage(ctx, "in")
	if isFlat && !in.GetNode().IsLeaf() {
		return f.ClientsPool.GetTreeClientWrite().DeleteNode(ctx, in)
	}
	resp, e := f.Next.DeleteNode(ctx, in, opts...)
	if isFlat && e == nil && resp.Success {
		// Update index directly
		return f.ClientsPool.GetTreeClientWrite().DeleteNode(ctx, in)
	}
	return resp, e
}

func (f *FlatStorageHandler) GetObject(ctx context.Context, node *tree.Node, requestData *models.GetRequestData) (io.ReadCloser, error) {
	if nodes.IsFlatStorage(ctx, "in") {
		if e := f.resolveUUID(ctx, node); e != nil {
			return nil, e
		}
	}
	return f.Next.GetObject(ctx, node, requestData)
}

func (f *FlatStorageHandler) CopyObject(ctx context.Context, from *tree.Node, to *tree.Node, requestData *models.CopyRequestData) (int64, error) {

	var revertNode *tree.Node
	if nodes.IsFlatStorage(ctx, "to") {
		if len(requestData.SrcVersionId) > 0 {
			if e := f.resolveUUID(ctx, to); e != nil {
				return 0, e
			}
		} else {
			// Insert in tree as temporary
			temporary := to.Clone()
			if dir, o := requestData.Metadata[common.XAmzMetaDirective]; o && dir == "COPY" { // MOVE CASE = Copy original metadata
				temporary.Uuid = from.Uuid
			} else if temporary.Uuid == "" || temporary.Uuid == from.Uuid {
				temporary.Uuid = uuid.New()
				// Copy - Uuid maybe specified by a previous handler
				if metaUuid, ok := requestData.Metadata[common.XAmzMetaNodeUuid]; ok && metaUuid != "" {
					temporary.Uuid = metaUuid
				}
			}
			temporary.Type = tree.NodeType_LEAF
			temporary.Etag = common.NodeFlagEtagTemporary
			if ctype := from.GetStringMeta(common.MetaNamespaceMime); ctype != "" {
				temporary.MustSetMeta(common.MetaNamespaceMime, ctype)
			}
			if _, er := f.ClientsPool.GetTreeClientWrite().CreateNode(ctx, &tree.CreateNodeRequest{Node: temporary}); er != nil {
				return 0, er
			}
			// Attach Uuid to target node
			to.Uuid = temporary.Uuid
			revertNode = temporary
		}
	}
	i, e := f.Next.CopyObject(ctx, from, to, requestData)
	if e == nil && nodes.IsFlatStorage(ctx, "to") {
		// Create an "in" context with resolver
		destInfo, _ := nodes.GetBranchInfo(ctx, "to")
		tgtCtx := nodes.WithBranchInfo(ctx, "in", destInfo)
		if requestData.Metadata != nil {
			if _, ok := requestData.Metadata[common.XPydioMoveUuid]; ok {
				tgtCtx = metadata.WithAdditionalMetadata(tgtCtx, requestData.Metadata)
			}
		}
		// Now store in index
		if er := f.postCreate(tgtCtx, "to", to, requestData.Metadata, from.GetStringMeta(common.MetaNamespaceMime)); er != nil {
			return i, er
		}
	} else if e != nil && revertNode != nil {
		if _, de := f.ClientsPool.GetTreeClientWrite().DeleteNode(ctx, &tree.DeleteNodeRequest{Node: revertNode}); de != nil {
			log.Logger(ctx).Error("Error while copying object and error while reverting index node", zap.Error(de), revertNode.ZapPath())
		} else {
			log.Logger(ctx).Warn("Error while copying object, reverted index node", revertNode.ZapPath())
		}
	}
	return i, e
}

func (f *FlatStorageHandler) PutObject(ctx context.Context, node *tree.Node, reader io.Reader, requestData *models.PutRequestData) (int64, error) {
	i, e := f.Next.PutObject(ctx, node, reader, requestData)
	if e == nil && nodes.IsFlatStorage(ctx, "in") {
		if er := f.postCreate(ctx, "in", node, requestData.Metadata, requestData.MetaContentType()); er != nil {
			return i, er
		}
	}
	return i, e
}

func (f *FlatStorageHandler) MultipartPutObjectPart(ctx context.Context, target *tree.Node, uploadID string, partNumberMarker int, reader io.Reader, requestData *models.PutRequestData) (models.MultipartObjectPart, error) {
	if nodes.IsFlatStorage(ctx, "in") {
		if e := f.resolveUUID(ctx, target); e != nil {
			return models.MultipartObjectPart{}, e
		}
	}
	return f.Next.MultipartPutObjectPart(ctx, target, uploadID, partNumberMarker, reader, requestData)
}

func (f *FlatStorageHandler) MultipartComplete(ctx context.Context, target *tree.Node, uploadID string, uploadedParts []models.MultipartObjectPart) (models.ObjectInfo, error) {
	if nodes.IsFlatStorage(ctx, "in") {
		if e := f.resolveUUID(ctx, target); e != nil {
			return models.ObjectInfo{}, e
		}
	}
	info, e := f.Next.MultipartComplete(ctx, target, uploadID, uploadedParts)
	if e == nil && nodes.IsFlatStorage(ctx, "in") {
		// We assume MultipartCreate has already set the clear-size in the index, otherwise we have to find the correct value
		meta := map[string]string{}
		if target.Size == 0 {
			if branchInfo, ok := nodes.GetBranchInfo(ctx, "in"); ok && branchInfo.EncryptionMode != object.EncryptionMode_CLEAR {
				if pS, e := f.encPlainSizeRecompute(ctx, target.GetUuid(), branchInfo.Name); e == nil {
					// We have to reload the size from encryption service
					//fmt.Println("Reading plain size from nodeKey service", pS)
					meta[common.XAmzMetaClearSize] = fmt.Sprintf("%d", pS)
				}
			} else {
				//fmt.Println("Setting size from uploaded S3 object", info.Size)
				meta[common.XAmzMetaClearSize] = fmt.Sprintf("%d", info.Size)
			}
		}
		if er := f.postCreate(ctx, "in", target, meta, ""); er != nil {
			return info, er
		}
	}
	return info, e
}

func (f *FlatStorageHandler) MultipartListObjectParts(ctx context.Context, target *tree.Node, uploadID string, partNumberMarker int, maxParts int) (lpi models.ListObjectPartsResult, err error) {
	if nodes.IsFlatStorage(ctx, "in") {
		if e := f.resolveUUID(ctx, target); e != nil {
			return models.ListObjectPartsResult{}, e
		}
	}
	return f.Next.MultipartListObjectParts(ctx, target, uploadID, partNumberMarker, maxParts)
}

func (f *FlatStorageHandler) MultipartAbort(ctx context.Context, target *tree.Node, uploadID string, requestData *models.MultipartRequestData) error {
	if nodes.IsFlatStorage(ctx, "in") {
		if e := f.resolveUUID(ctx, target); e != nil {
			return e
		}
	}
	return f.Next.MultipartAbort(ctx, target, uploadID, requestData)
}

func (f *FlatStorageHandler) resolveUUID(ctx context.Context, node *tree.Node) error {
	if node.GetUuid() != "" {
		return nil
	}
	if r, e := f.ReadNode(ctx, &tree.ReadNodeRequest{Node: node}); e != nil {
		return e
	} else {
		node.Uuid = r.GetNode().GetUuid()
	}
	return nil
}

// postCreate updates index after upload by re-read newly added S3 object to get ETag
func (f *FlatStorageHandler) postCreate(ctx context.Context, identifier string, node *tree.Node, requestMeta map[string]string, cType string) error {
	var updateNode *tree.Node
	if updateResp, err := f.ReadNode(ctx, &tree.ReadNodeRequest{Node: node}); err == nil {
		updateNode = updateResp.GetNode()
	} else if node.Uuid != "" {
		updateNode = node
	} else {
		return fmt.Errorf("missing uuid info to postCreate node")
	}
	stats, err := f.ReadNode(ctx, &tree.ReadNodeRequest{Node: node, ObjectStats: true})
	if err != nil {
		return err
	}
	updateNode.MTime = stats.GetNode().GetMTime()
	updateNode.Size = stats.GetNode().GetSize()
	if requestMeta != nil {
		if pS, o := requestMeta[common.XAmzMetaClearSize]; o {
			if metaSize, e := strconv.ParseInt(pS, 10, 64); e == nil {
				updateNode.Size = metaSize
			}
		}
	}
	updateNode.Etag = stats.GetNode().GetEtag()
	if updateNode.Etag == "" || strings.Contains(updateNode.Etag, "-") {
		newETag, e := f.recomputeETag(ctx, identifier, updateNode)
		if e == nil {
			log.Logger(ctx).Info("Recomputed ETag :" + updateNode.Etag + " => " + newETag)
			updateNode.Etag = newETag
		} else {
			log.Logger(ctx).Error("Cannot recompute ETag :"+updateNode.Etag, zap.Error(e))
		}
	}
	if cType != "" {
		updateNode.MustSetMeta(common.MetaNamespaceMime, cType)
	}
	_, er := f.ClientsPool.GetTreeClientWrite().CreateNode(ctx, &tree.CreateNodeRequest{Node: updateNode, UpdateIfExists: true})
	if er != nil {
		return er
	}
	return nil
}

func (f *FlatStorageHandler) recomputeETag(ctx context.Context, identifier string, node *tree.Node) (string, error) {

	src, _ := nodes.GetBranchInfo(ctx, identifier)

	// Init contextual metadata structures
	copyMeta := map[string]string{
		common.XAmzMetaDirective: "REPLACE",
	}
	//statOpts := minio.StatObjectOptions{}
	//getOpts := minio.GetObjectOptions{}
	if meta, ok := metadata.MinioMetaFromContext(ctx); ok {
		for k, v := range meta {
			//statOpts.Set(k, v)
			//getOpts.Set(k, v)
			copyMeta[k] = v
		}
	}

	// Load current metadata
	objectInfo, e := src.Client.StatObject(ctx, src.ObjectsBucket, node.GetUuid(), nil)
	if e != nil {
		return "", e
	}
	for k, v := range objectInfo.Metadata {
		copyMeta[k] = strings.Join(v, "")
	}

	if src.Client.CopyObjectMultipartThreshold() > 0 && objectInfo.Size > src.Client.CopyObjectMultipartThreshold() && src.StorageType != object.StorageType_LOCAL {

		// Cannot CopyObject on itself for files bigger than 5GB - compute Md5 and store it as metadata instead
		// Not necessary for real minio on fs (but required for Minio as S3 gateway or real S3)
		mm2, _ := metadata.MinioMetaFromContext(ctx)
		readCloser, _, e := src.Client.GetObject(ctx, src.ObjectsBucket, node.GetUuid(), mm2)
		if e != nil {
			return "", e
		}
		defer readCloser.Close()
		h := md5.New()
		if _, err := io.Copy(h, readCloser); err != nil {
			return "", err
		}
		checksum := fmt.Sprintf("%x", h.Sum(nil))
		copyMeta[common.XAmzMetaContentMd5] = checksum
		err := src.Client.CopyObjectMultipart(ctx, objectInfo, src.ObjectsBucket, objectInfo.Key, src.ObjectsBucket, objectInfo.Key, copyMeta, nil)
		return checksum, err

	} else {

		// Perform in-place copy to trigger ETag recomputation inside storage
		newInfo, copyErr := src.Client.CopyObject(ctx, src.ObjectsBucket, objectInfo.Key, src.ObjectsBucket, objectInfo.Key, copyMeta, nil, nil)
		if copyErr != nil {
			return "", copyErr
		}
		return newInfo.ETag, nil

	}

}

func (f *FlatStorageHandler) encPlainSizeRecompute(ctx context.Context, nodeUUID, dsName string) (int64, error) {
	if keyClient == nil {
		keyClient = encryption.NewNodeKeyManagerClient(grpc2.GetClientConnFromCtx(ctx, common.ServiceEncKey))
	}
	if resp, e := keyClient.GetNodePlainSize(ctx, &encryption.GetNodePlainSizeRequest{
		NodeId: nodeUUID,
		UserId: "ds:" + dsName,
	}); e == nil {
		return resp.GetSize(), nil
	} else {
		return 0, e
	}
}
