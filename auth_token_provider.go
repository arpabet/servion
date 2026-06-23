package servion

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"golang.org/x/xerrors"
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
				return xerrors.New("token must not contain comma")
			}

			hashedToken := hashToken(token)

			t.allowed[hashedToken] = AuthInfo{
				HashedToken: hashedToken,
				Subject:     hashedToken,
			}

		}
	}

	// clear raw tokens so they are not retained in memory
	t.Tokens = nil

	return nil
}

func (t *implAuthTokenProvider) Authenticate(token string) (AuthInfo, error) {
	h := hashToken(token)
	if info, ok := t.allowed[h]; ok {
		return info, nil
	}
	return AuthInfo{}, ErrUnauthorized
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
