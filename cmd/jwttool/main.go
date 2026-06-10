/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package main

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	flag "github.com/spf13/pflag"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate-keys":
		cmdGenerateKeys(os.Args[2:])
	case "generate-token":
		cmdGenerateToken(os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `jwttool — JWT key and token management for Servion

Usage:
  jwttool <command> [flags]

Commands:
  generate-keys    Generate an ECDSA P-256 key pair (base64 DER)
  generate-token   Generate a signed JWT token

Run "jwttool <command> --help" for details.
`)
}

// --- generate-keys ---

func cmdGenerateKeys(args []string) {
	fs := flag.NewFlagSet("generate-keys", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Generate an ECDSA P-256 key pair.

Outputs the private key and public key as base64-encoded DER strings,
ready to use in Servion configuration (jwt.public-key property).

Usage:
  jwttool generate-keys

`)
	}
	fs.Parse(args)

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		fatal("generate key: %v", err)
	}

	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		fatal("marshal private key: %v", err)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		fatal("marshal public key: %v", err)
	}

	privB64 := base64.StdEncoding.EncodeToString(privDER)
	pubB64 := base64.StdEncoding.EncodeToString(pubDER)

	fmt.Println("# ECDSA P-256 key pair")
	fmt.Println("# Keep the private key secret. Use the public key in jwt.public-key property.")
	fmt.Println()
	fmt.Printf("private-key=%s\n", privB64)
	fmt.Println()
	fmt.Printf("public-key=%s\n", pubB64)
}

// --- generate-token ---

func cmdGenerateToken(args []string) {
	fs := flag.NewFlagSet("generate-token", flag.ExitOnError)

	privateKey := fs.StringP("private-key", "k", "", "ECDSA private key (base64 DER)")
	secret := fs.String("secret", "", "HMAC shared secret (alternative to --private-key)")
	subject := fs.StringP("subject", "s", "", "Subject claim (sub)")
	issuer := fs.StringP("issuer", "i", "", "Issuer claim (iss)")
	audience := fs.StringP("audience", "a", "", "Audience claim (aud)")
	roles := fs.StringP("roles", "r", "", "Comma-separated roles")
	scopes := fs.String("scopes", "", "Comma-separated scopes")
	expiry := fs.StringP("expiry", "e", "1h", "Token expiration (e.g. 30m, 1h, 24h, 7d, 365d)")
	attrs := fs.StringSlice("attr", nil, "Custom attribute key=value (repeatable)")
	interactive := fs.BoolP("interactive", "I", false, "Interactive mode — prompt for missing values")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Generate a signed JWT token.

Supports ECDSA (--private-key) or HMAC (--secret) signing.
Use --interactive to be prompted for values.

Usage:
  jwttool generate-token [flags]

Flags:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  # ECDSA token
  jwttool generate-token -k <base64-private-key> -s user@example.com -r admin,editor -e 24h

  # HMAC token
  jwttool generate-token --secret my-secret -s service-account -e 8760h

  # Interactive mode
  jwttool generate-token -I
`)
	}
	fs.Parse(args)

	reader := bufio.NewReader(os.Stdin)

	if *interactive {
		if *privateKey == "" && *secret == "" {
			fmt.Print("Signing method (ecdsa/hmac) [ecdsa]: ")
			method := readLine(reader)
			if method == "" || method == "ecdsa" {
				fmt.Print("Private key (base64): ")
				*privateKey = readLine(reader)
			} else {
				fmt.Print("HMAC secret: ")
				*secret = readLine(reader)
			}
		}
		if *subject == "" {
			fmt.Print("Subject (sub): ")
			*subject = readLine(reader)
		}
		if *issuer == "" {
			fmt.Print("Issuer (iss) []: ")
			*issuer = readLine(reader)
		}
		if *audience == "" {
			fmt.Print("Audience (aud) []: ")
			*audience = readLine(reader)
		}
		if *roles == "" {
			fmt.Print("Roles (comma-separated) []: ")
			*roles = readLine(reader)
		}
		if *scopes == "" {
			fmt.Print("Scopes (comma-separated) []: ")
			*scopes = readLine(reader)
		}
		if len(*attrs) == 0 {
			fmt.Print("Attributes (key=value, comma-separated) []: ")
			line := readLine(reader)
			if line != "" {
				*attrs = strings.Split(line, ",")
			}
		}
		fmt.Printf("Expiry [%s]: ", *expiry)
		if exp := readLine(reader); exp != "" {
			*expiry = exp
		}
	}

	if *privateKey == "" && *secret == "" {
		fatal("either --private-key or --secret is required")
	}
	if *privateKey != "" && *secret != "" {
		fatal("--private-key and --secret are mutually exclusive")
	}
	if *subject == "" {
		fatal("--subject is required")
	}

	dur, err := parseDuration(*expiry)
	if err != nil {
		fatal("invalid expiry %q: %v", *expiry, err)
	}

	claims := jwt.MapClaims{
		"sub": *subject,
		"exp": jwt.NewNumericDate(time.Now().Add(dur)),
		"iat": jwt.NewNumericDate(time.Now()),
	}
	if *issuer != "" {
		claims["iss"] = *issuer
	}
	if *audience != "" {
		claims["aud"] = *audience
	}
	if *roles != "" {
		claims["roles"] = splitTrim(*roles)
	}
	if *scopes != "" {
		claims["scope"] = strings.Join(splitTrim(*scopes), " ")
	}
	for _, attr := range *attrs {
		k, v, ok := strings.Cut(strings.TrimSpace(attr), "=")
		if !ok {
			fatal("invalid attribute %q, expected key=value", attr)
		}
		claims[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}

	var tokenStr string
	if *privateKey != "" {
		tokenStr, err = signECDSA(*privateKey, claims)
	} else {
		tokenStr, err = signHMAC(*secret, claims)
	}
	if err != nil {
		fatal("sign token: %v", err)
	}

	fmt.Println(tokenStr)
}

func signECDSA(privB64 string, claims jwt.MapClaims) (string, error) {
	der, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil {
		return "", fmt.Errorf("base64 decode private key: %w", err)
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}
	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("key is not ECDSA, got %T", key)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	return token.SignedString(ecKey)
}

func signHMAC(secret string, claims jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// parseDuration extends time.ParseDuration with "d" suffix for days.
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		s = strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(s, "%d", &days); err != nil {
			return 0, fmt.Errorf("invalid days: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func splitTrim(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func readLine(r *bufio.Reader) string {
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
