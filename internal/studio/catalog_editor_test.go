package studio

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestCatalogEditorCreateReadSaveAndConflicts(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	created, err := workspace.CreateCatalog("Товары")
	if err != nil {
		t.Fatal(err)
	}
	if created.Catalog.Code.Type != metadata.StringType || created.Catalog.Code.Length != 9 || !created.Catalog.Code.Auto || !created.Catalog.Code.Unique ||
		created.Catalog.DescriptionLength != 64 || len(created.Catalog.Attributes) != 0 {
		t.Fatalf("unexpected new catalog defaults: %+v", created.Catalog)
	}
	if _, err := workspace.CreateCatalog("ТОВАРЫ"); err == nil {
		t.Fatal("duplicate catalog name accepted")
	}
	if _, err := workspace.CreateCatalog("../Справочник"); err == nil {
		t.Fatal("invalid name accepted")
	}
	if _, err := workspace.ReadCatalogEditor("configuration.yaml"); !errors.Is(err, ErrInvalidSourcePath) {
		t.Fatalf("non-catalog path: %v", err)
	}
	if _, err := workspace.ReadCatalogEditor("metadata/catalogs/" + created.Catalog.ID.String() + ".yaml"); !errors.Is(err, ErrInvalidSourcePath) {
		t.Fatalf("flat legacy path must be rejected: %v", err)
	}

	updated := created.Catalog
	updated.DescriptionLength = 300
	updated.Attributes = []metadata.Attribute{{
		ID: uuid.MustNew(), Name: "Артикул", Title: metadata.LocalizedText{"ru": "Артикул"},
		Types: []metadata.Type{{Kind: metadata.StringType, Length: 50}}, Indexed: true,
	}}
	updated.TableParts = []metadata.TablePart{{
		ID: uuid.MustNew(), Name: "Партии", Title: metadata.LocalizedText{"ru": "Партии"},
		Attributes: []metadata.Attribute{{ID: uuid.MustNew(), Name: "Количество", Title: metadata.LocalizedText{"ru": "Количество"}, Types: []metadata.Type{{Kind: metadata.NumberType, Precision: 15, Scale: 3}}}},
	}}
	saved, err := workspace.SaveCatalogEditor(created.Path, updated, created.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision == created.Revision {
		t.Fatal("revision did not change")
	}
	if len(saved.Catalog.Attributes) != 1 || len(saved.Catalog.TableParts) != 1 || saved.Catalog.DescriptionLength != 300 {
		t.Fatalf("save did not persist edits: %+v", saved.Catalog)
	}
	if _, err := workspace.SaveCatalogEditor(created.Path, updated, created.Revision); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("stale save: %v", err)
	}
	invalid := saved.Catalog
	invalid.ID = uuid.MustNew()
	if _, err := workspace.SaveCatalogEditor(saved.Path, invalid, saved.Revision); err == nil {
		t.Fatal("identity change accepted")
	}
	invalid = saved.Catalog
	invalid.Code.Length = 0
	if _, err := workspace.SaveCatalogEditor(saved.Path, invalid, saved.Revision); err == nil {
		t.Fatal("invalid code length accepted")
	}
	again, err := workspace.ReadCatalogEditor(saved.Path)
	if err != nil || again.Revision != saved.Revision {
		t.Fatalf("invalid save modified file: %v", err)
	}

	choices, err := loadTypeChoices(workspace.root)
	if err != nil {
		t.Fatal(err)
	}
	if len(choices.Catalogs) != 1 || choices.Catalogs[0].ID != created.Catalog.ID {
		t.Fatalf("type choices did not include the new catalog: %+v", choices)
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogAutoGrantsDefaultAccessToRoles(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}

	// A role that opts in to auto-granting new objects.
	granting, err := workspace.CreateRole("Автопривилегированная")
	if err != nil {
		t.Fatal(err)
	}
	granting.Role.GrantNewObjectsByDefault = true
	granting.Role.GrantNewFieldsByDefault = true
	granting, err = workspace.SaveRole(granting.Path, granting.Role, granting.Revision)
	if err != nil {
		t.Fatal(err)
	}

	// A role that does not opt in - must stay untouched throughout.
	plain, err := workspace.CreateRole("Обычная")
	if err != nil {
		t.Fatal(err)
	}

	created, err := workspace.CreateCatalog("Товары")
	if err != nil {
		t.Fatal(err)
	}

	grantingAfterCreate, err := workspace.ReadRole(granting.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(grantingAfterCreate.Role.Objects) != 1 || grantingAfterCreate.Role.Objects[0].Object != created.Catalog.ID ||
		!slices.Contains(grantingAfterCreate.Role.Objects[0].Operations, metadata.PermissionRead) {
		t.Fatalf("opted-in role did not gain default read access: %+v", grantingAfterCreate.Role.Objects)
	}
	plainAfterCreate, err := workspace.ReadRole(plain.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(plainAfterCreate.Role.Objects) != 0 {
		t.Fatalf("role without opt-in must stay untouched: %+v", plainAfterCreate.Role.Objects)
	}

	// Add a new attribute: the opted-in role (which already has the object)
	// gains default field access; the plain role has no object entry at all
	// and must not gain field access in isolation, even though it exists.
	updated := created.Catalog
	attributeID := uuid.MustNew()
	updated.Attributes = []metadata.Attribute{{
		ID: attributeID, Name: "Артикул", Title: metadata.LocalizedText{"ru": "Артикул"},
		Types: []metadata.Type{{Kind: metadata.StringType, Length: 50}},
	}}
	if _, err := workspace.SaveCatalogEditor(created.Path, updated, created.Revision); err != nil {
		t.Fatal(err)
	}

	grantingAfterField, err := workspace.ReadRole(granting.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(grantingAfterField.Role.Objects[0].Fields) != 1 || grantingAfterField.Role.Objects[0].Fields[0].Field != attributeID.String() {
		t.Fatalf("opted-in role did not gain default field access: %+v", grantingAfterField.Role.Objects[0].Fields)
	}
	plainAfterField, err := workspace.ReadRole(plain.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(plainAfterField.Role.Objects) != 0 {
		t.Fatalf("role without any object access must not gain a field grant: %+v", plainAfterField.Role.Objects)
	}
}

func TestCatalogEditorRoutesValidateMutations(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(workspace)
	for _, test := range []struct {
		body, contentType, csrf string
		status                  int
	}{
		{`{"name":"Товары"}`, "application/json", "", http.StatusForbidden},
		{`{"name":"Товары"}`, "text/plain", "1", http.StatusUnsupportedMediaType},
		{`{"name":"Товары","admin":true}`, "application/json", "1", http.StatusBadRequest},
		{`{"name":"Товары"} {}`, "application/json", "1", http.StatusBadRequest},
		{`{"name":"Товары"}`, "application/json", "1", http.StatusOK},
	} {
		request := httptest.NewRequest(http.MethodPost, "http://localhost/api/catalog", strings.NewReader(test.body))
		request.Header.Set("Content-Type", test.contentType)
		request.Header.Set("X-ML-CSRF", test.csrf)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("POST catalog: %d %s", response.Code, response.Body.String())
		}
	}
}

func TestCatalogEditorUI(t *testing.T) {
	runNodeTest(t, "catalog-editor.test.mjs")
}

func TestDeleteCatalogRemovesFolderAndChecksRevision(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	created, err := workspace.CreateCatalog("Товары")
	if err != nil {
		t.Fatal(err)
	}
	objectDirectory := filepath.Join(workspace.root, "metadata", "catalogs", created.Catalog.Name)
	if _, err := os.Stat(objectDirectory); err != nil {
		t.Fatalf("catalog folder missing before delete: %v", err)
	}
	if err := workspace.DeleteCatalog(created.Path, "stale-revision"); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	if _, err := os.Stat(objectDirectory); err != nil {
		t.Fatalf("catalog folder removed despite stale revision: %v", err)
	}
	if err := workspace.DeleteCatalog(created.Path, created.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(objectDirectory); !os.IsNotExist(err) {
		t.Fatalf("catalog folder still present after delete: %v", err)
	}
	if err := workspace.DeleteCatalog(created.Path, created.Revision); !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("deleting an already-deleted catalog: %v", err)
	}
}

func TestDeleteCatalogRouteRequiresCSRF(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	created, err := workspace.CreateCatalog("Товары")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(workspace)
	query := "?path=" + created.Path + "&expectedRevision=" + created.Revision
	forbidden := httptest.NewRequest(http.MethodDelete, "http://localhost/api/catalog"+query, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, forbidden)
	if response.Code != http.StatusForbidden {
		t.Fatalf("DELETE catalog without CSRF: %d %s", response.Code, response.Body.String())
	}
	allowed := httptest.NewRequest(http.MethodDelete, "http://localhost/api/catalog"+query, nil)
	allowed.Header.Set("X-ML-CSRF", "1")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, allowed)
	if response.Code != http.StatusNoContent {
		t.Fatalf("DELETE catalog: %d %s", response.Code, response.Body.String())
	}
}

func TestMetadataTreeOrdersObjectsAndAttributesAlphabetically(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// Create in an order that would NOT be alphabetical if the tree simply
	// preserved creation (UUID directory) order.
	yablonki, err := workspace.CreateCatalog("Яблоки")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.CreateCatalog("Абрикосы"); err != nil {
		t.Fatal(err)
	}

	updated := yablonki.Catalog
	updated.Attributes = []metadata.Attribute{
		{ID: uuid.MustNew(), Name: "Яркость", Title: metadata.LocalizedText{"ru": "Яркость"}, Types: []metadata.Type{{Kind: metadata.StringType, Length: 10}}},
		{ID: uuid.MustNew(), Name: "Артикул", Title: metadata.LocalizedText{"ru": "Артикул"}, Types: []metadata.Type{{Kind: metadata.StringType, Length: 10}}},
	}
	if _, err := workspace.SaveCatalogEditor(yablonki.Path, updated, yablonki.Revision); err != nil {
		t.Fatal(err)
	}

	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	catalogsNode, ok := findNodeByID(snapshot.Tree, "metadata/catalogs")
	if !ok || len(catalogsNode.Children) != 2 {
		t.Fatalf("catalogs node: %+v (ok=%v)", catalogsNode, ok)
	}
	if catalogsNode.Children[0].Title != "Абрикосы" || catalogsNode.Children[1].Title != "Яблоки" {
		t.Fatalf("catalogs are not alphabetically ordered: %q, %q", catalogsNode.Children[0].Title, catalogsNode.Children[1].Title)
	}
	yablonkiNode, ok := findNodeByID(snapshot.Tree, yablonki.Catalog.ID.String())
	if !ok {
		t.Fatalf("Яблоки node not found")
	}
	attributesNode, ok := findNodeByID(yablonkiNode, yablonki.Catalog.ID.String()+":attributes")
	if !ok || len(attributesNode.Children) != 2 {
		t.Fatalf("attributes group: %+v (ok=%v)", attributesNode, ok)
	}
	if attributesNode.Children[0].Title != "Артикул" || attributesNode.Children[1].Title != "Яркость" {
		t.Fatalf("attributes are not alphabetically ordered: %q, %q", attributesNode.Children[0].Title, attributesNode.Children[1].Title)
	}
}

func TestMetadataTreeExposesCatalogAttributesAndTableParts(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	created, err := workspace.CreateCatalog("Товары")
	if err != nil {
		t.Fatal(err)
	}
	attributeID, tablePartID := uuid.MustNew(), uuid.MustNew()
	updated := created.Catalog
	updated.Attributes = []metadata.Attribute{{
		ID: attributeID, Name: "Артикул", Title: metadata.LocalizedText{"ru": "Артикул"},
		Types: []metadata.Type{{Kind: metadata.StringType, Length: 50}},
	}}
	updated.TableParts = []metadata.TablePart{{
		ID: tablePartID, Name: "Партии", Title: metadata.LocalizedText{"ru": "Партии"},
		Attributes: []metadata.Attribute{{ID: uuid.MustNew(), Name: "Количество", Title: metadata.LocalizedText{"ru": "Количество"}, Types: []metadata.Type{{Kind: metadata.NumberType, Precision: 15, Scale: 3}}}},
	}}
	if _, err := workspace.SaveCatalogEditor(created.Path, updated, created.Revision); err != nil {
		t.Fatal(err)
	}

	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !treeContainsTitle(snapshot.Tree, "Реквизиты") || !treeContainsTitle(snapshot.Tree, "Табличные части") {
		t.Fatalf("tree does not group catalog attributes/table parts: %+v", snapshot.Tree)
	}
	attribute, ok := findNodeByFragment(snapshot.Tree, "attribute:"+attributeID.String())
	if !ok || attribute.Title != "Артикул" || attribute.Path != created.Path {
		t.Fatalf("attribute node missing or wrong: %+v (ok=%v)", attribute, ok)
	}
	tablePart, ok := findNodeByFragment(snapshot.Tree, "tablepart:"+tablePartID.String())
	if !ok || tablePart.Title != "Партии" || tablePart.Path != created.Path {
		t.Fatalf("table part node missing or wrong: %+v (ok=%v)", tablePart, ok)
	}
	// The "Реквизиты"/"Табличные части" GROUP nodes themselves must also
	// carry the catalog's Path — the tree-toolbar's context detection (is
	// this position "inside catalog X, on its attributes") relies on it,
	// not just on their leaf children.
	catalogNode, ok := findNodeByID(snapshot.Tree, created.Catalog.ID.String())
	if !ok {
		t.Fatalf("catalog node not found")
	}
	attributesGroup, ok := findNodeByID(catalogNode, created.Catalog.ID.String()+":attributes")
	if !ok || attributesGroup.Path != created.Path {
		t.Fatalf("attributes group node missing its own Path: %+v (ok=%v)", attributesGroup, ok)
	}
	tablePartsGroup, ok := findNodeByID(catalogNode, created.Catalog.ID.String()+":table-parts")
	if !ok || tablePartsGroup.Path != created.Path {
		t.Fatalf("table-parts group node missing its own Path: %+v (ok=%v)", tablePartsGroup, ok)
	}
}

func findNodeByFragment(node Node, fragment string) (Node, bool) {
	if node.Fragment == fragment {
		return node, true
	}
	for _, child := range node.Children {
		if found, ok := findNodeByFragment(child, fragment); ok {
			return found, true
		}
	}
	return Node{}, false
}

func findNodeByID(node Node, id string) (Node, bool) {
	if node.ID == id {
		return node, true
	}
	for _, child := range node.Children {
		if found, ok := findNodeByID(child, id); ok {
			return found, true
		}
	}
	return Node{}, false
}

func TestMetadataTreeAlwaysShowsFixedObjectGroupsEvenWhenEmpty(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	created, err := workspace.CreateCatalog("Пустой")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	catalogNode, ok := findNodeByID(snapshot.Tree, created.Catalog.ID.String())
	if !ok {
		t.Fatalf("catalog node not found: %+v", snapshot.Tree)
	}
	titles := make([]string, 0, len(catalogNode.Children))
	for _, child := range catalogNode.Children {
		titles = append(titles, child.Title)
	}
	for _, want := range []string{"Реквизиты", "Табличные части", "Формы", "Команды", "Макеты"} {
		found := false
		for _, title := range titles {
			if title == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("empty catalog is missing always-visible group %q: children=%v", want, titles)
		}
	}
}
