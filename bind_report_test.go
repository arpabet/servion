/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"errors"
	"net"
	"strings"
	"syscall"
	"testing"

	"go.uber.org/zap"
	"golang.org/x/xerrors"
)

/*
What a failed bind must tell, and to whom.

The audiences never overlap: the zap log is for whoever tails it later, stderr
is for the person at the terminal right now (in production the log is a rotated
file), and the returned error is for the exit code — an application with zero
listeners is not degraded, it is NOT RUNNING, and it used to exit 0 with no
output: the errgroup waited on an empty set, Wait returned nil, and the process
ended looking exactly like a successful daemonization. An operator who started
a second instance by accident read a healthy banner and got their prompt back.
*/

type fakeServer struct {
	bindErr error
	bound   bool
}

func (f *fakeServer) PostConstruct() error        { return nil }
func (f *fakeServer) Destroy() error              { return nil }
func (f *fakeServer) Bind() error                 { f.bound = f.bindErr == nil; return f.bindErr }
func (f *fakeServer) Alive() bool                 { return f.bound }
func (f *fakeServer) ListenAddress() net.Addr     { return EmptyAddr }
func (f *fakeServer) Serve() error                { return nil }
func (f *fakeServer) Shutdown() error             { return nil }
func (f *fakeServer) ShutdownCh() <-chan struct{} { return nil }

func addrInUse() error {
	return xerrors.Errorf("can not bind to port ':8480': %w",
		&net.OpError{Op: "listen", Err: syscall.EADDRINUSE})
}

func TestNothingBoundIsAnErrorNotASilentExit(t *testing.T) {
	var stderr strings.Builder
	servers := []Server{
		&fakeServer{bindErr: addrInUse()},
		&fakeServer{bindErr: addrInUse()},
	}
	bound, err := bindServers(servers, zap.NewNop(), &stderr)
	if err == nil {
		t.Fatal("every bind failed and bindServers reported success — the process would exit 0 looking like a daemonized start")
	}
	if len(bound) != 0 {
		t.Fatalf("bound = %d", len(bound))
	}
	if !strings.Contains(err.Error(), "no server could bind (2 of 2 failed)") {
		t.Fatalf("the error does not say what happened: %v", err)
	}
	// EADDRINUSE means, in practice, a second instance. Say so: the raw errno
	// tells an operator what happened without telling them what it means.
	if !strings.Contains(err.Error(), "another instance already running") {
		t.Fatalf("the likely cause is not named: %v", err)
	}
	// And the wrapped cause survives for errors.Is callers.
	if !errors.Is(err, syscall.EADDRINUSE) {
		t.Fatalf("the underlying errno was lost: %v", err)
	}
	if !strings.Contains(stderr.String(), "can not bind to port ':8480'") {
		t.Fatalf("nothing reached the terminal: %q", stderr.String())
	}
}

/*
TestPartialFailureKeepsServing pins the other half, which is just as
deliberate: one dead port must not kill the application, because the admin
surface that DID bind is how the operator fixes the port that did not. The
first-run onboarding flow depends on this — it boots with a known-bad port
precisely so the wizard can heal it.
*/
func TestPartialFailureKeepsServing(t *testing.T) {
	var stderr strings.Builder
	ok := &fakeServer{}
	servers := []Server{&fakeServer{bindErr: addrInUse()}, ok}
	bound, err := bindServers(servers, zap.NewNop(), &stderr)
	if err != nil {
		t.Fatalf("one healthy listener must be enough to run: %v", err)
	}
	if len(bound) != 1 || bound[0] != Server(ok) {
		t.Fatalf("bound = %v", bound)
	}
	// The person at the terminal is told both facts: what failed, and that the
	// application is running anyway so the failure can be repaired.
	out := stderr.String()
	if !strings.Contains(out, "can not bind") || !strings.Contains(out, "continuing with the rest") {
		t.Fatalf("stderr does not tell the story: %q", out)
	}
}

// A clean bind writes NOTHING to stderr: warnings that appear on every healthy
// start are warnings nobody reads.
func TestAHealthyStartIsSilent(t *testing.T) {
	var stderr strings.Builder
	if _, err := bindServers([]Server{&fakeServer{}}, zap.NewNop(), &stderr); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("a healthy start wrote %q — noise on the ordinary day teaches operators to ignore the channel", stderr.String())
	}
}
