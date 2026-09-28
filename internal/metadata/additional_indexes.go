package metadata

import (
	"fmt"
	"strings"
)

// Дополнительные индексы — an index the developer asks the database for by
// hand, beside the ones the platform builds on its own.
//
// It appeared in 8.3.26, so the demonstration configuration - which is older -
// uses it nowhere and could neither show the gap nor confirm the shape.
// Everything here comes from the syntax assistant of 8.3.27.
//
// **The map said four registers; the help says fourteen kinds.** Справочник,
// документ, журнал документов, задача, бизнес-процесс, план видов
// характеристик, план счетов, план видов расчёта, план обмена,
// последовательность and all four registers. That is the twelve kinds that
// carry the object's full-text flag plus the journal and the sequence - every
// kind with a table of its own, in other words, and the journal is in the list
// although it stores nothing itself. A constant, an enumeration and a
// recalculation have it nowhere.
//
// An additional index has four properties and, unlike an aggregate, a name:
//
//   - Имя - what the index is called;
//   - Таблица - which table of the object it is built over: the object's own,
//     one of its table parts, or a virtual one;
//   - ИндексируемыеПоля - the key fields, in order;
//   - ДополнительныеПоля - non-key fields carried inside the index. The help
//     adds that whether they really land there depends on the database, which
//     is PostgreSQL's INCLUDE said in other words.
//
// A field of either list is ПолеИндекса, and that type has exactly one
// property - its name. No direction: the prototype gives no ascending or
// descending here.
//
// **Order of key fields is part of the index.** This is the opposite of an
// aggregate, where the same dimensions folded the same way are the same cut
// whatever order they were written in - see accumulation_aggregates.go. An
// index on (склад, дата) answers questions an index on (дата, склад) does not,
// so two indexes differing only in order are two different indexes and neither
// is a copy of the other.

const (
	// maxAdditionalIndexes bounds the collection. The prototype states no
	// limit; this guards against a file that is not a list of indexes. Every
	// index is paid for on every write to the table.
	maxAdditionalIndexes = 64
	// maxAdditionalIndexFields is PostgreSQL's ceiling on the columns of one
	// index, key and included together. It is ours and not the prototype's,
	// and it is enforced here rather than discovered during a migration that
	// cannot finish.
	maxAdditionalIndexFields = 32
)

// AdditionalIndex is one index the configuration asks for.
type AdditionalIndex struct {
	Name string `yaml:"name" json:"name"`
	// Table is the object's own table when empty, and otherwise the name of a
	// table part or of one of the kind's virtual tables.
	Table string `yaml:"table,omitempty" json:"table,omitempty"`
	// IndexedFields are the key fields in the order they are searched by;
	// AdditionalFields ride along inside the index without being searched.
	IndexedFields    []string `yaml:"indexed_fields" json:"indexedFields"`
	AdditionalFields []string `yaml:"additional_fields,omitempty" json:"additionalFields,omitempty"`
}

// indexableTables is what one object offers an index: the folded name of each
// table against the folded names of the fields in it. The object's own table
// is the empty key.
type indexableTables map[string]map[string]bool

// virtualIndexTables is the name of every virtual table the prototype gives a
// kind, taken from its catalogue of query tables. A virtual table may be named
// by an index and its fields are deliberately not resolved here - they are not
// the object's fields, they are the query language's, and the model of them
// arrives with block 6. Until then such an index is carried as written: the
// second category of the import report, not a silent loss.
//
// «Изменения» stands beside them for every kind that can be registered in an
// exchange plan, because the prototype lists it as a table of that kind like
// any other.
func virtualIndexTables(kind Kind) []string {
	changes := []string{"Изменения"}
	switch kind {
	case CatalogKind, DocumentKind, ChartOfCharacteristicTypesKind, ChartOfAccountsKind,
		ChartOfCalculationTypesKind, ExchangePlanKind:
		return changes
	case TaskKind:
		return append([]string{"ЗадачиПоИсполнителю"}, changes...)
	case BusinessProcessKind:
		return append([]string{"Точки"}, changes...)
	case SequenceKind:
		return append([]string{"Границы"}, changes...)
	case InformationRegisterKind:
		return append([]string{"СрезПервых", "СрезПоследних"}, changes...)
	case AccumulationRegisterKind:
		return append([]string{"Остатки", "Обороты", "ОстаткиИОбороты"}, changes...)
	case AccountingRegisterKind:
		return append([]string{
			"Остатки", "Обороты", "ОстаткиИОбороты", "ОборотыДтКт", "ДвиженияССубконто", "Субконто",
		}, changes...)
	case CalculationRegisterKind:
		return append([]string{"ДанныеГрафика", "ФактическийПериодДействия"}, changes...)
	default:
		return nil
	}
}

