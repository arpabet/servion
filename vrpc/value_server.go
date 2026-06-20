/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servionvrpc

import (
	"fmt"
	"net"
	"sync"

	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.arpabet.com/value-rpc/valueserver"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type implValueServer struct {
	Log        *zap.Logger       `inject:""`
	Properties glue.Properties   `inject:""`
	Services   []ValueService    `inject:"optional,level=1"`
	Authorizer ConnectAuthorizer `inject:"optional"`
	Obfs       ObfsProfile       `inject:"optional"`
	Transport  Transport         `inject:"optional"`

	beanName string

	srv valueserver.Server

	alive        atomic.Bool
	shutdownOnce sync.Once
	shutdownCh   chan struct{}
}

/*
ValueServer wraps a value-rpc server into a servion.Server so the standard
servion runtime binds, serves and shuts it down alongside HTTP and gRPC servers.
It is registered automatically by ValueServerScanner; you rarely construct it
directly.

Unlike the gRPC server (where *grpc.Server exists before binding), a value-rpc
server is created together with its listener, so the listener is opened, the
server is created and all ValueService beans are registered in Bind().

Recognized properties (prefixed by beanName):

	<beanName>.bind-address    listen address; bare "host:port" or ":port" is TCP,
	                           or a scheme: "tcp://", "unix:///path.sock", "ws://host/path"
	<beanName>.keep-alive      TCP keepalive period (default 15s; ignored for unix)
	<beanName>.write-timeout   per-message write timeout (default 10s)
*/
func ValueServer(beanName string) servion.Server {
	return &implValueServer{beanName: beanName, shutdownCh: make(chan struct{})}
}

func (t *implValueServer) PostConstruct() error {
	t.alive.Store(false)
	return nil
}

func (t *implValueServer) Bind() (err error) {

	defer servion.PanicToError(&err)

	listenAddr := t.Properties.GetString(fmt.Sprintf("%s.bind-address", t.beanName), "")
	if listenAddr == "" {
		return fmt.Errorf("property '%s.bind-address' not found in server context", t.beanName)
	}

	keepAlive := t.Properties.GetDuration(fmt.Sprintf("%s.keep-alive", t.beanName), valueserver.KeepAlivePeriod)
	writeTimeout := t.Properties.GetDuration(fmt.Sprintf("%s.write-timeout", t.beanName), valueserver.DefaultTimeout)

	var lis valuerpc.Listener
	switch {
	case t.Transport != nil:
		// The application fully supplies the transport (e.g. obfs/tlscamo or
		// obfs/reality composed in its own module); keep-alive is its concern.
		lis, err = t.Transport.Listener(listenAddr, writeTimeout)
	case t.Obfs != nil:
		// Obfuscation shapes the byte stream below value-rpc's framing; it needs a
		// stream transport and supersedes keep-alive (cover traffic keeps it live).
		lis, err = obfsListener(listenAddr, t.Obfs.ObfsPolicy(), writeTimeout)
	default:
		lis, err = valuerpc.NewListener(listenAddr, keepAlive, writeTimeout, valuerpc.MaxFrameSize)
	}
	if err != nil {
		return fmt.Errorf("can not bind to '%s': %w", listenAddr, err)
	}

	srv, err := valueserver.NewServerWithListener(lis, t.Log)
	if err != nil {
		lis.Close()
		return err
	}
	t.srv = srv

	if t.Authorizer != nil {
		srv.SetConnectAuthorizer(t.Authorizer.AuthorizeConnect)
	}

	for _, svc := range t.Services {
		if err := svc.RegisterFunctions(srv); err != nil {
			return fmt.Errorf("registering value service %T: %w", svc, err)
		}
	}

	t.Log.Info("ValueServerBind",
		zap.String("bean", t.beanName),
		zap.String("addr", srv.Addr().String()),
		zap.Int("services", len(t.Services)),
		zap.Bool("authorizer", t.Authorizer != nil),
		zap.Bool("obfs", t.Obfs != nil),
		zap.Bool("transport", t.Transport != nil))

	return nil
}

func (t *implValueServer) Alive() bool {
	return t.alive.Load()
}

func (t *implValueServer) ListenAddress() net.Addr {
	if t.srv != nil {
		return t.srv.Addr()
	}
	return servion.EmptyAddr
}

func (t *implValueServer) Serve() (err error) {

	defer servion.PanicToError(&err)

	if t.srv == nil {
		return fmt.Errorf("value server '%s' is not bound", t.beanName)
	}

	addr := t.ListenAddress()
	t.Log.Info("ValueServerServe",
		zap.String("bean", t.beanName),
		zap.String("addr", addr.String()),
		zap.String("network", addr.Network()))

	// Run blocks until Close; it already returns nil on graceful shutdown.
	t.alive.Store(true)
	err = t.srv.Run()
	t.alive.Store(false)
	return err
}

func (t *implValueServer) Shutdown() (err error) {

	t.shutdownOnce.Do(func() {

		addr := t.ListenAddress()
		t.Log.Info("ValueServerShutdown",
			zap.String("addr", addr.String()),
			zap.String("network", addr.Network()))

		// notify everyone that we are shutting down
		close(t.shutdownCh)

		if t.srv != nil {
			err = t.srv.Close()
		}
	})

	return
}

func (t *implValueServer) ShutdownCh() <-chan struct{} {
	return t.shutdownCh
}

func (t *implValueServer) Destroy() error {
	// safe to call twice
	t.Shutdown()
	return nil
}
