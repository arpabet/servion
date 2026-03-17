package servion

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.arpabet.com/glue"
	"go.uber.org/zap"
)

func TestHttpServerFactory_MissingBindAddress(t *testing.T) {
	props := glue.NewProperties()

	f := &implHttpServerFactory{
		Log:        zap.NewNop(),
		Properties: props,
		beanName:   "test-server",
	}

	_, err := f.Object()
	if err == nil {
		t.Fatal("expected error when bind-address is missing")
	}
}

func TestHttpServerFactory_CreatesServer(t *testing.T) {
	props := glue.NewProperties()
	props.Set("test-server.bind-address", "127.0.0.1:0")
	props.Set("test-server.options", "")

	f := &implHttpServerFactory{
		Log:        zap.NewNop(),
		Properties: props,
		beanName:   "test-server",
	}

	obj, err := f.Object()
	if err != nil {
		t.Fatalf("Object: %v", err)
	}

	srv, ok := obj.(*http.Server)
	if !ok {
		t.Fatalf("expected *http.Server, got %T", obj)
	}

	if srv.Addr != "127.0.0.1:0" {
		t.Errorf("Addr = %q, want 127.0.0.1:0", srv.Addr)
	}
}

func TestHttpServerFactory_CustomTimeouts(t *testing.T) {
	props := glue.NewProperties()
	props.Set("test-server.bind-address", "127.0.0.1:0")
	props.Set("test-server.read-timeout", "10s")
	props.Set("test-server.write-timeout", "20s")
	props.Set("test-server.idle-timeout", "30s")

	f := &implHttpServerFactory{
		Log:        zap.NewNop(),
		Properties: props,
		beanName:   "test-server",
	}

	obj, err := f.Object()
	if err != nil {
		t.Fatalf("Object: %v", err)
	}

	srv := obj.(*http.Server)

	if srv.ReadTimeout != 10*time.Second {
		t.Errorf("ReadTimeout = %v, want 10s", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 20*time.Second {
		t.Errorf("WriteTimeout = %v, want 20s", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 30*time.Second {
		t.Errorf("IdleTimeout = %v, want 30s", srv.IdleTimeout)
	}
}

func TestHttpServerFactory_DefaultTimeouts(t *testing.T) {
	props := glue.NewProperties()
	props.Set("test-server.bind-address", "127.0.0.1:0")

	f := &implHttpServerFactory{
		Log:        zap.NewNop(),
		Properties: props,
		beanName:   "test-server",
	}

	obj, err := f.Object()
	if err != nil {
		t.Fatalf("Object: %v", err)
	}

	srv := obj.(*http.Server)

	if srv.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v, want 30s", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 30*time.Second {
		t.Errorf("WriteTimeout = %v, want 30s", srv.WriteTimeout)
	}
	if srv.IdleTimeout != time.Minute {
		t.Errorf("IdleTimeout = %v, want 1m", srv.IdleTimeout)
	}
}

type testHandler struct {
	pattern string
}

func (h *testHandler) Pattern() string { return h.pattern }
func (h *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}

type testMiddleware struct {
	order    int
	prefixes []string
	called   bool
}

func (m *testMiddleware) BeanOrder() int { return m.order }
func (m *testMiddleware) Match(pattern string) bool {
	for _, p := range m.prefixes {
		if len(pattern) >= len(p) && pattern[:len(p)] == p {
			return true
		}
	}
	return false
}
func (m *testMiddleware) Middleware(next http.Handler) http.Handler {
	m.called = true
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test-Middleware", "applied")
		next.ServeHTTP(w, r)
	})
}

func TestHttpServerFactory_WithHandlers(t *testing.T) {
	props := glue.NewProperties()
	props.Set("test-server.bind-address", "127.0.0.1:0")
	props.Set("test-server.options", "handlers")

	h := &testHandler{pattern: "/api/test"}

	f := &implHttpServerFactory{
		Log:        zap.NewNop(),
		Properties: props,
		Handlers:   []HttpHandler{h},
		beanName:   "test-server",
	}

	obj, err := f.Object()
	if err != nil {
		t.Fatalf("Object: %v", err)
	}

	if obj == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestHttpServerFactory_PostConstruct_SortsMiddleware(t *testing.T) {
	mw1 := &testMiddleware{order: 10, prefixes: []string{"/api"}}
	mw2 := &testMiddleware{order: 1, prefixes: []string{"/api"}}
	mw3 := &testMiddleware{order: 5, prefixes: []string{"/api"}}

	f := &implHttpServerFactory{
		Middlewares: []HttpMiddleware{mw1, mw2, mw3},
	}

	f.PostConstruct()

	if f.Middlewares[0].BeanOrder() != 1 {
		t.Errorf("first middleware order = %d, want 1", f.Middlewares[0].BeanOrder())
	}
	if f.Middlewares[1].BeanOrder() != 5 {
		t.Errorf("second middleware order = %d, want 5", f.Middlewares[1].BeanOrder())
	}
	if f.Middlewares[2].BeanOrder() != 10 {
		t.Errorf("third middleware order = %d, want 10", f.Middlewares[2].BeanOrder())
	}
}

func TestHttpServerFactory_ObjectType(t *testing.T) {
	f := HttpServerFactory("test")
	fb := f.(*implHttpServerFactory)
	if fb.ObjectType() != HttpServerClass {
		t.Errorf("ObjectType = %v, want %v", fb.ObjectType(), HttpServerClass)
	}
}

func TestHttpServerFactory_ObjectName(t *testing.T) {
	f := HttpServerFactory("my-server")
	fb := f.(*implHttpServerFactory)
	if fb.ObjectName() != "my-server" {
		t.Errorf("ObjectName = %q, want my-server", fb.ObjectName())
	}
}

func TestHttpServerFactory_Singleton(t *testing.T) {
	f := HttpServerFactory("test")
	fb := f.(*implHttpServerFactory)
	if !fb.Singleton() {
		t.Error("expected Singleton() to return true")
	}
}

func TestHttpServerFactory_WithMiddleware(t *testing.T) {
	props := glue.NewProperties()
	props.Set("test-server.bind-address", "127.0.0.1:0")
	props.Set("test-server.options", "handlers")

	h := &testHandler{pattern: "/api/test"}
	mw := &testMiddleware{order: 1, prefixes: []string{"/api"}}

	f := &implHttpServerFactory{
		Log:         zap.NewNop(),
		Properties:  props,
		Handlers:    []HttpHandler{h},
		Middlewares: []HttpMiddleware{mw},
		beanName:    "test-server",
	}
	f.PostConstruct()

	_, err := f.Object()
	if err != nil {
		t.Fatalf("Object: %v", err)
	}

	if !mw.called {
		t.Error("expected middleware to be applied to /api handler")
	}
}

func TestHttpServerFactory_MiddlewareNotMatchingSkipped(t *testing.T) {
	props := glue.NewProperties()
	props.Set("test-server.bind-address", "127.0.0.1:0")
	props.Set("test-server.options", "handlers")

	h := &testHandler{pattern: "/public/page"}
	mw := &testMiddleware{order: 1, prefixes: []string{"/api"}}

	f := &implHttpServerFactory{
		Log:         zap.NewNop(),
		Properties:  props,
		Handlers:    []HttpHandler{h},
		Middlewares: []HttpMiddleware{mw},
		beanName:    "test-server",
	}

	_, err := f.Object()
	if err != nil {
		t.Fatalf("Object: %v", err)
	}

	if mw.called {
		t.Error("middleware should not be applied to /public handler")
	}
}

func TestHttpServerFactory_IsEnabled(t *testing.T) {
	props := glue.NewProperties()
	props.Set("srv.tls", "true")
	props.Set("srv.gzip", "false")

	f := &implHttpServerFactory{
		Properties: props,
		beanName:   "srv",
	}

	if !f.isEnabled("tls") {
		t.Error("expected tls to be enabled")
	}
	if f.isEnabled("gzip") {
		t.Error("expected gzip to be disabled")
	}
	if f.isEnabled("nonexistent") {
		t.Error("expected nonexistent to be disabled")
	}
}

func TestServingAsset_ServeHTTP_Plain(t *testing.T) {
	plainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("plain content"))
	})

	sa := &servingAsset{
		pattern: "/test.html",
		plainH:  plainHandler,
	}

	r := httptest.NewRequest(http.MethodGet, "/test.html", nil)
	w := httptest.NewRecorder()
	sa.ServeHTTP(w, r)

	if w.Body.String() != "plain content" {
		t.Errorf("body = %q, want plain content", w.Body.String())
	}
}

