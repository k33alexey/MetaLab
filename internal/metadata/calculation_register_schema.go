package metadata

import (
	"fmt"
	"strings"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// PhysicalCalculationRegisterTable returns the stable table name for records.
func PhysicalCalculationRegisterTable(id uuid.UUID) (string, error) { return schemadiff.TableName(id) }

// PhysicalRecalculationTable returns the stable table name for one
// recalculation. A recalculation keeps its own rows: which records are waiting
// to be computed again.
func PhysicalRecalculationTable(id uuid.UUID) (string, error) {
	if id.IsZero() {
		return "", fmt.Errorf("physical PostgreSQL name requires a non-zero UUID")
	}
	return "tr_" + strings.ReplaceAll(id.String(), "-", ""), nil
}

func (catalog *Catalog) calculationRegisterTables(definition CalculationRegisterDefinition) ([]schemadiff.Table, error) {
	tableName, err := PhysicalCalculationRegisterTable(definition.ID)
	if err != nil {
		return nil, err
	}
	table := schemadiff.Table{
		Name: tableName,
		Columns: []schemadiff.Column{
			{Name: "record_id", Type: "uuid", Nullable: false},
			// The period a record is registered in, as opposed to the period
			// it acts over. Keeping them apart is what makes it possible to
			// enter in April something that acts over March.
			{Name: "period", Type: "timestamp with time zone", Nullable: false},
			{Name: "recorder_type", Type: "uuid", Nullable: false},
			{Name: "recorder_ref", Type: "uuid", Nullable: false},
			{Name: "line_no", Type: "integer", Nullable: false},
			{Name: "active", Type: "boolean", Nullable: false, Default: "true"},
			{Name: "calculation_type", Type: "uuid", Nullable: false},
			// Reversal is not a movement with the opposite sign: it is a mark
			// on the record, and a reversing record counts its period as
			// negative.
			{Name: "reversing", Type: "boolean", Nullable: false, Default: "false"},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("pk", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (record_id)"},
			{Name: physicalObjectName("ur", definition.ID), Type: "unique", Definition: "UNIQUE (recorder_type, recorder_ref, line_no)"},
		},
		Indexes: []schemadiff.Index{
			{Name: physicalObjectName("ip", definition.ID), Method: "btree", Keys: []string{"period", "record_id"}},
			{Name: physicalObjectName("ir", definition.ID), Method: "btree", Keys: []string{"recorder_type", "recorder_ref", "line_no"}},
			{Name: physicalObjectName("ic", definition.ID), Method: "btree", Keys: []string{"calculation_type"}},
		},
	}
	kinds, err := PhysicalCatalogTable(definition.ChartOfCalculationTypes)
	if err != nil {
		return nil, err
	}
	table.Constraints = append(table.Constraints, schemadiff.Constraint{
		Name: physicalObjectName("fc", definition.ID), Type: "foreign_key",
		Definition: "FOREIGN KEY (calculation_type) REFERENCES " + schemadiff.ApplicationSchema + "." + kinds + "(ref) DEFERRABLE INITIALLY DEFERRED",
	})
	// Each period that exists is two columns, because a period is an interval
	// and an interval without its end is a date.
	if definition.ActionPeriod {
		table.Columns = append(table.Columns,
			schemadiff.Column{Name: "action_period_start", Type: "timestamp with time zone", Nullable: false},
			schemadiff.Column{Name: "action_period_end", Type: "timestamp with time zone", Nullable: false},
		)
		table.Constraints = append(table.Constraints, schemadiff.Constraint{
			Name: physicalObjectName("ca", definition.ID), Type: "check", Definition: "CHECK (action_period_end >= action_period_start)",
		})
		table.Indexes = append(table.Indexes, schemadiff.Index{
			Name: physicalObjectName("ia", definition.ID), Method: "btree", Keys: []string{"action_period_start", "action_period_end"},
		})
	}
	if definition.BasePeriod {
		table.Columns = append(table.Columns,
			schemadiff.Column{Name: "base_period_start", Type: "timestamp with time zone", Nullable: false},
			schemadiff.Column{Name: "base_period_end", Type: "timestamp with time zone", Nullable: false},
		)
		table.Constraints = append(table.Constraints, schemadiff.Constraint{
			Name: physicalObjectName("cb", definition.ID), Type: "check", Definition: "CHECK (base_period_end >= base_period_start)",
		})
	}
	for _, field := range calculationRegisterFields(definition) {
		if err := catalog.appendAttributeSchema(&table, field); err != nil {
			return nil, fmt.Errorf("calculation register %s field %s: %w", definition.Name, field.Name, err)
		}
	}
	for _, attribute := range definition.Attributes {
		if err := catalog.appendAttributeSchema(&table, attribute); err != nil {
			return nil, fmt.Errorf("calculation register %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	tables := []schemadiff.Table{table}
	for _, recalculation := range definition.Recalculations {
		recalculated, err := catalog.recalculationTable(definition, recalculation)
		if err != nil {
			return nil, err
		}
		tables = append(tables, recalculated)
	}
	return tables, nil
}

// recalculationTable holds what is waiting to be computed again: which record
// made the entry, which kind of accrual it was, and the values of the
// dimensions the recalculation is found by.
func (catalog *Catalog) recalculationTable(definition CalculationRegisterDefinition, recalculation Recalculation) (schemadiff.Table, error) {
	tableName, err := PhysicalRecalculationTable(recalculation.ID)
	if err != nil {
		return schemadiff.Table{}, err
	}
	table := schemadiff.Table{
		Name: tableName,
		Columns: []schemadiff.Column{
			{Name: "record_id", Type: "uuid", Nullable: false},
			{Name: "recorder_type", Type: "uuid", Nullable: false},
			{Name: "recorder_ref", Type: "uuid", Nullable: false},
			{Name: "calculation_type", Type: "uuid", Nullable: false},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("pk", recalculation.ID), Type: "primary_key", Definition: "PRIMARY KEY (record_id)"},
		},
		Indexes: []schemadiff.Index{
			{Name: physicalObjectName("ir", recalculation.ID), Method: "btree", Keys: []string{"recorder_type", "recorder_ref"}},
		},
	}
	byID := map[uuid.UUID]CalculationRegisterDimension{}
	for _, dimension := range definition.Dimensions {
		byID[dimension.ID] = dimension
	}
	for _, dimension := range recalculation.Dimensions {
		source, ok := byID[dimension.RegisterDimension]
		if !ok {
			return schemadiff.Table{}, fmt.Errorf("recalculation %s dimension %s corresponds to nothing in %s",
				recalculation.Name, dimension.Name, definition.Name)
		}
		// The column takes its own identifier and the register dimension's
		// types: it holds the same values, for the same reason.
		if err := catalog.appendAttributeSchema(&table, Attribute{
			ID: dimension.ID, Name: dimension.Name, Title: dimension.Title, Types: source.Types, Indexing: IndexField,
		}); err != nil {
			return schemadiff.Table{}, fmt.Errorf("recalculation %s dimension %s: %w", recalculation.Name, dimension.Name, err)
		}
	}
	return table, nil
}
