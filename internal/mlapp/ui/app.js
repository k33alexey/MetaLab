function fieldInputValue(kind, data) {
  if (!data) return "";
  if (kind === "date") return data.length >= 16 ? data.slice(0, 16) : data;
  return data;
}

function readFieldValue(input) {
  const kind = input.dataset.kind || "string";
  if (input.type === "checkbox") return {kind, data: input.checked ? "true" : "false"};
  if (kind === "date") { if (!input.value) return {kind, data: ""}; return {kind, data: (input.value.length === 16 ? input.value + ":00" : input.value) + "Z"}; }
  return {kind, data: input.value};
}

// applyObjectState merges a fetched object's current field/table values into
// a freshly-fetched form shape, in place - the same pattern loadList already
// uses for list rows (table.rows = page.rows), just walking the whole item
// tree since object-form fields are nested inside a group.
function applyObjectState(form, state) {
  const walk = items => {
    for (const item of items || []) {
      if (item.kind === "field" && item.dataPath === "Posted") item.value = state.posted ? "true" : "false";
      else if (item.kind === "field" && item.dataPath && state.fields?.[item.dataPath]) item.value = state.fields[item.dataPath].data;
      if (item.kind === "table" && item.id?.startsWith("table-")) {
        const rows = state.tables?.[item.id.slice(6)] || [];
        item.rows = rows.map(row => ({values: Object.fromEntries(Object.entries(row).map(([key, value]) => [key, value.data]))}));
      }
      walk(item.children);
    }
  };
  walk(form.items);
}

// globalSearchEntries turns what this user may reach into the lines the global
// search offers. It searches METADATA - names of objects and their standard
// commands - not data: "Товар" finds the catalog «Товары» and «Товары: создать»,
// never a particular product. Searching data is a different feature with a
// different cost, and confusing the two is how a search box becomes slow.
//
// Only objects already in the navigation are considered, and those are exactly
// the ones the caller may read; "создать" appears only where the create right
// is among the object's operations, so the search never offers what the answer
// would refuse.
function globalSearchEntries(navigation) {
  const entries = [];
  for (const item of navigation || []) {
    if (item.id === "home" || !item.kind || !item.name) continue;
    entries.push({item, action: "list", title: item.title, caption: item.kindTitle || item.kind});
    if ((item.operations || []).includes("create")) {
      entries.push({item, action: "create", title: `${item.title}: создать`, caption: `${item.kindTitle || item.kind} · команда`});
    }
  }
  return entries;
}

// globalSearchMatches ranks by where the query is found: a name that STARTS
// with what was typed is what the person meant far more often than one that
// merely contains it, and an object outranks its own command so that typing a
// name and pressing Enter opens the list rather than creating something.
function globalSearchMatches(navigation, query, limit = 12) {
  const needle = (query || "").trim().toLocaleLowerCase();
  if (!needle) return [];
  const scored = [];
  for (const entry of globalSearchEntries(navigation)) {
    const haystack = `${entry.title} ${entry.item.name}`.toLocaleLowerCase();
    const at = haystack.indexOf(needle);
    if (at < 0) continue;
    const starts = entry.title.toLocaleLowerCase().startsWith(needle) || entry.item.name.toLocaleLowerCase().startsWith(needle);
    scored.push({entry, rank: (starts ? 0 : 1) * 10 + (entry.action === "list" ? 0 : 1)});
  }
  scored.sort((left, right) => left.rank - right.rank || left.entry.title.localeCompare(right.entry.title));
  return scored.slice(0, limit).map(item => item.entry);
}

// Панель открытых окон ML App: несколько окон приложения внутри одной вкладки
// браузера. Модель отделена от разметки и от сети - она отвечает только на
// вопросы «какие окна открыты», «какое активно» и «что будет, если открыть
// ещё одно», и потому проверяется тестами напрямую.

// MAX_WINDOWS - около десяти одновременно открытых окон. Предел существует не
// ради памяти, а ради самого пользователя: панель из тридцати вкладок перестаёт
// быть навигацией.
const MAX_WINDOWS = 10;

// windowKey опознаёт окно по тому, ЧТО в нём открыто: повторное открытие того
// же списка или той же записи переключает на уже открытое окно, а не плодит
// одинаковые. Новая (ещё не записанная) запись - исключение: каждая такая
// - самостоятельное окно, у них нет общей ссылки.
function windowKey(descriptor) {
  if (descriptor.kind === "home") return "home";
  const object = descriptor.object || {};
  if (descriptor.kind === "object") {
    const reference = descriptor.reference === "new" ? `new:${descriptor.id}` : descriptor.reference;
    return `object/${object.kind}/${object.name}/${reference}`;
  }
  return `list/${object.kind}/${object.name}`;
}

// storedWindows is what survives F5: what was open, not what was typed. The
// panel is restored from it, and each window reloads its own content when it is
// activated - a stale copy of a record would be worse than a short wait.
function storedWindows(windows, activeId) {
  return {
    activeId,
    windows: windows.map(item => ({
      id: item.id, kind: item.kind, title: item.title, reference: item.reference,
      modified: !!item.modified,
      object: item.object ? {kind: item.object.kind, name: item.object.name, title: item.object.title, id: item.object.id} : null,
    })),
  };
}

// restoreWindows accepts only what it can act on. A stored panel from another
// database, or an entry without an address to reopen, is discarded rather than
// shown as a window that cannot be opened.
function restoreWindows(stored) {
  if (!stored || !Array.isArray(stored.windows)) return {windows: [], activeId: null};
  const windows = [];
  for (const item of stored.windows) {
    if (!item || typeof item.id !== "string" || typeof item.kind !== "string") continue;
    if (item.kind !== "home" && (!item.object || !item.object.kind || !item.object.name)) continue;
    if (item.kind === "object" && !item.reference) continue;
    if (windows.some(existing => existing.id === item.id)) continue;
    windows.push({id: item.id, kind: item.kind, title: item.title || "Окно", reference: item.reference, object: item.object, modified: !!item.modified});
    if (windows.length >= MAX_WINDOWS) break;
  }
  const activeId = windows.some(item => item.id === stored.activeId) ? stored.activeId : windows.at(0)?.id || null;
  return {windows, activeId};
}

