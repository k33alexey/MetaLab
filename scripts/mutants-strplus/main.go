// Command mutants-strplus prints file:line:col of every + and += on strings in
// the package directory given. gremlins 0.6.0 turns such a + into -, which
// does not build, and reports the mutant KILLED; scripts/mutants_recheck.py
// uses this list to take them out of the count.
package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	dir := os.Args[1]
	fset := token.NewFileSet()
	names, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		panic(err)
	}
	pkgs := map[string][]*ast.File{}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			panic(err)
		}
		pkgs[f.Name.Name] = append(pkgs[f.Name.Name], f)
	}
	for pkgName, files := range pkgs {
		info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
		conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil), Error: func(error) {}}
		conf.Check(pkgName, fset, files, info)
		isString := func(e ast.Expr) bool {
			t := info.TypeOf(e)
			if t == nil {
				return false
			}
			b, ok := t.Underlying().(*types.Basic)
			return ok && b.Info()&types.IsString != 0
		}
		for _, f := range files {
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.BinaryExpr:
					if x.Op == token.ADD && isString(x.X) {
						p := fset.Position(x.OpPos)
						fmt.Printf("%s:%d:%d\n", filepath.Base(p.Filename), p.Line, p.Column)
					}
				case *ast.AssignStmt:
					if x.Tok == token.ADD_ASSIGN && isString(x.Lhs[0]) {
						p := fset.Position(x.TokPos)
						fmt.Printf("%s:%d:%d\n", filepath.Base(p.Filename), p.Line, p.Column)
					}
				}
				return true
			})
		}
	}
}
