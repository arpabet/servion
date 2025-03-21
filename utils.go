/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"github.com/pkg/errors"
	"net/http"
	"runtime/debug"
	"strings"
)

func PanicToError(err *error) {
	if r := recover(); r != nil {
		*err = errors.Errorf("%v, %s", r, debug.Stack())
	}
}

func ParseOptions(str string) map[string]bool {
	cache := make(map[string]bool)
	parts := strings.Split(str, ";")
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if len(key) > 0 {
			cache[key] = true
		}
	}
	return cache
}

const (
	acceptEncoding  = "Accept-Encoding"
	contentEncoding = "Content-Encoding"
)

type gzipHandler struct {
	handler http.Handler
}

type gzipWriter struct {
	w http.ResponseWriter
}

func (t gzipWriter) Header() http.Header {
	return t.w.Header()
}

func (t gzipWriter) Write(b []byte) (int, error) {
	return t.w.Write(b)
}

func (t gzipWriter) WriteHeader(statusCode int) {
	if statusCode == 200 {
		t.w.Header().Del(contentEncoding)
		t.w.Header().Set(contentEncoding, "gzip")
	}
	t.w.WriteHeader(statusCode)
}
