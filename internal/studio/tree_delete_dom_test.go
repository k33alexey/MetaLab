package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// treeDeleteWalk deletes from the tree by its «Удалить»: the role Кладовщик,
// answering «Нет» and then «Да», and the attribute Артикул of the open
// catalog Товары the same way; for each question it reads the dialog, and
// it notes any call of the browser's own confirm.
const treeDeleteWalk = `(async () => {
  const studio = window.__studio, answer = {questions: [], confirmed: false};
  window.confirm = () => { answer.confirmed = true; return true; };
  const row = text => [...document.querySelectorAll('#tree .studio-tree-row')].find(item => item.querySelector(':scope > span:not(.studio-tree-sign)')?.textContent === text);
  const remove = async reply => {
    document.querySelector('#tree-delete').click();
    await studio.settle();
    const dialog = document.querySelector('dialog[open]');
    if (!dialog) { answer.questions.push('no dialog'); return; }
    answer.questions.push(dialog.querySelector('h3').textContent + ' | ' + [...dialog.querySelectorAll('button')].map(button => button.textContent).join(', '));
    [...dialog.querySelectorAll('button')].find(button => button.textContent === reply).click();
    await studio.settle(); await studio.settle();
  };
  row('Кладовщик').click(); await studio.settle();
  await remove('Нет');
  answer.roleAfterNo = !!row('Кладовщик');
  await remove('Да');
  answer.roleAfterYes = !!row('Кладовщик');
  row('Товары').click(); await studio.settle();
  const attributes = () => [...document.querySelectorAll('#catalog-editor .catalog-list-item')].map(item => item.textContent);
  row('Артикул').click(); await studio.settle();
  await remove('Нет');
  answer.attributeAfterNo = attributes().includes('Артикул');
  await remove('Да');
  answer.attributeAfterYes = attributes().includes('Артикул');
  return answer;
})()`

// «Удалить» of the tree asks one question for an object and for a part of
// it, the prototype's (inventory, obshchee.md, «Удаление»: «Удалить объект
// конфигурации?», Да / Нет; the same on an attribute,
// vse-ogranicheniya-dostupa.md, udalenie-rekvizita-02) in ML's words
// (ML-STUDIO.md: objects of the configuration are objects of the project),
// in a dialog of Studio and never the browser's confirm. «Нет» deletes
// nothing, «Да» deletes. Defects caught: the two texts of their own
// («Удалить «X»?», «… Это действие необратимо.») through confirm() (3.9 of
// the revision of 14.10.2026); «Нет» that deletes; «Да» that does not.
func TestTheTreeAsksThePrototypesQuestionBeforeDeleting(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	var rolePath string
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
		overview, err := opened.ReadAllRoles()
		if err != nil {
			t.Fatal(err)
		}
		for _, role := range overview.Roles {
			if role.Role.Name == "Кладовщик" {
				rolePath = role.Path
			}
		}
	})
	result, err := browser.evaluate(ctx, page, treeDeleteWalk)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Questions                           []string
		Confirmed                           bool
		RoleAfterNo, RoleAfterYes           bool
		AttributeAfterNo, AttributeAfterYes bool
	}
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	question := "Удалить объект проекта? | Да, Нет"
	if want := []string{question, question, question, question}; !slices.Equal(got.Questions, want) || got.Confirmed {
		t.Errorf("the questions were %v (the browser's confirm called: %v), want %q four times in a dialog of Studio", got.Questions, got.Confirmed, question)
	}
	if !got.RoleAfterNo || got.RoleAfterYes {
		t.Errorf("the role is in the tree after «Нет»: %v, after «Да»: %v; want kept, then gone", got.RoleAfterNo, got.RoleAfterYes)
	}
	if rolePath == "" {
		t.Fatal("the role Кладовщик was not made")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rolePath))); !os.IsNotExist(err) {
		t.Errorf("the description of the deleted role is still on disk: %v", err)
	}
	if !got.AttributeAfterNo || got.AttributeAfterYes {
		t.Errorf("the attribute is in the window after «Нет»: %v, after «Да»: %v; want kept, then gone", got.AttributeAfterNo, got.AttributeAfterYes)
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
