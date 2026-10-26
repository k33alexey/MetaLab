package studio

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
)

// studioPageProbe runs in the page before Studio's own scripts. It notes
// every error the page meets - thrown, rejected or written to the console -
// with the step the walk was on, and counts requests still being read, so the
// walk knows when an editor has finished drawing what it fetched.
const studioPageProbe = `(() => {
  const studio = {problems: [], step: 'loading', pending: 0};
  window.__studio = studio;
  const note = text => studio.problems.push(studio.step + ': ' + text);
  addEventListener('error', event => note(event.error?.stack || event.message));
  addEventListener('unhandledrejection', event => note('unhandled rejection: ' + (event.reason?.stack || event.reason)));
  for (const level of ['error', 'warn']) {
    const write = console[level];
    console[level] = (...args) => { note('console.' + level + ': ' + args.join(' ')); write.apply(console, args); };
  }
  const count = promise => { studio.pending++; return promise.finally(() => { studio.pending--; }); };
  const fetch = window.fetch;
  window.fetch = function (...args) { return count(fetch.apply(this, args)); };
  for (const name of ['json', 'text']) {
    const read = Response.prototype[name];
    Response.prototype[name] = function () { return count(read.call(this)); };
  }
  // settle waits until nothing is being fetched or read and the page has
  // drawn; a status bar in red is noted and cleared.
  const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
  studio.settle = async () => {
    const deadline = Date.now() + 15000;
    for (let quiet = 0; quiet < 3;) {
      await sleep(quiet ? 0 : 5);
      if (Date.now() > deadline) { note('requests never finished'); return; }
      quiet = studio.pending === 0 ? quiet + 1 : 0;
    }
    await new Promise(resolve => requestAnimationFrame(() => resolve()));
    const status = document.querySelector('#status');
    if (status.classList.contains('error')) { note('status bar: ' + status.textContent); status.classList.remove('error'); }
  };
})();`

