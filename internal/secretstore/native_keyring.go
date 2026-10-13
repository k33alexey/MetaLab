//go:build !windows && !linux

package secretstore

import (
	"errors"

	keyring "github.com/zalando/go-keyring"
)

// The keychain of macOS is user-scoped: ML Service there is a user service of
// the administrator who writes the secrets.
type keyringBackend struct{}

func nativeBackend() backend { return keyringBackend{} }

func (keyringBackend) Set(service, account, secret string) error {
	return keyring.Set(service, account, secret)
}

func (keyringBackend) Get(service, account string) (string, error) {
	secret, err := keyring.Get(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return secret, err
}

func (keyringBackend) Delete(service, account string) error {
	err := keyring.Delete(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
