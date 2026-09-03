package auth

import "testing"

func TestSessionTokenIsOpaqueAndDigestIsStable(t *testing.T) {
	t.Parallel()
	first, digest, err := NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	second, secondDigest, err := NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 40 || first == second || digest == secondDigest || digest != SessionTokenDigest(first) {
		t.Fatalf("invalid session tokens: first=%q second=%q", first, second)
	}
}
