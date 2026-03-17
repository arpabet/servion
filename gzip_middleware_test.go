package servion

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGzipMiddleware_CompressLargeResponse(t *testing.T) {
	mw := &implGzipMiddleware{
		beanOrder: 1,
		Level:     1,
		Threshold: 64,
	}

	body := strings.Repeat("hello world ", 100) // ~1200 bytes
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(hAcceptEncoding, "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Response should be gzip encoded
	ce := w.Header().Get(hContentEncoding)
	if ce != encGzip {
		t.Errorf("Content-Encoding = %q, want %q", ce, encGzip)
	}

	// Decompress and verify
	gr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close()

	got, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != body {
		t.Errorf("decompressed body length = %d, want %d", len(got), len(body))
	}
}

func TestGzipMiddleware_SkipSmallResponse(t *testing.T) {
	mw := &implGzipMiddleware{
		beanOrder: 1,
		Level:     1,
		Threshold: 1024,
	}

	body := "small"
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(hAcceptEncoding, "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// Should not be gzip encoded
	if ce := w.Header().Get(hContentEncoding); ce == encGzip {
		t.Error("small response should not be gzip encoded")
	}

	if w.Body.String() != body {
		t.Errorf("body = %q, want %q", w.Body.String(), body)
	}
}

func TestGzipMiddleware_SkipNoAcceptEncoding(t *testing.T) {
	mw := &implGzipMiddleware{
		beanOrder: 1,
		Level:     1,
		Threshold: 10,
	}

	body := strings.Repeat("x", 100)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Accept-Encoding header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if ce := w.Header().Get(hContentEncoding); ce == encGzip {
		t.Error("should not gzip without Accept-Encoding")
	}

	if w.Body.String() != body {
		t.Errorf("body = %q, want %q", w.Body.String(), body)
	}
}

func TestGzipMiddleware_SkipHeadRequest(t *testing.T) {
	mw := &implGzipMiddleware{
		beanOrder: 1,
		Level:     1,
		Threshold: 10,
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodHead, "/", nil)
	r.Header.Set(hAcceptEncoding, "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if ce := w.Header().Get(hContentEncoding); ce == encGzip {
		t.Error("should not gzip HEAD request")
	}
}

func TestGzipMiddleware_DecompressRequest(t *testing.T) {
	mw := &implGzipMiddleware{
		beanOrder: 1,
		Level:     1,
		Threshold: 1024,
	}

	originalBody := "compressed request body"

	// Create gzip-compressed body
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write([]byte(originalBody))
	gw.Close()

	var receivedBody string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		receivedBody = string(b)
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodPost, "/", &buf)
	r.Header.Set(hContentEncoding, encGzip)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if receivedBody != originalBody {
		t.Errorf("body = %q, want %q", receivedBody, originalBody)
	}
}

func TestGzipMiddleware_InvalidGzipRequest(t *testing.T) {
	mw := &implGzipMiddleware{
		beanOrder: 1,
		Level:     1,
		Threshold: 1024,
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for invalid gzip")
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not gzip data"))
	r.Header.Set(hContentEncoding, encGzip)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGzipMiddleware_Match(t *testing.T) {
	mw := &implGzipMiddleware{
		SkipPrefixes: []string{"/images", "/videos", "/ws"},
	}

	tests := []struct {
		pattern string
		want    bool
	}{
		{"/api/data", true},
		{"/images/logo.png", false},
		{"/videos/intro.mp4", false},
		{"/ws/connect", false},
		{"/health", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			if got := mw.Match(tt.pattern); got != tt.want {
				t.Errorf("Match(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestGzipMiddleware_BeanOrder(t *testing.T) {
	mw := GzipMiddleware(42)
	if got := mw.BeanOrder(); got != 42 {
		t.Errorf("BeanOrder() = %d, want 42", got)
	}
}

func TestGzipMiddleware_CloseWithoutWrite(t *testing.T) {
	mw := &implGzipMiddleware{
		beanOrder: 1,
		Level:     1,
		Threshold: 1024,
	}

	// Handler that sets status but writes nothing
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(hAcceptEncoding, "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestGzipMiddleware_ExactThreshold(t *testing.T) {
	threshold := 64
	mw := &implGzipMiddleware{
		beanOrder: 1,
		Level:     1,
		Threshold: threshold,
	}

	body := strings.Repeat("x", threshold) // exactly at threshold
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(hAcceptEncoding, "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// At threshold should trigger gzip
	if ce := w.Header().Get(hContentEncoding); ce != encGzip {
		t.Errorf("Content-Encoding = %q, want %q at exact threshold", ce, encGzip)
	}
}

func TestGzipMiddleware_MultipleWrites(t *testing.T) {
	mw := &implGzipMiddleware{
		beanOrder: 1,
		Level:     1,
		Threshold: 20,
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Multiple small writes that together exceed threshold
		for i := 0; i < 10; i++ {
			w.Write([]byte("hello "))
		}
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(hAcceptEncoding, "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if ce := w.Header().Get(hContentEncoding); ce != encGzip {
		t.Errorf("Content-Encoding = %q, want %q after multiple writes", ce, encGzip)
	}

	// Verify content
	gr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close()

	got, _ := io.ReadAll(gr)
	expected := strings.Repeat("hello ", 10)
	if string(got) != expected {
		t.Errorf("decompressed len = %d, want %d", len(got), len(expected))
	}
}

func TestAdaptiveGzipWriter_CloseNoGzip(t *testing.T) {
	w := httptest.NewRecorder()
	aw := &adaptiveGzipWriter{
		ResponseWriter: w,
		level:          1,
		minSize:        1024,
	}

	// Write small data then close — should flush plain
	aw.Write([]byte("hello"))
	aw.Close()

	if w.Body.String() != "hello" {
		t.Errorf("body = %q, want hello", w.Body.String())
	}
}

func TestAdaptiveGzipWriter_CloseEmpty(t *testing.T) {
	w := httptest.NewRecorder()
	aw := &adaptiveGzipWriter{
		ResponseWriter: w,
		level:          1,
		minSize:        1024,
	}

	// Close without writing anything
	err := aw.Close()
	if err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestAdaptiveGzipWriter_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	aw := &adaptiveGzipWriter{
		ResponseWriter: w,
		level:          1,
		minSize:        1024,
	}

	aw.WriteHeader(http.StatusCreated)
	// Second call should be ignored
	aw.WriteHeader(http.StatusOK)

	if aw.status != http.StatusCreated {
		t.Errorf("status = %d, want %d", aw.status, http.StatusCreated)
	}
}

func TestAdaptiveGzipWriter_WriteAfterGzipStarted(t *testing.T) {
	w := httptest.NewRecorder()
	aw := &adaptiveGzipWriter{
		ResponseWriter: w,
		level:          1,
		minSize:        10,
	}

	// Write enough to start gzip
	aw.Write([]byte(strings.Repeat("a", 20)))
	// Write more after gzip is started
	aw.Write([]byte("more data"))
	aw.Close()

	// Should be valid gzip
	gr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close()

	got, _ := io.ReadAll(gr)
	expected := strings.Repeat("a", 20) + "more data"
	if string(got) != expected {
		t.Errorf("decompressed = %q, want %q", string(got), expected)
	}
}

func TestGzipMiddleware_ResponseWithStatus(t *testing.T) {
	mw := &implGzipMiddleware{
		beanOrder: 1,
		Level:     1,
		Threshold: 10,
	}

	body := strings.Repeat("x", 100)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(body))
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set(hAcceptEncoding, "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}
