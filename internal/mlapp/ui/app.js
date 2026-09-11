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
      const table = document.createElement("div"); table.className = "form-table"; table.setAttribute("role", "table"); table.setAttribute("aria-label", item.title || "Таблица");
      const empty = document.createElement("p"); empty.className = "empty"; empty.textContent = "Нет данных"; table.append(empty); return table;
    }
    const label = document.createElement("label"); label.className = "form-field";
    const title = document.createElement("span"); title.className = "field-title"; title.textContent = item.title || "";
    const input = document.createElement("input"); input.id = `field-${item.id}`; input.type = item.inputType || "text"; input.value = item.value || ""; input.readOnly = Boolean(item.readOnly); input.disabled = Boolean(item.disabled);
    label.append(title, input); return label;
  }
}

class MLAppShell extends HTMLElement {
  constructor() { super(); this._busy = false; this.addEventListener("ml-command", event => this.runCommand(event.detail.id)); }
  connectedCallback() { this.load(); }
  async load() {
    const databaseId = location.pathname.split("/").filter(Boolean).at(-1);
    try {
      const response = await fetch(`/api/databases/${encodeURIComponent(databaseId)}/app-bootstrap`, {headers: {"Accept": "application/json"}});
      if (!response.ok) throw new Error(response.status === 401 ? "Требуется вход" : "База недоступна");
      this.bootstrap = await response.json(); document.documentElement.lang = this.bootstrap.locale || "ru"; this.render(); this.startSessionMonitor(databaseId);
    } catch (error) { this.renderError(error.message); }
  }
  render() {
    const data = this.bootstrap; this.replaceChildren();
    const header = document.createElement("header"); header.className = "app-header";
    const brand = document.createElement("a"); brand.className = "brand"; brand.href = "/"; brand.textContent = "ML"; brand.setAttribute("aria-label", "ML Portal");
    const database = document.createElement("strong"); database.textContent = data.database.name;
    const search = document.createElement("input"); search.type = "search"; search.placeholder = "Поиск команд и объектов"; search.setAttribute("aria-label", "Глобальный поиск"); search.disabled = true;
    const user = document.createElement("span"); user.className = "user"; user.textContent = data.user.login; header.append(brand, database, search, user);
    const body = document.createElement("div"); body.className = "app-body";
    const nav = document.createElement("nav"); nav.className = "app-nav"; nav.setAttribute("aria-label", "Разделы");
    for (const item of data.navigation || []) { const button = document.createElement("button"); button.type = "button"; button.textContent = item.title; button.className = item.current ? "current" : ""; if (item.current) button.setAttribute("aria-current", "page"); nav.append(button); }
    const main = document.createElement("main"); main.id = "ml-workspace"; main.className = "workspace"; main.tabIndex = -1;
    const tabs = document.createElement("div"); tabs.className = "window-tabs"; tabs.setAttribute("role", "tablist");
    const tab = document.createElement("button"); tab.type = "button"; tab.className = "window-tab current"; tab.setAttribute("role", "tab"); tab.setAttribute("aria-selected", "true"); tab.textContent = data.form.title; tabs.append(tab);
    const form = document.createElement("ml-form"); form.model = data.form; main.append(tabs, form); body.append(nav, main); this.append(header, body);
  }
  async runCommand(id) {
    if (this._busy) return; this._busy = true;
    try { if (id === "refresh") await this.load(); else this.announce(`Команда «${id}» пока недоступна`); }
    finally { this._busy = false; }
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
