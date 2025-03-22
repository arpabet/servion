/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servionapi

import (
	"go.arpabet.com/glue"
	"reflect"
)

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
