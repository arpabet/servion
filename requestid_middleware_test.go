package servion

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
	mw := RequestIDMiddleware(0)

	var capturedID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := RequestIDFromContext(r.Context())
		if !ok {
			t.Fatal("expected request ID in context")
		}
		capturedID = id
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Middleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedID == "" {
		t.Fatal("expected non-empty request ID")
	}
	if rec.Header().Get(HeaderXRequestID) != capturedID {
		t.Errorf("response header = %q, want %q", rec.Header().Get(HeaderXRequestID), capturedID)
	}
}

func TestRequestIDMiddleware_PreservesExistingID(t *testing.T) {
	mw := RequestIDMiddleware(0)

	var capturedID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := RequestIDFromContext(r.Context())
		capturedID = id
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Middleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set(HeaderXRequestID, "existing-id-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedID != "existing-id-123" {
		t.Errorf("captured ID = %q, want %q", capturedID, "existing-id-123")
	}
	if rec.Header().Get(HeaderXRequestID) != "existing-id-123" {
		t.Errorf("response header = %q, want %q", rec.Header().Get(HeaderXRequestID), "existing-id-123")
	}
}

func TestRequestIDMiddleware_UniquePerRequest(t *testing.T) {
	mw := RequestIDMiddleware(0)

	ids := make(map[string]bool)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := RequestIDFromContext(r.Context())
		ids[id] = true
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Middleware(inner)
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	if len(ids) != 100 {
		t.Errorf("expected 100 unique IDs, got %d", len(ids))
	}
}

func TestRequestIDMiddleware_Match(t *testing.T) {
	mw := &implRequestIDMiddleware{Prefixes: []string{"/api", "/web"}}

	tests := []struct {
		prefix string
		want   bool
	}{
		{"/api/v1", true},
		{"/web/page", true},
		{"/other", false},
		{"/", false},
	}

	for _, tt := range tests {
		if got := mw.Match(tt.prefix); got != tt.want {
			t.Errorf("Match(%q) = %v, want %v", tt.prefix, got, tt.want)
		}
	}
}

func TestRequestIDMiddleware_BeanOrder(t *testing.T) {
	mw := RequestIDMiddleware(5)
	if mw.BeanOrder() != 5 {
		t.Errorf("BeanOrder() = %d, want 5", mw.BeanOrder())
	}
}

func TestRequestIDFromContext_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, ok := RequestIDFromContext(req.Context())
	if ok {
		t.Error("expected ok=false for missing request ID")
	}
}
