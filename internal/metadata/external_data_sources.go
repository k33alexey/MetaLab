package metadata

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// ExternalDataSourceKind holds the descriptions of databases this configuration
// reads and writes without owning them: the tables of somebody else's system,
// described so that the application can work with them the way it works with
// its own catalogs.
const ExternalDataSourceKind Kind = "external-data-sources"

// ExternalDataSourceTableKind names a table of a source where a kind is asked
// for - in a message, in the list of objects something is entered on the basis
// of. It is not a folder of the project: a table lies inside its source.
const ExternalDataSourceTableKind Kind = "external-data-source-tables"

// ExternalTableType is a reference to one record of an object table of an
// external data source. The reference is the table's identifier.
//
// Only a field of a table of the same source may have it for now. The prototype
// lets any attribute of the configuration hold such a reference, and carrying
// that needs a storage and a resolution the rest of the type system does not
// have yet - so validateTypes does not know the kind, and an attribute of a
// catalog that names it is refused as a kind it does not support rather than
// accepted and stored as nothing.
const ExternalTableType TypeKind = "external-data-source-table"

// ExternalDataSourceTablesDirectory is the folder of a source that holds its
// tables, one folder per table. The layout follows the prototype's export: a
// subordinate object that is an object of development in its own right lies
// in a folder named after its collection, beside the description of its owner,
// the way a form or a recalculation does.
const ExternalDataSourceTablesDirectory = "tables"

const (
	maxExternalTablesPerSource = 1024
	maxExternalFieldsPerTable  = 1024
	maxExternalKeyFields       = 16
	// maxNameInDataSource bounds a name of the other database. It is a name,
	// even when it is qualified by a schema and quoted.
	maxNameInDataSource = 512
	// maxExpressionInDataSource bounds a query in the language of the other
	// database. A query is text somebody wrote and reads, not a document.
	maxExpressionInDataSource = 64 << 10
)

// ExternalTableSource and ExternalTableDataType are the two enumerations that
// say what a table is: made of a table of the source or of an expression, and
// holding objects or records.
type ExternalTableSource string

const (
	ExternalTableFromTable      ExternalTableSource = "table"
	ExternalTableFromExpression ExternalTableSource = "expression"
)

type ExternalTableDataType string

const (
	// ExternalObjectData is a table of things - reference data, documents - a
	// row of which has a reference, is opened in a form of an object and may
	// be the type of a field of another table.
	ExternalObjectData ExternalTableDataType = "object"
	// ExternalNonObjectData is a table of records, opened in a form of a
	// record and referred to by nothing.
	ExternalNonObjectData ExternalTableDataType = "non-object"
)

// IsolationLevel is the isolation level of the transactions that change a
// table of a source. The source is somebody else's database and its own
// default is not ours to assume, so the level is said per table.
type IsolationLevel string

const (
	IsolationAuto            IsolationLevel = "auto"
	IsolationReadUncommitted IsolationLevel = "read-uncommitted"
	IsolationReadCommitted   IsolationLevel = "read-committed"
	IsolationRepeatableRead  IsolationLevel = "repeatable-read"
	IsolationSerializable    IsolationLevel = "serializable"
)

// ExternalDataSourceDefinition is one external data source and its tables.
type ExternalDataSourceDefinition struct {
	Format  int           `yaml:"format" json:"format"`
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// DataLock is the locking mode the source's tables take unless they say
	// otherwise.
	DataLock project.DataLockControlMode `yaml:"data_lock,omitempty" json:"dataLock,omitempty"`
	// Tables and Cubes are read from the source's folders of them, not from
	// this file.
	Tables []ExternalTable `yaml:"-" json:"tables,omitempty"`
	Cubes  []ExternalCube  `yaml:"-" json:"cubes,omitempty"`
}

// ExternalTableForms are the default forms of a table. A table has no
// auxiliary forms, unlike a catalog: the help lists four and only four.
type ExternalTableForms struct {
	Object string `yaml:"object,omitempty" json:"object,omitempty"`
	Record string `yaml:"record,omitempty" json:"record,omitempty"`
	List   string `yaml:"list,omitempty" json:"list,omitempty"`
	Choice string `yaml:"choice,omitempty" json:"choice,omitempty"`
}

// slots are the four roles, for the check that each names a form the table's
// folder holds.
func (forms ExternalTableForms) slots() []formSlot {
	return []formSlot{
		{"forms.object", forms.Object},
		{"forms.record", forms.Record},
		{"forms.list", forms.List},
		{"forms.choice", forms.Choice},
	}
}

