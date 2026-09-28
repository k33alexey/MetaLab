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
		children := map[string][]string{}
		for inner := 0; inner < element.NumField(); inner++ {
			sub := element.Field(inner)
			if sub.PkgPath != "" || sub.Type.Kind() != reflect.Slice || sub.Type.Elem().Kind() != reflect.Struct {
				continue
			}
			children[sub.Name] = metadataFieldNames(sub.Type.Elem())
		}
		dump[element.Name()] = map[string]any{"own": metadataFieldNames(element), "children": children}
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
