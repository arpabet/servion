/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package serviongrpc

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
)

const (
	gracefulShutdownTimeout = 2 * time.Second
	shutdownTimeout         = time.Second

	// alpnH2 is the ALPN protocol id gRPC requires when running over TLS (HTTP/2).
	alpnH2 = "h2"
)

type implGrpcServer struct {
	Container  glue.Container  `inject:""`
	Log        *zap.Logger     `inject:""`
	Properties glue.Properties `inject:""`
	TlsConfig  *tls.Config     `inject:"optional"`

	beanName   string
	listenAddr string

	srv      *grpc.Server
	listener net.Listener

	alive        atomic.Bool
	shutdownOnce sync.Once
	shutdownCh   chan struct{}
}

/*
GrpcServer wraps the *grpc.Server bean named beanName into a servion.Server, so
the standard servion runtime binds, serves and shuts it down alongside HTTP
servers. It is registered automatically by GrpcServerScanner; you rarely need to
construct it directly.
*/
func GrpcServer(beanName string) servion.Server {
	return &implGrpcServer{beanName: beanName, shutdownCh: make(chan struct{})}
}

func (t *implGrpcServer) PostConstruct() error {
	t.alive.Store(false)

	// Resolve the *grpc.Server produced by GrpcServerFactory in this same
	// container (level 1). Triggering its construction here also forces all
	// GrpcService beans to register before we start serving.
	for _, b := range t.Container.Bean(GrpcServerClass, 1) {
		if b.Name() == t.beanName {
			srv, ok := b.Object().(*grpc.Server)
			if !ok {
				return xerrors.Errorf("bean '%s' is not a *grpc.Server", t.beanName)
			}
			t.srv = srv
			return nil
		}
	}
	return xerrors.Errorf("grpc.Server bean '%s' not found in server context", t.beanName)
}

func (t *implGrpcServer) Bind() (err error) {

	t.listenAddr = t.Properties.GetString(fmt.Sprintf("%s.bind-address", t.beanName), "")
	if t.listenAddr == "" {
		return xerrors.Errorf("property '%s.bind-address' not found in server context", t.beanName)
	}

	t.listener, err = net.Listen("tcp", t.listenAddr)
	if err != nil {
		return xerrors.Errorf("can not bind to '%s': %w", t.listenAddr, err)
	}

	if t.TlsConfig != nil {
		t.listener = tls.NewListener(t.listener, ensureH2(t.TlsConfig.Clone()))
	}

	return nil
}

func (t *implGrpcServer) Alive() bool {
	return t.alive.Load()
}

func (t *implGrpcServer) ListenAddress() net.Addr {
	if t.listener != nil {
		return t.listener.Addr()
	}
	return servion.EmptyAddr
}

func (t *implGrpcServer) Shutdown() (err error) {

	t.shutdownOnce.Do(func() {

		addr := t.ListenAddress()
		t.Log.Info("GrpcServerShutdown",
			zap.String("addr", addr.String()),
			zap.String("network", addr.Network()))

		// notify everyone that we are shutting down
		close(t.shutdownCh)

		if !t.doGracefulStop() {
			t.doStop()
		}

		if t.listener != nil {
			t.listener.Close()
		}
	})

	return
}

func (t *implGrpcServer) doGracefulStop() bool {

	stopCh := make(chan struct{})
	go func() {
		t.srv.GracefulStop()
		close(stopCh)
	}()

	// wait a little bit for graceful shutdown of the gRPC server
	select {
	case <-stopCh:
		return true
	case <-time.After(gracefulShutdownTimeout):
		return false
	}
}

func (t *implGrpcServer) doStop() bool {

	stopCh := make(chan struct{})
	go func() {
		t.srv.Stop()
		close(stopCh)
	}()

	select {
	case <-stopCh:
		return true
	case <-time.After(shutdownTimeout):
		return false
	}
}

func (t *implGrpcServer) ShutdownCh() <-chan struct{} {
	return t.shutdownCh
}

func (t *implGrpcServer) Destroy() error {
	// safe to call twice
	t.Shutdown()
	return nil
}

func (t *implGrpcServer) Serve() (err error) {

	defer servion.PanicToError(&err)

	addr := t.ListenAddress()
	if t.TlsConfig != nil {
		t.Log.Info("GrpcServerServe",
			zap.String("addr", addr.String()),
			zap.String("network", addr.Network()),
			zap.Bool("tls", true),
			zap.Bool("insecure", t.TlsConfig.InsecureSkipVerify))
	} else {
		t.Log.Info("GrpcServerServe",
			zap.String("addr", addr.String()),
			zap.String("network", addr.Network()),
			zap.Bool("tls", false))
	}

	t.alive.Store(true)
	err = t.srv.Serve(t.listener)
	t.alive.Store(false)

	if err == nil || strings.Contains(err.Error(), "closed") {
		return nil
	}

	t.Log.Warn("GrpcServerClose", zap.Error(err))
	return err
}

// ensureH2 makes sure the TLS config advertises the HTTP/2 ALPN protocol, which
// gRPC requires when serving over TLS.
func ensureH2(cfg *tls.Config) *tls.Config {
	for _, p := range cfg.NextProtos {
		if p == alpnH2 {
			return cfg
		}
	}
	cfg.NextProtos = append(cfg.NextProtos, alpnH2)
	return cfg
}
