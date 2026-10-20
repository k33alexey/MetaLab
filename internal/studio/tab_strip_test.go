package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A strip of tabs is made by tabStrip of studio-common.js, which draws it as
// the prototype's (the owner's decision of 20.10.2026, STUDIO-SHELL.md
// «Закладки — одним обликом везде»). Defects caught: a strip of tabs built
// in an editor of its own, with its own look (as the role, «Все ограничения
// доступа» and «Настройка списка» were until 20.10.2026).
func TestEveryStripOfTabsIsMadeByOneHelper(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("ui/*.js")
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, "ui/index.html")
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if filepath.ToSlash(file) == "ui/studio-common.js" {
			want = 1
		}
		if count := strings.Count(string(content), "tablist"); count != want {
			t.Errorf("%s makes %d strips of tabs itself, want %d (tabStrip, studio-common.js)", file, count, want)
		}
	}
}

// tabStripWalk opens the role, «Все ограничения доступа» and its «Настройка
// списка», and reads each strip of tabs: the tabs, the one chosen, its
// background against the page it stands on and against the others, whether
// it stands higher, the focus; then moves along the strip by → and ← and
// reads what is shown.
const tabStripWalk = `(async () => {
  const studio = window.__studio, answer = {};
  const read = (strip, page) => {
    const tabs = [...strip.querySelectorAll('.studio-tab')], chosen = tabs.find(tab => tab.getAttribute('aria-selected') === 'true');
    const other = tabs.find(tab => tab !== chosen), style = getComputedStyle(chosen);
    return {strip: strip.className, role: strip.getAttribute('role'), tabs: tabs.map(tab => tab.textContent), chosen: chosen.textContent,
      roles: tabs.map(tab => tab.getAttribute('role')), stops: tabs.map(tab => tab.tabIndex),
      merges: style.backgroundColor === getComputedStyle(page).backgroundColor && style.borderBottomColor === style.backgroundColor,
      othersDarker: getComputedStyle(other).backgroundColor !== style.backgroundColor,
      higher: chosen.getBoundingClientRect().top < other.getBoundingClientRect().top};
  };
  const key = async (strip, name) => {
    strip.querySelector('.studio-tab[aria-selected="true"]').dispatchEvent(new KeyboardEvent('keydown', {key: name, bubbles: true}));
    await studio.settle();
    return {chosen: strip.querySelector('.studio-tab[aria-selected="true"]').textContent, focus: document.activeElement.textContent};
  };
  const tree = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.textContent.trim() === text);
  tree('Кладовщик').click(); await studio.settle();
  const role = document.querySelector('#role-editor'), roleStrip = role.querySelector('.studio-tabs');
  answer.role = read(roleStrip, role);
  answer.roleRight = await key(roleStrip, 'ArrowRight');
  answer.roleTemplates = !role.querySelector('.role-templates').hidden && role.querySelector('.role-body').hidden;
  answer.roleEnd = await key(roleStrip, 'ArrowRight');
  answer.roleLeft = await key(roleStrip, 'ArrowLeft');
  answer.roleRights = role.querySelector('.role-templates').hidden && !role.querySelector('.role-body').hidden;
  tree('Роли').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === 'Все ограничения доступа').click();
  await studio.settle();
  const overview = document.querySelector('#role-overview'), overviewStrip = () => overview.querySelector('.studio-tabs');
  answer.overview = read(overviewStrip(), overview);
  answer.overviewRight = await key(overviewStrip(), 'ArrowRight');
  answer.overviewHeadings = [...overview.querySelectorAll('.overview-block .role-list th')].map(cell => cell.textContent);
  [...overview.querySelectorAll('.studio-tab')].find(tab => tab.textContent === 'Ограничения доступа').click(); await studio.settle();
  [...overview.querySelectorAll('.overview-block .role-bulk button')].find(button => button.textContent === 'Действия').click();
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent.startsWith('Настройка списка')).click();
  await studio.settle();
  const dialog = document.querySelector('dialog[open]'), dialogStrip = dialog.querySelector('.studio-tabs');
  answer.dialog = read(dialogStrip, dialog);
  answer.dialogFocus = document.activeElement.textContent;
  answer.framed = [...dialog.querySelectorAll('.studio-tab-page')].map(page => getComputedStyle(page).borderLeftWidth + ' ' + getComputedStyle(page).borderTopWidth);
  answer.dialogRight = await key(dialogStrip, 'ArrowRight');
  answer.dialogPages = [...dialog.querySelectorAll('.studio-tab-page')].map(page => !page.hidden);
  [...dialog.querySelectorAll('.localized-title-actions button')].find(button => button.textContent === 'Отмена').click();
  await studio.settle();
  return answer;
})()`

