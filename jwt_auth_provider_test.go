package servion

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func makeHmacToken(t *testing.T, secret string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign HMAC token: %v", err)
	}
	return s
}

// generateECDSAKeys returns a private key and the public key as a base64-encoded DER string
// (raw key content without PEM header/footer).
func generateECDSAKeys(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ECDSA key: %v", err)
	}

	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	return priv, base64.StdEncoding.EncodeToString(der)
}

func TestJwtAuth_HmacValidToken(t *testing.T) {
	secret := "test-secret-key-at-least-32-chars"

	p := &implJwtAuthProvider{
		Secret:      secret,
		RolesClaim:  "roles",
		ScopesClaim: "scope",
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	tokenStr := makeHmacToken(t, secret, jwt.MapClaims{
		"sub":   "user-123",
		"iss":   "test-issuer",
		"email": "user@example.com",
		"name":  "John Doe",
		"roles": []interface{}{"admin", "user"},
		"scope": "read write",
		"exp":   jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})

	info, err := p.Authenticate(tokenStr)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	if info.Subject != "user-123" {
		t.Errorf("Subject = %q, want %q", info.Subject, "user-123")
	}
	if info.Issuer != "test-issuer" {
		t.Errorf("Issuer = %q, want %q", info.Issuer, "test-issuer")
	}
	if len(info.Roles) != 2 || info.Roles[0] != "admin" || info.Roles[1] != "user" {
		t.Errorf("Roles = %v, want [admin user]", info.Roles)
	}
	if len(info.Scopes) != 2 || info.Scopes[0] != "read" || info.Scopes[1] != "write" {
		t.Errorf("Scopes = %v, want [read write]", info.Scopes)
	}
	if info.Attributes["email"] != "user@example.com" {
		t.Errorf("Attributes[email] = %q, want %q", info.Attributes["email"], "user@example.com")
	}
	if info.Attributes["name"] != "John Doe" {
		t.Errorf("Attributes[name] = %q, want %q", info.Attributes["name"], "John Doe")
	}
	if info.HashedToken == "" {
		t.Error("HashedToken should not be empty")
	}
}

func TestJwtAuth_HmacExpiredToken(t *testing.T) {
	secret := "test-secret-key-at-least-32-chars"

	p := &implJwtAuthProvider{
		Secret:      secret,
		RolesClaim:  "roles",
		ScopesClaim: "scope",
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	tokenStr := makeHmacToken(t, secret, jwt.MapClaims{
		"sub": "user-123",
		"exp": jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	})

	_, err := p.Authenticate(tokenStr)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for expired token, got %v", err)
	}
}

func TestJwtAuth_HmacMissingExp(t *testing.T) {
	secret := "test-secret-key-at-least-32-chars"

	p := &implJwtAuthProvider{
		Secret:      secret,
		RolesClaim:  "roles",
		ScopesClaim: "scope",
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	tokenStr := makeHmacToken(t, secret, jwt.MapClaims{
		"sub": "user-123",
	})

	_, err := p.Authenticate(tokenStr)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for missing exp, got %v", err)
	}
}

func TestJwtAuth_HmacWrongSecret(t *testing.T) {
	p := &implJwtAuthProvider{
		Secret:      "correct-secret-key-12345678901234",
		RolesClaim:  "roles",
		ScopesClaim: "scope",
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	tokenStr := makeHmacToken(t, "wrong-secret-key-123456789012345", jwt.MapClaims{
		"sub": "user-123",
		"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})

	_, err := p.Authenticate(tokenStr)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for wrong secret, got %v", err)
	}
}

func TestJwtAuth_HmacIssuerValidation(t *testing.T) {
	secret := "test-secret-key-at-least-32-chars"

	p := &implJwtAuthProvider{
		Secret:      secret,
		Issuer:      "expected-issuer",
		RolesClaim:  "roles",
		ScopesClaim: "scope",
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	// Valid issuer
	tokenStr := makeHmacToken(t, secret, jwt.MapClaims{
		"sub": "user-123",
		"iss": "expected-issuer",
		"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	_, err := p.Authenticate(tokenStr)
	if err != nil {
		t.Fatalf("Authenticate valid issuer: %v", err)
	}

	// Wrong issuer
	tokenStr = makeHmacToken(t, secret, jwt.MapClaims{
		"sub": "user-123",
		"iss": "wrong-issuer",
		"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	_, err = p.Authenticate(tokenStr)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for wrong issuer, got %v", err)
	}
}

func TestJwtAuth_HmacAudienceValidation(t *testing.T) {
	secret := "test-secret-key-at-least-32-chars"

	p := &implJwtAuthProvider{
		Secret:      secret,
		Audience:    "my-api",
		RolesClaim:  "roles",
		ScopesClaim: "scope",
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	// Valid audience
	tokenStr := makeHmacToken(t, secret, jwt.MapClaims{
		"sub": "user-123",
		"aud": "my-api",
		"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	_, err := p.Authenticate(tokenStr)
	if err != nil {
		t.Fatalf("Authenticate valid audience: %v", err)
	}

	// Wrong audience
	tokenStr = makeHmacToken(t, secret, jwt.MapClaims{
		"sub": "user-123",
		"aud": "wrong-api",
		"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	_, err = p.Authenticate(tokenStr)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for wrong audience, got %v", err)
	}
}

func TestJwtAuth_ECDSAValidToken(t *testing.T) {
	priv, pubPEM := generateECDSAKeys(t)

	p := &implJwtAuthProvider{
		PublicKeyB64: pubPEM,
		RolesClaim:   "roles",
		ScopesClaim:  "scope",
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"sub":   "user-456",
		"roles": []interface{}{"editor"},
		"exp":   jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	tokenStr, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("sign ECDSA token: %v", err)
	}

	info, err := p.Authenticate(tokenStr)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	if info.Subject != "user-456" {
		t.Errorf("Subject = %q, want %q", info.Subject, "user-456")
	}
	if len(info.Roles) != 1 || info.Roles[0] != "editor" {
		t.Errorf("Roles = %v, want [editor]", info.Roles)
	}
}

func TestJwtAuth_ECDSARejectsHmacToken(t *testing.T) {
	_, pubPEM := generateECDSAKeys(t)

	p := &implJwtAuthProvider{
		PublicKeyB64: pubPEM,
		RolesClaim:   "roles",
		ScopesClaim:  "scope",
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	tokenStr := makeHmacToken(t, "some-secret-key-12345678901234567", jwt.MapClaims{
		"sub": "user-123",
		"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})

	_, err := p.Authenticate(tokenStr)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for HMAC token on ECDSA provider, got %v", err)
	}
}

func TestJwtAuth_ECDSAWrongKey(t *testing.T) {
	// Sign with one key, verify with another
	privSign, _ := generateECDSAKeys(t)
	_, pubPEMVerify := generateECDSAKeys(t)

	p := &implJwtAuthProvider{
		PublicKeyB64: pubPEMVerify,
		RolesClaim:   "roles",
		ScopesClaim:  "scope",
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"sub": "user-123",
		"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	tokenStr, err := token.SignedString(privSign)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	_, err = p.Authenticate(tokenStr)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for wrong ECDSA key, got %v", err)
	}
}

func TestJwtAuth_RolesAsCommaSeparatedString(t *testing.T) {
	secret := "test-secret-key-at-least-32-chars"

	p := &implJwtAuthProvider{
		Secret:      secret,
		RolesClaim:  "roles",
		ScopesClaim: "scope",
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	tokenStr := makeHmacToken(t, secret, jwt.MapClaims{
		"sub":   "user-123",
		"roles": "admin,editor,viewer",
		"exp":   jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})

	info, err := p.Authenticate(tokenStr)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	if len(info.Roles) != 3 {
		t.Fatalf("Roles len = %d, want 3", len(info.Roles))
	}
	if info.Roles[0] != "admin" || info.Roles[1] != "editor" || info.Roles[2] != "viewer" {
		t.Errorf("Roles = %v, want [admin editor viewer]", info.Roles)
	}
}

func TestJwtAuth_ScopesAsArray(t *testing.T) {
	secret := "test-secret-key-at-least-32-chars"

	p := &implJwtAuthProvider{
		Secret:      secret,
		RolesClaim:  "roles",
		ScopesClaim: "scope",
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	tokenStr := makeHmacToken(t, secret, jwt.MapClaims{
		"sub":   "user-123",
		"scope": []interface{}{"read", "write", "delete"},
		"exp":   jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})

	info, err := p.Authenticate(tokenStr)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	if len(info.Scopes) != 3 {
		t.Fatalf("Scopes len = %d, want 3", len(info.Scopes))
	}
}

func TestJwtAuth_CustomClaimNames(t *testing.T) {
	secret := "test-secret-key-at-least-32-chars"

	p := &implJwtAuthProvider{
		Secret:      secret,
		RolesClaim:  "realm_access",
		ScopesClaim: "permissions",
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	tokenStr := makeHmacToken(t, secret, jwt.MapClaims{
		"sub":          "user-123",
		"realm_access": []interface{}{"admin"},
		"permissions":  "read write",
		"exp":          jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})

	info, err := p.Authenticate(tokenStr)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	if len(info.Roles) != 1 || info.Roles[0] != "admin" {
		t.Errorf("Roles = %v, want [admin]", info.Roles)
	}
	if len(info.Scopes) != 2 || info.Scopes[0] != "read" || info.Scopes[1] != "write" {
		t.Errorf("Scopes = %v, want [read write]", info.Scopes)
	}
}

func TestJwtAuth_GarbageToken(t *testing.T) {
	p := &implJwtAuthProvider{
		Secret:      "test-secret-key-at-least-32-chars",
		RolesClaim:  "roles",
		ScopesClaim: "scope",
	}
	if err := p.PostConstruct(); err != nil {
		t.Fatalf("PostConstruct: %v", err)
	}

	_, err := p.Authenticate("not.a.jwt")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for garbage token, got %v", err)
	}
}

func TestJwtAuth_PostConstruct_NoConfig(t *testing.T) {
	p := &implJwtAuthProvider{}
	err := p.PostConstruct()
	if err == nil {
		t.Fatal("expected error when neither secret nor public-key configured")
	}
}

func TestJwtAuth_PostConstruct_BothConfigured(t *testing.T) {
	_, pubPEM := generateECDSAKeys(t)
	p := &implJwtAuthProvider{
		Secret:       "some-secret",
		PublicKeyB64: pubPEM,
	}
	err := p.PostConstruct()
	if err == nil {
		t.Fatal("expected error when both secret and public-key configured")
	}
}

func TestJwtAuth_PostConstruct_InvalidBase64(t *testing.T) {
	p := &implJwtAuthProvider{
		PublicKeyB64: "not-valid-base64!!!",
	}
	err := p.PostConstruct()
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestJwtAuth_PostConstruct_InvalidDER(t *testing.T) {
	// Valid base64, but not a valid DER-encoded public key
	p := &implJwtAuthProvider{
		PublicKeyB64: base64.StdEncoding.EncodeToString([]byte("not a real key")),
	}
	err := p.PostConstruct()
	if err == nil {
		t.Fatal("expected error for invalid DER content")
	}
}

func TestJwtAuth_Factory(t *testing.T) {
	a := JwtAuthProvider()
	if a == nil {
		t.Fatal("JwtAuthProvider() returned nil")
	}
}
