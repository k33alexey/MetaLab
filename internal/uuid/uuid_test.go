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

// A derived identity has to be the same on every call and different for every
// name: it stands in for an identity that was never written down, and two reads
// of the same thing must agree.
func TestDeriveIsStableAndDistinct(t *testing.T) {
	t.Parallel()
	namespace := MustNew()
	first, second := Derive(namespace, "language:ru"), Derive(namespace, "language:ru")
	if first != second || first.IsZero() {
		t.Fatalf("derived identity is not stable: %s vs %s", first, second)
	}
	if Derive(namespace, "language:uk") == first {
		t.Fatal("two names derived the same identity")
	}
	if Derive(MustNew(), "language:ru") == first {
		t.Fatal("two namespaces derived the same identity")
	}
	parsed, err := Parse(first.String())
	if err != nil || parsed != first {
		t.Fatalf("derived identity is not a canonical UUID: %v", err)
	}
	if first[6]&0xf0 != 0x80 || first[8]&0xc0 != 0x80 {
		t.Fatalf("derived identity does not carry the version and variant bits: %s", first)
	}
}
