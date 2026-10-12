package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func roleWorkspace(t *testing.T) (*Workspace, metadata.CatalogDefinition) {
	t.Helper()
	root := createProject(t)
	object := metadata.CatalogDefinition{Format: metadata.CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: metadata.LocalizedText{"ru": "Товары"}, Code: metadata.CatalogCode{Type: metadata.StringType, Length: 9}, DescriptionLength: 150,
		Attributes: []metadata.Attribute{{ID: uuid.MustNew(), Name: "Цена", Title: metadata.LocalizedText{"ru": "Цена"}, Types: []metadata.Type{{Kind: metadata.NumberType, Precision: 12, Scale: 2}}}}}
	var content bytes.Buffer
	if err := metadata.Encode(&content, object); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "metadata", "catalogs", object.Name)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "object.yaml"), content.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return workspace, object
}

func TestRoleEditorCreateReadSaveAndConflicts(t *testing.T) {
	t.Parallel()
	workspace, object := roleWorkspace(t)
	created, err := workspace.CreateRole("Продавец")
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Role.Objects) != 0 || len(created.Role.Commands) != 0 || len(created.Schema.Objects) != 2 {
		t.Fatalf("unsafe new role: %+v", created)
	}
	encoded, _ := json.Marshal(created)
	if !bytes.Contains(encoded, []byte(`"code":"ru"`)) {
		t.Fatalf("language JSON mismatch: %s", encoded)
	}
	if _, err := workspace.CreateRole("ПРОДАВЕЦ"); err == nil {
		t.Fatal("duplicate role name accepted")
	}
	if _, err := workspace.CreateRole("../Роль"); err == nil {
		t.Fatal("invalid name accepted")
	}
	if _, err := workspace.ReadRole("configuration.yaml"); !errors.Is(err, ErrInvalidSourcePath) {
		t.Fatalf("non-role path: %v", err)
	}
	created.Role.Objects = []metadata.ObjectPermission{{Object: object.ID, Operations: []metadata.PermissionOperation{metadata.PermissionRead}, Fields: []metadata.FieldPermission{{Field: "description", Operations: []metadata.PermissionOperation{metadata.PermissionView}}}}}
	saved, err := workspace.SaveRole(created.Path, created.Role, created.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision == created.Revision {
		t.Fatal("revision did not change")
	}
	if _, err := workspace.SaveRole(created.Path, created.Role, created.Revision); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("stale save: %v", err)
	}
	invalid := saved.Role
	invalid.ID = uuid.MustNew()
	if _, err := workspace.SaveRole(saved.Path, invalid, saved.Revision); err == nil {
		t.Fatal("identity change accepted")
	}
	invalid = saved.Role
	invalid.Objects = []metadata.ObjectPermission{{Object: object.ID, Operations: []metadata.PermissionOperation{"admin"}}}
	if _, err := workspace.SaveRole(saved.Path, invalid, saved.Revision); err == nil {
		t.Fatal("a right that is not one of the prototype's was accepted")
	}
	again, err := workspace.ReadRole(saved.Path)
	if err != nil || again.Revision != saved.Revision {
		t.Fatalf("invalid save modified file: %v", err)
	}
	// Posting on a catalog is a right the object has no use for, and the
	// prototype keeps such rights: it is carried, not refused.
	carried := again.Role
	carried.Objects = []metadata.ObjectPermission{{Object: object.ID, Operations: []metadata.PermissionOperation{metadata.PermissionPosting},
		Denied: []metadata.PermissionOperation{metadata.PermissionInteractiveDelete}}}
	stored, err := workspace.SaveRole(saved.Path, carried, again.Revision)
	if err != nil {
		t.Fatalf("a right the object does not use was refused: %v", err)
	}
	if reread, err := workspace.ReadRole(stored.Path); err != nil || !slices.Equal(reread.Role.Objects[0].Denied, carried.Objects[0].Denied) ||
		!slices.Equal(reread.Role.Objects[0].Operations, carried.Objects[0].Operations) {
		t.Fatalf("the right or the denial did not survive: %+v, %v", reread.Role.Objects, err)
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Fatal(err)
	}
}

