package servion

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCorsMiddleware_PreflightRequest(t *testing.T) {
	mw := &implCorsMiddleware{
		Prefixes:     []string{"/"},
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST"},
		AllowHeaders: []string{"Authorization", "Content-Type"},
		MaxAge:       "3600",
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called for preflight")
	})

	handler := mw.Middleware(inner)
	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("Allow-Origin = %q, want %q", got, "https://example.com")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST" {
		t.Errorf("Allow-Methods = %q, want %q", got, "GET, POST")
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "3600" {
		t.Errorf("Max-Age = %q, want %q", got, "3600")
	}
}

func TestCorsMiddleware_SimpleRequest(t *testing.T) {
	mw := &implCorsMiddleware{
		Prefixes:      []string{"/"},
		AllowOrigins:  []string{"*"},
		AllowMethods:  []string{"GET"},
		ExposeHeaders: []string{"X-Request-ID"},
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Middleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("Allow-Origin = %q, want %q", got, "https://example.com")
	}
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "X-Request-ID" {
		t.Errorf("Expose-Headers = %q, want %q", got, "X-Request-ID")
	}
}

func TestCorsMiddleware_NoOrigin(t *testing.T) {
	mw := &implCorsMiddleware{
		Prefixes:     []string{"/"},
		AllowOrigins: []string{"*"},
	}

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Middleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("inner handler should be called")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin should be empty, got %q", got)
	}
}

func TestCorsMiddleware_DisallowedOrigin(t *testing.T) {
	mw := &implCorsMiddleware{
		Prefixes:     []string{"/"},
		AllowOrigins: []string{"https://allowed.com"},
	}

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Middleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("inner handler should still be called")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin should be empty for disallowed origin, got %q", got)
	}
}

func TestCorsMiddleware_AllowCredentials(t *testing.T) {
	mw := &implCorsMiddleware{
		Prefixes:     []string{"/"},
		AllowOrigins: []string{"https://example.com"},
		AllowCreds:   true,
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Middleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q, want %q", got, "true")
	}
}

func TestCorsMiddleware_Match(t *testing.T) {
	mw := &implCorsMiddleware{Prefixes: []string{"/api"}}

	if !mw.Match("/api/v1") {
		t.Error("expected Match(/api/v1) = true")
	}
	if mw.Match("/web") {
		t.Error("expected Match(/web) = false")
	}
}

func TestCorsMiddleware_BeanOrder(t *testing.T) {
	mw := CorsMiddleware(1)
	if mw.BeanOrder() != 1 {
		t.Errorf("BeanOrder() = %d, want 1", mw.BeanOrder())
	}
}
