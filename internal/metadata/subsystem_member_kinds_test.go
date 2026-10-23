package metadata

import (
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A subsystem holds an object of every kind but a language and a WebSocket
// client, whose page «Подсистемы» of «Дополнительно» is empty (opis of 14 and
// 15.10.2026); a function of a source and a recalculation are not seen either
// way. The kinds are written here, not read from kindUsesOf; a kind saying
// nothing of it fails, so a new kind is not left out by omission.
//
// Defect caught: a kind the designer keeps out of a subsystem let in, or one
// it puts there refused or noted.
func TestASubsystemHoldsThePrototypesKinds(t *testing.T) {
	t.Parallel()
	refused := []Kind{LanguageKind, WebSocketClientKind}
	unseen := []Kind{ExternalFunctionKind, RecalculationKind}
	for _, item := range kinds.All() {
		got := usesOf(item.Kind).subsystem
		want := present
		switch {
		case slices.Contains(refused, item.Kind):
			want = absent
		case slices.Contains(unseen, item.Kind):
			want = unproven
		}
		switch {
		case got == unsaid:
			t.Errorf("%s says nothing of a subsystem holding it", item.Kind)
		case got != want:
			t.Errorf("a subsystem holds %s: %d, the prototype's %d", item.Kind, got, want)
		}
	}
}

// subsystemHolding writes a subsystem whose content is the one member.
func subsystemHolding(t *testing.T, root, member string) {
	t.Helper()
	id := uuid.MustNew().String()
	writeMetadata(t, root, SubsystemKind, id, "format: 1\nid: "+id+"\nname: Продажи\ntitle: {ru: Продажи}\nmembers: ["+member+"]\n")
}

// A language and a WebSocket client in the content of a subsystem are refused
// on reading it, naming the subsystem, the kind and the object.
//
// Defect caught: a subsystem holding what the designer offers no way to put
// into one, loaded as if it were a common module.
func TestASubsystemRefusesALanguageAndAWebSocketClient(t *testing.T) {
	t.Parallel()
	t.Run("language", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		configuration, err := project.ValidateLayout(root)
		if err != nil {
			t.Fatal(err)
		}
		subsystemHolding(t, root, configuration.Languages[0].ID.String())
		message := loadRefused(t, root, "a language in a subsystem")
		if !strings.Contains(message, "subsystem Продажи holds languages Русский") {
			t.Fatalf("the refusal %q names not the subsystem and the language", message)
		}
	})
	t.Run("websocket client", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeWebSocketClient(t, root, firstWebSocketClient, "Биржа", "")
		subsystemHolding(t, root, firstWebSocketClient)
		message := loadRefused(t, root, "a WebSocket client in a subsystem")
		if !strings.Contains(message, "subsystem Продажи holds websocket-clients Биржа") {
			t.Fatalf("the refusal %q names not the subsystem and the client", message)
		}
	})
}

// A recalculation in the content of a subsystem is taken and noted, as nothing
// shows whether the designer saves it.
//
// Defect caught: a kind not seen either way refused, or taken silently.
func TestASubsystemNotesARecalculation(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	chart := noteChart(t, root, ChartOfCalculationTypesKind, "")
	register, recalculation := uuid.MustNew().String(), uuid.MustNew().String()
	writeMetadata(t, root, CalculationRegisterKind, register, "format: 1\nid: "+register+"\nname: Начисления\ntitle: {ru: Начисления}\n"+
		"chart_of_calculation_types: "+chart+"\nperiodicity: month\n"+
		"resources:\n  - {id: "+uuid.MustNew().String()+", name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}\n"+
		"recalculations:\n  - {id: "+recalculation+", name: Перерасчет, title: {ru: Перерасчёт}}\n")
	subsystemHolding(t, root, recalculation)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	var noted []Note
	for _, note := range catalog.Notes() {
		if note.Kind == NoteUseWithoutSample {
			noted = append(noted, note)
		}
	}
	if len(noted) != 1 || noted[0].Where != "subsystems Продажи members" || noted[0].Written != "recalculations Начисления.Перерасчет" {
		t.Fatalf("notes = %+v, want the recalculation in the subsystem", noted)
	}
}