class MLCommandBar extends HTMLElement {
  set commands(value) { this._commands = Array.isArray(value) ? value : []; this.render(); }
  connectedCallback() { this.setAttribute("role", "toolbar"); this.setAttribute("aria-label", "Команды формы"); this.render(); }
  render() {
    this.replaceChildren();
    for (const command of this._commands || []) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = command.kind === "primary" ? "command primary" : "command";
      button.textContent = command.title;
      button.disabled = Boolean(command.disabled);
      button.dataset.command = command.id;
      button.addEventListener("click", () => this.dispatchEvent(new CustomEvent("ml-command", {bubbles: true, composed: true, detail: {id: command.id}})));
      this.append(button);
    }
  }
}

class MLForm extends HTMLElement {
  set model(value) { this._model = value; this.render(); }
  connectedCallback() { this.render(); }
  render() {
    if (!this._model) return;
    this.replaceChildren();
    const title = document.createElement("h1"); title.textContent = this._model.title;
    const commands = document.createElement("ml-command-bar"); commands.commands = this._model.commands;
    const content = document.createElement("div"); content.className = "form-content";
    for (const item of this._model.items || []) content.append(this.renderElement(item));
    this.append(title, commands, content);
  }
  renderElement(item) {
    if (item.kind === "group") {
      const group = document.createElement("section"); group.className = `form-group ${item.orientation === "horizontal" ? "horizontal" : "vertical"}`;
      if (item.title) { const heading = document.createElement("h2"); heading.textContent = item.title; group.append(heading); }
      const children = document.createElement("div"); children.className = "group-content";
      for (const child of item.children || []) children.append(this.renderElement(child));
      group.append(children); return group;
    }
    if (item.kind === "label") {
      const row = document.createElement("div"); row.className = "form-label";
      const title = document.createElement("span"); title.className = "field-title"; title.textContent = item.title || "";
      const value = document.createElement("span"); value.textContent = item.value || ""; row.append(title, value); return row;
    }
    if (item.kind === "button") {
      const button = document.createElement("button"); button.type = "button"; button.className = "command"; button.textContent = item.title || ""; button.disabled = Boolean(item.disabled);
      button.addEventListener("click", () => this.dispatchEvent(new CustomEvent("ml-command", {bubbles: true, composed: true, detail: {id: item.command}}))); return button;
    }
    if (item.kind === "table") {
      const editable = !this._model.list;
      const table = document.createElement("div"); table.className = "form-table"; table.setAttribute("role", "table"); table.setAttribute("aria-label", item.title || "Таблица");
      if (editable && item.id?.startsWith("table-")) table.dataset.part = item.id.slice(6);
      if (this._model.list?.searchFields?.length) table.addEventListener("contextmenu", event => { event.preventDefault(); this.dispatchEvent(new CustomEvent("ml-advanced-search", {bubbles: true, composed: true})); });
      const header = document.createElement("div"); header.className = "table-header"; header.setAttribute("role", "row");
      for (const column of item.children || []) {
        const cell = document.createElement("strong"); cell.setAttribute("role", "columnheader");
        if (editable) { cell.textContent = column.title || ""; }
        else { const sort = document.createElement("button"); sort.type = "button"; sort.className = "table-sort"; sort.textContent = column.title || ""; sort.addEventListener("click", () => this.dispatchEvent(new CustomEvent("ml-sort", {bubbles: true, composed: true, detail: {field: column.dataPath}}))); cell.append(sort); }
        header.append(cell);
      }
      if (editable) { const spacer = document.createElement("strong"); spacer.setAttribute("role", "columnheader"); header.append(spacer); }
      table.append(header);
      const body = document.createElement("div"); body.className = "table-body"; table.append(body);
      if (editable) {
        const renderRow = values => {
          const rowElement = document.createElement("div"); rowElement.className = "table-row editable"; rowElement.setAttribute("role", "row");
          for (const column of item.children || []) {
            const cell = document.createElement("span"); cell.setAttribute("role", "cell");
            const input = document.createElement("input"); input.type = column.inputType || "text"; input.dataset.column = column.dataPath || ""; input.dataset.kind = column.valueKind || "string";
            input.value = fieldInputValue(column.valueKind, values[column.dataPath]);
            cell.append(input); rowElement.append(cell);
          }
          const removeCell = document.createElement("span"); removeCell.setAttribute("role", "cell");
          const remove = document.createElement("button"); remove.type = "button"; remove.className = "icon-button"; remove.textContent = "✕"; remove.setAttribute("aria-label", "Удалить строку");
          remove.addEventListener("click", () => rowElement.remove());
          removeCell.append(remove); rowElement.append(removeCell);
          body.append(rowElement);
        };
        for (const row of item.rows || []) renderRow(row.values || {});
        const addButton = document.createElement("button"); addButton.type = "button"; addButton.className = "command"; addButton.textContent = "+ Добавить строку";
        addButton.addEventListener("click", () => renderRow({}));
        table.append(addButton);
        return table;
      }
      if (!item.rows?.length) { const empty = document.createElement("p"); empty.className = "empty"; empty.textContent = "Нет данных"; body.append(empty); return table; }
      for (const itemRow of item.rows) {
        const row = document.createElement("div"); row.className = "table-row"; row.setAttribute("role", "row"); row.dataset.reference = itemRow.reference; row.tabIndex = 0;
        for (const column of item.children || []) { const cell = document.createElement("span"); cell.setAttribute("role", "cell"); cell.textContent = itemRow.values?.[column.dataPath] ?? ""; row.append(cell); }
        row.addEventListener("click", () => this.dispatchEvent(new CustomEvent("ml-open-record", {bubbles: true, composed: true, detail: {reference: itemRow.reference}})));
        row.addEventListener("keydown", event => { if (event.key === "Enter") row.click(); });
        body.append(row);
      }
      return table;
    }
    const label = document.createElement("label"); label.className = "form-field";
    const title = document.createElement("span"); title.className = "field-title"; title.textContent = item.title || "";
    const input = document.createElement("input"); input.id = `field-${item.id}`; input.name = item.dataPath || item.id; input.type = item.inputType || "text"; input.dataset.path = item.dataPath || ""; input.dataset.kind = item.valueKind || "string";
    if (item.inputType === "checkbox") input.checked = item.value === "true"; else input.value = fieldInputValue(item.valueKind, item.value);
    input.readOnly = Boolean(item.readOnly); input.disabled = Boolean(item.disabled);
    label.append(title, input); return label;
  }
}

