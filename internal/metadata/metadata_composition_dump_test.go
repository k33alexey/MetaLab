package metadata

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// TestDumpMetadataComposition writes the composition of the metadata model as
// JSON for scripts/metadata_sweep.py to compare with the prototype's export.
//
// It lives here, beside the structures it reflects over, because the
// alternative is a list of field names kept in the script - and such a list
// would part company with the model on the first change, quietly, while still
// reporting a clean sweep. Reflection cannot go stale.
//
// It is a dump and not a check: what to make of a difference is not something
// a test can decide. The export says what the demo configuration uses, not
// what the platform has, so a difference is a question for the syntax
// assistant. A test that failed on every question would be red for ever and
// read by nobody.
func TestDumpMetadataComposition(t *testing.T) {
	path := os.Getenv("ML_METADATA_DUMP")
	if path == "" {
		t.Skip("ML_METADATA_DUMP is not set: the dump is taken by scripts/metadata_sweep.py")
	}
	dump := map[string]any{}
	catalogType := reflect.TypeOf(Catalog{})
	for index := 0; index < catalogType.NumField(); index++ {
		field := catalogType.Field(index)
		if field.PkgPath != "" || field.Type.Kind() != reflect.Slice {
			continue
		}
		element := field.Type.Elem()
		if element.Kind() != reflect.Struct {
			continue
		}
		// A child object is dumped the way a kind is - its whole set of names
		// and, separately, the names of the structure itself. The sweep needs
		// both: the substitution «one name of ours stands for several of
		// theirs» is safe only for a name that heads a group, and a dimension
		// or a command has groups of its own, presentation and choice among
		// them.
		dump[element.Name()] = map[string]any{
			"own": metadataFieldNames(element),
			// The names of the structure itself, without those of the
			// structures it folds in. The sweep needs the difference: one name
			// of ours often stands for several of the prototype's - our `code`
			// covers its CodeLength, CodeType and CheckUnique - and that
			// substitution is only safe for a name that heads a group. Applied
			// to a leaf it lies: our `value` inside filling once matched their
			// DataSeparationValue on the strength of one shared word.
			"top":      metadataTopFieldNames(element),
			"children": metadataChildren(element),
		}
	}
	encoded, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("выгружено структур: %d", len(dump))
}

// metadataChildren is the subordinate objects of a structure: every slice of
// structures it holds, by the name of the collection, with the same pair of
// name lists and with their own subordinates below them.
//
// Two levels are needed and not one. The export names a relation by three
// things - the kind, the owner and the child - and two of its relations are a
// grandchild: a method of a URL template of an HTTP service and a parameter of
// an operation of a web service. Stopping at one level would drop both from the
// sweep, and a relation nobody compares is silence that reads as agreement.
func metadataChildren(structType reflect.Type) map[string]any {
	return metadataChildrenAtDepth(structType, 0)
}

// maxChildDepth stops the walk where the export stops: kind, child, grandchild.
// It is a guard as much as a limit - a structure that holds a slice of its own
// kind would otherwise walk for ever.
const maxChildDepth = 2

func metadataChildrenAtDepth(structType reflect.Type, depth int) map[string]any {
	children := map[string]any{}
	if depth >= maxChildDepth {
		return children
	}
	for index := 0; index < structType.NumField(); index++ {
		field := structType.Field(index)
		if field.PkgPath != "" || field.Type.Kind() != reflect.Slice || field.Type.Elem().Kind() != reflect.Struct {
			continue
		}
		element := field.Type.Elem()
		children[field.Name] = map[string]any{
			"own":      metadataFieldNames(element),
			"top":      metadataTopFieldNames(element),
			"children": metadataChildrenAtDepth(element, depth+1),
		}
	}
	return children
}

// metadataTopFieldNames is the names of the structure itself: its own fields
// and those of the structures it folds in, without going into named structures
// below them.
func metadataTopFieldNames(structType reflect.Type) []string {
	seen := map[string]bool{}
	var walk func(reflect.Type)
	walk = func(current reflect.Type) {
		if current.Kind() == reflect.Pointer {
			current = current.Elem()
		}
		if current.Kind() != reflect.Struct {
			return
		}
		for index := 0; index < current.NumField(); index++ {
			field := current.Field(index)
			if field.PkgPath != "" {
				continue
			}
			name := strings.Split(field.Tag.Get("yaml"), ",")[0]
			kind := field.Type
			if kind.Kind() == reflect.Pointer {
				kind = kind.Elem()
			}
			if name == "" {
				if kind.Kind() == reflect.Struct {
					walk(kind)
				}
				continue
			}
			seen[name] = true
		}
	}
	walk(structType)
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return names
}

// metadataFieldNames is every YAML name a structure carries, its own and those
// of the structures it folds in. Names of nested structures are kept along with
// the names inside them, because the prototype keeps flat what we group: its
// «Формат» is our presentation.format, and the sweep matches one against the
// other by the last word of the path.
func metadataFieldNames(structType reflect.Type) []string {
	seen := map[string]bool{}
	var walk func(reflect.Type, int)
	walk = func(current reflect.Type, depth int) {
		if current.Kind() == reflect.Pointer {
			current = current.Elem()
		}
		if current.Kind() != reflect.Struct || depth > 4 {
			return
		}
		for index := 0; index < current.NumField(); index++ {
			field := current.Field(index)
			if field.PkgPath != "" {
				continue
			}
			name := strings.Split(field.Tag.Get("yaml"), ",")[0]
			kind := field.Type
			if kind.Kind() == reflect.Pointer {
				kind = kind.Elem()
			}
			// A field with no name of its own is folded in: its names are the
			// structure's own, at the same level.
			if name == "" {
				if kind.Kind() == reflect.Struct {
					walk(kind, depth)
				}
				continue
			}
			seen[name] = true
			// A value and a localized text are leaves: what is inside them is
			// the shape of one value, not properties of the object.
			if kind.Kind() == reflect.Struct && kind.Name() != "Value" && !strings.HasSuffix(kind.Name(), "Text") {
				walk(kind, depth+1)
			}
		}
	}
	walk(structType, 0)
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return names
}
