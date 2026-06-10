package servion

import (
	"testing"

	"go.uber.org/zap"
)

func TestZapLogFactory_Object(t *testing.T) {
	f := ZapLogFactory(true)

	obj, err := f.Object()
	if err != nil {
		t.Fatalf("Object: %v", err)
	}

	logger, ok := obj.(*zap.Logger)
	if !ok {
		t.Fatalf("expected *zap.Logger, got %T", obj)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestZapLogFactory_ObjectType(t *testing.T) {
	f := ZapLogFactory(true)
	if f.ObjectType() != ZapLogClass {
		t.Errorf("ObjectType = %v, want %v", f.ObjectType(), ZapLogClass)
	}
}

func TestZapLogFactory_ObjectName(t *testing.T) {
	f := ZapLogFactory(true)
	if f.ObjectName() != "zap_logger" {
		t.Errorf("ObjectName = %q, want zap_logger", f.ObjectName())
	}
}

func TestZapLogFactory_Production(t *testing.T) {
	f := ZapLogFactory(false)

	obj, err := f.Object()
	if err != nil {
		t.Fatalf("Object: %v", err)
	}

	logger, ok := obj.(*zap.Logger)
	if !ok {
		t.Fatalf("expected *zap.Logger, got %T", obj)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestZapLogFactory_Singleton(t *testing.T) {
	f := ZapLogFactory(true)
	if !f.Singleton() {
		t.Error("expected Singleton() = true")
	}
}
