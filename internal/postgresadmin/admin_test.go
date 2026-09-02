package postgresadmin

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
)

func TestSCRAMVerifierDoesNotContainPassword(t *testing.T) {
	t.Parallel()

	verifier, err := scramVerifier("do-not-store-this-password", defaultIterations)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(verifier, "SCRAM-SHA-256$4096:") || strings.Contains(verifier, "do-not-store") {
		t.Fatalf("verifier = %q", verifier)
	}
	if len(strings.Split(verifier, "$")) != 3 {
		t.Fatalf("verifier = %q", verifier)
	}
}

func TestCheckProvisioningCapabilities(t *testing.T) {
	t.Parallel()

	if !(Check{CanCreateDB: true, CanCreateRole: true}).CanProvision() {
		t.Fatal("complete capabilities rejected")
	}
	if (Check{CanCreateDB: true}).CanProvision() || (Check{Superuser: true, InRecovery: true}).CanProvision() {
		t.Fatal("incomplete or recovery capabilities accepted")
	}
}

func TestProvisionRejectsUnsafeNamesBeforeConnecting(t *testing.T) {
	t.Parallel()

	if _, err := Provision(t.Context(), zeroDescriptor(), "secret", "db;drop", "role"); err == nil {
		t.Fatal("unsafe database name accepted")
	}
}

func zeroDescriptor() (descriptor postgresconn.Descriptor) { return descriptor }