class MLAppShell extends HTMLElement {
  constructor() { super(); this._busy = false; this.navOpen = false; this.windows = []; this.activeWindowId = null;
    this.addEventListener("input", () => this.markWindowModified(true));
    this.addEventListener("change", () => this.markWindowModified(true));
    this.addEventListener("ml-command", event => this.runCommand(event.detail.id)); this.addEventListener("ml-sort", event => this.sortList(event.detail.field)); this.addEventListener("ml-advanced-search", () => this.toggleAdvancedSearch()); this.addEventListener("keydown", event => this.handleNavigationKey(event)); this.addEventListener("ml-open-record", event => this.openObjectRecord(this.currentObject, event.detail.reference)); }
  connectedCallback() { this.load(); }
  async load() {
    const databaseId = location.pathname.split("/").filter(Boolean).at(-1);
    try {
      const response = await fetch(`/api/databases/${encodeURIComponent(databaseId)}/app-bootstrap`, {headers: {"Accept": "application/json"}});
      if (!response.ok) throw new Error(response.status === 401 ? "Требуется вход" : "База недоступна");
      this.databaseId = databaseId; this.bootstrap = await response.json(); this.homeForm = this.bootstrap.form; document.documentElement.lang = this.bootstrap.locale || "ru";
      this.restorePanel(); this.guardUnload(); this.render(); this.startSessionMonitor(databaseId); this.startPublicationWatch(databaseId);
      await this.reopenActiveWindow();
    } catch (error) { this.renderError(error.message); }
  }
  render() {
    const data = this.bootstrap; this.replaceChildren();
    const header = document.createElement("header"); header.className = "app-header";
    const brand = document.createElement("a"); brand.className = "brand"; brand.href = "/"; brand.textContent = "ML"; brand.setAttribute("aria-label", "ML Portal");
    const navToggle = document.createElement("button"); navToggle.type = "button"; navToggle.className = "nav-toggle"; navToggle.textContent = "☰"; navToggle.setAttribute("aria-label", "Открыть разделы"); navToggle.setAttribute("aria-controls", "ml-navigation"); navToggle.setAttribute("aria-expanded", String(this.navOpen)); navToggle.addEventListener("click", () => this.toggleNavigation());
    const database = document.createElement("strong"); database.textContent = data.database.name;
    const search = document.createElement("input"); search.type = "search"; search.placeholder = "Поиск команд и объектов"; search.setAttribute("aria-label", "Глобальный поиск");
    search.autocomplete = "off"; search.setAttribute("role", "combobox"); search.setAttribute("aria-expanded", "false"); search.setAttribute("aria-controls", "ml-global-search-results");
    const results = document.createElement("ul"); results.id = "ml-global-search-results"; results.className = "global-search-results"; results.setAttribute("role", "listbox"); results.hidden = true;
    const searchBox = document.createElement("div"); searchBox.className = "global-search"; searchBox.append(search, results);
    this.globalSearch = {input: search, results, matches: [], active: -1};
    search.addEventListener("input", () => this.updateGlobalSearch());
    search.addEventListener("keydown", event => this.handleGlobalSearchKey(event));
    search.addEventListener("blur", () => setTimeout(() => this.closeGlobalSearch(), 150));
    const user = document.createElement("span"); user.className = "user"; user.textContent = data.user.login; header.append(brand, navToggle, database, searchBox, user);
    const body = document.createElement("div"); body.className = "app-body";
    const nav = document.createElement("nav"); nav.id = "ml-navigation"; nav.className = "app-nav"; nav.setAttribute("aria-label", "Разделы");
    for (const item of data.navigation || []) { const selected = this.currentObject ? item.id === this.currentObject.id : item.id === "home"; const button = document.createElement("button"); button.type = "button"; button.textContent = item.title; button.className = selected ? "current" : ""; if (selected) button.setAttribute("aria-current", "page"); button.addEventListener("click", () => { this.closeNavigation(); item.id === "home" ? this.openHome() : this.openForm(item, "list"); }); nav.append(button); }
    const backdrop = document.createElement("button"); backdrop.type = "button"; backdrop.className = "nav-backdrop"; backdrop.setAttribute("aria-label", "Закрыть разделы"); backdrop.addEventListener("click", () => this.closeNavigation());
    const main = document.createElement("main"); main.id = "ml-workspace"; main.className = "workspace"; main.tabIndex = -1;
    const tabs = this.renderWindowTabs();
    const form = document.createElement("ml-form"); form.model = data.form; main.append(tabs); if (data.form.list && this.listState) main.append(this.renderListControls()); main.append(form); body.append(nav, backdrop, main); this.append(header, body); this.classList.toggle("navigation-open", this.navOpen);
  }
  renderListControls() {
    const state = this.listState, options = this.bootstrap.form.list;
    const controls = document.createElement("section"); controls.className = "list-controls"; controls.setAttribute("aria-label", "Управление списком");
    if (options.searchEnabled) {
      const search = document.createElement("input"); search.type = "search"; search.value = state.search; search.placeholder = "Поиск в списке"; search.setAttribute("aria-label", "Поиск в списке");
      search.addEventListener("input", () => { state.search = search.value; });
      search.addEventListener("keydown", event => { if (event.key === "Enter") { state.search = search.value; this.loadList(true); } });
      const find = document.createElement("button"); find.type = "button"; find.className = "command"; find.textContent = "Найти"; find.addEventListener("click", () => { state.search = search.value; this.loadList(true); });
      controls.append(search, find);
      if (options.searchFields?.length) {
        const advanced = document.createElement("button"); advanced.type = "button"; advanced.className = "command"; advanced.textContent = "Расширенный поиск"; advanced.setAttribute("aria-expanded", String(state.advancedVisible)); advanced.addEventListener("click", () => this.toggleAdvancedSearch()); controls.append(advanced);
        if (state.advancedVisible) {
          const searchField = document.createElement("select"); searchField.setAttribute("aria-label", "Поле расширенного поиска");
          const all = document.createElement("option"); all.value = ""; all.textContent = "Все поля"; searchField.append(all);
          for (const item of options.searchFields) { const option = document.createElement("option"); option.value = item.name; option.textContent = item.title; option.selected = state.searchField === item.name; searchField.append(option); }
          searchField.addEventListener("change", () => { state.searchField = searchField.value; }); controls.append(searchField);
        }
      }
    }
    if (options.filterFields?.length) {
      const field = document.createElement("select"); field.setAttribute("aria-label", "Поле отбора");
      for (const item of options.filterFields) { const option = document.createElement("option"); option.value = item.name; option.textContent = item.title; field.append(option); }
      const value = document.createElement("input"); value.type = "text"; value.placeholder = "Значение отбора"; value.setAttribute("aria-label", "Значение отбора");
      const apply = document.createElement("button"); apply.type = "button"; apply.className = "command"; apply.textContent = "Отобрать"; apply.addEventListener("click", () => { state.filters = state.filters.filter(item => item.field !== field.value); state.filters.push({field: field.value, value: value.value}); this.loadList(true); });
      controls.append(field, value, apply);
    }
    if (state.filters.length) { const clear = document.createElement("button"); clear.type = "button"; clear.className = "command"; clear.textContent = `Сбросить отбор (${state.filters.length})`; clear.addEventListener("click", () => { state.filters = []; this.loadList(true); }); controls.append(clear); }
    const spacer = document.createElement("span"); spacer.className = "list-spacer"; controls.append(spacer);
    const size = document.createElement("select"); size.setAttribute("aria-label", "Строк на странице");
    for (const count of [20, 50, 100]) { const option = document.createElement("option"); option.value = String(count); option.textContent = String(count); option.selected = state.limit === count; size.append(option); }
    size.addEventListener("change", () => { state.limit = Number(size.value); this.loadList(true); });
    const previous = document.createElement("button"); previous.type = "button"; previous.className = "command"; previous.textContent = "Назад"; previous.disabled = !state.history.length; previous.addEventListener("click", () => this.previousListPage());
    const next = document.createElement("button"); next.type = "button"; next.className = "command"; next.textContent = "Далее"; next.disabled = !state.nextCursor; next.addEventListener("click", () => this.nextListPage());
    controls.append(size, previous, next); return controls;
  }
  async runCommand(id) {
    if (this._busy) return; this._busy = true;
    try {
      if (id === "refresh" || id === "Refresh") { if (this.listState) await this.loadList(false); else if (this.currentReference !== undefined) await this.openObjectRecord(this.currentObject, this.currentReference); else if (this.currentObject) await this.openForm(this.currentObject, this.currentFormKind); else await this.load(); }
      else if (id === "Create" && this.currentObject) await this.openObjectRecord(this.currentObject, "new");
      else if (id === "Close" && this.currentObject) await this.closeWindow(this.activeWindowId);
      else if ((id === "Save" || id === "SaveAndClose") && this.currentObject) await this.saveCurrentObject(id === "SaveAndClose");
      else if (id === "Post" && this.currentObject && this.currentReference && this.currentReference !== "new") await this.postCurrentDocument();
      else if (id === "UndoPosting" && this.currentObject && this.currentReference && this.currentReference !== "new") await this.undoCurrentDocumentPosting();
      else if (id === "SetDeletionMark" && this.currentObject && this.currentReference && this.currentReference !== "new") await this.toggleCurrentDeletionMark();
      else this.announce(`Команда «${id}» требует выбранного объекта`);
    }
    finally { this._busy = false; }
  }
  updateGlobalSearch() {
    const search = this.globalSearch;
    if (!search) return;
    search.matches = globalSearchMatches(this.bootstrap.navigation, search.input.value);
    search.active = search.matches.length ? 0 : -1;
    this.renderGlobalSearch();
  }
  renderGlobalSearch() {
    const search = this.globalSearch;
    search.results.replaceChildren();
    const query = search.input.value.trim();
    if (!query) { search.results.hidden = true; search.input.setAttribute("aria-expanded", "false"); return; }
    if (!search.matches.length) {
      const empty = document.createElement("li"); empty.className = "global-search-empty"; empty.textContent = "Ничего не найдено";
      search.results.append(empty);
    }
    search.matches.forEach((entry, index) => {
      const row = document.createElement("li"); row.setAttribute("role", "option"); row.setAttribute("aria-selected", String(index === search.active));
      row.className = index === search.active ? "global-search-row current" : "global-search-row";
      const title = document.createElement("span"); title.className = "global-search-title"; title.textContent = entry.title;
      const caption = document.createElement("span"); caption.className = "global-search-caption"; caption.textContent = entry.caption;
      row.append(title, caption);
      row.addEventListener("mousedown", event => { event.preventDefault(); this.runGlobalSearch(entry); });
      search.results.append(row);
    });
    search.results.hidden = false; search.input.setAttribute("aria-expanded", "true");
  }
  handleGlobalSearchKey(event) {
    const search = this.globalSearch;
    if (!search || search.results.hidden) { if (event.key === "Escape") this.closeGlobalSearch(); return; }
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      if (!search.matches.length) return;
      const step = event.key === "ArrowDown" ? 1 : -1;
      search.active = (search.active + step + search.matches.length) % search.matches.length;
      this.renderGlobalSearch();
      return;
    }
    if (event.key === "Enter") { event.preventDefault(); const entry = search.matches[search.active]; if (entry) this.runGlobalSearch(entry); return; }
    if (event.key === "Escape") { event.preventDefault(); this.closeGlobalSearch(); }
  }
  closeGlobalSearch() {
    const search = this.globalSearch;
    if (!search) return;
    search.results.hidden = true; search.results.replaceChildren(); search.matches = []; search.active = -1;
    search.input.setAttribute("aria-expanded", "false");
  }
  async runGlobalSearch(entry) {
    this.closeGlobalSearch();
    this.globalSearch.input.value = "";
    this.closeNavigation();
    if (entry.action === "create") { await this.openObjectRecord(entry.item, "new"); return; }
    await this.openForm(entry.item, "list");
  }
  // --- панель открытых окон -------------------------------------------------
  panelStorageKey() { return `ml-app-windows:${this.databaseId}`; }
  savePanel() {
    this.captureActiveWindow();
    try { sessionStorage.setItem(this.panelStorageKey(), JSON.stringify(storedWindows(this.windows || [], this.activeWindowId))); }
    catch { /* приватный режим или переполнение - панель просто не переживёт F5 */ }
  }
  restorePanel() {
    let stored = null;
    try { stored = JSON.parse(sessionStorage.getItem(this.panelStorageKey()) || "null"); } catch { stored = null; }
    const restored = restoreWindows(stored);
    this.windows = restored.windows; this.activeWindowId = restored.activeId;
    if (!this.windows.length) {
      this.windows = [{id: "home", kind: "home", title: this.homeForm.title, object: null, reference: null, modified: false}];
      this.activeWindowId = "home";
    }
  }
  activeWindow() { return (this.windows || []).find(item => item.id === this.activeWindowId) || null; }
  // captureActiveWindow stores what is on screen INTO the window it belongs to,
  // so switching away and back does not refetch or lose the list page, the
  // sorting or the record being edited.
  captureActiveWindow() {
    const active = this.activeWindow();
    if (!active) return;
    active.object = this.currentObject; active.reference = this.currentReference ?? null;
    active.form = this.bootstrap.form; active.objectState = this.objectState; active.listState = this.listState;
    active.formKind = this.currentFormKind;
  }
  applyWindow(item) {
    this.activeWindowId = item.id;
    this.currentObject = item.object || null; this.currentFormKind = item.formKind || (item.kind === "object" ? "object" : item.kind === "list" ? "list" : null);
    this.currentReference = item.kind === "object" ? item.reference : undefined;
    this.objectState = item.objectState || null; this.listState = item.listState || null;
    this.bootstrap.form = item.form || (item.kind === "home" ? this.homeForm : this.bootstrap.form);
  }
  async activateWindow(id) {
    if (id === this.activeWindowId) return;
    const target = (this.windows || []).find(item => item.id === id);
    if (!target) return;
    this.captureActiveWindow();
    this.applyWindow(target);
    this.savePanel();
    if (!target.form) { await this.reopenWindow(target); return; }
    this.render();
  }
  // reopenWindow loads a window that has an address but no content yet - after
  // F5, or after it was opened in the background.
  async reopenWindow(item) {
    if (item.kind === "home") { this.applyWindow(item); this.bootstrap.form = this.homeForm; this.render(); this.savePanel(); return; }
    if (item.kind === "object") { await this.openObjectRecord(item.object, item.reference, item.id); return; }
    await this.openForm(item.object, "list", item.id);
  }
  async reopenActiveWindow() {
    const active = this.activeWindow();
    if (!active || active.form) return;
    await this.reopenWindow(active);
  }
  // openWindow is the single door: it reuses a window that already shows the
  // same thing, and refuses to exceed the limit without asking - the choice of
  // what to close belongs to the person, not to a least-recently-used rule.
  async openWindow(descriptor, load) {
    const key = windowKey({...descriptor, id: descriptor.id || ""});
    const existing = (this.windows || []).find(item => windowKey(item) === key);
    if (existing) { this.captureActiveWindow(); this.applyWindow(existing); this.savePanel(); if (!existing.form) { await this.reopenWindow(existing); return existing; } this.render(); return existing; }
    if ((this.windows || []).length >= MAX_WINDOWS) { this.askWhichWindowToClose(descriptor, load); return null; }
    const item = {id: descriptor.id || `w${Date.now()}${Math.random().toString(36).slice(2, 6)}`, kind: descriptor.kind, title: descriptor.title, object: descriptor.object, reference: descriptor.reference ?? null, modified: false};
    this.captureActiveWindow();
    this.windows.push(item); this.activeWindowId = item.id;
    await load(item);
    this.savePanel();
    return item;
  }
  renderWindowTabs() {
    const tabs = document.createElement("div"); tabs.className = "window-tabs"; tabs.setAttribute("role", "tablist");
    for (const item of this.windows || []) {
      const tab = document.createElement("button"); tab.type = "button"; tab.setAttribute("role", "tab");
      const current = item.id === this.activeWindowId;
      tab.className = current ? "window-tab current" : "window-tab"; tab.setAttribute("aria-selected", String(current));
      const title = document.createElement("span"); title.textContent = (item.modified ? "• " : "") + (item.title || "Окно");
      if (item.modified) tab.title = "Есть несохранённые изменения";
      tab.append(title);
      tab.addEventListener("click", () => this.activateWindow(item.id));
      if (item.kind !== "home") {
        const close = document.createElement("span"); close.className = "window-tab-close"; close.textContent = "×"; close.setAttribute("role", "button"); close.setAttribute("aria-label", `Закрыть окно ${item.title || ""}`.trim());
        close.addEventListener("click", event => { event.stopPropagation(); this.closeWindow(item.id); });
        tab.append(close);
      }
      tabs.append(tab);
    }
    return tabs;
  }
  async openHome() {
    await this.openWindow({kind: "home", id: "home", title: this.homeForm.title, object: null, reference: null}, async () => {
      this.currentObject = null; this.currentFormKind = null; this.currentReference = undefined; this.objectState = null; this.listState = null;
      this.bootstrap.form = this.homeForm; this.render();
    });
  }
  async openObjectRecord(item, reference, windowID) {
    if (!windowID) {
      const title = reference === "new" ? `${item.title}: новая` : item.title;
      await this.openWindow({kind: "object", object: item, reference, title}, async created => this.openObjectRecord(item, reference, created.id));
      return;
    }
    const formResponse = await fetch(`/api/databases/${encodeURIComponent(this.databaseId)}/forms/${encodeURIComponent(item.kind)}/${encodeURIComponent(item.name)}/object`, {headers: {"Accept": "application/json"}});
    if (!formResponse.ok) { this.announce("Не удалось открыть форму"); return; }
    const stateResponse = await fetch(`/api/databases/${encodeURIComponent(this.databaseId)}/objects/${encodeURIComponent(item.kind)}/${encodeURIComponent(item.name)}/${encodeURIComponent(reference)}`, {headers: {"Accept": "application/json"}});
    if (!stateResponse.ok) { this.announce("Не удалось открыть запись"); return; }
    const form = await formResponse.json(), state = await stateResponse.json();
    applyObjectState(form, state);
    // state.reference is only really persisted when we weren't creating -
    // GetApplicationObject pre-allocates a reference for a new object
    // without saving it, so Save must still see "new", not that reference.
    this.currentObject = item; this.currentFormKind = "object"; this.currentReference = reference === "new" ? "new" : state.reference;
    this.objectState = state; this.listState = null;
    this.bootstrap.form = form; this.activeWindowId = windowID;
    const target = (this.windows || []).find(entry => entry.id === windowID);
    if (target) { target.kind = "object"; target.object = item; target.reference = this.currentReference; target.title = reference === "new" ? `${item.title}: новая` : form.title || item.title; target.modified = false; }
    this.render(); this.savePanel();
  }
  // closeWindow never discards work quietly: a window with unsaved changes asks
  // what to do with them, and "Отмена" means the window stays open.
  async closeWindow(id, options = {}) {
    const index = (this.windows || []).findIndex(item => item.id === id);
    if (index < 0) return false;
    const item = this.windows[index];
    if (item.kind === "home") return false;
    if (item.modified && !options.discard) {
      const decision = await this.askUnsavedDecision(item);
      if (decision === "cancel") return false;
      if (decision === "save") {
        const saved = await this.saveWindow(item);
        if (!saved) return false;
      }
    }
    const wasActive = item.id === this.activeWindowId;
    this.windows.splice(index, 1);
    if (!this.windows.length) this.windows = [{id: "home", kind: "home", title: this.homeForm.title, object: null, reference: null, modified: false}];
    if (wasActive) {
      const next = this.windows[Math.min(index, this.windows.length - 1)];
      this.activeWindowId = next.id;
      this.applyWindow(next);
      if (!next.form && next.kind !== "home") { await this.reopenWindow(next); return true; }
      if (next.kind === "home") this.bootstrap.form = this.homeForm;
    }
    this.render(); this.savePanel();
    return true;
  }
  // saveWindow saves a window that is not on screen by switching to it first:
  // the values being saved are the ones in the form, and the form is the DOM.
  async saveWindow(item) {
    if (item.id !== this.activeWindowId) {
      await this.activateWindow(item.id);
      if (item.id !== this.activeWindowId) return false;
    }
    const before = this.objectState?.reference;
    await this.saveCurrentObject(false);
    const stillModified = (this.windows || []).find(entry => entry.id === item.id)?.modified;
    return !stillModified || this.objectState?.reference !== before;
  }
  markWindowModified(modified) {
    const active = this.activeWindow();
    if (!active || active.kind !== "object" || active.modified === modified) return;
    active.modified = modified;
    const tabs = this.querySelector(".window-tabs");
    if (tabs) tabs.replaceWith(this.renderWindowTabs());
    this.savePanel();
  }
  // askUnsavedDecision offers exactly the three answers the situation has.
  askUnsavedDecision(item) {
    return new Promise(resolve => {
      const dialog = document.createElement("dialog"); dialog.className = "window-dialog";
      const title = document.createElement("h2"); title.textContent = "Несохранённые изменения";
      const text = document.createElement("p"); text.textContent = `В окне «${item.title}» есть изменения, которые не сохранены.`;
      const actions = document.createElement("div"); actions.className = "window-dialog-actions";
      for (const [answer, label, className] of [["save", "Сохранить", "primary"], ["discard", "Не сохранять", "secondary"], ["cancel", "Отмена", "secondary"]]) {
        const button = document.createElement("button"); button.type = "button"; button.className = className; button.textContent = label;
        button.addEventListener("click", () => { dialog.close(); dialog.remove(); resolve(answer); });
        actions.append(button);
      }
      dialog.addEventListener("cancel", event => { event.preventDefault(); dialog.close(); dialog.remove(); resolve("cancel"); });
      dialog.append(title, text, actions); this.append(dialog); dialog.showModal();
    });
  }
  // askWhichWindowToClose is what happens at the limit: nothing is closed
  // automatically and nothing is opened behind the person's back - they see
  // what is open, which of it is unsaved, and decide.
  askWhichWindowToClose(descriptor, load) {
    const dialog = document.createElement("dialog"); dialog.className = "window-dialog";
    const title = document.createElement("h2"); title.textContent = "Открыто предельное число окон";
    const text = document.createElement("p"); text.textContent = `Одновременно может быть открыто не больше ${MAX_WINDOWS} окон. Закройте одно, чтобы открыть новое.`;
    const list = document.createElement("ul"); list.className = "window-dialog-list";
    for (const item of this.windows || []) {
      if (item.kind === "home") continue;
      const row = document.createElement("li");
      const name = document.createElement("span"); name.textContent = item.title || "Окно";
      const mark = document.createElement("span"); mark.className = "window-dialog-mark"; mark.textContent = item.modified ? "есть несохранённые изменения" : "";
      const close = document.createElement("button"); close.type = "button"; close.className = "secondary"; close.textContent = "Закрыть";
      close.addEventListener("click", async () => {
        if (!await this.closeWindow(item.id)) return;
        dialog.close(); dialog.remove();
        await this.openWindow(descriptor, load);
      });
      row.append(name, mark, close); list.append(row);
    }
    const actions = document.createElement("div"); actions.className = "window-dialog-actions";
    const cancel = document.createElement("button"); cancel.type = "button"; cancel.className = "secondary"; cancel.textContent = "Отмена";
    cancel.addEventListener("click", () => { dialog.close(); dialog.remove(); });
    actions.append(cancel);
    dialog.addEventListener("cancel", event => { event.preventDefault(); dialog.close(); dialog.remove(); });
    dialog.append(title, text, list, actions); this.append(dialog); dialog.showModal();
  }
  collectObjectFormValues() {
    // A blank input means "leave unset" (e.g. an auto-generated Код/Номер),
    // not "set to empty string" - SetObjectProperty already rejects an
    // explicit empty catalog code outright, since that's exactly the signal
    // it uses to decide whether to auto-generate one at save time.
    const fields = {};
    for (const input of this.querySelectorAll(".form-field input[data-path]")) {
      if (!input.dataset.path) continue;
      const value = readFieldValue(input);
      if (value.data !== "") fields[input.dataset.path] = value;
    }
    const tables = {};
    for (const tableElement of this.querySelectorAll(".form-table[data-part]")) {
      const rows = [];
      for (const rowElement of tableElement.querySelectorAll(".table-row.editable")) {
        const row = {};
        for (const input of rowElement.querySelectorAll("input[data-column]")) {
          if (!input.dataset.column) continue;
          const value = readFieldValue(input);
          if (value.data !== "") row[input.dataset.column] = value;
        }
        rows.push(row);
      }
      tables[tableElement.dataset.part] = rows;
    }
    return {fields, tables};
  }
  async saveCurrentObject(close) {
    const payload = this.collectObjectFormValues(); payload.reference = this.currentReference || "new";
    const response = await fetch(`/api/databases/${encodeURIComponent(this.databaseId)}/objects/${encodeURIComponent(this.currentObject.kind)}/${encodeURIComponent(this.currentObject.name)}/save`, {
      method: "POST", headers: {"Content-Type": "application/json", "X-ML-CSRF": "1"}, body: JSON.stringify(payload),
    });
    const text = await response.text();
    if (!response.ok) { this.announce(text); return; }
    if (close) { this.markWindowModified(false); await this.closeWindow(this.activeWindowId, {discard: true}); return; }
    const state = JSON.parse(text);
    await this.openObjectRecord(this.currentObject, state.reference, this.activeWindowId);
    this.markWindowModified(false);
    this.announce("Сохранено");
  }
  async postCurrentDocument() {
    const response = await fetch(`/api/databases/${encodeURIComponent(this.databaseId)}/documents/${encodeURIComponent(this.currentObject.name)}/post`, {
      method: "POST", headers: {"Content-Type": "application/json", "X-ML-CSRF": "1"}, body: JSON.stringify({reference: this.currentReference}),
    });
    const text = await response.text();
    if (!response.ok) { this.announce(text); return; }
    await this.openObjectRecord(this.currentObject, this.currentReference);
    this.announce("Проведён");
  }
  async undoCurrentDocumentPosting() {
    const response = await fetch(`/api/databases/${encodeURIComponent(this.databaseId)}/documents/${encodeURIComponent(this.currentObject.name)}/undo-posting`, {
      method: "POST", headers: {"Content-Type": "application/json", "X-ML-CSRF": "1"}, body: JSON.stringify({reference: this.currentReference}),
    });
    const text = await response.text();
    if (!response.ok) { this.announce(text); return; }
    await this.openObjectRecord(this.currentObject, this.currentReference);
    this.announce("Проведение отменено");
  }
  async toggleCurrentDeletionMark() {
    const mark = !this.objectState?.deletionMark;
    const response = await fetch(`/api/databases/${encodeURIComponent(this.databaseId)}/objects/${encodeURIComponent(this.currentObject.kind)}/${encodeURIComponent(this.currentObject.name)}/deletion-mark`, {
      method: "POST", headers: {"Content-Type": "application/json", "X-ML-CSRF": "1"}, body: JSON.stringify({reference: this.currentReference, mark}),
    });
    const text = await response.text();
    if (!response.ok) { this.announce(text); return; }
    await this.openObjectRecord(this.currentObject, this.currentReference);
    this.announce(mark ? "Помечен на удаление" : "Пометка на удаление снята");
  }
  async openForm(item, formKind, windowID) {
    if (!windowID) {
      await this.openWindow({kind: formKind === "list" ? "list" : "object", object: item, reference: null, title: item.title},
        async created => this.openForm(item, formKind, created.id));
      return;
    }
    const response = await fetch(`/api/databases/${encodeURIComponent(this.databaseId)}/forms/${encodeURIComponent(item.kind)}/${encodeURIComponent(item.name)}/${formKind}`, {headers: {"Accept": "application/json"}});
    if (!response.ok) { this.announce("Не удалось открыть форму"); return; }
    this.currentObject = item; this.currentFormKind = formKind; this.bootstrap.form = await response.json();
    this.activeWindowId = windowID;
    const target = (this.windows || []).find(entry => entry.id === windowID);
    if (target) { target.object = item; target.reference = null; target.title = this.bootstrap.form.title || item.title; target.modified = false; }
    if (this.bootstrap.form.list) {
      this.listState = {limit: this.bootstrap.form.list.pageSize, search: "", searchField: "", advancedVisible: false, sort: "", descending: false, filters: [], cursor: null, nextCursor: null, history: []};
      await this.loadList(true); this.savePanel(); return;
    }
    this.listState = null; this.render(); this.savePanel();
  }
  async loadList(reset) {
    const state = this.listState; if (!state || !this.currentObject) return;
    if (reset) { state.cursor = null; state.nextCursor = null; state.history = []; }
    const query = new URLSearchParams({limit: String(state.limit)}); if (state.cursor) query.set("cursor", state.cursor); if (state.search) query.set("search", state.search); if (state.searchField) query.set("searchField", state.searchField); if (state.sort) { query.set("sort", state.sort); query.set("direction", state.descending ? "desc" : "asc"); }
    for (const filter of state.filters) query.append("filter", `${filter.field}=${filter.value}`);
    const response = await fetch(`/api/databases/${encodeURIComponent(this.databaseId)}/lists/${encodeURIComponent(this.currentObject.kind)}/${encodeURIComponent(this.currentObject.name)}?${query}`, {headers: {"Accept": "application/json"}});
    if (!response.ok) { this.announce("Не удалось загрузить список"); return; }
    const page = await response.json(); state.nextCursor = page.nextCursor || null; state.limit = page.pageSize;
    const table = this.findListTable(this.bootstrap.form.items || []); if (table) table.rows = page.rows || []; this.render();
  }
  findListTable(items) { for (const item of items) { if (item.kind === "table") return item; const nested = this.findListTable(item.children || []); if (nested) return nested; } return null; }
  async nextListPage() { if (!this.listState?.nextCursor) return; this.listState.history.push(this.listState.cursor); this.listState.cursor = this.listState.nextCursor; await this.loadList(false); }
  async previousListPage() { if (!this.listState?.history.length) return; this.listState.cursor = this.listState.history.pop(); await this.loadList(false); }
  async sortList(field) { if (!this.listState || !field) return; if (this.listState.sort === field) this.listState.descending = !this.listState.descending; else { this.listState.sort = field; this.listState.descending = false; } await this.loadList(true); }
  toggleAdvancedSearch() { if (!this.listState || !this.bootstrap.form.list?.searchFields?.length) return; this.listState.advancedVisible = !this.listState.advancedVisible; if (!this.listState.advancedVisible) this.listState.searchField = ""; this.render(); }
  toggleNavigation() { this.navOpen = !this.navOpen; this.classList.toggle("navigation-open", this.navOpen); const toggle = this.querySelector(".nav-toggle"); toggle?.setAttribute("aria-expanded", String(this.navOpen)); toggle?.setAttribute("aria-label", this.navOpen ? "Закрыть разделы" : "Открыть разделы"); if (this.navOpen) this.querySelector(".app-nav button")?.focus(); }
  closeNavigation() { if (!this.navOpen) return; this.navOpen = false; this.classList.remove("navigation-open"); const toggle = this.querySelector(".nav-toggle"); toggle?.setAttribute("aria-expanded", "false"); toggle?.setAttribute("aria-label", "Открыть разделы"); toggle?.focus(); }
  handleNavigationKey(event) {
    if (!this.navOpen) return;
    if (event.key === "Escape") { event.preventDefault(); this.closeNavigation(); return; }
    if (event.key !== "Tab") return;
    const items = [...this.querySelectorAll(".app-nav button")]; if (!items.length) return;
    const first = items[0], last = items.at(-1);
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
  }
  announce(message) {
    let status = this.querySelector(".app-status"); if (!status) { status = document.createElement("div"); status.className = "app-status"; status.setAttribute("role", "status"); this.append(status); }
    status.textContent = message;
  }
  renderError(message) { this.replaceChildren(); const panel = document.createElement("main"); panel.className = "error-panel"; const title = document.createElement("h1"); title.textContent = "ML App"; const text = document.createElement("p"); text.textContent = message; const back = document.createElement("a"); back.href = "/"; back.textContent = "Вернуться в Portal"; panel.append(title, text, back); this.append(panel); }
  // startPublicationWatch notices that the configuration was saved while this
  // session is open. It does NOT reload anything: an open form may hold
  // unsaved work, and taking that decision away from the person is exactly
  // what "безопасное обновление" must not do.
  startPublicationWatch(databaseId) {
    clearInterval(this._publicationWatch);
    const read = async () => {
      try {
        const response = await fetch(`/api/databases/${encodeURIComponent(databaseId)}/publication`, {headers: {"Accept": "application/json"}});
        if (!response.ok) return null;
        const marker = await response.json();
        return marker && marker.contentSha256 ? marker.contentSha256 : null;
      } catch { return null; }
    };
    read().then(marker => { this._publication = marker; });
    this._publicationWatch = setInterval(async () => {
      const marker = await read();
      if (!marker || !this._publication) { this._publication = this._publication || marker; return; }
      if (marker === this._publication) return;
      this._publication = marker;
      this.showUpdateBanner();
    }, 30000);
  }
  showUpdateBanner() {
    if (this.querySelector(".app-update")) return;
    const banner = document.createElement("div"); banner.className = "app-update"; banner.setAttribute("role", "status");
    const text = document.createElement("span"); text.textContent = "Конфигурация обновлена. Открытые окна продолжают работать на прежней версии.";
    const refresh = document.createElement("button"); refresh.type = "button"; refresh.className = "primary"; refresh.textContent = "Обновить";
    refresh.addEventListener("click", () => location.reload());
    const later = document.createElement("button"); later.type = "button"; later.className = "secondary"; later.textContent = "Позже";
    later.addEventListener("click", () => banner.remove());
    banner.append(text, refresh, later);
    this.prepend(banner);
  }
  startSessionMonitor(databaseId) {
    clearInterval(this._monitor); this._monitor = setInterval(async () => {
      const response = await fetch(`/api/databases/${encodeURIComponent(databaseId)}/session`);
      if (!response.ok) { clearInterval(this._monitor); location.href = "/"; return; }
      const session = await response.json();
      if (session.message) { this.announce(session.message); await fetch(`/api/sessions/${session.id}/message/ack`, {method: "POST", headers: {"X-ML-CSRF": "1"}}); }
    }, 5000);
  }
  // The browser's own confirmation is the only guard that works for closing a
  // tab; it is asked for only when something would actually be lost.
  guardUnload() {
    if (this._unloadGuard) return;
    this._unloadGuard = event => {
      if (!(this.windows || []).some(item => item.modified)) return;
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", this._unloadGuard);
  }
  disconnectedCallback() { clearInterval(this._monitor); clearInterval(this._publicationWatch); if (this._unloadGuard) window.removeEventListener("beforeunload", this._unloadGuard); }
}

customElements.define("ml-command-bar", MLCommandBar);
customElements.define("ml-form", MLForm);
customElements.define("ml-app-shell", MLAppShell);
