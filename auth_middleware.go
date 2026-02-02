package servion

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type implAuthMiddleware struct {
	beanOrder int

	Prefixes []string `value:"auth.prefixes,default=/api"`

	Authenticator Authenticator `inject:"-"`
}

func AuthMiddleware(beanOrder int) HttpMiddleware {
	return &implAuthMiddleware{beanOrder: beanOrder}
}

func (t *implAuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		h := r.Header.Get("Authorization")
		if h == "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Authorization header is missing", http.StatusUnauthorized)
			return
		}

		parts := strings.Fields(h)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "invalid Authorization header", http.StatusUnauthorized)
			return
		}

		auth, err := t.Authenticator.Authenticate(parts[1])
		if errors.Is(err, ErrUnauthorized) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if errors.Is(err, ErrServiceUnavailable) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), authContextKey, auth)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (t *implAuthMiddleware) BeanOrder() int {
	return t.beanOrder
}

func (t *implAuthMiddleware) Match(prefix string) bool {
	for _, p := range t.Prefixes {
		if strings.HasPrefix(prefix, p) {
			return true
		}
	}
	return false
}
