/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 *
 * Self-contained demo of value-rpc over an Xray-compatible REALITY transport
 * (obfs/xrayreality), wired into servion through the Transport bean. The server is
 * the genuine xtls/reality server — the exact code real Xray runs — so a client
 * built here is wire-compatible with Xray. It runs, in one process: a "real
 * borrowed website" (TLS 1.3), a servion value-rpc server using an xrayreality
 * Transport, an authenticated client, and a censor-style probe — showing that the
 * authenticated client gets the tunnel while the probe is relayed to the borrowed
 * site and sees THAT site's genuine certificate.
 *
 *   GOWORK=off go run .
 *
 * This example is a SEPARATE module so its uTLS / xtls-reality dependencies (via
 * obfs/xrayreality) never enter servion/vrpc. In production you would split server
 * and client, distribute the server's X25519 public key and the shortId out of
 * band, and point Dest at a real, high-reputation TLS 1.3 site whose name you borrow.
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

	reality "github.com/xtls/reality"
	"go.arpabet.com/glue"
	"go.arpabet.com/obfs/xrayreality"
	"go.arpabet.com/servion"
	servionvrpc "go.arpabet.com/servion/vrpc"
	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valueclient"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.arpabet.com/value-rpc/valueserver"
)

const borrowedName = "www.realsite.com"

// greeterService is the value-rpc service the authenticated tunnel reaches.
type greeterService struct{}

func (greeterService) RegisterValue(srv valueserver.Server) error {
	return srv.AddFunction("greet", valuerpc.String, valuerpc.String, func(ctx context.Context, args value.Value) (value.Value, error) {
		return value.Utf8("Hello, " + args.String() + "!"), nil
	})
}

// xrayTransport implements servionvrpc.Transport, composing obfs/xrayreality on both
// ends. The heavy uTLS / xtls-reality dependencies live in THIS module, not
// servion/vrpc.
type xrayTransport struct {
	priv    []byte // server: static X25519 private key (raw 32 bytes)
	pub     []byte // client: the matching public key
	shortID []byte // shared cohort identifier (<= 8 bytes)
	dest    string // server: real borrowed TLS 1.3 site probes are relayed to
}

func (t *xrayTransport) Listener(addr string, wt time.Duration) (valuerpc.Listener, error) {
	base, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	rl := xrayreality.Listener(base, xrayreality.ServerConfig{
		PrivateKey:  t.priv,
		ShortIDs:    [][]byte{t.shortID},
		ServerNames: []string{borrowedName},
		Dest:        t.dest,
		MaxTimeDiff: time.Minute,
	})
	return valuerpc.NewAcceptListener(
		func() (io.ReadWriteCloser, error) { return rl.Accept() },
		base.Addr(), rl.Close, wt), nil
}

func (t *xrayTransport) Dialer(addr string, wt time.Duration) (valuerpc.Dialer, error) {
	d := xrayreality.Dialer("tcp", addr, xrayreality.ClientConfig{
		PublicKey:  t.pub,
		ShortID:    t.shortID,
		ServerName: borrowedName,
	})
	return valuerpc.NewFuncDialer(func(ctx context.Context) (io.ReadWriteCloser, error) { return d() }, wt), nil
}

func main() {
	priv, pub, err := xrayreality.GenerateKeyPair()
	if err != nil {
		log.Fatal(err)
	}
	dest := startBorrowedSite() // the real TLS 1.3 "website" probes get relayed to

	transport := &xrayTransport{
		priv:    priv,
		pub:     pub,
		shortID: []byte("xraydemo"), // 8 bytes
		dest:    dest,
	}

	// --- servion value-rpc server using the xrayreality Transport bean ---
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
	if err := srv.Bind(); err != nil { // Bind builds the xrayreality.Listener, kicking off Dest detection
		log.Fatal(err)
	}
	go srv.Serve()
	defer srv.Shutdown()
	addr := srv.ListenAddress().String()

	// The xtls/reality library probes Dest once to learn its post-handshake record
	// lengths; the server handshake stalls until that completes, so wait for it before
	// the client connects.
	waitForDetection(dest)

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
	resp, err := cli.CallFunction(context.Background(), "greet", value.Utf8("World"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nauthenticated client (Xray-compatible) -> %s\n", resp.String())

	// --- censor-style probe: a plain TLS client with no REALITY auth. The genuine
	// server cannot authenticate it, so it is relayed to the borrowed site, and the
	// probe completes TLS against THAT site's genuine certificate. ---
	probe, err := tls.Dial("tcp", addr, &tls.Config{ServerName: borrowedName, InsecureSkipVerify: true, MinVersion: tls.VersionTLS13})
	if err != nil {
		log.Fatal(err)
	}
	defer probe.Close()
	_, _ = probe.Write([]byte("GET / HTTP/1.1\r\nHost: " + borrowedName + "\r\n\r\n"))
	page, _ := io.ReadAll(io.LimitReader(probe, 256))
	fmt.Printf("active probe (no auth) -> %q\n", string(page))
	if certs := probe.ConnectionState().PeerCertificates; len(certs) > 0 {
		fmt.Printf("active probe sees certificate for: %s\n", certs[0].Subject.CommonName)
	}
}

// waitForDetection blocks until the xtls/reality library's per-Dest post-handshake
// record-length detection (kicked off when the Listener is built) has finished for all
// ALPN variants, so the server's handshake does not stall waiting for it.
func waitForDetection(dest string) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ready := true
		for _, alpn := range []string{"0", "1", "2"} {
			v, ok := reality.GlobalPostHandshakeRecordsLens.Load(dest + " " + borrowedName + " " + alpn)
			if !ok {
				ready = false
				break
			}
			if _, placeholder := v.(bool); placeholder {
				ready = false
				break
			}
		}
		if ready {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// startBorrowedSite runs a minimal TLS 1.3 "real website" and returns its address.
func startBorrowedSite() string {
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{genCert(borrowedName)},
		MinVersion:   tls.VersionTLS13,
	})
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
				_, _ = c.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 21\r\n\r\nBorrowed real website"))
			}(c)
		}
	}()
	return ln.Addr().String()
}

// genCert builds a self-signed certificate for name (used by the borrowed site). The
// authenticated client verifies the server via REALITY's channel-bound HMAC, not this
// certificate, so it only needs to exist.
func genCert(name string) tls.Certificate {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{name},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		log.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(der)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}
}
