package metadata

import (
	"fmt"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// PhysicalInformationRegisterTable returns the stable PostgreSQL table name for a register UUID.
func PhysicalInformationRegisterTable(id uuid.UUID) (string, error) { return schemadiff.TableName(id) }

func (catalog *Catalog) informationRegisterTable(definition InformationRegisterDefinition) (schemadiff.Table, error) {
	tableName, err := PhysicalInformationRegisterTable(definition.ID)
	if err != nil {
		return schemadiff.Table{}, err
	}
	table := schemadiff.Table{
		Name: tableName,
		Columns: []schemadiff.Column{
			{Name: "record_id", Type: "uuid", Nullable: false},
			{Name: "record_key", Type: "character(64)", Nullable: false},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("pk", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (record_id)"},
			{Name: physicalObjectName("uq", definition.ID), Type: "unique", Definition: "UNIQUE (record_key)"},
		},
	}
	if definition.Periodicity != InformationRegisterPeriodNone {
		table.Columns = append(table.Columns, schemadiff.Column{Name: "period", Type: "timestamp with time zone", Nullable: false})
		table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("ip", definition.ID), Method: "btree", Keys: []string{"period DESC", "record_id"}})
	}
	if definition.WriteMode == InformationRegisterRecorder {
		table.Columns = append(table.Columns,
			schemadiff.Column{Name: "recorder_type", Type: "uuid", Nullable: false},
			schemadiff.Column{Name: "recorder_ref", Type: "uuid", Nullable: false},
			schemadiff.Column{Name: "line_no", Type: "integer", Nullable: false},
			schemadiff.Column{Name: "active", Type: "boolean", Nullable: false, Default: "true"},
		)
		table.Constraints = append(table.Constraints, schemadiff.Constraint{
			Name: physicalObjectName("ur", definition.ID), Type: "unique", Definition: "UNIQUE (recorder_type, recorder_ref, line_no)",
		})
	}
	for _, dimension := range definition.Dimensions {
		if err := catalog.appendInformationRegisterField(&table, dimension, true); err != nil {
			return schemadiff.Table{}, fmt.Errorf("information register %s dimension %s: %w", definition.Name, dimension.Name, err)
		}
	}
	for _, resource := range definition.Resources {
		if err := catalog.appendInformationRegisterField(&table, resource, false); err != nil {
			return schemadiff.Table{}, fmt.Errorf("information register %s resource %s: %w", definition.Name, resource.Name, err)
		}
	}
	for _, attribute := range definition.Attributes {
		if err := catalog.appendInformationRegisterField(&table, attribute, false); err != nil {
			return schemadiff.Table{}, fmt.Errorf("information register %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	return table, nil
}

func (catalog *Catalog) appendInformationRegisterField(table *schemadiff.Table, field Attribute, dimension bool) error {
	before := len(table.Indexes)
	if err := catalog.appendAttributeSchema(table, field); err != nil {
		return err
	}
	if !dimension || field.Indexed {
		return nil
	}
	column, err := PhysicalAttributeColumn(field.ID)
	if err != nil {
		return err
	}
	storage, err := catalog.attributeStorage(field.Types)
	if err != nil {
		return err
	}
	method := informationRegisterIndexMethod(storage)
	table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("i", field.ID), Method: method, Keys: []string{column}})
	if len(table.Indexes) == before {
		return fmt.Errorf("failed to add dimension index")
	}
	return nil
}
