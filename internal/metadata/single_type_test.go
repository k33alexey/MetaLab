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

// A type description may be empty - an attribute of a report or a data
// processor left without a type is arbitrary - so code that takes the first
// type by index panics on it. Nothing in the compiler stops that, and a test
// sees it only if it happens to load such an attribute. So the rule is held
// here: outside SingleType, no list named Types or types is indexed or sliced in any
// source file of the repository. The defect caught is new code that indexes
// one directly, which panics on the first untyped attribute it meets.
func TestNoTypeListIsIndexedDirectly(t *testing.T) {
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
		ast.Inspect(file, func(node ast.Node) bool {
			if function, ok := node.(*ast.FuncDecl); ok && function.Name.Name == "SingleType" && function.Recv == nil {
				return false
			}
			var list ast.Expr
			switch access := node.(type) {
			case *ast.IndexExpr:
				list = access.X
			case *ast.SliceExpr:
				// types[1:] panics on an empty list just as types[0] does.
				list = access.X
			default:
				return true
			}
			name := ""
			switch target := list.(type) {
			case *ast.Ident:
				name = target.Name
			case *ast.SelectorExpr:
				name = target.Sel.Name
			}
			if name == "Types" || name == "types" {
				found = append(found, fileSet.Position(node.Pos()).String())
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, place := range found {
		t.Errorf("%s indexes a type description directly; an empty one panics here - use SingleType", place)
	}
}

func TestSingleTypeIsOnlyExactlyOne(t *testing.T) {
	t.Parallel()
	if _, ok := SingleType(nil); ok {
		t.Fatal("an empty type description was taken for one type")
	}
	if _, ok := SingleType([]Type{{Kind: StringType}, {Kind: NumberType}}); ok {
		t.Fatal("a composite type was taken for one type")
	}
	if single, ok := SingleType([]Type{{Kind: BooleanType}}); !ok || single.Kind != BooleanType {
		t.Fatalf("one type was not returned: %v %v", single, ok)
	}
}
