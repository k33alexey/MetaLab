package studio

import (
	"encoding/json"
	"testing"
)

// treeIndentWalk opens every branch of the tree and reads, for every row,
// how deep it lies and where its icon stands.
const treeIndentWalk = `(async () => {
  document.querySelectorAll('#tree details').forEach(item => { item.open = true; });
  await new Promise(resolve => requestAnimationFrame(resolve));
  return [...document.querySelectorAll('#tree summary, #tree .leaf')].map(row => {
    let depth = 0;
    for (let at = row.parentElement; at && at.id !== 'tree'; at = at.parentElement) if (at.classList.contains('children')) depth++;
    return {title: row.textContent.trim(), leaf: row.classList.contains('leaf'), depth, icon: row.querySelector('svg, img').getBoundingClientRect().left};
  });
})()`

// Every row of the tree stands one step right of the row it lies under, a
// row without anything to unfold as much as one with: the icon of a row is
// where its depth puts it, whatever the branch, as in the prototype's tree.
// Defects caught: the rows that hold nothing - a subsystem, a role, an
// attribute, a module - with their icon under their parent's, because the
// place of the sign of unfolding is kept only by the rows that have it
// (noticed by the owner 19.10.2026).
func TestEveryRowOfTheTreeStandsOneStepRightOfItsParent(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(*Workspace) {})
	result, err := browser.evaluate(ctx, page, treeIndentWalk)
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Title string
		Leaf  bool
		Depth int
		Icon  float64
	}
	if err := json.Unmarshal(result, &rows); err != nil {
		t.Fatalf("%v: %s", err, result)
	}
	icons := map[int]float64{}
	leaves, nodes := 0, 0
	for _, row := range rows {
		if row.Leaf {
			leaves++
		} else {
			nodes++
		}
		if at, ok := icons[row.Depth]; !ok {
			icons[row.Depth] = row.Icon
		} else if at != row.Icon {
			t.Errorf("%q (depth %d, leaf %v) has its icon at %v, the rows of its depth at %v", row.Title, row.Depth, row.Leaf, row.Icon, at)
		}
	}
	for depth := 1; depth < len(icons); depth++ {
		if step := icons[depth] - icons[depth-1]; step < 10 {
			t.Errorf("depth %d stands %v right of depth %d, want one step", depth, step, depth-1)
		}
	}
	if leaves == 0 || nodes == 0 || len(icons) < 4 {
		t.Fatalf("the tree has %d leaves, %d nodes and %d depths: too little to tell", leaves, nodes, len(icons))
	}
}
