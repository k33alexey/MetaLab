package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A button that opens a menu is made by menuButton of studio-common.js, which
// gives it the sign of the menu; the row of the main menu, whose titles the
// prototype draws without one, is the one other place. Defects caught: a
// button of several actions built in an editor of its own, without the sign
// (as «Действия» of the role were until 19.10.2026).
func TestEveryButtonOfAMenuIsMadeByOneHelper(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("ui/*.js")
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, "ui/index.html")
	allowed := map[string]int{"ui/studio-common.js": 1, "ui/studio-menu.js": 1}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if count := strings.Count(string(content), "aria-haspopup"); count != allowed[filepath.ToSlash(file)] {
			t.Errorf("%s makes %d buttons of a menu itself, want %d (menuButton, studio-common.js)", file, count, allowed[filepath.ToSlash(file)])
		}
	}
}

// menuButtonWalk opens the role and «Все ограничения доступа» and reads every
// button of a menu outside the main menu: its text and the sign after it.
const menuButtonWalk = `(async () => {
  const studio = window.__studio;
  const answer = [];
  const read = where => {
    for (const button of document.querySelectorAll('button[aria-haspopup="menu"]')) {
      if (button.closest('#main-menu')) continue;
      answer.push({where, text: button.textContent, sign: getComputedStyle(button, '::after').content});
    }
  };
  [...document.querySelectorAll('#tree summary, #tree .leaf')].find(item => item.textContent.trim() === 'Кладовщик').click();
  await studio.settle();
  read('role');
  const roles = [...document.querySelectorAll('#tree summary, #tree .leaf')].find(item => item.textContent.trim() === 'Роли');
  roles.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === 'Все ограничения доступа').click();
  await studio.settle();
  read('all restrictions');
  return answer;
})()`

// Every button of several actions in the window of a role and in «Все
// ограничения доступа» - «Действия» of the tree of rights and of the lists -
// shows the sign of its menu after its text, as the prototype's «Действия ▾»
// (the owner's rule of 19.10.2026). Defects caught: a button of a menu
// without the sign; the sign drawn by one editor and not by another.
func TestEveryButtonOfAMenuShowsItsSign(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, menuButtonWalk)
	if err != nil {
		t.Fatal(err)
	}
	var buttons []struct{ Where, Text, Sign string }
	if err := json.Unmarshal(result, &buttons); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	seen := map[string]bool{}
	for _, button := range buttons {
		seen[button.Where] = true
		if button.Sign != `"▾"` {
			t.Errorf("%s: the button %q of a menu shows %s after its text, want ▾", button.Where, button.Text, button.Sign)
		}
	}
	if !seen["role"] || !seen["all restrictions"] {
		t.Fatalf("buttons of a menu found: %+v, want some in the role and in all restrictions", buttons)
	}
}
