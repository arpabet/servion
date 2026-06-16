/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 *
 * Self-contained demo of value-rpc over an active-probe-resistant transport
 * (obfs/reality), wired into servion through the Transport bean. It runs, in one
 * process: a fallback "real website", a servion value-rpc server using a reality
 * Transport, an authenticated client, and a censor-style probe — showing that the
 * authenticated client gets the tunnel while the probe is reverse-proxied to the
 * fallback site.
 *
 *   GOWORK=off go run .
 *
 * This example is a SEPARATE module so its uTLS dependency (via obfs/reality and
 * obfs/tlscamo) never enters servion/vrpc. In production you would split server and
 * client, use a real certificate for the fronted domain, and distribute the token
 * out of band.
 */

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"time"

	"go.arpabet.com/glue"
	"go.arpabet.com/obfs/reality"
	"go.arpabet.com/obfs/tlscamo"
	"go.arpabet.com/servion"
	servionvrpc "go.arpabet.com/servion/vrpc"
	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valueclient"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.arpabet.com/value-rpc/valueserver"
)

// greeterService is the value-rpc service the authenticated tunnel reaches.
type greeterService struct{}

func (greeterService) RegisterValue(srv valueserver.Server) error {
	return srv.AddFunction("greet", valuerpc.String, valuerpc.String, func(args value.Value) (value.Value, error) {
		return value.Utf8("Hello, " + args.String() + "!"), nil
	})
}

// realityTransport implements servionvrpc.Transport, composing obfs/reality on
// both ends. The heavy uTLS dependency lives in THIS module, not servion/vrpc.
type realityTransport struct {
	serverTLS  *tls.Config    // server: certificate for the fronted domain
	token      []byte         // shared pre-auth secret
	fallback   string         // server: real site shown to unauthenticated probes
	serverName string         // client: SNI / verified name
	rootCAs    *x509.CertPool // client: trust anchor
}

func (t *realityTransport) Listener(addr string, wt time.Duration) (valuerpc.Listener, error) {
	base, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	rl := reality.Listener(base, reality.ServerConfig{
		TLSConfig: t.serverTLS,
		Token:     t.token,
		Fallback:  t.fallback,
	})
	return valuerpc.NewAcceptListener(
		func() (io.ReadWriteCloser, error) { return rl.Accept() },
		base.Addr(), rl.Close, wt), nil
}

func (t *realityTransport) Dialer(addr string, wt time.Duration) (valuerpc.Dialer, error) {
	d := reality.Dialer("tcp", addr, reality.ClientConfig{
		TLS:   tlscamo.Config{ServerName: t.serverName, RootCAs: t.rootCAs, Fingerprint: tlscamo.Chrome},
		Token: t.token,
	})
	return valuerpc.NewFuncDialer(func() (io.ReadWriteCloser, error) { return d() }, wt), nil
}

func main() {
	cert, pool := genCert()
	token := []byte("a-32-byte-pre-shared-reality-key") // 32 bytes
	fallback := startFallback()                         // the "real website" probes see

	transport := &realityTransport{
		serverTLS:  &tls.Config{Certificates: []tls.Certificate{cert}}, // ALPN left empty
		token:      token,
		fallback:   fallback,
		serverName: "localhost",
		rootCAs:    pool,
	}

	// --- servion value-rpc server using the reality Transport bean ---
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

	// --- authenticated client (servion value-rpc client + the same Transport) ---
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
	resp, err := cli.CallFunction("greet", value.Utf8("World"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nauthenticated client -> %s\n", resp.String())

	// --- censor-style probe: completes TLS but sends no token -> fallback site ---
	probe, err := tls.Dial("tcp", addr, &tls.Config{ServerName: "localhost", RootCAs: pool})
	if err != nil {
		log.Fatal(err)
	}
	defer probe.Close()
	_, _ = probe.Write([]byte("GET / HTTP/1.1\r\nHost: localhost\r\n\r\n"))
	page, _ := io.ReadAll(io.LimitReader(probe, 256))
	fmt.Printf("active probe (no token) -> %q\n", string(page))
}

// startFallback runs a minimal plaintext "real website" and returns its address.
func startFallback() string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = c.Read(make([]byte, 512)) // consume the request
				_, _ = c.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 21\r\n\r\nFallback real website"))
			}(c)
		}
	}()
	return ln.Addr().String()
}

// genCert builds a self-signed certificate for localhost/127.0.0.1 and a pool that
// trusts it (a real deployment uses a real certificate for the fronted domain).
func genCert() (tls.Certificate, *x509.CertPool) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "localhost"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost"},
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
