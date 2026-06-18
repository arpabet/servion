/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 *
 * Self-contained demo of value-rpc over QUIC, wired into servion through the
 * Transport bean. It runs, in one process: a servion value-rpc server and an
 * authenticated client, both using a QUIC Transport from the
 * go.arpabet.com/value-rpc/quic submodule (TLS 1.3 is mandatory for QUIC, so the
 * transport carries a *tls.Config on each side).
 *
 *   GOWORK=off go run .
 *
 * This example is a SEPARATE module so its QUIC dependency
 * (github.com/quic-go/quic-go) never enters servion/vrpc. In production you would
 * split server and client and use a real certificate instead of the self-signed
 * one generated here.
 */

package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"log"
	"math/big"
	"net"
	"time"

	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	servionvrpc "go.arpabet.com/servion/vrpc"
	"go.arpabet.com/value"
	quic "go.arpabet.com/value-rpc/quic"
	"go.arpabet.com/value-rpc/valueclient"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.arpabet.com/value-rpc/valueserver"
)

const serverName = "localhost"

// greeterService is the value-rpc service reached over the QUIC tunnel.
type greeterService struct{}

func (greeterService) RegisterValue(srv valueserver.Server) error {
	return srv.AddFunction("greet", valuerpc.String, valuerpc.String, func(ctx context.Context, args value.Value) (value.Value, error) {
		return value.Utf8("Hello, " + args.String() + "!"), nil
	})
}

// quicTransport implements servionvrpc.Transport over QUIC. value-rpc/quic already
// returns a valuerpc.Listener / valuerpc.Dialer, so the Transport is a thin adapter
// that supplies the mandatory TLS config on each side; the heavy quic-go dependency
// lives in THIS module, not servion/vrpc.
type quicTransport struct {
	serverTLS *tls.Config // server: certificate presented to clients (QUIC ALPN set by the lib)
	clientTLS *tls.Config // client: trust anchor + verified server name
}

func (t *quicTransport) Listener(addr string, wt time.Duration) (valuerpc.Listener, error) {
	return quic.NewListener(addr, t.serverTLS, wt)
}

func (t *quicTransport) Dialer(addr string, wt time.Duration) (valuerpc.Dialer, error) {
	return quic.NewDialer(addr, t.clientTLS, wt), nil
}

func main() {
	cert, pool := genCert()

	transport := &quicTransport{
		serverTLS: &tls.Config{Certificates: []tls.Certificate{cert}},
		clientTLS: &tls.Config{RootCAs: pool, ServerName: serverName},
	}

	// --- servion value-rpc server using the QUIC Transport bean ---
	serverCtx, err := glue.New(
		glue.MapPropertySource{"value-server.bind-address": "127.0.0.1:0"},
		servion.ZapLogFactory(true),
		servionvrpc.ValueServerScanner("value-server", transport, &greeterService{}),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer serverCtx.Close()
	srv := serverCtx.Bean(servion.ServerClass, glue.DefaultSearchLevel)[0].Object().(servion.Server)
	if err := srv.Bind(); err != nil {
		log.Fatal(err)
	}
	go srv.Serve()
	defer srv.Shutdown()
	addr := srv.ListenAddress().String()

	// --- client (servion value-rpc client + the same Transport) ---
	clientCtx, err := glue.New(
		glue.MapPropertySource{"value-client.connect-address": addr},
		servion.ZapLogFactory(true),
		transport, // same bean, used in its client role
		servionvrpc.ValueClientScanner("value-client"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer clientCtx.Close()
	cli := clientCtx.Bean(servionvrpc.ValueClientClass, glue.DefaultSearchLevel)[0].Object().(valueclient.Client)
	if err := cli.Connect(); err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	resp, err := cli.CallFunction(context.Background(), "greet", value.Utf8("World"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nclient over QUIC -> %s\n", resp.String())
}

// genCert builds a self-signed certificate for localhost/127.0.0.1 and a pool that
// trusts it (a real deployment uses a real certificate).
func genCert() (tls.Certificate, *x509.CertPool) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: serverName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{serverName},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		log.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}
