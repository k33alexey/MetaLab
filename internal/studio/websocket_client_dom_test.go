package studio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// webSocketClientWalk makes a client by «Добавить» and reads what opens,
// then its palette; opens «Настройка заголовков», adds a header and reads
// the field; saves.
const webSocketClientWalk = `(async () => {
  const studio = window.__studio, steps = {};
  window.confirm = () => true;` + objectWindowSteps + `
  const settle = () => new Promise(resolve => setTimeout(resolve, 120)).then(() => studio.settle());
  const dialog = () => document.querySelector('dialog[open]');
  await openObject('WebSocket-клиенты');
  document.querySelector('#tree-add').click(); await settle();
  steps.opened = document.querySelector('#file-path').textContent;
  await openObject('WebSocketКлиент1');
  steps.palette = {title: document.querySelector('#inspector-title').textContent,
    rows: [...document.querySelectorAll('#properties .object-palette-row .object-palette-caption')].map(item => item.textContent),
    timeout: windowField('#properties', 'Таймаут (секунд)').querySelector('input').value,
    password: windowField('#properties', 'Пароль').querySelector('input').disabled};
  windowField('#properties', 'Заголовки').querySelector('button').click(); await settle();
  steps.window = {title: dialog()?.querySelector('h3')?.textContent || '', columns: [...dialog().querySelectorAll('th')].map(item => item.textContent),
    buttons: [...dialog().querySelectorAll('.object-window-panel button')].map(item => item.title + (item.disabled ? ' (недоступна)' : ''))};
  [...dialog().querySelectorAll('.object-window-panel button')].find(item => item.title === 'Добавить').click(); await settle();
  const [name, value] = dialog().querySelectorAll('tbody input');
  name.value = 'X-Version'; name.dispatchEvent(new Event('input', {bubbles: true}));
  value.value = '2'; value.dispatchEvent(new Event('input', {bubbles: true}));
  [...dialog().querySelectorAll('.localized-title-actions button')].find(item => item.textContent === 'ОК').click(); await settle();
  steps.headers = windowField('#properties', 'Заголовки').querySelector('input').value;
  const password = windowField('#properties', 'Пароль').querySelector('input');
  password.focus(); password.value = 'секрет'; password.dispatchEvent(new Event('input', {bubbles: true})); await settle();
  steps.dirty = !document.querySelector('#save').disabled;
  document.querySelector('#save').click(); await settle();
  steps.dots = windowField('#properties', 'Пароль').querySelector('input').value;
  steps.saved = !document.querySelector('#save').disabled;
  return steps;
})()`

// A WebSocket client is the prototype's (docs/block-00/websocket-klient.md,
// «Палитра», «Новый клиент»; snapshots websocket-klient/01, 06, 09):
// «Добавить» opens the empty module of the new client at once; its palette
// has the prototype's rows, the timeout 30 and the password not typed into;
// «Настройка заголовков» - the table «Заголовок — Значение», «Добавить»,
// «Скопировать», «Удалить», «Переместить вверх», «Переместить вниз» - adds a
// header shown «X-Version(2)»; the password typed by dots marks the client
// changed; «Сохранить» writes the header into the file and the password
// into the protected store of the system, the file saying password_set
// alone, the field showing dots then. Defects caught: the module not
// opened; a row or the timeout not the prototype's; the window of the
// headers not the prototype's or its edit lost; the password field not
// taking a password, the password not sent, sent into the file or not
// into the store.
func TestTheWebSocketClientIsThePrototypes(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	store := &memorySecrets{entries: map[string]string{}}
	ctx, browser, page := openStudioInBrowser(t, root, func(workspace *Workspace) { workspace.SetSecretStore(store) })
	result, err := browser.evaluate(ctx, page, webSocketClientWalk)
	if err != nil {
		t.Fatal(err)
	}
	var steps struct {
		Opened  string
		Palette struct {
			Title, Timeout string
			Rows           []string
			Password       bool
		}
		Window struct {
			Title            string
			Columns, Buttons []string
		}
		Headers, Dots string
		Saved, Dirty  bool
	}
	if err := json.Unmarshal(result, &steps); err != nil {
		t.Fatalf("the walk answered %s: %v", result, err)
	}
	expect := func(what string, got, want any) {
		t.Helper()
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("%s: %s, want %s", what, gotJSON, wantJSON)
		}
	}
	expect("what «Добавить» opens", steps.Opened, "metadata/websocket-clients/WebSocketКлиент1/МодульКлиента.bsl")
	expect("the palette", []any{steps.Palette.Title, steps.Palette.Rows, steps.Palette.Timeout, steps.Palette.Password},
		[]any{"Свойства: WebSocketКлиент1", []string{"Имя", "Синоним", "Комментарий", "Предопределенный", "Подключать автоматически", "Модуль",
			"URL сервера", "Пользователь", "Пароль", "Заголовки", "Использовать прокси ОС", "Использовать аутентификацию ОС", "Таймаут (секунд)"}, "30", false})
	expect("«Настройка заголовков»", steps.Window, struct {
		Title            string
		Columns, Buttons []string
	}{"Настройка заголовков", []string{"Заголовок", "Значение"}, []string{"Добавить", "Скопировать (недоступна)", "Удалить (недоступна)",
		"Переместить вверх (недоступна)", "Переместить вниз (недоступна)"}})
	expect("the headers", steps.Headers, "X-Version(2)")
	expect("changed by the password", steps.Dirty, true)
	expect("saved", steps.Saved, false)
	expect("the password after the save", steps.Dots, "••••••••")
	client, _ := os.ReadFile(filepath.Join(root, "metadata", "websocket-clients", "WebSocketКлиент1", "object.yaml"))
	if !strings.Contains(string(client), "name: X-Version") || !strings.Contains(string(client), "password_set: true") || strings.Contains(string(client), "секрет") {
		t.Errorf("on disk the client is %s", client)
	}
	var values []string
	for key, value := range store.entries {
		if strings.HasPrefix(key, "websocket.10000000-0000-4000-8000-000000000001.") && strings.HasSuffix(key, ".password") {
			values = append(values, value)
		}
	}
	expect("the store", values, []string{"секрет"})
}