// ExternalTableHierarchy is how the rows of an object table nest.
type ExternalTableHierarchy struct {
	// ParentField holds the reference to the row's parent.
	ParentField string `yaml:"parent_field" json:"parentField"`
	// UnfilledParentValue is the value the parent field holds when a row has
	// no parent. Absent means Null, which is what the prototype offers first;
	// otherwise it is a value of the key's type - an empty string, a zero.
	UnfilledParentValue *Value `yaml:"unfilled_parent_value,omitempty" json:"unfilledParentValue,omitempty"`
}

// ExternalTable is one table of a source.
type ExternalTable struct {
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"`
	// Presentations and RecordPresentations are how the table and one row of
	// it are named. A table has both sets: an object table opens a row as an
	// object, a table of records opens it as a record, and the help gives the
	// table the properties of either.
	Presentations       `yaml:",inline" json:",inline"`
	RecordPresentations `yaml:",inline" json:",inline"`

	// TableType says whether the table is a table of the source or an
	// expression in the source's query language; exactly one of
	// NameInDataSource and ExpressionInDataSource goes with it.
	TableType              ExternalTableSource   `yaml:"table_type,omitempty" json:"tableType,omitempty"`
	NameInDataSource       string                `yaml:"name_in_data_source,omitempty" json:"nameInDataSource,omitempty"`
	ExpressionInDataSource string                `yaml:"expression_in_data_source,omitempty" json:"expressionInDataSource,omitempty"`
	DataType               ExternalTableDataType `yaml:"data_type" json:"dataType"`
	// KeyFields are the fields the table's key is made of, in the key's
	// order. The order matters: it is the order of the key.
	KeyFields         []string                `yaml:"key_fields,omitempty" json:"keyFields,omitempty"`
	PresentationField string                  `yaml:"presentation_field,omitempty" json:"presentationField,omitempty"`
	Hierarchy         *ExternalTableHierarchy `yaml:"hierarchy,omitempty" json:"hierarchy,omitempty"`
	// DataVersionField holds the version of a row in the other database: it
	// grows with every change, and a write compares it to know nobody else
	// changed the row meanwhile.
	DataVersionField string                      `yaml:"data_version_field,omitempty" json:"dataVersionField,omitempty"`
	DataLock         project.DataLockControlMode `yaml:"data_lock,omitempty" json:"dataLock,omitempty"`
	DataLockFields   []string                    `yaml:"data_lock_fields,omitempty" json:"dataLockFields,omitempty"`
	ReadOnly         bool                        `yaml:"read_only,omitempty" json:"readOnly,omitempty"`
	IsolationLevel   IsolationLevel              `yaml:"isolation_level,omitempty" json:"isolationLevel,omitempty"`

	// The settings of choosing and typing a row: the part of a reference
	// object's that the help gives a table. A table has no choice mode, no
	// default presentation and no full-text search on input.
	EditType             EditType          `yaml:"edit_type,omitempty" json:"editType,omitempty"`
	QuickChoice          bool              `yaml:"quick_choice,omitempty" json:"quickChoice,omitempty"`
	InputByString        []string          `yaml:"input_by_string,omitempty" json:"inputByString,omitempty"`
	SearchStringMode     SearchStringMode  `yaml:"search_string_mode,omitempty" json:"searchStringMode,omitempty"`
	ChoiceDataGetMode    ChoiceDataGetMode `yaml:"choice_data_get_mode,omitempty" json:"choiceDataGetMode,omitempty"`
	CreateOnInput        UsageMode         `yaml:"create_on_input,omitempty" json:"createOnInput,omitempty"`
	ChoiceHistoryOnInput ChoiceHistory     `yaml:"choice_history_on_input,omitempty" json:"choiceHistoryOnInput,omitempty"`

	BasedOn         []uuid.UUID            `yaml:"based_on,omitempty" json:"basedOn,omitempty"`
	Characteristics []ObjectCharacteristic `yaml:"characteristics,omitempty" json:"characteristics,omitempty"`
	Forms           ExternalTableForms     `yaml:"forms,omitempty" json:"forms,omitempty"`
	// Commands and Templates are the table's own, as a catalog's are: each
	// keeps a folder in the table's folder.
	Commands  []ObjectCommand  `yaml:"commands,omitempty" json:"commands,omitempty"`
	Templates []ObjectTemplate `yaml:"templates,omitempty" json:"templates,omitempty"`
	Fields    []ExternalField  `yaml:"fields,omitempty" json:"fields,omitempty"`
}

