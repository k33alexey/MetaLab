package studio

import (
	"encoding/json"
	"testing"
)

// treeClickWalk clicks and double-clicks rows of the project tree and of the
// tree of rights of the role Кладовщик the way a mouse does - a click with
// its count, two of them and a double click - and reads after each step
// whether the row stands open and chosen.
const treeClickWalk = `(async () => {
  const studio = window.__studio, steps = {};
  const caption = details => details.querySelector(':scope > .studio-tree-row').textContent.trim();
  const node = (host, text) => [...host.querySelectorAll('details')].find(item => caption(item) === text);
  const press = async (target, count) => {
    for (let detail = 1; detail <= count; detail++) {
      for (const type of ['mousedown', 'mouseup', 'click']) target.dispatchEvent(new MouseEvent(type, {bubbles: true, cancelable: true, detail}));
    }
    if (count === 2) target.dispatchEvent(new MouseEvent('dblclick', {bubbles: true, cancelable: true, detail: 2}));
    await studio.settle();
  };
  const state = (host, text) => { const item = node(host, text); return {open: item.open, chosen: item.querySelector(':scope > .studio-tree-row').matches('.selected') || !!item.querySelector(':scope > .studio-tree-row .selected')}; };
  const tree = document.querySelector('#tree'), label = (host, text) => node(host, text).querySelector(':scope > .studio-tree-row > span:not(.studio-tree-sign), :scope > .studio-tree-row > button');
  const root = tree.querySelector('details');
  await press(label(tree, 'Справочники'), 1); steps.branchClicked = state(tree, 'Справочники');
  await press(label(tree, 'Справочники'), 2); steps.branchDoubled = state(tree, 'Справочники');
  await press(label(tree, 'Товары'), 2); steps.objectDoubled = state(tree, 'Товары');
  await press(label(tree, 'Справочники'), 2); steps.branchDoubledAgain = state(tree, 'Справочники');
  await press(node(tree, 'Справочники').querySelector(':scope > .studio-tree-row > .studio-tree-sign'), 1); steps.signClicked = state(tree, 'Справочники');
  await press(label(tree, caption(root)), 1); steps.rootClicked = root.open;
  [...tree.querySelectorAll('.studio-tree-row')].find(item => item.textContent.trim() === 'Кладовщик').click(); await studio.settle();
  const rights = document.querySelector('#role-editor .role-tree');
  await press(label(rights, 'Справочники'), 1); steps.rightsClicked = state(rights, 'Справочники');
  await press(label(rights, 'Справочники'), 2); steps.rightsDoubled = state(rights, 'Справочники');
  await press(label(rights, 'Справочники'), 2); steps.rightsDoubledAgain = state(rights, 'Справочники');
  return steps;
})()`

// A single click on a row of the project tree or of the tree of rights only
// chooses it; a double click on a branch opens it and a second one closes it,
// as does the sign of unfolding; a double click on an object does what the
// object does, not fold it; a click on the root leaves the tree open
// (configurator 22.10.2026, block-00/obolochka.md and rol.md; STUDIO-SHELL.md).
// Defect caught (2.437, case 2в-14, question В6): a single click on the
// caption of a branch of the project tree opened and closed it.
func TestASingleClickInATreeOnlyChooses(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(opened *Workspace) {
		addRestrictedRole(t, opened, root)
	})
	result, err := browser.evaluate(ctx, page, treeClickWalk)
	if err != nil {
		t.Fatal(err)
	}
	type state struct{ Open, Chosen bool }
	var got struct {
		BranchClicked, BranchDoubled, ObjectDoubled, BranchDoubledAgain, SignClicked, RightsClicked, RightsDoubled, RightsDoubledAgain state
		RootClicked                                                                                                                    bool
	}
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	for _, check := range []struct {
		name      string
		got, want state
	}{
		{"a click on «Справочники»", got.BranchClicked, state{false, true}},
		{"a double click on «Справочники»", got.BranchDoubled, state{true, true}},
		{"a double click on Товары", got.ObjectDoubled, state{false, true}},
		{"a second double click on «Справочники»", got.BranchDoubledAgain, state{false, true}},
		{"a click on the sign of «Справочники»", got.SignClicked, state{true, true}},
		{"a click on «Справочники» of the tree of rights", got.RightsClicked, state{false, true}},
		{"a double click on «Справочники» of the tree of rights", got.RightsDoubled, state{true, true}},
		{"a second double click on «Справочники» of the tree of rights", got.RightsDoubledAgain, state{false, true}},
	} {
		if check.got != check.want {
			t.Errorf("%s leaves it open %v, chosen %v; want open %v, chosen %v", check.name, check.got.Open, check.got.Chosen, check.want.Open, check.want.Chosen)
		}
	}
	if !got.RootClicked {
		t.Error("a click on the root of the project tree closed it")
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
}
