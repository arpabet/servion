package servion

import (
	"testing"
)

func TestRunCommand_Factory(t *testing.T) {
	cmd := RunCommand()
	if cmd == nil {
		t.Fatal("RunCommand() returned nil")
	}

	impl := cmd.(*implRunCommand)
	if impl.Command() != "run" {
		t.Errorf("Command() = %q, want run", impl.Command())
	}
}

func TestRunCommand_Help(t *testing.T) {
	cmd := RunCommand()
	impl := cmd.(*implRunCommand)
	short, long := impl.Help()
	if short == "" {
		t.Error("expected non-empty short help")
	}
	if long == "" {
		t.Error("expected non-empty long help")
	}
}

func TestRunCommand_WithBeans(t *testing.T) {
	extra := &testHandler{pattern: "/test"}
	cmd := RunCommand(extra)
	impl := cmd.(*implRunCommand)
	if len(impl.beans) != 1 {
		t.Errorf("beans count = %d, want 1", len(impl.beans))
	}
}
