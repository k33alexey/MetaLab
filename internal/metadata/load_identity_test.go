package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A file of the project is edited by hand as often as by Studio, so what a
// file says it is has to agree with where it lies: a flat object is found by
// the identifier in its file name, an object with a folder by the folder's
// name. An object that disagrees would be read as one thing and found as
// another. Each kind decodes on its own path, so each one is tried.
func TestAnObjectThatDisagreesWithWhereItLiesIsRefused(t *testing.T) {
	t.Parallel()
	flat := []struct {
		kind Kind
		body string
	}{
		{CommonAttributeKind, "name: Организация\ntitle: {ru: Организация}\ntypes: [{kind: boolean}]\n"},
		{CommonModuleKind, "name: Общий\ntitle: {ru: Общий}\nserver: true\nmodule: " + uuid.MustNew().String() + "\n"},
		{CommandGroupKind, "name: Обмены\ntitle: {ru: Обмены}\ncategory: actions-panel\n"},
		{StyleItemKind, "name: ЦветФона\ntitle: {ru: ЦветФона}\ntype: color\nvalue:\n  color: {source: absolute, rgb: '#1C55AE'}\n"},
		{StyleKind, "name: Основной\ntitle: {ru: Основной}\n"},
		{FunctionalOptionKind, "name: Склады\ntitle: {ru: Склады}\nlocation: {kind: constants, object: " + uuid.MustNew().String() + "}\n"},
		{FunctionalOptionParameterKind, "name: Организация\ntitle: {ru: Организация}\nuse:\n  - {kind: catalogs, object: " + uuid.MustNew().String() + "}\n"},
		{NumeratorKind, "name: Сквозной\ntitle: {ru: Сквозной}\nnumber: {type: string, length: 11, periodicity: year}\n"},
	}
	for _, test := range flat {
		t.Run(string(test.kind), func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			file, inside := uuid.MustNew().String(), uuid.MustNew().String()
			writeMetadata(t, root, test.kind, file, "format: 1\nid: "+inside+"\n"+test.body)
			_, err := Load(root)
			// A numerator keeps a folder named by its identifier, the rest a
			// file; the message names which.
			if err == nil || !strings.Contains(err.Error(), "metadata UUID "+inside+" does not match") || !strings.Contains(err.Error(), "UUID "+file) {
				t.Fatalf("Load() error = %v", err)
			}
		})
	}

	folders := []struct {
		kind Kind
		body string
		want string
	}{
		{SettingsStorageKind, "title: {ru: Хранилище}\n", "object Хранилище lies in folder Другое"},
		{FilterCriterionKind, "title: {ru: Связанные}\ntypes: []\n", "object Хранилище lies in folder Другое"},
		{SequenceKind, "title: {ru: Движение}\n", "object Хранилище lies in folder Другое"},
		{CommonTemplateKind, "title: {ru: Макет}\nkind: text\n", "common template Хранилище lies in a folder called Другое"},
		{CommonPictureKind, "title: {ru: Картинка}\n", "common picture Хранилище lies in a folder called Другое"},
	}
	for _, test := range folders {
		t.Run(string(test.kind), func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			directory := filepath.Join(root, "metadata", string(test.kind), "Другое")
			if err := os.MkdirAll(directory, 0o755); err != nil {
				t.Fatal(err)
			}
			body := "format: 1\nid: " + uuid.MustNew().String() + "\nname: Хранилище\n" + test.body
			if err := os.WriteFile(filepath.Join(directory, project.ObjectMetadataFile), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v", err)
			}
		})
	}
}
