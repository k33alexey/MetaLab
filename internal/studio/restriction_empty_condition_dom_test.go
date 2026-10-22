package studio

import (
	"encoding/json"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// emptyRestrictionWalk leaves the condition empty in each window that writes
// a restriction: the window of a restriction of the role Кладовщик, the
// wizard of «Все ограничения доступа» and its group «Изменить»; then, in the
// group «Изменить», changes only the fields of Ревизор's restriction without
// a condition, chosen with Кладовщик's and alone, and copies it to
// Бухгалтер the same two ways; then copies Кладовщик's alone.
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
  await choose(row('Контрагенты', 'ГДЕ В'));
  await choose(row('Ревизор'), {ctrlKey: true});
  await command('edit');
  seen.mixedFlag = box('Изменять ограничение').checked;
  await mark('Код', true); await press('ОК');
  seen.mixed = refused();
  if (dialog()) await press('Отмена');
  await choose(row('Ревизор'));
  await command('edit');
  await mark('Изменять ограничение', false);
  await mark('Код', true); await press('ОК');
  seen.alone = refused();
  if (dialog()) await press('Отмена');
  seen.fieldsKept = [row('Контрагенты', 'ГДЕ В'), row('Ревизор')].every(item => item?.textContent.includes('ИНН') && !item.textContent.includes('Код'));
  const accountant = () => rows().filter(item => item.textContent.includes('Бухгалтер')).map(item => item.textContent);
  const before = accountant();
  for (const [key, mixed] of [['copyMixed', true], ['copyAlone', false]]) {
    await choose(row('Ревизор'));
    if (mixed) await choose(row('Контрагенты', 'ГДЕ В'), {ctrlKey: true});
    await command('copy');
    await mark('Бухгалтер', true); await press('ОК');
    seen[key] = refused();
    if (dialog()) await press('Отмена');
  }
  seen.accountantKept = JSON.stringify(accountant()) === JSON.stringify(before);
  await choose(row('Контрагенты', 'ГДЕ В'));
  await command('copy');
  await mark('Бухгалтер', true); await press('ОК');
  if (titleOf() === 'Заменить?') await press('Да');
  seen.copied = !dialog() && !!row('Бухгалтер', 'ГДЕ В');
  return seen;
})()`

// TestNoWindowWritesARestrictionWithoutACondition catches a window of
// restrictions writing one with no condition (ACCESS-RIGHTS, «Пустое
// ограничение в окне роли»; snapshot okno-roli-pustoe-ogranichenie): the
// window of a restriction of a role, the wizard and the group «Изменить» of
// «Все ограничения доступа» each refuse a condition of spaces with the
// prototype's text and stay open, the restriction as it was; a condition
// given, the wizard writes. The group «Изменить» changing only the fields
// refuses as well when one of the chosen restrictions has no condition (such
// come with loading; the owner's decision of 22.10.2026, question В1 of the
// series 2.449): «ОК» writes every chosen one. «Копировать ограничения
// доступа» refuses the same way when one of the chosen restrictions has no
// condition and copies none (the owner's decision of 22.10.2026, question of
// the report of 2.452). Defects caught: «ОК» writing the empty condition
// (2.449 and before, in every window), the refusal closing the window, the
// group writing a restriction without a condition it was not asked to change
// (2.451 and before), the copying carrying one to another role, or the
// others chosen with it (2.452 and before), the copying refusing one with a
// condition.
func TestNoWindowWritesARestrictionWithoutACondition(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	roles := addRestrictionRoles(t, root)
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) {
		roles(workspace)
		// Ревизор restricts the reading of Контрагенты by ИНН with no
		// condition, as loading takes it (255 such in energy).
		partners, err := uuid.Parse(demoPartners)
		if err != nil {
			t.Fatal(err)
		}
		read := metadata.PermissionRead
		created, err := workspace.CreateRole("Ревизор")
		if err != nil {
			t.Fatal(err)
		}
		created.Role.Objects = []metadata.ObjectPermission{{Object: partners, Operations: []metadata.PermissionOperation{read},
			Restrictions: []metadata.RightRestriction{{Right: read, Fields: []string{demoPartnersTIN}}}}}
		if _, err := workspace.SaveRole(created.Path, created.Role, created.Revision); err != nil {
			t.Fatal(err)
		}
	})
	result, err := browser.evaluate(ctx, page, emptyRestrictionWalk)
	if err != nil {
		t.Fatal(err)
	}
	type refused struct{ Title, Problem string }
	var seen struct {
		Role, Wizard, Group, Mixed, Alone, CopyMixed, CopyAlone                           refused
		RoleKept, WizardWritten, GroupKept, MixedFlag, FieldsKept, AccountantKept, Copied bool
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
		{"the group «Изменить» of fields over a restriction with a condition and one without", seen.Mixed, "Ограничение доступа"},
		{"the group «Изменить» of fields alone over a restriction without a condition", seen.Alone, "Ограничение доступа"},
		{"the copying of a restriction with a condition and one without", seen.CopyMixed, "Копировать ограничения"},
		{"the copying of a restriction without a condition alone", seen.CopyAlone, "Копировать ограничения"},
	} {
		if check.seen.Title != check.title || check.seen.Problem != refusal {
			t.Errorf("%s with an empty condition: window %q says %q, want %q open saying %q", check.window, check.seen.Title, check.seen.Problem, check.title, refusal)
		}
	}
	if !seen.RoleKept || !seen.GroupKept {
		t.Errorf("a refused empty condition changed the restriction: in the role %v, in the group %v", seen.RoleKept, seen.GroupKept)
	}
	if seen.MixedFlag {
		t.Error("the group «Изменить» over conditions that differ has «Изменять ограничение» checked")
	}
	if !seen.FieldsKept {
		t.Error("a refused group «Изменить» of fields changed the fields of a restriction")
	}
	if !seen.AccountantKept {
		t.Error("a refused copying changed the restrictions of Бухгалтер")
	}
	if !seen.Copied {
		t.Error("the copying of a restriction with a condition did not write it to Бухгалтер")
	}
	if !seen.WizardWritten {
		t.Error("the wizard did not write the restriction once its condition was given")
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
