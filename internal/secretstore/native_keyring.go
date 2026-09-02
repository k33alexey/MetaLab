//go:build !windows

package secretstore

import (
	"errors"

	keyring "github.com/zalando/go-keyring"
)

type nativeBackend struct{}

func (nativeBackend) Set(service, account, secret string) error {
	return keyring.Set(service, account, secret)
}

func (nativeBackend) Get(service, account string) (string, error) {
	secret, err := keyring.Get(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return secret, err
}

func (nativeBackend) Delete(service, account string) error {
	err := keyring.Delete(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
