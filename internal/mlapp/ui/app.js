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
  constructor() { super(); this._busy = false; this.navOpen = false; this.addEventListener("ml-command", event => this.runCommand(event.detail.id)); this.addEventListener("ml-sort", event => this.sortList(event.detail.field)); this.addEventListener("ml-advanced-search", () => this.toggleAdvancedSearch()); this.addEventListener("keydown", event => this.handleNavigationKey(event)); this.addEventListener("ml-open-record", event => this.openObjectRecord(this.currentObject, event.detail.reference)); }
  connectedCallback() { this.load(); }
  async load() {
    const databaseId = location.pathname.split("/").filter(Boolean).at(-1);
    try {
      const response = await fetch(`/api/databases/${encodeURIComponent(databaseId)}/app-bootstrap`, {headers: {"Accept": "application/json"}});
      if (!response.ok) throw new Error(response.status === 401 ? "Требуется вход" : "База недоступна");
      this.databaseId = databaseId; this.bootstrap = await response.json(); this.homeForm = this.bootstrap.form; document.documentElement.lang = this.bootstrap.locale || "ru"; this.render(); this.startSessionMonitor(databaseId);
    } catch (error) { this.renderError(error.message); }
  }
  render() {
    const data = this.bootstrap; this.replaceChildren();
    const header = document.createElement("header"); header.className = "app-header";
    const brand = document.createElement("a"); brand.className = "brand"; brand.href = "/"; brand.textContent = "ML"; brand.setAttribute("aria-label", "ML Portal");
    const navToggle = document.createElement("button"); navToggle.type = "button"; navToggle.className = "nav-toggle"; navToggle.textContent = "☰"; navToggle.setAttribute("aria-label", "Открыть разделы"); navToggle.setAttribute("aria-controls", "ml-navigation"); navToggle.setAttribute("aria-expanded", String(this.navOpen)); navToggle.addEventListener("click", () => this.toggleNavigation());
    const database = document.createElement("strong"); database.textContent = data.database.name;
    const search = document.createElement("input"); search.type = "search"; search.placeholder = "Поиск команд и объектов"; search.setAttribute("aria-label", "Глобальный поиск"); search.disabled = true;
    const user = document.createElement("span"); user.className = "user"; user.textContent = data.user.login; header.append(brand, navToggle, database, search, user);
    const body = document.createElement("div"); body.className = "app-body";
    const nav = document.createElement("nav"); nav.id = "ml-navigation"; nav.className = "app-nav"; nav.setAttribute("aria-label", "Разделы");
    for (const item of data.navigation || []) { const selected = this.currentObject ? item.id === this.currentObject.id : item.id === "home"; const button = document.createElement("button"); button.type = "button"; button.textContent = item.title; button.className = selected ? "current" : ""; if (selected) button.setAttribute("aria-current", "page"); button.addEventListener("click", () => { this.closeNavigation(); item.id === "home" ? this.openHome() : this.openForm(item, "list"); }); nav.append(button); }
    const backdrop = document.createElement("button"); backdrop.type = "button"; backdrop.className = "nav-backdrop"; backdrop.setAttribute("aria-label", "Закрыть разделы"); backdrop.addEventListener("click", () => this.closeNavigation());
    const main = document.createElement("main"); main.id = "ml-workspace"; main.className = "workspace"; main.tabIndex = -1;
    const tabs = document.createElement("div"); tabs.className = "window-tabs"; tabs.setAttribute("role", "tablist");
    const tab = document.createElement("button"); tab.type = "button"; tab.className = "window-tab current"; tab.setAttribute("role", "tab"); tab.setAttribute("aria-selected", "true"); tab.textContent = data.form.title; tabs.append(tab);
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
      else if (id === "Close" && this.currentObject) { this.currentReference = undefined; this.objectState = null; await this.openForm(this.currentObject, "list"); }
      else if ((id === "Save" || id === "SaveAndClose") && this.currentObject) await this.saveCurrentObject(id === "SaveAndClose");
      else if (id === "Post" && this.currentObject && this.currentReference && this.currentReference !== "new") await this.postCurrentDocument();
      else if (id === "UndoPosting" && this.currentObject && this.currentReference && this.currentReference !== "new") await this.undoCurrentDocumentPosting();
      else if (id === "SetDeletionMark" && this.currentObject && this.currentReference && this.currentReference !== "new") await this.toggleCurrentDeletionMark();
      else this.announce(`Команда «${id}» требует выбранного объекта`);
    }
    finally { this._busy = false; }
  }
  openHome() { this.currentObject = null; this.currentFormKind = null; this.currentReference = undefined; this.objectState = null; this.listState = null; this.bootstrap.form = this.homeForm; this.render(); }
  async openObjectRecord(item, reference) {
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
    this.bootstrap.form = form; this.render();
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
    if (close) { this.currentReference = undefined; this.objectState = null; await this.openForm(this.currentObject, "list"); return; }
    const state = JSON.parse(text);
    await this.openObjectRecord(this.currentObject, state.reference);
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
  async openForm(item, formKind) {
    const response = await fetch(`/api/databases/${encodeURIComponent(this.databaseId)}/forms/${encodeURIComponent(item.kind)}/${encodeURIComponent(item.name)}/${formKind}`, {headers: {"Accept": "application/json"}});
    if (!response.ok) { this.announce("Не удалось открыть форму"); return; }
    this.currentObject = item; this.currentFormKind = formKind; this.bootstrap.form = await response.json();
    if (this.bootstrap.form.list) {
      this.listState = {limit: this.bootstrap.form.list.pageSize, search: "", searchField: "", advancedVisible: false, sort: "", descending: false, filters: [], cursor: null, nextCursor: null, history: []};
      await this.loadList(true); return;
    }
    this.listState = null; this.render();
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
  startSessionMonitor(databaseId) {
    clearInterval(this._monitor); this._monitor = setInterval(async () => {
      const response = await fetch(`/api/databases/${encodeURIComponent(databaseId)}/session`);
      if (!response.ok) { clearInterval(this._monitor); location.href = "/"; return; }
      const session = await response.json();
      if (session.message) { this.announce(session.message); await fetch(`/api/sessions/${session.id}/message/ack`, {method: "POST", headers: {"X-ML-CSRF": "1"}}); }
    }, 5000);
  }
  disconnectedCallback() { clearInterval(this._monitor); }
}

customElements.define("ml-command-bar", MLCommandBar);
customElements.define("ml-form", MLForm);
customElements.define("ml-app-shell", MLAppShell);
