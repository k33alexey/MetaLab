package publication

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Rights are the part of a configuration that changes most often and touches
// no table: a role edit has to version what the database holds without
// claiming the schema itself moved, or every rights change would ask the
// owner to confirm a migration that isn't there.
func TestInspectCarriesRolesAndSeparatesThemFromTheSchema(t *testing.T) {
	t.Parallel()
	root := publicationProject(t)
	constant := metadata.Constant{Format: metadata.CurrentFormat, ID: uuid.MustNew(), Name: "Режим", Title: metadata.LocalizedText{"ru": "Режим"}, Types: []metadata.Type{{Kind: metadata.BooleanType}}}
	form := metadata.ManagedForm{Format: metadata.CurrentFormat, ID: uuid.MustNew(), Name: "Форма", Title: metadata.LocalizedText{"ru": "Форма"}, Kind: metadata.CommonForm,
		Commands: []metadata.ManagedFormCommand{{ID: uuid.MustNew(), Name: "Обновить", Title: metadata.LocalizedText{"ru": "Обновить"}, Action: metadata.FormCommandRefresh}}}
	role := metadata.RoleDefinition{Format: metadata.CurrentFormat, ID: uuid.MustNew(), Name: "Читатель", Title: metadata.LocalizedText{"ru": "Читатель"},
		Objects:  []metadata.ObjectPermission{{Object: constant.ID, Operations: []metadata.PermissionOperation{metadata.PermissionRead}, Fields: []metadata.FieldPermission{{Field: "value", Operations: []metadata.PermissionOperation{metadata.PermissionRead}}}}},
		Commands: []metadata.CommandPermission{{Form: form.ID, Command: form.Commands[0].ID}}}
	write := func(relative string, value any) {
		t.Helper()
		var encoded bytes.Buffer
		if err := metadata.Encode(&encoded, value); err != nil {
			t.Fatal(err)
		}
		writeSourceFile(t, root, relative, encoded.Bytes())
	}
	write("metadata/constants/"+constant.ID.String()+".yaml", constant)
	write("metadata/common-forms/"+form.ID.String()+".yaml", form)
	rolePath := "metadata/roles/" + role.ID.String() + ".yaml"
	write(rolePath, role)

	first, err := inspect(context.Background(), root, SourceState{Dirty: true})
	if err != nil || len(first.Runtime.Roles) != 1 || first.Runtime.Roles[0].ID != role.ID {
		t.Fatalf("roles = %+v error=%v", first.Runtime.Roles, err)
	}

	role.Objects[0].Operations = append(role.Objects[0].Operations, metadata.PermissionUpdate)
	write(rolePath, role)
	second, err := inspect(context.Background(), root, SourceState{Dirty: true})
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentSHA256 == second.ContentSHA256 || first.SchemaSHA256 != second.SchemaSHA256 {
		t.Fatal("a rights change must version the sources without moving the database schema")
	}

	role.Commands[0].Command = uuid.MustNew()
	write(rolePath, role)
	if _, err := inspect(context.Background(), root, SourceState{}); err == nil {
		t.Fatal("inspect accepted a role granting a command no form declares")
	}
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(rolePath))); err != nil {
		t.Fatal(err)
	}
}
