package servion

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

func newTestRateLimiter(limit int, interval time.Duration) *implRateLimiterMiddleware {
	return &implRateLimiterMiddleware{
		beanOrder:      1,
		Log:            zap.NewNop(),
		Prefixes:       []string{"/api"},
		Limit:          limit,
		Interval:       interval,
		ClientIDHeader: "X-Forwarded-For",
		buckets:        make(map[string]*rateBucket),
	}
}

func TestRateLimiter_AllowUnderLimit(t *testing.T) {
	rl := newTestRateLimiter(5, time.Minute)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	for i := 0; i < 5; i++ {
		r := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		r.Header.Set("X-Forwarded-For", "1.2.3.4")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want %d", i, w.Code, http.StatusOK)
		}
	}
}

func TestRateLimiter_BlockOverLimit(t *testing.T) {
	rl := newTestRateLimiter(3, time.Minute)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	for i := 0; i < 3; i++ {
		r := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		r.Header.Set("X-Forwarded-For", "1.2.3.4")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	}

	// 4th request should be blocked
	r := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	retryAfter := w.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("expected Retry-After header")
	}
}

func TestRateLimiter_DifferentClients(t *testing.T) {
	rl := newTestRateLimiter(1, time.Minute)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	// Client A uses their 1 request
	r := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	r.Header.Set("X-Forwarded-For", "1.1.1.1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("client A: status = %d, want %d", w.Code, http.StatusOK)
	}

	// Client B should still be allowed
	r = httptest.NewRequest(http.MethodGet, "/api/data", nil)
	r.Header.Set("X-Forwarded-For", "2.2.2.2")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("client B: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRateLimiter_SkipOptions(t *testing.T) {
	rl := newTestRateLimiter(0, time.Minute) // limit=0 would block everything

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	r := httptest.NewRequest(http.MethodOptions, "/api/data", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("OPTIONS should be skipped, got status %d", w.Code)
	}
}

func TestRateLimiter_SkipMissingXFF(t *testing.T) {
	rl := newTestRateLimiter(0, time.Minute) // limit=0 would block

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	r := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	// No X-Forwarded-For header
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("missing XFF should pass through, got status %d", w.Code)
	}
}

func TestRateLimiter_ResetAfterInterval(t *testing.T) {
	rl := newTestRateLimiter(1, 50*time.Millisecond)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	// Use the 1 allowed request
	r := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("first request: status = %d, want %d", w.Code, http.StatusOK)
	}

	// Should be blocked
	r = httptest.NewRequest(http.MethodGet, "/api/data", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	// Wait for interval to pass
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again
	r = httptest.NewRequest(http.MethodGet, "/api/data", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("after reset: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRateLimiter_XFFMultipleIPs(t *testing.T) {
	rl := newTestRateLimiter(1, time.Minute)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(next)

	// First request with chained XFF
	r := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Second request from same original client should be blocked
	r = httptest.NewRequest(http.MethodGet, "/api/data", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 9.9.9.9")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d (same original client)", w.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimiter_Match(t *testing.T) {
	rl := &implRateLimiterMiddleware{
		Prefixes: []string{"/api", "/admin"},
	}

	tests := []struct {
		pattern string
		want    bool
	}{
		{"/api/data", true},
		{"/api", true},
		{"/admin/users", true},
		{"/health", false},
		{"/public/page", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			if got := rl.Match(tt.pattern); got != tt.want {
				t.Errorf("Match(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestRateLimiter_BeanOrder(t *testing.T) {
	rl := RateLimiterMiddleware(10)
	if got := rl.BeanOrder(); got != 10 {
		t.Errorf("BeanOrder() = %d, want 10", got)
	}
}

func TestRateLimiter_PostConstructAndDestroy(t *testing.T) {
	rt := newMockRuntime(true)
	rl := &implRateLimiterMiddleware{
		beanOrder:      1,
		Log:            zap.NewNop(),
		Runtime:        rt,
		Prefixes:       []string{"/api"},
		Limit:          10,
		Interval:       50 * time.Millisecond,
		ClientIDHeader: "X-Forwarded-For",
		buckets:        make(map[string]*rateBucket),
	}

	if err := rl.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	// Destroy should stop the cleaner goroutine
	rt.Shutdown(false)
	if err := rl.Destroy(); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
}

func TestRateLimiter_CleanExpired(t *testing.T) {
	rl := newTestRateLimiter(10, 10*time.Millisecond)

	// Add a bucket that's expired
	rl.buckets["old-client"] = &rateBucket{
		count:     5,
		lastReset: time.Now().Add(-time.Minute),
	}
	// Add a bucket that's still active
	rl.buckets["new-client"] = &rateBucket{
		count:     1,
		lastReset: time.Now(),
	}

	rl.doCleanExpired()

	if _, ok := rl.buckets["old-client"]; ok {
		t.Error("expected expired bucket to be cleaned")
	}
	if _, ok := rl.buckets["new-client"]; !ok {
		t.Error("expected active bucket to remain")
	}
}
