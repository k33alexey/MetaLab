package secretstore

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestNativeCredentialStoreIntegration(t *testing.T) {
	if os.Getenv("ML_TEST_NATIVE_SECRET_STORE") == "" {
		t.Skip("ML_TEST_NATIVE_SECRET_STORE is not set")
	}
	store := New()
	key := fmt.Sprintf("test.native.%d", time.Now().UnixNano())
	const secret = "MetaLab temporary credential-store test"
	t.Cleanup(func() { _ = store.Delete(key) })
	if err := store.Set(key, secret); err != nil {
		t.Fatal(err)
	}
	read, err := store.Get(key)
	if err != nil || read != secret {
		t.Fatalf("secret matches = %v, error = %v", read == secret, err)
	}
	if err := store.Delete(key); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after delete error = %v", err)
	}
}
