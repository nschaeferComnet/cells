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
package role

import (
	"sync"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/protobuf/types/known/anypb"

	// Run tests against SQLite
	"github.com/pydio/cells/v4/common/dao"
	"github.com/pydio/cells/v4/common/dao/sqlite"
	"github.com/pydio/cells/v4/common/proto/idm"
	"github.com/pydio/cells/v4/common/proto/service"
	"github.com/pydio/cells/v4/common/service/errors"
	"github.com/pydio/cells/v4/common/sql"
	"github.com/pydio/cells/v4/common/utils/configx"
	"github.com/pydio/cells/v4/common/utils/uuid"
)

var (
	mockDAO DAO
	wg      sync.WaitGroup
)

func TestMain(m *testing.M) {

	var options = configx.New()

	if d, e := dao.InitDAO(sqlite.Driver, sqlite.SharedMemDSN, "role", NewDAO, options); e != nil {
		panic(e)
	} else {
		mockDAO = d.(DAO)
	}

	m.Run()
	wg.Wait()
}

func TestCrud(t *testing.T) {

	Convey("Create Role", t, func() {
		{
			_, _, err := mockDAO.Add(&idm.Role{
				Label: "",
			})
			So(err, ShouldNotBeNil)
			So(errors.FromError(err).Code, ShouldEqual, 400)
		}
		{
			r, _, err := mockDAO.Add(&idm.Role{
				Label:       "NewRole",
				LastUpdated: int32(time.Now().Unix()),
			})

			So(err, ShouldBeNil)
			So(r.Uuid, ShouldNotBeEmpty)
		}
	})

	Convey("Get Role", t, func() {

		roleUuid := uuid.New()
		gRoleUuid := uuid.New()
		roleTime := int32(time.Now().Unix())
		_, _, err := mockDAO.Add(&idm.Role{
			Uuid:        roleUuid,
			Label:       "New Role",
			LastUpdated: roleTime,
			GroupRole:   false,
		})
		So(err, ShouldBeNil)
		_, _, err2 := mockDAO.Add(&idm.Role{
			Uuid:        gRoleUuid,
			Label:       "Group Role",
			LastUpdated: roleTime,
			GroupRole:   true,
		})
		So(err2, ShouldBeNil)
		_, _, err3 := mockDAO.Add(&idm.Role{
			Uuid:        uuid.New(),
			Label:       "User Role",
			LastUpdated: roleTime,
			UserRole:    true,
		})
		So(err3, ShouldBeNil)
		ownedUUID := uuid.New()
		_, _, err4 := mockDAO.Add(&idm.Role{
			Uuid:          ownedUUID,
			Label:         "Owned Role",
			LastUpdated:   roleTime,
			ForceOverride: true,
		})
		So(err4, ShouldBeNil)

		singleQ := &idm.RoleSingleQuery{
			Uuid: []string{roleUuid},
		}
		singleQA, _ := anypb.New(singleQ)
		query := &service.Query{
			SubQueries: []*anypb.Any{singleQA},
		}
		var roles []*idm.Role
		e := mockDAO.Search(query, &roles)
		So(e, ShouldBeNil)
		So(roles, ShouldHaveLength, 1)
		for _, role := range roles {
			So(role.Uuid, ShouldEqual, roleUuid)
			So(role.Label, ShouldEqual, "New Role")
			So(role.LastUpdated, ShouldEqual, roleTime)
			So(role.GroupRole, ShouldBeFalse)
			So(role.UserRole, ShouldBeFalse)
			break
		}

		{
			c, e := mockDAO.Count(&service.Query{})
			So(e, ShouldBeNil)
			So(c, ShouldEqual, 5)
		}

		{
			count, e2 := mockDAO.Count(query)
			So(e2, ShouldBeNil)
			So(count, ShouldEqual, 1)
		}

		{
			count, e2 := mockDAO.Delete(query)
			So(e2, ShouldBeNil)
			So(count, ShouldEqual, 1)
		}

		{
			count, e2 := mockDAO.Count(query)
			So(e2, ShouldBeNil)
			So(count, ShouldEqual, 0)
		}

		{
			c, e := mockDAO.Count(&service.Query{})
			So(e, ShouldBeNil)
			So(c, ShouldEqual, 4)
		}

		{
			singleQA, _ := anypb.New(&idm.RoleSingleQuery{
				IsGroupRole: true,
			})
			query := &service.Query{
				SubQueries: []*anypb.Any{singleQA},
			}
			c, e := mockDAO.Count(query)
			So(e, ShouldBeNil)
			So(c, ShouldEqual, 1)
		}

		{
			singleQA, _ := anypb.New(&idm.RoleSingleQuery{
				IsGroupRole: true,
				Not:         true,
			})
			query := &service.Query{
				SubQueries: []*anypb.Any{singleQA},
			}
			c, e := mockDAO.Count(query)
			So(e, ShouldBeNil)
			So(c, ShouldEqual, 3)
		}

		{
			singleQA, _ := anypb.New(&idm.RoleSingleQuery{
				IsUserRole: true,
			})
			query := &service.Query{
				SubQueries: []*anypb.Any{singleQA},
			}
			c, e := mockDAO.Count(query)
			So(e, ShouldBeNil)
			So(c, ShouldEqual, 1)
		}

		{
			singleQA, _ := anypb.New(&idm.RoleSingleQuery{
				IsUserRole: true,
				Not:        true,
			})
			query := &service.Query{
				SubQueries: []*anypb.Any{singleQA},
			}
			c, e := mockDAO.Count(query)
			So(e, ShouldBeNil)
			So(c, ShouldEqual, 3)
		}

		{
			singleQA, _ := anypb.New(&idm.RoleSingleQuery{
				Uuid: []string{ownedUUID},
			})
			query := &service.Query{
				SubQueries: []*anypb.Any{singleQA},
			}
			roles = []*idm.Role{}
			e := mockDAO.Search(query, &roles)
			So(e, ShouldBeNil)
			So(roles, ShouldHaveLength, 1)
			So(roles[0].ForceOverride, ShouldBeTrue)
		}

		// {
		// 	singleQA, _ := anypb.New(&idm.RoleSingleQuery{})
		// 	query := &service.Query{
		// 		SubQueries: []*anypb.Any{singleQA},
		// 	}
		// 	c, e := mockDAO.Count(query)
		// 	So(e, ShouldBeNil)
		// 	So(c, ShouldEqual, 1)
		// }

		// {
		// 	singleQA, _ := anypb.New(&idm.RoleSingleQuery{})
		// 	query := &service.Query{
		// 		SubQueries: []*anypb.Any{singleQA},
		// 	}
		// 	c, e := mockDAO.Count(query)
		// 	So(e, ShouldBeNil)
		// 	So(c, ShouldEqual, 1)
		// }

		// {
		// 	singleQA, _ := anypb.New(&idm.RoleSingleQuery{
		// 		Not: true,
		// 	})
		// 	query := &service.Query{
		// 		SubQueries: []*anypb.Any{singleQA},
		// 	}
		// 	c, e := mockDAO.Count(query)
		// 	So(e, ShouldBeNil)
		// 	So(c, ShouldEqual, 3)
		// }

		{
			singleQA, _ := anypb.New(&idm.RoleSingleQuery{
				Label: "New*",
			})
			query := &service.Query{
				SubQueries: []*anypb.Any{singleQA},
			}
			c, e := mockDAO.Count(query)
			So(e, ShouldBeNil)
			So(c, ShouldEqual, 1)
		}

		{
			_, _, err2 := mockDAO.Add(&idm.Role{
				Uuid:        gRoleUuid,
				Label:       "Rename Role",
				LastUpdated: 0,
				GroupRole:   true,
			})
			So(err2, ShouldBeNil)
		}

	})

}

