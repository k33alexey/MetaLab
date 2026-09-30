package metadata

import (
	"strings"
	"testing"
)

const (
	firstFunction  = "f8200000-0000-4000-8000-000000000001"
	secondFunction = "f8200000-0000-4000-8000-000000000002"
)

// functionSource is a source with the functions given, written into its own
// description the way an export writes them.
func functionSource(t *testing.T, functions string) string {
	t.Helper()
	root := metadataProject(t)
	writeExternalSource(t, root, warehouseSource, "Склад", "functions:\n"+functions)
	return root
}

// A function lies in the description of its source and carries its expression,
// whether it returns a value, and the value's type. Catches a function read
// from a folder of its own, which is not where an export writes it, and a
// property lost on the way.
func TestExternalFunctionIsCarriedInItsSource(t *testing.T) {
	t.Parallel()
	root := functionSource(t, `  - id: `+firstFunction+`
    name: Остаток
    title: {ru: Остаток}
    return_value: true
    types: [{kind: number, precision: 15, scale: 3}]
    expression_in_data_source: "SELECT SUM(quantity) FROM dbo.Stock WHERE goods = &1 {AND warehouse IN (&2[])}"
  - id: `+secondFunction+`
    name: Пересчитать
    title: {ru: Пересчитать}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, _ := catalog.ExternalDataSource("Склад")
	if len(source.Functions) != 2 {
		t.Fatalf("a function was lost: %+v", source.Functions)
	}
	balance := source.Functions[0]
	if !balance.ReturnValue || len(balance.Types) != 1 || !strings.Contains(balance.ExpressionInDataSource, "&2[]") {
		t.Fatalf("a property of the function was lost: %+v", balance)
	}
	// A function made by hand has no expression yet: the configurator lets it
	// be written later.
	if recount := source.Functions[1]; recount.ExpressionInDataSource != "" || recount.ReturnValue {
		t.Fatalf("a function without an expression came back as %+v", recount)
	}
	source.Functions[0].Types[0].Precision = 1
	if again, _ := catalog.ExternalDataSource("Склад"); again.Functions[0].Types[0].Precision != 15 {
		t.Fatal("changing a copy changed the catalog")
	}
}

// The parameters of an expression are checked for what the notation says
// unambiguously and nothing more. Catches a parameter counted from zero, a
// variable number of values that is not the last parameter, and - the other
// direction - braces and ampersands of the other database's own language
// refused as if they were ours.
func TestExternalFunctionParametersAreCheckedByTheNotationOnly(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ expression, says string }{
		"a parameter counted from zero":   {"SELECT &0", "&0 is not a parameter"},
		"a variable number not last":      {"SELECT f(&1[], &2)", "only the last parameter may - the last here is &2"},
		"two parameters of variable size": {"SELECT f(&1[], &2[])", "both take a variable number of values"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := functionSource(t, "  - id: "+firstFunction+"\n    name: Ф\n    title: {ru: Ф}\n    expression_in_data_source: \""+test.expression+"\"\n")
			if message := loadRefused(t, root, name); !strings.Contains(message, test.says) {
				t.Fatalf("the error does not say what is wrong: %v", message)
			}
		})
	}
	for _, expression := range []string{
		"SELECT {fn NOW()}",            // an ODBC escape, not an optional block
		"SELECT flags & 4 FROM t",      // an operator, not a parameter
		"SELECT &1 {, &2} {, &3[]}",    // optional blocks and a variable last
		"SELECT &2, &1 FROM t WHERE {", // unbalanced braces are the other side's business
	} {
		root := functionSource(t, "  - id: "+firstFunction+"\n    name: Ф\n    title: {ru: Ф}\n    expression_in_data_source: \""+expression+"\"\n")
		if _, err := Load(root); err != nil {
			t.Errorf("the expression %q was refused: %v", expression, err)
		}
	}
}

// Functions are named within their source and identified across the
// configuration, and a function's type refers only to its own source. Catches
// two functions of one name, a function sharing an identifier with a table,
// and a result typed by a table that is not there.
func TestExternalFunctionIdentityAndTypeAreChecked(t *testing.T) {
	t.Parallel()
	twice := functionSource(t, "  - id: "+firstFunction+"\n    name: Остаток\n    title: {ru: Остаток}\n  - id: "+secondFunction+"\n    name: остаток\n    title: {ru: Остаток}\n")
	if message := loadRefused(t, twice, "two functions of one name"); !strings.Contains(message, "functions[1].name is used twice") {
		t.Fatalf("the error does not say what is wrong: %v", message)
	}
	root := metadataProject(t)
	writeExternalSource(t, root, warehouseSource, "Склад", "functions:\n  - id: "+goodsTable+"\n    name: Остаток\n    title: {ru: Остаток}\n")
	writeExternalTable(t, root, "Склад", goodsTable, "Товары", goodsBody)
	if message := loadRefused(t, root, "a function sharing a table's identifier"); !strings.Contains(message, "use "+goodsTable) {
		t.Fatalf("the error does not name the identifier: %v", message)
	}
	nothing := functionSource(t, "  - id: "+firstFunction+"\n    name: Ф\n    title: {ru: Ф}\n    return_value: true\n    types: [{kind: external-data-source-table, reference: f8200000-0000-4000-8000-0000000000ff}]\n")
	if message := loadRefused(t, nothing, "a result typed by nothing"); !strings.Contains(message, "function Ф field Ф refers to unknown table") {
		t.Fatalf("the error does not say what is wrong: %v", message)
	}
}
