/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servionvrpc

import (
	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	"go.arpabet.com/value-rpc/valueclient"
)

type valueServerScanner struct {
	beanName string
	scan     []interface{}
}

/*
ValueServerScanner registers a value-rpc server named beanName (as a
servion.Server that binds and serves it) and forwards the extra beans — the
ValueService implementations and an optional ConnectAuthorizer. It is the vRPC
counterpart of servion.HttpServerScanner / serviongrpc.GrpcServerScanner and is
passed to servion.RunCommand.

	servion.RunCommand(
		servionvrpc.ValueServerScanner("value-server",
			&greeterService{},
		),
	)
*/
func ValueServerScanner(beanName string, scan ...interface{}) glue.Scanner {
	return &valueServerScanner{
		beanName: beanName,
		scan:     scan,
	}
}

func (t *valueServerScanner) ScannerBeans() []interface{} {
	beans := []interface{}{
		ValueServer(t.beanName),
		&struct {
			// make them visible / force construction
			Servers []servion.Server `inject:"optional"`
		}{},
	}
	return append(beans, t.scan...)
}

type valueClientScanner struct {
	beanName string
	scan     []interface{}
}

/*
ValueClientScanner registers a valueclient.Client named beanName and forwards the
extra beans, typically the service beans that inject and use the client.
*/
func ValueClientScanner(beanName string, scan ...interface{}) glue.Scanner {
	return &valueClientScanner{
		beanName: beanName,
		scan:     scan,
	}
}

func (t *valueClientScanner) ScannerBeans() []interface{} {
	beans := []interface{}{
		ValueClientFactory(t.beanName),
		&struct {
			// make them visible
			Clients []valueclient.Client `inject:"optional"`
		}{},
	}
	return append(beans, t.scan...)
}