// objectIndexTables describes an object that has an own table and table parts.
//
// The table parts the platform gives are here beside the declared ones, under
// both names they answer to: an index over the kinds of analytics of an
// account is built over a real table like any other, and refusing it because
// the developer did not declare that part would refuse a working index.
func objectIndexTables(kind Kind, attributes []Attribute, parts []TablePart) indexableTables {
	tables := indexableTables{"": fieldNameSet(standardFieldsOfKind(kind), attributeNames(attributes))}
	for _, part := range parts {
		tables[strings.ToLower(part.Name)] = fieldNameSet(tablePartStandardFields, attributeNames(part.Attributes))
	}
	for _, part := range standardTablePartsOfKind(kind) {
		fields := fieldNameSet(part.fields, nil)
		tables[strings.ToLower(part.ru)] = fields
		tables[strings.ToLower(part.en)] = fields
	}
	return tables
}

// recordIndexTables describes a register or anything else whose own table is
// the only one it has. Dimensions, resources and attributes are all fields of
// that one table.
//
// It takes names rather than fields because the four registers do not agree on
// what a field is: an accounting register and a calculation register keep
// structures of their own, which is a point of its own further down the block.
// An index needs the name and nothing else.
func recordIndexTables(standard []standardField, groups ...[]string) indexableTables {
	var all []string
	for _, group := range groups {
		all = append(all, group...)
	}
	return indexableTables{"": fieldNameSet(standard, all)}
}

// attributeNames is the names of ordinary fields, for the callers whose fields
// are ordinary.
func attributeNames(attributes []Attribute) []string {
	names := make([]string, len(attributes))
	for position, attribute := range attributes {
		names[position] = attribute.Name
	}
	return names
}

func fieldNameSet(standard []standardField, fields []string) map[string]bool {
	names := make(map[string]bool, len(standard)*2+len(fields))
	for folded := range standardNames(standard) {
		names[folded] = true
	}
	for _, field := range fields {
		names[foldStandardName(field)] = true
	}
	return names
}

