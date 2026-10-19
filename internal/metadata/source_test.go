package metadata

import (
	"reflect"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Every kind the layout of a project has is read through kindInfos, so the
// load and Studio's save know it alike, and is checked against where it lies.
// The kinds come from the layout, not from the table: a kind the layout gained
// and the table did not is what Studio used to save unchecked. Defects caught:
// a kind missing from the table (read by nobody, saved as bare YAML), a
// subordinate kind given a list of its own (read twice), and a decoded type -
// a form among them - without the head every description has: a Format, an
// ID, a Name and a Title (headOf would panic on the first file).
func TestEverySourceKindIsChecked(t *testing.T) {
	t.Parallel()
	// Read otherwise, each for a reason: a common form is a form, read by
	// DecodeSource as one; a language lies in the configuration file.
	readOtherwise := []string{"common-forms", "languages"}
	for _, kind := range project.MetadataKinds() {
		if slices.Contains(readOtherwise, kind) {
			if item, ok := kindInfoOf(Kind(kind)); ok && (item.decode != nil || item.add != nil) {
				t.Errorf("%s is read otherwise and has a reading in kindInfos as well", kind)
			}
			continue
		}
		item, ok := kindInfoOf(Kind(kind))
		if !ok {
			t.Errorf("%s has no reading", kind)
			continue
		}
		if item.add == nil || item.objects == nil {
			t.Errorf("%s is read and goes nowhere", kind)
		}
	}
	for _, kind := range project.SubordinateObjectKinds() {
		item, ok := kindInfoOf(Kind(kind))
		if !ok {
			t.Errorf("subordinate %s has no reading", kind)
		} else if item.add != nil || item.objects != nil {
			t.Errorf("subordinate %s goes into a list of its own, and its owner collects it", kind)
		}
	}
	seen := map[Kind]bool{}
	for _, item := range kindInfos {
		if seen[item.kind] {
			t.Errorf("%s is read twice", item.kind)
		}
		seen[item.kind] = true
		if item.decode != nil {
			checkDescriptionHead(t, string(item.kind), item.described)
		}
	}
	checkDescriptionHead(t, "a form", reflect.TypeFor[ManagedForm]())
}

// checkDescriptionHead requires of a type a description is read into the four
// fields headOf reads, each of the type it reads it as.
func checkDescriptionHead(t *testing.T, kind string, described reflect.Type) {
	t.Helper()
	for name, want := range map[string]reflect.Type{
		"Format": reflect.TypeFor[int](), "ID": reflect.TypeFor[uuid.UUID](),
		"Name": reflect.TypeFor[string](), "Title": reflect.TypeFor[LocalizedText](),
	} {
		if field, ok := described.FieldByName(name); !ok || field.Type != want {
			t.Errorf("%s decodes %s, which has no %s of %s in the head every description has", kind, described, name, want)
		}
	}
}

// Every kind the model knows is named by the kinds package, and the kinds
// package names no kind the model does not know. Defects caught: a kind
// added to the layout or the reading without its names (its full name, its
// collection in code, its module names and its caption would be missing,
// and kinds.Must would panic on the first module of it), and a descriptor
// left behind for a kind gone from the model.
func TestEveryKindIsNamed(t *testing.T) {
	t.Parallel()
	known := map[Kind]bool{}
	for _, kind := range append(project.MetadataKinds(), project.SubordinateObjectKinds()...) {
		known[Kind(kind)] = true
	}
	for _, item := range kindInfos {
		known[item.kind] = true
	}
	// A recalculation lies inside its calculation register and is read with
	// it: it is a kind of the prototype's tree, not of the layout.
	known[RecalculationKind] = true
	for kind := range known {
		if _, ok := kinds.Of(kind); !ok {
			t.Errorf("%s has no names", kind)
		}
	}
	for _, item := range kinds.All() {
		if !known[item.Kind] {
			t.Errorf("%s is named and the model does not know it", item.Kind)
		}
	}
}

// The folder of an object is checked against what its description declares,
// read off the description itself (declaredFilesOf). That needs the walk to
// find the description, and the fields it reads to be what it reads them as.
// Defects caught: a kind whose folder is checked but whose description cannot
// be found - its forms, commands and templates silently unchecked - and a
// field of forms, commands or templates of another type, which would panic on
// the first object of the kind.
func TestEveryCheckedFolderIsReadOffItsDescription(t *testing.T) {
	t.Parallel()
	for _, item := range kindInfos {
		if kindCompositionOf[item.kind].files == nil {
			continue
		}
		if item.object == nil || item.described == nil {
			t.Errorf("%s has its folder checked, and its description cannot be found", item.kind)
			continue
		}
		if forms, ok := item.described.FieldByName("Forms"); ok && !forms.Type.Implements(reflect.TypeFor[formRoles]()) {
			t.Errorf("%s keeps forms of %s, which says no form roles", item.kind, forms.Type)
		}
		for name, want := range map[string]reflect.Type{
			"Commands": reflect.TypeFor[[]ObjectCommand](), "Templates": reflect.TypeFor[[]ObjectTemplate](),
		} {
			if field, ok := item.described.FieldByName(name); ok && field.Type != want {
				t.Errorf("%s keeps %s of %s, not %s", item.kind, name, field.Type, want)
			}
		}
	}
}
