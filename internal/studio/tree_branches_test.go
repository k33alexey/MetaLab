package studio

import (
	"slices"
	"testing"
	"time"
)

// titlesOf is the captions of the nodes, in their order.
func titlesOf(nodes []Node) []string {
	var titles []string
	for _, node := range nodes {
		titles = append(titles, node.Title)
	}
	return titles
}

// An external data source shows its tables, then its cubes; a table shows its
// fields, and a cube its dimension tables above its dimensions and resources,
// as the prototype's tree does (inventory of block 00, «Ветки под объектом»,
// snapshot derevo/10-vneshnij-istochnik). Defects caught: an object inside a
// source shown without the branches of its parts; the cubes above the tables
// (the collections in the order of their folders' names); the dimension tables
// below the cube's own forms; a field, a dimension or a resource missing from
// its branch.
func TestObjectsInsideASourceShowTheirBranches(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	const source = "metadata/external-data-sources/Склад"
	writeProjectFile(t, root, source+"/object.yaml", "format: 1\nid: f9000000-0000-4000-8000-000000000031\nname: Склад\ntitle: {ru: Склад}\n")
	writeProjectFile(t, root, source+"/tables/Товары/object.yaml", "format: 1\nid: f9000000-0000-4000-8000-000000000032\nname: Товары\ntitle: {ru: Товары}\n"+
		"name_in_data_source: dbo.Goods\ndata_type: non-object\nfields:\n"+
		"  - {id: f9000000-0000-4000-8000-000000000033, name: Код, title: {ru: Код}, types: [{kind: string, length: 20}], name_in_data_source: code}\n")
	writeProjectFile(t, root, source+"/cubes/Продажи/object.yaml", "format: 1\nid: f9000000-0000-4000-8000-000000000034\nname: Продажи\ntitle: {ru: Продажи}\n"+
		"name_in_data_source: Sales\ndimensions:\n"+
		"  - {id: f9000000-0000-4000-8000-000000000035, name: Склад, title: {ru: Склад}, types: [{kind: external-data-source-dimension-table, reference: f9000000-0000-4000-8000-000000000037}]}\n"+
		"resources:\n"+
		"  - {id: f9000000-0000-4000-8000-000000000036, name: Количество, title: {ru: Количество}, types: [{kind: number, precision: 15, scale: 3}], name_in_data_source: Quantity}\n")
	writeProjectFile(t, root, source+"/cubes/Продажи/dimension-tables/Склады/object.yaml", "format: 1\nid: f9000000-0000-4000-8000-000000000037\nname: Склады\ntitle: {ru: Склады}\n"+
		"name_in_data_source: Stores\nfields:\n"+
		"  - {id: f9000000-0000-4000-8000-000000000038, name: Адрес, title: {ru: Адрес}, types: [{kind: string, length: 50}], name_in_data_source: address}\n")
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	find := func(id string) Node {
		t.Helper()
		node, ok := findNodeByID(snapshot.Tree, id)
		if !ok {
			t.Fatalf("the tree has no %s", id)
		}
		return node
	}
	sourceNode := find(source + "/object.yaml")
	if got := titlesOf(sourceNode.Children); len(got) < 2 || !slices.Equal(got[:2], []string{"Таблицы", "Кубы"}) {
		t.Errorf("the source shows %v, want Таблицы and then Кубы first", got)
	}
	for _, check := range []struct {
		id, branch string
		first      []string
		holds      []string
	}{
		{"f9000000-0000-4000-8000-000000000032", "Поля", []string{"Поля", "Формы", "Команды", "Макеты"}, []string{"Код"}},
		{"f9000000-0000-4000-8000-000000000034", "Измерения", []string{"Таблицы измерений", "Измерения", "Ресурсы", "Формы", "Команды", "Макеты"}, []string{"Склад"}},
		{"f9000000-0000-4000-8000-000000000034", "Ресурсы", nil, []string{"Количество"}},
		{"f9000000-0000-4000-8000-000000000037", "Поля", []string{"Поля", "Формы", "Команды", "Макеты"}, []string{"Адрес"}},
	} {
		node := find(check.id)
		got := titlesOf(node.Children)
		if check.first != nil && (len(got) < len(check.first) || !slices.Equal(got[:len(check.first)], check.first)) {
			t.Errorf("%s shows %v, want %v first", node.Title, got, check.first)
		}
		index := slices.Index(got, check.branch)
		if index < 0 {
			continue
		}
		if held := titlesOf(node.Children[index].Children); !slices.Equal(held, check.holds) {
			t.Errorf("%s: %s holds %v, want %v", node.Title, check.branch, held, check.holds)
		}
	}
}

// TestParentsThatComeBackLeaveTheirSubsystemsAtTheTop catches the nesting of
// subsystems under their parents looping on a chain of parents that comes
// back to itself, or losing what is in it: such subsystems stay at the top,
// a nested one under its parent, so the tree still shows what the loader
// refuses. The walk is given a few seconds, not the ten minutes a hung test
// would take.
func TestParentsThatComeBackLeaveTheirSubsystemsAtTheTop(t *testing.T) {
	t.Parallel()
	nodes := []Node{{ID: "a", Title: "А"}, {ID: "b", Title: "Б"}, {ID: "c", Title: "В"}, {ID: "d", Title: "Г"}}
	parents := map[string]string{"a": "b", "b": "a", "c": "d"}
	done := make(chan []Node, 1)
	go func() { done <- nestUnderParents(nodes, parents) }()
	select {
	case top := <-done:
		if got := titlesOf(top); !slices.Equal(got, []string{"А", "Б", "Г"}) {
			t.Fatalf("the top of the tree is %v, want А, Б and Г", got)
		}
		if got := titlesOf(top[2].Children); !slices.Equal(got, []string{"В"}) {
			t.Fatalf("Г holds %v, want В", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a chain of parents that comes back to itself is walked for ever")
	}
}
