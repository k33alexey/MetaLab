package uuid

import "testing"

func TestNewCreatesVersion4UUID(t *testing.T) {
	t.Parallel()

	id, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if id.IsZero() {
		t.Fatal("New() returned zero UUID")
	}
	if version := id[6] >> 4; version != 4 {
		t.Fatalf("UUID version = %d, want 4", version)
	}
	if variant := id[8] >> 6; variant != 2 {
		t.Fatalf("UUID variant = %d, want 2", variant)
	}
}

func TestParseRoundTrip(t *testing.T) {
	t.Parallel()

	const source = "018f1f72-3b4c-7d6e-8f90-123456789abc"
	id, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got := id.String(); got != source {
		t.Fatalf("UUID.String() = %q, want %q", got, source)
	}
}

func TestParseRejectsInvalidUUID(t *testing.T) {
	t.Parallel()

	values := []string{
		"",
		"018f1f72-3b4c-7d6e-8f90",
		"018f1f72_3b4c-7d6e-8f90-123456789abc",
		"018f1f72-3b4c-7d6e-8f90-123456789abz",
	}

	for _, value := range values {
		if _, err := Parse(value); err == nil {
			t.Errorf("Parse(%q) returned no error", value)
		}
	}
}

func TestTextRoundTrip(t *testing.T) {
	t.Parallel()

	original := MustNew()
	data, err := original.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText() error = %v", err)
	}

	var restored UUID
	if err := restored.UnmarshalText(data); err != nil {
		t.Fatalf("UnmarshalText() error = %v", err)
	}
	if restored != original {
		t.Fatalf("restored UUID = %s, want %s", restored, original)
	}
}

func TestZeroUUIDCannotBeMarshaled(t *testing.T) {
	t.Parallel()

	if _, err := (UUID{}).MarshalText(); err == nil {
		t.Fatal("MarshalText() returned no error for zero UUID")
	}
}
