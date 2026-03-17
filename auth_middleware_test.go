package servion

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware_ValidToken(t *testing.T) {
	mw := &implAuthMiddleware{
		beanOrder: 1,
		Prefixes:  []string{"/api"},
		Authenticator: &mockAuthenticator{
			authFunc: func(token string) (AuthInfo, error) {
				if token == "valid-token" {
					return AuthInfo{Subject: "user1", Scopes: []string{"read"}}, nil
				}
				return AuthInfo{}, ErrUnauthorized
			},
		},
	}

	var gotAuth AuthInfo
	var gotOk bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotOk = AuthFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	r.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !gotOk {
		t.Fatal("expected auth info in context")
	}
	if gotAuth.Subject != "user1" {
		t.Errorf("Subject = %q, want user1", gotAuth.Subject)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	mw := &implAuthMiddleware{
		beanOrder: 1,
		Prefixes:  []string{"/api"},
		Authenticator: &mockAuthenticator{
			authFunc: func(token string) (AuthInfo, error) {
				return AuthInfo{}, ErrUnauthorized
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if got := w.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Errorf("WWW-Authenticate = %q, want Bearer", got)
	}
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	mw := &implAuthMiddleware{
		beanOrder: 1,
		Prefixes:  []string{"/api"},
		Authenticator: &mockAuthenticator{
			authFunc: func(token string) (AuthInfo, error) {
				return AuthInfo{}, ErrUnauthorized
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := mw.Middleware(next)

	tests := []struct {
		name   string
		header string
	}{
		{"basic auth", "Basic dXNlcjpwYXNz"},
		{"no scheme", "just-a-token"},
		{"empty bearer", "Bearer"},
		{"extra parts", "Bearer token extra"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/data", nil)
			r.Header.Set("Authorization", tt.header)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	mw := &implAuthMiddleware{
		beanOrder: 1,
		Prefixes:  []string{"/api"},
		Authenticator: &mockAuthenticator{
			authFunc: func(token string) (AuthInfo, error) {
				return AuthInfo{}, ErrUnauthorized
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	r.Header.Set("Authorization", "Bearer bad-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_ServiceUnavailable(t *testing.T) {
	mw := &implAuthMiddleware{
		beanOrder: 1,
		Prefixes:  []string{"/api"},
		Authenticator: &mockAuthenticator{
			authFunc: func(token string) (AuthInfo, error) {
				return AuthInfo{}, ErrServiceUnavailable
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	r.Header.Set("Authorization", "Bearer some-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestAuthMiddleware_InternalError(t *testing.T) {
	mw := &implAuthMiddleware{
		beanOrder: 1,
		Prefixes:  []string{"/api"},
		Authenticator: &mockAuthenticator{
			authFunc: func(token string) (AuthInfo, error) {
				return AuthInfo{}, errors.New("database connection failed")
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	r.Header.Set("Authorization", "Bearer some-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestAuthMiddleware_SkipOptions(t *testing.T) {
	mw := &implAuthMiddleware{
		beanOrder: 1,
		Prefixes:  []string{"/api"},
		Authenticator: &mockAuthenticator{
			authFunc: func(token string) (AuthInfo, error) {
				t.Error("authenticator should not be called for OPTIONS")
				return AuthInfo{}, ErrUnauthorized
			},
		},
	}

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Middleware(next)

	r := httptest.NewRequest(http.MethodOptions, "/api/data", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if !called {
		t.Error("next handler should be called for OPTIONS")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_CaseInsensitiveBearer(t *testing.T) {
	mw := &implAuthMiddleware{
		beanOrder: 1,
		Prefixes:  []string{"/api"},
		Authenticator: &mockAuthenticator{
			authFunc: func(token string) (AuthInfo, error) {
				return AuthInfo{Subject: "user1"}, nil
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw.Middleware(next)

	for _, scheme := range []string{"bearer", "BEARER", "Bearer"} {
		t.Run(scheme, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/data", nil)
			r.Header.Set("Authorization", scheme+" valid-token")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want %d for scheme %q", w.Code, http.StatusOK, scheme)
			}
		})
	}
}

func TestAuthMiddleware_Match(t *testing.T) {
	mw := &implAuthMiddleware{
		Prefixes: []string{"/api", "/admin"},
	}

	tests := []struct {
		pattern string
		want    bool
	}{
		{"/api/users", true},
		{"/admin/dashboard", true},
		{"/public/page", false},
		{"/healthz", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			if got := mw.Match(tt.pattern); got != tt.want {
				t.Errorf("Match(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestAuthMiddleware_BeanOrder(t *testing.T) {
	mw := AuthMiddleware(5)
	if got := mw.BeanOrder(); got != 5 {
		t.Errorf("BeanOrder() = %d, want 5", got)
	}
}
