package metadata

import (
	"fmt"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
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
			{Name: "number", Type: documentNumberSQLType(definition.Number), Nullable: false},
			{Name: "number_period", Type: "integer", Nullable: false, Default: "0"},
			{Name: "date", Type: "timestamp with time zone", Nullable: false},
			{Name: "posted", Type: "boolean", Nullable: false, Default: "false"},
		},
		Constraints: []schemadiff.Constraint{{Name: physicalObjectName("pk", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (ref)"}},
		Indexes: []schemadiff.Index{{
			Name: physicalObjectName("id", definition.ID), Method: "btree", Keys: []string{"date DESC", "ref DESC"},
		}},
	}
	if definition.Number.Unique {
		table.Constraints = append(table.Constraints, schemadiff.Constraint{
			Name: physicalObjectName("uq", definition.ID), Type: "unique", Definition: "UNIQUE (number_period, number)",
		})
	} else {
		table.Indexes = append(table.Indexes, schemadiff.Index{
			Name: physicalObjectName("in", definition.ID), Method: "btree", Keys: []string{"number_period", "number"},
		})
	}
	for _, attribute := range definition.Attributes {
		if err := catalog.appendAttributeSchema(&table, attribute); err != nil {
			return schemadiff.Table{}, nil, fmt.Errorf("document %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
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
	return fmt.Sprintf("character varying(%d)", number.Length)
}
