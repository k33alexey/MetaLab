'use strict';

/* Studio-wide current display language: every localized title field shows
   this one language collapsed to a single line; a magnifying-glass button
   opens all configured translations at once (в духе 1С). Starts from the
   ML Project's own default language (set in Manager, carried into Studio)
   and can be switched within Studio to any configured project language. */
const StudioLanguage = (() => {
  let current = null;
  const listeners = new Set();
  return {
    get() { return current; },
    set(code) {
      if (!code || code === current) return;
      current = code;
      for (const listener of listeners) listener(code);
    },
    init(defaultCode) { if (current === null) current = defaultCode; },
    onChange(listener) { listeners.add(listener); return () => listeners.delete(listener); },
  };
})();

/* resolveLocalizedText picks one translation by the chain of the server's
   project.LocalizedText.Resolve, step for step: the language asked for, the
   same language without its region, a stored translation of that language
   with a region, the project's default language, the configured languages in
   order, and any non-empty value at all. Studio keeps no chain of its own:
   five editors used to pick their own way, and one title showed in different
   languages side by side. TestStudioResolvesTextLikeTheServer runs the same
   cases through both. */
function resolveLocalizedText(text, language, defaultLanguage, languages) {
  if (!text || typeof text !== 'object') return '';
  const value = key => (Object.hasOwn(text, key) && typeof text[key] === 'string' ? text[key].trim() : '');
  const keys = Object.keys(text).sort();
  if (value(language)) return value(language);
  const cut = (language || '').indexOf('-');
  if (cut > 0 && value(language.slice(0, cut))) return value(language.slice(0, cut));
  const requested = languageBase(language);
  if (requested) {
    const key = keys.find(item => languageBase(item) === requested && value(item));
    if (key !== undefined) return value(key);
  }
  if (value(defaultLanguage)) return value(defaultLanguage);
  for (const item of languages || []) if (value(item.code)) return value(item.code);
  const key = keys.find(item => value(item));
  return key === undefined ? '' : value(key);
}

/* languageBase is project.LanguageBase: the language of a code without its
   region, in lower case. */
function languageBase(code) {
  return (code || '').trim().toLowerCase().split('-')[0];
}

/* localizedTitle is what Studio shows for anything with a localized title:
   the translation in the current display language by the project's chain,
   and the name when there is no translation at all. */
function localizedTitle(item, defaultLanguage, languages) {
  return resolveLocalizedText(item?.title, StudioLanguage.get() || defaultLanguage, defaultLanguage, languages) || item?.name || '';
}

/* languageTitle names a language on itself, as the server does for the
   language list: a list where one language is written in another reads as a
   mistake. Without a synonym the name stays, and the code after it — an empty
   row in the list of languages is worse than a technical one. */
function languageTitle(language, defaultLanguage, languages) {
  return resolveLocalizedText(language.title, language.code, defaultLanguage, languages) || language.name || language.code;
}

/* uniqueName returns base, or base followed by the first number from 2 that
   is not taken; used holds the taken names in lower case, because names in
   a configuration differ by more than case. */
function uniqueName(base, used) {
  let index = 1, name = base;
  while (used.has(name.toLocaleLowerCase())) name = base + (++index);
  return name;
}

/* node is the one way Studio editors build an element: a tag, its text and
   its class. */
function node(tag, text, className) {
  const element = document.createElement(tag);
  if (text !== undefined) element.textContent = text;
  if (className) element.className = className;
  return element;
}

/* labeledField is a property row: a caption and its control in one label.
   The form inspector lays its rows out its own way and passes its class. */
function labeledField(label, control, className = 'catalog-field') {
  const wrapper = node('label', undefined, className);
  wrapper.append(node('span', label), control);
  return wrapper;
}

/* textInput is a free-text control. A limit is set only where the model has
   one (options.maxLength): a limit the model does not know refuses what the
   prototype keeps. Text is applied on every keystroke; a number is applied
   once it is committed, as a Number. options: kind 'area' (rows), type,
   min, max, maxLength, placeholder, list, disabled. */
function textInput(value, apply, options = {}) {
  const input = node(options.kind === 'area' ? 'textarea' : 'input');
  if (options.kind === 'area') input.rows = options.rows || 3;
  else input.type = options.type || 'text';
  input.value = value ?? '';
  for (const key of ['min', 'max', 'maxLength', 'placeholder']) if (options[key] !== undefined) input[key] = options[key];
  if (options.list) input.setAttribute('list', options.list);
  input.disabled = !!options.disabled;
  if (options.type === 'number') input.addEventListener('change', () => apply(Number(input.value)));
  else input.addEventListener('input', () => apply(input.value));
  return input;
}

