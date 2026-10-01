package metadata

import (
	"fmt"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func (catalog *Catalog) documentTables(definition DocumentDefinition) (schemadiff.Table, []schemadiff.Table, error) {
	tableName, err := PhysicalDocumentTable(definition.ID)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	table := schemadiff.Table{
		Name: tableName,
		Columns: []schemadiff.Column{
			{Name: "ref", Type: "uuid", Nullable: false},
			{Name: "version", Type: "bigint", Nullable: false, Default: "1"},
			{Name: "number_period", Type: "integer", Nullable: false, Default: "0"},
			{Name: "date", Type: "timestamp with time zone", Nullable: false},
			{Name: "posted", Type: "boolean", Nullable: false, Default: "false"},
			{Name: "deletion_mark", Type: "boolean", Nullable: false, Default: "false"},
		},
		Constraints: []schemadiff.Constraint{{Name: physicalObjectName("pk", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (ref)"}},
		Indexes: []schemadiff.Index{{
			Name: physicalObjectName("id", definition.ID), Method: "btree", Keys: []string{"date DESC", "ref DESC"},
		}, {Name: physicalObjectName("im", definition.ID), Method: "btree", Keys: []string{"deletion_mark"}}},
	}
	// A number of length 0 is switched off: no column, and nothing indexed or
	// kept unique by it - ИТС 1590 builds the number index only for a length
	// other than 0. The column has the empty number for a default, so that a
	// number lengthened from 0 can be added to a table that has rows.
	if documentHasNumber(definition) {
		column := schemadiff.Column{Name: "number", Type: documentNumberSQLType(definition.Number), Nullable: false,
			Default: emptyCodeDefault(CatalogCode{Type: definition.Number.Type, FixedLength: definition.Number.FixedLength})}
		table.Columns = append(table.Columns[:2], append([]schemadiff.Column{column}, table.Columns[2:]...)...)
		if definition.Number.Unique {
			table.Constraints = append(table.Constraints, schemadiff.Constraint{
				Name: physicalObjectName("uq", definition.ID), Type: "unique", Definition: "UNIQUE (number_period, number)",
			})
		} else {
			table.Indexes = append(table.Indexes, schemadiff.Index{
				Name: physicalObjectName("in", definition.ID), Method: "btree", Keys: []string{"number_period", "number"},
			})
		}
	}
	for _, attribute := range definition.Attributes {
		if err := catalog.appendAttributeSchema(&table, attribute); err != nil {
			return schemadiff.Table{}, nil, fmt.Errorf("document %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	numberColumns := map[string]listColumn{}
	if documentHasNumber(definition) {
		numberColumns["number"] = listColumn{name: "number", kind: definition.Number.Type}
	}
	appendListSearchIndexes(&table, definition.ID, definition.List, []string{"Number"}, definition.Attributes, numberColumns)
	parts, err := catalog.tablePartTables("document", definition.Name, definition.ID, definition.TableParts)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	return table, parts, nil
}

func documentNumberSQLType(number DocumentNumber) string {
	if number.Type == NumberType {
		return fmt.Sprintf("numeric(%d,0)", number.Length)
	}
	if number.FixedLength {
		return fmt.Sprintf("character(%d)", number.Length)
	}
	return fmt.Sprintf("character varying(%d)", number.Length)
}

// documentHasNumber says the document's number is there at all: a length of 0
// switches it off.
func documentHasNumber(definition DocumentDefinition) bool { return definition.Number.Length > 0 }

// appendNumberColumn gives the table of a business process or a task its
// number where it has one: a length of 0 switches it off - the owner checked
// both on 01.10.2026 - and the table then has no such column, no index and no
// uniqueness by it. The column's default is the empty number, so that a number
// lengthened from 0 can be added to a table that has rows.
func appendNumberColumn(table *schemadiff.Table, id uuid.UUID, number DocumentNumber) {
	if number.Length == 0 {
		return
	}
	column := schemadiff.Column{Name: "number", Type: documentNumberSQLType(number), Nullable: false,
		Default: emptyCodeDefault(CatalogCode{Type: number.Type, FixedLength: number.FixedLength})}
	table.Columns = append(table.Columns[:2], append([]schemadiff.Column{column}, table.Columns[2:]...)...)
	if number.Unique {
		table.Constraints = append(table.Constraints, schemadiff.Constraint{Name: physicalObjectName("uq", id), Type: "unique", Definition: "UNIQUE (number)"})
	} else {
		table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("ic", id), Method: "btree", Keys: []string{"number"}})
	}
}
