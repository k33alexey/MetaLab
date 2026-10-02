package metadata

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// Some properties are carried as they were written and mean something else
// when the application runs: a bound written on a field that is not a number,
// a filling value of a type the field does not hold, a command drawn as a
// picture it does not have, the level a hierarchical dimension table was saved
// with. Each has one reader that gives the meaning, and code that reads the
// property itself gets it wrong without an error - a bound with a comma read
// as no bound, a reference filled into a string, an empty button, a query on
// one level of a hierarchy instead of all of them.
//
// Nothing in the compiler stops that, so the rule is held here: outside the
// code that checks, decodes or copies the model, the property is read only by
// its reader. The defect caught is new code that reads one directly.
var effectiveReaders = []struct {
	// selector is the property as code names it: one field, or an owner and
	// a field, as Filling.Value.
	selector []string
	reader   string
}{
	{[]string{"MinValue"}, "NumberBound"},
	{[]string{"MaxValue"}, "NumberBound"},
	{[]string{"Filling", "Value"}, "EffectiveFillingValue"},
	{[]string{"Representation"}, "CommandRepresentation.ShownAs"},
	{[]string{"LevelNumber"}, "ExternalDimensionTable.EffectiveLevelNumber"},
}

// mayReadAsWritten says the function deals with the model as it is written:
// checks it, reads it from a file, copies it - or is a reader itself.
func mayReadAsWritten(function string) bool {
	for _, prefix := range []string{"validate", "Validate", "Decode", "decode", "clone"} {
		if strings.HasPrefix(function, prefix) {
			return true
		}
	}
	switch function {
	case "NumberBound", "EffectiveFillingValue", "ShownAs", "EffectiveLevelNumber":
		return true
	}
	return false
}

func TestCarriedPropertiesAreReadThroughTheirReaders(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	var found []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "docs", "bin", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil || mayReadAsWritten(function.Name.Name) {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				for _, guarded := range effectiveReaders {
					if !selects(selector, guarded.selector) {
						continue
					}
					found = append(found, fileSet.Position(selector.Sel.Pos()).String()+
						" reads "+strings.Join(guarded.selector, ".")+" in "+function.Name.Name+"; read it through "+guarded.reader)
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, place := range found {
		t.Error(place)
	}
}

// selects says the expression ends in the names given, owner first.
func selects(selector *ast.SelectorExpr, names []string) bool {
	if selector.Sel.Name != names[len(names)-1] {
		return false
	}
	if len(names) == 1 {
		return true
	}
	owner, ok := selector.X.(*ast.SelectorExpr)
	return ok && selects(owner, names[:len(names)-1])
}
