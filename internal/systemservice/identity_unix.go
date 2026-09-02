//go:build darwin || linux

package systemservice

import kservice "github.com/kardianos/service"

func configureServiceIdentity(configuration *kservice.Config) {
	// Keychain and Secret Service are user-scoped. A user service keeps the same
	// protected-store identity after Manager exits.
	configuration.Option["UserService"] = true
}
