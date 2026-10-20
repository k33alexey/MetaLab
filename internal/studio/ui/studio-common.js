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

/* textPresentation is the server's project.LocalizedText.Presentation, step
   for step: the translation in the language asked for, and the name where
   there is none - not a translation in another language (METADATA-OBJECTS.md,
   «Нет перевода на языке — имя»; the owner's decision of 19.10.2026). A
   text of spaces is none. TestStudioShowsTextLikeTheServer runs the same
   cases through both. */
function textPresentation(text, language, name) {
  const value = text && typeof text === 'object' && Object.hasOwn(text, language) && typeof text[language] === 'string' ? text[language].trim() : '';
  return value || name;
}

/* setTranslation writes one translation of a localized text - a synonym,
   a text of the configuration: a blank one is no translation, as the model
   counts it (validateTitle: a translation says something in printable
   characters), and is taken away. Answers the text, made when there was
   none. */
function setTranslation(text, language, value) {
  text ||= {};
  if (value.trim()) text[language] = value; else delete text[language];
  return text;
}

/* localizedTitle is what Studio shows for anything with a localized title:
   its presentation in the language Studio edits in. */
function localizedTitle(item, defaultLanguage) {
  return textPresentation(item?.title, StudioLanguage.get() || defaultLanguage, item?.name || '');
}

/* languageTitle names a language by its synonym in the language Studio
   edits in, as the prototype names the languages of «Строки на разных
   языках» (ERP, whose main language is Ukrainian: «Російська»,
   «Українська»; the owner's decision of 19.10.2026) - one rule for every
   place Studio names a language, and with no synonym in that language its
   name, as anything else is shown. The code stands only for a language with
   no name. */
