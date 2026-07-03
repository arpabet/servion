/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.arpabet.com/glue"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
)

type implHttpServerFactory struct {
	Log         *zap.Logger            `inject:""`
	Properties  glue.Properties        `inject:""`
	Handlers    []HttpHandler          `inject:"optional,level=1"`
	Middlewares []HttpMiddleware       `inject:"optional,level=1"`
	Resources   []*glue.ResourceSource `inject:"optional"`
	TlsConfig   *tls.Config            `inject:"optional"`

	beanName string
}

func HttpServerFactory(beanName string) glue.FactoryBean {
	return &implHttpServerFactory{beanName: beanName}
}

func (t *implHttpServerFactory) PostConstruct() error {
	if len(t.Middlewares) > 1 {
		sort.Slice(t.Middlewares, func(i, j int) bool {
			return t.Middlewares[i].BeanOrder() < t.Middlewares[j].BeanOrder()
		})
	}
	return nil
}

func (t *implHttpServerFactory) isEnabled(name string) bool {
	return t.Properties.GetBool(fmt.Sprintf("%s.%s", t.beanName, name), false)
}

func (t *implHttpServerFactory) Object() (object interface{}, err error) {

	defer PanicToError(&err)

	listenAddr := t.Properties.GetString(fmt.Sprintf("%s.%s", t.beanName, "bind-address"), "")

	if listenAddr == "" {
		return nil, xerrors.Errorf("property '%s.bind-address' not found in server context", t.beanName)
	}

	options := ParseOptions(t.Properties.GetString(fmt.Sprintf("%s.%s", t.beanName, "options"), ""))

	serveMux := mux.NewRouter()

	visitedPatterns := make(map[string]bool)

	var handlerList []string
	if options["handlers"] {
		for _, handler := range t.Handlers {
			pattern := handler.Pattern()
			if visitedPatterns[pattern] {
				t.Log.Warn("PatternExist", zap.String("pattern", pattern), zap.Any("handler", handler))
			} else {
				visitedPatterns[pattern] = true

				// Wrap handler with middlewares in reverse order so that
				// the first middleware in the list runs first on the request.
				var h http.Handler = handler

				if len(t.Middlewares) > 0 { // nil-safe, also works if empty
					for i := len(t.Middlewares) - 1; i >= 0; i-- {
						middleware := t.Middlewares[i]
						if middleware != nil && middleware.Match(pattern) { // extra safety check
							h = middleware.Middleware(h)
						}
					}
				}

				handlerList = append(handlerList, pattern)
				serveMux.Handle(pattern, h)

			}
		}
	}

	var assetList []string
	var groupedAssets map[string]*servingAsset
	if options["assets"] || options["spa"] {
		groupedAssets = t.groupAssets()
	}

	if options["assets"] {
		for pattern, handler := range groupedAssets {
			if visitedPatterns[pattern] {
				t.Log.Warn("PatternExist", zap.String("pattern", pattern))
			}
			visitedPatterns[pattern] = true
			assetList = append(assetList, pattern)
			serveMux.Handle(pattern, handler)
		}
	}

	if options["spa"] {
		if index, ok := groupedAssets["/"]; ok {
			exclude := ParsePrefixList(t.Properties.GetString(fmt.Sprintf("%s.%s", t.beanName, "spa-exclude"), "/api"))
			serveMux.NotFoundHandler = &spaFallback{index: index, exclude: exclude}
		} else {
			t.Log.Warn("SpaIndexHtmlNotFound", zap.String("bean", t.beanName))
		}
	}

	var tlsConfig *tls.Config
	if options["tls"] {
		if t.TlsConfig != nil {
			tlsConfig = t.TlsConfig.Clone()
		} else {
			t.Log.Warn("TLSConfigNotFound", zap.String("bean", t.beanName))
		}
	}

	readTimeout := t.Properties.GetDuration(fmt.Sprintf("%s.%s", t.beanName, "read-timeout"), 30*time.Second)
	writeTimeout := t.Properties.GetDuration(fmt.Sprintf("%s.%s", t.beanName, "write-timeout"), 30*time.Second)
	idleTimeout := t.Properties.GetDuration(fmt.Sprintf("%s.%s", t.beanName, "idle-timeout"), time.Minute)

	t.Log.Info("HTTPServerFactory",
		zap.String("listenAddr", listenAddr),
		zap.String("bean", t.beanName),
		zap.Strings("handlers", handlerList),
		zap.Strings("assets", assetList),
		zap.Any("options", options),
		zap.Bool("tls", tlsConfig != nil))

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      serveMux,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
		TLSConfig:    tlsConfig,
	}

	return srv, nil

}

