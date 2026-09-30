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
	if definition.Kind == AccumulationRegisterBalance {
		movements.Columns = append(movements.Columns, schemadiff.Column{Name: "movement_kind", Type: "smallint", Nullable: false})
		movements.Constraints = append(movements.Constraints, schemadiff.Constraint{
			// PostgreSQL rewrites a scalar IN-list against constants into
			// "= ANY (ARRAY[...])" internally and echoes that form back from
			// pg_get_constraintdef - writing it that way here keeps every
			// re-application of an unchanged schema a true no-op instead of
			// a spurious "replace constraint" (destructive) diff each time.
			Name: physicalObjectName("cm", definition.ID), Type: "check", Definition: "CHECK (movement_kind = ANY (ARRAY[1, 2]))",
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
			// The combination comes first and the row number last, because
			// that is the order everything asks in: a write looks for a free
			// row of one combination, and so does the statement that makes
			// another one. With the number in the middle - where it sat while
			// it was a hash and every statement knew it in advance - the same
			// lookup could only use the period and had to read a whole month
			// of rows to find one combination.
			{Name: physicalObjectName("pt", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (total_period, dimension_key, totals_split)"},
			// No ceiling on the number. Rows of totals appear when writers
			// collide, and their count follows the concurrency the base has
			// actually seen: the prototype has as many rows per combination as
			// the most transactions that ever wrote it at once. A ceiling of sixteen was the old
			// mechanism's, where the number addressed one of sixteen rows
			// chosen by a hash; here the seventeenth concurrent writer would
			// simply be refused.
			{Name: physicalObjectName("ct", definition.ID), Type: "check", Definition: "CHECK (totals_split >= 0)"},
		},
	}
	for _, dimension := range definition.Dimensions {
		if err := catalog.appendInformationRegisterField(&movements, dimension.Attribute, true); err != nil {
			return schemadiff.Table{}, schemadiff.Table{}, fmt.Errorf("accumulation register %s dimension %s: %w", definition.Name, dimension.Name, err)
		}
		// The totals keep the dimension as a plain column and never index it:
		// a row of totals is found by its combination, and the combination is
		// hashed into a key of its own - see the primary key of the table.
		totalDimension := dimension.Attribute
		totalDimension.Indexing = DontIndex
		if err := catalog.appendAttributeSchema(&totals, totalDimension); err != nil {
			return schemadiff.Table{}, schemadiff.Table{}, fmt.Errorf("accumulation register %s total dimension %s: %w", definition.Name, dimension.Name, err)
		}
	}
	if definition.Kind == AccumulationRegisterBalance {
		// A balance is read at one period - a month start or the present
		// totals - and filtered by dimensions inside it; see
		// balanceTotalsIndexKeys. A turnover register's totals are not read
		// by anything yet, and an index nobody reads is a write everybody
		// pays for.
		keys, err := catalog.balanceTotalsIndexKeys(definition)
		if err != nil {
			return schemadiff.Table{}, schemadiff.Table{}, fmt.Errorf("accumulation register %s totals index: %w", definition.Name, err)
		}
		totals.Indexes = append(totals.Indexes, schemadiff.Index{Name: physicalObjectName("it", definition.ID), Method: "btree", Keys: keys})
		dimensionIndexes, err := catalog.balanceTotalsDimensionIndexes(definition)
		if err != nil {
			return schemadiff.Table{}, schemadiff.Table{}, fmt.Errorf("accumulation register %s totals index: %w", definition.Name, err)
		}
		totals.Indexes = append(totals.Indexes, dimensionIndexes...)
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
