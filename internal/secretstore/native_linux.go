//go:build linux

package secretstore

// A server has no unlocked Secret Service keyring, and a key kept on the same
// machine protects nothing root cannot read, so a secret is stored as is and
// the rights on the file and the directory keep it from other users.
const secretExtension = ".secret"

func sealSecret(plain []byte) ([]byte, error) { return plain, nil }

func openSecret(stored []byte) ([]byte, error) { return stored, nil }
