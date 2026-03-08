package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"

	"filippo.io/age"

	"github.com/dohr-michael/ozzie/internal/secrets"
)

// LocalAuth implements Authenticator using an age-encrypted filesystem token.
type LocalAuth struct {
	token string // plaintext token kept in memory for comparison
}

// NewLocalAuth generates a new random token, encrypts it with the age recipient,
// writes the ENC[age:...] blob to tokenPath, and returns the authenticator.
func NewLocalAuth(tokenPath string, recipient *age.X25519Recipient) (*LocalAuth, error) {
	token, err := generateToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	encrypted, err := secrets.Encrypt(token, recipient)
	if err != nil {
		return nil, fmt.Errorf("encrypt local token: %w", err)
	}
	if err := os.WriteFile(tokenPath, []byte(encrypted), 0o600); err != nil {
		return nil, fmt.Errorf("write local token: %w", err)
	}
	return &LocalAuth{token: token}, nil
}

func (a *LocalAuth) AuthenticateHTTP(r *http.Request) (string, error) {
	return a.authenticate(r)
}

func (a *LocalAuth) AuthenticateWS(r *http.Request) (string, error) {
	return a.authenticate(r)
}

func (a *LocalAuth) authenticate(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", ErrUnauthorized
	}
	bearer := strings.TrimPrefix(header, "Bearer ")
	if subtle.ConstantTimeCompare([]byte(bearer), []byte(a.token)) != 1 {
		return "", ErrUnauthorized
	}
	return DeviceLocal, nil
}

// generateToken returns a hex-encoded random token of n bytes (2*n hex chars).
func generateToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