function languageTitle(language, defaultLanguage) {
  return localizedTitle(language, defaultLanguage) || language.code;
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

/* clearableSelect is a drop-down list with a button that clears it, as the
   prototype's field of a choice from a list: the clearing chooses the empty
   value '', which selectInput's own change applies. */
function clearableSelect(choices, value, apply, label) {
  const wrapper = node('span', undefined, 'clearable-select');
  const select = selectInput([['', ''], ...choices], value, apply, label);
  const clear = node('button', '×', 'clearable-select-clear');
  clear.type = 'button';
  clear.setAttribute('aria-label', label ? `Очистить: ${label}` : 'Очистить');
  clear.addEventListener('click', () => { select.value = ''; select.dispatchEvent(new Event('change', {bubbles: true})); });
  wrapper.append(select, clear);
  return wrapper;
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

/* fieldButton is a button of a field - the magnifier, the opening, the
   clearing, the choice - drawn by its icon of the set, with act done on a
   click. */
function fieldButton(icon, title, act) {
  const button = iconButton(icon, title);
  button.classList.add('field-button');
  button.addEventListener('click', act);
  return button;
}

/* fieldWithButtons puts the buttons of a field inside its frame at the right
   edge, as the prototype's fields keep them: the field gives them room
   inside itself, so its outer width does not depend on how many buttons it
   has, and the fields of a window and of the palette stand on one grid
   (ML-STUDIO.md, «кнопки поля — внутри поля справа»). control is the input,
   the text area or the list; buttons are made by fieldButton. The control
   fills the field, and the field is as wide as the editor makes its plain
   controls: the editor's rule of width names .field-with-buttons beside
   them. */
function fieldWithButtons(control, buttons) {
  const field = node('span', undefined, 'field-with-buttons'), tray = node('span', undefined, 'field-buttons');
  control.classList.add('field-control');
  if (buttons.length) control.style.paddingRight = `calc(${buttons.length} * var(--field-button-width) + 4px)`;
  tray.append(...buttons);
  field.append(control, tray);
  return field;
}

/* createLocalizedField is the one field of a text written in every language
   of the project - a synonym, a title, an information, an explanation - as
   the prototype has it: the translation in the language Studio edits in
   (StudioLanguage, the project's default language until switched) and the
   magnifier inside the field at the right, which opens «Строки на разных
   языках» with a string for every language of the project (ML-STUDIO.md,
   «многоязычное поле»). The magnifier stands while the project has more than
   one language (the owner's decision of 19.10.2026). The translations are
   never spread out in the window of an object. project holds languages -
   every language of the project, as the configuration writes them - and
   defaultLanguage; getValue(code) reads a translation, setValue(code, value)
   writes one; onChange() is called after every edit, in the field or in the
   window; options go to textInput - kind 'area', maxLength, disabled.
   destroy() stops following the language of Studio. */
/* titleTextOptions are the options of a localized title of an object. The
   single line and the limit of 512 are not the model's: BLOCKS, block 18,
   «переводимые тексты в Studio многострочны и без предела длины». */
const titleTextOptions = Object.freeze({maxLength: 512});

function createLocalizedField(project, getValue, setValue, onChange, options = {}) {
  const languages = project.languages || [];
  const current = () => StudioLanguage.get() || project.defaultLanguage || languages[0]?.code || '';
  const input = textInput('', value => { setValue(current(), value); onChange(); }, options);
  const refresh = () => { input.value = getValue(current()) || ''; };
  const buttons = [];
  if (languages.length > 1) {
    const open = fieldButton('field-translations', 'Строки на разных языках', async () => {
      if (await openLocalizedTextDialog(project, getValue, setValue, options)) { refresh(); onChange(); }
    });
    open.disabled = !!options.disabled;
    buttons.push(open);
  }
  const field = fieldWithButtons(input, buttons);
  field.destroy = StudioLanguage.onChange(refresh);
  refresh();
  return field;
}

/* openLocalizedTextDialog is the prototype's window «Строки на разных
   языках» (the owner's snapshot of 19.10.2026, block-00 konfiguraciya):
   the caption of each language above its string, named by languageTitle,
   ОК and «Отмена» in a column to the right, no «Справка». The strings are written on ОК alone; «Отмена»
   and Escape leave the text as it was. The string of the language Studio
   edits in is focused, its text selected. Answers whether anything was
   written. */
async function openLocalizedTextDialog(project, getValue, setValue, options) {
  const content = document.createDocumentFragment(), strings = [], languages = project.languages;
  for (const language of languages) {
    const input = textInput(getValue(language.code) || '', () => {}, {...options, disabled: false});
    const label = node('label');
    label.append(node('span', languageTitle(language, project.defaultLanguage)), input);
    content.append(label);
    strings.push([language.code, input]);
  }
  const answer = showDialog({title: 'Строки на разных языках', content, side: true, answers: [['ok', 'ОК'], [null, 'Отмена']]});
  const focused = strings.find(([code]) => code === StudioLanguage.get())?.[1] || strings[0]?.[1];
  focused?.focus();
  focused?.select();
  if (await answer !== 'ok') return false;
  let written = false;
  for (const [code, input] of strings) {
    if (input.value === (getValue(code) || '')) continue;
    setValue(code, input.value);
    written = true;
  }
  return written;
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

/* hostedEditor is the part of an editor's contract with index.html that is
   the same for every editor of a file: close() forgets the file (reset) and
   hides and empties the host, setDisabled(flag) makes the host inert while
   Studio opens, saves or creates something, selectFragment(fragment) passes
   the part a node of the tree names - `kind:id`, as the server writes it
   (Fragment of a node, studio.go) - to own.select(kind, id). own - the
   editor's own methods, open, value and the rest. */
function hostedEditor(host, reset, own) {
  return {
    close() { reset(); host.hidden = true; host.replaceChildren(); },
    setDisabled(disabled) { host.inert = disabled; },
    selectFragment(fragment) {
      const at = fragment ? fragment.indexOf(':') : -1;
      if (at > 0) own.select?.(fragment.slice(0, at), fragment.slice(at + 1));
    },
    ...own,
  };
}

/* studioRequest is the one way a page of Studio asks its server to change
   or work out something: every such request carries X-ML-CSRF, which the
   server asks of it against a page of another site, and a body goes as JSON
   (validateStudioMutation, decodeStudioJSON in studio.go). method is POST
   unless named; signal and keepalive pass to fetch. Answers the response as
   fetch does. */
function studioRequest(path, {method = 'POST', body, signal, keepalive} = {}) {
  const headers = {'X-ML-CSRF': '1'};
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  return fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body), signal, keepalive});
}

