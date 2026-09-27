package metadata

import (
	"strings"
	"testing"
)

const (
	tablePartPropsID    = "40000000-0000-4000-8000-000000000061"
	tablePartPropsPart  = "40000000-0000-4000-8000-000000000062"
	tablePartPropsField = "40000000-0000-4000-8000-000000000063"
)

func catalogWithTablePartYAML(hierarchy, part string) string {
	return `format: 1
id: ` + tablePartPropsID + `
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
` + hierarchy + `table_parts:
  - id: ` + tablePartPropsPart + `
    name: Состав
    title: {ru: Состав}
` + part + `    attributes:
      - id: ` + tablePartPropsField + `
        name: Материал
        title: {ru: Материал}
        types: [{kind: string, length: 50}]
`
}

// A table part carries five settings beyond its name and its fields, and the
// model carried none of them. All five have to come back out of the project.
func TestTablePartKeepsItsOwnProperties(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, tablePartPropsID, catalogWithTablePartYAML(
		"hierarchy: {enabled: true, kind: folders-and-items}\n",
		`    comment: состав изделия
    tooltip: {ru: Из чего изделие собрано}
    fill_checking: show-error
    line_number_length: 7
    use: for-folder-and-item
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	definition, ok := catalog.CatalogDefinition("номенклатура")
	if !ok || len(definition.TableParts) != 1 {
		t.Fatalf("catalog = %+v, found=%v", definition.TableParts, ok)
	}
	part := definition.TableParts[0]
	if part.Comment != "состав изделия" || part.ToolTip["ru"] != "Из чего изделие собрано" ||
		part.FillChecking != ShowFillingError || part.LineNumberLength != 7 || part.Use != UseForFolderAndItem {
		t.Fatalf("table part = %+v", part)
	}
	// The tooltip is localized, so a lookup must hand out a copy of it.
	definition.TableParts[0].ToolTip["ru"] = "Изменено"
	again, _ := catalog.CatalogDefinition("Номенклатура")
	if again.TableParts[0].ToolTip["ru"] != "Из чего изделие собрано" {
		t.Fatal("the catalog handed out its own table part tooltip")
	}
}

// The width a line number is stored in is 5 to 9 - the help gives the range
// outright, and all 164 parts of the demonstration configuration that state it
// say 5.
func TestLineNumberLengthIsBoundedByThePrototype(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		length  string
		accepts bool
	}{
		"пять — минимум":    {"5", true},
		"девять — максимум": {"9", true},
		"четыре — мало":     {"4", false},
		"десять — много":    {"10", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, CatalogKind, tablePartPropsID,
				catalogWithTablePartYAML("", "    line_number_length: "+want.length+"\n"))
			_, err := Load(root)
			switch {
			case want.accepts && err != nil:
				t.Fatalf("refused: %v", err)
			case !want.accepts && (err == nil || !strings.Contains(err.Error(), "line_number_length must be 5..9")):
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// The width is about storage, and the rows of a report's table part are never
// stored - they live as long as the report runs. The help lists the kinds that
// have the property and names neither reports nor data processors, and the export
// agrees exactly: of 185 parts, the 21 without a width are the 20 data processors
// and the one report.
func TestARunningObjectsTablePartHasNoStoredLineNumber(t *testing.T) {
	t.Parallel()
	for name, kind := range map[string]Kind{"отчёт": ReportKind, "обработка": DataProcessorKind} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, kind, tablePartPropsID, `format: 1
id: `+tablePartPropsID+`
name: Ведомость
title: {ru: Ведомость}
table_parts:
  - id: `+tablePartPropsPart+`
    name: Состав
    title: {ru: Состав}
    line_number_length: 5
    attributes:
      - id: `+tablePartPropsField+`
        name: Материал
        title: {ru: Материал}
        types: [{kind: string, length: 50}]
`)
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), "never stored") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// Whom a part belongs to is a catalog's and a chart of characteristic types'
// setting, the same as for an attribute. The prototype writes it on 111 parts of
// the demonstration configuration and every one of them belongs to those two
// kinds.
func TestTablePartUseBelongsToTheTwoKindsThatHaveIt(t *testing.T) {
	t.Parallel()
	t.Run("справочник с группами", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeMetadata(t, root, CatalogKind, tablePartPropsID, catalogWithTablePartYAML(
			"hierarchy: {enabled: true, kind: folders-and-items}\n", "    use: for-folder\n"))
		if _, err := Load(root); err != nil {
			t.Fatalf("refused: %v", err)
		}
	})
	// Reaching folders on an object that has none is a setting on nothing.
	t.Run("справочник без групп не достаёт до групп", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeMetadata(t, root, CatalogKind, tablePartPropsID,
			catalogWithTablePartYAML("", "    use: for-folder\n"))
		_, err := Load(root)
		if err == nil || !strings.Contains(err.Error(), "reaches folders") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("у документа такого свойства нет", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeMetadata(t, root, DocumentKind, documentID, `format: 1
id: `+documentID+`
name: Накладная
title: {ru: Накладная}
number: {type: string, length: 11, auto: true, periodicity: year}
table_parts:
  - id: `+tablePartPropsPart+`
    name: Состав
    title: {ru: Состав}
    use: for-item
    attributes:
      - id: `+tablePartPropsField+`
        name: Материал
        title: {ru: Материал}
        types: [{kind: string, length: 50}]
`)
		_, err := Load(root)
		if err == nil || !strings.Contains(err.Error(), "catalog or a chart of characteristic types") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("четвёртого значения нет", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeMetadata(t, root, CatalogKind, tablePartPropsID,
			catalogWithTablePartYAML("", "    use: for-everything\n"))
		_, err := Load(root)
		if err == nil || !strings.Contains(err.Error(), "use must be for-item") {
			t.Fatalf("err = %v", err)
		}
	})
}

// Being included in the automatic check of filling is a setting with two values
// and no third.
func TestTablePartFillCheckingTakesItsTwoValues(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		value   string
		accepts bool
	}{
		"не проверять":    {"dont-check", true},
		"показать ошибку": {"show-error", true},
		"третьего нет":    {"warn", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, CatalogKind, tablePartPropsID,
				catalogWithTablePartYAML("", "    fill_checking: "+want.value+"\n"))
			_, err := Load(root)
			switch {
			case want.accepts && err != nil:
				t.Fatalf("refused: %v", err)
			case !want.accepts && (err == nil || !strings.Contains(err.Error(), "fill_checking must be")):
				t.Fatalf("err = %v", err)
			}
		})
	}
}