// ExternalField is one field of a table of a source.
//
// It is a field, and the palette of a field is on it: presentation, choice,
// filling, the check of filling. Eight properties of an attribute are not, and
// the syntax assistant agrees by silence - the field has no indexing, full-text
// search, data history or «использование», no bounds of the value, no choice
// of folders and items and no link by type. validateExternalFieldStorage
// refuses all eight. Three properties are its own: the name of the column in
// the other database, whether the column may hold Null, and whether the field
// is read only.
type ExternalField struct {
	Attribute        `yaml:",inline" json:",inline"`
	NameInDataSource string `yaml:"name_in_data_source" json:"nameInDataSource"`
	AllowNull        bool   `yaml:"allow_null,omitempty" json:"allowNull,omitempty"`
	ReadOnly         bool   `yaml:"read_only,omitempty" json:"readOnly,omitempty"`
}

// DecodeExternalDataSource reads and validates the description of one source,
// without its tables.
func DecodeExternalDataSource(source string, reader io.Reader, configuration project.Project) (ExternalDataSourceDefinition, error) {
	var value ExternalDataSourceDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return ExternalDataSourceDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateDataLockMode("data_lock", value.DataLock)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return ExternalDataSourceDefinition{}, err
	}
	return value, nil
}

// DecodeExternalTable reads and validates one table of a source: everything the
// table can answer about itself, which is almost everything - the fields its
// lists name are its own. What it cannot answer is whether the tables its
// fields refer to are there, which the source answers, and what it is entered
// on the basis of, which the configuration does.
func DecodeExternalTable(source string, reader io.Reader, configuration project.Project) (ExternalTable, error) {
	var value ExternalTable
	if err := decodeStrict(source, reader, &value); err != nil {
		return ExternalTable{}, err
	}
	if err := issuesError(source, value.Format, validateExternalTable(value, configuration)); err != nil {
		return ExternalTable{}, err
	}
	return value, nil
}

func validateExternalTable(table ExternalTable, configuration project.Project) []string {
	issues := validateBase(table.Format, table.ID, table.Name, table.Title, configuration)
	issues = append(issues, validatePresentations(table.Presentations, configuration)...)
	issues = append(issues, validateRecordPresentations(table.RecordPresentations, configuration)...)
	issues = append(issues, validateExternalTableSource(table)...)
	switch table.DataType {
	case ExternalObjectData, ExternalNonObjectData:
	default:
		issues = append(issues, "data_type must be object or non-object")
	}
	switch table.IsolationLevel {
	case "", IsolationAuto, IsolationReadUncommitted, IsolationReadCommitted, IsolationRepeatableRead, IsolationSerializable:
	default:
		issues = append(issues, "isolation_level must be auto, read-uncommitted, read-committed, repeatable-read or serializable")
	}
	issues = append(issues, validateDataLockMode("data_lock", table.DataLock)...)
	if !validEditType(table.EditType) {
		issues = append(issues, "edit_type must be in-dialog, in-list or both-ways")
	}
	if !validSearchStringMode(table.SearchStringMode) {
		issues = append(issues, "search_string_mode must be begin or any-part")
	}
	if !validChoiceDataGetMode(table.ChoiceDataGetMode) {
		issues = append(issues, "choice_data_get_mode must be directly or background")
	}
	if !validUsageMode(table.CreateOnInput) {
		issues = append(issues, "create_on_input must be auto, use or dont-use")
	}
	if !validChoiceHistory(table.ChoiceHistoryOnInput) {
		issues = append(issues, "choice_history_on_input must be auto, use or dont-use")
	}
	issues = append(issues, validateBasedOn(table.BasedOn)...)
	issues = append(issues, validateObjectCharacteristics(table.Characteristics)...)
	issues = append(issues, validateObjectCommands(table.Commands, table.ID, configuration)...)
	issues = append(issues, validateObjectTemplates(table.Templates, configuration)...)
	fields, fieldIssues := validateExternalFields(table, configuration)
	issues = append(issues, fieldIssues...)
	issues = append(issues, validateExternalTableShape(table, fields)...)
	return issues
}