// validateAdditionalIndexes resolves every index against the object that asks
// for it.
//
// What is refused, and why each one:
//
//   - an index with no name, or two indexes under one name. The prototype
//     names them, and a migration has to say which index it is creating,
//     dropping or leaving alone;
//   - an index with no key fields, which is not an index;
//   - a table the object has not got, and a field the named table has not got:
//     both describe an index nothing can build;
//   - a field repeated inside one list, and a field that is a key field and an
//     included one at once. It is already in the index, and asking twice does
//     not put it there twice;
//   - more than PostgreSQL will take in one index.
//
// What is deliberately not refused: two indexes over the same fields of the
// same table. They have names, a developer can tell them apart and maintain
// them, and they may differ in what they carry along - unlike two aggregates,
// which have nothing to tell apart at all.
func validateAdditionalIndexes(indexes []AdditionalIndex, kind Kind, tables indexableTables) []string {
	if len(indexes) == 0 {
		return nil
	}
	if len(indexes) > maxAdditionalIndexes {
		return []string{fmt.Sprintf("additional_indexes must not contain more than %d items", maxAdditionalIndexes)}
	}
	virtual := make(map[string]bool)
	for _, name := range virtualIndexTables(kind) {
		virtual[strings.ToLower(name)] = true
	}
	var issues []string
	named := make(map[string]int, len(indexes))
	for position, index := range indexes {
		prefix := fmt.Sprintf("additional_indexes[%d]", position)
		switch folded := strings.ToLower(index.Name); {
		case index.Name == "":
			issues = append(issues, prefix+".name must not be empty")
		case !validIdentifier(index.Name):
			issues = append(issues, prefix+".name must be an identifier")
		default:
			if previous, exists := named[folded]; exists {
				issues = append(issues, fmt.Sprintf("%s.name repeats the name of additional_indexes[%d]", prefix, previous))
			}
			named[folded] = position
		}
		if len(index.IndexedFields) == 0 {
			issues = append(issues, prefix+".indexed_fields must name at least one field")
		}
		if total := len(index.IndexedFields) + len(index.AdditionalFields); total > maxAdditionalIndexFields {
			issues = append(issues, fmt.Sprintf(
				"%s names %d fields, and an index takes at most %d", prefix, total, maxAdditionalIndexFields))
		}
		table, known := tables[strings.ToLower(index.Table)]
		if !known {
			if virtual[strings.ToLower(index.Table)] {
				// Named a virtual table of this kind. The index is carried and
				// its fields are left alone - see virtualIndexTables.
				continue
			}
			issues = append(issues, prefix+".table names "+describeIndexTable(index.Table)+", which this object has not got")
			continue
		}
		seen := make(map[string]bool, len(index.IndexedFields)+len(index.AdditionalFields))
		for _, list := range []struct {
			name   string
			fields []string
		}{{"indexed_fields", index.IndexedFields}, {"additional_fields", index.AdditionalFields}} {
			for at, field := range list.fields {
				where := fmt.Sprintf("%s.%s[%d]", prefix, list.name, at)
				folded := foldStandardName(field)
				switch {
				case field == "":
					issues = append(issues, where+" must name a field")
				case !table[folded]:
					issues = append(issues, where+" names "+field+", which is not a field of "+describeIndexTable(index.Table))
				case seen[folded]:
					issues = append(issues, where+" names "+field+", which is already in this index")
				default:
					seen[folded] = true
				}
			}
		}
	}
	return issues
}

func describeIndexTable(table string) string {
	if table == "" {
		return "the object's own table"
	}
	return "table " + table
}

func cloneAdditionalIndexes(indexes []AdditionalIndex) []AdditionalIndex {
	if indexes == nil {
		return nil
	}
	cloned := make([]AdditionalIndex, len(indexes))
	for position, index := range indexes {
		index.IndexedFields = append([]string(nil), index.IndexedFields...)
		index.AdditionalFields = append([]string(nil), index.AdditionalFields...)
		cloned[position] = index
	}
	return cloned
}

// accountingFieldNames and calculationDimensionNames are the same thing for
// the two registers that keep structures of their own instead of the ordinary
// field. Until they share one - a point of its own further down the block -
// the name has to be fetched from each.
func accountingFieldNames(fields []AccountingRegisterField) []string {
	names := make([]string, len(fields))
	for position, field := range fields {
		names[position] = field.Name
	}
	return names
}

func calculationDimensionNames(dimensions []CalculationRegisterDimension) []string {
	names := make([]string, len(dimensions))
	for position, dimension := range dimensions {
		names[position] = dimension.Name
	}
	return names
}

// sequenceStandardFields is what a sequence's own table holds beside its
// dimensions: the recorder and the moment it sits at. It is written here and
// not in standardFieldsOfKind because a sequence carries no descriptions of
// its standard fields in the model, and giving it a set there would start
// refusing dimension names as reserved - a change of its own, not this one's.
var sequenceStandardFields = []standardField{{"Период", "Period"}, {"Регистратор", "Recorder"}}

func sequenceDimensionNames(dimensions []SequenceDimension) []string {
	names := make([]string, len(dimensions))
	for position, dimension := range dimensions {
		names[position] = dimension.Name
	}
	return names
}

func journalColumnNames(columns []JournalColumn) []string {
	names := make([]string, len(columns))
	for position, column := range columns {
		names[position] = column.Name
	}
	return names
}
