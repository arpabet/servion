/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"net/http"

	"go.arpabet.com/glue"
)

type httpServerScanner struct {
	beanName string
	scan     []interface{}
}

func HttpServerScanner(beanName string, scan ...interface{}) glue.Scanner {
	return &httpServerScanner{
		beanName: beanName,
		scan:     scan,
	}
}

func (t *httpServerScanner) ScannerBeans() []interface{} {
	beans := []interface{}{
		HttpServerFactory(t.beanName),
		&struct {
			// make them visible
			Servers     []Server       `inject:"optional"`
			HttpServers []*http.Server `inject:""`
		}{},
	}
	return append(beans, t.scan...)
}
