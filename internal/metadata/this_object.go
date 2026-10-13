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
// признака учета>; ВнешнийИсточникДанныхТаблицаОбъект.<Имя поля>).
//
// The fields are read off the fields a restriction of reading the object
// names (restrictionFieldsOf): the fields of the object itself, not those of
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
				for _, field := range catalog.restrictionFieldsOf(object.kind, id) {
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
			result[project.ModuleAddress{Kind: string(object.kind), Object: object.name, Module: module}] = members
		}
	}
	return result
}
