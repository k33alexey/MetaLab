package metadata

import (
	"fmt"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// maxCharacteristicsPerObject is where a list of characteristic descriptions
// stops being a list. The demonstration configuration's richest object has
// three of them.
const maxCharacteristicsPerObject = 64

// A characteristic is a property an object gains without the configuration
// being changed: the user invents the property, the user fills it in, and both
// live in tables that already exist. The object itself declares none of it -
// it only says where to look, and that is what a characteristic description
// is.
//
// It comes in two halves, and neither means anything alone. The table of kinds
// says which properties this object may have; the table of values says where
// what was filled in is kept. The object is the one that owns the description,
// because the same pair of tables serves many objects at once and each of them
// reads its own slice of it.

// StandardField names a field the platform gives a table itself. Such a field
// is declared nowhere - it is there because of the kind of table, not because
// somebody added it - so it cannot be pointed at by identifier the way an
// attribute is.
type StandardField string

const (
	// RefStandardField is the reference to the row: to the object's own row in
	// its table, and to the owning row from a line of a table part.
	RefStandardField StandardField = "ref"
	// ParentStandardField is the row this one sits under. Only a hierarchical
	// object has it.
	ParentStandardField StandardField = "parent"
	// CodeStandardField and DescriptionStandardField are the two fields the
	// dialog of the prototype calls "the name or the identifier of the
	// characteristic": a chart of characteristic types is keyed by one of
	// them as readily as by its reference.
	CodeStandardField        StandardField = "code"
	DescriptionStandardField StandardField = "description"
)

// characteristicKindTypes are the kinds of object a characteristic
// description can be written on, each with the type of a reference to it.
// They are the nine reference kinds and nothing else: a characteristic hangs
// off a reference, and a register row has none to hang off.
var characteristicKindTypes = map[Kind]TypeKind{
	CatalogKind:                    CatalogType,
	DocumentKind:                   DocumentType,
	EnumerationKind:                EnumerationType,
	ChartOfCharacteristicTypesKind: CharacteristicTypesType,
	ChartOfAccountsKind:            AccountType,
	ChartOfCalculationTypesKind:    CalculationTypeType,
	BusinessProcessKind:            BusinessProcessType,
	TaskKind:                       TaskType,
	ExchangePlanKind:               ExchangePlanType,
}

// CharacteristicTable addresses a table a characteristic reads: an object, or
// one table part of one. An information register is addressed the same way,
// and it is the usual place values are kept in - so the kinds allowed here are
// the nine that carry a description plus that one.
type CharacteristicTable struct {
	Kind   Kind      `yaml:"kind" json:"kind"`
	Object uuid.UUID `yaml:"object" json:"object"`
	// TablePart, when named, is the table part of that object; without it the
	// table is the object's own.
	TablePart *uuid.UUID `yaml:"table_part,omitempty" json:"tablePart,omitempty"`
}

func (table CharacteristicTable) key() string {
	key := string(table.Kind) + ":" + table.Object.String()
	if table.TablePart != nil {
		key += ":" + table.TablePart.String()
	}
	return key
}

// CharacteristicField addresses one field of such a table: an attribute the
// description declares, or a field the platform gives the table itself. One or
// the other, never both and never neither.
type CharacteristicField struct {
	Attribute *uuid.UUID    `yaml:"attribute,omitempty" json:"attribute,omitempty"`
	Standard  StandardField `yaml:"standard,omitempty" json:"standard,omitempty"`
}

func (field CharacteristicField) empty() bool {
	return field.Attribute == nil && field.Standard == ""
}

// CharacteristicTypes is the half that says which characteristics an object
// may have: the table they are listed in, and how one row of it is read.
type CharacteristicTypes struct {
	Table CharacteristicTable `yaml:"table" json:"table"`
	// Key holds the characteristic itself - what the table of values will
	// name as the kind of a filled-in value.
	Key CharacteristicField `yaml:"key" json:"key"`
	// Filter and FilterValue cut the table down to the rows that belong to
	// this object: one table of kinds serves the whole configuration, and
	// without the cut every object would offer every property in it. They come
	// together - a field with nothing to compare against filters nothing, and
	// a value with no field has nowhere to be compared.
	Filter      *CharacteristicField `yaml:"filter,omitempty" json:"filter,omitempty"`
	FilterValue *Value               `yaml:"filter_value,omitempty" json:"filterValue,omitempty"`
	// DataPath is the field a characteristic's path in the data is built from.
	// Left unnamed, the presentation of the row is used instead.
	DataPath *CharacteristicField `yaml:"data_path,omitempty" json:"dataPath,omitempty"`
	// MultipleValuesUse marks a characteristic that holds several values at
	// once. Left unnamed, every characteristic holds one.
	MultipleValuesUse *CharacteristicField `yaml:"multiple_values_use,omitempty" json:"multipleValuesUse,omitempty"`
}

// CharacteristicValues is the half that says where what the user filled in is
// kept: the table of values, and the three fields that make one row of it -
// whose value it is, which characteristic it is, and the value itself.
type CharacteristicValues struct {
	Table  CharacteristicTable `yaml:"table" json:"table"`
	Object CharacteristicField `yaml:"object" json:"object"`
	Type   CharacteristicField `yaml:"type" json:"type"`
	Value  CharacteristicField `yaml:"value" json:"value"`
	// MultipleValuesKey tells apart the several values of one multiple
	// characteristic, and MultipleValuesOrder is what they are ordered by.
	// Left unnamed, the value itself does both.
	MultipleValuesKey   *CharacteristicField `yaml:"multiple_values_key,omitempty" json:"multipleValuesKey,omitempty"`
	MultipleValuesOrder *CharacteristicField `yaml:"multiple_values_order,omitempty" json:"multipleValuesOrder,omitempty"`
}

// ObjectCharacteristic is one description: the two halves together.
type ObjectCharacteristic struct {
	Types  CharacteristicTypes  `yaml:"types" json:"types"`
	Values CharacteristicValues `yaml:"values" json:"values"`
}

// validateObjectCharacteristics checks the descriptions of one object as far
// as the object alone allows: the shape of every address, and the fields that
// only make sense together. Whether the tables and fields are there at all is
// answered by the whole configuration, in validateCharacteristics.
func validateObjectCharacteristics(characteristics []ObjectCharacteristic) []string {
	if len(characteristics) > maxCharacteristicsPerObject {
		return []string{fmt.Sprintf("characteristics must not contain more than %d items", maxCharacteristicsPerObject)}
	}
	var issues []string
	seen := map[string]bool{}
	for index, characteristic := range characteristics {
		prefix := fmt.Sprintf("characteristics[%d]", index)
		types, values := characteristic.Types, characteristic.Values
		issues = append(issues, validateCharacteristicTable(prefix+".types.table", types.Table)...)
		issues = append(issues, validateCharacteristicField(prefix+".types.key", types.Key)...)
		issues = append(issues, validateCharacteristicTable(prefix+".values.table", values.Table)...)
		for _, named := range []struct {
			path  string
			field CharacteristicField
		}{
			{prefix + ".values.object", values.Object},
			{prefix + ".values.type", values.Type},
			{prefix + ".values.value", values.Value},
		} {
			issues = append(issues, validateCharacteristicField(named.path, named.field)...)
		}
		for _, optional := range []struct {
			path  string
			field *CharacteristicField
		}{
			{prefix + ".types.filter", types.Filter},
			{prefix + ".types.data_path", types.DataPath},
			{prefix + ".types.multiple_values_use", types.MultipleValuesUse},
			{prefix + ".values.multiple_values_key", values.MultipleValuesKey},
			{prefix + ".values.multiple_values_order", values.MultipleValuesOrder},
		} {
			if optional.field == nil {
				continue
			}
			issues = append(issues, validateCharacteristicField(optional.path, *optional.field)...)
		}
		switch {
		case types.Filter != nil && types.FilterValue == nil:
			issues = append(issues, prefix+".types.filter_value must be set beside the filter field")
		case types.Filter == nil && types.FilterValue != nil:
			issues = append(issues, prefix+".types.filter must be set beside the filter value")
		}
		key := types.Table.key() + "|" + values.Table.key()
		if types.FilterValue != nil {
			key += "|" + string(types.FilterValue.Kind) + ":" + types.FilterValue.Data
		}
		if seen[key] {
			issues = append(issues, prefix+" repeats a characteristic the object already has")
		}
		seen[key] = true
	}
	return issues
}

func validateCharacteristicTable(path string, table CharacteristicTable) []string {
	var issues []string
	if _, ok := characteristicKindTypes[table.Kind]; !ok && table.Kind != InformationRegisterKind {
		issues = append(issues, path+".kind is not a kind of object a characteristic is read from")
	}
	if table.Object.IsZero() {
		issues = append(issues, path+".object must be a non-zero UUID")
	}
	if table.TablePart != nil && table.TablePart.IsZero() {
		issues = append(issues, path+".table_part must be a non-zero UUID")
	}
	return issues
}

// validateCharacteristicField checks one address. Every address written down
// has to name something: the fields that may be left out are left out whole,
// and an address naming nothing is a field the description meant to have.
func validateCharacteristicField(path string, field CharacteristicField) []string {
	if field.empty() {
		return []string{path + " must name an attribute or a standard field"}
	}
	var issues []string
	if field.Attribute != nil && field.Standard != "" {
		issues = append(issues, path+" must name an attribute or a standard field, not both")
	}
	if field.Attribute != nil && field.Attribute.IsZero() {
		issues = append(issues, path+".attribute must be a non-zero UUID")
	}
	if field.Standard != "" && !validStandardField(field.Standard) {
		issues = append(issues, path+".standard is not a standard field")
	}
	return issues
}

func validStandardField(field StandardField) bool {
	switch field {
	case RefStandardField, ParentStandardField, CodeStandardField, DescriptionStandardField:
		return true
	default:
		return false
	}
}

func cloneObjectCharacteristics(characteristics []ObjectCharacteristic) []ObjectCharacteristic {
	if characteristics == nil {
		return nil
	}
	result := make([]ObjectCharacteristic, len(characteristics))
	for index, characteristic := range characteristics {
		characteristic.Types.Table = cloneCharacteristicTable(characteristic.Types.Table)
		characteristic.Types.Key = cloneCharacteristicField(characteristic.Types.Key)
		characteristic.Types.Filter = cloneOptionalCharacteristicField(characteristic.Types.Filter)
		characteristic.Types.DataPath = cloneOptionalCharacteristicField(characteristic.Types.DataPath)
		characteristic.Types.MultipleValuesUse = cloneOptionalCharacteristicField(characteristic.Types.MultipleValuesUse)
		if characteristic.Types.FilterValue != nil {
			filterValue := *characteristic.Types.FilterValue
			characteristic.Types.FilterValue = &filterValue
		}
		characteristic.Values.Table = cloneCharacteristicTable(characteristic.Values.Table)
		characteristic.Values.Object = cloneCharacteristicField(characteristic.Values.Object)
		characteristic.Values.Type = cloneCharacteristicField(characteristic.Values.Type)
		characteristic.Values.Value = cloneCharacteristicField(characteristic.Values.Value)
		characteristic.Values.MultipleValuesKey = cloneOptionalCharacteristicField(characteristic.Values.MultipleValuesKey)
		characteristic.Values.MultipleValuesOrder = cloneOptionalCharacteristicField(characteristic.Values.MultipleValuesOrder)
		result[index] = characteristic
	}
	return result
}

func cloneCharacteristicTable(table CharacteristicTable) CharacteristicTable {
	if table.TablePart != nil {
		tablePart := *table.TablePart
		table.TablePart = &tablePart
	}
	return table
}

func cloneCharacteristicField(field CharacteristicField) CharacteristicField {
	if field.Attribute != nil {
		attribute := *field.Attribute
		field.Attribute = &attribute
	}
	return field
}

func cloneOptionalCharacteristicField(field *CharacteristicField) *CharacteristicField {
	if field == nil {
		return nil
	}
	cloned := cloneCharacteristicField(*field)
	return &cloned
}

// characteristicTable is one addressed table resolved against the
// configuration: the fields the platform gives it, and the fields its
// description declares. Both keep their types, because pointing at a field is
// not enough - a table of values whose field cannot hold what the table of
// kinds keys by is a pair of tables that will never join.
type characteristicTable struct {
	standard   map[StandardField][]Type
	attributes map[uuid.UUID][]Type
}

// field answers what one address holds. The second result is false when the
// table has no such field.
func (table characteristicTable) field(field CharacteristicField) ([]Type, bool) {
	if field.Standard != "" {
		types, ok := table.standard[field.Standard]
		return types, ok
	}
	if field.Attribute == nil {
		return nil, false
	}
	types, ok := table.attributes[*field.Attribute]
	return types, ok
}

// resolveCharacteristicTable finds an addressed table and says what it holds.
func (catalog *Catalog) resolveCharacteristicTable(where string, table CharacteristicTable) (characteristicTable, error) {
	elements, ok := catalog.objectElementsOf(table.Kind, table.Object)
	if !ok {
		return characteristicTable{}, fmt.Errorf("%s reads %s %s, which is not in the configuration", where, table.Kind, table.Object)
	}
	resolved := characteristicTable{standard: catalog.standardFieldsOf(table.Kind, table.Object), attributes: elements.attributes}
	if table.TablePart == nil {
		return resolved, nil
	}
	attributes, ok := elements.tableParts[*table.TablePart]
	if !ok {
		return characteristicTable{}, fmt.Errorf("%s reads table part %s, which that object does not have", where, table.TablePart)
	}
	// A line of a table part keeps one standard field worth addressing here:
	// the reference to the row it belongs to. It has no code and no parent of
	// its own, and the object's ones are not its.
	byID := make(map[uuid.UUID][]Type, len(attributes))
	for _, attribute := range attributes {
		byID[attribute.ID] = attribute.Types
	}
	standard := map[StandardField][]Type{}
	if reference, ok := resolved.standard[RefStandardField]; ok {
		standard[RefStandardField] = reference
	}
	return characteristicTable{standard: standard, attributes: byID}, nil
}

// standardFieldsOf is what the platform gives a table of this kind, with the
// types of each. A register is given nothing here: its own standard fields -
// the period, the recorder, the line number - are not what a characteristic is
// read by, and a description naming one of them is a mistake worth reporting.
func (catalog *Catalog) standardFieldsOf(kind Kind, id uuid.UUID) map[StandardField][]Type {
	referenceKind, ok := characteristicKindTypes[kind]
	if !ok {
		return map[StandardField][]Type{}
	}
	fields := map[StandardField][]Type{RefStandardField: {referenceType(referenceKind, id)}}
	addCode := func(code CatalogCode, descriptionLength int) {
		if code.Length > 0 {
			fields[CodeStandardField] = []Type{catalogCodeType(code)}
		}
		if descriptionLength > 0 {
			fields[DescriptionStandardField] = []Type{{Kind: StringType, Length: descriptionLength}}
		}
	}
	addParent := func(hierarchy Hierarchy) {
		if hierarchy.Enabled {
			fields[ParentStandardField] = []Type{referenceType(referenceKind, id)}
		}
	}
	switch kind {
	case CatalogKind:
		item := catalog.Catalogs[catalog.catalogByID[id]]
		addCode(item.Code, item.DescriptionLength)
		addParent(item.Hierarchy)
	case ChartOfCharacteristicTypesKind:
		item := catalog.ChartsOfCharacteristicTypes[catalog.chartOfCharacteristicTypesByID[id]]
		addCode(item.Code, item.DescriptionLength)
		addParent(item.Hierarchy)
	case ChartOfAccountsKind:
		item := catalog.ChartsOfAccounts[catalog.chartOfAccountsByID[id]]
		addCode(item.Code, item.DescriptionLength)
	case ChartOfCalculationTypesKind:
		item := catalog.ChartsOfCalculationTypes[catalog.chartOfCalculationTypesByID[id]]
		addCode(item.Code, item.DescriptionLength)
	case ExchangePlanKind:
		item := catalog.ExchangePlans[catalog.exchangePlanByID[id]]
		addCode(item.Code, item.DescriptionLength)
	case TaskKind:
		item := catalog.Tasks[catalog.taskByID[id]]
		addCode(CatalogCode{}, item.DescriptionLength)
	}
	return fields
}

// characteristicOwner is one object with characteristics of its own, gathered
// so that the nine kinds are checked by one piece of code rather than nine
// copies of it.
type characteristicOwner struct {
	what            string
	kind            Kind
	id              uuid.UUID
	characteristics []ObjectCharacteristic
}

func (catalog *Catalog) characteristicOwners() []characteristicOwner {
	var owners []characteristicOwner
	add := func(what string, kind Kind, id uuid.UUID, characteristics []ObjectCharacteristic) {
		if len(characteristics) == 0 {
			return
		}
		owners = append(owners, characteristicOwner{what: what, kind: kind, id: id, characteristics: characteristics})
	}
	for _, item := range catalog.Catalogs {
		add("catalog "+item.Name, CatalogKind, item.ID, item.Characteristics)
	}
	for _, item := range catalog.Documents {
		add("document "+item.Name, DocumentKind, item.ID, item.Characteristics)
	}
	for _, item := range catalog.Enumerations {
		add("enumeration "+item.Name, EnumerationKind, item.ID, item.Characteristics)
	}
	for _, item := range catalog.ChartsOfCharacteristicTypes {
		add("chart of characteristic types "+item.Name, ChartOfCharacteristicTypesKind, item.ID, item.Characteristics)
	}
	for _, item := range catalog.ChartsOfAccounts {
		add("chart of accounts "+item.Name, ChartOfAccountsKind, item.ID, item.Characteristics)
	}
	for _, item := range catalog.ChartsOfCalculationTypes {
		add("chart of calculation types "+item.Name, ChartOfCalculationTypesKind, item.ID, item.Characteristics)
	}
	for _, item := range catalog.BusinessProcesses {
		add("business process "+item.Name, BusinessProcessKind, item.ID, item.Characteristics)
	}
	for _, item := range catalog.Tasks {
		add("task "+item.Name, TaskKind, item.ID, item.Characteristics)
	}
	for _, item := range catalog.ExchangePlans {
		add("exchange plan "+item.Name, ExchangePlanKind, item.ID, item.Characteristics)
	}
	return owners
}

// validateCharacteristics resolves every characteristic description against
// the configuration. A description that points at a table or a field that is
// not there does not fail: it reads nothing, and the object silently has no
// additional properties at all.
func (catalog *Catalog) validateCharacteristics() error {
	for _, owner := range catalog.characteristicOwners() {
		for index, characteristic := range owner.characteristics {
			where := fmt.Sprintf("%s characteristics[%d]", owner.what, index)
			if err := catalog.validateCharacteristic(where, owner, characteristic); err != nil {
				return err
			}
		}
	}
	return nil
}

func (catalog *Catalog) validateCharacteristic(where string, owner characteristicOwner, characteristic ObjectCharacteristic) error {
	kinds, err := catalog.resolveCharacteristicTable(where+" types", characteristic.Types.Table)
	if err != nil {
		return err
	}
	keyTypes, err := characteristicFieldTypes(where+" types.key", kinds, characteristic.Types.Key)
	if err != nil {
		return err
	}
	if characteristic.Types.Filter != nil {
		filterTypes, err := characteristicFieldTypes(where+" types.filter", kinds, *characteristic.Types.Filter)
		if err != nil {
			return err
		}
		// The value is checked against the field the way a value is checked
		// anywhere else: a filter the field can never equal cuts the table
		// down to nothing, and the object then offers no characteristics at
		// all without one word being said about why.
		if _, err := catalog.normalizeTypes(where+" types.filter_value", filterTypes, *characteristic.Types.FilterValue); err != nil {
			return err
		}
	}
	for _, optional := range []struct {
		path  string
		field *CharacteristicField
	}{
		{where + " types.data_path", characteristic.Types.DataPath},
		{where + " types.multiple_values_use", characteristic.Types.MultipleValuesUse},
	} {
		if optional.field == nil {
			continue
		}
		if _, err := characteristicFieldTypes(optional.path, kinds, *optional.field); err != nil {
			return err
		}
	}
	values, err := catalog.resolveCharacteristicTable(where+" values", characteristic.Values.Table)
	if err != nil {
		return err
	}
	objectTypes, err := characteristicFieldTypes(where+" values.object", values, characteristic.Values.Object)
	if err != nil {
		return err
	}
	typeTypes, err := characteristicFieldTypes(where+" values.type", values, characteristic.Values.Type)
	if err != nil {
		return err
	}
	if _, err := characteristicFieldTypes(where+" values.value", values, characteristic.Values.Value); err != nil {
		return err
	}
	for _, optional := range []struct {
		path  string
		field *CharacteristicField
	}{
		{where + " values.multiple_values_key", characteristic.Values.MultipleValuesKey},
		{where + " values.multiple_values_order", characteristic.Values.MultipleValuesOrder},
	} {
		if optional.field == nil {
			continue
		}
		if _, err := characteristicFieldTypes(optional.path, values, *optional.field); err != nil {
			return err
		}
	}
	// The two halves and the object are held together by what their fields can
	// hold. A table of values whose object field cannot hold a reference to
	// this object keeps somebody else's values, and one whose kind field
	// cannot hold what the table of kinds is keyed by keeps values of nothing.
	held, err := catalog.expandTypes(objectTypes, nil)
	if err != nil {
		return fmt.Errorf("%s values.object: %w", where, err)
	}
	reference := []Type{referenceType(characteristicKindTypes[owner.kind], owner.id)}
	if !typesIntersect(reference, held) {
		return fmt.Errorf("%s values.object cannot hold a reference to %s, so it keeps no values of it", where, owner.what)
	}
	keyed, err := catalog.expandTypes(keyTypes, nil)
	if err != nil {
		return fmt.Errorf("%s types.key: %w", where, err)
	}
	named, err := catalog.expandTypes(typeTypes, nil)
	if err != nil {
		return fmt.Errorf("%s values.type: %w", where, err)
	}
	if !typesIntersect(keyed, named) {
		return fmt.Errorf("%s values.type cannot hold what types.key holds, so the two tables never join", where)
	}
	return nil
}

func characteristicFieldTypes(where string, table characteristicTable, field CharacteristicField) ([]Type, error) {
	types, ok := table.field(field)
	if ok {
		return types, nil
	}
	if field.Standard != "" {
		return nil, fmt.Errorf("%s names the standard field %s, which that table does not have", where, field.Standard)
	}
	return nil, fmt.Errorf("%s names attribute %s, which that table does not have", where, field.Attribute)
}