func TestServingAsset_ServeHTTP_Gzip(t *testing.T) {
	gzipHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("gzip content"))
	})
	plainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("plain content"))
	})

	sa := &servingAsset{
		pattern: "/test.html",
		gzipH:   gzipHandler,
		plainH:  plainHandler,
	}

	r := httptest.NewRequest(http.MethodGet, "/test.html", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	sa.ServeHTTP(w, r)

	if w.Body.String() != "gzip content" {
		t.Errorf("body = %q, want gzip content", w.Body.String())
	}
}

func TestServingAsset_ServeHTTP_NoHandler(t *testing.T) {
	sa := &servingAsset{pattern: "/missing.html"}

	r := httptest.NewRequest(http.MethodGet, "/missing.html", nil)
	w := httptest.NewRecorder()
	sa.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestServingAsset_AcceptGzip(t *testing.T) {
	sa := &servingAsset{}

	tests := []struct {
		header string
		want   bool
	}{
		{"gzip", true},
		{"deflate, gzip, br", true},
		{"deflate, br", false},
		{"", false},
	}

	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Accept-Encoding", tt.header)
		if got := sa.acceptGzip(r); got != tt.want {
			t.Errorf("acceptGzip(%q) = %v, want %v", tt.header, got, tt.want)
		}
	}
}

