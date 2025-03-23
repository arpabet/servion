/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"github.com/pkg/errors"
	"go.arpabet.com/cligo"
	"go.arpabet.com/servion/servionapi"
	"go.uber.org/atomic"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type implRuntime struct {
	CliApplication cligo.CliApplication `inject:""`

	profile    string
	runtimeErr atomic.Error

	executable    string
	executableDir string
	homeDir       string

	shuttingDown atomic.Bool
	shutdownCh   chan struct{} // sends only close channel event
	restarting   atomic.Bool
	shutdownOnce sync.Once
}

func NewRuntime(profile string, homeDir string) servionapi.Runtime {
	t := &implRuntime{
		profile:    profile,
		homeDir:    homeDir,
		shutdownCh: make(chan struct{}),
	}
	t.runtimeErr.Store(nil)
	return t
}

func (t *implRuntime) BeanName() string {
	return "runtime"
}

func (t *implRuntime) GetStats(cb func(name, value string) bool) error {
	cb("name", t.CliApplication.Name())
	cb("version", t.CliApplication.Version())
	cb("build", t.CliApplication.Build())

	cb("executable", t.executable)
	cb("home", t.homeDir)
	cb("profile", t.profile)
	return nil
}

// PostConstruct implements glue.InitializingBean
func (t *implRuntime) PostConstruct() (err error) {

	defer PanicToError(&err)

	absHomeDir, err := filepath.Abs(t.homeDir)
	if err != nil {
		return errors.Errorf("failed to get abs home directory: %s, %v", t.homeDir, err)
	}
	t.homeDir = absHomeDir

	t.executable = os.Args[0]
	t.executableDir, err = filepath.Abs(filepath.Dir(t.executable))
	if err != nil {
		return err
	}
	t.executable = filepath.Base(t.executable)
	if t.homeDir == "." {
		t.homeDir, err = filepath.Abs(t.homeDir)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *implRuntime) Profile() string {
	return t.profile
}

func (t *implRuntime) Executable() string {
	return t.executable
}

func (t *implRuntime) HomeDir() string {
	return t.homeDir
}

func (t *implRuntime) Active() bool {
	return !t.shuttingDown.Load()
}

func (t *implRuntime) Shutdown(restart bool) {
	t.shutdownOnce.Do(func() {
		t.restarting.Store(restart)
		t.shuttingDown.Store(true)
		t.runtimeErr.Store(errors.New("closed"))
		close(t.shutdownCh)
	})
}

func (t *implRuntime) Restarting() bool {
	return t.restarting.Load()
}

func (t *implRuntime) Deadline() (deadline time.Time, ok bool) {
	return time.Now(), false
}

func (t *implRuntime) Value(key interface{}) interface{} {
	return nil
}

func (t *implRuntime) Done() <-chan struct{} {
	return t.shutdownCh
}

func (t *implRuntime) Err() error {
	return t.runtimeErr.Load()
}
