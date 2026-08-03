/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	"go.arpabet.com/glue"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
)

func PanicToError(err *error) {
	if r := recover(); r != nil {
		*err = xerrors.Errorf("%v, %s", r, debug.Stack())
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

func ParsePrefixList(str string) []string {
	var list []string
	for _, part := range strings.Split(str, ";") {
		if prefix := strings.TrimSpace(part); prefix != "" {
			list = append(list, prefix)
		}
	}
	return list
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

func doWithServers(core glue.Container, cb func([]Server) error) (err error) {

	var childList []glue.Container

	defer func() {

		var listErr []error
		if r := recover(); r != nil {
			listErr = append(listErr, xerrors.Errorf("recovered on error: %v", r))
		}

		for _, ctx := range childList {
			if ctx != core {
				if e := ctx.Close(); e != nil {
					listErr = append(listErr, e)
				}
			}
		}

		if len(listErr) > 0 {
			err = xerrors.Errorf("%v", listErr)
		}

	}()

	if len(core.Children()) == 0 {
		// no child contexts found, use core context for server
		childList = append(childList, core)
	} else {
		for _, child := range core.Children() {
			// Initialize child context, by default they are not initialized
			if ctx, err := child.Object(); err != nil {
				return xerrors.Errorf("server creation context '%v' failed: %w", child, err)
			} else {
				childList = append(childList, ctx)
			}
		}
	}

	var serverList []Server
	for _, ctx := range childList {

		for i, bean := range ctx.Bean(ServerClass, glue.DefaultSearchLevel) {
			if srv, ok := bean.Object().(Server); ok {
				serverList = append(serverList, srv)
			} else {
				return xerrors.Errorf("invalid object found for servionapi.Server on position %d in child context: %v", i, ctx)
			}
		}

		for i, bean := range ctx.Bean(HttpServerClass, glue.DefaultSearchLevel) {
			if srv, ok := bean.Object().(*http.Server); ok {
				s := NewHttpServer(srv)
				if err := ctx.Inject(s); err != nil {
					return xerrors.Errorf("injection error for server '%s' of *http.Server on position %d in child context %v: %w", srv.Addr, i, ctx, err)
				}
				serverList = append(serverList, s)
			} else {
				return xerrors.Errorf("invalid object found for *http.Server on position %d in child context %v", i, ctx)
			}
		}

	}

	return cb(serverList)
}

/*
bindServers binds every server, reporting failures to THREE audiences that never
overlap:

  - the zap log, for whoever tails it later;
  - stderr, for the person at the terminal RIGHT NOW — in production the zap
    log is usually a rotated file, and a bind failure that goes only there is
    invisible at the exact moment someone is watching;
  - the returned error, when NOTHING bound — an application with zero
    listeners is not degraded, it is not running, and before this it exited 0
    with no output: the errgroup waited on an empty set, Wait returned nil,
    and the process ended looking exactly like a successful daemonization.

A PARTIAL failure still serves, deliberately: an application whose admin
surface is up can be used to FIX the port that failed, and killing everything
over one conflict would take that remedy away.
*/
func bindServers(servers []Server, log *zap.Logger, stderr io.Writer) ([]Server, error) {
	var bound []Server
	var bindErrs []error
	for _, server := range servers {
		if err := server.Bind(); err != nil {
			log.Error("Bind", zap.Error(err))
			fmt.Fprintf(stderr, "warning: %v\n", err)
			bindErrs = append(bindErrs, err)
		} else {
			bound = append(bound, server)
		}
	}
	if len(bound) == 0 {
		// The hint comes BEFORE the %w: xerrors renders the wrapped chain at
		// the end, so a suffix after the verb would make the same bind failure
		// print twice — once inside the join, once as the chain.
		return nil, xerrors.Errorf("no server could bind (%d of %d failed)%s: %w",
			len(bindErrs), len(servers), addrInUseHint(bindErrs), errors.Join(bindErrs...))
	}
	if len(bindErrs) > 0 {
		fmt.Fprintf(stderr, "warning: %d of %d servers failed to bind — continuing with the rest so the failure can be fixed from the admin surface\n",
			len(bindErrs), len(servers))
	}
	return bound, nil
}

/*
addrInUseHint names the LIKELY cause when a bind failed with EADDRINUSE: another
instance of the same application. It is the single most common way to reach
"nothing bound" — a second `run` in another terminal — and the raw errno tells
an operator what happened without telling them what it means.
*/
func addrInUseHint(errs []error) string {
	for _, err := range errs {
		if errors.Is(err, syscall.EADDRINUSE) {
			return " — is another instance already running?"
		}
	}
	return ""
}

func runServers(runtime Runtime, core glue.Container, log *zap.Logger) error {

	return doWithServers(core, func(servers []Server) (err error) {

		defer PanicToError(&err)
		defer log.Sync()

		if len(servers) == 0 {
			return xerrors.New("servionapi.Server instances are not found in server context")
		}

		c, cancel := context.WithCancel(runtime)
		defer cancel()

		boundServers, err := bindServers(servers, log, os.Stderr)
		if err != nil {
			return err
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

			select {
			case signal = <-signalCh:
			case <-runtime.Done():
				signal = syscall.SIGABRT
			}

			log.Info("StopSignal", zap.String("signal", signal.String()))

			if signal == syscall.SIGHUP {
				// restart application
				runtime.Shutdown(true)
			} else {
				runtime.Shutdown(false)
			}

		}()

		return g.Wait()
	})

}