func TestGzipHeaderHandler_ServeHTTP(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data"))
	})

	handler := gzipHeaderHandler{h: inner}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if ce := w.Header().Get("Content-Encoding"); ce != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip", ce)
	}
	if w.Body.String() != "data" {
		t.Errorf("body = %q, want data", w.Body.String())
	}
}

func TestHttpServerFactory_GroupAssets_Empty(t *testing.T) {
	f := &implHttpServerFactory{
		Log: zap.NewNop(),
	}

	result := f.groupAssets()
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestHttpServerFactory_GroupAssets_Plain(t *testing.T) {
	f := &implHttpServerFactory{
		Log: zap.NewNop(),
		Resources: []*glue.ResourceSource{
			{
				Name:       "assets",
				AssetNames: []string{"style.css", "app.js"},
				AssetFiles: http.Dir(t.TempDir()),
			},
		},
	}

	result := f.groupAssets()
	if _, ok := result["/style.css"]; !ok {
		t.Error("expected /style.css in grouped assets")
	}
	if _, ok := result["/app.js"]; !ok {
		t.Error("expected /app.js in grouped assets")
	}
}

func TestHttpServerFactory_GroupAssets_Gzip(t *testing.T) {
	f := &implHttpServerFactory{
		Log: zap.NewNop(),
		Resources: []*glue.ResourceSource{
			{
				Name:       "assets-gzip",
				AssetNames: []string{"style.css"},
				AssetFiles: http.Dir(t.TempDir()),
			},
		},
	}

	result := f.groupAssets()
	sa, ok := result["/style.css"]
	if !ok {
		t.Fatal("expected /style.css in grouped assets")
	}
	if sa.gzipH == nil {
		t.Error("expected gzip handler to be set")
	}
}

func TestHttpServerFactory_GroupAssets_IndexHTML(t *testing.T) {
	f := &implHttpServerFactory{
		Log: zap.NewNop(),
		Resources: []*glue.ResourceSource{
			{
				Name:       "assets",
				AssetNames: []string{"index.html"},
				AssetFiles: http.Dir(t.TempDir()),
			},
		},
	}

	result := f.groupAssets()
	// index.html should create both /index.html and / patterns
	if _, ok := result["/index.html"]; !ok {
		t.Error("expected /index.html in grouped assets")
	}
	if _, ok := result["/"]; !ok {
		t.Error("expected / (root) redirect from index.html")
	}
}

func TestHttpServerFactory_GroupAssets_NonAssetSkipped(t *testing.T) {
	f := &implHttpServerFactory{
		Log: zap.NewNop(),
		Resources: []*glue.ResourceSource{
			{
				Name:       "templates", // not "assets" prefix
				AssetNames: []string{"page.html"},
				AssetFiles: http.Dir(t.TempDir()),
			},
		},
	}

	result := f.groupAssets()
	if len(result) != 0 {
		t.Errorf("expected no grouped assets for non-asset resource, got %d", len(result))
	}
}

func TestHttpServerFactory_GroupAssets_PlainAndGzip(t *testing.T) {
	f := &implHttpServerFactory{
		Log: zap.NewNop(),
		Resources: []*glue.ResourceSource{
			{
				Name:       "assets",
				AssetNames: []string{"app.js"},
				AssetFiles: http.Dir(t.TempDir()),
			},
			{
				Name:       "assets-gzip",
				AssetNames: []string{"app.js"},
				AssetFiles: http.Dir(t.TempDir()),
			},
		},
	}

	result := f.groupAssets()
	sa, ok := result["/app.js"]
	if !ok {
		t.Fatal("expected /app.js in grouped assets")
	}
	if sa.plainH == nil {
		t.Error("expected plain handler")
	}
	if sa.gzipH == nil {
		t.Error("expected gzip handler")
	}
}

func TestHttpServerFactory_WithAssets(t *testing.T) {
	props := glue.NewProperties()
	props.Set("test-server.bind-address", "127.0.0.1:0")
	props.Set("test-server.options", "assets")

	f := &implHttpServerFactory{
		Log:        zap.NewNop(),
		Properties: props,
		beanName:   "test-server",
		Resources: []*glue.ResourceSource{
			{
				Name:       "assets",
				AssetNames: []string{"style.css"},
				AssetFiles: http.Dir(t.TempDir()),
			},
		},
	}

	obj, err := f.Object()
	if err != nil {
		t.Fatalf("Object: %v", err)
	}

	if obj == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestHttpServerFactory_HandlersDisabled(t *testing.T) {
	props := glue.NewProperties()
	props.Set("test-server.bind-address", "127.0.0.1:0")
	// options does not include "handlers"

	h := &testHandler{pattern: "/api/test"}

	f := &implHttpServerFactory{
		Log:        zap.NewNop(),
		Properties: props,
		Handlers:   []HttpHandler{h},
		beanName:   "test-server",
	}

	obj, err := f.Object()
	if err != nil {
		t.Fatalf("Object: %v", err)
	}

	// Server should still be created
	if obj == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestHttpServerFactory_TLSWithConfig(t *testing.T) {
	props := glue.NewProperties()
	props.Set("test-server.bind-address", "127.0.0.1:0")
	props.Set("test-server.options", "tls")

	tlsCfg := &tls.Config{
		InsecureSkipVerify: true,
	}

	f := &implHttpServerFactory{
		Log:        zap.NewNop(),
		Properties: props,
		TlsConfig:  tlsCfg,
		beanName:   "test-server",
	}

	obj, err := f.Object()
	if err != nil {
		t.Fatalf("Object: %v", err)
	}

	srv := obj.(*http.Server)
	if srv.TLSConfig == nil {
		t.Fatal("expected non-nil TLSConfig")
	}
	// Should be a clone, not the same pointer
	if srv.TLSConfig == tlsCfg {
		t.Error("expected TLSConfig to be cloned, not the same pointer")
	}
}

func TestHttpServerFactory_TLSWarning(t *testing.T) {
	props := glue.NewProperties()
	props.Set("test-server.bind-address", "127.0.0.1:0")
	props.Set("test-server.options", "tls")

	f := &implHttpServerFactory{
		Log:        zap.NewNop(),
		Properties: props,
		beanName:   "test-server",
		// TlsConfig is nil — should log warning
	}

	obj, err := f.Object()
	if err != nil {
		t.Fatalf("Object: %v", err)
	}

	srv := obj.(*http.Server)
	if srv.TLSConfig != nil {
		t.Error("expected nil TLSConfig when no TlsConfig bean provided")
	}
}
