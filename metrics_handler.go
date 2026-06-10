/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "servion",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "servion",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	httpResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "servion",
			Name:      "http_response_size_bytes",
			Help:      "HTTP response size in bytes.",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 7),
		},
		[]string{"method", "path"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, httpResponseSize)
}

type implMetricsHandler struct {
	MetricsPattern string `value:"metrics.pattern,default=/metrics"`
}

// MetricsHandler creates a Prometheus metrics HttpHandler bean.
// Exposes /metrics endpoint for scraping.
//
// Configuration properties:
//
//	metrics.pattern – URL pattern (default "/metrics")
func MetricsHandler() HttpHandler {
	return &implMetricsHandler{}
}

func (t *implMetricsHandler) Pattern() string {
	return t.MetricsPattern
}

func (t *implMetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
}

// MetricsMiddleware instruments HTTP handlers with Prometheus metrics.
type implMetricsMiddleware struct {
	beanOrder int
	Prefixes  []string `value:"metrics.prefixes,default=/"`
}

func MetricsMiddleware(beanOrder int) HttpMiddleware {
	return &implMetricsMiddleware{beanOrder: beanOrder}
}

func (t *implMetricsMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		sw := &metricsStatusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		duration := time.Since(start).Seconds()
		path := r.URL.Path
		method := r.Method
		status := strconv.Itoa(sw.status)

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(duration)
		httpResponseSize.WithLabelValues(method, path).Observe(float64(sw.written))
	})
}

func (t *implMetricsMiddleware) BeanOrder() int {
	return t.beanOrder
}

func (t *implMetricsMiddleware) Match(prefix string) bool {
	for _, p := range t.Prefixes {
		if strings.HasPrefix(prefix, p) {
			return true
		}
	}
	return false
}

type metricsStatusWriter struct {
	http.ResponseWriter
	status  int
	written int
}

func (w *metricsStatusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *metricsStatusWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.written += n
	return n, err
}
