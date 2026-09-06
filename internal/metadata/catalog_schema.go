package metadata

import (
	"fmt"
	"strings"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// ApplicationSchema builds the exact PostgreSQL schema owned by ML metadata.
func (catalog *Catalog) ApplicationSchema() (schemadiff.Schema, error) {
	if catalog == nil {
		return schemadiff.Schema{}, fmt.Errorf("metadata catalog is required")
	}
	schema := schemadiff.Schema{Name: schemadiff.ApplicationSchema, Exists: true}
	for _, definition := range catalog.Catalogs {
		table, parts, err := catalog.catalogTables(definition)
		if err != nil {
			return schemadiff.Schema{}, err
		}
		schema.Tables = append(schema.Tables, table)
		schema.Tables = append(schema.Tables, parts...)
	}
	for _, definition := range catalog.Documents {
		table, parts, err := catalog.documentTables(definition)
		if err != nil {
			return schemadiff.Schema{}, err
		}
		schema.Tables = append(schema.Tables, table)
		schema.Tables = append(schema.Tables, parts...)
	}
	if err := schema.NormalizeAndValidate(); err != nil {
		return schemadiff.Schema{}, fmt.Errorf("build application schema: %w", err)
	}
	return schema, nil
}

// PhysicalCatalogTable returns the stable PostgreSQL table name for a catalog UUID.
func PhysicalCatalogTable(id uuid.UUID) (string, error) { return schemadiff.TableName(id) }

// PhysicalAttributeColumn returns the stable PostgreSQL column name for an attribute UUID.
func PhysicalAttributeColumn(id uuid.UUID) (string, error) { return schemadiff.ColumnName(id) }

// PhysicalDocumentTable returns the stable PostgreSQL table name for a document UUID.
func PhysicalDocumentTable(id uuid.UUID) (string, error) { return schemadiff.TableName(id) }

func (catalog *Catalog) catalogTables(definition CatalogDefinition) (schemadiff.Table, []schemadiff.Table, error) {
	tableName, err := PhysicalCatalogTable(definition.ID)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	table := schemadiff.Table{
		Name: tableName,
		Columns: []schemadiff.Column{
			{Name: "ref", Type: "uuid", Nullable: false},
			{Name: "version", Type: "bigint", Nullable: false, Default: "1"},
			{Name: "code", Type: codeSQLType(definition.Code), Nullable: false},
			{Name: "description", Type: fmt.Sprintf("character varying(%d)", definition.DescriptionLength), Nullable: false, Default: "''::character varying"},
		},
		Constraints: []schemadiff.Constraint{{Name: physicalObjectName("pk", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (ref)"}},
	}
	if definition.Code.Unique {
		table.Constraints = append(table.Constraints, schemadiff.Constraint{Name: physicalObjectName("uq", definition.ID), Type: "unique", Definition: "UNIQUE (code)"})
	} else {
		table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("ic", definition.ID), Method: "btree", Keys: []string{"code"}})
	}
	for _, attribute := range definition.Attributes {
		if err := catalog.appendAttributeSchema(&table, attribute); err != nil {
			return schemadiff.Table{}, nil, fmt.Errorf("catalog %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	parts, err := catalog.tablePartTables("catalog", definition.Name, definition.ID, definition.TableParts)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	return table, parts, nil
}

func (catalog *Catalog) tablePartTables(ownerKind, ownerName string, ownerID uuid.UUID, definitions []TablePart) ([]schemadiff.Table, error) {
	ownerTable, err := schemadiff.TableName(ownerID)
	if err != nil {
		return nil, err
	}
	parts := make([]schemadiff.Table, 0, len(definitions))
	for _, part := range definitions {
		partName, err := PhysicalCatalogTable(part.ID)
		if err != nil {
			return nil, err
		}
		partTable := schemadiff.Table{
			Name: partName,
			Columns: []schemadiff.Column{
				{Name: "owner_ref", Type: "uuid", Nullable: false},
				{Name: "line_no", Type: "integer", Nullable: false},
			},
			Constraints: []schemadiff.Constraint{
				{Name: physicalObjectName("pk", part.ID), Type: "primary_key", Definition: "PRIMARY KEY (owner_ref, line_no)"},
				{Name: physicalObjectName("fk", part.ID), Type: "foreign_key", Definition: "FOREIGN KEY (owner_ref) REFERENCES " + schemadiff.ApplicationSchema + "." + ownerTable + "(ref) ON DELETE CASCADE"},
			},
		}
		for _, attribute := range part.Attributes {
			if err := catalog.appendAttributeSchema(&partTable, attribute); err != nil {
				return nil, fmt.Errorf("%s %s table part %s attribute %s: %w", ownerKind, ownerName, part.Name, attribute.Name, err)
			}
		}
		parts = append(parts, partTable)
	}
	return parts, nil
}

func (catalog *Catalog) appendAttributeSchema(table *schemadiff.Table, attribute Attribute) error {
	columnName, err := PhysicalAttributeColumn(attribute.ID)
	if err != nil {
		return err
	}
	storage, err := catalog.attributeStorage(attribute.Types)
	if err != nil {
		return err
	}
	table.Columns = append(table.Columns, schemadiff.Column{Name: columnName, Type: storage.sqlType, Nullable: !attribute.Required})
	if attribute.Indexed {
		table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("i", attribute.ID), Method: "btree", Keys: []string{columnName}})
	}
	if storage.referenceObject != nil {
		target, err := schemadiff.TableName(*storage.referenceObject)
		if err != nil {
			return err
		}
		table.Constraints = append(table.Constraints, schemadiff.Constraint{
			Name: physicalObjectName("fk", attribute.ID), Type: "foreign_key",
			Definition: "FOREIGN KEY (" + columnName + ") REFERENCES " + schemadiff.ApplicationSchema + "." + target + "(ref)",
		})
	}
	return nil
}

type attributeStorage struct {
	sqlType         string
	valueType       TypeKind
	referenceObject *uuid.UUID
	composite       bool
}

func (catalog *Catalog) attributeStorage(types []Type) (attributeStorage, error) {
	resolved, err := catalog.expandTypes(types, nil)
	if err != nil || len(resolved) == 0 {
		return attributeStorage{}, fmt.Errorf("resolve attribute types: %w", err)
	}
	if len(resolved) != 1 {
		return attributeStorage{sqlType: "jsonb", composite: true}, nil
	}
	item := resolved[0]
	storage := attributeStorage{valueType: item.Kind}
	switch item.Kind {
	case StringType:
		storage.sqlType = "text"
		if item.Length > 0 {
			storage.sqlType = fmt.Sprintf("character varying(%d)", item.Length)
		}
	case NumberType:
		storage.sqlType = fmt.Sprintf("numeric(%d,%d)", item.Precision, item.Scale)
	case BooleanType:
		storage.sqlType = "boolean"
	case DateType:
		storage.sqlType = "timestamp with time zone"
	case UUIDType, EnumerationType, CatalogType, DocumentType:
		storage.sqlType = "uuid"
		if item.Kind == CatalogType || item.Kind == DocumentType {
			id := *item.Reference
			storage.referenceObject = &id
		}
	default:
		return attributeStorage{}, fmt.Errorf("unsupported resolved type %s", item.Kind)
	}
	return storage, nil
}

func codeSQLType(code CatalogCode) string {
	if code.Type == NumberType {
		return fmt.Sprintf("numeric(%d,0)", code.Length)
	}
	return fmt.Sprintf("character varying(%d)", code.Length)
}

func physicalObjectName(prefix string, id uuid.UUID) string {
	return prefix + "_" + strings.ReplaceAll(id.String(), "-", "")
}