// validateExternalTableSource checks what the table is made of: a table of the
// source by its name, or an expression by its text - one of the two, never
// both, because a table read from two places reads from the one nobody meant.
func validateExternalTableSource(table ExternalTable) []string {
	var issues []string
	switch table.TableType {
	case "", ExternalTableFromTable:
		if table.NameInDataSource == "" {
			issues = append(issues, "name_in_data_source must name the table of the source this table is")
		}
		if table.ExpressionInDataSource != "" {
			issues = append(issues, "expression_in_data_source belongs to a table made of an expression, and this one is made of a table")
		}
	case ExternalTableFromExpression:
		if strings.TrimSpace(table.ExpressionInDataSource) == "" {
			issues = append(issues, "expression_in_data_source must hold the expression this table is made of")
		}
		if table.NameInDataSource != "" {
			issues = append(issues, "name_in_data_source belongs to a table made of a table, and this one is made of an expression")
		}
	default:
		issues = append(issues, "table_type must be table or expression")
	}
	issues = append(issues, validateNameInDataSource("name_in_data_source", table.NameInDataSource)...)
	if len(table.ExpressionInDataSource) > maxExpressionInDataSource {
		issues = append(issues, fmt.Sprintf("expression_in_data_source must not exceed %d bytes", maxExpressionInDataSource))
	}
	return issues
}

// validateNameInDataSource checks a name of the other database in form: one
// line, bounded. What it may contain is the other database's business - a
// schema, quotes, brackets, spaces inside them - and refusing a name we do not
// recognise would refuse a table that exists.
func validateNameInDataSource(path, value string) []string {
	if utf8.RuneCountInString(value) > maxNameInDataSource {
		return []string{fmt.Sprintf("%s must not exceed %d characters", path, maxNameInDataSource)}
	}
	if strings.ContainsAny(value, "\r\n\x00") {
		return []string{path + " must be one line"}
	}
	return nil
}

// validateExternalFields checks every field on its own and returns them by
// folded name for the checks of the lists that name them.
func validateExternalFields(table ExternalTable, configuration project.Project) (map[string]ExternalField, []string) {
	if len(table.Fields) > maxExternalFieldsPerTable {
		return map[string]ExternalField{}, []string{fmt.Sprintf("fields must not contain more than %d items", maxExternalFieldsPerTable)}
	}
	return checkExternalFieldGroups(configuration, externalFieldGroup{"fields", table.Fields})
}

// externalFieldGroup is one list of fields of a source's object under the
// path its messages name it by: the fields of a table, the dimensions of a
// cube, its resources.
type externalFieldGroup struct {
	path   string
	fields []ExternalField
}

// validateExternalFieldGroups checks the lists of fields of one object as one
// namespace, for an object that needs nothing back from the check.
func validateExternalFieldGroups(configuration project.Project, groups ...externalFieldGroup) []string {
	_, issues := checkExternalFieldGroups(configuration, groups...)
	return issues
}

// checkExternalFieldGroups checks every field of one object on its own, and
// the groups together as one namespace: a dimension and a resource of a cube
// are both fields of the cube to a query, and two of one name would be one
// field read twice. It returns the fields by folded name for the checks of the
// lists that name them.
func checkExternalFieldGroups(configuration project.Project, groups ...externalFieldGroup) (map[string]ExternalField, []string) {
	fields := map[string]ExternalField{}
	var issues []string
	ids := map[uuid.UUID]bool{}
	var linked []fieldGroup
	for _, group := range groups {
		attributes := make([]Attribute, 0, len(group.fields))
		for index, field := range group.fields {
			prefix := fmt.Sprintf("%s[%d]", group.path, index)
			if field.ID.IsZero() {
				issues = append(issues, prefix+".id must be a non-zero UUID")
			} else if ids[field.ID] {
				issues = append(issues, prefix+".id is used twice")
			}
			ids[field.ID] = true
			if !validIdentifier(field.Name) || utf8.RuneCountInString(field.Name) > 128 {
				issues = append(issues, prefix+".name must start with a letter, contain only letters or digits and not exceed 128 characters")
			} else if _, taken := fields[strings.ToLower(field.Name)]; taken {
				issues = append(issues, prefix+".name is used twice")
			} else {
				fields[strings.ToLower(field.Name)] = field
			}
			issues = append(issues, validateTitle(prefix+".title", field.Title, configuration)...)
			if field.NameInDataSource == "" {
				issues = append(issues, prefix+".name_in_data_source must name the column this field is")
			}
			issues = append(issues, validateNameInDataSource(prefix+".name_in_data_source", field.NameInDataSource)...)
			issues = append(issues, validateExternalFieldTypes(prefix+".types", field.Types)...)
			issues = append(issues, validateFieldSettings(prefix, field.Attribute, configuration)...)
			issues = append(issues, validateFieldStorage(prefix, field.Attribute)...)
			issues = append(issues, validateExternalFieldStorage(prefix, field)...)
			attributes = append(attributes, field.Attribute)
		}
		linked = append(linked, fieldGroup{group.path, attributes})
	}
	issues = append(issues, validateFieldLinks(linked, nil)...)
	return fields, issues
}

