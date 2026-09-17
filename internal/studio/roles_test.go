package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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
	directory := filepath.Join(root, "metadata", "catalogs", object.ID.String())
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
	if len(created.Role.Objects) != 0 || len(created.Role.Commands) != 0 || len(created.Schema.Objects) != 1 {
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
	if _, err := workspace.ReadRole("mlproject.yaml"); !errors.Is(err, ErrInvalidSourcePath) {
		t.Fatalf("non-role path: %v", err)
	}
	created.Role.Objects = []metadata.ObjectPermission{{Object: object.ID, Operations: []metadata.PermissionOperation{metadata.PermissionRead}, Fields: []metadata.FieldPermission{{Field: "description", Operations: []metadata.PermissionOperation{metadata.PermissionRead}}}}}
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
	invalid.Objects = []metadata.ObjectPermission{{Object: object.ID, Operations: []metadata.PermissionOperation{metadata.PermissionPost}}}
	if _, err := workspace.SaveRole(saved.Path, invalid, saved.Revision); err == nil {
		t.Fatal("unsupported operation accepted")
	}
	again, err := workspace.ReadRole(saved.Path)
	if err != nil || again.Revision != saved.Revision {
		t.Fatalf("invalid save modified file: %v", err)
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Fatal(err)
	}
}

func TestRoleEditorRepairsDanglingReferences(t *testing.T) {
	t.Parallel()
	workspace, object := roleWorkspace(t)
	created, err := workspace.CreateRole("Читатель")
	if err != nil {
		t.Fatal(err)
	}
	created.Role.Objects = []metadata.ObjectPermission{{Object: object.ID, Operations: []metadata.PermissionOperation{metadata.PermissionRead}, Fields: []metadata.FieldPermission{{Field: uuid.MustNew().String(), Operations: []metadata.PermissionOperation{metadata.PermissionRead}}}}}
	var content bytes.Buffer
	if err := metadata.Encode(&content, created.Role); err != nil {
		t.Fatal(err)
	}
	// Emulate an external edit/deleted attribute: runtime must reject it while
	// the editor must still open the role for repair.
	if err := os.WriteFile(filepath.Join(workspace.root, created.Path), content.Bytes(), 0o600); err != nil {
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