// studioPageWalk clicks every node of the configuration tree. The first time
// an editor shows a file, it is walked in every language of the project: each
// element the editor lists is chosen, each restriction and template opens
// its window, and so does each window of a filter and of the translations of
// a localized field, each then cancelled. Then the query designer and the
// debugger are opened. A step that ends with the status bar in red is a
// problem too: an editor that cannot open a file says so there, not in the
// console.
const studioPageWalk = `(async () => {
  const studio = window.__studio, problems = studio.problems, seen = new Set(), walked = new Set(), titles = {};
  let steps = 0;
  const describe = item => (item.textContent || item.title || item.tagName).trim().replace(/\s+/g, ' ').slice(0, 60);
  const settle = () => { steps++; return studio.settle(); };
  async function cancelDialog() {
    const dialog = document.querySelector('dialog[open]');
    if (!dialog) { problems.push(studio.step + ': no window opened'); return; }
    // «Отмена» by its caption: «Справка» stands after it.
    const buttons = [...dialog.querySelectorAll('.localized-title-actions button')];
    (buttons.find(button => button.textContent === 'Отмена') || buttons[buttons.length - 1]).click();
    await settle();
    if (dialog.open) problems.push(studio.step + ': the window stayed open after Cancel');
  }
  const editors = [
    ['module', () => !document.querySelector('#editor-host').hidden && !document.querySelector('#editor').disabled, document, [{select: '#bsl-symbols .bsl-item'}]],
    ['form', '#form-designer', null, [{select: '.form-node-row'}]],
    ['role', '#role-editor', null, [
      {select: '.role-tree-item', then: [{select: '.role-permissions .check-list-row', then: [{open: '.role-restrictions .role-list tbody tr[tabindex]'}]}]},
      {select: '.studio-tab', then: [{open: '.role-list tbody tr[tabindex]'}]},
      {press: '.role-tree-bar .role-subsystems'},
      {press: '.role-identity .field-with-buttons button'},
    ]],
    ['roles', '#role-overview[data-view=roles]', null, [{select: '.role-tree-item'}]],
    ['restrictions', '#role-overview[data-view=restrictions]', null, [{select: '.studio-tab', then: [{open: '.overview-block .role-list tbody tr[tabindex]'}, {press: '.overview-block .role-bulk button[data-command=filter]'}]}]],
    ['catalog', '#catalog-editor', null, [{select: '.object-window-page', then: [{press: '.field-with-buttons button'}, {select: '.object-window-body .studio-tree-row'}]}]],
    ['project', '#project-editor', null, [{press: '.field-with-buttons button'}]],
    ['languages', '#languages-editor', null, [{press: '.field-with-buttons button:enabled'}]],
  ];
  function shown() {
    for (const [name, where, host, plan] of editors) {
      if (typeof where === 'function') { if (where()) return [name, host, plan]; continue; }
      const element = document.querySelector(where);
      if (element && !element.hidden) return [name, element, plan];
    }
    return null;
  }
  async function walk(host, plan, context) {
    for (const {select, open, press, then} of plan) {
      for (let index = 0; index < 500; index++) {
        const items = [...host.querySelectorAll(select || open || press)].filter(item => select || item.offsetParent !== null);
        if (index >= items.length) break;
        const item = items[index];
        studio.step = context + ' · ' + describe(item);
        if (select || press) item.click(); else item.dispatchEvent(new MouseEvent('dblclick', {bubbles: true}));
        await settle();
        if (open || press) await cancelDialog();
        if (then) await walk(host, then, studio.step);
      }
    }
  }
  const languageSelect = document.querySelector('#studio-language');
  const languages = [...languageSelect.options].map(option => option.value);
  const tree = () => [...document.querySelectorAll('#tree .studio-tree-row')];
  if (!tree().length) problems.push('the configuration tree is empty');
  // Every node is drawn with its icon, and the icon is in the sprite: a
  // node named an icon the set does not draw, or a sprite that never came,
  // leaves an empty square in the tree.
  for (const item of tree()) {
    const href = item.querySelector(':scope > .tree-icon use')?.getAttribute('href');
    if (!href || !document.querySelector('symbol' + href)) problems.push('tree · ' + describe(item) + ': no icon drawn (' + href + ')');
  }
  // A node's context menu is opened anew before each of its commands: a
  // command closes the menu it was chosen from.
  const menu = item => {
    item.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
    return [...document.querySelectorAll('.context-menu .context-menu-item')];
  };
  for (let index = 0; index < tree().length; index++) {
    const item = tree()[index], context = 'tree · ' + describe(item);
    studio.step = context;
    item.click();
    await settle();
    await visit(context);
    const commands = menu(tree()[index]).length;
    closeContextMenu();
    for (let command = 0; command < commands; command++) {
      const button = menu(tree()[index])[command];
      if (!button) { problems.push(context + ': the menu lost its command ' + (command + 1)); break; }
      const step = context + ' · menu ' + describe(button);
      studio.step = step;
      button.click();
      await settle();
      if (document.querySelector('.context-menu')) problems.push(step + ': the menu stayed open');
      await visit(step);
    }
  }
  async function visit(context) {
    const editor = shown();
    if (!editor) return;
    const [name, host, plan] = editor;
    seen.add(name);
    const key = name + ' ' + document.querySelector('#file-path').textContent;
    if (walked.has(key)) return;
    walked.add(key);
    // The language shown last is the one Studio started with, so the next
    // node opens as it would for the user.
    const order = [...languages.filter(code => code !== languageSelect.value), languageSelect.value];
    for (const code of order) {
      studio.step = context + ' · language ' + code;
      languageSelect.value = code;
      languageSelect.dispatchEvent(new Event('change'));
      await settle();
      titles[code] ??= {header: document.querySelector('#studio-title').textContent, tab: document.title};
      await walk(host, plan, studio.step);
    }
  }
  studio.step = 'query designer';
  document.querySelector('#query-open').click();
  await settle();
  if (document.querySelector('#query-dialog').open) seen.add('query');
  const sources = document.querySelector('#query-source-select');
  for (const option of [...sources.options]) {
    studio.step = 'query designer · source ' + option.textContent;
    sources.value = option.value;
    sources.dispatchEvent(new Event('change'));
    document.querySelector('#query-add-source').click();
    await settle();
  }
  await walk(document.querySelector('#query-dialog'), [{select: '#query-sources input[type=checkbox]'}], 'query designer');
  studio.step = 'query designer · build';
  document.querySelector('#query-build').click();
  await settle();
  document.querySelector('#query-close').click();
  await settle();
  studio.step = 'debugger';
  document.querySelector('#debug-open').click();
  await settle();
  if (!document.querySelector('#debug-panel').hidden) seen.add('debugger');
  document.querySelector('#debug-close').click();
  await settle();
  return {editors: [...seen].sort(), languages, titles, steps};
})()`