// isExternalReference says whether a type is a reference to something of a
// source: a row of an object table, or a member of a dimension.
func isExternalReference(kind TypeKind) bool {
	return kind == ExternalTableType || kind == ExternalDimensionTableType
}

// validateExternalFieldTypes checks the types of a field: the reference to a
// table of the source here, everything else where every type is checked.
func validateExternalFieldTypes(path string, types []Type) []string {
	var issues []string
	var rest []Type
	var restIndexes []int
	for index, item := range types {
		if !isExternalReference(item.Kind) {
			rest, restIndexes = append(rest, item), append(restIndexes, index)
			continue
		}
		prefix := fmt.Sprintf("%s[%d]", path, index)
		if item.Reference == nil || item.Reference.IsZero() {
			issues = append(issues, prefix+".reference is required")
		}
		if item.Length != 0 || item.Precision != 0 || item.Scale != 0 || item.FixedLength || item.NonNegative || item.DateParts != "" {
			issues = append(issues, prefix+" has unsupported qualifiers")
		}
	}
	if len(types) > 32 || len(types) == 0 {
		return append(issues, path+" must contain 1..32 types")
	}
	if len(rest) == 0 {
		return issues
	}
	// The rest is checked by the one function every type goes through, and
	// its messages are put back under the index the type has in the field.
	for _, issue := range validateTypes(path, rest, uuid.UUID{}) {
		for position := len(rest) - 1; position >= 0; position-- {
			issue = strings.ReplaceAll(issue, fmt.Sprintf("%s[%d]", path, position), fmt.Sprintf("%s[%d]", path, restIndexes[position]))
		}
		issues = append(issues, issue)
	}
	return issues
}

// validateExternalFieldStorage refuses the eight properties of an attribute a
// field of a source does not have. Why refuse and not ignore: a setting nobody
// reads looks like a setting. An index on a column of somebody else's database
// is a promise we cannot keep - the index is theirs to build - and full-text
// search, data history and «использование» are all things the platform does
// with its own tables and not with somebody else's.
func validateExternalFieldStorage(prefix string, field ExternalField) []string {
	var issues []string
	refused := []struct {
		set  bool
		name string
	}{
		{field.Indexing != "", "indexing"},
		{field.FullTextSearch != "", "full_text_search"},
		{field.DataHistory != "", "data_history"},
		{field.Use != "", "use"},
		{field.Presentation.MinValue != nil, "presentation.min_value"},
		{field.Presentation.MaxValue != nil, "presentation.max_value"},
		{field.Choice.FoldersAndItems != "", "choice.folders_and_items"},
		{field.Choice.LinkByType != nil, "choice.link_by_type"},
	}
	for _, property := range refused {
		if property.set {
			issues = append(issues, prefix+"."+property.name+" belongs to an attribute of the configuration's own objects, and a field of an external data source has none")
		}
	}
	return issues
}

