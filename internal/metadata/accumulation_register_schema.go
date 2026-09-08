package metadata

import (
	"fmt"
	"strings"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// PhysicalAccumulationRegisterTable returns the stable movement table name.
func PhysicalAccumulationRegisterTable(id uuid.UUID) (string, error) { return schemadiff.TableName(id) }

// PhysicalAccumulationRegisterTotalsTable returns the stable monthly totals table name.
func PhysicalAccumulationRegisterTotalsTable(id uuid.UUID) (string, error) {
	if id.IsZero() {
		return "", fmt.Errorf("physical PostgreSQL name requires a non-zero UUID")
	}
	return "ta_" + strings.ReplaceAll(id.String(), "-", ""), nil
}

func (catalog *Catalog) accumulationRegisterTables(definition AccumulationRegisterDefinition) (schemadiff.Table, schemadiff.Table, error) {
	movementName, err := PhysicalAccumulationRegisterTable(definition.ID)
	if err != nil {
		return schemadiff.Table{}, schemadiff.Table{}, err
	}
	totalsName, err := PhysicalAccumulationRegisterTotalsTable(definition.ID)
	if err != nil {
		return schemadiff.Table{}, schemadiff.Table{}, err
	}
	movements := schemadiff.Table{
		Name: movementName,
		Columns: []schemadiff.Column{
			{Name: "record_id", Type: "uuid", Nullable: false},
			{Name: "dimension_key", Type: "character(64)", Nullable: false},
			{Name: "period", Type: "timestamp with time zone", Nullable: false},
			{Name: "recorder_type", Type: "uuid", Nullable: false},
			{Name: "recorder_ref", Type: "uuid", Nullable: false},
			{Name: "line_no", Type: "integer", Nullable: false},
			{Name: "active", Type: "boolean", Nullable: false, Default: "true"},
			{Name: "totals_split", Type: "smallint", Nullable: false},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("pk", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (record_id)"},
			{Name: physicalObjectName("ur", definition.ID), Type: "unique", Definition: "UNIQUE (recorder_type, recorder_ref, line_no)"},
			{Name: physicalObjectName("cs", definition.ID), Type: "check", Definition: "CHECK (totals_split >= 0 AND totals_split < 16)"},
		},
		Indexes: []schemadiff.Index{
			{Name: physicalObjectName("ip", definition.ID), Method: "btree", Keys: []string{"period", "record_id"}},
			{Name: physicalObjectName("ir", definition.ID), Method: "btree", Keys: []string{"recorder_type", "recorder_ref", "line_no"}},
		},
	}
	if definition.Kind == AccumulationRegisterBalance {
		movements.Columns = append(movements.Columns, schemadiff.Column{Name: "movement_kind", Type: "smallint", Nullable: false})
		movements.Constraints = append(movements.Constraints, schemadiff.Constraint{
			Name: physicalObjectName("cm", definition.ID), Type: "check", Definition: "CHECK (movement_kind IN (1, 2))",
		})
	}
	totals := schemadiff.Table{
		Name: totalsName,
		Columns: []schemadiff.Column{
			{Name: "total_period", Type: "timestamp with time zone", Nullable: false},
			{Name: "totals_split", Type: "smallint", Nullable: false},
			{Name: "dimension_key", Type: "character(64)", Nullable: false},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("pt", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (total_period, totals_split, dimension_key)"},
			{Name: physicalObjectName("ct", definition.ID), Type: "check", Definition: "CHECK (totals_split >= 0 AND totals_split < 16)"},
		},
	}
	for _, dimension := range definition.Dimensions {
		if err := catalog.appendInformationRegisterField(&movements, dimension, true); err != nil {
			return schemadiff.Table{}, schemadiff.Table{}, fmt.Errorf("accumulation register %s dimension %s: %w", definition.Name, dimension.Name, err)
		}
		totalDimension := dimension
		totalDimension.Indexed = false
		if err := catalog.appendAttributeSchema(&totals, totalDimension); err != nil {
			return schemadiff.Table{}, schemadiff.Table{}, fmt.Errorf("accumulation register %s total dimension %s: %w", definition.Name, dimension.Name, err)
		}
	}
	for _, resource := range definition.Resources {
		if err := catalog.appendAttributeSchema(&movements, resource); err != nil {
			return schemadiff.Table{}, schemadiff.Table{}, fmt.Errorf("accumulation register %s resource %s: %w", definition.Name, resource.Name, err)
		}
		column, _ := PhysicalAttributeColumn(resource.ID)
		totals.Columns = append(totals.Columns, schemadiff.Column{Name: column, Type: "numeric", Nullable: false, Default: "0"})
	}
	for _, attribute := range definition.Attributes {
		if err := catalog.appendAttributeSchema(&movements, attribute); err != nil {
			return schemadiff.Table{}, schemadiff.Table{}, fmt.Errorf("accumulation register %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	return movements, totals, nil
}
