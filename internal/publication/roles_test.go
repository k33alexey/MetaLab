package publication

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestPackagePublishesAndVerifiesRoles(t *testing.T) {
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
		absolute := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
			t.Fatal(err)
		}
		var encoded bytes.Buffer
		if err := metadata.Encode(&encoded, value); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, encoded.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("metadata/constants/"+constant.ID.String()+".yaml", constant)
	write("forms/"+form.ID.String()+".yaml", form)
	rolePath := "metadata/roles/" + role.ID.String() + ".yaml"
	write(rolePath, role)
	firstPath := filepath.Join(t.TempDir(), "first.mlpkg")
	first, err := BuildFile(context.Background(), root, firstPath, SourceState{Dirty: true})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyFile(context.Background(), firstPath)
	if err != nil || len(verified.Runtime.Roles) != 1 || !reflect.DeepEqual(first.Runtime.Roles, verified.Runtime.Roles) || verified.Runtime.Roles[0].ID != role.ID {
		t.Fatalf("published roles missing or changed: %v", err)
	}
	// A runtime grant injected into package.json must not override YAML sources.
	tampered := filepath.Join(t.TempDir(), "tampered.mlpkg")
	rewriteRoleSnapshot(t, firstPath, tampered)
	if _, err := VerifyFile(context.Background(), tampered); err == nil || !strings.Contains(err.Error(), "runtime metadata") {
		t.Fatalf("accepted tampered runtime grants: %v", err)
	}
	role.Objects[0].Operations = append(role.Objects[0].Operations, metadata.PermissionUpdate)
	write(rolePath, role)
	secondPath := filepath.Join(t.TempDir(), "second.mlpkg")
	second, err := BuildFile(context.Background(), root, secondPath, SourceState{Dirty: true})
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentSHA256 == second.ContentSHA256 || first.SchemaSHA256 != second.SchemaSHA256 {
		t.Fatal("role change must version the publication without changing the database schema")
	}
	if _, err := VerifyFile(context.Background(), secondPath); err != nil {
		t.Fatal(err)
	}
	// Subsequent source edits cannot change an already-built publication.
	role.Commands[0].Command = uuid.MustNew()
	write(rolePath, role)
	if _, err := BuildFile(context.Background(), root, filepath.Join(t.TempDir(), "broken.mlpkg"), SourceState{}); err == nil {
		t.Fatal("publication accepted a missing form command")
	}
	if _, err := VerifyFile(context.Background(), firstPath); err != nil {
		t.Fatalf("project edits affected the earlier publication: %v", err)
	}
}

func rewriteRoleSnapshot(t *testing.T, source, destination string) {
	t.Helper()
	reader, err := zip.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	file, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	for _, entry := range reader.File {
		input, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(input)
		_ = input.Close()
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name == "package.json" {
			var manifest Manifest
			if err := json.Unmarshal(content, &manifest); err != nil {
				t.Fatal(err)
			}
			manifest.Runtime.Roles[0].Objects[0].Operations = append(manifest.Runtime.Roles[0].Objects[0].Operations, metadata.PermissionUpdate)
			content, err = json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
		}
		output, err := writer.Create(entry.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := output.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}
