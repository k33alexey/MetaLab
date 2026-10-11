package studio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Studio saves a description only if the load would read it, and the load
// reads one only if Studio would save it: both read through one function. Each
// case is a defect the two copies had before they were one:
//   - a numerator: Studio did not take its identifier out of the file and
//     refused every one ("metadata UUID 00000000-...");
//   - a scheduled job: Studio knew 30 kinds of the loader's 42 and saved any
//     other description as long as it was YAML, which the load then refused;
//   - a constant and a form whose folder differs from the name in case alone:
//     the load compared folded case, so on a file system that ignores case it
//     read an object under a name nobody wrote; the folder is made from the
//     name, so they agree exactly.
func TestStudioSavesWhatTheLoaderReads(t *testing.T) {
	t.Parallel()
	numerator, job, constant, catalog, form := uuid.MustNew().String(), uuid.MustNew().String(),
		uuid.MustNew().String(), uuid.MustNew().String(), uuid.MustNew().String()
	for name, test := range map[string]struct {
		relative string
		body     string
		extra    map[string]string
		refusal  string
	}{
		"нумератор": {
			relative: "metadata/document-numerators/" + numerator + ".yaml",
			body: "format: 1\nid: " + numerator + "\nname: Сквозной\ntitle: {ru: Сквозной}\n" +
				"number: {type: string, length: 11, periodicity: year}\n",
		},
		"регламентное задание с чужим свойством": {
			relative: "metadata/scheduled-jobs/" + job + ".yaml",
			body:     "format: 1\nid: " + job + "\nname: Обмен\ntitle: {ru: Обмен}\nunknown_property: true\n",
			refusal:  "unknown_property",
		},
		"константа в папке другого регистра": {
			relative: "metadata/constants/Режим/object.yaml",
			body:     "format: 1\nid: " + constant + "\nname: режим\ntitle: {ru: Режим}\ntypes: [{kind: boolean}]\n",
			refusal:  "object режим lies in a folder called Режим",
		},
		"форма в папке другого регистра": {
			relative: "metadata/catalogs/Товары/forms/ФормаСписка/form.yaml",
			body:     "format: 1\nid: " + form + "\nname: формаСписка\ntitle: {ru: Список}\nkind: list\n",
			extra: map[string]string{"metadata/catalogs/Товары/object.yaml": "format: 1\nid: " + catalog +
				"\nname: Товары\ntitle: {ru: Товары}\ncode: {type: string, length: 9}\ndescription_length: 25\nforms: {list: ФормаСписка}\n"},
			refusal: "form формаСписка lies in a folder called ФормаСписка",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := createProject(t)
			files := map[string]string{test.relative: test.body}
			for relative, body := range test.extra {
				files[relative] = body
			}
			for relative, body := range files {
				absolute := filepath.Join(root, filepath.FromSlash(relative))
				if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(absolute, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			_, loadErr := metadata.Load(root)
			workspace, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			opened, err := workspace.ReadSource(test.relative)
			if err != nil {
				t.Fatal(err)
			}
			_, saveErr := workspace.SaveSource(test.relative, test.body, opened.Revision)
			for what, err := range map[string]error{"load": loadErr, "save": saveErr} {
				if test.refusal == "" && err != nil {
					t.Fatalf("the %s refused: %v", what, err)
				}
				if test.refusal != "" && (err == nil || !strings.Contains(err.Error(), test.refusal)) {
					t.Fatalf("the %s error = %v, want %q", what, err, test.refusal)
				}
			}
		})
	}
}