/* selectInput chooses one of choices, given as [value, text] pairs. */
function selectInput(choices, value, apply, label) {
  const select = node('select');
  for (const [key, text] of choices) {
    const option = node('option', text);
    option.value = key;
    select.append(option);
  }
  select.value = value;
  if (label) select.setAttribute('aria-label', label);
  select.addEventListener('change', () => apply(select.value));
  return select;
}

/* checkboxField is a box with its caption after it. */
function checkboxField(label, checked, apply, className = 'catalog-check', disabled = false) {
  const wrapper = node('label', undefined, className), input = node('input');
  input.type = 'checkbox';
  input.checked = checked;
  input.disabled = disabled;
  input.addEventListener('change', () => apply(input.checked));
  wrapper.append(input, node('span', label));
  return wrapper;
}

/* localizedTextField writes one text in every language of the project, a row
   per language with the language's code beside it, so which translation is
   being written is never a guess. value(code) reads a translation,
   apply(code, text) writes one; options go to every row's textInput. */
function localizedTextField(label, languages, value, apply, options = {}) {
  const wrapper = node('div', undefined, 'project-localized');
  wrapper.append(node('span', label, 'project-localized-label'));
  const rows = node('div', undefined, 'project-localized-rows');
  for (const language of languages) {
    const row = node('div', undefined, 'project-localized-row');
    row.append(node('span', language.code, 'project-language-chip'),
      textInput(value(language.code), text => apply(language.code, text), options));
    rows.append(row);
  }
  wrapper.append(rows);
  return wrapper;
}

/* createLocalizedTitleField renders one collapsed-to-one-line title input
   bound to StudioLanguage.get(), plus a 🔍 button (hidden when the project
   has only one language — nothing to expand) that opens every configured
   language as its own input in a small dialog.
   languages: [{code,title}], getValue(code)=>string, setValue(code,value),
   onChange() called after every edit (collapsed or expanded). */
function createLocalizedTitleField(languages, getValue, setValue, onChange) {
  const wrapper = document.createElement('div');
  wrapper.className = 'localized-title-field';
  const input = document.createElement('input');
  input.type = 'text';
  input.maxLength = 512;
  function currentLanguage() { return StudioLanguage.get() || languages[0]?.code || ''; }
  function caption(code) { return languages.find(item => item.code === code)?.title || code; }
  function refresh() { input.value = getValue(currentLanguage()) || ''; input.title = caption(currentLanguage()); }
  input.addEventListener('input', () => { setValue(currentLanguage(), input.value); onChange(); });
  wrapper.append(input);
  if (languages.length > 1) {
    const expand = document.createElement('button');
    expand.type = 'button';
    expand.className = 'localized-title-expand';
    expand.title = 'Заголовки на всех языках проекта';
    expand.textContent = '🔍';
    expand.addEventListener('click', () => openLocalizedTitleDialog(languages, getValue, setValue, () => { refresh(); onChange(); }));
    wrapper.append(expand);
  }
  const unsubscribe = StudioLanguage.onChange(refresh);
  wrapper.dataset.localizedTitleField = 'true';
  wrapper.destroy = unsubscribe;
  refresh();
  return wrapper;
}

function openLocalizedTitleDialog(languages, getValue, setValue, onChange) {
  const dialog = document.createElement('dialog');
  dialog.className = 'localized-title-dialog';
  const form = document.createElement('form');
  form.method = 'dialog';
  const heading = document.createElement('h3');
  heading.textContent = 'Заголовок на языках проекта';
  form.append(heading);
  for (const language of languages) {
    const label = document.createElement('label');
    label.append(document.createTextNode(language.title || language.code));
    const fieldInput = document.createElement('input');
    fieldInput.type = 'text';
    fieldInput.maxLength = 512;
    fieldInput.value = getValue(language.code) || '';
    fieldInput.addEventListener('input', () => { setValue(language.code, fieldInput.value); onChange(); });
    label.append(fieldInput);
    form.append(label);
  }
  const actions = document.createElement('div');
  actions.className = 'localized-title-actions';
  const close = document.createElement('button');
  close.type = 'submit';
  close.className = 'primary';
  close.textContent = 'Готово';
  actions.append(close);
  form.append(actions);
  dialog.append(form);
  document.body.append(dialog);
  dialog.addEventListener('close', () => dialog.remove());
  dialog.showModal();
}

/* roleObjectRight is a right of a role on an object as the prototype reads
   it: written in the role, it is what is written; not written, it is the
   role's flag «Устанавливать права для новых объектов» - the prototype's role
   of full rights writes nothing but its denials. The role editor and the
   window of all roles read rights through it alone. */
