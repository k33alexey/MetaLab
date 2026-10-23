package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A command stands in a group: «Проверка конфигурации» refuses one with none -
// «Не указана группа, в которую входит команда по умолчанию» (opis of
// 15.10.2026, komanda.md, on a command of a catalog; a common command is the
// same validateCommandShape). The text is written out here.
//
// Defect caught: a command of an object or a common command with no group
// loaded, drawn nowhere; one with a standard group or a group of the
// configuration refused.
func TestACommandNamesItsGroup(t *testing.T) {
	t.Parallel()
	const refusal = "names no group: the designer refuses a command with no group it stands in by default"
	common := func(place string) error {
		_, err := DecodeCommonCommand("object.yaml", strings.NewReader("format: 1\nid: "+uuid.MustNew().String()+
			"\nname: Открыть\ntitle: {ru: Открыть}\n"+place), metadataConfiguration())
		return err
	}
	object := func(place string) error {
		_, err := DecodeCatalog("object.yaml", strings.NewReader("format: 1\nid: "+uuid.MustNew().String()+
			"\nname: Контрагенты\ntitle: {ru: Контрагенты}\ncode: {type: string, length: 9}\ndescription_length: 150\n"+
			"commands:\n  - {id: "+uuid.MustNew().String()+", name: Открыть, title: {ru: Открыть}"+place+"}\n"), metadataConfiguration())
		return err
	}
	if err := common(""); err == nil || !strings.Contains(err.Error(), "command "+refusal) {
		t.Errorf("a common command with no group: %v", err)
	}
	if err := object(""); err == nil || !strings.Contains(err.Error(), "commands[0] "+refusal) {
		t.Errorf("a command of an object with no group: %v", err)
	}
	for _, place := range []string{"group: actions-panel-tools", "group_ref: " + uuid.MustNew().String()} {
		if err := common(place + "\n"); err != nil {
			t.Errorf("a common command in %s was refused: %v", place, err)
		}
		if err := object(", " + place); err != nil {
			t.Errorf("a command of an object in %s was refused: %v", place, err)
		}
	}
}
