/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

type implAccessLogMiddleware struct {
	beanOrder int

	Log *zap.Logger `inject:""`

	Prefixes []string `value:"accesslog.prefixes,default=/"`
}

func AccessLogMiddleware(beanOrder int) HttpMiddleware {
	return &implAccessLogMiddleware{beanOrder: beanOrder}
}

func (t *implAccessLogMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		duration := time.Since(start)

		fields := []zap.Field{
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", sw.status),
			zap.Duration("duration", duration),
			zap.Int("bytes", sw.written),
			zap.String("remote", r.RemoteAddr),
		}

		if id, ok := RequestIDFromContext(r.Context()); ok {
			fields = append(fields, zap.String("requestId", id))
		}

		if ua := r.UserAgent(); ua != "" {
			fields = append(fields, zap.String("userAgent", ua))
		}

		t.Log.Info("access", fields...)
	})
}

func (t *implAccessLogMiddleware) BeanOrder() int {
	return t.beanOrder
}

func (t *implAccessLogMiddleware) Match(prefix string) bool {
	for _, p := range t.Prefixes {
		if strings.HasPrefix(prefix, p) {
			return true
		}
	}
	return false
}

type statusWriter struct {
	http.ResponseWriter
	status  int
	written int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.written += n
	return n, err
}
