package metadata

import (
	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
)

// ThisObjectMembers is what ЭтотОбъект has in each module of an object of the
// configuration that runs around an object of data (kinds.Descriptor.
// ThisObject), by where the module lies: the members of the type, and for an
// object its fields too - the version of its data, its standard attributes,
// the attributes, table parts and fields it declares, a common attribute it
// takes among them, and the standard table parts of its kind (help:
// СправочникОбъект.<Имя реквизита>, <Имя табличной части>, <Имя общего
// реквизита>; ЗадачаОбъект.<Имя реквизита адресации>; ПланСчетовОбъект.<Имя
// признака учета>; ВнешнийИсточникДанныхТаблицаОбъект.<Имя поля>), and for
// the value manager of a constant the common attributes it holds beside its
// value (КонстантаМенеджерЗначения.<Имя>.<Имя общего реквизита>).
//
// The fields are read off every field the object keeps (objectFieldsOf),
// the parent of a catalog that is not hierarchical too: the help of
// СправочникОбъект does not make them hang on the object, as the help of the
// table of a query does. The fields of the object itself, not those of
// its table parts. The flags of accounting of an ext dimension are fields of
// the lines of ВидыСубконто and no member of the object.
func (catalog *Catalog) ThisObjectMembers() map[project.ModuleAddress][]kinds.Member {
	result := map[project.ModuleAddress][]kinds.Member{}
	if catalog == nil {
		return result
	}
	for id, object := range catalog.objects {
		descriptor, ok := kinds.Of(object.kind)
		if !ok {
			continue
		}
		for _, module := range descriptor.Layout().Modules {
			objectType, ok := descriptor.ThisObject(module)
			if !ok {
				continue
			}
			members := objectType.Members
			if objectType.Declared {
				for _, field := range catalog.objectFieldsOf(object.kind, id) {
					if field.parent != "" || field.group == ExtDimensionAccountingFlagsPart {
						continue
					}
					english := field.english
					if english == "" {
						english = field.name
					}
					members = append(members, kinds.Member{Name: kinds.Words{Russian: field.name, English: english}})
				}
			}
			if valueFields := kindCompositionOf[object.kind].valueFields; valueFields != nil {
				for _, field := range valueFields(catalog, id) {
					members = append(members, kinds.Member{Name: kinds.Words{Russian: field.Name, English: field.Name}})
				}
			}
			result[project.ModuleAddress{Kind: string(object.kind), Object: object.name, Module: module}] = members
		}
	}
	return result
}
