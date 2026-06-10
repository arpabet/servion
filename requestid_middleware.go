/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

const (
	HeaderXRequestID = "X-Request-ID"
)

type requestIDContextKeyType struct{}

var requestIDContextKey = requestIDContextKeyType{}

// RequestIDFromContext extracts the request ID from the context.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDContextKey).(string)
	return id, ok
}

type implRequestIDMiddleware struct {
	beanOrder int

	Prefixes []string `value:"requestid.prefixes,default=/"`
}

func RequestIDMiddleware(beanOrder int) HttpMiddleware {
	return &implRequestIDMiddleware{beanOrder: beanOrder}
}

func (t *implRequestIDMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderXRequestID)
		if id == "" {
			id = generateRequestID()
		}

		ctx := context.WithValue(r.Context(), requestIDContextKey, id)
		w.Header().Set(HeaderXRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (t *implRequestIDMiddleware) BeanOrder() int {
	return t.beanOrder
}

func (t *implRequestIDMiddleware) Match(prefix string) bool {
	for _, p := range t.Prefixes {
		if strings.HasPrefix(prefix, p) {
			return true
		}
	}
	return false
}

func generateRequestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
