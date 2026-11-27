package servion

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"go.arpabet.com/glue"
	"go.uber.org/zap"
)

// fakeClock is advanced by hand so window and expiry tests never sleep.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newFakeClock() *fakeClock { return &fakeClock{t: time.Unix(1_700_000_000, 0)} }

func newTestFail2Ban(clock *fakeClock) *implFail2BanMiddleware {
	f := Fail2BanMiddleware(1).(*implFail2BanMiddleware)
	f.Log = zap.NewNop()
	f.Header = "CF-Connecting-IP"
	f.MaxRetry = 5
	f.FindTime = time.Minute
	f.BanTime = time.Hour
	f.IgnorePaths = []string{"/favicon.ico", "/assets/"}
	f.now = clock.now
	return f
}

// fail2banSite answers 200 on "/" and 404 on every other path.
func fail2banSite() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Write([]byte("home"))
			return
		}
		http.NotFound(w, r)
	})
}

func fail2banGet(h http.Handler, path, clientIP string) int {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if clientIP != "" {
		r.Header.Set("CF-Connecting-IP", clientIP)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

func TestFail2Ban_BansAfterMaxRetry(t *testing.T) {
	clock := newFakeClock()
	h := newTestFail2Ban(clock).RouterMiddleware(fail2banSite())

	for i := 0; i < 4; i++ {
		fail2banGet(h, "/.env", "203.0.113.7")
		clock.advance(time.Second)
	}
	if code := fail2banGet(h, "/", "203.0.113.7"); code != http.StatusOK {
		t.Fatalf("after 4 strikes: status = %d, want 200", code)
	}

	fail2banGet(h, "/wp-login.php", "203.0.113.7")
	if code := fail2banGet(h, "/", "203.0.113.7"); code != http.StatusNotFound {
		t.Fatalf("after 5 strikes: status = %d, want 404 (banned)", code)
	}
}

func TestFail2Ban_OtherClientsUnaffected(t *testing.T) {
	h := newTestFail2Ban(newFakeClock()).RouterMiddleware(fail2banSite())

	for i := 0; i < 5; i++ {
		fail2banGet(h, "/.env", "203.0.113.7")
	}
	if code := fail2banGet(h, "/", "198.51.100.1"); code != http.StatusOK {
		t.Fatalf("other client: status = %d, want 200", code)
	}
}

func TestFail2Ban_StrikesOutsideFindTimeExpire(t *testing.T) {
	clock := newFakeClock()
	h := newTestFail2Ban(clock).RouterMiddleware(fail2banSite())

	// Strikes at 0s, 15s, 30s, 45s and 60s: the first leaves the 1m window
	// as the fifth lands, so only four count.
	for i := 0; i < 5; i++ {
		if i > 0 {
			clock.advance(15 * time.Second)
		}
		fail2banGet(h, "/.env", "203.0.113.7")
	}
	if code := fail2banGet(h, "/", "203.0.113.7"); code != http.StatusOK {
		t.Fatalf("strikes spread over more than findtime: status = %d, want 200", code)
	}

	// At 61s the strikes from 15s to 61s are five within the window.
	clock.advance(time.Second)
	fail2banGet(h, "/.env", "203.0.113.7")
	if code := fail2banGet(h, "/", "203.0.113.7"); code != http.StatusNotFound {
		t.Fatalf("five strikes within findtime: status = %d, want 404 (banned)", code)
	}
}

func TestFail2Ban_BanExpires(t *testing.T) {
	clock := newFakeClock()
	h := newTestFail2Ban(clock).RouterMiddleware(fail2banSite())

	for i := 0; i < 5; i++ {
		fail2banGet(h, "/.env", "203.0.113.7")
	}
	clock.advance(time.Hour - time.Second)
	if code := fail2banGet(h, "/", "203.0.113.7"); code != http.StatusNotFound {
		t.Fatalf("before bantime ends: status = %d, want 404 (banned)", code)
	}
	clock.advance(time.Second)
	if code := fail2banGet(h, "/", "203.0.113.7"); code != http.StatusOK {
		t.Fatalf("after bantime: status = %d, want 200", code)
	}
}

func TestFail2Ban_IgnoredPathsDontCount(t *testing.T) {
	h := newTestFail2Ban(newFakeClock()).RouterMiddleware(fail2banSite())

	for i := 0; i < 10; i++ {
		fail2banGet(h, "/favicon.ico", "203.0.113.7")
		fail2banGet(h, "/assets/index-0ldHash.js", "203.0.113.7")
	}
	if code := fail2banGet(h, "/", "203.0.113.7"); code != http.StatusOK {
		t.Fatalf("404s on ignored paths: status = %d, want 200", code)
	}
}

func TestFail2Ban_OnlyNotFoundCounts(t *testing.T) {
	h := newTestFail2Ban(newFakeClock()).RouterMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/forbidden":
			http.Error(w, "forbidden", http.StatusForbidden)
		case "/broken":
			http.Error(w, "broken", http.StatusInternalServerError)
		default:
			w.Write([]byte("ok"))
		}
	}))

	for i := 0; i < 10; i++ {
		fail2banGet(h, "/forbidden", "203.0.113.7")
		fail2banGet(h, "/broken", "203.0.113.7")
	}
	if code := fail2banGet(h, "/", "203.0.113.7"); code != http.StatusOK {
		t.Fatalf("403s and 500s: status = %d, want 200", code)
	}
}