// studioGutterFits opens the session module, of as many lines as the Go side
// wrote, goes to its last line, puts a breakpoint and the line being executed
// there and answers where the marker, the number and the inside of the gutter
// stand, left and right edges, which marker the last line shows and how many
// rows the gutter has drawn. Defects caught: a number running under the
// marker or cut off by the edge of the gutter once the numbers grow longer (a
// module of five- and six-digit lines); a gutter that stops numbering at some
// line (it did at 50 000); a gutter taller or shorter than the text, its
// numbers drifting from their lines; a gutter that draws every line, which
// cost 0.8 s a click on a module of 44 000 lines; a breakpoint that lays out
// the whole module again.
const studioGutterFits = `(async () => {
  const studio = window.__studio, last = %d;
  studio.step = 'gutter of ' + last + ' lines';
  document.querySelector('#refresh').click();
  await studio.settle();
  document.querySelector('#tree summary').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: 10, clientY: 10}));
  [...document.querySelectorAll('.context-menu .context-menu-item')].find(item => item.textContent === 'Открыть модуль сеанса').click();
  await studio.settle();
  bslEditor.goTo({line: last, column: 1});
  bslEditor.setBreakpoints([last]);
  bslEditor.setExecutionLine(last);
  await studio.settle();
  // A breakpoint set on a line already drawn: what it costs to lay the page
  // out again is what the change disturbed beyond the gutter.
  document.body.offsetHeight;
  bslEditor.setBreakpoints([last, last - 1]);
  const before = performance.now();
  document.body.offsetHeight;
  const layout = performance.now() - before;
  bslEditor.setBreakpoints([last]);
  await studio.settle();
  const gutter = document.querySelector('#line-gutter .gutter-scroller'), style = getComputedStyle(gutter), box = gutter.getBoundingClientRect();
  const digits = [...gutter.querySelectorAll('.gutter-digits')].find(item => item.textContent === String(last));
  if (!digits) return {Text: 'no row of line ' + last + ' drawn'};
  const marker = digits.previousElementSibling;
  const edges = element => { const rect = element.getBoundingClientRect(); return [rect.left, rect.right]; };
  return {Marker: edges(marker), Digits: edges(digits), Text: digits.textContent, Icon: marker.querySelector('use')?.getAttribute('href') || '',
    Rows: gutter.querySelectorAll('.gutter-line').length, Drift: gutter.scrollHeight - document.querySelector('#editor').scrollHeight, Layout: layout,
    Gutter: [box.left + parseFloat(style.borderLeftWidth) + parseFloat(style.paddingLeft), box.right - parseFloat(style.borderRightWidth) - parseFloat(style.paddingRight)]};
})()`

