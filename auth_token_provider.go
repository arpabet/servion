package servion

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type implAuthTokenProvider struct {
	allowed map[string]AuthInfo

	Tokens []string `value:"auth.tokens"`
}

func AuthTokenProvider() Authenticator {
	return &implAuthTokenProvider{allowed: make(map[string]AuthInfo)}
}

func (t *implAuthTokenProvider) PostConstruct() error {

	for _, token := range t.Tokens {
		token = strings.TrimSpace(token)
		if token != "" {
			if strings.Contains(token, ",") {
				return fmt.Errorf("token must not contain comma")
			}

			sum := sha256.Sum256([]byte(token))
			hashedToken := hex.EncodeToString(sum[:])

			t.allowed[token] = AuthInfo{
				HashedToken: hashedToken,
				Subject:     hashedToken,
			}

		}
	}

	return nil
}

func (t *implAuthTokenProvider) Authenticate(token string) (AuthInfo, error) {
	if info, ok := t.allowed[token]; ok {
		return info, nil
	} else {
		return AuthInfo{}, ErrUnauthorized
	}

}
