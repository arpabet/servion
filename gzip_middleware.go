package servion

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type implGzipMiddleware struct {
	beanOrder    int
	Level        int      `value:"gzip.level,default=1"`                  // gzip compression level
	Threshold    int      `value:"gzip.threshold,default=1024"`           // bytes, default 1024
	SkipPrefixes []string `value:"gzip.skip,default=/images;/videos;/ws"` // URL prefixes NOT to gzip
}

func GzipMiddleware(beanOrder int) HttpMiddleware {
	return &implGzipMiddleware{beanOrder: beanOrder}
}

func (t *implGzipMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// ---- REQUEST ----
		if isGzippedRequest(r) {
			r2, err := decompressRequest(r)
			if err != nil {
				http.Error(w, "invalid gzip request body", http.StatusBadRequest)
				return
			}
			r = r2
		}

		// ---- RESPONSE ----
		if !acceptsGzip(r) || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}

		aw := &adaptiveGzipWriter{
			ResponseWriter: w,
			level:          t.Level,
			minSize:        t.Threshold,
		}
		defer aw.Close()

		next.ServeHTTP(aw, r)

		// Small response → send plain
		if !aw.started {
			if aw.status == 0 {
				aw.status = http.StatusOK
			}
			w.Header().Set(hContentLength, strconv.Itoa(aw.buf.Len()))
			w.WriteHeader(aw.status)
			io.Copy(w, &aw.buf)
		}
	})
}

func (t *implGzipMiddleware) Match(pattern string) bool {
	for _, p := range t.SkipPrefixes {
		if strings.HasPrefix(pattern, p) {
			return false
		}
	}
	return true
}

func (t *implGzipMiddleware) BeanOrder() int {
	return t.beanOrder
}

const (
	hAcceptEncoding  = "Accept-Encoding"
	hContentEncoding = "Content-Encoding"
	hContentLength   = "Content-Length"
	hVary            = "Vary"
	encGzip          = "gzip"
)

func acceptsGzip(r *http.Request) bool {
	for _, v := range strings.Split(r.Header.Get(hAcceptEncoding), ",") {
		if strings.TrimSpace(v) == encGzip {
			return true
		}
	}
	return false
}

func isGzippedRequest(r *http.Request) bool {
	return strings.Contains(r.Header.Get(hContentEncoding), encGzip)
}

func decompressRequest(r *http.Request) (*http.Request, error) {
	zr, err := gzip.NewReader(r.Body)
	if err != nil {
		return nil, err
	}

	r2 := r.Clone(r.Context())
	r2.Body = struct {
		io.Reader
		io.Closer
	}{
		Reader: zr,
		Closer: zr,
	}

	r2.Header = r.Header.Clone()
	r2.Header.Del(hContentEncoding)
	r2.Header.Del(hContentLength)

	return r2, nil
}

type adaptiveGzipWriter struct {
	http.ResponseWriter

	level   int
	minSize int
	buf     bytes.Buffer
	size    int
	gw      *gzip.Writer
	started bool
	status  int
}

func (w *adaptiveGzipWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
}

func (w *adaptiveGzipWriter) Write(p []byte) (int, error) {
	w.size += len(p)

	// Not started yet → buffer
	if !w.started {
		w.buf.Write(p)

		if w.buf.Len() < w.minSize {
			return len(p), nil
		}

		// Threshold exceeded → start gzip
		w.startGzip()
	}

	return w.gw.Write(p)
}

func (w *adaptiveGzipWriter) startGzip() {
	w.started = true

	h := w.Header()
	h.Set(hContentEncoding, encGzip)
	h.Add(hVary, hAcceptEncoding)
	h.Del(hContentLength)

	if w.status == 0 {
		w.status = http.StatusOK
	}

	w.ResponseWriter.WriteHeader(w.status)

	gw, _ := gzip.NewWriterLevel(w.ResponseWriter, w.level)
	w.gw = gw

	io.Copy(w.gw, &w.buf)
	w.buf.Reset()
}

func (w *adaptiveGzipWriter) Close() error {
	if w.started {
		return w.gw.Close()
	}
	return nil
}
