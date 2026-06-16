/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servionvrpc

import (
	"fmt"
	"reflect"
	"strings"

	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	"go.arpabet.com/value-rpc/valueclient"
	"go.uber.org/zap"
)

type implValueClientFactory struct {
	Log        *zap.Logger     `inject:""`
	Properties glue.Properties `inject:""`
	Obfs       ObfsProfile     `inject:"optional"`
	Transport  Transport       `inject:"optional"`

	beanName string
}

/*
ValueClientFactory creates a valueclient.Client bean named beanName.

The target address is taken from "<beanName>.connect-address". If empty it is
derived from the matching server bean by replacing "client" with "server" in
beanName and reading "<server>.bind-address" (a 0.0.0.0 or empty host is rewritten
to 127.0.0.1; scheme prefixes like unix:// or ws:// pass through), so a client
co-located with its server needs no extra configuration.

The returned client is not connected yet — call Connect() before use, consistent
with value-rpc's explicit-connect model.

Recognized properties (prefixed by beanName):

	<beanName>.connect-address   target address (overrides the derivation above)
	<beanName>.socks5            optional SOCKS5 proxy "host:port" (TCP only)
	<beanName>.timeout-ms        per-call timeout in milliseconds
*/
func ValueClientFactory(beanName string) glue.FactoryBean {
	return &implValueClientFactory{beanName: beanName}
}

func (t *implValueClientFactory) Object() (object interface{}, err error) {

	defer servion.PanicToError(&err)

	connectAddr := t.Properties.GetString(fmt.Sprintf("%s.connect-address", t.beanName), "")
	if connectAddr == "" {
		serverBean := strings.ReplaceAll(t.beanName, "client", "server")
		bindAddr := t.Properties.GetString(fmt.Sprintf("%s.bind-address", serverBean), "")
		if bindAddr == "" {
			return nil, fmt.Errorf("neither property '%s.connect-address' nor '%s.bind-address' is found in context", t.beanName, serverBean)
		}
		connectAddr = localizeAddr(bindAddr)
	}

	socks5 := t.Properties.GetString(fmt.Sprintf("%s.socks5", t.beanName), "")

	t.Log.Info("ValueClientFactory",
		zap.String("bean", t.beanName),
		zap.String("connectAddr", connectAddr),
		zap.Bool("obfs", t.Obfs != nil),
		zap.Bool("transport", t.Transport != nil))

	var cli valueclient.Client
	switch {
	case t.Transport != nil:
		// The application fully supplies the dialer (e.g. obfs/tlscamo, obfs/reality).
		dialer, derr := t.Transport.Dialer(connectAddr, valueclient.DefaultTimeout)
		if derr != nil {
			return nil, derr
		}
		cli = valueclient.NewClientWithDialer(dialer)
	case t.Obfs != nil:
		// Obfuscation dials and shapes the connection itself, so socks5 (which
		// would proxy the dial) does not compose with it and is ignored.
		dialer, derr := obfsDialer(connectAddr, t.Obfs.ObfsPolicy(), valueclient.DefaultTimeout)
		if derr != nil {
			return nil, derr
		}
		cli = valueclient.NewClientWithDialer(dialer)
	default:
		cli = valueclient.NewClient(connectAddr, socks5)
	}

	if timeoutMls := t.Properties.GetInt(fmt.Sprintf("%s.timeout-ms", t.beanName), 0); timeoutMls > 0 {
		cli.SetTimeout(int64(timeoutMls))
	}

	return cli, nil
}

func (t *implValueClientFactory) ObjectType() reflect.Type { return ValueClientClass }

func (t *implValueClientFactory) ObjectName() string { return t.beanName }

func (t *implValueClientFactory) Singleton() bool { return true }

// localizeAddr turns a server bind address into a dialable client address by
// replacing a wildcard or empty host with the loopback interface. Schemed
// addresses (unix://, ws://, ...) and explicit hosts pass through unchanged.
func localizeAddr(bindAddr string) string {
	if strings.Contains(bindAddr, "://") {
		return bindAddr
	}
	if strings.HasPrefix(bindAddr, "0.0.0.0:") {
		return "127.0.0.1" + bindAddr[len("0.0.0.0"):]
	}
	if strings.HasPrefix(bindAddr, ":") {
		return "127.0.0.1" + bindAddr
	}
	return bindAddr
}
