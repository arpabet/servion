/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package serviongrpc_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	serviongrpc "go.arpabet.com/servion/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// --- a tiny hand-wired service so the tests need no protoc ---

const helloMethod = "/servion.test.Echo/Hello"

type echoService struct{}

func (t *echoService) RegisterGrpc(srv *grpc.Server) {
	srv.RegisterService(&echoServiceDesc, t)
}

func helloHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(wrapperspb.StringValue)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		subject := "anonymous"
		if info, ok := servion.AuthFromContext(ctx); ok {
			subject = info.Subject
		}
		return wrapperspb.String("hello " + req.(*wrapperspb.StringValue).Value + " from " + subject), nil
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: helloMethod}
	return interceptor(ctx, in, info, handler)
}

var echoServiceDesc = grpc.ServiceDesc{
	ServiceName: "servion.test.Echo",
	HandlerType: (*interface{})(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "Hello", Handler: helloHandler},
	},
}

// stubAuth accepts a single known token.
type stubAuth struct{}

func stubAuthBean() servion.Authenticator { return stubAuth{} }

func (stubAuth) Authenticate(token string) (servion.AuthInfo, error) {
	if token == "good-token" {
		return servion.AuthInfo{Subject: "alice", Roles: []string{"admin"}}, nil
	}
	return servion.AuthInfo{}, servion.ErrUnauthorized
}

// startServer builds a glue context, binds and serves the gRPC server in the
// background and returns its actual listen address plus a teardown func.
func startServer(t *testing.T, beans ...interface{}) (addr string, teardown func()) {
	t.Helper()

	all := append([]interface{}{
		glue.MapPropertySource{"grpc-server.bind-address": "127.0.0.1:0"},
		servion.ZapLogFactory(true),
	}, beans...)

	ctx, err := glue.New(all...)
	if err != nil {
		t.Fatalf("context: %v", err)
	}

	list := ctx.Bean(servion.ServerClass, glue.DefaultSearchLevel)
	if len(list) != 1 {
		ctx.Close()
		t.Fatalf("expected exactly 1 servion.Server, got %d", len(list))
	}
	srv := list[0].Object().(servion.Server)

	if err := srv.Bind(); err != nil {
		ctx.Close()
		t.Fatalf("bind: %v", err)
	}
	go srv.Serve()

	return srv.ListenAddress().String(), func() {
		srv.Shutdown()
		ctx.Close()
	}
}

func dial(t *testing.T, addr string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

func TestGrpcServer_HealthAndReflection(t *testing.T) {

	addr, teardown := startServer(t,
		glue.MapPropertySource{"grpc-server.options": "health;reflection"},
		serviongrpc.GrpcServerScanner("grpc-server"),
	)
	defer teardown()

	conn := dial(t, addr)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("expected SERVING, got %v", resp.Status)
	}
}

func TestGrpcServer_ServiceCall(t *testing.T) {

	addr, teardown := startServer(t,
		serviongrpc.GrpcServerScanner("grpc-server", &echoService{}),
	)
	defer teardown()

	conn := dial(t, addr)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp := new(wrapperspb.StringValue)
	if err := conn.Invoke(ctx, helloMethod, wrapperspb.String("world"), resp); err != nil {
		t.Fatalf("invoke: %v", err)
	}
	// no auth interceptor configured -> anonymous identity
	if resp.Value != "hello world from anonymous" {
		t.Fatalf("unexpected response: %q", resp.Value)
	}
}

func TestGrpcServer_AuthInterceptor(t *testing.T) {

	addr, teardown := startServer(t,
		serviongrpc.GrpcServerScanner("grpc-server",
			&echoService{},
			serviongrpc.AuthInterceptor(10),
			stubAuthBean(),
		),
	)
	defer teardown()

	conn := dial(t, addr)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp := new(wrapperspb.StringValue)

	// 1) no token -> Unauthenticated
	err := conn.Invoke(ctx, helloMethod, wrapperspb.String("world"), resp)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}

	// 2) bad token -> Unauthenticated
	badCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer nope"))
	err = conn.Invoke(badCtx, helloMethod, wrapperspb.String("world"), resp)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated for bad token, got %v", err)
	}

	// 3) good token -> handler runs and sees the authenticated subject
	goodCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer good-token"))
	if err := conn.Invoke(goodCtx, helloMethod, wrapperspb.String("world"), resp); err != nil {
		t.Fatalf("invoke with good token: %v", err)
	}
	if !strings.Contains(resp.Value, "from alice") {
		t.Fatalf("expected authenticated subject in response, got %q", resp.Value)
	}
}

func TestGrpcClientFactory_Connect(t *testing.T) {

	addr, teardown := startServer(t,
		glue.MapPropertySource{"grpc-server.options": "health"},
		serviongrpc.GrpcServerScanner("grpc-server"),
	)
	defer teardown()

	// build a client through the factory, pointing it at the real address
	clientCtx, err := glue.New(
		glue.MapPropertySource{"grpc-client.connect-address": addr},
		servion.ZapLogFactory(true),
		serviongrpc.GrpcClientScanner("grpc-client"),
	)
	if err != nil {
		t.Fatalf("client context: %v", err)
	}
	defer clientCtx.Close()

	list := clientCtx.Bean(serviongrpc.GrpcClientConnClass, glue.DefaultSearchLevel)
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 *grpc.ClientConn, got %d", len(list))
	}
	conn := list[0].Object().(*grpc.ClientConn)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check via factory client: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("expected SERVING, got %v", resp.Status)
	}
}
