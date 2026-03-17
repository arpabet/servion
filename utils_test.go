package servion

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPanicToError_Recovers(t *testing.T) {
	var err error
	func() {
		defer PanicToError(&err)
		panic("boom")
	}()
	if err == nil {
		t.Fatal("expected error from panic")
	}
}

func TestPanicToError_NoPanic(t *testing.T) {
	var err error
	func() {
		defer PanicToError(&err)
	}()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestParseOptions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]bool
	}{
		{"empty", "", map[string]bool{}},
		{"single", "handlers", map[string]bool{"handlers": true}},
		{"multiple", "handlers;assets;tls", map[string]bool{"handlers": true, "assets": true, "tls": true}},
		{"whitespace", " handlers ; assets ", map[string]bool{"handlers": true, "assets": true}},
		{"trailing semicolon", "handlers;", map[string]bool{"handlers": true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseOptions(tt.input)
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("key %q: got %v, want %v", k, got[k], v)
				}
			}
			for k := range got {
				if _, ok := tt.want[k]; !ok {
					t.Errorf("unexpected key %q in result", k)
				}
			}
		})
	}
}

func TestAcceptsGzip(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   bool
	}{
		{"gzip only", "gzip", true},
		{"gzip with others", "deflate, gzip, br", true},
		{"no gzip", "deflate, br", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set(hAcceptEncoding, tt.header)
			if got := acceptsGzip(r); got != tt.want {
				t.Errorf("acceptsGzip(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}

func TestIsGzippedRequest(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   bool
	}{
		{"gzip", "gzip", true},
		{"empty", "", false},
		{"other", "deflate", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set(hContentEncoding, tt.header)
			if got := isGzippedRequest(r); got != tt.want {
				t.Errorf("isGzippedRequest(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}

func TestGzipWriter_SetsHeader(t *testing.T) {
	w := httptest.NewRecorder()
	gw := gzipWriter{w: w}
	gw.WriteHeader(200)
	if got := w.Header().Get(contentEncoding); got != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip", got)
	}
}

func TestGzipWriter_Write(t *testing.T) {
	w := httptest.NewRecorder()
	gw := gzipWriter{w: w}
	n, err := gw.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 5 {
		t.Errorf("Write returned %d, want 5", n)
	}
	if w.Body.String() != "hello" {
		t.Errorf("body = %q, want hello", w.Body.String())
	}
}

func TestGzipWriter_Header(t *testing.T) {
	w := httptest.NewRecorder()
	gw := gzipWriter{w: w}
	h := gw.Header()
	if h == nil {
		t.Fatal("expected non-nil header")
	}
	h.Set("X-Test", "value")
	if w.Header().Get("X-Test") != "value" {
		t.Error("expected header to pass through")
	}
}

func TestGzipWriter_NoHeaderOnError(t *testing.T) {
	w := httptest.NewRecorder()
	gw := gzipWriter{w: w}
	gw.WriteHeader(404)
	if got := w.Header().Get(contentEncoding); got == "gzip" {
		t.Error("should not set gzip Content-Encoding on non-200 status")
	}
}
