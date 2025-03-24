/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servionapi

import (
	"crypto/tls"
	"go.arpabet.com/glue"
	"net"
	"net/http"
	"reflect"
)

var ServerRole = "server"

var (
	TlsConfigClass  = reflect.TypeOf((*tls.Config)(nil))  // *tls.Config
	HttpServerClass = reflect.TypeOf((*http.Server)(nil)) // *http.Server
)

var ServerClass = reflect.TypeOf((*Server)(nil)).Elem()

type EmptyAddrType struct {
}

func (t EmptyAddrType) Network() string {
	return ""
}

func (t EmptyAddrType) String() string {
	return ""
}

var EmptyAddr net.Addr = EmptyAddrType{}

type Server interface {
	glue.InitializingBean
	glue.DisposableBean

	/**
	Bind server to the port.
	We separated it from the Serve, because we want to start application even if some servers were not able to bind.
	*/

	Bind() error

	/**
	Checks if server alive.
	*/

	Alive() bool

	/**
	Gets the actual listen address that could be different from bind address.
	The good example is if you bing to ip:0 it would have random port assigned to the socket.

	For non active server return EmptyAddr
	*/

	ListenAddress() net.Addr

	/**
	Runs actual server. The error code is the server exit code.
	We automatically filtering the 'closed' socket error codes, because they does not bring something valuable.
	*/

	Serve() error

	/**
	Shutdown server by the request.
	*/

	Shutdown() error

	/**
	ShutdownCh returns a channel that can be selected to wait
	for the server to perform a shutdown.
	*/

	ShutdownCh() <-chan struct{}
}

/**
HTTP Handler interface for routing the HTTP request to specific pattern.
*/

var HttpHandlerClass = reflect.TypeOf((*HttpHandler)(nil)).Elem()

type HttpHandler interface {
	http.Handler

	/**
	Returns the url pattern used to serve the page.
	*/

	Pattern() string
}
