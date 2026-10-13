package metadata

import (
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/kinds"
)

// ManagedObject is an object of the configuration the code reaches through
// the global context: by its collection there (Справочники.Банки,
// ПараметрыСеанса.ТекущийПользователь), or through the manager of the object
// it lies inside (ВнешниеИсточникиДанных.Склад.Таблицы.Товары).
type ManagedObject struct {
	Kind Kind
	// Name is the object's name after its owner's, as the configuration
	// tree names it: Склад.Товары, Начисления.Перерасчет.
	Name string
	// Members are those of its manager (kinds.Descriptor.Manager), and the
	// items it has besides: the values of an enumeration and the predefined
	// items of a catalog or a chart (help: ПеречислениеМенеджер.<Имя
	// перечисления>.<Имя значения>, СправочникМенеджер.<Имя справочника>.
	// <Имя предопределенного элемента справочника>), and the objects inside
	// it called as its methods (kinds.Descriptor.OwnerMethod: the functions
	// of a source of data). None for a kind with no manager, whose
	// collection gives the value itself.
	Members []kinds.Member
}

// ManagedObjects is every object of the configuration the code reaches
// through the global context, by kind and name.
func (catalog *Catalog) ManagedObjects() []ManagedObject {
	if catalog == nil {
		return nil
	}
	items := map[Kind]map[string][]kinds.Member{}
	for _, kind := range predefinedValueKinds {
		if kind.items {
			items[kind.kind] = catalog.predefinedMembers(kind.kind)
		}
	}
	for _, object := range catalog.objects {
		descriptor, ok := kinds.Of(object.kind)
		if !ok || !descriptor.OwnerMethod() {
			continue
		}
		dot := strings.LastIndex(object.name, ".")
		owner, name := strings.ToLower(object.name[:max(dot, 0)]), object.name[dot+1:]
		kind := descriptor.Layout().Owner
		if items[kind] == nil {
			items[kind] = map[string][]kinds.Member{}
		}
		items[kind][owner] = append(items[kind][owner], kinds.Member{Name: kinds.Words{Russian: name, English: name}, Method: true})
	}
	var result []ManagedObject
	for _, object := range catalog.objects {
		descriptor, ok := kinds.Of(object.kind)
		if !ok {
			continue
		}
		manager, managed := descriptor.Manager()
		if !managed && !descriptor.InGlobalContext() {
			continue
		}
		members := append(manager.Members, items[object.kind][strings.ToLower(object.name)]...)
		result = append(result, ManagedObject{Kind: object.kind, Name: object.name, Members: members})
	}
	slices.SortFunc(result, func(left, right ManagedObject) int {
		return strings.Compare(string(left.Kind)+"\x00"+left.Name, string(right.Kind)+"\x00"+right.Name)
	})
	return result
}

// predefinedMembers is the items of each object of a kind with items, as
// members of its manager, by the object's name in lower case. An item is
// named alike in both languages of the code.
func (catalog *Catalog) predefinedMembers(kind Kind) map[string][]kinds.Member {
	list, ok := catalog.objectsOf(kind)
	if !ok {
		return nil
	}
	result := make(map[string][]kinds.Member, list.Len())
	for index := range list.Len() {
		definition := list.Index(index)
		values := predefinedItemsOf(kind, definition)
		members := make([]kinds.Member, 0, values.Len())
		for position := range values.Len() {
			name := values.Index(position).FieldByName("Name").String()
			members = append(members, kinds.Member{Name: kinds.Words{Russian: name, English: name}})
		}
		result[strings.ToLower(definition.FieldByName("Name").String())] = members
	}
	return result
}