func TestFail2Ban_UnidentifiedClientsPassThrough(t *testing.T) {
	f := newTestFail2Ban(newFakeClock())
	h := f.RouterMiddleware(fail2banSite())

	for _, ip := range []string{"", "not-an-ip"} {
		for i := 0; i < 10; i++ {
			if code := fail2banGet(h, "/.env", ip); code != http.StatusNotFound {
				t.Fatalf("header %q: status = %d, want the site's own 404", ip, code)
			}
		}
		if code := fail2banGet(h, "/", ip); code != http.StatusOK {
			t.Errorf("header %q: status = %d, want 200 (never banned)", ip, code)
		}
	}
	if len(f.clients) != 0 {
		t.Errorf("tracked %d clients, want 0", len(f.clients))
	}
}

func TestFail2Ban_ClientKey(t *testing.T) {
	f := newTestFail2Ban(newFakeClock())

	cases := []struct {
		header string
		want   string
		ok     bool
	}{
		{"203.0.113.7", "203.0.113.7", true},
		{" 203.0.113.7 ", "203.0.113.7", true},
		{"203.0.113.7, 10.0.0.1", "203.0.113.7", true},
		{"::ffff:203.0.113.7", "203.0.113.7", true},
		{"2001:db8:1:2:3:4:5:6", "2001:db8:1:2::/64", true},
		{"", "", false},
		{"not-an-ip", "", false},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if c.header != "" {
			r.Header.Set("CF-Connecting-IP", c.header)
		}
		got, ok := f.clientKey(r)
		if got != c.want || ok != c.ok {
			t.Errorf("clientKey(%q) = (%q, %v), want (%q, %v)", c.header, got, ok, c.want, c.ok)
		}
	}
}

func TestFail2Ban_IPv6PrefixSharesBan(t *testing.T) {
	h := newTestFail2Ban(newFakeClock()).RouterMiddleware(fail2banSite())

	for i := 1; i <= 5; i++ {
		fail2banGet(h, "/.env", fmt.Sprintf("2001:db8:1:2::%d", i))
	}
	if code := fail2banGet(h, "/", "2001:db8:1:2::99"); code != http.StatusNotFound {
		t.Errorf("same /64: status = %d, want 404 (banned)", code)
	}
	if code := fail2banGet(h, "/", "2001:db8:1:3::1"); code != http.StatusOK {
		t.Errorf("other /64: status = %d, want 200", code)
	}
}

func TestFail2Ban_CleanerForgetsIdleClients(t *testing.T) {
	clock := newFakeClock()
	f := newTestFail2Ban(clock)
	h := f.RouterMiddleware(fail2banSite())

	fail2banGet(h, "/.env", "198.51.100.1") // one strike, never banned
	for i := 0; i < 5; i++ {
		fail2banGet(h, "/.env", "203.0.113.7")
	}

	clock.advance(2 * time.Minute)
	f.doCleanExpired()
	if _, ok := f.clients["198.51.100.1"]; ok {
		t.Error("client with only expired strikes was kept")
	}
	if _, ok := f.clients["203.0.113.7"]; !ok {
		t.Error("banned client was dropped before its ban expired")
	}

	clock.advance(time.Hour)
	f.doCleanExpired()
	if len(f.clients) != 0 {
		t.Errorf("tracked %d clients after every ban expired, want 0", len(f.clients))
	}
}

