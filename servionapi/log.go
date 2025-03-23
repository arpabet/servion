/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servionapi

import (
	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
	"reflect"
)

var (
	ZapLogClass     = reflect.TypeOf((*zap.Logger)(nil))
	LumberjackClass = reflect.TypeOf((*lumberjack.Logger)(nil))
)