// validateExternalTableShape checks what holds the table together: the lists
// that name its fields, and what an object table has that a table of records
// has not.
//
// The configurator states the rule of the key outright: one key field makes the
// table object data, more than one makes it records and takes the additional
// characteristics away. So an object table has exactly one key field. A table
// of records may have none - the configurator creates a table with no fields at
// all - and that is not refused.
func validateExternalTableShape(table ExternalTable, fields map[string]ExternalField) []string {
	var issues []string
	named := func(path, name string) (ExternalField, bool) {
		field, ok := fields[strings.ToLower(name)]
		if !ok {
			issues = append(issues, fmt.Sprintf("%s names %q, which is not a field of this table", path, name))
		}
		return field, ok
	}
	list := func(path string, names []string, bound int) {
		if len(names) > bound {
			issues = append(issues, fmt.Sprintf("%s must not contain more than %d items", path, bound))
			return
		}
		seen := map[string]bool{}
		for index, name := range names {
			if _, ok := named(fmt.Sprintf("%s[%d]", path, index), name); !ok {
				continue
			}
			if seen[strings.ToLower(name)] {
				issues = append(issues, fmt.Sprintf("%s[%d] names %q twice", path, index, name))
			}
			seen[strings.ToLower(name)] = true
		}
	}
	list("key_fields", table.KeyFields, maxExternalKeyFields)
	list("data_lock_fields", table.DataLockFields, maxExternalFieldsPerTable)
	list("input_by_string", table.InputByString, maxInputByStringFields)
	for index, name := range table.InputByString {
		// The rule of input by string as for any object - one type, a string
		// or a number - without the index: the column is indexed or not in
		// the other database, and nothing here can say which.
		if field, ok := fields[strings.ToLower(name)]; ok &&
			(len(field.Types) != 1 || (field.Types[0].Kind != StringType && field.Types[0].Kind != NumberType)) {
			issues = append(issues, fmt.Sprintf("input_by_string[%d] names %q, and a field is searched by only if it is of one type, string or number", index, name))
		}
	}
	if table.PresentationField != "" {
		named("presentation_field", table.PresentationField)
	}
	if table.DataVersionField != "" {
		named("data_version_field", table.DataVersionField)
	}
	if table.Hierarchy != nil {
		if table.Hierarchy.ParentField == "" {
			issues = append(issues, "hierarchy.parent_field must name the field that holds the parent")
		} else {
			named("hierarchy.parent_field", table.Hierarchy.ParentField)
		}
	}

	if table.DataType != ExternalObjectData {
		// A table of records has no row to refer to, so nothing that is about
		// a row as a thing: no presentation of it, no nesting, no form of an
		// object, no choosing one, no additional characteristics.
		for _, refused := range []struct {
			set  bool
			what string
		}{
			{table.PresentationField != "", "presentation_field"},
			{table.Hierarchy != nil, "hierarchy"},
			{table.Forms.Object != "", "forms.object"},
			{table.Forms.Choice != "", "forms.choice"},
			{len(table.Characteristics) > 0, "characteristics"},
		} {
			if refused.set {
				issues = append(issues, refused.what+" belongs to a table of object data, and this one holds records")
			}
		}
		return issues
	}
	if len(table.KeyFields) != 1 {
		issues = append(issues, "key_fields must name exactly one field: a table of object data is keyed by one field, and more than one make it a table of records")
	}
	if table.Forms.Record != "" {
		issues = append(issues, "forms.record belongs to a table of records, and this one holds object data")
	}
	return issues
}

// ObjectTable reports whether the table holds object data: whether a row of it
// has a reference.
func (table ExternalTable) ObjectTable() bool { return table.DataType == ExternalObjectData }

// Field returns one field of the table by name, folded case.
func (table ExternalTable) Field(name string) (ExternalField, bool) {
	for _, field := range table.Fields {
		if strings.EqualFold(field.Name, name) {
			return field, true
		}
	}
	return ExternalField{}, false
}

func cloneExternalField(field ExternalField) ExternalField {
	field.Attribute = cloneAttribute(field.Attribute)
	return field
}

func cloneExternalTable(table ExternalTable) ExternalTable {
	table.Title = cloneTitle(table.Title)
	table.Presentations = clonePresentations(table.Presentations)
	table.RecordPresentation = cloneTitle(table.RecordPresentation)
	table.ExtendedRecordPresentation = cloneTitle(table.ExtendedRecordPresentation)
	table.KeyFields = slices.Clone(table.KeyFields)
	table.DataLockFields = slices.Clone(table.DataLockFields)
	table.InputByString = slices.Clone(table.InputByString)
	table.BasedOn = slices.Clone(table.BasedOn)
	table.Characteristics = cloneObjectCharacteristics(table.Characteristics)
	table.Commands = cloneObjectCommands(table.Commands)
	table.Templates = cloneObjectTemplates(table.Templates)
	if table.Hierarchy != nil {
		hierarchy := *table.Hierarchy
		hierarchy.UnfilledParentValue = cloneValuePointer(hierarchy.UnfilledParentValue)
		table.Hierarchy = &hierarchy
	}
	fields := make([]ExternalField, len(table.Fields))
	for index, field := range table.Fields {
		fields[index] = cloneExternalField(field)
	}
	table.Fields = fields
	return table
}

func cloneExternalDataSource(source ExternalDataSourceDefinition) ExternalDataSourceDefinition {
	source.Title = cloneTitle(source.Title)
	tables := make([]ExternalTable, len(source.Tables))
	for index, table := range source.Tables {
		tables[index] = cloneExternalTable(table)
	}
	source.Tables = tables
	cubes := make([]ExternalCube, len(source.Cubes))
	for index, cube := range source.Cubes {
		cubes[index] = cloneExternalCube(cube)
	}
	source.Cubes = cubes
	return source
}

// externalTableLocation is where a table lies: which source, which table of it.
type externalTableLocation struct{ source, table int }

// externalDimensionTableLocation is where a dimension table lies.
type externalDimensionTableLocation struct{ source, cube, table int }

