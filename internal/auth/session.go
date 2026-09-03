package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// NewSessionToken returns an opaque bearer token and the digest safe to persist.
func NewSessionToken() (string, [sha256.Size]byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", [sha256.Size]byte{}, fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	return token, SessionTokenDigest(token), nil
}

// SessionTokenDigest converts an opaque token to its fixed-size database representation.
func SessionTokenDigest(token string) [sha256.Size]byte { return sha256.Sum256([]byte(token)) }
