package metadata

import (
	"fmt"
	"reflect"
	"slices"
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
	for _, definition := range catalog.ChartsOfCharacteristicTypes {
		table, parts, err := catalog.chartOfCharacteristicTypesTables(definition)
		if err != nil {
			return schemadiff.Schema{}, err
		}
		schema.Tables = append(schema.Tables, table)
		schema.Tables = append(schema.Tables, parts...)
	}
	for _, definition := range catalog.ChartsOfAccounts {
		table, parts, err := catalog.chartOfAccountsTables(definition)
		if err != nil {
			return schemadiff.Schema{}, err
		}
		schema.Tables = append(schema.Tables, table)
		schema.Tables = append(schema.Tables, parts...)
	}
	for _, definition := range catalog.ChartsOfCalculationTypes {
		table, parts, err := catalog.chartOfCalculationTypesTables(definition)
		if err != nil {
			return schemadiff.Schema{}, err
		}
		schema.Tables = append(schema.Tables, table)
		schema.Tables = append(schema.Tables, parts...)
	}
	for _, definition := range catalog.Tasks {
		table, parts, err := catalog.taskTables(definition)
		if err != nil {
			return schemadiff.Schema{}, err
		}
		schema.Tables = append(schema.Tables, table)
		schema.Tables = append(schema.Tables, parts...)
	}
	for _, definition := range catalog.AccountingRegisters {
		table, err := catalog.accountingRegisterTable(definition)
		if err != nil {
			return schemadiff.Schema{}, err
		}
		schema.Tables = append(schema.Tables, table)
	}
	for _, definition := range catalog.CalculationRegisters {
		tables, err := catalog.calculationRegisterTables(definition)
		if err != nil {
			return schemadiff.Schema{}, err
		}
		schema.Tables = append(schema.Tables, tables...)
	}
	for _, definition := range catalog.Sequences {
		table, err := catalog.sequenceTable(definition)
		if err != nil {
			return schemadiff.Schema{}, err
		}
		schema.Tables = append(schema.Tables, table)
	}
	for _, definition := range catalog.ExchangePlans {
		table, parts, err := catalog.exchangePlanTables(definition)
		if err != nil {
			return schemadiff.Schema{}, err
		}
		schema.Tables = append(schema.Tables, table)
		schema.Tables = append(schema.Tables, parts...)
	}
	for _, definition := range catalog.BusinessProcesses {
		table, parts, err := catalog.businessProcessTables(definition)
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
	for _, definition := range catalog.InformationRegisters {
		table, err := catalog.informationRegisterTable(definition)
		if err != nil {
			return schemadiff.Schema{}, err
		}
		schema.Tables = append(schema.Tables, table)
	}
	for _, definition := range catalog.AccumulationRegisters {
		movements, totals, err := catalog.accumulationRegisterTables(definition)
		if err != nil {
			return schemadiff.Schema{}, err
		}
		schema.Tables = append(schema.Tables, movements, totals)
	}
	if err := catalog.validatePhysicalLimits(schema); err != nil {
		return schemadiff.Schema{}, err
	}
	if err := schema.NormalizeAndValidate(); err != nil {
		return schemadiff.Schema{}, fmt.Errorf("build application schema: %w", err)
	}
	return schema, nil
}

const (
	// maxTableColumns is PostgreSQL's ceiling on the columns of one table.
	maxTableColumns = 1600
	// maxFixedRowBytes bounds what a row holds in values of fixed width -
	// references, moments, flags, counters. A value of variable width, a string
	// or a number, is moved out of the row when the row grows; a fixed one
	// stays, and a row of them past the page is refused at the first insert,
	// long after the table was created without a word. The page is 8 KiB and
	// the row's own header and the page's take some of it.
	maxFixedRowBytes = 8000
)

// validatePhysicalLimits refuses a table the database would refuse. The model
// sets no limit of its own on how many fields an object has - neither does the
// prototype, and real configurations keep registers of fifty dimensions - so
// what is left are PostgreSQL's, and they are checked here, when the schema is
// built, rather than met by "Сохранить данные" as a message of the database.
func (catalog *Catalog) validatePhysicalLimits(schema schemadiff.Schema) error {
	for _, table := range schema.Tables {
		fixed := 23 + (len(table.Columns)+7)/8
		for _, column := range table.Columns {
			fixed += fixedColumnBytes(column.Type)
		}
		switch {
		case len(table.Columns) > maxTableColumns:
			return fmt.Errorf("%s has %d columns, and PostgreSQL takes at most %d in a table", catalog.describeTable(table.Name), len(table.Columns), maxTableColumns)
		case fixed > maxFixedRowBytes:
			return fmt.Errorf("%s holds %d bytes of fixed-width values in a row, and a row of them past %d is refused at the first write", catalog.describeTable(table.Name), fixed, maxFixedRowBytes)
		}
	}
	return nil
}

// fixedColumnBytes is how much a value of the column takes in the row whatever
// it is; zero for a value the database may move out of the row.
func fixedColumnBytes(sqlType string) int {
	switch sqlType {
	case "uuid":
		return 16
	case "bigint", "timestamp with time zone", "timestamp without time zone":
		return 8
	case "integer":
		return 4
	case "smallint":
		return 2
	case "boolean":
		return 1
	}
	return 0
}

// describeTable names the object a physical table belongs to, found by the
// identifier the table's name is built from.
func (catalog *Catalog) describeTable(name string) string {
	hexID := name
	if cut := strings.LastIndex(name, "_"); cut >= 0 {
		hexID = name[cut+1:]
	}
	if len(hexID) == 32 {
		if id, err := uuid.Parse(hexID[:8] + "-" + hexID[8:12] + "-" + hexID[12:16] + "-" + hexID[16:20] + "-" + hexID[20:]); err == nil {
			if owner, ok := catalog.objectNameByID(id); ok {
				return owner + " (table " + name + ")"
			}
		}
	}
	return "table " + name
}

// objectNameByID finds an object, or a table part of one, by its identifier
// among every collection of the catalog. It walks the collections by
// reflection because it runs only on the way to an error, where a list kept
// by hand would be the one place a new kind was forgotten.
func (catalog *Catalog) objectNameByID(id uuid.UUID) (string, bool) {
	value := reflect.ValueOf(catalog).Elem()
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		if !value.Type().Field(index).IsExported() || field.Kind() != reflect.Slice {
			continue
		}
		for position := 0; position < field.Len(); position++ {
			item := reflect.Indirect(field.Index(position))
			if item.Kind() != reflect.Struct {
				continue
			}
			itemID, name := item.FieldByName("ID"), item.FieldByName("Name")
			if !itemID.IsValid() || !name.IsValid() || itemID.Type() != reflect.TypeOf(uuid.UUID{}) {
				continue
			}
			if itemID.Interface().(uuid.UUID) == id {
				return name.String(), true
			}
			if parts := item.FieldByName("TableParts"); parts.IsValid() && parts.Kind() == reflect.Slice {
				for part := 0; part < parts.Len(); part++ {
					if parts.Index(part).FieldByName("ID").Interface().(uuid.UUID) == id {
						return name.String() + "." + parts.Index(part).FieldByName("Name").String(), true
					}
				}
			}
		}
	}
	return "", false
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
			{Name: "deletion_mark", Type: "boolean", Nullable: false, Default: "false"},
			{Name: "predefined_name", Type: "character varying(128)", Nullable: true},
		},
		Constraints: []schemadiff.Constraint{
			{Name: physicalObjectName("pk", definition.ID), Type: "primary_key", Definition: "PRIMARY KEY (ref)"},
			{Name: physicalObjectName("up", definition.ID), Type: "unique", Definition: "UNIQUE (predefined_name)"},
		},
		Indexes: []schemadiff.Index{{Name: physicalObjectName("im", definition.ID), Method: "btree", Keys: []string{"deletion_mark"}}},
	}
	appendCodeColumn(&table, definition.ID, definition.Code)
	appendDescriptionColumn(&table, definition.DescriptionLength)
	appendHierarchyColumns(&table, definition.ID, definition.Hierarchy)
	if err := catalog.ownerColumns(&table, definition); err != nil {
		return schemadiff.Table{}, nil, fmt.Errorf("catalog %s owner: %w", definition.Name, err)
	}
	for _, attribute := range definition.Attributes {
		if err := catalog.appendAttributeSchema(&table, attribute); err != nil {
			return schemadiff.Table{}, nil, fmt.Errorf("catalog %s attribute %s: %w", definition.Name, attribute.Name, err)
		}
	}
	appendListSearchIndexes(&table, definition.ID, definition.List, []string{"Description", "Code"}, definition.Attributes, codeAndDescriptionColumns(definition.Code, definition.DescriptionLength))
	parts, err := catalog.tablePartTables("catalog", definition.Name, definition.ID, definition.TableParts)
	if err != nil {
		return schemadiff.Table{}, nil, err
	}
	return table, parts, nil
}