func TestQueryBuilder(t *testing.T) {

	Convey("Query Builder", t, func() {

		singleQ1, singleQ2 := new(idm.RoleSingleQuery), new(idm.RoleSingleQuery)

		singleQ1.Uuid = []string{"role1"}
		singleQ2.Uuid = []string{"role2"}

		singleQ1Any, err := anypb.New(singleQ1)
		So(err, ShouldBeNil)

		singleQ2Any, err := anypb.New(singleQ2)
		So(err, ShouldBeNil)

		var singleQueries []*anypb.Any
		singleQueries = append(singleQueries, singleQ1Any)
		singleQueries = append(singleQueries, singleQ2Any)

		simpleQuery := &service.Query{
			SubQueries: singleQueries,
			Operation:  service.OperationType_OR,
			Offset:     0,
			Limit:      10,
		}

		s := sql.NewQueryBuilder(simpleQuery, new(queryBuilder)).Expression("sqlite")
		So(s, ShouldNotBeNil)
		//So(s, ShouldEqual, "(uuid='role1') OR (uuid='role2')")

	})

	Convey("Query Builder W/ subquery", t, func() {

		singleQ1, singleQ2, singleQ3 := new(idm.RoleSingleQuery), new(idm.RoleSingleQuery), new(idm.RoleSingleQuery)

		singleQ1.Uuid = []string{"role1"}
		singleQ2.Uuid = []string{"role2"}
		singleQ3.Uuid = []string{"role3_1", "role3_2", "role3_3"}

		singleQ1Any, err := anypb.New(singleQ1)
		So(err, ShouldBeNil)

		singleQ2Any, err := anypb.New(singleQ2)
		So(err, ShouldBeNil)

		singleQ3Any, err := anypb.New(singleQ3)
		So(err, ShouldBeNil)

		subQuery1 := &service.Query{
			SubQueries: []*anypb.Any{singleQ1Any, singleQ2Any},
			Operation:  service.OperationType_OR,
		}

		subQuery2 := &service.Query{
			SubQueries: []*anypb.Any{singleQ3Any},
		}

		subQuery1Any, err := anypb.New(subQuery1)
		So(err, ShouldBeNil)

		subQuery2Any, err := anypb.New(subQuery2)
		So(err, ShouldBeNil)

		composedQuery := &service.Query{
			SubQueries: []*anypb.Any{
				subQuery1Any,
				subQuery2Any,
			},
			Offset:    0,
			Limit:     10,
			Operation: service.OperationType_AND,
		}

		s := sql.NewQueryBuilder(composedQuery, new(queryBuilder)).Expression("sqlite")
		So(s, ShouldNotBeNil)
		//So(s, ShouldEqual, "((uuid='role1') OR (uuid='role2')) AND ((uuid in ('role3_1','role3_2','role3_3')))")

	})

}

