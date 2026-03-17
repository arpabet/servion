package servion

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

func TestAuthTokenProvider_ValidToken(t *testing.T) {
	p := &implAuthTokenProvider{
		allowed: make(map[string]AuthInfo),
		Tokens:  []string{"secret123"},
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	info, err := p.Authenticate("secret123")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	sum := sha256.Sum256([]byte("secret123"))
	expectedHash := hex.EncodeToString(sum[:])

	if info.HashedToken != expectedHash {
		t.Errorf("HashedToken = %q, want %q", info.HashedToken, expectedHash)
	}
	if info.Subject != expectedHash {
		t.Errorf("Subject = %q, want %q", info.Subject, expectedHash)
	}
}

func TestAuthTokenProvider_InvalidToken(t *testing.T) {
	p := &implAuthTokenProvider{
		allowed: make(map[string]AuthInfo),
		Tokens:  []string{"secret123"},
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	_, err := p.Authenticate("wrong-token")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestAuthTokenProvider_MultipleTokens(t *testing.T) {
	p := &implAuthTokenProvider{
		allowed: make(map[string]AuthInfo),
		Tokens:  []string{"token-a", "token-b", "token-c"},
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	for _, tok := range []string{"token-a", "token-b", "token-c"} {
		_, err := p.Authenticate(tok)
		if err != nil {
			t.Errorf("Authenticate(%q) unexpected error: %v", tok, err)
		}
	}

	_, err := p.Authenticate("token-d")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for unknown token, got %v", err)
	}
}

func TestAuthTokenProvider_EmptyTokenSkipped(t *testing.T) {
	p := &implAuthTokenProvider{
		allowed: make(map[string]AuthInfo),
		Tokens:  []string{"", "  ", "valid"},
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	// Only "valid" should be registered
	if len(p.allowed) != 1 {
		t.Errorf("expected 1 allowed token, got %d", len(p.allowed))
	}

	_, err := p.Authenticate("valid")
	if err != nil {
		t.Errorf("Authenticate(valid) unexpected error: %v", err)
	}
}

func TestAuthTokenProvider_CommaInToken(t *testing.T) {
	p := &implAuthTokenProvider{
		allowed: make(map[string]AuthInfo),
		Tokens:  []string{"has,comma"},
	}
	err := p.PostConstruct()
	if err == nil {
		t.Fatal("expected error for token with comma")
	}
}

func TestAuthTokenProvider_Factory(t *testing.T) {
	a := AuthTokenProvider()
	if a == nil {
		t.Fatal("AuthTokenProvider() returned nil")
	}
}