// appendHierarchyColumns gives a table the columns nesting needs: the parent,
// and - where folders exist - whether the row is one.
//
// The parent points at the same table, so the key is deferrable: a whole tree
// is written in one transaction, and the order inside it is the application's
// business, not ours. Deletion is restricted rather than cascading: losing a
// folder must not silently take its contents with it.
func appendHierarchyColumns(table *schemadiff.Table, id uuid.UUID, hierarchy Hierarchy) {
	if !hierarchy.Enabled {
		return
	}
	table.Columns = append(table.Columns, schemadiff.Column{Name: "parent", Type: "uuid", Nullable: true})
	if hierarchy.Kind == FoldersAndItemsHierarchy {
		table.Columns = append(table.Columns, schemadiff.Column{Name: "is_folder", Type: "boolean", Nullable: false, Default: "false"})
	}
	table.Constraints = append(table.Constraints, schemadiff.Constraint{
		Name: physicalObjectName("fp", id), Type: "foreign_key",
		Definition: "FOREIGN KEY (parent) REFERENCES " + schemadiff.ApplicationSchema + "." + table.Name + "(ref) DEFERRABLE INITIALLY DEFERRED",
	})
	// Reading the children of a parent is the query a hierarchical list makes
	// on every open, so it gets its own index.
	table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("ip", id), Method: "btree", Keys: []string{"parent"}})
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
	// Nullable without exception. The prototype keeps no field that the
	// database refuses to leave empty: an unfilled field there holds the
	// default of its type, and whether that is acceptable is a question asked
	// of the user by the filling check, not of the writer by the table.
	table.Columns = append(table.Columns, schemadiff.Column{Name: columnName, Type: storage.sqlType, Nullable: true})
	if attribute.Indexing.indexes() {
		table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("i", attribute.ID), Method: "btree", Keys: []string{columnName}})
	}
	if storage.referenceObject != nil {
		target, err := schemadiff.TableName(*storage.referenceObject)
		if err != nil {
			return err
		}
		table.Constraints = append(table.Constraints, schemadiff.Constraint{
			Name: physicalObjectName("fk", attribute.ID), Type: "foreign_key",
			Definition: "FOREIGN KEY (" + columnName + ") REFERENCES " + schemadiff.ApplicationSchema + "." + target + "(ref) DEFERRABLE INITIALLY DEFERRED",
		})
	}
	return nil
}

