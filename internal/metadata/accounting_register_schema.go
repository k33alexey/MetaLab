package metadata

import (
	"fmt"
	"strings"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// PhysicalAccountingRegisterTable returns the stable table name for entries.
func PhysicalAccountingRegisterTable(id uuid.UUID) (string, error) { return schemadiff.TableName(id) }

// sideColumn names one side of a field kept twice. A field that differs
// between debit and credit needs two columns, and both have to be derived from
// the one identifier the field has, so that renaming nothing renames nothing.
func sideColumn(id uuid.UUID, debit bool) (string, error) {
	if id.IsZero() {
		return "", fmt.Errorf("physical PostgreSQL name requires a non-zero UUID")
	}
	prefix := "cc"
	if debit {
		prefix = "cd"
	}
	return prefix + "_" + strings.ReplaceAll(id.String(), "-", ""), nil
}

// extDimensionColumn names one slot of analytics. An account carries a fixed
// number of them, and each slot holds two things: which kind of analytics it
// is and the value itself.
func extDimensionColumn(slot int, debit, value bool, correspondence bool) string {
	name := fmt.Sprintf("ext%d", slot)
	if correspondence {
		if debit {
			name += "_dr"
		} else {
			name += "_cr"
		}
	}
	if value {
		return name + "_value"
	}
	return name + "_kind"
}

func (catalog *Catalog) accountingRegisterTable(definition AccountingRegisterDefinition) (schemadiff.Table, error) {
	tableName, err := PhysicalAccountingRegisterTable(definition.ID)
	if err != nil {
		return schemadiff.Table{}, err
	}
	table := schemadiff.Table{
		Name: tableName,
		Columns: []schemadiff.Column{
			{Name: "record_id", Type: "uuid", Nullable: false},
			{Name: "period", Type: "timestamp with time zone", Nullable: false},
			{Name: "recorder_type", Type: "uuid", Nullable: false},
			{Name: "recorder_ref", Type: "uuid", Nullable: false},
			{Name: "line_no", Type: "integer", Nullable: false},
			{Name: "active", Type: "boolean", Nullable: false, Default: "true"},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("pk", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (record_id)"},
			{Name: physicalObjectName("ur", definition.ID), Type: "unique", Definition: "UNIQUE (recorder_type, recorder_ref, line_no)"},
		},
		Indexes: []schemadiff.Index{
			{Name: physicalObjectName("ip", definition.ID), Method: "btree", Keys: []string{"period", "record_id"}},
			{Name: physicalObjectName("ir", definition.ID), Method: "btree", Keys: []string{"recorder_type", "recorder_ref", "line_no"}},
		},
	}
	if definition.TotalsSplitting {
		table.Columns = append(table.Columns, schemadiff.Column{Name: "totals_split", Type: "smallint", Nullable: false, Default: "0"})
		table.Constraints = append(table.Constraints, schemadiff.Constraint{
			Name: physicalObjectName("cs", definition.ID), Type: "check", Definition: "CHECK (totals_split >= 0 AND totals_split < 16)",
		})
	}
	accountsTable, err := PhysicalCatalogTable(definition.ChartOfAccounts)
	if err != nil {
		return schemadiff.Table{}, err
	}
	// One account per side under double entry, one account in total without
	// it. Both sides point at the same chart: an entry between two charts of
	// accounts is not an entry.
	accounts := []struct{ column, tag string }{{"account", "a"}}
	if definition.Correspondence {
		accounts = []struct{ column, tag string }{{"account_dr", "d"}, {"account_cr", "c"}}
	}
	for _, account := range accounts {
		table.Columns = append(table.Columns, schemadiff.Column{Name: account.column, Type: "uuid", Nullable: false})
		table.Constraints = append(table.Constraints, schemadiff.Constraint{
			Name: physicalObjectName("f"+account.tag, definition.ID), Type: "foreign_key",
			Definition: "FOREIGN KEY (" + account.column + ") REFERENCES " + schemadiff.ApplicationSchema + "." + accountsTable + "(ref) DEFERRABLE INITIALLY DEFERRED",
		})
		table.Indexes = append(table.Indexes, schemadiff.Index{
			Name: physicalObjectName("ia"+account.tag, definition.ID), Method: "btree", Keys: []string{account.column},
		})
	}
	for _, group := range [][]AccountingRegisterField{definition.Dimensions, definition.Resources} {
		for _, field := range group {
			if err := catalog.appendAccountingField(&table, definition, field); err != nil {
				return schemadiff.Table{}, err
			}
		}
	}
	for _, attribute := range definition.Attributes {
		if err := catalog.appendAttributeSchema(&table, attribute); err != nil {
			return schemadiff.Table{}, fmt.Errorf("accounting register %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	catalog.appendExtDimensionColumns(&table, definition)
	return table, nil
}

func (catalog *Catalog) appendAccountingField(table *schemadiff.Table, definition AccountingRegisterDefinition, field AccountingRegisterField) error {
	storage, err := catalog.attributeStorage(field.Types)
	if err != nil {
		return fmt.Errorf("accounting register %s field %s: %w", definition.Name, field.Name, err)
	}
	// A balance field is one value for the whole entry; a field that is not
	// keeps a value per side, because that is what "not the same on both
	// sides" means.
	columns := []string{}
	if !definition.Correspondence || field.Balance {
		name, err := PhysicalAttributeColumn(field.ID)
		if err != nil {
			return err
		}
		columns = append(columns, name)
	} else {
		for _, debit := range []bool{true, false} {
			name, err := sideColumn(field.ID, debit)
			if err != nil {
				return err
			}
			columns = append(columns, name)
		}
	}
	for index, name := range columns {
		table.Columns = append(table.Columns, schemadiff.Column{Name: name, Type: storage.sqlType, Nullable: true})
		if field.Indexed {
			table.Indexes = append(table.Indexes, schemadiff.Index{
				Name: physicalObjectName(fmt.Sprintf("i%d", index), field.ID), Method: "btree", Keys: []string{name},
			})
		}
	}
	return nil
}

// appendExtDimensionColumns lays out the analytics of an entry. How many slots
// there are is the chart's answer, not the register's: the register has no ext
// dimensions of its own, it carries whatever the accounts of its chart carry.
func (catalog *Catalog) appendExtDimensionColumns(table *schemadiff.Table, definition AccountingRegisterDefinition) {
	index, ok := catalog.chartOfAccountsByID[definition.ChartOfAccounts]
	if !ok {
		return
	}
	chart := catalog.ChartsOfAccounts[index]
	if chart.ExtDimensionTypes == nil || chart.MaxExtDimensionCount == 0 {
		return
	}
	sides := []bool{true}
	if definition.Correspondence {
		sides = []bool{true, false}
	}
	for slot := 1; slot <= chart.MaxExtDimensionCount; slot++ {
		for _, debit := range sides {
			kind := extDimensionColumn(slot, debit, false, definition.Correspondence)
			value := extDimensionColumn(slot, debit, true, definition.Correspondence)
			// The kind of analytics is a characteristic of the chart's own
			// chart of characteristic types; the value is whatever that
			// characteristic allows, so it is stored the way a composite value
			// is stored.
			table.Columns = append(table.Columns,
				schemadiff.Column{Name: kind, Type: "uuid", Nullable: true},
				schemadiff.Column{Name: value, Type: "jsonb", Nullable: true},
			)
			table.Indexes = append(table.Indexes, schemadiff.Index{
				Name: physicalObjectName(strings.ReplaceAll(kind, "_", ""), definition.ID), Method: "btree", Keys: []string{kind},
			})
		}
	}
}
