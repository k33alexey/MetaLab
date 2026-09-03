package publication

import (
	"context"
	"errors"
	"testing"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
)

func TestActivateRequiresConfirmationBeforeReadingPackageOrDatabase(t *testing.T) {
	_, _, err := Activate(context.Background(), nil, ActivationRequest{})
	if !errors.Is(err, schemadiff.ErrConfirmationRequired) {
		t.Fatalf("Activate() error = %v", err)
	}
}

func TestCurrentAndListVersionsValidateArguments(t *testing.T) {
	if _, _, err := Current(context.Background(), nil); err == nil {
		t.Fatal("Current() accepted a nil database")
	}
	if _, err := ListVersions(context.Background(), nil, 10); err == nil {
		t.Fatal("ListVersions() accepted a nil database")
	}
}
