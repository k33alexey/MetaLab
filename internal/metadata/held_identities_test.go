package metadata

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// describedIn is the description of the composite project that names the
// object id at its head.
func describedIn(t *testing.T, root, id string) string {
	t.Helper()
	var found string
	err := filepath.WalkDir(filepath.Join(root, "metadata"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".yaml" || found != "" {
			return err
		}
		content, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(content), "\nid: "+id+"\n") {
			found = path
		}
		return err
	})
	if err != nil || found == "" {
		t.Fatalf("no description of %s: %v", id, err)
	}
	return found
}

// TestWhatAnObjectHoldsHasAnIdentifierOfItsOwn catches an identifier of what
// an object holds beside its parts - a predefined item, a point of a route,
// a dimension of a recalculation - left out of the one check that every
// identifier of the configuration is used once (heldIdentities): one of them
// taking the identifier of another object would name two things, and a role,
// an option or a reference would reach either.
func TestWhatAnObjectHoldsHasAnIdentifierOfItsOwn(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		object, add, replace, with, word string
	}{
		"a predefined item": {object: cmpGoods, add: "predefined:\n  - {id: " + cmpWarehouses + ", name: Основной}\n", word: "predefined item Основной"},
		"a point of a route": {object: cmpProcess, add: "route:\n  points:\n    - {id: " + cmpWarehouses + ", name: Старт, kind: start}\n" +
			"    - {id: " + compositeID(97) + ", name: Конец, kind: completion}\n  transitions:\n    - {from: Старт, to: Конец}\n", word: "route point Старт"},
		"a dimension of a recalculation": {object: cmpPayroll, replace: "{id: " + cmpRecalcDimension, with: "{id: " + cmpWarehouses, word: "dimension Лицо"},
	}
	for name, item := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := compositeProject(t)
			if _, err := Load(root); err != nil {
				t.Fatalf("the project does not load before the change: %v", err)
			}
			path := describedIn(t, root, item.object)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			text := string(content) + item.add
			if item.replace != "" {
				if strings.Count(text, item.replace) != 1 {
					t.Fatalf("%s does not name %q once", path, item.replace)
				}
				text = strings.Replace(text, item.replace, item.with, 1)
			}
			if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err = Load(root)
			if !errors.Is(err, ErrDuplicateID) || !strings.Contains(err.Error(), item.word) {
				t.Fatalf("%s with the identifier of another object: %v", name, err)
			}
		})
	}
}
