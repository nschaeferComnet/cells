/*
 * Copyright (c) 2019-2022. Abstrium SAS <team (at) pydio.com>
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
	"fmt"
	"time"

	"log"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/test/bufconn"

	clientcontext "github.com/pydio/cells/v4/common/client/context"
	cgrpc "github.com/pydio/cells/v4/common/client/grpc"
	pbregistry "github.com/pydio/cells/v4/common/proto/registry"
	"github.com/pydio/cells/v4/common/registry"
	servercontext "github.com/pydio/cells/v4/common/server/context"
	"github.com/pydio/cells/v4/common/service"
	servicecontext "github.com/pydio/cells/v4/common/service/context"
	discoveryregistry "github.com/pydio/cells/v4/discovery/registry"

	_ "github.com/pydio/cells/v4/common/registry/config"
)

type mock struct {
	helloworld.UnimplementedGreeterServer
}

func (m *mock) SayHello(ctx context.Context, req *helloworld.HelloRequest) (*helloworld.HelloReply, error) {
	resp := &helloworld.HelloReply{Message: "Greetings " + req.Name}

	return resp, nil
}

func createApp1(reg registry.Registry) *bufconn.Listener {
	ctx := context.Background()
	ctx = servicecontext.WithRegistry(ctx, reg)
	ctx = servercontext.WithRegistry(ctx, reg)

	listener := bufconn.Listen(1024 * 1024)
	srv := New(ctx, WithListener(listener))

	svcRegistry := service.NewService(
		service.Name("pydio.grpc.test.registry"),
		service.Context(ctx),
		service.WithServer(srv),
		service.WithGRPC(func(ctx context.Context, srv *grpc.Server) error {
			pbregistry.RegisterRegistryServer(srv, discoveryregistry.NewHandler(reg))
			return nil
		}),
	)

	// Create a new service
	svcHello := service.NewService(
		service.Name("pydio.grpc.test.service"),
		service.Context(ctx),
		service.WithServer(srv),
		service.WithGRPC(func(ctx context.Context, srv *grpc.Server) error {
			helloworld.RegisterGreeterServer(srv, &mock{})
			return nil
		}),
	)

	srv.BeforeServe(svcHello.Start)
	srv.BeforeStop(svcHello.Stop)

	srv.BeforeServe(svcRegistry.Start)
	srv.BeforeStop(svcRegistry.Stop)

	go func() {
		if err := srv.Serve(); err != nil {
			log.Fatal(err)
		}
	}()

	return listener
}

func createApp2(reg registry.Registry) {
	ctx := context.Background()
	ctx = servicecontext.WithRegistry(ctx, reg)
	ctx = servercontext.WithRegistry(ctx, reg)

	listener := bufconn.Listen(1024 * 1024)
	srv := New(ctx, WithListener(listener))

	// Create a new service
	svcHello := service.NewService(
		service.Name("pydio.grpc.test.service"),
		service.Context(ctx),
		service.WithServer(srv),
		service.WithGRPC(func(ctx context.Context, srv *grpc.Server) error {
			helloworld.RegisterGreeterServer(srv, &mock{})
			return nil
		}),
	)

	srv.BeforeServe(svcHello.Start)
	srv.BeforeStop(svcHello.Stop)

	go func() {
		if err := srv.Serve(); err != nil {
			log.Fatal(err)
		}
	}()
}

func TestServiceRegistry(t *testing.T) {

	ctx := context.Background()
	mem, err := registry.OpenRegistry(ctx, "mem:///")
	if err != nil {
		log.Fatal("could not create memory registry", err)
	}

	listenerApp1 := createApp1(&delayedRegistry{mem})

	conn, err := grpc.Dial("cells:///", grpc.WithInsecure(), grpc.WithResolvers(cgrpc.NewBuilder(mem)), grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		return listenerApp1.Dial()
	}))
	if err != nil {
		log.Fatal("no conn", err)
	}

	ctx = clientcontext.WithClientConn(ctx, conn)

	cli1 := helloworld.NewGreeterClient(cgrpc.GetClientConnFromCtx(ctx, "test.registry"))
	resp1, err1 := cli1.SayHello(ctx, &helloworld.HelloRequest{Name: "test"})

	fmt.Println(resp1, err1)

	cli2 := helloworld.NewGreeterClient(cgrpc.GetClientConnFromCtx(ctx, "service.that.does.not.exist"))
	resp2, err2 := cli2.SayHello(ctx, &helloworld.HelloRequest{Name: "test"}, grpc.WaitForReady(false))

	fmt.Println(resp2, err2)

	//reg := registryservice.NewRegistry(registryservice.WithConn(conn))
	//
	//createApp2(reg)
	//
	//fmt.Println(listenerApp1, reg)

	//cgrpc.RegisterMock("test.registry", discoverytest.NewRegistryService())
	//ctx := context.Background()
	//mem, _ := registry.OpenRegistry(ctx, "memory://")
	//
	//ctx = servicecontext.WithRegistry(ctx, reg)
	//ctx = servercontext.WithRegistry(ctx, reg)
	//
	//listener := bufconn.Listen(1024 * 1024)
	//srv := New(ctx, WithListener(listener))
	//
	//svcRegistry := service.NewService(
	//	service.Name("test.registry"),
	//	service.Context(ctx),
	//	service.WithServer(srv),
	//	service.WithGRPC(func(ctx context.Context, srv *grpc.Server) error {
	//		pbregistry.RegisterRegistryServer(srv, discoveryregistry.NewHandler(mem))
	//		return nil
	//	}),
	//)
	//
	//srv.BeforeServe(svcRegistry.Start)
	//srv.BeforeStop(svcRegistry.Stop)
	//
	//reg, _ := registry.OpenRegistry(ctx, "grpc://test.registry")
	//
	//// Create a new service
	//svc := service.NewService(
	//	service.Name("test.service"),
	//	service.Context(ctx),
	//	service.WithServer(srv),
	//	service.WithGRPC(func(ctx context.Context, srv *grpc.Server) error {
	//		helloworld.RegisterGreeterServer(srv, &mock{})
	//		return nil
	//	}),
	//)
	//
	//srv.BeforeServe(svc.Start)
	//srv.BeforeStop(svc.Stop)
	//
	//go func() {
	//	<-time.After(5 * time.Second)
	//	if err := srv.Serve(); err != nil {
	//		log.Fatal(err)
	//	}
	//}()
	//conn2 := cgrpc.GetClientConnFromCtx(ctx, "test.service", cgrpc.WithDialOptions(
	//	grpc.WithResolvers(cgrpc.NewBuilder(reg)),
	//))

	//
	//cli2 := helloworld.NewGreeterClient(conn)
	//resp2, err2 := cli2.SayHello(ctx, &helloworld.HelloRequest{Name: "test2"})
	//
	//fmt.Println(resp2, err2)
}

type delayedRegistry struct {
	registry.Registry
}

func (r *delayedRegistry) Register(i registry.Item) error {
	go func() {
		<-time.After(5 * time.Second)
		r.Registry.Register(i)
	}()

	return nil
}
