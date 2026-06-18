/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 *
 * Self-contained demo of value-rpc over a TLS connection whose ClientHello is
 * camouflaged to look like a real browser (obfs/tlscamo), wired into servion
 * through the Transport bean. It runs, in one process: a servion value-rpc server
 * behind a plain TLS listener, and a client whose handshake mimics Chrome's
 * ClientHello (fingerprint, ALPN, extension order) instead of Go's distinctive
 * default — so a censor fingerprinting the TLS handshake sees an ordinary browser.
 *
 *   GOWORK=off go run .
 *
 * This example is a SEPARATE module so its uTLS dependency (via obfs/tlscamo)
 * never enters servion/vrpc. Unlike reality/xreality, tlscamo only camouflages the
 * handshake fingerprint; it is not active-probe-resistant — the server is an
 * ordinary TLS endpoint. In production use a real certificate and borrow a
 * plausible ServerName.
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
	"io"
	"log"
	"math/big"
	"net"
	"time"

	"go.arpabet.com/glue"
	"go.arpabet.com/obfs/tlscamo"
	"go.arpabet.com/servion"
	servionvrpc "go.arpabet.com/servion/vrpc"
	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valueclient"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.arpabet.com/value-rpc/valueserver"
)

const serverName = "localhost"

// greeterService is the value-rpc service reached over the camouflaged TLS tunnel.
type greeterService struct{}

func (greeterService) RegisterValue(srv valueserver.Server) error {
	return srv.AddFunction("greet", valuerpc.String, valuerpc.String, func(ctx context.Context, args value.Value) (value.Value, error) {
		return value.Utf8("Hello, " + args.String() + "!"), nil
	})
}

// tlscamoTransport implements servionvrpc.Transport: the server is an ordinary TLS
// endpoint, while the client dials with a browser-mimicking ClientHello via
// obfs/tlscamo. The heavy uTLS dependency lives in THIS module, not servion/vrpc.
type tlscamoTransport struct {
	serverTLS *tls.Config    // server: certificate presented to clients
	rootCAs   *x509.CertPool // client: trust anchor for the server certificate
}

func (t *tlscamoTransport) Listener(addr string, wt time.Duration) (valuerpc.Listener, error) {
	ln, err := tls.Listen("tcp", addr, t.serverTLS)
	if err != nil {
		return nil, err
	}
	return valuerpc.NewAcceptListener(
		func() (io.ReadWriteCloser, error) { return ln.Accept() },
		ln.Addr(), ln.Close, wt), nil
}

func (t *tlscamoTransport) Dialer(addr string, wt time.Duration) (valuerpc.Dialer, error) {
	// Chrome is the default fingerprint; Roll would rotate browsers per dial.
	dial := tlscamo.Dialer("tcp", addr, tlscamo.Config{
		ServerName:  serverName,
		Fingerprint: tlscamo.Chrome,
		RootCAs:     t.rootCAs,
	})
	return valuerpc.NewFuncDialer(func(ctx context.Context) (io.ReadWriteCloser, error) {
		return dial()
	}, wt), nil
}

func main() {
	cert, pool := genCert()

	transport := &tlscamoTransport{
		serverTLS: &tls.Config{Certificates: []tls.Certificate{cert}},
		rootCAs:   pool,
	}

	// --- servion value-rpc server behind a plain TLS listener ---
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

	// --- client whose ClientHello mimics Chrome (servion client + the Transport) ---
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
	fmt.Printf("\nclient (Chrome-mimicking ClientHello) -> %s\n", resp.String())
}

// genCert builds a self-signed certificate for localhost/127.0.0.1 and a pool that
// trusts it (a real deployment uses a real certificate for the borrowed name).
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
