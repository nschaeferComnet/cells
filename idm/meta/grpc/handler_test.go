/*
 * Copyright (c) 2018. Abstrium SAS <team (at) pydio.com>
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

package grpc

import (
	"context"
	"sync"
	"testing"

	"github.com/pydio/cells/v4/common/dao"
	"github.com/pydio/cells/v4/common/dao/sqlite"
	"github.com/pydio/cells/v4/common/proto/idm"
	"github.com/pydio/cells/v4/common/service/context/metadata"
	"github.com/pydio/cells/v4/common/utils/configx"
	"github.com/pydio/cells/v4/idm/meta"

	. "github.com/smartystreets/goconvey/convey"
)

var (
	wg      sync.WaitGroup
	ctx     context.Context
	mockDAO meta.DAO
)

func TestMain(m *testing.M) {
	var options = configx.New()

	if d, e := dao.InitDAO(sqlite.Driver, sqlite.SharedMemDSN, "meta_grpc", meta.NewDAO, options); e != nil {
		panic(e)
	} else {
		mockDAO = d.(meta.DAO)
	}

	ctx = context.Background()
	ctx = metadata.NewContext(ctx, map[string]string{})

	m.Run()
	wg.Wait()
}

func TestRole(t *testing.T) {

	h := &Handler{dao: mockDAO}

	Convey("Test DAO", t, func() {
		nsDao := mockDAO.GetNamespaceDao()
		So(nsDao, ShouldNotBeNil)
	})

	Convey("Test NS Handler", t, func() {

		namespaces := []*idm.UserMetaNamespace{{
			Namespace: "namespace",
			Label:     "label",
		}}
		_, err := h.UpdateUserMetaNamespace(ctx, &idm.UpdateUserMetaNamespaceRequest{Namespaces: namespaces, Operation: idm.UpdateUserMetaNamespaceRequest_PUT})
		So(err, ShouldBeNil)

	})

}