// Every editor of Studio builds its page in code no Go test reaches: an
// attribute of a catalog did not open from 2.50 to 2.279, and nobody saw it.
// Here Studio runs in a real browser on a copy of the example with a role
// that has a restriction and a template. Every node of the tree is opened;
// every editor is walked in every language of the project, choosing each
// element it lists and opening each restriction and template; the query
// designer and the debugger are opened too. Defects caught: an editor that
// throws, rejects or writes an error while opening, choosing an element or
// changing the language; one that answers in red on the status bar; a request
// that fails; a window that does not open or does not close; and an editor no
// node of the tree reaches any more; a header or tab that names the
// configuration otherwise than by its synonym in the display language (2.303:
// "[object Object]"). Without Chrome the test is skipped, as the tests run
// on Node are without Node.
func TestStudioEditorsWorkInBrowser(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) { addRestrictedRole(t, workspace, root) })
	result, err := browser.evaluate(ctx, page, studioPageWalk)
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Editors   []string `json:"editors"`
		Languages []string `json:"languages"`
		Titles    map[string]struct {
			Header string `json:"header"`
			Tab    string `json:"tab"`
		} `json:"titles"`
		Steps int `json:"steps"`
	}
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	t.Logf("%d steps, editors %v, languages %v", report.Steps, report.Editors, report.Languages)

	for _, lines := range []int{9, 12000, 100000} {
		writeBSLTestSource(t, root, project.SessionModuleFile, "Процедура П()\n"+strings.Repeat("\n", lines-2)+"КонецПроцедуры")
		result, err = browser.evaluate(ctx, page, fmt.Sprintf(studioGutterFits, lines))
		if err != nil {
			t.Fatal(err)
		}
		var gutter struct {
			Marker, Digits, Gutter [2]float64
			Text, Icon             string
			Rows, Drift            int
			Layout                 float64
		}
		if err := json.Unmarshal(result, &gutter); err != nil {
			t.Fatalf("the page answered %s: %v", result, err)
		}
		if gutter.Text != strconv.Itoa(lines) || gutter.Marker[1] > gutter.Digits[0] || gutter.Digits[1] > gutter.Gutter[1] || gutter.Marker[0] < gutter.Gutter[0] {
			t.Errorf("a module of %d lines: the marker %v and the number %q %v do not stand side by side inside the gutter %v", lines, gutter.Marker, gutter.Text, gutter.Digits, gutter.Gutter)
		}
		if gutter.Icon != "#icon-breakpoint-mark-current" {
			t.Errorf("a module of %d lines: the last line, stopped on with a breakpoint, is marked %q", lines, gutter.Icon)
		}
		// The gutter and the text are one height, or the numbers drift away
		// from their lines down a long module (1250 px at 100 000 lines).
		if gutter.Drift < -1 || gutter.Drift > 1 {
			t.Errorf("a module of %d lines: the gutter and the text differ in height by %d px", lines, gutter.Drift)
		}
		// A breakpoint lays out the gutter, not the highlighted text of the
		// module beside it: 270 ms at 44 000 lines and 630 ms at 100 000 while
		// the rows were a cell of the editor's grid; 1 ms since.
		if gutter.Layout > 150 {
			t.Errorf("a module of %d lines: a breakpoint laid the page out again in %.0f ms", lines, gutter.Layout)
		}
		// The rows in sight and a margin around them - not every line.
		if gutter.Rows > 300 {
			t.Errorf("a module of %d lines: the gutter drew %d rows", lines, gutter.Rows)
		}
	}
	for _, problem := range browser.problems() {
		t.Errorf("browser: %s", problem)
	}
	for _, editor := range []string{"catalog", "debugger", "form", "languages", "module", "project", "query", "restrictions", "role", "roles"} {
		if !slices.Contains(report.Editors, editor) {
			t.Errorf("no step opened the %s editor; opened %v", editor, report.Editors)
		}
	}
	if len(report.Languages) < 3 {
		t.Errorf("the example has three languages, the page offers %v", report.Languages)
	}
	// The synonyms are those written in examples/sales-and-warehouse.
	for code, synonym := range map[string]string{"ru": "Продажи и склад", "uk": "Продажі і склад", "en": "Sales and warehouse"} {
		if title := report.Titles[code]; title.Header != "ML Studio: "+synonym || title.Tab != "ML Studio — "+synonym {
			t.Errorf("in language %s the header reads %q and the tab %q, want the synonym %q", code, title.Header, title.Tab, synonym)
		}
	}
}

// openStudioInBrowser serves Studio on the project at root, prepared by
// prepare, and opens it in the browser with studioPageProbe, waiting for the
// tree to be drawn. Without Chrome, or on Windows, the test is skipped.
func openStudioInBrowser(t *testing.T, root string, prepare func(*Workspace)) (context.Context, *devtools, string) {
	t.Helper()
	chrome := findChrome()
	if chrome == "" {
		t.Skip("Chrome or Chromium is required for this test (set ML_TEST_CHROME to its path)")
	}
	if runtime.GOOS == "windows" {
		t.Skip("the browser is driven through descriptors 3 and 4, which Windows does not pass")
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	prepare(workspace)
	server := httptest.NewServer(NewHandler(workspace))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	browser := startChrome(t, chrome)
	page, err := browser.openPage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := browser.call(ctx, page, "Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": studioPageProbe}); err != nil {
		t.Fatal(err)
	}
	if _, err := browser.call(ctx, page, "Page.navigate", map[string]any{"url": server.URL + "/"}); err != nil {
		t.Fatal(err)
	}
	if _, err := browser.evaluate(ctx, page, `(async () => {
		while (!document.querySelector('#tree .studio-tree-row') || window.__studio.pending) await new Promise(resolve => setTimeout(resolve, 20));
	})()`); err != nil {
		t.Fatal(err)
	}
	return ctx, browser, page
}

