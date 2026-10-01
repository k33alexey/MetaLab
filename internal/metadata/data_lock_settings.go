package metadata

import (
	"fmt"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
)

// Блокировка данных, как её описывает объект метаданных. Two properties, and
// they are not one group however alike they sound: the mode is on almost every
// kind that stores anything, the fields only on the eight that have an object
// of their own.
//
//	                                     режим   поля
//	константа                              +      -
//	справочник, документ, три плана,
//	план обмена, бизнес-процесс, задача     +      +
//	четыре регистра, последовательность     +      -
//	перерасчёт, операция веб-сервиса        +      -
//	конфигурация целиком                    +      -
//
// The syntax assistant gives РежимУправленияБлокировкойДанных to eighteen
// kinds and to the configuration, and ПоляБлокировкиДанных to nine - the eight
// above and the table of an external data source, whose properties are not
// unfolded. The demonstration configuration writes the mode on fourteen kinds
// and the fields on those eight, filled on twenty-six objects.
//
// The mode is carried, not acted upon. ML takes managed locks: a write locks
// what it writes and nothing else. A configuration that asked for automatic
// locking says so, we keep the answer, and the import report says the setting
// is transferred and not implemented - the same treatment the configuration
// root's own mode already gets.

// validateDataLockMode checks the mode wherever it stands.
//
// Three values, not two. The enumeration a metadata object uses is
// РежимУправленияБлокировкойДанныхПоУмолчанию, and it has a third value the
// enumeration used at runtime has not: «автоматический и управляемый», the
// answer of an object that is read under one regime and written under the
// other. We had two of the three on a constant, so a configuration using the
// third would have been refused at the door.
func validateDataLockMode(path string, mode project.DataLockControlMode) []string {
	switch mode {
	case "", project.AutomaticDataLock, project.ManagedDataLock, project.AutomaticAndManagedDataLock:
		return nil
	}
	return []string{path + " must be automatic, managed or automatic-and-managed"}
}

// validateDataLockFields resolves the fields an object may be locked by
// against the object that carries them.
//
// The list is looser than the one input by string walks, and deliberately so.
// Input by string is a search on every keystroke, so the help states a rule -
// a string or a number, indexed - and a field that breaks it describes a
// search that never matches. A lock is taken once per write over whatever the
// developer named, and the help states no rule at all: it says of three kinds
// of field that they are "не рекомендуется", which is advice and not a
// prohibition. Refusing what the prototype merely discourages would refuse a
// configuration the prototype accepts.
//
// What is refused is a name that is not there, because a lock by a field the
// object has not got is a lock nobody can take.
func validateDataLockFields(fields []ObjectField, kind Kind, attributes []Attribute) []string {
	standard := standardNames(standardFieldsOfKind(kind))
	declared := make(map[string]bool, len(attributes))
	for _, attribute := range attributes {
		declared[strings.ToLower(attribute.Name)] = true
	}
	var issues []string
	seen := map[string]bool{}
	for index, field := range fields {
		prefix := fmt.Sprintf("data_lock_fields[%d]", index)
		var key string
		switch {
		case field.Standard != "" && field.Attribute != "":
			issues = append(issues, prefix+" names both a standard field and an attribute")
			continue
		case field.Standard != "":
			canonical, ok := standard[foldStandardName(field.Standard)]
			if !ok {
				issues = append(issues, prefix+".standard is not a standard field of this kind of object")
				continue
			}
			key = "standard:" + canonical
		case field.Attribute != "":
			if !declared[strings.ToLower(field.Attribute)] {
				issues = append(issues, prefix+".attribute is not an attribute of this object")
				continue
			}
			key = "attribute:" + strings.ToLower(field.Attribute)
		default:
			issues = append(issues, prefix+" names neither a standard field nor an attribute")
			continue
		}
		if seen[key] {
			issues = append(issues, prefix+" is already among the fields this object is locked by")
		}
		seen[key] = true
	}
	return issues
}

func cloneDataLockFields(fields []ObjectField) []ObjectField { return slices.Clone(fields) }
