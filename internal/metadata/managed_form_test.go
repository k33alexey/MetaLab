package metadata

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestManagedFormDecodeValidateAndRoundTrip(t *testing.T) {
	t.Parallel()
	manifest := managedFormManifest()
	formID, groupID, fieldID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	source := "format: 1\nid: " + formID.String() + "\nname: \u0424\u043e\u0440\u043c\u0430\u0422\u043e\u0432\u0430\u0440\u0430\ntitle: {ru: \u0424\u043e\u0440\u043c\u0430 \u0442\u043e\u0432\u0430\u0440\u0430}\nkind: object\nitems:\n" +
		"  - id: " + groupID.String() + "\n    name: \u041e\u0441\u043d\u043e\u0432\u043d\u0430\u044f\u0413\u0440\u0443\u043f\u043f\u0430\n    kind: group\n    orientation: vertical\n    children:\n" +
		"      - id: " + fieldID.String() + "\n        name: \u041d\u0430\u0438\u043c\u0435\u043d\u043e\u0432\u0430\u043d\u0438\u0435\n        kind: field\n        title: {ru: \u041d\u0430\u0438\u043c\u0435\u043d\u043e\u0432\u0430\u043d\u0438\u0435}\n        read_only: true\n"
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(source), manifest)
	if err != nil {
		t.Fatal(err)
	}
	if form.Kind != ObjectForm || len(form.Items) != 1 || len(form.Items[0].Children) != 1 || !form.Items[0].Children[0].ReadOnly {
		t.Fatalf("decoded form = %+v", form)
	}
	var first, second bytes.Buffer
	if err := Encode(&first, form); err != nil {
		t.Fatal(err)
	}
	canonical := first.String()
	restored, err := DecodeManagedForm("form.yaml", strings.NewReader(canonical), manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := Encode(&second, restored); err != nil || canonical != second.String() {
		t.Fatalf("unstable form YAML: error=%v\nfirst=%s\nsecond=%s", err, canonical, second.String())
	}
}

func TestManagedFormRejectsUnknownAndInvalidTree(t *testing.T) {
	t.Parallel()
	manifest := managedFormManifest()
	formID, duplicateID := uuid.MustNew(), uuid.MustNew()
	base := "format: 1\nid: " + formID.String() + "\nname: \u0424\u043e\u0440\u043c\u0430\ntitle: {ru: \u0424\u043e\u0440\u043c\u0430}\nkind: object\n"
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(base+"unknown: true\n"), manifest); err == nil || !strings.Contains(err.Error(), "field unknown") {
		t.Fatalf("unknown-field error = %v", err)
	}
	invalid := base + "items:\n" +
		"  - {id: " + duplicateID.String() + ", name: \u041f\u043e\u043b\u0435, kind: field, orientation: vertical, children: [{id: " + uuid.MustNew().String() + ", name: \u0412\u043b\u043e\u0436\u0435\u043d\u043d\u043e\u0435, kind: label}]}\n" +
		"  - {id: " + duplicateID.String() + ", name: \u043f\u043e\u043b\u0435, kind: group, orientation: diagonal, read_only: true}\n"
	_, err := DecodeManagedForm("form.yaml", strings.NewReader(invalid), manifest)
	for _, expected := range []string{"id must be unique", "name must be unique", "children are allowed only for groups", "orientation is allowed only for groups", "orientation must be vertical or horizontal", "read_only is not allowed for a group"} {
		if err == nil || !strings.Contains(err.Error(), expected) {
			t.Fatalf("validation error %q missing from %v", expected, err)
		}
	}
}

func TestManagedFormRejectsUnsupportedFormatAndDepth(t *testing.T) {
	t.Parallel()
	manifest := managedFormManifest()
	form := ManagedForm{Format: 2, ID: uuid.MustNew(), Name: "\u0424\u043e\u0440\u043c\u0430", Title: LocalizedText{"ru": "\u0424\u043e\u0440\u043c\u0430"}, Kind: ObjectForm}
	if err := ValidateManagedForm("form.yaml", form, manifest); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("format error = %v", err)
	}
	form.Format = CurrentFormat
	leaf := ManagedFormElement{ID: uuid.MustNew(), Name: "\u042d\u043b\u0435\u043c\u0435\u043d\u0442", Kind: FormElementGroup, Orientation: FormVertical}
	for index := 0; index < MaxManagedFormDepth; index++ {
		leaf = ManagedFormElement{ID: uuid.MustNew(), Name: "\u0413\u0440\u0443\u043f\u043f\u0430" + string(rune('A'+index%26)) + string(rune('A'+index/26)), Kind: FormElementGroup, Orientation: FormVertical, Children: []ManagedFormElement{leaf}}
	}
	form.Items = []ManagedFormElement{leaf}
	if err := ValidateManagedForm("form.yaml", form, manifest); err == nil || !strings.Contains(err.Error(), "maximum nesting depth") {
		t.Fatalf("depth error = %v", err)
	}
}

func managedFormManifest() project.Project {
	return project.Project{Format: 1, ID: uuid.MustNew(), Name: "Demo", Title: "Demo", DefaultLanguage: "ru", Languages: []project.Language{{Name: "\u0420\u0443\u0441\u0441\u043a\u0438\u0439", Title: "\u0420\u0443\u0441\u0441\u043a\u0438\u0439", Code: "ru"}, {Name: "English", Title: "English", Code: "en"}}}
}

func BenchmarkDecodeManagedForm(b *testing.B) {
	manifest := managedFormManifest()
	form := ManagedForm{Format: 1, ID: uuid.MustNew(), Name: "\u0424\u043e\u0440\u043c\u0430", Title: LocalizedText{"ru": "\u0424\u043e\u0440\u043c\u0430"}, Kind: ObjectForm}
	for index := 0; index < 1_000; index++ {
		form.Items = append(form.Items, ManagedFormElement{ID: uuid.MustNew(), Name: fmtFormName(index), Kind: FormElementField})
	}
	var source bytes.Buffer
	if err := Encode(&source, form); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(source.Len()))
	b.ResetTimer()
	for range b.N {
		if _, err := DecodeManagedForm("form.yaml", bytes.NewReader(source.Bytes()), manifest); err != nil {
			b.Fatal(err)
		}
	}
}

func fmtFormName(index int) string {
	const digits = "0123456789"
	if index == 0 {
		return "\u041f\u043e\u043b\u04350"
	}
	var suffix [20]byte
	position := len(suffix)
	for index > 0 {
		position--
		suffix[position] = digits[index%10]
		index /= 10
	}
	return "\u041f\u043e\u043b\u0435" + string(suffix[position:])
}