func (t *implHttpServerFactory) ObjectType() reflect.Type { return HttpServerClass }

func (t *implHttpServerFactory) ObjectName() string {

	return t.beanName
}

func (t *implHttpServerFactory) Singleton() bool {
	return true
}

type servingAsset struct {
	pattern string
	plainH  http.Handler
	gzipH   http.Handler
}

func (t *servingAsset) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if t.gzipH != nil && t.acceptGzip(r) {
		t.gzipH.ServeHTTP(w, r)
		return
	}
	if t.plainH != nil {
		t.plainH.ServeHTTP(w, r)
		return
	}
	http.Error(w, "resource not found", http.StatusNotFound)
}

func (t *servingAsset) acceptGzip(r *http.Request) bool {
	list := strings.Split(r.Header.Get(acceptEncoding), ",")
	for _, enc := range list {
		enc = strings.TrimSpace(enc)
		if "gzip" == enc {
			return true
		}
	}
	return false
}

func (t *implHttpServerFactory) groupAssets() map[string]*servingAsset {

	cache := make(map[string]*servingAsset)

	for _, res := range t.Resources {
		if strings.HasPrefix(res.Name, "assets") {

			var gzip bool
			var handler http.Handler
			handler = http.FileServer(res.AssetFiles)

			if strings.HasSuffix(res.Name, "gzip") {
				handler = gzipHeaderHandler{h: handler}
				gzip = true
			}

			for _, name := range res.AssetNames {

			Again:

				pattern := "/" + name
				s, ok := cache[pattern]
				if !ok {
					s = &servingAsset{pattern: pattern}
					cache[pattern] = s
				}

				if gzip {
					if s.gzipH != nil {
						t.Log.Warn("GzipHandlerExist", zap.String("pattern", pattern), zap.String("asset", name), zap.Any("files", res.AssetFiles))
					}
					s.gzipH = handler
				} else {
					if s.plainH != nil {
						t.Log.Warn("PlainHandlerExist", zap.String("pattern", pattern), zap.String("asset", name), zap.Any("files", res.AssetFiles))
					}
					s.plainH = handler
				}

				if name == "index.html" {
					name = ""
					goto Again
				}

			}

		}

	}

	return cache
}

// spaFallback serves the embedded index.html for unmatched paths so that
// single-page applications using history-mode routing can deep link.
// Non-GET/HEAD requests, excluded prefixes, and paths with a file extension
// still return 404.
type spaFallback struct {
	index   http.Handler
	exclude []string
}

func (t *spaFallback) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	for _, prefix := range t.exclude {
		if strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
	}
	if path.Ext(r.URL.Path) != "" {
		http.NotFound(w, r)
		return
	}
	// Rewrite to "/" rather than "/index.html": http.FileServer redirects any
	// path ending in "/index.html" to "./", which would lose the deep link.
	r2 := r.Clone(r.Context())
	r2.URL.Path = "/"
	r2.URL.RawPath = ""
	t.index.ServeHTTP(w, r2)
}

type gzipHeaderHandler struct {
	h http.Handler
}

func (t gzipHeaderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.h.ServeHTTP(gzipWriter{w}, r)
}
