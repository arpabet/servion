/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type implJwtAuthProvider struct {
	// HMAC shared secret (used for HS256/HS384/HS512)
	Secret string `value:"jwt.secret,default="`

	// ECDSA public key as base64-encoded DER (PKIX format), used for ES256/ES384/ES512.
	// This is the raw key content without PEM header/footer lines.
	PublicKeyB64 string `value:"jwt.public-key,default="`

	// Expected issuer (optional, validated if set)
	Issuer string `value:"jwt.issuer,default="`

	// Expected audience (optional, validated if set)
	Audience string `value:"jwt.audience,default="`

	// JWT claim name for roles (default "roles")
	RolesClaim string `value:"jwt.roles-claim,default=roles"`

	// JWT claim name for scopes (default "scope")
	ScopesClaim string `value:"jwt.scopes-claim,default=scope"`

	keyFunc  jwt.Keyfunc
	ecdsaPub *ecdsa.PublicKey
}

// JwtAuthProvider creates a JWT-based Authenticator.
//
// Configuration properties:
//
//	jwt.secret       – HMAC shared secret for HS* algorithms
//	jwt.public-key   – ECDSA public key as base64 string (DER/PKIX content without PEM header/footer)
//	jwt.issuer       – expected issuer claim (optional)
//	jwt.audience     – expected audience claim (optional)
//	jwt.roles-claim  – claim name containing roles (default "roles")
//	jwt.scopes-claim – claim name containing scopes (default "scope")
func JwtAuthProvider() Authenticator {
	return &implJwtAuthProvider{}
}

func (t *implJwtAuthProvider) PostConstruct() error {
	if t.Secret == "" && t.PublicKeyB64 == "" {
		return fmt.Errorf("jwt: either jwt.secret or jwt.public-key must be configured")
	}

	if t.Secret != "" && t.PublicKeyB64 != "" {
		return fmt.Errorf("jwt: jwt.secret and jwt.public-key are mutually exclusive")
	}

	if t.PublicKeyB64 != "" {
		der, err := base64.StdEncoding.DecodeString(t.PublicKeyB64)
		if err != nil {
			return fmt.Errorf("jwt: failed to base64-decode jwt.public-key: %w", err)
		}
		pub, err := x509.ParsePKIXPublicKey(der)
		if err != nil {
			return fmt.Errorf("jwt: failed to parse public key: %w", err)
		}
		ecPub, ok := pub.(*ecdsa.PublicKey)
		if !ok {
			return fmt.Errorf("jwt: public key is not ECDSA, got %T", pub)
		}
		t.ecdsaPub = ecPub
		t.keyFunc = func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
				return nil, fmt.Errorf("jwt: unexpected signing method %v", token.Header["alg"])
			}
			return t.ecdsaPub, nil
		}
	} else {
		t.keyFunc = func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("jwt: unexpected signing method %v", token.Header["alg"])
			}
			return []byte(t.Secret), nil
		}
	}

	return nil
}

func (t *implJwtAuthProvider) Authenticate(tokenStr string) (AuthInfo, error) {
	opts := []jwt.ParserOption{jwt.WithExpirationRequired()}
	if t.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(t.Issuer))
	}
	if t.Audience != "" {
		opts = append(opts, jwt.WithAudience(t.Audience))
	}

	token, err := jwt.Parse(tokenStr, t.keyFunc, opts...)
	if err != nil {
		return AuthInfo{}, ErrUnauthorized
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return AuthInfo{}, ErrUnauthorized
	}

	info := AuthInfo{
		HashedToken: hashToken(tokenStr),
		Subject:     claimString(claims, "sub"),
		Issuer:      claimString(claims, "iss"),
		Roles:       claimStringSlice(claims, t.RolesClaim),
		Scopes:      parseScopeClaim(claims, t.ScopesClaim),
		Attributes:  make(map[string]string),
	}

	// populate common claims as attributes
	for _, key := range []string{"email", "name", "preferred_username", "jti"} {
		if v := claimString(claims, key); v != "" {
			info.Attributes[key] = v
		}
	}

	return info, nil
}

func claimString(claims jwt.MapClaims, key string) string {
	v, ok := claims[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func claimStringSlice(claims jwt.MapClaims, key string) []string {
	v, ok := claims[key]
	if !ok {
		return nil
	}

	// ["admin", "user"]
	if arr, ok := v.([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}

	// "admin,user" or "admin user"
	if s, ok := v.(string); ok {
		return splitClaimString(s)
	}

	return nil
}

// parseScopeClaim handles the "scope" claim which is typically space-delimited per RFC 6749
func parseScopeClaim(claims jwt.MapClaims, key string) []string {
	v, ok := claims[key]
	if !ok {
		return nil
	}

	// space-delimited string per OAuth2 spec: "read write admin"
	if s, ok := v.(string); ok {
		return splitClaimString(s)
	}

	// array form: ["read", "write"]
	if arr, ok := v.([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}

	return nil
}

func splitClaimString(s string) []string {
	var result []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ','
	}) {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
