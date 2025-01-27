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

package mock

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pydio/cells/v4/common/nodes/models"
)

// New creates a new mock Client with an optional list of buckets
func New(buckets ...string) *Client {
	c := &Client{
		Buckets: map[string]map[string][]byte{},
	}
	for _, b := range buckets {
		c.Buckets[b] = make(map[string][]byte)
	}
	return c
}

// Client is an in-memory implementation of the nodes.StorageClient interface
type Client struct {
	Buckets map[string]map[string][]byte
}

func (c *Client) ListBuckets(ctx context.Context) (bb []models.BucketInfo, e error) {
	for b := range c.Buckets {
		bb = append(bb, models.BucketInfo{Name: b, CreationDate: time.Now()})
	}
	return
}

func (c *Client) MakeBucket(ctx context.Context, bucketName string, location string) (err error) {
	if _, ok := c.Buckets[bucketName]; ok {
		return fmt.Errorf("bucket already exists")
	}
	c.Buckets[bucketName] = map[string][]byte{}
	return nil
}

func (c *Client) RemoveBucket(ctx context.Context, bucketName string) error {
	if _, ok := c.Buckets[bucketName]; !ok {
		return fmt.Errorf("bucket not found %s", bucketName)
	}
	delete(c.Buckets, bucketName)
	return nil
}

func (c *Client) GetObject(ctx context.Context, bucketName, objectName string, opts models.ReadMeta) (io.ReadCloser, models.ObjectInfo, error) {
	bucket, ok := c.Buckets[bucketName]
	if !ok {
		return nil, models.ObjectInfo{}, fmt.Errorf("bucket not found %s", bucketName)
	}
	if object, ok := bucket[objectName]; ok {
		return newReadCloser(object), models.ObjectInfo{Size: int64(len(object))}, nil
	} else {
		return nil, models.ObjectInfo{}, fmt.Errorf("object not found")
	}
}

func (c *Client) StatObject(ctx context.Context, bucketName, objectName string, opts models.ReadMeta) (models.ObjectInfo, error) {
	bucket, ok := c.Buckets[bucketName]
	if !ok {
		return models.ObjectInfo{}, fmt.Errorf("bucket not found %s", bucketName)
	}
	if object, ok := bucket[objectName]; ok {
		return models.ObjectInfo{Size: int64(len(object))}, nil
	} else {
		return models.ObjectInfo{}, fmt.Errorf("object not found")
	}
}

func (c *Client) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts models.PutMeta) (n int64, err error) {
	bucket, ok := c.Buckets[bucketName]
	if !ok {
		return 0, fmt.Errorf("bucket not found %s", bucketName)
	}
	bucket[objectName], _ = io.ReadAll(reader)
	return int64(len(bucket[objectName])), nil
}

func (c *Client) RemoveObject(ctx context.Context, bucketName, objectName string) error {
	bucket, ok := c.Buckets[bucketName]
	if !ok {
		return fmt.Errorf("bucket not found %s", bucketName)
	}
	if _, ok := bucket[objectName]; !ok {
		return fmt.Errorf("object not found")
	}
	delete(bucket, objectName)
	return nil
}

func (c *Client) ListObjects(ctx context.Context, bucketName, prefix, marker, delimiter string, maxKeys int) (result models.ListBucketResult, err error) {
	bucket, ok := c.Buckets[bucketName]
	if !ok {
		return result, fmt.Errorf("bucket not found %s", bucketName)
	}
	for objName, data := range bucket {
		result.Contents = append(result.Contents, models.ObjectInfo{
			Key:          objName,
			LastModified: time.Now(),
			Size:         int64(len(data)),
		})
	}
	return result, nil
}

func (c *Client) NewMultipartUpload(ctx context.Context, bucket, object string, opts models.PutMeta) (uploadID string, err error) {
	return "", fmt.Errorf("not.implemented")
}

func (c *Client) ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker, delimiter string, maxUploads int) (result models.ListMultipartUploadsResult, err error) {
	return result, fmt.Errorf("not.implemented")
}

func (c *Client) ListObjectParts(ctx context.Context, bucketName, objectName, uploadID string, partNumberMarker, maxParts int) (models.ListObjectPartsResult, error) {
	return models.ListObjectPartsResult{}, fmt.Errorf("not.implemented")
}

func (c *Client) CompleteMultipartUpload(ctx context.Context, bucket, object, uploadID string, parts []models.MultipartObjectPart) (string, error) {
	return "", fmt.Errorf("not.implemented")
}

func (c *Client) PutObjectPart(ctx context.Context, bucket, object, uploadID string, partID int, data io.Reader, size int64, md5Base64, sha256Hex string) (models.MultipartObjectPart, error) {
	return models.MultipartObjectPart{}, fmt.Errorf("not.implemented")
}

func (c *Client) AbortMultipartUpload(ctx context.Context, bucket, object, uploadID string) error {
	return fmt.Errorf("not.implemented")
}

func (c *Client) CopyObject(ctx context.Context, sourceBucket, sourceObject, destBucket, destObject string, srcMeta, metadata map[string]string, progress io.Reader) (models.ObjectInfo, error) {
	srcBucket, ok := c.Buckets[sourceBucket]
	if !ok {
		return models.ObjectInfo{}, fmt.Errorf("src bucket not found")
	}
	srcObjBytes, ok2 := srcBucket[sourceObject]
	if !ok2 {
		return models.ObjectInfo{}, fmt.Errorf("src object not found")
	}
	dstBucket, ok3 := c.Buckets[destBucket]
	if !ok3 {
		return models.ObjectInfo{}, fmt.Errorf("dest bucket not found")
	}
	dstBucket[destObject] = make([]byte, len(srcObjBytes))
	copy(dstBucket[destObject], srcObjBytes)
	return models.ObjectInfo{Size: int64(len(srcObjBytes))}, nil
}

func (c *Client) CopyObjectMultipartThreshold() int64 {
	return 0
}

func (c *Client) CopyObjectMultipart(ctx context.Context, srcObject models.ObjectInfo, srcBucket, srcPath, destBucket, destPath string, meta map[string]string, progress io.Reader) error {
	return fmt.Errorf("not.implemented")
}

type mockReadCloser struct {
	*bytes.Buffer
}

func newReadCloser(bb []byte) *mockReadCloser {
	return &mockReadCloser{Buffer: bytes.NewBuffer(bb)}
}

func (m *mockReadCloser) Close() error {
	return nil
}
