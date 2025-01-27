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

package s3

import (
	"context"
	"log"
	"sync"
	"testing"

	minio "github.com/minio/minio-go/v7"
	. "github.com/smartystreets/goconvey/convey"

	"github.com/pydio/cells/v4/common"
	"github.com/pydio/cells/v4/common/proto/tree"
)

func TestStat(t *testing.T) {
	Convey("Test Stat file", t, func() {
		c := NewS3Mock()
		fileInfo, err := c.Stat("file")
		fakeFileInfo := &S3FileInfo{
			Object: minio.ObjectInfo{
				Key:  "file",
				ETag: "filemd5",
			},
			isDir: false,
		}
		So(err, ShouldBeNil)
		So(fileInfo, ShouldNotBeNil)
		So(fileInfo, ShouldResemble, fakeFileInfo)

	})

	Convey("Test Stat folder", t, func() {
		c := NewS3Mock()
		fileInfo, err := c.Stat("folder")
		fakeFolderInfo := &S3FileInfo{
			Object: minio.ObjectInfo{
				Key: "folder/" + common.PydioSyncHiddenFile,
			},
			isDir: true,
		}
		So(err, ShouldBeNil)
		So(fileInfo, ShouldNotBeNil)
		So(fileInfo, ShouldResemble, fakeFolderInfo)

	})

	Convey("Test Stat unknown file", t, func() {
		c := NewS3Mock()
		fileInfo, err := c.Stat("file2")
		So(err, ShouldNotBeNil)
		So(fileInfo, ShouldBeNil)
	})
}

func TestLoadNodeS3(t *testing.T) {

	Convey("Load existing node", t, func() {

		c := NewS3Mock()
		node, err := c.LoadNode(context.Background(), "file", true)
		So(err, ShouldBeNil)
		So(node, ShouldNotBeNil)
		So(node.Etag, ShouldEqual, "filemd5")

	})

}

func TestWalkS3(t *testing.T) {

	Convey("Test walking the tree", t, func() {

		c := NewS3Mock()
		objects := make(map[string]*tree.Node)
		walk := func(path string, node *tree.Node, err error) {
			log.Println("Walk " + path)
			objects[path] = node
		}
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Walk(walk, "/", true)
		}()
		wg.Wait()

		log.Println(objects)
		// Will include the root
		So(objects, ShouldHaveLength, 3)
		So(objects["folder"].Uuid, ShouldNotBeEmpty)
		So(objects["folder"].Etag, ShouldBeEmpty)
		So(objects["folder"].Type, ShouldEqual, tree.NodeType_COLLECTION)

		So(objects["file"].Uuid, ShouldBeEmpty)
		So(objects["file"].Etag, ShouldNotBeEmpty)
		So(objects["file"].Type, ShouldEqual, tree.NodeType_LEAF)
	})
}

func TestDeleteNodeS3(t *testing.T) {

	Convey("Test Delete Node", t, func() {

		c := NewS3Mock()
		err := c.DeleteNode(context.Background(), "file")
		So(err, ShouldBeNil)

	})

}

func TestMoveNodeS3(t *testing.T) {

	Convey("Test Move Node", t, func() {

		c := NewS3Mock()
		err := c.MoveNode(context.Background(), "/file", "/file1")
		So(err, ShouldBeNil)

	})

}

func TestGetWriterOnS3(t *testing.T) {

	Convey("Test Get Writer on node", t, func() {

		c := NewS3Mock()
		w, _, _, err := c.GetWriterOn(context.Background(), "/file", 0)
		So(err, ShouldBeNil)
		defer w.Close()
		So(w, ShouldNotBeNil)

	})

}

func TestGetReaderOnS3(t *testing.T) {

	Convey("Test Get Reader on node", t, func() {

		c := NewS3Mock()
		o, e := c.GetReaderOn("/file")
		So(o, ShouldNotBeNil)
		// We know that there will be an error as Object is not mocked, yet
		So(e, ShouldNotBeNil)

	})

}
