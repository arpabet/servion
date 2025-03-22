/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"context"
	"github.com/pkg/errors"
	"go.arpabet.com/glue"
	"go.arpabet.com/servion/servionapi"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gopkg.in/natefinch/lumberjack.v2"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
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

func doWithServers(core glue.Context, cb func([]servionapi.Server) error) (err error) {

	var contextList []glue.Context

	defer func() {

		var listErr []error
		if r := recover(); r != nil {
			listErr = append(listErr, errors.Errorf("recovered on error: %v", r))
		}

		for _, ctx := range contextList {
			if ctx != core {
				if e := ctx.Close(); e != nil {
					listErr = append(listErr, e)
				}
			}
		}

		if len(listErr) > 0 {
			err = errors.Errorf("%+v", listErr)
		}

	}()

	if len(core.Children()) == 0 {
		// no child contexts found, use core context for server
		contextList = append(contextList, core)
	} else {
		for _, child := range core.Children() {
			// Initialize child context, by default they are not initialized
			if ctx, err := child.Object(); err != nil {
				return errors.Errorf("server creation context '%v' failed by %v", child, err)
			} else {
				contextList = append(contextList, ctx)
			}
		}
	}

	var serverList []servionapi.Server
	for _, ctx := range contextList {

		for i, bean := range ctx.Bean(servionapi.ServerClass, glue.DefaultLevel) {
			if srv, ok := bean.Object().(servionapi.Server); ok {
				serverList = append(serverList, srv)
			} else {
				return errors.Errorf("invalid object found for servionapi.Server on position %d in child context: %v", i, ctx)
			}
		}

		for i, bean := range ctx.Bean(servionapi.HttpServerClass, glue.DefaultLevel) {
			if srv, ok := bean.Object().(*http.Server); ok {
				s := NewHttpServer(srv)
				if err := ctx.Inject(s); err != nil {
					return errors.Errorf("injection error for server '%s' of *http.Server on position %d in child context %v, %v", srv.Addr, i, ctx, err)
				}
				serverList = append(serverList, s)
			} else {
				return errors.Errorf("invalid object found for *http.Server on position %d in child context %v", i, ctx)
			}
		}

	}

	return cb(serverList)
}

func runServers(runtime servionapi.Runtime, core glue.Context, log *zap.Logger) error {

	return doWithServers(core, func(servers []servionapi.Server) (err error) {

		defer PanicToError(&err)
		defer log.Sync()

		if len(servers) == 0 {
			return errors.New("servionapi.Server instances are not found in server context")
		}

		c, cancel := context.WithCancel(runtime)
		defer cancel()

		var boundServers []servionapi.Server
		for _, server := range servers {
			if err := server.Bind(); err != nil {
				log.Error("Bind", zap.Error(err))
			} else {
				boundServers = append(boundServers, server)
			}
		}

		cnt := 0
		g, groupCtx := errgroup.WithContext(c)

		for _, server := range boundServers {
			g.Go(server.Serve)
			cnt++
		}
		log.Info("ServionStarted", zap.Int("Servers", cnt))

		// if application shutdown or first server stops then groupCtx going to be canceled
		// if groupCtx canceled we need to shutdown all servers
		// ALL or Nothing
		go func() {
			select {
			case <-groupCtx.Done():
				for _, server := range boundServers {
					g.Go(server.Shutdown)
				}
			}
		}()

		go func() {

			signalCh := make(chan os.Signal, 10)
			signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

			var signal os.Signal

		waitAgain:
			select {
			case signal = <-signalCh:
			case <-runtime.Done():
				signal = syscall.SIGABRT
			}

			log.Info("StopSignal", zap.String("signal", signal.String()))

			if signal == syscall.SIGHUP {
				list := core.Bean(servionapi.LumberjackClass, 1)
				if len(list) > 0 {
					for _, bean := range list {
						if logger, ok := bean.Object().(*lumberjack.Logger); ok {
							logger.Rotate()
						}
					}
					goto waitAgain
				}
				// no lumberjack found, restart application
				runtime.Shutdown(true)
			} else {
				runtime.Shutdown(false)
			}

		}()

		return g.Wait()
	})

}
