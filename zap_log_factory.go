/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"reflect"

	"go.arpabet.com/glue"
	"go.uber.org/zap"
)

type implZapLogFactory struct {
	Properties  glue.Properties `inject:""`
	development bool
}

func ZapLogFactory(development bool) glue.FactoryBean {
	return &implZapLogFactory{development: development}
}

func (t *implZapLogFactory) Object() (object interface{}, err error) {
	defer PanicToError(&err)

	if t.development {
		return zap.NewDevelopment()
	}
	return zap.NewProduction()
}

func (t *implZapLogFactory) ObjectType() reflect.Type { return ZapLogClass }

func (t *implZapLogFactory) ObjectName() string {
	return "zap_logger"
}

func (t *implZapLogFactory) Singleton() bool {
	return true
}
