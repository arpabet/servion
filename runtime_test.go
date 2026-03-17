package servion

import (
	"testing"
	"time"
)

func TestRuntime_InitialState(t *testing.T) {
	rt := NewRuntime("/tmp/test")

	if !rt.Active() {
		t.Error("expected Active() = true initially")
	}
	if rt.Restarting() {
		t.Error("expected Restarting() = false initially")
	}
}

func TestRuntime_BeanName(t *testing.T) {
	rt := NewRuntime("/tmp")
	if got := rt.BeanName(); got != "runtime" {
		t.Errorf("BeanName() = %q, want runtime", got)
	}
}

func TestRuntime_Shutdown(t *testing.T) {
	rt := NewRuntime("/tmp")

	rt.Shutdown(false)

	if rt.Active() {
		t.Error("expected Active() = false after shutdown")
	}
	if rt.Restarting() {
		t.Error("expected Restarting() = false for non-restart shutdown")
	}

	// Done channel should be closed
	select {
	case <-rt.Done():
		// expected
	case <-time.After(time.Second):
		t.Fatal("Done() channel not closed after shutdown")
	}

	// Err should be non-nil
	if rt.Err() == nil {
		t.Error("expected non-nil Err() after shutdown")
	}
}

func TestRuntime_ShutdownWithRestart(t *testing.T) {
	rt := NewRuntime("/tmp")

	rt.Shutdown(true)

	if rt.Active() {
		t.Error("expected Active() = false after shutdown")
	}
	if !rt.Restarting() {
		t.Error("expected Restarting() = true for restart shutdown")
	}
}

func TestRuntime_DoubleShutdown(t *testing.T) {
	rt := NewRuntime("/tmp")

	rt.Shutdown(false)
	// Second shutdown should not panic
	rt.Shutdown(true)

	// First shutdown wins — restarting should be false
	if rt.Restarting() {
		t.Error("expected first Shutdown(false) to take effect, not the second")
	}
}

func TestRuntime_DoneBeforeShutdown(t *testing.T) {
	rt := NewRuntime("/tmp")

	select {
	case <-rt.Done():
		t.Fatal("Done() should not be closed before shutdown")
	default:
		// expected
	}
}

func TestRuntime_Deadline(t *testing.T) {
	rt := NewRuntime("/tmp")
	_, ok := rt.Deadline()
	if ok {
		t.Error("expected Deadline() ok = false")
	}
}

func TestRuntime_Value(t *testing.T) {
	rt := NewRuntime("/tmp")
	if v := rt.Value("anything"); v != nil {
		t.Errorf("Value() = %v, want nil", v)
	}
}

func TestRuntime_GetStats(t *testing.T) {
	// GetStats requires CliApplication which is injected.
	// Test via mock runtime instead.
	mr := newMockRuntime(true)
	mr.stats["version"] = "1.0.0"

	var collected []string
	err := mr.GetStats(func(name, value string) bool {
		collected = append(collected, name+"="+value)
		return true
	})
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if len(collected) == 0 {
		t.Error("expected at least one stat")
	}
}

func TestRuntime_HomeDir(t *testing.T) {
	rt := NewRuntime("/opt/myapp")
	impl := rt.(*implRuntime)
	if impl.homeDir != "/opt/myapp" {
		t.Errorf("homeDir = %q, want /opt/myapp", impl.homeDir)
	}
}

func TestRuntime_Executable(t *testing.T) {
	rt := NewRuntime("/tmp")
	impl := rt.(*implRuntime)
	// Before PostConstruct, executable is empty
	if impl.Executable() != "" {
		t.Errorf("Executable() = %q before PostConstruct, want empty", impl.Executable())
	}
}

func TestRuntime_HomeDirMethod(t *testing.T) {
	rt := NewRuntime("/opt/test")
	if rt.HomeDir() != "/opt/test" {
		t.Errorf("HomeDir() = %q, want /opt/test", rt.HomeDir())
	}
}

func TestRuntime_ErrBeforeShutdown(t *testing.T) {
	rt := NewRuntime("/tmp")
	impl := rt.(*implRuntime)
	if impl.Err() != nil {
		t.Error("expected nil Err() before shutdown")
	}
}
