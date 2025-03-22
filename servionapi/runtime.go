/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servionapi

import (
	"context"
	"go.arpabet.com/glue"
	"reflect"
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
		Gets application runtime profile, could be: dev, qa, prod and etc.
	*/
	Profile() string

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
