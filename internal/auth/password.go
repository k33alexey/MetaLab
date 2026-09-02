// Package auth implements local MetaLab credentials and recovery secrets.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	passwordMemory       uint32 = 19 * 1024
	passwordTime         uint32 = 2
	passwordThreads      uint8  = 1
	passwordSaltLen             = 16
	passwordHashLen             = 32
	minimumPasswordRunes        = 8
	maximumPasswordBytes        = 1024
	recoveryCodeCount           = 10
	recoveryCodeBytes           = 16
)

var (
	// ErrInvalidPasswordHash indicates a malformed or unsupported stored hash.
	ErrInvalidPasswordHash = errors.New("invalid password hash")
	// ErrPasswordPolicy indicates a password rejected by the current policy.
	ErrPasswordPolicy = errors.New("password does not meet policy")
)

// ValidatePassword applies the initial MetaLab password policy.
func ValidatePassword(password string) error {
	if len(password) > maximumPasswordBytes {
		return fmt.Errorf("%w: password is longer than %d bytes", ErrPasswordPolicy, maximumPasswordBytes)
	}
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < minimumPasswordRunes {
		return fmt.Errorf("%w: password must contain at least %d characters", ErrPasswordPolicy, minimumPasswordRunes)
	}
	return nil
}

// HashPassword creates a self-describing Argon2id password hash.
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	salt := make([]byte, passwordSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, passwordTime, passwordMemory, passwordThreads, passwordHashLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, passwordMemory, passwordTime, passwordThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword compares a password with a self-describing Argon2id hash.
func VerifyPassword(encoded, password string) (bool, error) {
	parameters, salt, expected, err := parsePasswordHash(encoded)
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey([]byte(password), salt, parameters.time, parameters.memory, parameters.threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

// SpendPasswordWork equalizes an unknown or disabled account with a real check.
func SpendPasswordWork(password string) {
	var salt [passwordSaltLen]byte
	_ = argon2.IDKey([]byte(password), salt[:], passwordTime, passwordMemory, passwordThreads, passwordHashLen)
}

type hashParameters struct {
	time    uint32
	memory  uint32
	threads uint8
}

func parsePasswordHash(encoded string) (hashParameters, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v="+strconv.Itoa(argon2.Version) {
		return hashParameters{}, nil, nil, ErrInvalidPasswordHash
	}
	var parameters hashParameters
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &parameters.memory, &parameters.time, &parameters.threads); err != nil {
		return hashParameters{}, nil, nil, ErrInvalidPasswordHash
	}
	if parameters.memory < 8*1024 || parameters.memory > 256*1024 || parameters.time == 0 || parameters.time > 10 || parameters.threads == 0 || parameters.threads > 16 {
		return hashParameters{}, nil, nil, ErrInvalidPasswordHash
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) < 16 || len(salt) > 64 {
		return hashParameters{}, nil, nil, ErrInvalidPasswordHash
	}
	hash, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(hash) < 16 || len(hash) > 64 {
		return hashParameters{}, nil, nil, ErrInvalidPasswordHash
	}
	return parameters, salt, hash, nil
}

// GenerateRecoveryCodes creates single-display, high-entropy recovery codes.
func GenerateRecoveryCodes() ([]string, error) {
	codes := make([]string, recoveryCodeCount)
	for index := range codes {
		secret := make([]byte, recoveryCodeBytes)
		if _, err := rand.Read(secret); err != nil {
			return nil, fmt.Errorf("generate recovery code: %w", err)
		}
		plain := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
		codes[index] = groupCode(plain)
	}
	return codes, nil
}

// RecoveryCodeDigest returns the irreversible database representation of a code.
func RecoveryCodeDigest(code string) ([32]byte, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(normalized)
	if err != nil || len(decoded) != recoveryCodeBytes {
		return [32]byte{}, fmt.Errorf("invalid recovery code")
	}
	return sha256.Sum256(decoded), nil
}

// GenerateTemporaryPassword creates a random password for a local emergency reset.
func GenerateTemporaryPassword() (string, error) {
	secret := make([]byte, 18)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate temporary password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(secret), nil
}

func groupCode(code string) string {
	groups := make([]string, 0, (len(code)+3)/4)
	for len(code) > 4 {
		groups = append(groups, code[:4])
		code = code[4:]
	}
	groups = append(groups, code)
	return strings.Join(groups, "-")
}
