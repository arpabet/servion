/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package serviongrpc

import (
	"context"
	"errors"
	"strings"

	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// defaultAuthExempt holds the method prefixes that bypass authentication out of
// the box (health checks and server reflection) so infrastructure probes and
// debugging tools keep working without credentials.
var defaultAuthExempt = []string{
	"/grpc.health.v1.Health/",
	"/grpc.reflection.",
}

type implAuthInterceptor struct {
	Authenticator servion.Authenticator `inject:""`
	Properties    glue.Properties       `inject:""`

	beanOrder int
	exempt    []string
}

/*
AuthInterceptor returns an Interceptor bean that authenticates incoming gRPC
calls using the servion.Authenticator in the context. It is the gRPC counterpart
of servion.AuthMiddleware and implements both UnaryInterceptor and
StreamInterceptor.

Each non-exempt call must carry an "authorization: Bearer <token>" header; the
token is validated by the Authenticator and on success the resulting
servion.AuthInfo is stored in the context, retrievable downstream with
servion.AuthFromContext - exactly as with the HTTP middleware. Health and
reflection methods are exempt by default; extend the exempt list with the
comma-separated property "grpc.auth.exempt" (fully-qualified method prefixes,
e.g. "/myapp.Public/").

beanOrder controls chaining order relative to other interceptors; lower runs
first.
*/
func AuthInterceptor(beanOrder int) Interceptor {
	return &implAuthInterceptor{beanOrder: beanOrder}
}

func (t *implAuthInterceptor) PostConstruct() error {
	t.exempt = append([]string{}, defaultAuthExempt...)
	for _, p := range strings.Split(t.Properties.GetString("grpc.auth.exempt", ""), ",") {
		if p = strings.TrimSpace(p); p != "" {
			t.exempt = append(t.exempt, p)
		}
	}
	return nil
}

func (t *implAuthInterceptor) BeanName() string { return "grpc-auth-interceptor" }

func (t *implAuthInterceptor) BeanOrder() int { return t.beanOrder }

func (t *implAuthInterceptor) isExempt(fullMethod string) bool {
	for _, p := range t.exempt {
		if strings.HasPrefix(fullMethod, p) {
			return true
		}
	}
	return false
}

func (t *implAuthInterceptor) authenticate(ctx context.Context, fullMethod string) (context.Context, error) {

	if t.isExempt(fullMethod) {
		return ctx, nil
	}

	token, err := bearerFromContext(ctx)
	if err != nil {
		return nil, err
	}

	info, err := t.Authenticator.Authenticate(token)
	if errors.Is(err, servion.ErrUnauthorized) {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	if errors.Is(err, servion.ErrServiceUnavailable) {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return servion.ContextWithAuth(ctx, info), nil
}

func (t *implAuthInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, err := t.authenticate(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func (t *implAuthInterceptor) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, err := t.authenticate(ss.Context(), info.FullMethod)
		if err != nil {
			return err
		}
		return handler(srv, &authServerStream{ServerStream: ss, ctx: ctx})
	}
}

// authServerStream overrides Context so downstream stream handlers observe the
// authenticated context produced by the interceptor.
type authServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authServerStream) Context() context.Context { return s.ctx }

func bearerFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", status.Error(codes.Unauthenticated, "invalid authorization header")
	}
	return parts[1], nil
}
