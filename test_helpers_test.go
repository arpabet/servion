package servion

import (
	"context"
	"sync"
	"time"
)

// mockRuntime implements Runtime for testing.
type mockRuntime struct {
	active     bool
	restarting bool
	shutdownCh chan struct{}
	closedOnce sync.Once
	stats      map[string]string
	err        error
}

func newMockRuntime(active bool) *mockRuntime {
	return &mockRuntime{
		active:     active,
		shutdownCh: make(chan struct{}),
		stats:      make(map[string]string),
	}
}

func (m *mockRuntime) PostConstruct() error { return nil }
func (m *mockRuntime) BeanName() string     { return "test-runtime" }
func (m *mockRuntime) Executable() string   { return "test" }
func (m *mockRuntime) HomeDir() string      { return "/tmp" }
func (m *mockRuntime) Active() bool         { return m.active }
func (m *mockRuntime) Restarting() bool     { return m.restarting }

func (m *mockRuntime) Shutdown(restart bool) {
	m.closedOnce.Do(func() {
		m.active = false
		m.restarting = restart
		m.err = context.Canceled
		close(m.shutdownCh)
	})
}

func (m *mockRuntime) GetStats(cb func(name, value string) bool) error {
	for k, v := range m.stats {
		if !cb(k, v) {
			break
		}
	}
	return nil
}

func (m *mockRuntime) Deadline() (time.Time, bool)       { return time.Time{}, false }
func (m *mockRuntime) Done() <-chan struct{}              { return m.shutdownCh }
func (m *mockRuntime) Err() error                        { return m.err }
func (m *mockRuntime) Value(key interface{}) interface{} { return nil }

// mockComponent implements Component for testing.
type mockComponent struct {
	name  string
	stats map[string]string
}

func (m *mockComponent) BeanName() string { return m.name }
func (m *mockComponent) GetStats(cb func(name, value string) bool) error {
	for k, v := range m.stats {
		if !cb(k, v) {
			break
		}
	}
	return nil
}

// mockAuthenticator implements Authenticator for testing.
type mockAuthenticator struct {
	authFunc func(token string) (AuthInfo, error)
}

func (m *mockAuthenticator) Authenticate(token string) (AuthInfo, error) {
	return m.authFunc(token)
}