func TestResourceRules(t *testing.T) {

	Convey("Test Add Rule", t, func() {

		err := mockDAO.AddPolicy("resource-id", &service.ResourcePolicy{Action: service.ResourcePolicyAction_READ, Subject: "subject1"})
		So(err, ShouldBeNil)

	})

	Convey("Select Rules", t, func() {

		rules, err := mockDAO.GetPoliciesForResource("resource-id")
		So(rules, ShouldHaveLength, 1)
		So(err, ShouldBeNil)

	})

	Convey("Delete Rules", t, func() {

		err := mockDAO.DeletePoliciesForResource("resource-id")
		So(err, ShouldBeNil)

		rules, err := mockDAO.GetPoliciesForResource("resource-id")
		So(rules, ShouldHaveLength, 0)
		So(err, ShouldBeNil)

	})

	Convey("Delete Rules For Action", t, func() {

		mockDAO.AddPolicy("resource-id", &service.ResourcePolicy{Action: service.ResourcePolicyAction_READ, Subject: "subject1"})
		mockDAO.AddPolicy("resource-id", &service.ResourcePolicy{Action: service.ResourcePolicyAction_WRITE, Subject: "subject1"})

		rules, err := mockDAO.GetPoliciesForResource("resource-id")
		So(rules, ShouldHaveLength, 2)

		err = mockDAO.DeletePoliciesForResourceAndAction("resource-id", service.ResourcePolicyAction_READ)
		So(err, ShouldBeNil)

		rules, err = mockDAO.GetPoliciesForResource("resource-id")
		So(rules, ShouldHaveLength, 1)
		So(err, ShouldBeNil)

	})

}