/* postJSON is studioRequest with body that answers the JSON of a response
   that went well and throws the server's text, trimmed, of one that did not. */
async function postJSON(path, body, options = {}) {
  const response = await studioRequest(path, {...options, body});
  if (!response.ok) throw new Error((await response.text()).trim());
  return response.json();
}

/* showDialog shows a dialog of the page, never confirm(): a heading, an
   explanation, any content, and buttons, each [key, caption]. It answers
   with the key of the button pressed, or null when the dialog is closed
   without one. check(key), when given, is asked before the dialog closes on
   a button with a key: a text it returns is shown and keeps the dialog open.
   A button may take a third element, {act, enabled, primary}: act() is done
   in place of closing, as the steps of a wizard do; enabled() makes it
   unavailable without hiding it, as the prototype's buttons are; the
   primary one - the first whose primary() holds, or else the first
   available - is pressed by Enter, and not at all while it is unavailable.
   Both are asked again after every change in the dialog. A null in place
   of a button keeps its place empty, as the prototype keeps the place of a
   button a window of the same kind shows. help names the window's page of studioHelp: «Справка» and
   F1 open it over the dialog. */
function showDialog({title, explanation, content, answers, check, className, help, side}) {
  return new Promise(resolve => {
    const dialog = document.createElement('dialog');
    // side — кнопки столбцом справа от содержимого, как в окнах прототипа
    // (снимки shabl-16, obolochka/32); без него — строкой под ним.
    dialog.className = 'localized-title-dialog' + (side ? ' dialog-side' : '') + (className ? ' ' + className : '');
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
    const buttons = answers.flatMap(place => {
      if (!place) { actions.append(node('span', undefined, 'dialog-gap')); return []; }
      const [key, text, step] = place, button = node('button', text);
      button.type = 'submit';
      button.addEventListener('click', event => {
        const refusal = key !== null && check ? check(key) : '';
        if (refusal) { event.preventDefault(); problem.textContent = refusal; problem.hidden = false; return; }
        problem.hidden = true;
        if (step?.act) { event.preventDefault(); step.act(); arrange(); return; }
        answer = key;
      });
      actions.append(button);
      return [[button, step]];
    });
    if (help) {
      const button = node('button', 'Справка');
      button.type = 'button';
      button.addEventListener('click', () => showHelp(help));
      actions.append(button);
    }
    let primary = null;
    function arrange() {
      for (const [button, step] of buttons) button.disabled = !!step?.enabled && !step.enabled();
      primary = (buttons.find(([, step]) => step?.primary?.()) || buttons.find(([button]) => !button.disabled))?.[0] || null;
      for (const [button] of buttons) button.classList.toggle('primary', button === primary);
    }
    arrange();
    // What the content changed is read again by every button: the handlers of
    // the content run first, the form hears the change after them.
    for (const name of ['change', 'input', 'click']) form.addEventListener(name, arrange);
    // Enter presses the primary button, not the first one of the form: on the
    // second step of a wizard that is «< Назад».
    form.addEventListener('keydown', event => {
      if (event.key === 'F1' && help) { event.preventDefault(); showHelp(help); return; }
      if (event.key !== 'Enter' || event.target.tagName === 'TEXTAREA' || event.target.tagName === 'BUTTON') return;
      event.preventDefault();
      if (primary && !primary.disabled) primary.click();
    });
    form.append(actions);
    dialog.append(form);
    document.body.append(dialog);
    dialog.addEventListener('close', () => { dialog.remove(); resolve(answer); });
    dialog.showModal();
  });
}

/* showHelp opens a page of studioHelp (studio-help.js) over what is open: its
   heading and paragraphs. */
function showHelp(key) {
  const page = studioHelp[key], content = node('div', undefined, 'studio-help');
  for (const paragraph of page.text) content.append(node('p', paragraph));
  return showDialog({title: page.title, content, className: 'studio-help-dialog', answers: [[null, 'Закрыть']]});
}