func informationRegisterIndexMethod(storage attributeStorage) string {
	if storage.composite || storage.valueType == StringType {
		return "hash"
	}
	return "btree"
}

type attributeStorage struct {
	sqlType         string
	valueType       TypeKind
	referenceObject *uuid.UUID
	composite       bool
}

func (catalog *Catalog) attributeStorage(types []Type) (attributeStorage, error) {
	// A set is stored the way a composite type is stored, and the question is
	// asked before anything is counted. A set holding a single object today
	// would otherwise get that object's own column and a foreign key to its
	// table, and the day a second object joined the set the table would have
	// to be rebuilt - while the whole promise of a set is that nothing has to
	// be touched for a new object to fall into it.
	open, err := catalog.typesOpenToConfiguration(types, nil)
	if err != nil {
		return attributeStorage{}, fmt.Errorf("resolve attribute types: %w", err)
	}
	if open {
		return attributeStorage{sqlType: "jsonb", composite: true}, nil
	}
	resolved, err := catalog.expandTypes(types, nil)
	if err != nil || len(resolved) == 0 {
		return attributeStorage{}, fmt.Errorf("resolve attribute types: %w", err)
	}
	// A defined type may hold the object of a document or a value table, and
	// the designer lets such a defined type be given to a stored attribute -
	// but there is nothing to store: an object is what a reference points at,
	// not a value of its own, and the configurations being moved never do it.
	// The decision of the owner of 01.10.2026: refuse it when the database is
	// built, by name, rather than give it a column - one jsonb column would
	// take a composite type with an object in it without a word.
	for _, item := range resolved {
		if isObjectType(item.Kind) || isValueType(item.Kind) {
			return attributeStorage{}, fmt.Errorf("type %s lives in memory only and cannot be stored: an object or a manager is what a reference points at, use the reference", item.Kind)
		}
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
			if item.FixedLength {
				storage.sqlType = fmt.Sprintf("character(%d)", item.Length)
			}
		}
	case NumberType:
		storage.sqlType = fmt.Sprintf("numeric(%d,%d)", item.Precision, item.Scale)
	case BooleanType:
		storage.sqlType = "boolean"
	case DateType:
		// Storage does not follow the date-parts qualifier: a moment is a
		// moment, and narrowing the column would lose what the developer can
		// still widen back to later without a migration.
		storage.sqlType = "timestamp with time zone"
	case ValueStorageType:
		storage.sqlType = "bytea"
	case RoutePointType:
		// A point of a route is not a row of any table: it is part of the map
		// the configuration draws, and it is carried by name.
		storage.sqlType = "character varying(128)"
	case UUIDType, EnumerationType, CatalogType, DocumentType, CharacteristicTypesType, AccountType,
		CalculationTypeType, BusinessProcessType, TaskType, ExchangePlanType:
		storage.sqlType = "uuid"
		if item.Kind != UUIDType && item.Kind != EnumerationType {
			id := *item.Reference
			storage.referenceObject = &id
		}
	default:
		return attributeStorage{}, fmt.Errorf("unsupported resolved type %s", item.Kind)
	}
	return storage, nil
}