func TestFail2Ban_TrackingIsBounded(t *testing.T) {
	f := newTestFail2Ban(newFakeClock())
	f.maxTracked = 2
	h := f.RouterMiddleware(fail2banSite())

	fail2banGet(h, "/.env", "198.51.100.1")
	fail2banGet(h, "/.env", "198.51.100.2")
	for i := 0; i < 5; i++ {
		fail2banGet(h, "/.env", "198.51.100.3")
	}
	if len(f.clients) != 2 {
		t.Errorf("tracked %d clients, want the cap of 2", len(f.clients))
	}
	if code := fail2banGet(h, "/", "198.51.100.3"); code != http.StatusOK {
		t.Errorf("untracked client: status = %d, want 200", code)
	}
}

func TestFail2Ban_PostConstructRejectsBadConfig(t *testing.T) {
	for name, breakConfig := range map[string]func(*implFail2BanMiddleware){
		"maxretry": func(f *implFail2BanMiddleware) { f.MaxRetry = 0 },
		"findtime": func(f *implFail2BanMiddleware) { f.FindTime = 0 },
		"bantime":  func(f *implFail2BanMiddleware) { f.BanTime = 0 },
	} {
		f := newTestFail2Ban(newFakeClock())
		f.Runtime = newMockRuntime(true)
		breakConfig(f)
		if err := f.PostConstruct(); err == nil {
			f.Destroy()
			t.Errorf("%s: PostConstruct accepted an invalid value", name)
		}
	}
}

func TestFail2Ban_DefaultsFromContainer(t *testing.T) {
	ctx, err := glue.New(zap.NewNop(), newMockRuntime(true), Fail2BanMiddleware(1))
	if err != nil {
		t.Fatalf("glue.New: %v", err)
	}
	defer ctx.Close()

	beans := ctx.Bean(HttpRouterMiddlewareClass, glue.DefaultSearchLevel)
	if len(beans) != 1 {
		t.Fatalf("found %d router middlewares, want 1", len(beans))
	}
	f := beans[0].Object().(*implFail2BanMiddleware)

	if f.Header != "CF-Connecting-IP" || f.MaxRetry != 5 || f.FindTime != time.Minute || f.BanTime != time.Hour {
		t.Errorf("defaults = header %q, maxretry %d, findtime %s, bantime %s; want CF-Connecting-IP, 5, 1m, 1h",
			f.Header, f.MaxRetry, f.FindTime, f.BanTime)
	}
	wantIgnore := []string{"/favicon.ico", "/apple-touch-icon", "/robots.txt", "/sitemap.xml", "/.well-known/", "/assets/"}
	if !reflect.DeepEqual(f.IgnorePaths, wantIgnore) {
		t.Errorf("ignore-paths = %q, want %q", f.IgnorePaths, wantIgnore)
	}
}

// TestFail2Ban_WiredByServerFactory checks the real wiring: glue injects the
// middleware into the server factory, and it counts the router's own 404s for
// paths no handler is registered on.
func TestFail2Ban_WiredByServerFactory(t *testing.T) {
	ctx, err := glue.New(
		glue.MapPropertySource{
			"test-server.bind-address": "127.0.0.1:0",
			"test-server.options":      "handlers",
		},
		zap.NewNop(),
		newMockRuntime(true),
		HttpServerFactory("test-server"),
		&testHandler{pattern: "/ok"},
		Fail2BanMiddleware(1),
	)
	if err != nil {
		t.Fatalf("glue.New: %v", err)
	}
	defer ctx.Close()

	servers := ctx.Bean(HttpServerClass, glue.DefaultSearchLevel)
	if len(servers) != 1 {
		t.Fatalf("found %d http servers, want 1", len(servers))
	}
	h := servers[0].Object().(*http.Server).Handler

	for i := 0; i < 5; i++ {
		fail2banGet(h, "/wp-login.php", "203.0.113.7")
	}
	if code := fail2banGet(h, "/ok", "203.0.113.7"); code != http.StatusNotFound {
		t.Errorf("banned client: status = %d, want 404", code)
	}
	if code := fail2banGet(h, "/ok", "198.51.100.1"); code != http.StatusOK {
		t.Errorf("other client: status = %d, want 200", code)
	}
}