// TestStudioOpensAProjectWhoseRoleKeepsItsRights catches Studio refusing the
// folder a role keeps its grid of rights in: since rights.json, a project
// with one saved role could no longer be opened, the tree failing with
// «unexpected source path». The folder is the loader's to check, so a file
// it does not allow there is still refused.
func TestStudioOpensAProjectWhoseRoleKeepsItsRights(t *testing.T) {
	t.Parallel()
	workspace, object := roleWorkspace(t)
	created, err := workspace.CreateRole("Продавец")
	if err != nil {
		t.Fatal(err)
	}
	created.Role.Objects = []metadata.ObjectPermission{{Object: object.ID, Operations: []metadata.PermissionOperation{metadata.PermissionRead}}}
	if _, err := workspace.SaveRole(created.Path, created.Role, created.Revision); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(workspace.root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := reopened.Snapshot()
	if err != nil {
		t.Fatalf("the project with a saved role does not open: %v", err)
	}
	if !treeContainsTitle(snapshot.Tree, "Продавец") {
		t.Fatal("the role is not in the tree")
	}
	stray := filepath.Join(workspace.root, "metadata", "roles", created.Role.ID.String(), "notes.txt")
	if err := os.WriteFile(stray, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Snapshot(); err == nil || !strings.Contains(err.Error(), "notes.txt") {
		t.Fatalf("a file a role's folder does not keep was taken: %v", err)
	}
}

// TestARoleIsComparedByItsOwnRevision catches Studio taking a role with a
// grid of rights for one changed outside it on every check: it compared the
// revision of the description file with the role's, which counts the grid
// too, so the editor reloaded every few seconds and blocked saving an edit
// as a conflict. The role's route answers «not modified» to the role's own
// revision, and answers again once the grid changes on disk.
func TestARoleIsComparedByItsOwnRevision(t *testing.T) {
	t.Parallel()
	workspace, object := roleWorkspace(t)
	created, err := workspace.CreateRole("Продавец")
	if err != nil {
		t.Fatal(err)
	}
	created.Role.Objects = []metadata.ObjectPermission{{Object: object.ID, Operations: []metadata.PermissionOperation{metadata.PermissionRead}}}
	saved, err := workspace.SaveRole(created.Path, created.Role, created.Revision)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(workspace)
	ask := func(revision string) int {
		r := httptest.NewRequest(http.MethodGet, "http://localhost/api/role?path="+saved.Path, nil)
		r.Header.Set("If-None-Match", `"`+revision+`"`)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}
	if code := ask(saved.Revision); code != http.StatusNotModified {
		t.Fatalf("an unchanged role with rights is answered %d", code)
	}
	grid := filepath.Join(workspace.root, "metadata", "roles", saved.Role.ID.String(), metadata.RoleRightsFile)
	content, err := os.ReadFile(grid)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(grid, append(content, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := ask(saved.Revision); code != http.StatusOK {
		t.Fatalf("a role whose rights changed on disk is answered %d", code)
	}
}

// Restrictions must survive a real save/read cycle, and a restriction the
// project cannot resolve must be refused before it reaches the working copy.
// A role carries three flags of the prototype, and the third - independent
// rights of child objects - decides whether a right on an attribute, a table
// part or a command needs the same right on its object (help, the role
// editor; erp sets it in 2 roles). Defect caught: the flag was not in the
// model, so it was lost at import and could not be set in the editor.
func TestRoleKeepsItsThreeFlags(t *testing.T) {
	t.Parallel()
	workspace, _ := roleWorkspace(t)
	created, err := workspace.CreateRole("Ответственный")
	if err != nil {
		t.Fatal(err)
	}
	if created.Role.IndependentChildRights || created.Role.GrantNewObjectsByDefault || created.Role.GrantNewFieldsByDefault {
		t.Fatalf("a new role was given a flag nobody set: %+v", created.Role)
	}
	// Through the routes the editor uses, under the names the editor writes:
	// the flag has to cross JSON both ways, not only YAML.
	handler := NewHandler(workspace)
	body, err := json.Marshal(map[string]any{"path": created.Path, "expectedRevision": created.Revision,
		"role": map[string]any{"format": created.Role.Format, "id": created.Role.ID, "name": created.Role.Name, "title": created.Role.Title,
			"independentChildRights": true, "grantNewFieldsByDefault": true}})
	if err != nil {
		t.Fatal(err)
	}
	put := httptest.NewRequest(http.MethodPut, "http://localhost/api/role", bytes.NewReader(body))
	put.Header.Set("Content-Type", "application/json")
	put.Header.Set("X-ML-CSRF", "1")
	saved := httptest.NewRecorder()
	handler.ServeHTTP(saved, put)
	if saved.Code != http.StatusOK {
		t.Fatalf("PUT role: %d %s", saved.Code, saved.Body.String())
	}
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "http://localhost/api/role?path="+url.QueryEscape(created.Path), nil))
	if !strings.Contains(get.Body.String(), `"independentChildRights":true`) {
		t.Fatalf("the editor is not given the flag back: %s", get.Body.String())
	}
	catalog, err := metadata.Load(workspace.root)
	if err != nil {
		t.Fatal(err)
	}
	role, ok := catalog.Role("Ответственный")
	if !ok || !role.IndependentChildRights || !role.GrantNewFieldsByDefault || role.GrantNewObjectsByDefault {
		t.Fatalf("the flags did not survive saving and loading: %+v", role)
	}
}

// A role's restrictions and templates are text, as the prototype keeps them,
// and they are saved and loaded as written. Defects caught: a restriction or
// a template lost on the way, and one the prototype cannot have accepted -
// a field of another object, a right the role does not write, a second
// restriction where only reading takes several, fields on a right other than
// reading, a template under a name that is not one or under one taken.
func TestRoleEditorSavesRestrictions(t *testing.T) {
	t.Parallel()
	workspace, object := roleWorkspace(t)
	created, err := workspace.CreateRole("Кладовщик")
	if err != nil {
		t.Fatal(err)
	}
	attribute := object.Attributes[0].ID.String()
	templates := []metadata.RestrictionTemplate{{Name: "ПоЦене(Поле)", Condition: "ГДЕ #Параметр(1) В (10, 20)"}}
	restricted := func() []metadata.ObjectPermission {
		return []metadata.ObjectPermission{{Object: object.ID,
			Operations: []metadata.PermissionOperation{metadata.PermissionRead, metadata.PermissionUpdate},
			Restrictions: []metadata.RightRestriction{
				{Right: metadata.PermissionRead, Fields: []string{"ref", "dataversion"}},
				{Right: metadata.PermissionRead, Condition: `#ПоЦене("Цена")`},
				{Right: metadata.PermissionUpdate, Condition: "ГДЕ Цена > 0"}}}}
	}
	created.Role.RestrictionTemplates, created.Role.Objects = templates, restricted()
	saved, err := workspace.SaveRole(created.Path, created.Role, created.Revision)
	if err != nil {
		t.Fatal(err)
	}
	again, err := workspace.ReadRole(saved.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again.Role.RestrictionTemplates, templates) || !reflect.DeepEqual(again.Role.Objects[0].Restrictions, restricted()[0].Restrictions) {
		t.Fatalf("restrictions did not round-trip: %+v %+v", again.Role.RestrictionTemplates, again.Role.Objects)
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Fatalf("saved role does not load as a project: %v", err)
	}

	broken := map[string]func(*metadata.RoleDefinition){
		"field of another object": func(r *metadata.RoleDefinition) { r.Objects[0].Restrictions[0].Fields[0] = uuid.MustNew().String() },
		"right not written":       func(r *metadata.RoleDefinition) { r.Objects[0].Restrictions[2].Right = metadata.PermissionDelete },
		"right not restricted": func(r *metadata.RoleDefinition) {
			r.Objects[0].Operations = append(r.Objects[0].Operations, metadata.PermissionView)
			r.Objects[0].Restrictions[2].Right = metadata.PermissionView
		},
		"second restriction of a right other than reading": func(r *metadata.RoleDefinition) {
			r.Objects[0].Restrictions = append(r.Objects[0].Restrictions, metadata.RightRestriction{Right: metadata.PermissionUpdate, Condition: "ГДЕ ЛОЖЬ"})
		},
		"fields on a right other than reading": func(r *metadata.RoleDefinition) { r.Objects[0].Restrictions[2].Fields = []string{attribute} },
		"two restrictions on the other fields": func(r *metadata.RoleDefinition) { r.Objects[0].Restrictions[0].Fields = nil },
		"a field restricted twice": func(r *metadata.RoleDefinition) {
			r.Objects[0].Restrictions[1].Fields = []string{"ref"}
		},
		"template name that is not one": func(r *metadata.RoleDefinition) { r.RestrictionTemplates[0].Name = "По Цене(Поле)" },
		"template name taken": func(r *metadata.RoleDefinition) {
			r.RestrictionTemplates = append(r.RestrictionTemplates, metadata.RestrictionTemplate{Name: "поцене", Condition: "ГДЕ ИСТИНА"})
		},
	}
	for name, mutate := range broken {
		invalid := again.Role
		invalid.RestrictionTemplates = slices.Clone(templates)
		invalid.Objects = restricted()
		mutate(&invalid)
		if _, err := workspace.SaveRole(again.Path, invalid, again.Revision); err == nil {
			t.Fatalf("accepted a role with %s", name)
		}
	}
	unchanged, err := workspace.ReadRole(saved.Path)
	if err != nil || unchanged.Revision != again.Revision {
		t.Fatalf("refused save still modified the file: %v", err)
	}
}

func TestRoleEditorRepairsDanglingReferences(t *testing.T) {
	t.Parallel()
	workspace, object := roleWorkspace(t)
	created, err := workspace.CreateRole("Читатель")
	if err != nil {
		t.Fatal(err)
	}
	created.Role.Objects = []metadata.ObjectPermission{{Object: object.ID, Operations: []metadata.PermissionOperation{metadata.PermissionRead}, Fields: []metadata.FieldPermission{{Field: uuid.MustNew().String(), Operations: []metadata.PermissionOperation{metadata.PermissionView}}}}}
	// Emulate an external edit/deleted attribute: runtime must reject it while
	// the editor must still open the role for repair.
	if err := metadata.WriteRole(workspace.root, created.Role); err != nil {
		t.Fatal(err)
	}
	if _, err := metadata.Load(workspace.root); err == nil {
		t.Fatal("runtime accepted dangling grant")
	}
	opened, err := workspace.ReadRole(created.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(opened.Role.Objects[0].Fields) != 1 {
		t.Fatal("editor silently removed a grant")
	}
	opened.Role.Objects[0].Fields = nil
	if _, err := workspace.SaveRole(opened.Path, opened.Role, opened.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Fatal(err)
	}
}

func TestRoleEditorRoutesValidateMutations(t *testing.T) {
	t.Parallel()
	workspace, _ := roleWorkspace(t)
	handler := NewHandler(workspace)
	for _, test := range []struct {
		body, contentType, csrf string
		status                  int
	}{
		{`{"name":"Роль"}`, "application/json", "", http.StatusForbidden},
		{`{"name":"Роль"}`, "text/plain", "1", http.StatusUnsupportedMediaType},
		{`{"name":"Роль","admin":true}`, "application/json", "1", http.StatusBadRequest},
		{`{"name":"Роль"} {}`, "application/json", "1", http.StatusBadRequest},
		{`{"name":"Роль"}`, "application/json", "1", http.StatusOK},
	} {
		r := httptest.NewRequest(http.MethodPost, "http://localhost/api/role", strings.NewReader(test.body))
		r.Header.Set("Content-Type", test.contentType)
		r.Header.Set("X-ML-CSRF", test.csrf)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatalf("POST role: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestDeleteRoleRemovesFileAndChecksRevision(t *testing.T) {
	t.Parallel()
	workspace, _ := roleWorkspace(t)
	created, err := workspace.CreateRole("Продавец")
	if err != nil {
		t.Fatal(err)
	}
	absolute := filepath.Join(workspace.root, filepath.FromSlash(created.Path))
	if _, err := os.Stat(absolute); err != nil {
		t.Fatalf("role file missing before delete: %v", err)
	}
	if err := workspace.DeleteRole(created.Path, "stale-revision"); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	if _, err := os.Stat(absolute); err != nil {
		t.Fatalf("role file removed despite stale revision: %v", err)
	}
	if err := workspace.DeleteRole(created.Path, created.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(absolute); !os.IsNotExist(err) {
		t.Fatalf("role file still present after delete: %v", err)
	}
	if err := workspace.DeleteRole(created.Path, created.Revision); !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("deleting an already-deleted role: %v", err)
	}
}

func TestRoleEditorUI(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for role editor tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, node, "--test", "../../scripts/role-editor.test.mjs").CombinedOutput()
	if err != nil {
		t.Fatalf("role editor tests: %v\n%s", err, output)
	}
}

func TestReadAllRolesReturnsEveryRoleOrderedAndWithSchema(t *testing.T) {
	t.Parallel()
	workspace, object := roleWorkspace(t)
	for _, name := range []string{"Продавец", "Кладовщик", "Администратор"} {
		if _, err := workspace.CreateRole(name); err != nil {
			t.Fatal(err)
		}
	}
	seller, err := workspace.ReadRole(overviewPathOf(t, workspace, "Продавец"))
	if err != nil {
		t.Fatal(err)
	}
	seller.Role.Objects = []metadata.ObjectPermission{{Object: object.ID, Operations: []metadata.PermissionOperation{metadata.PermissionRead},
		Restrictions: []metadata.RightRestriction{{Right: metadata.PermissionRead, Condition: "ГДЕ Цена = 10"}}}}
	if _, err := workspace.SaveRole(seller.Path, seller.Role, seller.Revision); err != nil {
		t.Fatal(err)
	}
	overview, err := workspace.ReadAllRoles()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range overview.Roles {
		names = append(names, entry.Role.Name)
	}
	if want := []string{"Администратор", "Кладовщик", "Продавец"}; !slices.Equal(names, want) {
		t.Fatalf("roles are not listed in name order: %v", names)
	}
	if len(overview.Schema.Objects) != 2 || len(overview.Languages) == 0 || overview.DefaultLanguage == "" {
		t.Fatalf("overview is missing what the screen renders: %+v", overview.Schema)
	}
	for _, entry := range overview.Roles {
		if entry.Role.Name != "Продавец" {
			continue
		}
		if restrictions := entry.Role.Objects[0].Restrictions; len(restrictions) != 1 || restrictions[0].Condition != "ГДЕ Цена = 10" {
			t.Fatalf("restriction lost on the way to the screen: %+v", entry.Role.Objects)
		}
	}
}

// A role file that cannot be decoded must fail the whole screen: showing the
// other roles as if they were all of them would understate who has access.
func TestReadAllRolesRefusesToHideAnUnreadableRole(t *testing.T) {
	t.Parallel()
	workspace, _ := roleWorkspace(t)
	created, err := workspace.CreateRole("Продавец")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace.root, filepath.FromSlash(created.Path)), []byte("format: 1\nname: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.ReadAllRoles(); err == nil {
		t.Fatal("an unreadable role was silently omitted")
	}
	request := httptest.NewRequest(http.MethodGet, "http://localhost/api/roles", nil)
	response := httptest.NewRecorder()
	NewHandler(workspace).ServeHTTP(response, request)
	if response.Code == http.StatusOK {
		t.Fatalf("the route reported success for an unreadable project: %s", response.Body.String())
	}
}

func TestRoleOverviewRouteServesEveryRole(t *testing.T) {
	t.Parallel()
	workspace, _ := roleWorkspace(t)
	if _, err := workspace.CreateRole("Продавец"); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "http://localhost/api/roles", nil)
	response := httptest.NewRecorder()
	NewHandler(workspace).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET roles: %d %s", response.Code, response.Body.String())
	}
	var overview RoleOverview
	if err := json.Unmarshal(response.Body.Bytes(), &overview); err != nil {
		t.Fatal(err)
	}
	if len(overview.Roles) != 1 || overview.Roles[0].Role.Name != "Продавец" || overview.Roles[0].Path == "" {
		t.Fatalf("unexpected overview: %s", response.Body.String())
	}
}

func overviewPathOf(t *testing.T, workspace *Workspace, name string) string {
	t.Helper()
	overview, err := workspace.ReadAllRoles()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range overview.Roles {
		if entry.Role.Name == name {
			return entry.Path
		}
	}
	t.Fatalf("role %q not found", name)
	return ""
}

func TestRoleOverviewUI(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for role overview tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, node, "--test", "../../scripts/role-overview.test.mjs").CombinedOutput()
	if err != nil {
		t.Fatalf("role overview tests: %v\n%s", err, output)
	}
}
