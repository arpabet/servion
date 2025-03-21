/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"go.arpabet.com/glue"
	"go.uber.org/zap"
	"reflect"
)

type implZapLogFactory struct {
	Properties glue.Properties `inject`
}

func ZapLogFactory() glue.FactoryBean {
	return &implZapLogFactory{}
}

func (t *implZapLogFactory) Object() (object interface{}, err error) {
	defer PanicToError(&err)

	return zap.NewDevelopment()
}

func (t *implZapLogFactory) ObjectType() reflect.Type { return ZapLogClass }

func (t *implZapLogFactory) ObjectName() string {
	return "zap_logger"
}

func (t *implZapLogFactory) Singleton() bool {
	return true
}
