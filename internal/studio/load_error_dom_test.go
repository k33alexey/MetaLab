package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// formWithAKeyTooMany is the common form of the demo project, the file a
// developer gets past Studio with a key ML does not read.
const formWithAKeyTooMany = "metadata/common-forms/ДемоФормаТовара/form.yaml"

// TestTheSnapshotSaysWhyTheProjectDoesNotRead catches the refusal of the
// load thrown away (2.438, case 2в-11; question В7 of the series of 2.437):
// a description with a key ML does not read opened the project with the
// status «Готово» and nothing else to see. The snapshot names the file from
// the project's folder and the key, in one line; once the key is gone it
// names nothing.
func TestTheSnapshotSaysWhyTheProjectDoesNotRead(t *testing.T) {
	t.Parallel()
	root := copyDemoProjectForStudioTest(t)
	form := filepath.Join(root, filepath.FromSlash(formWithAKeyTooMany))
	original, err := os.ReadFile(form)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(form, append(append([]byte{}, original...), "лишний_ключ: 1\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatalf("the project with a key too many does not open: %v", err)
	}
	if !strings.Contains(snapshot.LoadError, formWithAKeyTooMany) || !strings.Contains(snapshot.LoadError, "лишний_ключ") ||
		strings.Contains(snapshot.LoadError, root) || strings.Contains(snapshot.LoadError, "\n") {
		t.Errorf("the snapshot says %q, want the file from the project's folder and the key, in one line", snapshot.LoadError)
	}
	if err := os.WriteFile(form, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if snapshot, err = workspace.Snapshot(); err != nil || snapshot.LoadError != "" {
		t.Errorf("the project that reads says %q (%v)", snapshot.LoadError, err)
	}
}

// loadErrorWalk reads the status line of Studio; with refresh it rereads the
// project first, as «Обновить» does.
const loadErrorWalk = `(async refresh => {
  const studio = window.__studio;
  if (refresh) { document.querySelector('#refresh').click(); await studio.settle(); }
  await studio.settle();
  const line = document.querySelector('#load-error');
  return {shown: !line.hidden, text: line.textContent, status: document.querySelector('#status').textContent};
})`

// Studio says in its status line why the project does not read as a whole,
// when it opens it and when it reads it again, and keeps saying it while the
// description does not read; the line goes once it does (BLOCKS 2в,
// «непрочитанное описание — причина строкой состояния», the owner's
// decision for the question В7 of 22.10.2026). Defect caught: the status
// «Готово» over a project whose form did not read.
func TestTheStatusLineSaysWhyTheProjectDoesNotRead(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	form := filepath.Join(root, filepath.FromSlash(formWithAKeyTooMany))
	original, err := os.ReadFile(form)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(form, append(append([]byte{}, original...), "лишний_ключ: 1\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	type line struct {
		Shown        bool
		Text, Status string
	}
	read := func(refresh bool) line {
		t.Helper()
		result, err := browser.evaluate(ctx, page, loadErrorWalk+"("+map[bool]string{true: "true", false: "false"}[refresh]+")")
		if err != nil {
			t.Fatal(err)
		}
		var got line
		if err := json.Unmarshal(result, &got); err != nil {
			t.Fatalf("%v: %s", err, result)
		}
		return got
	}
	for _, step := range []struct {
		name string
		got  line
	}{{"as it opens", read(false)}, {"read again", read(true)}} {
		if !step.got.Shown || !strings.Contains(step.got.Text, formWithAKeyTooMany) || !strings.Contains(step.got.Text, "лишний_ключ") {
			t.Errorf("%s the status line says %q beside %q, want the file and the key", step.name, step.got.Text, step.got.Status)
		}
	}
	if err := os.WriteFile(form, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := read(true); got.Shown {
		t.Errorf("once the form reads the status line still says %q", got.Text)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
