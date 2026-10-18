package publication

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
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
	role := metadata.RoleDefinition{Format: metadata.CurrentFormat, ID: uuid.MustNew(), Name: "Читатель", Title: metadata.LocalizedText{"ru": "Читатель"},
		Objects: []metadata.ObjectPermission{{Object: constant.ID, Operations: []metadata.PermissionOperation{metadata.PermissionRead}}}}
	write := func(relative string, value any) {
		t.Helper()
		if role, ok := value.(metadata.RoleDefinition); ok {
			if err := metadata.WriteRole(root, role); err != nil {
				t.Fatal(err)
			}
			return
		}
		var encoded bytes.Buffer
		if err := metadata.Encode(&encoded, value); err != nil {
			t.Fatal(err)
		}
		writeSourceFile(t, root, relative, encoded.Bytes())
	}
	write("metadata/constants/"+constant.Name+"/"+project.ObjectMetadataFile, constant)
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

	// A role gives no right on a command of a form (ACCESS-RIGHTS, «Команды
	// формы»); one an earlier ML saved with such rights is not published.
	role.Commands = []metadata.CommandPermission{{Form: uuid.MustNew(), Command: uuid.MustNew()}}
	write(rolePath, role)
	if _, err := inspect(context.Background(), root, SourceState{}); err == nil {
		t.Fatal("inspect accepted a role giving a right on a command of a form")
	}
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(rolePath))); err != nil {
		t.Fatal(err)
	}
}
