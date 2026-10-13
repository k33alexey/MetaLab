//go:build darwin || linux

package systemservice

import kservice "github.com/kardianos/service"

func configureServiceIdentity(configuration *kservice.Config) {
	// The macOS keychain is user-scoped: a user service keeps the identity of
	// the administrator who stored the secrets. On Linux the secrets belong to
	// the service account (secretstore.ServiceAccount); running the service
	// as a system service under it is a separate item of docs/BLOCKS.md.
	configuration.Option["UserService"] = true
}
