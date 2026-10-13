//go:build darwin

package systemservice

import kservice "github.com/kardianos/service"

func configureServiceIdentity(configuration *kservice.Config) {
	// The macOS keychain is user-scoped: a user service keeps the identity of
	// the administrator who stored the secrets.
	configuration.Option["UserService"] = true
}

func prepareInstall(string, string) error { return nil }
