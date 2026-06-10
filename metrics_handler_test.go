package servion

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsHandler_Pattern(t *testing.T) {
	h := &implMetricsHandler{MetricsPattern: "/metrics"}
	if h.Pattern() != "/metrics" {
		t.Errorf("Pattern() = %q, want /metrics", h.Pattern())
	}
}

func TestMetricsHandler_ServesPrometheus(t *testing.T) {
	h := &implMetricsHandler{MetricsPattern: "/metrics"}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "go_") {
		t.Error("expected Prometheus Go runtime metrics in response")
	}
}

func TestMetricsMiddleware_RecordsMetrics(t *testing.T) {
	mw := &implMetricsMiddleware{
		Prefixes: []string{"/"},
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	handler := mw.Middleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMetricsMiddleware_CapturesStatusCode(t *testing.T) {
	mw := &implMetricsMiddleware{
		Prefixes: []string{"/"},
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := mw.Middleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestMetricsMiddleware_Match(t *testing.T) {
	mw := &implMetricsMiddleware{Prefixes: []string{"/api"}}

	if !mw.Match("/api/v1") {
		t.Error("expected Match(/api/v1) = true")
	}
	if mw.Match("/web") {
		t.Error("expected Match(/web) = false")
	}
}

func TestMetricsMiddleware_BeanOrder(t *testing.T) {
	mw := MetricsMiddleware(2)
	if mw.BeanOrder() != 2 {
		t.Errorf("BeanOrder() = %d, want 2", mw.BeanOrder())
	}
}