function roleObjectRight(role, grant, right) {
  if (grant?.operations?.includes(right)) return true;
  if (grant?.denied?.includes(right)) return false;
  return !!role.grantNewObjectsByDefault;
}

/* rolePartRight is a right of a role on a part of an object - an attribute,
   a table part, a command, an operation - as the prototype reads it, by the
   exports of the configurator: written in the role, it is what is written.
   Not written, a field (a part offered Edit beside View) has the object's
   right of the same name while the role's flag «Устанавливать права для
   реквизитов и табличных частей по умолчанию» is set, and no right while it
   is not; a command, an operation of a web service and a method of an HTTP
   service have the flag for new objects, as an object has. written is what
   the role writes on the part, found by the caller in its own index. */
function rolePartRight(role, grant, written, part, right) {
  if (written?.operations?.includes(right)) return true;
  if (written?.denied?.includes(right)) return false;
  if (part.operations.includes('edit')) return !!role.grantNewFieldsByDefault && roleObjectRight(role, grant, right);
  return !!role.grantNewObjectsByDefault;
}

/* restrictionFieldsCaption names the fields a restriction of access to data
   checks, as the prototype's role editor names them: a restriction with no
   fields is the one on «Прочие поля», a field of a table part is named after
   the part. object is the schema's object, title gives a field its caption
   on the language shown. */
function restrictionFieldsCaption(object, restriction, title) {
  if (!restriction.fields?.length) return 'Прочие поля';
  const own = new Map((object?.restrictionFields || []).map(field => [field.key, field]));
  const caption = field => title(field) || field.name || field.key;
  return restriction.fields.map(key => {
    const field = own.get(key);
    if (!field) return key;
    const parent = field.parent && own.get(field.parent);
    return parent ? `${caption(parent)}.${caption(field)}` : caption(field);
  }).join(', ');
}

/* showDialog shows a dialog of the page, never confirm(): a heading, an
   explanation, any content, and buttons, each [key, caption]. It answers
   with the key of the button pressed, or null when the dialog is closed
   without one. check(key), when given, is asked before the dialog closes on
   a button with a key: a text it returns is shown and keeps the dialog open.
   A button may take a third element, {act, shown}, for the steps of a
   wizard: act() is done in place of closing, and after it every button is
   shown or hidden by its shown(); the first shown is the primary one. */
function showDialog({title, explanation, content, answers, check, className}) {
  return new Promise(resolve => {
    const dialog = document.createElement('dialog');
    dialog.className = 'localized-title-dialog' + (className ? ' ' + className : '');
    const form = document.createElement('form');
    form.method = 'dialog';
    form.append(node('h3', title));
    if (explanation) form.append(node('p', explanation, 'muted'));
    if (content) form.append(content);
    const problem = node('p', undefined, 'dialog-problem');
    problem.hidden = true;
    form.append(problem);
    const actions = node('div', undefined, 'localized-title-actions');
    let answer = null;
    const buttons = answers.map(([key, text, step]) => {
      const button = node('button', text);
      button.type = 'submit';
      button.addEventListener('click', event => {
        const refusal = key !== null && check ? check(key) : '';
        if (refusal) { event.preventDefault(); problem.textContent = refusal; problem.hidden = false; return; }
        problem.hidden = true;
        if (step?.act) { event.preventDefault(); step.act(); arrange(); return; }
        answer = key;
      });
      actions.append(button);
      return [button, step];
    });
    // A hidden button is disabled too, so Enter does not press it.
    function arrange() {
      let primary = true;
      for (const [button, step] of buttons) {
        const shown = !step?.shown || step.shown();
        button.hidden = !shown; button.disabled = !shown;
        button.classList.toggle('primary', shown && primary);
        if (shown) primary = false;
      }
    }
    arrange();
    form.append(actions);
    dialog.append(form);
    document.body.append(dialog);
    dialog.addEventListener('close', () => { dialog.remove(); resolve(answer); });
    dialog.showModal();
  });
}

/* showContextMenu shows the commands of what was right-clicked at the
   point of the click: items are [text, act]. One menu is open at a time; it
   closes when a command is chosen, on Escape, on a click elsewhere, when the
   page scrolls or loses focus. A menu opened from the keyboard (Shift+F10,
   the menu key) or by a button, as «Действия», stands under the element
   instead. The arrows move between commands, Enter chooses one. */
