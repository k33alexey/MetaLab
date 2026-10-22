package studio

import (
	"encoding/json"
	"testing"
)

// emptyRestrictionWalk leaves the condition empty in each window that writes
// a restriction: the window of a restriction of the role Кладовщик, the
// wizard of «Все ограничения доступа» and its group «Изменить».
const emptyRestrictionWalk = `(async () => {` + restrictionWalkTools + `
  const seen = {};
  const clear = async () => { const text = dialog().querySelector('textarea'); text.value = '  '; text.dispatchEvent(new Event('input', {bubbles: true})); await studio.settle(); };
  const refused = () => ({title: titleOf(), problem: dialog()?.querySelector('.dialog-problem:not([hidden])')?.textContent || ''});
  byText('#tree .studio-tree-row', 'Кладовщик').click();
  await studio.settle();
  const editor = document.querySelector('#role-editor');
  editor.querySelector('[data-object="` + demoPartners + `"]').click(); await studio.settle();
  await choose(all('.role-restrictions tbody tr', editor).find(item => item.textContent.includes('ГДЕ В')));
  editor.querySelector('.role-restrictions [data-command="edit"]').click(); await studio.settle();
  await clear(); await press('ОК');
  seen.role = refused();
  await press('Отмена');
  seen.roleKept = all('.role-restrictions tbody tr', editor).some(item => item.textContent.includes('ГДЕ В'));
  await overview('Все ограничения доступа');
  document.querySelector('.overview-block .role-list').dispatchEvent(new KeyboardEvent('keydown', {key: 'Insert', bubbles: true}));
  await studio.settle();
  await mark('Контрагенты', true); await mark('Бухгалтер', true);
  await press('Далее >');
  await mark('Чтение', true);
  await clear(); await press('ОК');
  seen.wizard = refused();
  const text = dialog().querySelector('textarea');
  text.value = 'ГДЕ Е'; text.dispatchEvent(new Event('input', {bubbles: true})); await studio.settle();
  await press('ОК');
  seen.wizardWritten = !!row('Бухгалтер', 'ГДЕ Е');
  await choose(row('Контрагенты', 'ГДЕ В'));
  await command('edit');
  await clear(); await press('ОК');
  seen.group = refused();
  await press('Отмена');
  seen.groupKept = !!row('Контрагенты', 'ГДЕ В');
  return seen;
})()`

// TestNoWindowWritesARestrictionWithoutACondition catches a window of
// restrictions writing one with no condition (ACCESS-RIGHTS, «Пустое
// ограничение в окне роли»; snapshot okno-roli-pustoe-ogranichenie): the
// window of a restriction of a role, the wizard and the group «Изменить» of
// «Все ограничения доступа» each refuse a condition of spaces with the
// prototype's text and stay open, the restriction as it was; a condition
// given, the wizard writes. Defects caught: «ОК» writing the empty condition
// (2.449 and before, in every window), the refusal closing the window.
func TestNoWindowWritesARestrictionWithoutACondition(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, addRestrictionRoles(t, root))
	result, err := browser.evaluate(ctx, page, emptyRestrictionWalk)
	if err != nil {
		t.Fatal(err)
	}
	type refused struct{ Title, Problem string }
	var seen struct {
		Role, Wizard, Group                refused
		RoleKept, WizardWritten, GroupKept bool
	}
	if err := json.Unmarshal(result, &seen); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	const refusal = "Нельзя записывать пустое ограничение доступа."
	for _, check := range []struct {
		window string
		seen   refused
		title  string
	}{
		{"the window of a restriction of a role", seen.Role, "Ограничение доступа · Чтение"},
		{"the wizard", seen.Wizard, "Новое ограничение доступа"},
		{"the group «Изменить»", seen.Group, "Ограничение доступа"},
	} {
		if check.seen.Title != check.title || check.seen.Problem != refusal {
			t.Errorf("%s with an empty condition: window %q says %q, want %q open saying %q", check.window, check.seen.Title, check.seen.Problem, check.title, refusal)
		}
	}
	if !seen.RoleKept || !seen.GroupKept {
		t.Errorf("a refused empty condition changed the restriction: in the role %v, in the group %v", seen.RoleKept, seen.GroupKept)
	}
	if !seen.WizardWritten {
		t.Error("the wizard did not write the restriction once its condition was given")
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
