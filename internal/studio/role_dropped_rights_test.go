package studio

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A role saved while ML still kept the four clients opens in Studio, so that
// «Убрать недоступные права» can repair it, and is not saved until it is
// repaired (ACCESS-RIGHTS, «Роли, сохранённые прежней версией ML»). Defects
// caught: Studio refusing to open such a role - nothing could repair it then;
// the editor not told which rights take the place of the clients; such a role
// saved as it is; the repaired role refused.
func TestStudioOpensARoleWithClientsMLDoesNotKeep(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	created, err := workspace.CreateRole("Кладовщик")
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	old := created.Role
	old.Objects = []metadata.ObjectPermission{{Object: configuration.Configuration.ID, Operations: []metadata.PermissionOperation{"thin-client", "web-client", "thick-client"}}}
	if err := metadata.WriteRole(root, old); err != nil {
		t.Fatal(err)
	}
	opened, err := workspace.ReadRole(created.Path)
	if err != nil {
		t.Fatalf("Studio does not open the role: %v", err)
	}
	if got := opened.Role.Objects[0].Operations; len(got) != 3 {
		t.Fatalf("the role opens with %v", got)
	}
	for right, want := range map[metadata.PermissionOperation]metadata.PermissionOperation{"thin-client": "run-application", "web-client": "run-application", "thick-client": "", "mobile-client": "",
		"interactive-open-ext-data-processors": "", "interactive-open-ext-reports": "", "configuration-extensions-administration": "",
		"collaboration-system-info-base-registration": ""} {
		if got, ok := opened.Schema.DroppedRights[right]; !ok || got != want {
			t.Errorf("the editor takes %s for %q (%v), want %q", right, got, ok, want)
		}
	}
	if _, err := workspace.SaveRole(opened.Path, opened.Role, opened.Revision); err == nil || !strings.Contains(err.Error(), "role Кладовщик gives the right thin-client, which ML does not keep") {
		t.Fatalf("the role is saved as it is: %v", err)
	}
	repaired := opened.Role
	repaired.Objects = []metadata.ObjectPermission{{Object: configuration.Configuration.ID, Operations: []metadata.PermissionOperation{"run-application"}}}
	if _, err := workspace.SaveRole(opened.Path, repaired, opened.Revision); err != nil {
		t.Fatalf("the repaired role is refused: %v", err)
	}
}

// The same role walked in the browser: the editor warns, «Убрать недоступные
// права» puts «Запуск приложения» in place of the clients, and «Сохранить»
// writes it. Defects caught: no warning on such a role, a repair that leaves
// a client, a save the page cannot make.
func TestRoleEditorRepairsClientsMLDoesNotKeep(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	var path string
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		created, err := workspace.CreateRole("Кладовщик")
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := workspace.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		old := created.Role
		old.Objects = []metadata.ObjectPermission{{Object: snapshot.Configuration.ID, Operations: []metadata.PermissionOperation{"thin-client", "web-client", "mobile-client"}}}
		if err := metadata.WriteRole(root, old); err != nil {
			t.Fatal(err)
		}
		path = created.Path
	})
	result, err := browser.evaluate(ctx, page, `(async () => {
  const studio = window.__studio;
  [...document.querySelectorAll('#tree .leaf')].find(item => item.textContent.trim() === 'Кладовщик').click();
  await studio.settle();
  const warning = document.querySelector('#role-editor .role-warning');
  const shown = !!warning && !warning.hidden;
  [...warning.querySelectorAll('button')].find(button => button.textContent === 'Убрать недоступные права').click();
  await studio.settle();
  const after = !warning.hidden;
  document.querySelector('#save').click();
  await studio.settle();
  return {shown, after, status: document.querySelector('#status').textContent};
})()`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), `"shown":true`) || !strings.Contains(string(result), `"after":false`) {
		t.Fatalf("the editor answered %s", result)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := workspace.ReadRole(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Role.Objects) != 1 || len(saved.Role.Objects[0].Operations) != 1 || saved.Role.Objects[0].Operations[0] != "run-application" {
		t.Fatalf("the role saved as %+v (%s)", saved.Role.Objects, result)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}

// A role an earlier ML saved with rights on commands of forms opens in Studio
// with the warning, «Убрать недоступные права» takes them away and
// «Сохранить» writes the role without them; as it was, it is not saved
// (ACCESS-RIGHTS, «Команды формы»). Defects caught: Studio refusing to open
// such a role; such a role saved as it is; a repair that leaves a command, so
// the role can never be saved; no warning on it.
func TestRoleEditorTakesAwayCommandsOfForms(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	var path string
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		created, err := workspace.CreateRole("Кладовщик")
		if err != nil {
			t.Fatal(err)
		}
		old := created.Role
		old.Commands = []metadata.CommandPermission{{Form: uuid.MustNew(), Command: uuid.MustNew()}}
		if err := metadata.WriteRole(root, old); err != nil {
			t.Fatal(err)
		}
		opened, err := workspace.ReadRole(created.Path)
		if err != nil || len(opened.Role.Commands) != 1 {
			t.Fatalf("Studio does not open the role: %v", err)
		}
		if _, err := workspace.SaveRole(opened.Path, opened.Role, opened.Revision); err == nil || !strings.Contains(err.Error(), "role Кладовщик gives rights on 1 commands of forms") {
			t.Fatalf("the role is saved as it is: %v", err)
		}
		path = created.Path
	})
	result, err := browser.evaluate(ctx, page, `(async () => {
  const studio = window.__studio;
  [...document.querySelectorAll('#tree .leaf')].find(item => item.textContent.trim() === 'Кладовщик').click();
  await studio.settle();
  const warning = document.querySelector('#role-editor .role-warning');
  const shown = !!warning && !warning.hidden;
  [...warning.querySelectorAll('button')].find(button => button.textContent === 'Убрать недоступные права').click();
  await studio.settle();
  const after = !warning.hidden;
  document.querySelector('#save').click();
  await studio.settle();
  return {shown, after, status: document.querySelector('#status').textContent};
})()`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), `"shown":true`) || !strings.Contains(string(result), `"after":false`) {
		t.Fatalf("the editor answered %s", result)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := workspace.ReadRole(path)
	if err != nil || len(saved.Role.Commands) != 0 {
		t.Fatalf("the role saved with %+v (%v, %s)", saved.Role.Commands, err, result)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
