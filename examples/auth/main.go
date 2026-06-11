/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 *
 * JWT authentication example.
 *
 * Generate keys and a token:
 *
 *   go run ./cmd/jwttool generate-keys
 *   go run ./cmd/jwttool generate-token -k <private-key> -s user@example.com -r admin -e 1h
 *
 * Start the server (paste the public-key value below):
 *
 *   go run ./examples/auth run
 *
 * Test:
 *
 *   curl http://localhost:8000/status
 *   curl -H "Authorization: Bearer <token>" http://localhost:8000/api/me
 */

package main

import (
	"encoding/json"
	"net/http"

	"go.arpabet.com/cligo"
	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
)

// --- /status — public, no auth required ---

type StatusHandler struct{}

func (h *StatusHandler) Pattern() string { return "/status" }

func (h *StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// --- /api/me — requires a valid JWT ---

type MeHandler struct{}

func (h *MeHandler) Pattern() string { return "/api/me" }

func (h *MeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	auth, ok := servion.AuthFromContext(r.Context())
	if !ok {
		http.Error(w, "auth context missing", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"subject":    auth.Subject,
		"issuer":     auth.Issuer,
		"roles":      auth.Roles,
		"scopes":     auth.Scopes,
		"attributes": auth.Attributes,
	})
}

func main() {

	properties := glue.MapPropertySource{
		"http-server.bind-address": "0.0.0.0:8000",
		"http-server.options":      "handlers",

		// auth middleware only protects /api prefixed routes
		"auth.prefixes": "/api",

		// paste the public-key from: go run ./cmd/jwttool generate-keys
		// "jwt.public-key": "<your-base64-public-key>",

		// or use HMAC for quick testing
		"jwt.secret": "change-me-to-a-real-secret-in-production",
	}

	beans := []interface{}{
		properties,
		servion.RunCommand(servion.HttpServerScanner("http-server",
			&StatusHandler{},
			&MeHandler{},
			servion.AuthMiddleware(10),
			servion.JwtAuthProvider(),
			servion.HealthHandler(),
		)),
		servion.ZapLogFactory(true),
	}

	cligo.Main(cligo.Beans(beans...))
}