function showContextMenu(event, items) {
  event.preventDefault();
  closeContextMenu();
  const menu = node('div', undefined, 'context-menu');
  menu.setAttribute('role', 'menu');
  for (const [text, act] of items) {
    const button = node('button', text, 'context-menu-item');
    button.type = 'button';
    button.setAttribute('role', 'menuitem');
    button.addEventListener('click', () => { closeContextMenu(); act(); });
    menu.append(button);
  }
  const commands = [...menu.children];
  menu.addEventListener('keydown', key => {
    const at = commands.indexOf(document.activeElement);
    if (key.key === 'Escape') { key.preventDefault(); closeContextMenu(); }
    else if (key.key === 'ArrowDown' || key.key === 'ArrowUp') {
      key.preventDefault();
      commands[(at + (key.key === 'ArrowDown' ? 1 : commands.length - 1)) % commands.length].focus();
    }
  });
  document.body.append(menu);
  let x = event.clientX, y = event.clientY;
  if ((event.type === 'click' || (!x && !y)) && event.currentTarget instanceof Element) {
    const box = event.currentTarget.getBoundingClientRect();
    x = box.left; y = box.bottom;
  }
  menu.style.left = Math.max(0, Math.min(x, innerWidth - menu.offsetWidth)) + 'px';
  menu.style.top = Math.max(0, Math.min(y, innerHeight - menu.offsetHeight)) + 'px';
  const away = other => { if (!menu.contains(other.target)) closeContextMenu(); };
  const close = () => closeContextMenu();
  closeContextMenu.release = () => {
    removeEventListener('mousedown', away, true); removeEventListener('scroll', close, true);
    removeEventListener('blur', close); removeEventListener('resize', close);
  };
  addEventListener('mousedown', away, true); addEventListener('scroll', close, true);
  addEventListener('blur', close); addEventListener('resize', close);
  commands[0]?.focus();
}

function closeContextMenu() {
  closeContextMenu.release?.();
  closeContextMenu.release = null;
  document.querySelector('.context-menu')?.remove();
}

/* askChoice asks a question in a dialog of the page and answers with the key
   of the button pressed, or null when the dialog is closed without one. */
function askChoice(question, explanation, answers) {
  return showDialog({title: question, explanation, answers});
}

/* queryTextField edits a text in the query language - the condition of a
   restriction of access, a template of one - with its keywords, strings,
   parameters and instructions highlighted. The highlighting is the server's,
   by the language's own list of keywords (/api/query/highlight); until it
   answers, and when it does not, the text shows plain. apply(text) is called
   on every edit. */
function queryTextField(value, apply, label) {
  const wrapper = node('div', undefined, 'query-text');
  const layer = node('pre', undefined, 'query-text-layer');
  layer.setAttribute('aria-hidden', 'true');
  const code = node('code');
  layer.append(code);
  const textarea = document.createElement('textarea');
  textarea.value = value;
  textarea.spellcheck = false;
  if (label) textarea.setAttribute('aria-label', label);
  wrapper.append(layer, textarea);
  let timer = 0, controller = null, last = [];
  // Until the server answers, an edited text keeps the highlighting it had:
  // a text going plain on every key would blink.
  function paint(text, highlights) {
    last = highlights;
    code.replaceChildren();
    let cursor = 0;
    for (const item of highlights) {
      if (item.start < cursor || item.end <= item.start) continue;
      code.append(document.createTextNode(text.slice(cursor, item.start)), node('span', text.slice(item.start, item.end), `tok-${item.kind}`));
      cursor = item.end;
    }
    // The last line break of a pre takes no line, so one more is put after it.
    code.append(document.createTextNode(text.slice(cursor) + '\n'));
    layer.scrollTop = textarea.scrollTop; layer.scrollLeft = textarea.scrollLeft;
  }
  async function highlight() {
    const text = textarea.value;
    controller?.abort(); controller = new AbortController();
    try {
      const response = await fetch('/api/query/highlight', {method: 'POST', headers: {'Content-Type': 'application/json', 'X-ML-CSRF': '1'}, body: JSON.stringify({text}), signal: controller.signal});
      if (!response.ok || textarea.value !== text) return;
      paint(text, (await response.json()).highlights || []);
    } catch (error) { if (error.name !== 'AbortError') paint(textarea.value, []); }
  }
  textarea.addEventListener('input', () => { apply(textarea.value); paint(textarea.value, last); clearTimeout(timer); timer = setTimeout(highlight, 150); });
  textarea.addEventListener('scroll', () => { layer.scrollTop = textarea.scrollTop; layer.scrollLeft = textarea.scrollLeft; });
  paint(value, []);
  highlight();
  wrapper.textarea = textarea;
  return wrapper;
}
