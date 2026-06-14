/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package serviongrpc

import (
	"fmt"
	"reflect"
	"sort"

	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type implGrpcServerFactory struct {
	Log        *zap.Logger         `inject:""`
	Properties glue.Properties     `inject:""`
	Services   []GrpcService       `inject:"optional,level=1"`
	Unary      []UnaryInterceptor  `inject:"optional,level=1"`
	Stream     []StreamInterceptor `inject:"optional,level=1"`

	beanName string
}

/*
GrpcServerFactory creates a *grpc.Server bean named beanName. It collects every
GrpcService bean in the same container and registers it on the server, and chains
any UnaryInterceptor / StreamInterceptor beans found in the container (ordered by
BeanOrder).

Recognized properties (prefixed by beanName):

	<beanName>.options             semicolon separated flags: "health;reflection"
	<beanName>.max-recv-msg-size   max inbound message size in bytes (0 = grpc default)
	<beanName>.max-send-msg-size   max outbound message size in bytes (0 = grpc default)

The "health" flag installs the standard grpc.health.v1.Health service (useful for
Kubernetes gRPC probes); the "reflection" flag enables server reflection (useful
for grpcurl and debugging).
*/
func GrpcServerFactory(beanName string) glue.FactoryBean {
	return &implGrpcServerFactory{beanName: beanName}
}

func (t *implGrpcServerFactory) Object() (object interface{}, err error) {

	defer servion.PanicToError(&err)

	options := servion.ParseOptions(t.Properties.GetString(fmt.Sprintf("%s.options", t.beanName), ""))

	var opts []grpc.ServerOption

	if n := t.Properties.GetInt(fmt.Sprintf("%s.max-recv-msg-size", t.beanName), 0); n > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(n))
	}
	if n := t.Properties.GetInt(fmt.Sprintf("%s.max-send-msg-size", t.beanName), 0); n > 0 {
		opts = append(opts, grpc.MaxSendMsgSize(n))
	}

	if unary := t.unaryInterceptors(); len(unary) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(unary...))
	}
	if stream := t.streamInterceptors(); len(stream) > 0 {
		opts = append(opts, grpc.ChainStreamInterceptor(stream...))
	}

	srv := grpc.NewServer(opts...)

	for _, svc := range t.Services {
		svc.RegisterGrpc(srv)
	}

	if options["health"] {
		registerHealth(srv)
	}
	if options["reflection"] {
		reflection.Register(srv)
	}

	var serviceList []string
	for name := range srv.GetServiceInfo() {
		serviceList = append(serviceList, name)
	}
	sort.Strings(serviceList)

	t.Log.Info("GrpcServerFactory",
		zap.String("bean", t.beanName),
		zap.Strings("services", serviceList),
		zap.Int("unaryInterceptors", len(t.Unary)),
		zap.Int("streamInterceptors", len(t.Stream)),
		zap.Bool("health", options["health"]),
		zap.Bool("reflection", options["reflection"]))

	return srv, nil
}

func (t *implGrpcServerFactory) ObjectType() reflect.Type { return GrpcServerClass }

func (t *implGrpcServerFactory) ObjectName() string { return t.beanName }

func (t *implGrpcServerFactory) Singleton() bool { return true }

func (t *implGrpcServerFactory) unaryInterceptors() []grpc.UnaryServerInterceptor {
	list := make([]UnaryInterceptor, len(t.Unary))
	copy(list, t.Unary)
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].BeanOrder() < list[j].BeanOrder()
	})
	out := make([]grpc.UnaryServerInterceptor, 0, len(list))
	for _, in := range list {
		if fn := in.UnaryInterceptor(); fn != nil {
			out = append(out, fn)
		}
	}
	return out
}

func (t *implGrpcServerFactory) streamInterceptors() []grpc.StreamServerInterceptor {
	list := make([]StreamInterceptor, len(t.Stream))
	copy(list, t.Stream)
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].BeanOrder() < list[j].BeanOrder()
	})
	out := make([]grpc.StreamServerInterceptor, 0, len(list))
	for _, in := range list {
		if fn := in.StreamInterceptor(); fn != nil {
			out = append(out, fn)
		}
	}
	return out
}
