/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"context"
	"crypto/tls"
	"go.arpabet.com/glue"
	"go.uber.org/zap"
	"net"
	"net/http"
	"reflect"
)

var (
	ZapLogClass = reflect.TypeOf((*zap.Logger)(nil))
)

var RuntimeClass = reflect.TypeOf((*Runtime)(nil)).Elem()

/*
Runtime is the base entry point class for golang application.
*/
type Runtime interface {
	context.Context
	glue.InitializingBean
	glue.NamedBean
	Component

	/*
		Gets application binary name, used on startup, could be different with application name
	*/
	Executable() string

	/*
		Gets home directory of the application, could be overriden by flags
	*/
	HomeDir() string

	/*
		Indicator if application is active and not in shutting down mode
	*/
	Active() bool

	/*
		Sets the flag that application is in shutting down mode then notify all go routines by ShutdownChannel then notify signal channel with interrupt signal

		Additionally sets the flag that application is going to be restarted after shutdown
	*/
	Shutdown(restart bool)

	/*
		Indicator if application needs to be restarted by autoupdate or remote command after shutdown
	*/
	Restarting() bool
}

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

// ComponentClass Generic component class that has a name and ability to GetStats
var ComponentClass = reflect.TypeOf((*Component)(nil)).Elem()

type Component interface {
	glue.NamedBean

	/*
		Gets status with name=value key pair.
		Server responds status request with stats ordered by key.
	*/

	GetStats(cb func(name, value string) bool) error
}