/* showContextMenu shows the commands of what was right-clicked at the
   point of the click: items are [text, act]. One menu is open at a time; it
   closes when a command is chosen, on Escape, on a click elsewhere, when the
   page scrolls or loses focus. A menu opened from the keyboard (Shift+F10,
   the menu key) or by a button, as «Действия», stands under the element
   instead. The arrows move between commands, Enter chooses one. An item
   [text, act, false] is shown unavailable, as the prototype's menus show a
   command that cannot act now; [text, act, enabled, key] shows its key at
   the right, as «Ins» of «Добавить»; a fifth element names the icon of the
   command, drawn before its text, and once one item has an icon every item
   keeps the place of one, so the texts stand in a column, as in the
   prototype's menus; a sixth, true, marks the action of the object itself,
   done by a double click, bold, as the prototype's «Открыть модуль» of a
   common module; null is a line between groups. */
function showContextMenu(event, items) {
  event.preventDefault();
  closeContextMenu();
  const menu = node('div', undefined, 'context-menu');
  menu.setAttribute('role', 'menu');
  const icons = items.some(item => item?.[4]);
  for (const item of items) {
    if (!item) { menu.append(node('div', undefined, 'context-menu-separator')); continue; }
    const [text, act, enabled = true, key, icon, main] = item;
    const button = node('button', text, main ? 'context-menu-item context-menu-main' : 'context-menu-item');
    button.type = 'button';
    if (icons) button.prepend(iconElement(icon));
    if (key) button.append(node('span', key, 'context-menu-key'));
    button.disabled = !enabled;
    button.setAttribute('role', 'menuitem');
    button.addEventListener('click', () => { closeContextMenu(); act(); });
    menu.append(button);
  }
  const commands = [...menu.querySelectorAll('.context-menu-item')].filter(button => !button.disabled);
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
  placeMenu(menu, x, y);
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

/* placeMenu puts a menu shown at x, y where it stays in the window: moved
   left and up by what would stand out of it - a context menu, a menu of the
   main menu and its submenus alike. */
function placeMenu(menu, x, y) {
  menu.style.left = Math.max(0, Math.min(x, innerWidth - menu.offsetWidth)) + 'px';
  menu.style.top = Math.max(0, Math.min(y, innerHeight - menu.offsetHeight)) + 'px';
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
      const response = await studioRequest('/api/query/highlight', {body: {text}, signal: controller.signal});
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

/* Icons are drawn from one sprite, the icons of internal/icons (ICONS.md,
   «Визуальный язык»): the page puts it in once and an icon is a <use> of its
   symbol by name. The sprite is our own server's, so it goes in as markup. A
   <use> drawn before the sprite has come resolves once it is there. */
const StudioIcons = (() => {
  let loading = null;
  return {
    load() {
      if (!loading) {
        loading = fetch('/ui/icons.svg')
          .then(response => (response.ok ? response.text() : ''))
          .then(text => { if (text) document.body.insertAdjacentHTML('afterbegin', text); })
          .catch(() => {});
      }
      return loading;
    },
  };
})();

function iconElement(name) {
  const svgNS = 'http://www.w3.org/2000/svg';
  const svg = document.createElementNS(svgNS, 'svg');
  svg.setAttribute('class', 'tree-icon');
  svg.setAttribute('width', '16');
  svg.setAttribute('height', '16');
  svg.setAttribute('aria-hidden', 'true');
  if (name) {
    const use = document.createElementNS(svgNS, 'use');
    use.setAttribute('href', '#icon-' + name);
    svg.append(use);
  }
  return svg;
}

/* menuButton is a button that holds more than one action - «Действия» - and
   opens them as a menu under itself (showContextMenu), drawn with the sign
   of a menu after its text, as every such button of the prototype is (the
   owner's rule of 19.10.2026; snapshots obolochka/31-derevo-dejstviya,
   configurator-2026-10-18/vse-ogr-01). items() answers the items of the
   menu at the moment of the click. Every button of Studio that opens a menu
   is made here. */
function menuButton(text, items, className) {
  const button = node('button', text, className ? `menu-button ${className}` : 'menu-button');
  button.type = 'button';
  button.setAttribute('aria-haspopup', 'menu');
  button.addEventListener('click', event => showContextMenu(event, items()));
  return button;
}

/* treeNode is a row of a tree of Studio with what lies beneath it, every
   tree drawn as the project tree is and as the prototype's are (inventory of
   block 00, derevo; snapshot configurator-2026-10-14/rol-otbor-kassa): the
   sign of unfolding, or its empty place on a row with nothing beneath, so a
   row stands one step right of the row it lies under whatever it holds; the
   icon of what the row is; the caption - a text, or an element such as the
   button that chooses the row. A row with children opens and closes; open is
   how it stands first, toggled(open) is told each time it is opened or
   closed. Answers {element, row}: the element to put in the tree and the
   row to hang clicks and the menu on. Every tree of Studio is made of these;
   the trees of marks - the filter by subsystems, the fields of a
   restriction - are rows of restrictionTreeRow. */
function treeNode({icon, caption, children = [], open = false, toggled}) {
  const row = node(children.length ? 'summary' : 'div', undefined, 'studio-tree-row');
  row.append(iconElement(icon), typeof caption === 'string' ? node('span', caption) : caption);
  if (!children.length) {
    row.classList.add('studio-tree-leaf');
    return {element: row, row};
  }
  const element = node('details', undefined, 'studio-tree-node');
  element.open = open;
  if (toggled) element.addEventListener('toggle', () => toggled(element.open));
  const inside = node('div', undefined, 'studio-tree-children');
  inside.append(...children);
  element.append(row, inside);
  return {element, row};
}

/* tabStrip is a strip of tabs, drawn as the prototype's (the owner's
   decision of 20.10.2026, STUDIO-SHELL.md «Закладки — одним обликом
   везде»; snapshots configurator-2026-10-18/shabl-01,
   configurator-2026-10-19/shabl-19): the tab chosen stands higher than the
   others and merges with its page beneath. tabs are [key, text] in their
   order; chosen is the key shown chosen; choose(key) is called when another
   tab is chosen, by a click or by ← and → along the strip. The tab chosen
   takes the focus, as the prototype's does (snapshot vse-ogr-16), and is the
   one the focus comes to from outside the strip. The strip's choose(key)
   shows another tab chosen without calling it. Every strip of tabs of
   Studio is made here; a page of tabs in a dialog takes the class
   studio-tab-page, which frames it as the prototype's. */
function tabStrip(tabs, chosen, choose) {
  const strip = node('div', undefined, 'studio-tabs');
  strip.setAttribute('role', 'tablist');
  let current = chosen;
  const buttons = tabs.map(([key, text]) => {
    const tab = node('button', text, 'studio-tab');
    tab.type = 'button'; tab.dataset.tab = key;
    tab.setAttribute('role', 'tab');
    tab.addEventListener('click', () => pick(key));
    return tab;
  });
  function show() {
    for (const tab of buttons) {
      const on = tab.dataset.tab === current;
      tab.setAttribute('aria-selected', String(on)); tab.tabIndex = on ? 0 : -1;
    }
  }
  function pick(key) {
    buttons.find(tab => tab.dataset.tab === key).focus();
    if (key === current) return;
    current = key; show(); choose(key);
  }
  strip.addEventListener('keydown', event => {
    const by = {ArrowLeft: -1, ArrowRight: 1}[event.key];
    if (!by) return;
    event.preventDefault();
    const next = buttons[buttons.findIndex(tab => tab.dataset.tab === current) + by];
    if (next) pick(next.dataset.tab);
  });
  strip.append(...buttons);
  strip.choose = key => { current = key; show(); };
  show();
  return strip;
}

// iconButton is a button drawn by its icon alone, the command named in its
// tooltip and for a screen reader.
function iconButton(name, title) {
  const button = document.createElement('button');
  button.type = 'button';
  button.className = 'icon-only';
  button.title = title;
  button.setAttribute('aria-label', title);
  button.append(iconElement(name));
  return button;
}
