package servion

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestHttpServer_BindAndServe(t *testing.T) {
	srv := &http.Server{
		Addr: "127.0.0.1:0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok"))
		}),
	}

	s := NewHttpServer(srv)
	s.(*implHttpServer).Log = zap.NewNop()
	s.PostConstruct()

	if err := s.Bind(); err != nil {
		t.Fatalf("Bind: %v", err)
	}

	addr := s.ListenAddress()
	if addr == EmptyAddr {
		t.Fatal("expected non-empty listen address after bind")
	}

	// Serve in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Serve()
	}()

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	if !s.Alive() {
		t.Error("expected server to be alive after serve")
	}

	// Make request
	resp, err := http.Get(fmt.Sprintf("http://%s/", addr.String()))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("body = %q, want ok", string(body))
	}

	// Shutdown
	if err := s.Shutdown(); err != nil {
		t.Errorf("Shutdown: %v", err)
	}

	// Wait for Serve to return
	if err := <-errCh; err != nil {
		t.Errorf("Serve returned error: %v", err)
	}

	if s.Alive() {
		t.Error("expected server to not be alive after shutdown")
	}
}

func TestHttpServer_ShutdownCh(t *testing.T) {
	srv := &http.Server{Addr: "127.0.0.1:0"}
	s := NewHttpServer(srv)
	s.(*implHttpServer).Log = zap.NewNop()

	select {
	case <-s.ShutdownCh():
		t.Fatal("shutdown channel should not be closed yet")
	default:
	}

	s.Shutdown()

	select {
	case <-s.ShutdownCh():
		// expected
	case <-time.After(time.Second):
		t.Fatal("shutdown channel should be closed after shutdown")
	}
}

func TestHttpServer_DoubleShutdown(t *testing.T) {
	srv := &http.Server{Addr: "127.0.0.1:0"}
	s := NewHttpServer(srv)
	s.(*implHttpServer).Log = zap.NewNop()

	// Double shutdown should not panic
	s.Shutdown()
	s.Shutdown()
}

func TestHttpServer_Destroy(t *testing.T) {
	srv := &http.Server{Addr: "127.0.0.1:0"}
	s := NewHttpServer(srv)
	s.(*implHttpServer).Log = zap.NewNop()

	// Destroy calls Shutdown internally, should not panic
	if err := s.Destroy(); err != nil {
		t.Errorf("Destroy: %v", err)
	}
}

func TestHttpServer_ListenAddress_BeforeBind(t *testing.T) {
	srv := &http.Server{Addr: "127.0.0.1:0"}
	s := NewHttpServer(srv)

	if addr := s.ListenAddress(); addr != EmptyAddr {
		t.Errorf("ListenAddress before bind = %v, want EmptyAddr", addr)
	}
}

func TestHttpServer_BindFailure(t *testing.T) {
	// First, bind a port
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	// Try to bind the same port
	srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port)}
	s := NewHttpServer(srv)

	if err := s.Bind(); err == nil {
		t.Error("expected bind error on occupied port")
	}
}

func TestHttpServer_AliveBeforeServe(t *testing.T) {
	srv := &http.Server{Addr: "127.0.0.1:0"}
	s := NewHttpServer(srv)
	s.(*implHttpServer).Log = zap.NewNop()
	s.PostConstruct()

	if s.Alive() {
		t.Error("expected server to not be alive before serve")
	}
}