// appendCodeColumn gives a table its code, where it has one: a code of length 0
// is switched off, and the table has no such column - the storage structure
// of the prototype shows none, checked by the owner on 01.10.2026 - and no
// index or uniqueness by it either (ИТС 1590).
//
// The column has a default, the empty code, for one reason: lengthening a code
// from 0 adds the column to a table that already has rows, and the prototype
// gives those rows an empty code - an empty string, checked the same day. A
// column NOT NULL without a default could not be added there at all.
func appendCodeColumn(table *schemadiff.Table, id uuid.UUID, code CatalogCode) {
	if code.Length == 0 {
		return
	}
	column := schemadiff.Column{Name: "code", Type: codeSQLType(code), Nullable: false, Default: emptyCodeDefault(code)}
	// The code stands where it always stood, after the identity and the
	// version, so a table built anew lays out as before.
	table.Columns = append(table.Columns[:2], append([]schemadiff.Column{column}, table.Columns[2:]...)...)
	if code.Unique {
		table.Constraints = append(table.Constraints, schemadiff.Constraint{Name: physicalObjectName("uq", id), Type: "unique", Definition: "UNIQUE (code)"})
	} else {
		table.Indexes = append(table.Indexes, schemadiff.Index{Name: physicalObjectName("ic", id), Method: "btree", Keys: []string{"code"}})
	}
}

// dropSwitchedOffColumn takes away a column a length of 0 switches off, on a
// table whose columns are written out whole. With any other length the table
// stays exactly as it was, so a base that has one migrates nothing.
func dropSwitchedOffColumn(table *schemadiff.Table, name string, length int) {
	if length != 0 {
		return
	}
	table.Columns = slices.DeleteFunc(table.Columns, func(column schemadiff.Column) bool { return column.Name == name })
}

// appendDescriptionColumn gives a table its description where it has one - a
// length of 0 switches it off as it does a code. It goes after the code, where
// it always stood, and its default is the empty description, which lets it be
// added to a table that has rows.
func appendDescriptionColumn(table *schemadiff.Table, length int) {
	if length == 0 {
		return
	}
	at := 2
	if len(table.Columns) > 2 && table.Columns[2].Name == "code" {
		at = 3
	}
	column := schemadiff.Column{Name: "description", Type: fmt.Sprintf("character varying(%d)", length), Nullable: false, Default: "''::character varying"}
	table.Columns = append(table.Columns[:at], append([]schemadiff.Column{column}, table.Columns[at:]...)...)
}

// emptyCodeDefault is the empty code as PostgreSQL spells it back, so that an
// unchanged schema compares equal: zero for a number, the empty string cast to
// the column's own family for a string.
func emptyCodeDefault(code CatalogCode) string {
	switch {
	case code.Type == NumberType:
		return "0"
	case code.FixedLength:
		return "''::bpchar"
	default:
		return "''::character varying"
	}
}

// codeAndDescriptionColumns are the standard columns a list is searched by,
// without the code where it is switched off.
func codeAndDescriptionColumns(code CatalogCode, descriptionLength int) map[string]listColumn {
	columns := map[string]listColumn{}
	if descriptionLength > 0 {
		columns["description"] = listColumn{name: "description", kind: StringType}
	}
	if code.Length > 0 {
		columns["code"] = listColumn{name: "code", kind: code.Type}
	}
	return columns
}

func codeSQLType(code CatalogCode) string {
	if code.Type == NumberType {
		return fmt.Sprintf("numeric(%d,0)", code.Length)
	}
	// A fixed code is padded to its width, and that is what character(n) is:
	// the database pads on write and compares disregarding the padding, which
	// is the prototype's behaviour exactly.
	if code.FixedLength {
		return fmt.Sprintf("character(%d)", code.Length)
	}
	return fmt.Sprintf("character varying(%d)", code.Length)
}

func physicalObjectName(prefix string, id uuid.UUID) string {
	return prefix + "_" + strings.ReplaceAll(id.String(), "-", "")
}