// ExternalDataSource returns one source by name, folded case, with its tables.
func (catalog *Catalog) ExternalDataSource(name string) (ExternalDataSourceDefinition, bool) {
	index, ok := catalog.externalDataSourceByName[strings.ToLower(name)]
	if !ok {
		return ExternalDataSourceDefinition{}, false
	}
	return cloneExternalDataSource(catalog.ExternalDataSources[index]), true
}

// ExternalDataSourceByID returns one source by identifier.
func (catalog *Catalog) ExternalDataSourceByID(id uuid.UUID) (ExternalDataSourceDefinition, bool) {
	index, ok := catalog.externalDataSourceByID[id]
	if !ok {
		return ExternalDataSourceDefinition{}, false
	}
	return cloneExternalDataSource(catalog.ExternalDataSources[index]), true
}

// ExternalTableByID returns one table by identifier - which is how a field
// that refers to it names it - together with the name of its source.
func (catalog *Catalog) ExternalTableByID(id uuid.UUID) (ExternalTable, string, bool) {
	location, ok := catalog.externalTableByID[id]
	if !ok {
		return ExternalTable{}, "", false
	}
	source := catalog.ExternalDataSources[location.source]
	return cloneExternalTable(source.Tables[location.table]), source.Name, true
}

// loadExternalDataSources reads every source and its tables. A source is a
// folder with its description and a folder of tables; a table is a folder with
// its description.
//
// The folder of a table is an object folder: its modules, forms, commands and
// templates lie there the way a catalog's lie in its, and are checked the same
// way, by validateExternalTableFiles once the whole configuration is read.
func (catalog *Catalog) loadExternalDataSources(root string, configuration project.Project) error {
	return loadObjectKind(root, ExternalDataSourceKind, func(source string, file *os.File, name string) error {
		value, err := DecodeExternalDataSource(source, file, configuration)
		if err != nil {
			return err
		}
		if !strings.EqualFold(value.Name, name) {
			return fmt.Errorf("external data source %s lies in a folder called %s", value.Name, name)
		}
		directory := filepath.Join(root, "metadata", string(ExternalDataSourceKind), name)
		tables, err := loadExternalTables(directory, filepath.ToSlash(filepath.Dir(source)), configuration)
		if err != nil {
			return err
		}
		value.Tables = tables
		cubes, err := loadExternalCubes(directory, filepath.ToSlash(filepath.Dir(source)), configuration)
		if err != nil {
			return err
		}
		value.Cubes = cubes
		catalog.ExternalDataSources = append(catalog.ExternalDataSources, value)
		return nil
	})
}

func loadExternalTables(directory, relative string, configuration project.Project) ([]ExternalTable, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Name() == project.ObjectMetadataFile && !entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 {
			continue
		}
		if slices.Contains(project.SubordinateCollections(string(ExternalDataSourceKind)), entry.Name()) && entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 {
			continue
		}
		return nil, fmt.Errorf("%s keeps %q, and a source keeps its description and the folders of its tables and cubes", relative, entry.Name())
	}
	tablesDirectory := filepath.Join(directory, ExternalDataSourceTablesDirectory)
	tableEntries, err := os.ReadDir(tablesDirectory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(tableEntries) > maxExternalTablesPerSource {
		return nil, fmt.Errorf("%s holds more than %d tables", relative, maxExternalTablesPerSource)
	}
	tables := make([]ExternalTable, 0, len(tableEntries))
	for _, entry := range tableEntries {
		where := relative + "/" + ExternalDataSourceTablesDirectory + "/" + entry.Name()
		if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("unexpected metadata source %q: a table is a folder", where)
		}
		if err := project.ObjectName(entry.Name()); err != nil {
			return nil, fmt.Errorf("table folder %q: %w", where, err)
		}
		path := filepath.Join(tablesDirectory, entry.Name(), project.ObjectMetadataFile)
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open %s/%s: %w", where, project.ObjectMetadataFile, err)
		}
		table, decodeErr := DecodeExternalTable(where+"/"+project.ObjectMetadataFile, file, configuration)
		file.Close()
		if decodeErr != nil {
			return nil, decodeErr
		}
		if !strings.EqualFold(table.Name, entry.Name()) {
			return nil, fmt.Errorf("table %s lies in a folder called %s", table.Name, entry.Name())
		}
		tables = append(tables, table)
	}
	return tables, nil
}