// Every strip of tabs - in the window of a role, in «Все ограничения
// доступа» and in the dialog «Настройка списка» - is the prototype's
// (the owner's decision of 20.10.2026 by the page «было → станет»; snapshots
// configurator-2026-10-18/shabl-01, configurator-2026-10-19/shabl-19).
// Defects caught: a strip not made by tabStrip, or without the roles of a
// strip of tabs; the tab chosen not merging with the page it stands on -
// another background, or the line of the strip under it - or drawn as the
// others, or not higher than they; a tab not chosen reachable by Tab, or the
// chosen one not; → and ← not moving along the strip, not taking the focus,
// moving past its end, or not showing the page of the tab; a tab of «Все
// ограничения доступа» marked as a pressed button; the dialog opened with
// the focus off its tab; its pages not framed or framed from above.
func TestEveryStripOfTabsIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, tabStripWalk)
	if err != nil {
		t.Fatal(err)
	}
	type strip struct {
		Strip, Role, Chosen          string
		Tabs, Roles                  []string
		Stops                        []int
		Merges, OthersDarker, Higher bool
	}
	type move struct{ Chosen, Focus string }
	var got struct {
		Role, Overview, Dialog                                   strip
		RoleRight, RoleEnd, RoleLeft, OverviewRight, DialogRight move
		RoleTemplates, RoleRights                                bool
		OverviewHeadings, Framed                                 []string
		DialogFocus                                              string
		DialogPages                                              []bool
	}
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	for _, check := range []struct {
		name   string
		strip  strip
		tabs   []string
		chosen string
	}{
		{"the role", got.Role, []string{"Права", "Шаблоны ограничений"}, "Права"},
		{"all restrictions", got.Overview, []string{"Ограничения доступа", "Шаблоны ограничений"}, "Ограничения доступа"},
		{"the list settings", got.Dialog, []string{"Колонки", "Прочее"}, "Колонки"},
	} {
		s := check.strip
		if s.Strip != "studio-tabs" || s.Role != "tablist" || strings.Join(s.Roles, ",") != "tab,tab" {
			t.Errorf("%s: the strip is %q with the role %q, its tabs %v, want studio-tabs, tablist, tab", check.name, s.Strip, s.Role, s.Roles)
		}
		if strings.Join(s.Tabs, ",") != strings.Join(check.tabs, ",") || s.Chosen != check.chosen {
			t.Errorf("%s: the tabs are %v with %q chosen, want %v with %q", check.name, s.Tabs, s.Chosen, check.tabs, check.chosen)
		}
		if len(s.Stops) != 2 || s.Stops[0] != 0 || s.Stops[1] != -1 {
			t.Errorf("%s: the tabs stop Tab as %v, want the chosen one alone (0, -1)", check.name, s.Stops)
		}
		if !s.Merges || !s.OthersDarker || !s.Higher {
			t.Errorf("%s: the tab chosen merges with its page %v, differs from the others %v, stands higher %v; want all", check.name, s.Merges, s.OthersDarker, s.Higher)
		}
	}
	for _, check := range []struct {
		name      string
		got, want move
	}{
		{"→ in the role", got.RoleRight, move{"Шаблоны ограничений", "Шаблоны ограничений"}},
		{"→ at the end of the role's strip", got.RoleEnd, move{"Шаблоны ограничений", "Шаблоны ограничений"}},
		{"← in the role", got.RoleLeft, move{"Права", "Права"}},
		{"→ in all restrictions", got.OverviewRight, move{"Шаблоны ограничений", "Шаблоны ограничений"}},
		{"→ in the list settings", got.DialogRight, move{"Прочее", "Прочее"}},
	} {
		if check.got != check.want {
			t.Errorf("%s: chosen %q with the focus on %q, want %q and %q", check.name, check.got.Chosen, check.got.Focus, check.want.Chosen, check.want.Focus)
		}
	}
	if !got.RoleTemplates || !got.RoleRights {
		t.Errorf("the role shows its templates after → %v, its rights after ← %v; want both", got.RoleTemplates, got.RoleRights)
	}
	if !strings.Contains(strings.Join(got.OverviewHeadings, ","), "Текст шаблона") {
		t.Errorf("all restrictions after → shows the columns %v, want those of the templates", got.OverviewHeadings)
	}
	if got.DialogFocus != "Колонки" {
		t.Errorf("the list settings open with the focus on %q, want its tab «Колонки»", got.DialogFocus)
	}
	if strings.Join(got.Framed, ",") != "1px 0px,1px 0px" || len(got.DialogPages) != 2 || got.DialogPages[0] || !got.DialogPages[1] {
		t.Errorf("the pages of the dialog are framed as %v and shown %v after →, want framed but from above and «Прочее» alone shown", got.Framed, got.DialogPages)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