// addRestrictedRole gives the project a role with a right on the catalog
// Товары, a restriction on it and a template, so that the role editor and the
// overview of roles have something in every list they draw.
func addRestrictedRole(t *testing.T, workspace *Workspace, root string) {
	t.Helper()
	configuration, err := metadata.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	var goods *metadata.CatalogDefinition
	for index := range configuration.Catalogs {
		if configuration.Catalogs[index].Name == "Товары" {
			goods = &configuration.Catalogs[index]
		}
	}
	if goods == nil {
		t.Fatal("the example has no catalog Товары")
	}
	created, err := workspace.CreateRole("Кладовщик")
	if err != nil {
		t.Fatal(err)
	}
	created.Role.RestrictionTemplates = []metadata.RestrictionTemplate{{Name: "Разрешено(Поле)", Condition: "ГДЕ #Параметр(1) = ИСТИНА"}}
	created.Role.Objects = []metadata.ObjectPermission{{Object: goods.ID,
		Operations:   []metadata.PermissionOperation{metadata.PermissionRead, metadata.PermissionUpdate},
		Restrictions: []metadata.RightRestriction{{Right: metadata.PermissionRead, Condition: "ГДЕ НЕ ПометкаУдаления"}}}}
	if _, err := workspace.SaveRole(created.Path, created.Role, created.Revision); err != nil {
		t.Fatal(err)
	}
}

