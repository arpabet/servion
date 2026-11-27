/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/xerrors"
)

// fail2banMaxTracked bounds memory: past this many tracked clients, new
// clients are not tracked until the cleaner frees room. Existing bans still hold.
const fail2banMaxTracked = 100_000

/*
implFail2BanMiddleware bans a client that collects MaxRetry 404 responses
within FindTime: for BanTime, every request from it gets a plain 404 without
reaching the router. Scanners probing for /.env, /wp-login.php and the like
trip it in seconds; a person following a broken link does not.

It is an HttpRouterMiddleware, so it counts every 404 the server sends,
including the ones from the not-found and SPA fallbacks, which per-handler
HttpMiddleware never sees.

The client is identified by Header (CF-Connecting-IP by default), never by the
TCP peer, which behind a proxy is the proxy. The header can only be trusted
when the origin accepts connections from Cloudflare alone; otherwise anyone can
send it. Requests without a parseable address pass through untracked. IPv6
clients are tracked by their /64, which usually belongs to a single host.
*/
type implFail2BanMiddleware struct {
	beanOrder int

	Log *zap.Logger `inject:""`

	Runtime Runtime `inject:""`

	// Header holding the client address, set by the edge proxy.
	Header string `value:"fail2ban.header,default=CF-Connecting-IP"`

	// MaxRetry 404 responses within FindTime ban the client for BanTime.
	MaxRetry int           `value:"fail2ban.maxretry,default=5"`
	FindTime time.Duration `value:"fail2ban.findtime,default=1m"`
	BanTime  time.Duration `value:"fail2ban.bantime,default=1h"`

	// 404s under these path prefixes never count: browsers request them on
	// their own, and stale tabs ask for hashed bundles a deploy replaced.
	IgnorePaths []string `value:"fail2ban.ignore-paths,default=/favicon.ico;/apple-touch-icon;/robots.txt;/sitemap.xml;/.well-known/;/assets/"`

	now        func() time.Time
	maxTracked int

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	clients map[string]*fail2banClient

	warnMissingHeader sync.Once
}

type fail2banClient struct {
	strikes     []time.Time // 404s within the last FindTime, oldest first
	bannedUntil time.Time
}

func Fail2BanMiddleware(beanOrder int) HttpRouterMiddleware {
	return &implFail2BanMiddleware{
		beanOrder:  beanOrder,
		now:        time.Now,
		maxTracked: fail2banMaxTracked,
		clients:    make(map[string]*fail2banClient),
	}
}

func (t *implFail2BanMiddleware) PostConstruct() error {
	if t.MaxRetry < 1 || t.FindTime <= 0 || t.BanTime <= 0 {
		return xerrors.Errorf("fail2ban: need maxretry >= 1 and positive findtime and bantime, got maxretry=%d findtime=%s bantime=%s",
			t.MaxRetry, t.FindTime, t.BanTime)
	}
	t.ctx, t.cancel = context.WithCancel(t.Runtime)
	t.wg.Add(1)
	go t.cleanerLoop()
	return nil
}

func (t *implFail2BanMiddleware) Destroy() error {
	if t.cancel != nil {
		t.cancel()
	}
	t.wg.Wait()
	return nil
}

func (t *implFail2BanMiddleware) BeanOrder() int {
	return t.beanOrder
}

func (t *implFail2BanMiddleware) RouterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client, ok := t.clientKey(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		if t.banned(client) {
			http.NotFound(w, r)
			return
		}
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		if sw.status == http.StatusNotFound && !t.ignored(r.URL.Path) {
			t.strike(client, r)
		}
	})
}

// clientKey returns what bans are keyed on: the client IP from Header, or its
// /64 for IPv6. A list-valued header contributes its first entry.
func (t *implFail2BanMiddleware) clientKey(r *http.Request) (string, bool) {
	raw, _, _ := strings.Cut(r.Header.Get(t.Header), ",")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		// Loopback peers are local probes and tools, not a misconfigured proxy.
		if peer, err := netip.ParseAddrPort(r.RemoteAddr); err != nil || !peer.Addr().IsLoopback() {
			t.warnMissingHeader.Do(func() {
				t.Log.Warn("Fail2BanMissingHeader",
					zap.String("header", t.Header),
					zap.String("remoteAddr", r.RemoteAddr),
					zap.String("path", r.URL.Path))
			})
		}
		return "", false
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return "", false
	}
	addr = addr.Unmap().WithZone("")
	if addr.Is6() {
		prefix, err := addr.Prefix(64)
		if err != nil {
			return "", false
		}
		return prefix.String(), true
	}
	return addr.String(), true
}

func (t *implFail2BanMiddleware) banned(client string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	c, ok := t.clients[client]
	return ok && t.now().Before(c.bannedUntil)
}

func (t *implFail2BanMiddleware) ignored(path string) bool {
	for _, p := range t.IgnorePaths {
		if p != "" && strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// strike records a 404 for client and bans it once MaxRetry fall within FindTime.
func (t *implFail2BanMiddleware) strike(client string, r *http.Request) {
	now := t.now()

	t.mu.Lock()
	c, ok := t.clients[client]
	if !ok {
		if len(t.clients) >= t.maxTracked {
			t.mu.Unlock()
			return
		}
		c = &fail2banClient{}
		t.clients[client] = c
	}
	c.strikes = append(dropUntil(c.strikes, now.Add(-t.FindTime)), now)
	strikes := len(c.strikes)
	if strikes < t.MaxRetry {
		t.mu.Unlock()
		return
	}
	c.strikes = nil
	c.bannedUntil = now.Add(t.BanTime)
	t.mu.Unlock()

	t.Log.Warn("Fail2BanBanned",
		zap.String("client", client),
		zap.Int("strikes", strikes),
		zap.Duration("bantime", t.BanTime),
		zap.String("host", r.Host),
		zap.String("path", r.URL.Path),
		zap.String("userAgent", r.UserAgent()))
}

// dropUntil removes the times at or before cutoff from the front of times,
// which is sorted oldest first.
func dropUntil(times []time.Time, cutoff time.Time) []time.Time {
	i := 0
	for i < len(times) && !times[i].After(cutoff) {
		i++
	}
	return times[i:]
}

func (t *implFail2BanMiddleware) cleanerLoop() {
	defer t.wg.Done()

	ticker := time.NewTicker(t.FindTime)
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

// doCleanExpired forgets clients that are neither banned nor have strikes in
// the current window.
func (t *implFail2BanMiddleware) doCleanExpired() {
	now := t.now()
	cutoff := now.Add(-t.FindTime)

	t.mu.Lock()
	defer t.mu.Unlock()
	for client, c := range t.clients {
		c.strikes = dropUntil(c.strikes, cutoff)
		if len(c.strikes) == 0 && !now.Before(c.bannedUntil) {
			delete(t.clients, client)
		}
	}
}
