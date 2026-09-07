package metadata

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestEnsureObjectIntegrityStorageUsesPlatformSchema(t *testing.T) {
	t.Parallel()
	recorder := &recordingExecutor{}
	if err := EnsureObjectIntegrityStorage(context.Background(), recorder); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(recorder.statement, "ml_core.object_sequences") || !strings.Contains(recorder.statement, "ml_core.object_deletions") {
		t.Fatalf("object integrity storage SQL = %q", recorder.statement)
	}
	if err := EnsureObjectIntegrityStorage(context.Background(), nil); err == nil {
		t.Fatal("EnsureObjectIntegrityStorage accepted nil executor")
	}
}

func TestAutomaticIdentifierFormatting(t *testing.T) {
	t.Parallel()
	if value, err := formatAutomaticIdentifier("42", StringType, 5); err != nil || value != "00042" {
		t.Fatalf("string identifier=%q error=%v", value, err)
	}
	if value, err := formatAutomaticIdentifier("42", NumberType, 5); err != nil || value != "42" {
		t.Fatalf("numeric identifier=%q error=%v", value, err)
	}
	for _, value := range []string{"", "1x", "123456"} {
		if _, err := formatAutomaticIdentifier(value, StringType, 5); err == nil {
			t.Fatalf("identifier %q was accepted", value)
		}
	}
}

func TestReferenceIntegrityErrorContract(t *testing.T) {
	t.Parallel()
	err := &ReferenceIntegrityError{Uses: []ReferenceUse{{OwnerKind: "document", OwnerName: "Заказ", Field: "Товар"}}}
	if !errors.Is(err, ErrObjectReferenced) {
		t.Fatalf("error %v does not wrap ErrObjectReferenced", err)
	}
}
