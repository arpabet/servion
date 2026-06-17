/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 *
 * Self-contained demo of value-rpc over a REALITY-style transport (obfs/xreality),
 * wired into servion through the Transport bean. It runs, in one process: a "real
 * borrowed website", a servion value-rpc server using an xreality Transport, an
 * authenticated client, and a censor-style probe — showing that the authenticated
 * client gets the tunnel while the probe is raw-spliced to the borrowed site and sees
 * that site's genuine certificate.
 *
 *   GOWORK=off go run .
 *
 * This example is a SEPARATE module so its uTLS dependency (via obfs/xreality) never
 * enters servion/vrpc. In production you would split server and client, distribute the
 * server's X25519 public key and the shortId out of band, and point Dest at a real,
 * high-reputation TLS site whose name you borrow.
 */

package main

import (
	"crypto/ecdh"
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
	"go.arpabet.com/obfs"
	"go.arpabet.com/obfs/xreality"
	"go.arpabet.com/servion"
	servionvrpc "go.arpabet.com/servion/vrpc"
	"go.arpabet.com/value"
	"go.arpabet.com/value-rpc/valueclient"
	"go.arpabet.com/value-rpc/valuerpc"
	"go.arpabet.com/value-rpc/valueserver"
)

const borrowedName = "www.realsite.com"

// shaping is the traffic-shaping policy applied INSIDE the REALITY tunnel, on both
// ends symmetrically (same CellSize). REALITY hides the *handshake* (it looks like a
// browser hitting a real site); the shaper hides the *post-handshake traffic shape*
// (per-operation sizes and timing) that handshake camouflage leaves exposed — the two
// are independent layers and the recipe is to run both. Fixed 512-byte cells make
// every value-rpc frame identical on the wire; FRONT adds front-loaded cover padding.
var shaping = obfs.Policy{
	CellSize: 512,
	Front:    &obfs.FrontConfig{Window: 500 * time.Millisecond, MaxCount: 6},
}

// greeterService is the value-rpc service the authenticated tunnel reaches.
type greeterService struct{}

func (greeterService) RegisterValue(srv valueserver.Server) error {
	return srv.AddFunction("greet", valuerpc.String, valuerpc.String, func(args value.Value) (value.Value, error) {
		return value.Utf8("Hello, " + args.String() + "!"), nil
	})
}

// realityTransport implements servionvrpc.Transport, composing obfs/xreality on both
// ends. The heavy uTLS dependency lives in THIS module, not servion/vrpc.
type realityTransport struct {
	priv     *ecdh.PrivateKey // server: static X25519 private key
	pub      []byte           // client: the matching public key
	shortID  []byte           // shared cohort identifier
	serverTL *tls.Config      // server: certificate presented to authenticated clients
	dest     string           // server: real borrowed site probes are spliced to
}

func (t *realityTransport) Listener(addr string, wt time.Duration) (valuerpc.Listener, error) {
	base, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	rl := xreality.Listener(base, xreality.ServerConfig{
		PrivateKey: t.priv,
		ShortIDs:   [][]byte{t.shortID},
		TLSConfig:  t.serverTL,
		Dest:       t.dest,
		TimeSkew:   90 * time.Second,
	})
	return valuerpc.NewAcceptListener(
		func() (io.ReadWriteCloser, error) {
			c, err := rl.Accept()
			if err != nil {
				return nil, err
			}
			return obfs.Wrap(c, shaping), nil // shape INSIDE the REALITY tunnel
		},
		base.Addr(), rl.Close, wt), nil
}

func (t *realityTransport) Dialer(addr string, wt time.Duration) (valuerpc.Dialer, error) {
	d := xreality.Dialer("tcp", addr, xreality.ClientConfig{
		ServerPublicKey: t.pub,
		ShortID:         t.shortID,
		ServerName:      borrowedName,
	})
	return valuerpc.NewFuncDialer(func() (io.ReadWriteCloser, error) {
		c, err := d()
		if err != nil {
			return nil, err
		}
		return obfs.Wrap(c, shaping), nil // shape INSIDE the REALITY tunnel (same policy as the server)
	}, wt), nil
}

func main() {
	priv, err := xreality.GenerateX25519()
	if err != nil {
		log.Fatal(err)
	}
	dest := startBorrowedSite() // the real "website" probes get spliced to

	transport := &realityTransport{
		priv:     priv,
		pub:      priv.PublicKey().Bytes(),
		shortID:  []byte("democoho"), // 8 bytes
		serverTL: &tls.Config{Certificates: []tls.Certificate{genCert(borrowedName)}},
		dest:     dest,
	}

	// --- servion value-rpc server using the xreality Transport bean ---
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
	fmt.Printf("   (value-rpc traffic shaped inside the tunnel: %d-byte cells + FRONT padding)\n", shaping.CellSize)

	// --- censor-style probe: a plain TLS client with no REALITY auth. The server
	// cannot authenticate it, so it is raw-spliced to the borrowed site, and the probe
	// completes TLS against THAT site's genuine certificate. ---
	probe, err := tls.Dial("tcp", addr, &tls.Config{ServerName: borrowedName, InsecureSkipVerify: true})
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

// startBorrowedSite runs a minimal TLS "real website" and returns its address.
func startBorrowedSite() string {
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{genCert(borrowedName)}})
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

// genCert builds a self-signed certificate for name. The authenticated client skips
// CA verification and authenticates the server via the channel-bound HMAC instead, so
// this certificate only needs to exist, not chain to a public CA.
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
