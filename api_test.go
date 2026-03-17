package servion

import (
	"context"
	"testing"
)

func TestAuthFromContext_Present(t *testing.T) {
	info := AuthInfo{
		Subject: "user1",
		Scopes:  []string{"read", "write"},
		Roles:   []string{"admin"},
	}
	ctx := context.WithValue(context.Background(), authContextKey, info)

	got, ok := AuthFromContext(ctx)
	if !ok {
		t.Fatal("expected auth info in context")
	}
	if got.Subject != "user1" {
		t.Errorf("Subject = %q, want %q", got.Subject, "user1")
	}
	if len(got.Scopes) != 2 {
		t.Errorf("Scopes = %v, want 2 elements", got.Scopes)
	}
	if len(got.Roles) != 1 || got.Roles[0] != "admin" {
		t.Errorf("Roles = %v, want [admin]", got.Roles)
	}
}

func TestAuthFromContext_Missing(t *testing.T) {
	_, ok := AuthFromContext(context.Background())
	if ok {
		t.Error("expected no auth info in empty context")
	}
}

func TestEmptyAddr(t *testing.T) {
	if got := EmptyAddr.Network(); got != "" {
		t.Errorf("Network() = %q, want empty", got)
	}
	if got := EmptyAddr.String(); got != "" {
		t.Errorf("String() = %q, want empty", got)
	}
}
