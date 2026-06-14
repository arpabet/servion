/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package serviongrpc

import (
	"crypto/tls"
	"fmt"
	"reflect"
	"strings"

	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type implGrpcClientFactory struct {
	Log        *zap.Logger     `inject:""`
	Properties glue.Properties `inject:""`
	TlsConfig  *tls.Config     `inject:"optional"`

	beanName string
}

/*
GrpcClientFactory creates a *grpc.ClientConn bean named beanName.

The target address is taken from "<beanName>.connect-address". If that is empty
it is derived from the matching server bean by replacing "client" with "server"
in beanName and reading "<server>.bind-address" (a 0.0.0.0 or empty host is
rewritten to 127.0.0.1), so a client co-located with its server needs no extra
configuration.

Recognized properties (prefixed by beanName):

	<beanName>.connect-address     target host:port (overrides the derivation above)
	<beanName>.max-recv-msg-size   max inbound message size in bytes
	<beanName>.auth-token          bearer token sent as per-RPC credentials on every call

TLS is used when a *tls.Config bean is present in the context, otherwise the
connection is insecure (plaintext) which is the common case for in-cluster
traffic where TLS is terminated by the infrastructure.
*/
func GrpcClientFactory(beanName string) glue.FactoryBean {
	return &implGrpcClientFactory{beanName: beanName}
}

func (t *implGrpcClientFactory) Object() (object interface{}, err error) {

	defer servion.PanicToError(&err)

	connectAddr := t.Properties.GetString(fmt.Sprintf("%s.connect-address", t.beanName), "")
	if connectAddr == "" {
		serverBean := strings.ReplaceAll(t.beanName, "client", "server")
		bindAddr := t.Properties.GetString(fmt.Sprintf("%s.bind-address", serverBean), "")
		if bindAddr == "" {
			return nil, fmt.Errorf("neither property '%s.connect-address' nor '%s.bind-address' is found in context", t.beanName, serverBean)
		}
		connectAddr = localizeAddr(bindAddr)
	}

	t.Log.Info("GrpcClientFactory",
		zap.String("bean", t.beanName),
		zap.String("connectAddr", connectAddr),
		zap.Bool("tls", t.TlsConfig != nil))

	return t.dial(connectAddr)
}

func (t *implGrpcClientFactory) ObjectType() reflect.Type { return GrpcClientConnClass }

func (t *implGrpcClientFactory) ObjectName() string { return t.beanName }

func (t *implGrpcClientFactory) Singleton() bool { return true }

func (t *implGrpcClientFactory) transportCreds() credentials.TransportCredentials {
	if t.TlsConfig != nil {
		return credentials.NewTLS(t.TlsConfig)
	}
	return insecure.NewCredentials()
}

func (t *implGrpcClientFactory) dial(connectAddr string) (*grpc.ClientConn, error) {

	var opts []grpc.DialOption

	opts = append(opts, grpc.WithTransportCredentials(t.transportCreds()))

	if n := t.Properties.GetInt(fmt.Sprintf("%s.max-recv-msg-size", t.beanName), 0); n > 0 {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(n)))
	}

	if token := t.Properties.GetString(fmt.Sprintf("%s.auth-token", t.beanName), ""); token != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(tokenAuth{token: token, secure: t.TlsConfig != nil}))
	}

	// non-blocking; the connection is established lazily on first use so server
	// startup order does not matter.
	return grpc.Dial(connectAddr, opts...)
}

// localizeAddr turns a server bind address into a dialable client address by
// replacing a wildcard or empty host with the loopback interface.
func localizeAddr(bindAddr string) string {
	if strings.HasPrefix(bindAddr, "0.0.0.0:") {
		return "127.0.0.1" + bindAddr[len("0.0.0.0"):]
	}
	if strings.HasPrefix(bindAddr, ":") {
		return "127.0.0.1" + bindAddr
	}
	return bindAddr
}