// findChrome names the browser the DOM tests drive: ML_TEST_CHROME when it
// is set, otherwise Chrome or Chromium where they are usually installed.
func findChrome() string {
	if path := os.Getenv("ML_TEST_CHROME"); path != "" {
		return path
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	if runtime.GOOS == "darwin" {
		for _, path := range []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		} {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

// devtools speaks the DevTools protocol with a headless Chrome over the
// pipe Chrome opens on descriptors 3 and 4: one JSON message per write, ended
// by a zero byte.
type devtools struct {
	write    *os.File
	mutex    sync.Mutex
	next     int
	waiting  map[int]chan devtoolsMessage
	seen     []string
	finished chan struct{}
}

type devtoolsMessage struct {
	ID        int                       `json:"id,omitempty"`
	Method    string                    `json:"method,omitempty"`
	Params    json.RawMessage           `json:"params,omitempty"`
	Result    json.RawMessage           `json:"result,omitempty"`
	Error     *struct{ Message string } `json:"error,omitempty"`
	SessionID string                    `json:"sessionId,omitempty"`
}

// startChrome starts the browser and closes it, with every process it
// started, when the test ends.
func startChrome(t *testing.T, chrome string) *devtools {
	t.Helper()
	browserReads, testWrites, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	testReads, browserWrites, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	arguments := []string{"--headless=new", "--remote-debugging-pipe", "--user-data-dir=" + t.TempDir(),
		"--no-first-run", "--no-default-browser-check", "--disable-extensions", "--disable-sync",
		"--disable-background-networking", "--disable-component-update", "--mute-audio",
		// Wide enough for the palette beside a window: Studio hides it below
		// 850 pixels, and the headless window is 800 by default.
		"--window-size=1280,800", "about:blank"}
	if runtime.GOOS == "linux" {
		// CI runners refuse Chrome the user namespaces its sandbox needs;
		// the page is Studio's own, served by this test.
		arguments = append(arguments, "--no-sandbox")
	}
	command := exec.Command(chrome, arguments...)
	command.ExtraFiles = []*os.File{browserReads, browserWrites}
	if err := command.Start(); err != nil {
		t.Fatalf("start %s: %v", chrome, err)
	}
	browserReads.Close()
	browserWrites.Close()
	browser := &devtools{write: testWrites, waiting: map[int]chan devtoolsMessage{}, finished: make(chan struct{})}
	go browser.read(testReads)
	exited := make(chan struct{})
	go func() {
		command.Wait()
		close(exited)
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		browser.call(ctx, "", "Browser.close", nil)
		select {
		case <-exited:
		case <-time.After(10 * time.Second):
			command.Process.Kill()
			<-exited
		}
		testWrites.Close()
		<-browser.finished
		testReads.Close()
	})
	return browser
}

func (browser *devtools) read(pipe io.Reader) {
	defer close(browser.finished)
	reader := bufio.NewReader(pipe)
	for {
		frame, err := reader.ReadBytes(0)
		if err != nil {
			return
		}
		var message devtoolsMessage
		if json.Unmarshal(frame[:len(frame)-1], &message) != nil {
			continue
		}
		if message.ID != 0 {
			browser.mutex.Lock()
			reply := browser.waiting[message.ID]
			delete(browser.waiting, message.ID)
			browser.mutex.Unlock()
			if reply != nil {
				reply <- message
			}
			continue
		}
		browser.event(message)
	}
}

// event keeps what the browser reports past the page's own probe: a failed
// request, a message of the browser itself, an error thrown before the probe
// ran. A window of the page's own - confirm or alert - is dismissed so the
// walk does not hang on it, and kept as a problem.
func (browser *devtools) event(message devtoolsMessage) {
	var params struct {
		Type string `json:"type"`
		Args []struct {
			Value       any    `json:"value"`
			Description string `json:"description"`
		} `json:"args"`
		ExceptionDetails struct {
			Text      string `json:"text"`
			Exception struct {
				Description string `json:"description"`
			} `json:"exception"`
		} `json:"exceptionDetails"`
		Entry struct {
			Level string `json:"level"`
			Text  string `json:"text"`
			URL   string `json:"url"`
		} `json:"entry"`
		Message string `json:"message"`
	}
	json.Unmarshal(message.Params, &params)
	var problem string
	switch message.Method {
	case "Runtime.consoleAPICalled":
		parts := make([]string, 0, len(params.Args))
		for _, argument := range params.Args {
			if argument.Description != "" {
				parts = append(parts, argument.Description)
			} else {
				parts = append(parts, fmt.Sprint(argument.Value))
			}
		}
		problem = "console." + params.Type + ": " + strings.Join(parts, " ")
	case "Runtime.exceptionThrown":
		problem = "exception: " + params.ExceptionDetails.Text + " " + params.ExceptionDetails.Exception.Description
	case "Log.entryAdded":
		if params.Entry.Level != "error" && params.Entry.Level != "warning" {
			return
		}
		problem = "log " + params.Entry.Level + ": " + params.Entry.Text + " " + params.Entry.URL
	case "Page.javascriptDialogOpening":
		problem = "the page opened a window of the browser: " + params.Message
		go browser.call(context.Background(), message.SessionID, "Page.handleJavaScriptDialog", map[string]any{"accept": false})
	default:
		return
	}
	browser.mutex.Lock()
	browser.seen = append(browser.seen, problem)
	browser.mutex.Unlock()
}

func (browser *devtools) problems() []string {
	browser.mutex.Lock()
	defer browser.mutex.Unlock()
	return slices.Clone(browser.seen)
}

func (browser *devtools) call(ctx context.Context, session, method string, params any) (json.RawMessage, error) {
	encodedParams, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	if params == nil {
		encodedParams = nil
	}
	reply := make(chan devtoolsMessage, 1)
	browser.mutex.Lock()
	browser.next++
	id := browser.next
	browser.waiting[id] = reply
	frame, err := json.Marshal(devtoolsMessage{ID: id, Method: method, Params: encodedParams, SessionID: session})
	if err == nil {
		_, err = browser.write.Write(append(frame, 0))
	}
	browser.mutex.Unlock()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	select {
	case message := <-reply:
		if message.Error != nil {
			return nil, fmt.Errorf("%s: %s", method, message.Error.Message)
		}
		return message.Result, nil
	case <-browser.finished:
		return nil, fmt.Errorf("%s: the browser closed the pipe", method)
	case <-ctx.Done():
		return nil, fmt.Errorf("%s: %w", method, ctx.Err())
	}
}

// openPage opens a tab and turns on the reports event keeps.
func (browser *devtools) openPage(ctx context.Context) (string, error) {
	var target struct {
		TargetID string `json:"targetId"`
	}
	var attached struct {
		SessionID string `json:"sessionId"`
	}
	result, err := browser.call(ctx, "", "Target.createTarget", map[string]any{"url": "about:blank"})
	if err == nil {
		err = json.Unmarshal(result, &target)
	}
	if err == nil {
		result, err = browser.call(ctx, "", "Target.attachToTarget", map[string]any{"targetId": target.TargetID, "flatten": true})
	}
	if err == nil {
		err = json.Unmarshal(result, &attached)
	}
	for _, domain := range []string{"Runtime", "Log", "Page"} {
		if err == nil {
			_, err = browser.call(ctx, attached.SessionID, domain+".enable", nil)
		}
	}
	return attached.SessionID, err
}

// evaluate runs an expression in the page, waits for the promise it gives
// and answers its value as JSON.
func (browser *devtools) evaluate(ctx context.Context, page, expression string) (json.RawMessage, error) {
	result, err := browser.call(ctx, page, "Runtime.evaluate", map[string]any{"expression": expression, "awaitPromise": true, "returnByValue": true})
	if err != nil {
		return nil, err
	}
	var answer struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text      string `json:"text"`
			Exception struct {
				Description string `json:"description"`
			} `json:"exception"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(result, &answer); err != nil {
		return nil, err
	}
	if answer.ExceptionDetails != nil {
		return nil, errors.New(answer.ExceptionDetails.Text + " " + answer.ExceptionDetails.Exception.Description)
	}
	return answer.Result.Value, nil
}
