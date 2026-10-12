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
// subordinate kind given a list of its own (read twice), and a decoded type
// without an ID or a Name (sourceIdentity would panic on the first file).
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
		if item.decode == nil {
			continue
		}
		id, idOK := item.described.FieldByName("ID")
		name, nameOK := item.described.FieldByName("Name")
		if !idOK || id.Type != reflect.TypeFor[uuid.UUID]() || !nameOK || name.Type.Kind() != reflect.String {
			t.Errorf("%s decodes %s, which has no ID and Name to check against where it lies", item.kind, item.described)
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
