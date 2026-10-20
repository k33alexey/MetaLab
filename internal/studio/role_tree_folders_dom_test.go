package studio

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// roleTreeFoldersWalk opens the window of the role Кладовщик and reads the
// folders under the catalog Товары, the document ПродажаТоваров and the
// register ОстаткиТоваров, the rows under the table part of the document,
// and the rights of the empty folder «Табличные части» of Товары once chosen.
const roleTreeFoldersWalk = `(async () => {
  const studio = window.__studio, editor = document.querySelector('#role-editor');
  [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === 'Кладовщик').click();
  await studio.settle();
  const object = name => [...editor.querySelectorAll('.role-tree-item[data-object]')].find(item => item.title === name).closest('details');
  const beneath = details => [...details.querySelector(':scope > .studio-tree-children').children].map(item => ({
    caption: item.matches('details') ? item.querySelector(':scope > .studio-tree-row').textContent.trim() : item.textContent.trim(),
    folds: item.matches('details')}));
  const answer = {goods: beneath(object('Товары')), sale: beneath(object('ПродажаТоваров')), stock: beneath(object('ОстаткиТоваров')), service: beneath(object('Обмен'))};
  answer.table = beneath(editor.querySelector('[data-group="10000000-0000-4000-8000-000000000202 table-parts"]').closest('details')
    .querySelector('.studio-tree-children details'));
  editor.querySelector('[data-group="10000000-0000-4000-8000-000000000101 table-parts"]').click(); await studio.settle();
  answer.empty = [...editor.querySelectorAll('.role-permissions .check-list-row')].map(row => row.textContent.trim() + (row.querySelector('input').checked ? ' ✓' : ''));
  return answer;
})()`

// The tree of rights shows under an object every folder of its kind, in the
// kind's order, the empty ones too, without the mark of opening, as the
// prototype's window of a role (pass of 20.10.2026, block-00/rol «Папки
// частей»; written out here, not read off the descriptors): a catalog and a
// document - Реквизиты, Табличные части, Стандартные реквизиты, Команды; a
// register of accumulation - Измерения, Ресурсы, Реквизиты, Стандартные
// реквизиты, Команды. An empty folder chosen shows the rights it gives over
// what it would hold. Under a table part its attributes stand without
// НомерСтроки; the operations of a web service stand right under it, with no
// folder. Defects caught: the empty folders left out; one order for
// every kind; an empty folder without rights; the line number under a table
// part; a folder «Операции» under a web service.
func TestTheTreeOfRightsShowsEveryFolderOfTheKind(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	writeProjectFile(t, root, "metadata/web-services/Обмен/object.yaml", "format: 1\nid: "+uuid.MustNew().String()+"\nname: Обмен\ntitle: {ru: Обмен}\n"+
		"namespace: http://example.org/exchange/1.0\noperations:\n  - {id: "+uuid.MustNew().String()+", name: Проверка, title: {ru: Проверка}, procedure: Проверка, "+
		"return_type: {namespace: \"http://www.w3.org/2001/XMLSchema\", name: string}}\n")
	writeProjectFile(t, root, "metadata/web-services/Обмен/МодульСервиса.bsl", "Функция Проверка()\n\tВозврат \"\";\nКонецФункции\n")
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, roleTreeFoldersWalk)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		Caption string
		Folds   bool
	}
	var got struct {
		Goods, Sale, Stock, Table, Service []row
		Empty                              []string
	}
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	captions := func(rows []row) []string {
		var result []string
		for _, item := range rows {
			result = append(result, item.Caption)
		}
		return result
	}
	reference := []string{"Реквизиты", "Табличные части", "Стандартные реквизиты", "Команды"}
	for name, rows := range map[string][]row{"Товары": got.Goods, "ПродажаТоваров": got.Sale} {
		if !slices.Equal(captions(rows), reference) {
			t.Errorf("under %s the folders %v, want %v", name, captions(rows), reference)
		}
	}
	if want := []string{"Измерения", "Ресурсы", "Реквизиты", "Стандартные реквизиты", "Команды"}; !slices.Equal(captions(got.Stock), want) {
		t.Errorf("under ОстаткиТоваров the folders %v, want %v", captions(got.Stock), want)
	}
	if len(got.Goods) == 4 && (got.Goods[1].Folds || !got.Goods[2].Folds) {
		t.Errorf("Товары: the empty «Табличные части» folds %v, the standard attributes %v; want an empty folder without the mark of opening",
			got.Goods[1].Folds, got.Goods[2].Folds)
	}
	if want := []string{"Проверка"}; !slices.Equal(captions(got.Service), want) {
		t.Errorf("under the web service %v, want its operation right under it", captions(got.Service))
	}
	if want := []string{"Товар", "Количество", "Цена"}; !slices.Equal(captions(got.Table), want) {
		t.Errorf("under the table part of ПродажаТоваров %v, want %v", captions(got.Table), want)
	}
	if want := []string{"Просмотр", "Редактирование"}; !slices.Equal(got.Empty, want) {
		t.Errorf("the empty «Табличные части» of Товары shows %v, want %v", got.Empty, want)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
