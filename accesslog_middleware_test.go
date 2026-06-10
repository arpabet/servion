package servion

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestAccessLogMiddleware_LogsRequest(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	mw := &implAccessLogMiddleware{
		Log:      logger,
		Prefixes: []string{"/"},
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	})

	handler := mw.Middleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}

	entry := logs.All()[0]
	if entry.Message != "access" {
		t.Errorf("message = %q, want %q", entry.Message, "access")
	}

	fields := make(map[string]interface{})
	for _, f := range entry.Context {
		fields[f.Key] = f
	}

	if _, ok := fields["method"]; !ok {
		t.Error("missing 'method' field")
	}
	if _, ok := fields["path"]; !ok {
		t.Error("missing 'path' field")
	}
	if _, ok := fields["status"]; !ok {
		t.Error("missing 'status' field")
	}
	if _, ok := fields["duration"]; !ok {
		t.Error("missing 'duration' field")
	}
	if _, ok := fields["bytes"]; !ok {
		t.Error("missing 'bytes' field")
	}
	if _, ok := fields["userAgent"]; !ok {
		t.Error("missing 'userAgent' field")
	}
}

func TestAccessLogMiddleware_CapturesStatus(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	mw := &implAccessLogMiddleware{
		Log:      logger,
		Prefixes: []string{"/"},
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := mw.Middleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}

	for _, f := range logs.All()[0].Context {
		if f.Key == "status" && f.Integer != http.StatusNotFound {
			t.Errorf("status = %d, want %d", f.Integer, http.StatusNotFound)
		}
	}
}

func TestAccessLogMiddleware_IncludesRequestID(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	ridMw := RequestIDMiddleware(0)
	logMw := &implAccessLogMiddleware{
		Log:      logger,
		Prefixes: []string{"/"},
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Chain: requestID -> accessLog -> handler
	handler := ridMw.Middleware(logMw.Middleware(inner))
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}

	var hasRequestID bool
	for _, f := range logs.All()[0].Context {
		if f.Key == "requestId" {
			hasRequestID = true
		}
	}
	if !hasRequestID {
		t.Error("expected 'requestId' field in log entry")
	}
}

func TestAccessLogMiddleware_Match(t *testing.T) {
	mw := &implAccessLogMiddleware{Prefixes: []string{"/api"}}

	if !mw.Match("/api/v1") {
		t.Error("expected Match(/api/v1) = true")
	}
	if mw.Match("/web") {
		t.Error("expected Match(/web) = false")
	}
}

func TestAccessLogMiddleware_BeanOrder(t *testing.T) {
	mw := AccessLogMiddleware(3)
	if mw.BeanOrder() != 3 {
		t.Errorf("BeanOrder() = %d, want 3", mw.BeanOrder())
	}
}

func TestStatusWriter_DefaultStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, status: http.StatusOK}

	sw.Write([]byte("test"))

	if sw.status != http.StatusOK {
		t.Errorf("status = %d, want %d", sw.status, http.StatusOK)
	}
	if sw.written != 4 {
		t.Errorf("written = %d, want 4", sw.written)
	}
}
