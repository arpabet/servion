package servion

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// implRateLimiterMiddleware implements HttpMiddleware
type implRateLimiterMiddleware struct {
	beanOrder int

	Log *zap.Logger `inject:""`

	Runtime Runtime `inject:""`

	// Prefixes to apply rate limiting
	Prefixes []string `value:"ratelimit.prefixes,default=/api"`

	// Maximum requests per interval
	Limit int `value:"ratelimit.limit,default=10"`

	// Sliding window interval
	Interval time.Duration `value:"ratelimit.interval,default=1s"`

	ClientIDHeader string `value:"ratelimit.header,default=X-Forwarded-For"`

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Internal storage: map clientID -> *bucket
	mu      sync.Mutex
	buckets map[string]*rateBucket
}

// rateBucket tracks requests
type rateBucket struct {
	count     int
	lastReset time.Time
}

func RateLimiterMiddleware(beanOrder int) HttpMiddleware {
	return &implRateLimiterMiddleware{
		beanOrder: beanOrder,
		buckets:   make(map[string]*rateBucket),
	}
}

func (t *implRateLimiterMiddleware) PostConstruct() error {
	t.ctx, t.cancel = context.WithCancel(t.Runtime)
	t.wg.Add(1)
	go t.cleanerLoop()
	return nil
}

func (t *implRateLimiterMiddleware) Destroy() error {
	if t.cancel != nil {
		t.cancel()
	}
	t.wg.Wait()
	return nil
}

func (t *implRateLimiterMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Skip OPTIONS
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		xff := r.Header.Get(t.ClientIDHeader)
		if xff == "" {
			// you should make sure that proxy setup correct header
			// never use remoteAddr, since this rate limiter is designed for app behind proxy, no need to limit proxy itself
			t.Log.Warn("RateLimiterMissingXFF",
				zap.String("X-Forwarded-For", t.ClientIDHeader),
				zap.String("remoteAddr", r.RemoteAddr),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("query", r.URL.RawQuery),
				zap.String("user-agent", r.UserAgent()),
			)
			next.ServeHTTP(w, r)
			return
		}
		parts := strings.Split(xff, ",")
		clientID := strings.TrimSpace(parts[0])

		t.mu.Lock()
		bucket, ok := t.buckets[clientID]
		if !ok {
			bucket = &rateBucket{count: 0, lastReset: time.Now()}
			t.buckets[clientID] = bucket
		}

		// Reset if interval passed
		now := time.Now()
		if now.Sub(bucket.lastReset) > t.Interval {
			bucket.count = 0
			bucket.lastReset = now
		}

		if bucket.count >= t.Limit {
			t.mu.Unlock()
			w.Header().Set("Retry-After", strconv.Itoa(int(t.Interval.Seconds())))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		bucket.count++
		t.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

func (t *implRateLimiterMiddleware) BeanOrder() int {
	return t.beanOrder
}

func (t *implRateLimiterMiddleware) Match(prefix string) bool {
	for _, p := range t.Prefixes {
		if strings.HasPrefix(prefix, p) {
			return true
		}
	}
	return false
}

func (t *implRateLimiterMiddleware) cleanerLoop() {
	defer t.wg.Done()

	ticker := time.NewTicker(t.Interval * 10)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return

		case <-ticker.C:
			t.doCleanExpired()
		}
	}
}

func (t *implRateLimiterMiddleware) doCleanExpired() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	for id, b := range t.buckets {
		if now.Sub(b.lastReset) > t.Interval*5 {
			delete(t.buckets, id)
		}
	}
}
