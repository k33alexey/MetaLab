package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A command placed in a group that is gone is noted, wherever the command
// lives: a common command, and every command of every object - not only those
// of the first one looked at.
func TestEveryCommandInAGroupThatIsGoneIsNoted(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	gone := []string{uuid.MustNew().String(), uuid.MustNew().String(), uuid.MustNew().String()}
	writeCommonCommand(t, root, "Обмен", "format: 1\nid: "+uuid.MustNew().String()+"\nname: Обмен\ntitle: {ru: Обмен}\ngroup_ref: "+gone[0]+"\n", true)
	for index, name := range []string{"Банки", "Склады"} {
		id := uuid.MustNew().String()
		writeMetadata(t, root, CatalogKind, id, "format: 1\nid: "+id+"\nname: "+name+"\ntitle: {ru: "+name+"}\n"+
			"code: {type: string, length: 9, auto: true}\ndescription_length: 150\n"+
			"commands:\n  - {id: "+uuid.MustNew().String()+", name: Подбор, title: {ru: Подбор}, group_ref: "+gone[index+1]+"}\n")
		writeCommandModule(t, root, CatalogKind, name, "Подбор")
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	noted := map[string]bool{}
	for _, item := range catalog.UnresolvedReferences() {
		noted[item.ID.String()] = true
	}
	for _, id := range gone {
		if !noted[id] {
			t.Fatalf("group %s is not noted; noted %v", id, catalog.UnresolvedReferences())
		}
	}
}

// The parameter of a common command is typed like any field, and a type
// naming nothing is refused - on the common command as on the commands of
// objects that come after it.
func TestACommonCommandParameterTypedByNothingIsRefused(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		common    string
		ownedType string
	}{
		{"on the common command", "parameter: [{kind: catalog, reference: %s}]\n", ""},
		{"on an object command after a sound common one", "", "parameter: [{kind: catalog, reference: %s}], "},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			missing := uuid.MustNew().String()
			fill := func(text string) string { return strings.ReplaceAll(text, "%s", missing) }
			writeCommonCommand(t, root, "Обмен", "format: 1\nid: "+uuid.MustNew().String()+"\nname: Обмен\ntitle: {ru: Обмен}\n"+fill(test.common), true)
			id := uuid.MustNew().String()
			writeMetadata(t, root, CatalogKind, id, "format: 1\nid: "+id+"\nname: Банки\ntitle: {ru: Банки}\n"+
				"code: {type: string, length: 9, auto: true}\ndescription_length: 150\n"+
				"commands:\n  - {id: "+uuid.MustNew().String()+", name: Подбор, title: {ru: Подбор}, "+fill(test.ownedType)+"parameter_use: single}\n")
			writeCommandModule(t, root, CatalogKind, "Банки", "Подбор")
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), "parameter references unknown catalog "+missing) {
				t.Fatalf("Load() error = %v", err)
			}
		})
	}
}
