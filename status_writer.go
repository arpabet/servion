/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"bufio"
	"io"
	"net"
	"net/http"
)

/*
statusWriter records the response status and body size for middlewares that
act after the handler has run (access log, fail2ban). It stays transparent to
the handler: WebSocket upgrades (Hijack), streaming (Flush), sendfile
(ReadFrom) and http.ResponseController (Unwrap) all reach the underlying
writer, so wrapping a route never breaks it.
*/
type statusWriter struct {
	http.ResponseWriter
	status      int // final status; keeps its initial value until the handler sets one
	written     int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	// 1xx responses (e.g. 103 Early Hints) come before the final status, and
	// net/http ignores any WriteHeader after the final one.
	if !w.wroteHeader && code >= 200 {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true
	n, err := w.ResponseWriter.Write(b)
	w.written += n
	return n, err
}

func (w *statusWriter) ReadFrom(src io.Reader) (int64, error) {
	w.wroteHeader = true
	var n int64
	var err error
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		n, err = rf.ReadFrom(src)
	} else {
		n, err = io.Copy(w.ResponseWriter, src)
	}
	w.written += int(n)
	return n, err
}

func (w *statusWriter) Flush() {
	w.wroteHeader = true
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