// validateExternalTableFiles checks the folder of every table the way the
// folder of a catalog is checked: the modules it keeps play roles the table
// has - an object module for object data, a record set module for records, a
// manager module for either - and the forms, commands and templates it
// declares are there, and nothing else is.
func (catalog *Catalog) validateExternalTableFiles(root string) error {
	for _, source := range catalog.ExternalDataSources {
		for _, table := range source.Tables {
			modules := recordSetKindModules
			if table.ObjectTable() {
				modules = objectKindModules
			}
			if err := catalog.validateObjectFileSources(objectFiles{root: root, directoryKind: ExternalDataSourceTableKind,
				kind: "external data source table", name: source.Name + "." + table.Name, modules: modules,
				formSlots: table.Forms.slots(), commands: table.Commands, templates: table.Templates}); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateExternalDataSources checks what a table cannot check about itself:
// that the tables its fields refer to are object tables of the same source,
// and that the values the table names are values of the fields they stand for.
//
// A reference to a table of another source is refused and not merely unusual:
// the reference is a key of one database, and there is no query that joins two
// databases nobody else can see into each other.
func (catalog *Catalog) validateExternalDataSources() error {
	for _, source := range catalog.ExternalDataSources {
		for _, table := range source.Tables {
			owner := fmt.Sprintf("external data source %s table %s", source.Name, table.Name)
			if err := catalog.validateExternalFieldReferences(source, owner, table.Fields); err != nil {
				return err
			}
			if table.Hierarchy != nil && table.Hierarchy.UnfilledParentValue != nil && len(table.KeyFields) == 1 {
				key, _ := table.Field(table.KeyFields[0])
				if _, err := catalog.normalizeTypes(owner+" hierarchy.unfilled_parent_value", key.Types, *table.Hierarchy.UnfilledParentValue); err != nil {
					return fmt.Errorf("%w: the value a row without a parent holds is a value of the key", err)
				}
			}
		}
		for _, cube := range source.Cubes {
			owner := fmt.Sprintf("external data source %s cube %s", source.Name, cube.Name)
			if err := catalog.validateExternalFieldReferences(source, owner, append(slices.Clone(cube.Dimensions), cube.Resources...)); err != nil {
				return err
			}
			for _, table := range cube.DimensionTables {
				if err := catalog.validateExternalFieldReferences(source, owner+" dimension table "+table.Name, table.Fields); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// validateExternalFieldReferences resolves what the fields of one object of a
// source refer to: an object table or a dimension table of the same source,
// and nothing of another. The rest of their types, and the values they are
// filled with, are checked the way any field's are.
func (catalog *Catalog) validateExternalFieldReferences(source ExternalDataSourceDefinition, owner string, fields []ExternalField) error {
	tables := make(map[uuid.UUID]ExternalTable, len(source.Tables))
	for _, table := range source.Tables {
		tables[table.ID] = table
	}
	dimensionTables := map[uuid.UUID]bool{}
	for _, cube := range source.Cubes {
		for _, table := range cube.DimensionTables {
			dimensionTables[table.ID] = true
		}
	}
	for _, field := range fields {
		var primitive []Type
		for _, item := range field.Types {
			switch item.Kind {
			case ExternalTableType:
				target, ok := tables[*item.Reference]
				if !ok {
					if _, elsewhere := catalog.externalTableByID[*item.Reference]; elsewhere {
						return fmt.Errorf("%s field %s refers to a table of another source: a reference is a key of one database", owner, field.Name)
					}
					return fmt.Errorf("%s field %s refers to unknown table %s", owner, field.Name, item.Reference)
				}
				if !target.ObjectTable() {
					return fmt.Errorf("%s field %s refers to table %s, which holds records and has no reference", owner, field.Name, target.Name)
				}
			case ExternalDimensionTableType:
				if !dimensionTables[*item.Reference] {
					if _, elsewhere := catalog.externalDimensionTableByID[*item.Reference]; elsewhere {
						return fmt.Errorf("%s field %s refers to a dimension table of another source: a reference is a key of one database", owner, field.Name)
					}
					return fmt.Errorf("%s field %s refers to unknown dimension table %s", owner, field.Name, item.Reference)
				}
			default:
				primitive = append(primitive, item)
			}
		}
		if err := catalog.validateReferences(owner+" field "+field.Name, primitive); err != nil {
			return err
		}
		if field.Filling.Value != nil {
			if len(primitive) == 0 {
				return fmt.Errorf("%s field %s is filled with a value, and a reference to something of a source is not a value a description can hold", owner, field.Name)
			}
			if _, err := catalog.normalizeTypes(owner+" field "+field.Name+" filling", primitive, *field.Filling.Value); err != nil {
				return err
			}
		}
	}
	return nil
}
