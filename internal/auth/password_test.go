package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestPasswordHashAndVerify(t *testing.T) {
	t.Parallel()

	encoded, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded, "correct") || !strings.HasPrefix(encoded, "$argon2id$") {
		t.Fatalf("encoded hash = %q", encoded)
	}
	valid, err := VerifyPassword(encoded, "correct horse battery staple")
	if err != nil || !valid {
		t.Fatalf("valid = %v, error = %v", valid, err)
	}
	valid, err = VerifyPassword(encoded, "incorrect password")
	if err != nil || valid {
		t.Fatalf("valid = %v, error = %v", valid, err)
	}
}

func TestPasswordValidationAndMalformedHashes(t *testing.T) {
	t.Parallel()

	if err := ValidatePassword("коротко"); err == nil {
		t.Fatal("short password accepted")
	}
	if err := ValidatePassword("надежный пароль"); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyPassword("$argon2id$v=19$m=9999999,t=2,p=1$c2FsdHNhbHRzYWx0c2FsdA$aGFzaGhhc2hoYXNoaGFzaA", "password"); !errors.Is(err, ErrInvalidPasswordHash) {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
}

func TestRecoveryCodesAreUniqueAndDigestible(t *testing.T) {
	t.Parallel()

	codes, err := GenerateRecoveryCodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != recoveryCodeCount {
		t.Fatalf("code count = %d", len(codes))
	}
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if _, exists := seen[code]; exists {
			t.Fatalf("duplicate recovery code %q", code)
		}
		seen[code] = struct{}{}
		digest, err := RecoveryCodeDigest(strings.ToLower(code))
		if err != nil || digest == [32]byte{} {
			t.Fatalf("digest = %x, error = %v", digest, err)
		}
	}
	if _, err := RecoveryCodeDigest("invalid"); err == nil {
		t.Fatal("invalid recovery code accepted")
	}
}

func TestTemporaryPasswordMeetsPolicy(t *testing.T) {
	t.Parallel()

	password, err := GenerateTemporaryPassword()
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidatePassword(password); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkHashPassword(b *testing.B) {
	for b.Loop() {
		if _, err := HashPassword("correct horse battery staple"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVerifyPassword(b *testing.B) {
	encoded, err := HashPassword("correct horse battery staple")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		if valid, err := VerifyPassword(encoded, "correct horse battery staple"); err != nil || !valid {
			b.Fatalf("valid = %v, error = %v", valid, err)
		}
	}
}
