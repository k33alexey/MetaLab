package metadata

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
)

// Every object the code reaches through the global context is there, under
// its name after its owner's, with the members of its manager and what it
// has besides; nothing the code does not reach is. Catches a kind left out -
// an object inside another, a session parameter -, a kind let in that has no
// collection in the global context (a common module, a defined type), the
// predefined items or the values of one object offered on another of its
// kind, and the functions of a source not called through it. Expected are
// the help's (Глобальный контекст; СправочникМенеджер.<Имя>.<Имя
// предопределенного элемента справочника>; ПеречислениеМенеджер.<Имя>.<Имя
// значения>; ВнешнийИсточникДанныхМенеджер.<Имя>.<Имя функции>()).
func TestManagedObjectsAreWhatTheCodeReaches(t *testing.T) {
	t.Parallel()
	root := compositeProject(t)
	warehouses := filepath.Join(root, "metadata", string(CatalogKind), "Склады", "object.yaml")
	file, err := os.OpenFile(warehouses, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("predefined:\n  - {id: " + compositeID(900) + ", name: Основной, code: \"001\"}\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	objects := map[string]ManagedObject{}
	for _, object := range catalog.ManagedObjects() {
		objects[string(object.Kind)+" "+object.Name] = object
	}
	has := func(key, member string, method bool) bool {
		return slices.ContainsFunc(objects[key].Members, func(item kinds.Member) bool {
			return item.Name.Russian == member && item.Method == method
		})
	}
	for _, key := range []string{"catalogs Склады", "catalogs Номенклатура", "enumerations ВидыТоваров",
		"external-data-sources Склад", "external-data-source-tables Склад.Товары", "external-data-source-cubes Склад.Продажи",
		"external-data-source-dimension-tables Склад.Продажи.Товары", "recalculations Начисления.Перерасчет",
		"ws-references Поставщик", "settings-storages Варианты", "filter-criteria Связанные"} {
		if len(objects[key].Members) == 0 {
			t.Errorf("%s is not reached through a manager: %+v", key, objects[key])
		}
	}
	if object, ok := objects["session-parameters ТекущийПользователь"]; !ok || len(object.Members) != 0 {
		t.Errorf("a session parameter is reached by ПараметрыСеанса, with no manager: %+v, %v", object, ok)
	}
	for _, key := range []string{"common-modules Общий", "defined-types ДенежнаяСумма", "roles Кладовщик",
		"external-data-source-functions Склад.Остаток", "subsystems Склад"} {
		if _, ok := objects[key]; ok {
			t.Errorf("%s is reached through no manager of the global context", key)
		}
	}
	for _, test := range []struct {
		key, member string
		method, has bool
	}{
		{"catalogs Склады", "Основной", false, true},
		{"catalogs Склады", "НайтиПоКоду", true, true},
		{"catalogs Номенклатура", "Основной", false, false},
		{"enumerations ВидыТоваров", "Товар", false, true},
		{"catalogs Номенклатура", "Товар", false, false},
		{"external-data-sources Склад", "Остаток", true, true},
		{"external-data-sources Склад", "Таблицы", false, true},
		{"external-data-source-cubes Склад.Продажи", "Остаток", true, false},
	} {
		if has(test.key, test.member, test.method) != test.has {
			t.Errorf("%s has %s (method %v): %v, want %v", test.key, test.member, test.method, !test.has, test.has)
		}
	}
	if !slices.IsSortedFunc(catalog.ManagedObjects(), func(left, right ManagedObject) int {
		return strings.Compare(string(left.Kind)+" "+left.Name, string(right.Kind)+" "+right.Name)
	}) {
		t.Error("the objects are not in the order of kind and name, and come out of a map differently each time")
	}
}
