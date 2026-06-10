/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"net/http"
	"strings"
)

type implCorsMiddleware struct {
	beanOrder int

	Prefixes       []string `value:"cors.prefixes,default=/"`
	AllowOrigins   []string `value:"cors.allow-origins,default=*"`
	AllowMethods   []string `value:"cors.allow-methods,default=GET;POST;PUT;DELETE;PATCH;OPTIONS"`
	AllowHeaders   []string `value:"cors.allow-headers,default=Authorization;Content-Type;X-Request-ID"`
	ExposeHeaders  []string `value:"cors.expose-headers,default=X-Request-ID"`
	AllowCreds     bool     `value:"cors.allow-credentials,default=false"`
	MaxAge         string   `value:"cors.max-age,default=86400"`
}

func CorsMiddleware(beanOrder int) HttpMiddleware {
	return &implCorsMiddleware{beanOrder: beanOrder}
}

func (t *implCorsMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		if !t.isOriginAllowed(origin) {
			next.ServeHTTP(w, r)
			return
		}

		h := w.Header()
		h.Set("Access-Control-Allow-Origin", origin)

		if t.AllowCreds {
			h.Set("Access-Control-Allow-Credentials", "true")
		}

		if len(t.ExposeHeaders) > 0 {
			h.Set("Access-Control-Expose-Headers", strings.Join(t.ExposeHeaders, ", "))
		}

		// Preflight
		if r.Method == http.MethodOptions {
			h.Set("Access-Control-Allow-Methods", strings.Join(t.AllowMethods, ", "))
			h.Set("Access-Control-Allow-Headers", strings.Join(t.AllowHeaders, ", "))
			h.Set("Access-Control-Max-Age", t.MaxAge)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (t *implCorsMiddleware) isOriginAllowed(origin string) bool {
	for _, o := range t.AllowOrigins {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
}

func (t *implCorsMiddleware) BeanOrder() int {
	return t.beanOrder
}

func (t *implCorsMiddleware) Match(prefix string) bool {
	for _, p := range t.Prefixes {
		if strings.HasPrefix(prefix, p) {
			return true
		}
	}
	return false
}
