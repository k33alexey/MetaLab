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

/* textFromName is the text the prototype makes of a name for its synonym or
   its description (docs/block-00/obshchee.md, «Текст из имени»): the words
   of the name parted by spaces, from the second on in small letters -
   «ТестовыйСправочникОписи» is «Тестовый справочник описи». A word begins
   with a capital after a small letter. How the prototype parts a name with a
   digit, an underscore or a capital no small letter follows («СтавкаНДС»,
   «Реквизит1», «СчетА») the inventory does not say: such a name makes no
   text (the series of 2.559, question В1), and the name stands where the
   text would. */
function textFromName(name) {
  if (!name || /[\d_]|\p{Lu}(?!\p{Ll})/u.test(name)) return '';
  return name.replace(/(?<=\p{Ll})\p{Lu}/gu, letter => ' ' + letter.toLocaleLowerCase());
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

/* numberedName is the name «Добавить» gives what it makes: base and the
   first number from 1 not taken - «Реквизит1», «ТабличнаяЧасть1»
   (docs/block-00/obshchee.md, «Добавить» без диалога имени); used holds the
   taken names in lower case. Its twin on the server is numberedName of
   names.go, which names a new object; TestANewNameIsNumberedLikeOnThePage
   runs both. */
function numberedName(base, used) {
  let index = 1;
  while (used.has((base + index).toLocaleLowerCase())) index++;
  return base + index;
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

/* StudioLimits are the model's ceilings on what a window types in, as the
   server sends them with the project (metadata.EditorLimits): name, code,
   description. A window sets a field by them through limited() and keeps no
   number of its own. */
const StudioLimits = {};

/* PlatformNamespaces are the short names of the namespaces of the platform
   by their address, as the server sends them with the project (metadata.
   PlatformXDTONamespaceNames; ML-STUDIO.md, «Пространства платформы —
   коротким именем»): a window shows a namespace by namespaceShown and takes
   one typed by namespaceTyped, and keeps no list of its own. */
const PlatformNamespaces = {};

/* namespaceShown is how a namespace is shown: one of the platform's by its
   short name - «data/core» -, any other whole. */
function namespaceShown(namespace) {
  return Object.hasOwn(PlatformNamespaces, namespace) ? PlatformNamespaces[namespace] : namespace;
}

/* namespaceTyped is the namespace typed: the short name of one of the
   platform's gives its address, anything else stays as typed. */
function namespaceTyped(text) {
  const typed = text.trim();
  return Object.keys(PlatformNamespaces).find(namespace => PlatformNamespaces[namespace] === typed) || text;
}

/* limited answers the options of a field with the model's ceiling of what
   it holds - a text by its length, a number by its value; a field the model
   sets no ceiling on is given none. */
function limited(field, options = {}) {
  const limit = StudioLimits[field];
  if (!limit) return options;
  return options.type === 'number' ? {...options, max: limit} : {...options, maxLength: limit};
}

/* textInput is a free-text control. A limit is set only where the model has
   one (limited): a limit the model does not know refuses what the
   prototype keeps. Text is applied on every keystroke; a number is applied
   once it is committed, as a Number. A number beyond its ceilings - the
   field's min and max, a fraction, or what apply refuses by answering false
   - is not taken: the prototype's question «Отменить ввод?» (askCancelInput)
   brings back the value before on «Да» and leaves the field to be put right
   on «Нет», in every window (docs/block-00/obshchee.md, «Пределы в окне»;
   vse-ogranicheniya-dostupa.md, «Настройка списка»). options: kind 'area'
   (rows), type, min, max, maxLength, placeholder, list, disabled. */
function textInput(value, apply, options = {}) {
  const input = node(options.kind === 'area' ? 'textarea' : 'input');
  if (options.kind === 'area') input.rows = options.rows || 3;
  else input.type = options.type || 'text';
  input.value = value ?? '';
  for (const key of ['min', 'max', 'maxLength', 'placeholder']) if (options[key] !== undefined) input[key] = options[key];
  if (options.list) input.setAttribute('list', options.list);
  input.disabled = !!options.disabled;
  if (options.type === 'number') {
    let before = input.value;
    input.addEventListener('change', async () => {
      if (input.value !== '' && input.validity.valid && apply(Number(input.value)) !== false) { before = input.value; return; }
      if (await askCancelInput()) input.value = before; else input.focus();
    });
  } else input.addEventListener('input', () => apply(input.value));
  return input;
}

/* wholeNumberInput is a field of a whole number from options.min to
   options.max: what is not one, or beyond them, is asked «Отменить ввод?»
   (textInput); a number taken is given to apply. */
function wholeNumberInput(value, apply, options = {}) {
  return textInput(String(value), number => {
    if (!Number.isInteger(number)) return false;
    apply(number);
  }, {...options, type: 'number'});
}

/* watchName checks a name once its field is left or Enter is pressed, as the
   prototype checks it (docs/block-00/obshchee.md, «Имя и синоним»): a name
   the server refuses is answered at once with the prototype's words, and the
   field stays to be put right. The rule, the branch and the words are the
   server's (/api/name, names.go); check() answers what to ask it - {path,
   name} for the name of an object, {path, description, at} for a part of the
   open one. Answers the promise of the last check, for a test to wait on. */
function watchName(input, check) {
  return watchValue(input, '/api/name', check);
}

/* watchValue is watchName for any value the server checks: when the field
   is left it posts check() to address and shows the refusal it answers -
   {refusal} - in the prototype's words, the field to be put right after
   «ОК». Answers the promise of the last check. */
function watchValue(input, address, check) {
  let last = Promise.resolve();
  input.addEventListener('change', () => {
    last = (async () => {
      let answer;
      try { answer = await postJSON(address, check()); } catch { return; }
      if (!answer.refusal) return;
      await askChoice(answer.refusal, '', [['ok', 'ОК']]);
      if (input.isConnected) input.focus();
    })();
  });
  return () => last;
}

/* nameInput is the field of a name in every window of Studio: a text by the
   model's ceiling of a name (limited) checked by the server once it is left
   (watchName). apply(value) is called on every keystroke - where a title
   follows the name, the caller lets it follow there; check() is watchName's.
   options are textInput's (disabled). */
function nameInput(value, apply, check, options = {}) {
  const input = textInput(value, apply, limited('name', options));
  watchName(input, check);
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

/* listCell is the prototype's cell of a table chosen from a list with
   pictures (docs/block-00/obshchee.md, «Ячейка-перечень таблицы страницы»;
   snapshot configurator-2026-10-26/series-2.578/03): the value's picture
   and caption; a double click, Cmd+Down or Alt+Down, or the button «Выбрать
   из списка» - a table shows it in its current row - opens the list of
   choices - [value, caption, icon] in their order - under the cell, and
   the one chosen is apply(value). */
function listCell(choices, value, apply, label) {
  const cell = node('div', undefined, 'choice-cell');
  cell.tabIndex = 0;
  cell.setAttribute('aria-label', label);
  const chosen = choices.find(([key]) => key === value);
  cell.append(iconElement(chosen?.[2]), node('span', chosen?.[1] || ''));
  const open = event => showContextMenu(event, choices.map(([key, caption, icon]) => [caption, () => apply(key), true, undefined, icon]));
  // The list opens under the cell, whatever opened it.
  const under = event => ({preventDefault: () => event.preventDefault(), currentTarget: cell, type: 'click'});
  cell.addEventListener('dblclick', event => open(under(event)));
  cell.addEventListener('keydown', event => { if (event.key === 'ArrowDown' && (event.metaKey || event.altKey)) open(under(event)); });
  // The one button of a drop-down list of every field and cell, in a cell of
  // its own parted by a line, as a field of a form of the prototype's
  // application (the owner's decision of 26.10.2026).
  const pick = fieldButton('input-field-list', 'Выбрать из списка (Cmd+Down)', event => open(under(event)));
  pick.classList.add('choice-cell-button');
  cell.append(pick);
  return cell;
}

/* clearableSelect is a drop-down list with a button that clears it, as the
   prototype's field of a choice from a list: the clearing chooses the empty
   value '', which selectInput's own change applies. */
function clearableSelect(choices, value, apply, label) {
  const wrapper = node('span', undefined, 'clearable-select');
  const select = selectInput([['', ''], ...choices], value, apply, label);
  const clear = clearButton(label ? `Очистить: ${label}` : 'Очистить', () => { select.value = ''; select.dispatchEvent(new Event('change', {bubbles: true})); });
  clear.classList.add('clearable-select-clear');
  wrapper.append(select, clear);
  return wrapper;
}

/* clearButton is the prototype's clearing of a field, the cross: title
   names it for a screen reader, act clears. */
function clearButton(title, act) {
  const clear = node('button', '×');
  clear.type = 'button';
  clear.title = title;
  clear.setAttribute('aria-label', title);
  clear.addEventListener('click', act);
  return clear;
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

/* checkList is a list of boxes with a current row, as the prototype's list
   of rights: a click on a caption makes its row current, a click on a box
   marks it and makes its row current too (configurator 22.10.2026,
   block-00/rol.md); the arrows move the current row and the space marks it. items
   are {key, caption, checked, mixed}: a mixed box stands for many marked
   in part, as the prototype's grey one, and the space or a click marks it.
   mark(key, checked) marks one, choose(key) makes one current. */
function checkList(items, current, mark, choose) {
  const list = node('div', undefined, 'check-list');
  list.tabIndex = 0;
  list.setAttribute('role', 'listbox');
  for (const item of items) {
    const row = node('div', undefined, 'check-list-row' + (item.key === current ? ' current' : '')), input = node('input');
    row.setAttribute('role', 'option');
    row.setAttribute('aria-selected', String(item.key === current));
    row.dataset.key = item.key;
    input.type = 'checkbox';
    input.checked = item.checked;
    input.indeterminate = !!item.mixed;
    input.tabIndex = -1;
    input.setAttribute('aria-label', item.caption);
    input.addEventListener('change', () => { mark(item.key, input.checked); choose(item.key); });
    row.addEventListener('click', event => { if (event.target !== input) choose(item.key); });
    row.append(input, node('span', item.caption));
    list.append(row);
  }
  list.addEventListener('keydown', event => markListKey(event, items, current, choose, mark));
  return list;
}

/* markListKey is the keyboard of a list of marks, the prototype's: ↑ and ↓
   move the current row, the space marks it or takes its mark away. rows are
   {key, checked, checkable} as shown, checkable left out where every row has
   a box; answers whether the key was the list's. */
function markListKey(event, rows, current, choose, mark) {
  const at = rows.findIndex(row => row.key === current);
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault();
    const next = rows[Math.min(rows.length - 1, Math.max(0, at + (event.key === 'ArrowDown' ? 1 : -1)))];
    if (next) choose(next.key);
    return true;
  }
  if (event.key === ' ' && at >= 0 && rows[at].checkable !== false) {
    event.preventDefault();
    mark(rows[at].key, !rows[at].checked);
    return true;
  }
  return false;
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

/* fieldChoiceButton is the button «…» of a field that opens a window of
   choice - the type, a list of values: the prototype draws it as three dots,
   a text and not a picture of the set. */
function fieldChoiceButton(title, act) {
  const button = node('button', '…', 'field-button field-choice');
  button.type = 'button';
  button.title = title;
  button.setAttribute('aria-label', title);
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
   destroy() stops following the language of Studio; follow() is below. */
/* titleTextOptions are the options of a localized title of an object: no
   ceiling, as the model sets none («Пределы на состав»). The single line is
   not the model's: BLOCKS, block 18, «переводимые тексты в Studio
   многострочны». */
const titleTextOptions = Object.freeze({});

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
  /* follow(before, after) is the text following its name, one rule for
     every pair «Имя» + «Синоним» and «Имя» + «Наименование» (ML-STUDIO.md,
     «Поведение платформы»): while the text in the language the field shows
     is empty or still the one the name before made, the name after makes it
     anew; a text put right by hand stays. */
  field.follow = (before, after) => {
    const language = current(), written = getValue(language) || '';
    if (written && written !== textFromName(before)) return;
    setValue(language, textFromName(after));
    refresh();
  };
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

/* restrictionShownFields is what the prototype's windows show of the fields
   a restriction of access to data checks: those the object has now
   (restrictionFields); a field a setting of the object took away - Родитель
   of a catalog with no hierarchy - is not shown, and a restriction naming
   only such fields is not shown at all: null (configurator 23.10.2026,
   absent-20…27; ACCESS-RIGHTS, «Ограничение по полю, которого у объекта
   нет»). Writing the role takes them away (DropAbsentRestrictionFields on
   the server). [] is «Прочие поля». */
function restrictionShownFields(object, restriction) {
  const named = restriction.fields || [];
  const shown = named.filter(key => object?.restrictionFields?.some(field => field.key === key));
  return named.length && !shown.length ? null : shown;
}

/* restrictionFieldsCaption names the fields a restriction of access to data
   checks, as the prototype's role editor names them: a restriction with no
   fields is the one on «Прочие поля», a field of a table part is named after
   the part; a field the object does not have now is not named
   (restrictionShownFields). object is the schema's object, title gives a
   field its caption on the language shown. */
function restrictionFieldsCaption(object, restriction, title) {
  const fields = restrictionShownFields(object, restriction);
  if (!fields?.length) return 'Прочие поля';
  const own = new Map(object.restrictionFields.map(field => [field.key, field]));
  const caption = field => title(field) || field.name || field.key;
  return fields.map(key => {
    const field = own.get(key);
    const parent = field.parent && own.get(field.parent);
    return parent ? `${caption(parent)}.${caption(field)}` : caption(field);
  }).join(', ');
}

/* hostedEditor is the part of an editor's contract with index.html that is
   the same for every editor of a file: close() forgets the file (reset) and
   hides and empties the host, setDisabled(flag) makes the host inert while
   Studio opens, saves or creates something, selectFragment(fragment) passes
   the part a node of the tree names - `kind:id`, as the server writes it
   (Fragment of a node, studio.go) - to own.select(kind, id), and answers
   whether the editor chose the part it names. own - the
   editor's own methods, open, value and the rest. */
function hostedEditor(host, reset, own) {
  return {
    close() { reset(); host.hidden = true; host.replaceChildren(); },
    setDisabled(disabled) { host.inert = disabled; },
    selectFragment(fragment) {
      const at = fragment ? fragment.indexOf(':') : -1;
      return at > 0 && !!own.select?.(fragment.slice(0, at), fragment.slice(at + 1));
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
function showDialog({title, caption, explanation, content, answers, check, className, help, side}) {
  return new Promise(resolve => {
    const dialog = document.createElement('dialog');
    // side — кнопки столбцом справа от содержимого, как в окнах прототипа
    // (снимки shabl-16, obolochka/32); без него — строкой под ним.
    dialog.className = 'localized-title-dialog' + (side ? ' dialog-side' : '') + (className ? ' ' + className : '');
    const form = document.createElement('form');
    form.method = 'dialog';
    // caption is the window's own title above what it asks - «ML Studio»
    // of a question (questionCaption).
    if (caption) form.append(node('div', caption, 'dialog-caption'));
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
/* menuItems puts items into a menu or a submenu, one way for both: an item
   is [text, act, enabled, key, icon, main] - bold where main - and null a
   line; the icons stand where one item has one. act an array is a submenu -
   «Добавить ▸» of a subsystem (podsistema/15), of the root
   (series-2.595/35) - of items alike, opened beside its item. */
function menuItems(menu, items) {
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
    if (Array.isArray(act)) {
      button.classList.add('context-menu-parent');
      button.append(node('span', '▸', 'context-menu-key'));
      button.addEventListener('click', () => {
        menu.querySelector('.context-submenu')?.remove();
        const submenu = node('div', undefined, 'context-menu context-submenu');
        submenu.setAttribute('role', 'menu');
        menuItems(submenu, act);
        submenu.style.top = `${button.offsetTop}px`;
        submenu.style.left = `${button.offsetWidth}px`;
        menu.append(submenu);
        submenu.querySelector('button:not(:disabled)')?.focus();
      });
    } else {
      button.addEventListener('click', () => { closeContextMenu(); act(); });
    }
    menu.append(button);
  }
}

function showContextMenu(event, items) {
  event.preventDefault();
  closeContextMenu();
  const menu = node('div', undefined, 'context-menu');
  menu.setAttribute('role', 'menu');
  menuItems(menu, items);
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

/* questionCaption is the title of a window asking a question or warning:
   where the prototype writes «Конфигуратор», the product writes «ML Studio»
   (ML-STUDIO.md, «„Проект“ вместо „конфигурации“», the owner, 24.10.2026). */
const questionCaption = 'ML Studio';

/* askChoice asks a question in a dialog of the page and answers with the key
   of the button pressed, or null when the dialog is closed without one. */
function askChoice(question, explanation, answers) {
  return showDialog({title: question, caption: questionCaption, explanation, answers});
}

/* askDeleteObject is the one question before an object or a part of it is
   deleted, the prototype's (obshchee.md, «Удаление»: «Удалить объект
   конфигурации?», Да / Нет; the same on an attribute) in ML's words
   (ML-STUDIO.md, «„Проект" вместо „конфигурации"»), asked by the tree and by
   the panels of a window alike. Answers whether «Да» was pressed. */
async function askDeleteObject() {
  return await askChoice('Удалить объект проекта?', '', [['yes', 'Да'], [null, 'Нет']]) === 'yes';
}

/* moveInCircle moves the item at index of list one place up (by -1) or down
   (by 1) in place, round the ends as the prototype moves an object in its
   branch: the first up goes last, the last down goes first (obshchee.md,
   «Перемещение и сортировка»). Answers the index it stands at then. */
function moveInCircle(list, index, by) {
  const to = (index + by + list.length) % list.length;
  const [item] = list.splice(index, 1);
  list.splice(to, 0, item);
  return to;
}

/* moveWithin moves the item at index of list one place up (by -1) or down
   (by 1) in place, nothing at the ends - as the windows of a command
   interface move a command or a subsection (podsistema.md, «Досъёмка
   28.10.2026, поздний вечер»; konfiguraciya.md, «Рабочая область…»: the
   arrows at the ends do nothing). Answers whether it moved. */
function moveWithin(list, index, by) {
  const to = index + by;
  if (index < 0 || to < 0 || to >= list.length) return false;
  [list[index], list[to]] = [list[to], list[index]];
  return true;
}

/* radioFrame is a frame of the prototype's window with its caption on the
   border and a choice of one in it: choices are [value, text] in their
   order, value the one chosen - none while the object says none - and
   apply(value) is called when another is chosen; disabled leaves the choice
   shown and unavailable, as the prototype's «Тип кода» at a code of no
   length. */
function radioFrame(caption, choices, value, apply, disabled = false) {
  const frame = node('fieldset', undefined, 'studio-frame'), name = 'radio-' + crypto.randomUUID();
  frame.append(node('legend', caption));
  frame.disabled = disabled;
  for (const [key, text] of choices) {
    const label = node('label', undefined, 'studio-radio'), input = node('input');
    input.type = 'radio'; input.name = name; input.value = key; input.checked = key === value;
    input.addEventListener('change', () => { if (input.checked) apply(key); });
    label.append(input, node('span', text));
    frame.append(label);
  }
  return frame;
}

/* searchField is the line of search over a list of the prototype's - the
   window «Выбор объекта», the palette: the text and its clearing cross,
   changed() called on every change. Answers the line and its input. */
function searchField(className, changed) {
  const search = textInput('', changed, {placeholder: 'Поиск'});
  search.setAttribute('aria-label', 'Поиск');
  const line = node('div', undefined, className);
  line.append(search, clearButton('Очистить поиск', () => { search.value = ''; changed(); search.focus(); }));
  return [line, search];
}

/* markBox is the box of a row of a tree of marks, standing before its icon
   (treeNode's mark): checked while marked holds key, which it adds or takes
   away; changed() is told of each mark. */
function markBox(caption, marked, key, changed = () => {}) {
  const box = node('input');
  box.type = 'checkbox'; box.checked = marked.has(key); box.className = 'object-choice-box';
  box.setAttribute('aria-label', caption);
  box.addEventListener('change', () => { if (box.checked) marked.add(key); else marked.delete(key); changed(); });
  return box;
}

/* objectList is a list of objects of a page of the window of an object, the
   prototype's (docs/block-00/spravochnik.md, pages «Владельцы» and «Ввод на
   основании»; snapshots spravochnik/okno-05, okno-12): its caption with
   «Изменить» and «Удалить» at the right and the rows beneath, one of them
   current. items are [key, caption]; choose(key) makes a row current;
   change() is «Изменить» and a double click on a row; remove(key) takes the
   current row away, unavailable without rows. Answers [head, list]. */
function objectList(caption, items, current, choose, change, remove) {
  const head = node('div', undefined, 'object-window-list-head');
  const edit = iconButton('change', 'Изменить'), drop = iconButton('delete', 'Удалить');
  edit.addEventListener('click', change);
  drop.disabled = !items.length;
  drop.addEventListener('click', () => remove(current));
  head.append(node('span', caption), edit, drop);
  return [head, objectRows(items, current, choose, change)];
}

/* objectRows is the list of rows of objectList, without its head: items are
   [key, caption] or [key, caption, sign, mark] - sign an element drawn
   before the caption, an icon or an image, mark one at the right end of the
   row - one of them current; choose(key) makes a row current, open() is a
   double click on a row. */
function objectRows(items, current, choose, open = () => {}) {
  const list = node('div', undefined, 'object-window-list');
  list.setAttribute('role', 'listbox');
  for (const [key, text, sign, mark] of items) {
    const row = node('div', sign || mark ? undefined : text, 'object-window-list-row' + (key === current ? ' current' : ''));
    if (sign || mark) row.append(...(sign ? [sign] : []), node('span', text), ...(mark ? [mark] : []));
    row.title = text;
    row.setAttribute('role', 'option');
    row.setAttribute('aria-selected', String(key === current));
    row.addEventListener('click', () => choose(key));
    row.addEventListener('dblclick', open);
    list.append(row);
  }
  return list;
}

/* choiceOrder is the identifiers of the objects of a tree of «Выбор
   объекта» in its order: of a branch's branches first, then its own. */
function choiceOrder(tree) {
  const order = [];
  (function collect(groups) { for (const group of groups) { collect(group.children || []); for (const item of group.objects || []) order.push(item.id); } })(tree);
  return order;
}

/* chooseObjects is the platform's window «Выбор объекта» of objects of the
   project marked by boxes (docs/block-00/spravochnik.md, the window of the
   owners; snapshots spravochnik/vladelcy-pyat-vidov, vladelcy-vybor-obekta):
   a search over the names with its clearing, the tree of the branches of the
   kinds offered - «Общие» holding its own, as the project tree - with a box
   at every object and none at a branch, «ОК» and «Отмена» in a column at the
   right. tree is the server's (objectChoiceTree): [{title, icon, children,
   objects: [{id, name, icon}]}]; chosen are the identifiers marked first;
   open shows the branches open, as «Выбор объекта: Роль» of «Отбор по
   ролям» shows its root (snapshot series-2.603/45); first shows the first
   branch open at each level, as «Редактировать состав…» of «Все подсистемы»
   opens the root, «Общие» and «Общие модули» (snapshot predsyomka-1/25).
   Answers the identifiers marked, in the order of the tree, on «ОК», and
   null on «Отмена». */
async function chooseObjects(title, tree, chosen, {open = false, first = false} = {}) {
  const marked = new Set(chosen), content = node('div', undefined, 'object-choice'), order = choiceOrder(tree);
  const [searchRow, search] = searchField('object-choice-search', () => draw());
  const rows = node('div', undefined, 'object-choice-tree');
  content.append(searchRow, rows);
  function draw() {
    const text = search.value.trim().toLocaleLowerCase();
    const branch = (group, leading) => {
      const objects = (group.objects || []).filter(item => !text || item.name.toLocaleLowerCase().includes(text)).map(item => {
        return treeNode({icon: item.icon, caption: item.name, mark: markBox(item.name, marked, item.id)}).element;
      });
      const children = [...(group.children || []).map((inner, at) => branch(inner, leading && at === 0)).filter(Boolean), ...objects];
      if (text && !children.length) return null;
      return treeNode({icon: group.icon, caption: group.title, children, open: open || !!text || leading}).element;
    };
    rows.replaceChildren(...tree.map((group, at) => branch(group, first && at === 0)).filter(Boolean));
  }
  draw();
  const answer = showDialog({title, content, side: true, className: 'object-choice-dialog', answers: [['ok', 'ОК'], [null, 'Отмена']]});
  search.focus();
  return await answer === 'ok' ? order.filter(id => marked.has(id)) : null;
}

/* chooseOneObject is the platform's window of the choice of one object of
   the project - «Выбор управляемой формы» of the home page
   (konfiguraciya.md, «Досъёмка 27.10.2026»; snapshot series-2.595/42): the
   search over the full names with its clearing, the tree of the branches -
   «Общие» holding its own - and OK, «Отмена» in a column at the right. tree
   is the server's (objectChoiceTree): an object holding items is a row to
   open, chosen by one of them; any other is chosen itself. current is the
   identifier chosen first: the tree opens down to it. A double click
   chooses too. Answers the identifier chosen on OK - OK is unavailable
   while nothing is - and null on «Отмена». */
async function chooseOneObject(title, tree, current) {
  let chosen = current || null;
  const content = node('div', undefined, 'object-choice'), rows = node('div', undefined, 'object-choice-tree');
  const [searchRow, search] = searchField('object-choice-search', () => draw());
  content.append(searchRow, rows);
  let accept = () => {};
  const leaf = item => {
    const {element, row} = treeNode({icon: item.icon, caption: item.name});
    row.classList.toggle('selected', item.id === chosen);
    row.title = item.fullName || item.name;
    row.addEventListener('click', () => { chosen = item.id; draw(); });
    row.addEventListener('dblclick', () => { chosen = item.id; accept(); });
    return element;
  };
  function draw() {
    const text = search.value.trim().toLocaleLowerCase();
    const found = item => !text || (item.fullName || item.name).toLocaleLowerCase().includes(text);
    const holds = item => item.id === chosen || (item.items || []).some(holds);
    const object = item => {
      if (!item.items) return found(item) ? {element: leaf(item), open: holds(item)} : null;
      const inner = item.items.filter(found).map(leaf);
      if (!inner.length) return null;
      return {element: treeNode({icon: item.icon, caption: item.name, children: inner, open: !!text || holds(item)}).element, open: holds(item)};
    };
    const branch = group => {
      const children = [...(group.children || []).map(branch), ...(group.objects || []).map(object)].filter(Boolean);
      if (text && !children.length) return null;
      const open = !!text || children.some(child => child.open);
      return {element: treeNode({icon: group.icon, caption: group.title, children: children.map(child => child.element), open}).element, open};
    };
    rows.replaceChildren(...tree.map(branch).filter(Boolean).map(item => item.element));
    rows.querySelector('.selected')?.scrollIntoView?.({block: 'nearest'});
  }
  draw();
  const answer = showDialog({title, content, side: true, className: 'object-choice-dialog',
    answers: [['ok', 'OK', {enabled: () => !!chosen}], [null, 'Отмена']]});
  const dialog = content.closest('dialog');
  accept = () => dialog.querySelector('button.primary')?.click();
  search.focus();
  return await answer === 'ok' ? chosen : null;
}

/* The visibility of an item of the command interface or of the home page
   to a role (obshchee.md, «Видимость по ролям»): what the roles say -
   project.InterfaceVisibility.Roles, [{role, visible}] - leaves a role
   'default' while it says nothing, 'on' or 'off' where it does. A click on
   the box of a role goes round, as the prototype's does in «Настройка
   видимости» and in the list of commands: the default, off, on, the default
   again (turnRoleVisibility answers what the roles say after it). */
function roleVisibilityState(said, role) {
  const item = (said || []).find(one => one.role === role);
  return item ? (item.visible ? 'on' : 'off') : 'default';
}
function turnRoleVisibility(said, role) {
  const next = [...(said || [])], at = next.findIndex(item => item.role === role);
  if (at < 0) next.push({role, visible: false});
  else if (!next[at].visible) next[at] = {role, visible: true};
  else next.splice(at, 1);
  return next;
}

/* roleVisibilityBox is the box of a role in three states - grey and marked
   for the default - labelled label; click() is a click on it. */
function roleVisibilityBox(state, label, click) {
  const box = node('span', undefined, 'role-visibility');
  box.dataset.state = state;
  box.tabIndex = 0;
  box.setAttribute('role', 'checkbox');
  box.setAttribute('aria-checked', {default: 'mixed', on: 'true', off: 'false'}[state]);
  box.setAttribute('aria-label', label);
  box.addEventListener('click', event => { event.stopPropagation(); click(); });
  box.addEventListener('keydown', event => { if (event.key === ' ') { event.preventDefault(); click(); } });
  return box;
}

/* visibilityDiffers says the roles see an item differently: one of them says
   other than the common box (the link «Различная» of the home page; question
   В4 of series 2.646). */
function visibilityDiffers(visibility) {
  return (visibility.roles || []).some(item => item.visible !== visibility.common);
}

/* showVisibilitySettings is the platform's window «Настройка видимости»
   (obshchee.md, «Видимость по ролям»; snapshot series-2.595/44): the box
   «Видимость» - the common one - and under «Видимость по ролям:» a row a
   role of the project, by its synonym, in its order, with its box in three
   states (roleVisibilityBox); OK, «Отмена», «Справка». visibility is
   project.InterfaceVisibility, roles the project's ([{id, title}]). Answers
   the visibility on OK - the roles in the order they were said - and null
   on «Отмена». */
async function showVisibilitySettings(visibility, roles) {
  let common = visibility.common, said = [...(visibility.roles || [])];
  const content = node('div', undefined, 'visibility-settings'), list = node('div', undefined, 'visibility-settings-roles');
  content.append(checkboxField('Видимость', common, value => { common = value; }, 'visibility-settings-common'), node('div', 'Видимость по ролям:'), list);
  const draw = () => list.replaceChildren(...roles.map(role => {
    const row = node('div', undefined, 'visibility-settings-role');
    row.append(roleVisibilityBox(roleVisibilityState(said, role.id), 'Видимость: ' + role.title, () => { said = turnRoleVisibility(said, role.id); draw(); }),
      node('span', role.title));
    return row;
  }));
  draw();
  if (await showDialog({title: 'Настройка видимости', content, className: 'visibility-settings-dialog', help: 'visibility-settings',
    answers: [['ok', 'OK'], [null, 'Отмена']]}) !== 'ok') return null;
  return {common, ...(said.length ? {roles: said} : {})};
}

/* chooseFromList is the platform's window of a choice of one from a list:
   title, the caption head over the list, items [key, caption, sign, mark]
   in their order (objectRows), current the row current at first - the
   first without it; OK is unavailable on a row available(key) refuses; a
   double click is OK. The windows: «Выбор группы команд» (podsistema.md,
   «Досъёмка 28.10.2026, поздний вечер»; snapshots vopr-2.636/07, 09, 26),
   «Тип элемента» and «Выбор стандартной панели» of the client interface
   (konfiguraciya.md, «Досъёмка 27.10.2026»; series-2.595/54, 55). Answers
   the key of the row current on OK, null on «Отмена». */
async function chooseFromList({title, head, items, current = items[0]?.[0], available = () => true, help}) {
  const content = node('div', undefined, 'list-choice'), rows = node('div');
  let accept = () => {};
  const draw = () => rows.replaceChildren(objectRows(items, current, key => { current = key; draw(); }, () => accept()));
  content.append(...(head ? [node('div', head, 'list-choice-head')] : []), rows);
  draw();
  const answer = showDialog({title, content, className: 'list-choice-dialog', help,
    answers: [['ok', 'OK', {enabled: () => available(current)}], [null, 'Отмена']]});
  const ok = [...content.closest('dialog').querySelectorAll('button')].find(button => button.textContent === 'OK');
  accept = () => { if (available(current)) ok.click(); };
  return await answer === 'ok' ? current : null;
}

/* chooseCommandGroup is «Выбор группы команд» of the windows of a command
   interface: the list «Имя» of the groups offered, [key, caption] in their
   order. */
function chooseCommandGroup(groups) {
  return chooseFromList({title: 'Выбор группы команд', head: 'Имя', items: groups});
}

/* PaletteView is how the palette of properties is shown, the same for every
   object until changed, as the prototype's palette keeps its buttons
   pressed from object to object: by categories (tabs) or by the alphabet,
   only the important properties or all, and the tab last chosen. */
const PaletteView = {alphabetical: false, important: false, tab: ''};

/* objectPalette is the palette of properties of an object, one for every
   kind (docs/block-00/obolochka.md, «Палитра свойств»; obshchee.md,
   «Палитра свойств»): the buttons «Сортировка по алфавиту», «Сортировка по
   категориям», «Показывать только важные», «Отменить редактирование» and
   «Применить»; the search, which lists the properties of every tab whose
   caption holds the text, the match marked; the tabs of the categories; a
   row a property, its caption at the left; at the bottom the explanation of
   the property chosen. tabs are the kind's (kinds.Palette): {key, title,
   important, rows: [{key, title, field, important, description}]} - a tab
   «только важные» keeps is important, and so is a row it keeps. control(row)
   builds the field of a row, null for a row the object has no field for.
   «Отменить редактирование» brings back what the field held when it was
   entered, «Применить» leaves it - both available while a field is edited.
   visible(row) says a row stands at all for the object shown (the rows of
   an attribute by its type). Answers the element. */
function objectPalette(tabs, control, visible = () => true) {
  const element = node('div', undefined, 'object-palette');
  const bar = node('div', undefined, 'object-palette-bar');
  const alphabet = iconButton('sort-alphabetical', 'Сортировка по алфавиту'), categories = iconButton('sort-by-category', 'Сортировка по категориям');
  const important = iconButton('only-important', 'Показывать только важные');
  const cancel = iconButton('cancel-edit', 'Отменить редактирование'), apply = iconButton('apply', 'Применить');
  const [searchRow, search] = searchField('object-palette-search', () => draw());
  const strip = node('div'), rows = node('div', undefined, 'object-palette-rows'), explanation = node('div', undefined, 'object-palette-explanation');
  bar.append(alphabet, categories, important, cancel, apply);
  element.append(bar, searchRow, strip, rows, explanation);
  alphabet.addEventListener('click', () => { PaletteView.alphabetical = true; draw(); });
  categories.addEventListener('click', () => { PaletteView.alphabetical = false; draw(); });
  important.addEventListener('click', () => { PaletteView.important = !PaletteView.important; draw(); });
  // The field being edited and what it held when it was entered.
  let editing = null, before = '';
  function arrange() {
    const changed = !!editing && editing.value !== before;
    cancel.disabled = !changed; apply.disabled = !changed;
  }
  rows.addEventListener('focusin', event => {
    const field = event.target;
    if (!field.matches('input:not([type=checkbox]), textarea, select')) return;
    editing = field; before = field.value; arrange();
  });
  rows.addEventListener('input', arrange);
  rows.addEventListener('focusout', () => setTimeout(() => { if (!rows.contains(document.activeElement)) { editing = null; arrange(); } }));
  cancel.addEventListener('mousedown', event => event.preventDefault());
  apply.addEventListener('mousedown', event => event.preventDefault());
  cancel.addEventListener('click', () => {
    if (!editing) return;
    editing.value = before;
    editing.dispatchEvent(new Event('input', {bubbles: true}));
    editing.dispatchEvent(new Event('change', {bubbles: true}));
    arrange();
  });
  apply.addEventListener('click', () => { editing?.blur(); editing = null; arrange(); });
  const kept = item => (!PaletteView.important || item.important) && (!item.field || visible(item));
  function line(row, text) {
    if (row.field === 'separator') return node('hr', undefined, 'object-palette-separator');
    const label = node('div', undefined, 'object-palette-row'), caption = node('span', undefined, 'object-palette-caption');
    // A caption cut by a narrow palette is read whole in its hint.
    caption.title = row.title;
    const at = text ? row.title.toLocaleLowerCase().indexOf(text) : -1;
    if (at < 0) caption.textContent = row.title;
    else caption.append(row.title.slice(0, at), node('mark', row.title.slice(at, at + text.length), 'object-palette-match'), row.title.slice(at + text.length));
    label.append(caption, control(row) || node('span'));
    label.addEventListener('focusin', () => { explanation.textContent = row.description || ''; });
    label.addEventListener('click', () => { explanation.textContent = row.description || ''; });
    return label;
  }
  function draw() {
    const text = search.value.trim().toLocaleLowerCase(), shown = tabs.filter(kept);
    alphabet.setAttribute('aria-pressed', String(PaletteView.alphabetical));
    categories.setAttribute('aria-pressed', String(!PaletteView.alphabetical));
    important.setAttribute('aria-pressed', String(PaletteView.important));
    let list;
    if (text || PaletteView.alphabetical) {
      strip.replaceChildren();
      list = shown.flatMap(tab => tab.rows.filter(kept)).filter(row => row.field !== 'separator' && (!text || row.title.toLocaleLowerCase().includes(text)));
      if (PaletteView.alphabetical) list = [...list].sort((left, right) => left.title.localeCompare(right.title, 'ru'));
    } else {
      if (!shown.some(tab => tab.key === PaletteView.tab)) PaletteView.tab = shown[0]?.key || '';
      strip.replaceChildren(tabStrip(shown.map(tab => [tab.key, tab.title]), PaletteView.tab, key => { PaletteView.tab = key; draw(); }));
      list = (shown.find(tab => tab.key === PaletteView.tab)?.rows || []).filter(kept);
    }
    rows.replaceChildren(...list.map(row => line(row, text)));
    editing = null; arrange();
  }
  draw();
  return element;
}

/* pathValue and setPathValue read and write a property of a description by
   its key, dotted for one inside another - code.length, presentation.mask -
   as the palette names it. A list, a text or a set of translations left
   empty is not written at all. */
function pathValue(object, path) {
  return path.split('.').reduce((value, key) => value?.[key], object);
}
function setPathValue(object, path, value) {
  const keys = path.split('.'), last = keys.pop();
  let target = object;
  for (const key of keys) target = target[key] ||= {};
  const empty = value === '' || (Array.isArray(value) ? !value.length : value && typeof value === 'object' && !Object.keys(value).length);
  if (empty) delete target[last]; else target[last] = value;
}

/* objectChoiceItem finds an object of a tree of the window «Выбор объекта»
   (objectChoiceTree) by its identifier. */
function objectChoiceItem(branches, id) {
  for (const branch of branches || []) {
    const item = branch.objects?.find(object => object.id === id) || objectChoiceItem(branch.children, id);
    if (item) return item;
  }
  return null;
}

/* typedObjectSubject is what the palette of an object holding a value of a
   type edits - a constant, a common attribute: the object (valueSubject),
   its types by the window of the type over source.typeOffer, its name
   checked at source.path; owner names its kind's folder. */
function typedObjectSubject(source, object, owner) {
  return valueSubject(object, source.typeOffer, types => { object.types = types; }, {owner,
    setName: value => { object.name = value; }, check: () => ({path: source.path, name: object.name})});
}

/* valueSubject is what the palette edits of anything that holds a value of
   a type - an attribute, a constant: its properties by their keys, and the
   qualifiers of its one type (type.length, type.precision…) written by the
   model of the window of the type (typeEditorModel), by its rules and
   ceilings - a value beyond one is refused (false). setTypes(types) writes
   the types; own are the subject's own: owner, setName, check. types are
   the kinds of its types as the palette tells them apart: string, number,
   date, boolean, reference and the rest by their kind. */
function valueSubject(target, offer, setTypes, own) {
  return {target, types: (target.types || []).map(type => type.reference ? 'reference' : type.kind), ...own,
    get(path) {
      if (!path.startsWith('type.')) return pathValue(target, path);
      const type = target.types?.[0] || {}, field = path.slice(5);
      return field === 'unlimited' ? !type.length : type[field];
    },
    set(path, value) {
      if (!path.startsWith('type.')) { setPathValue(target, path, value); return true; }
      const types = typeEditorModel(offer, target.types);
      if (types.setQualifier(path.slice(5), value) === false) return false;
      setTypes(types.result().types);
      return true;
    }};
}

/* paletteShown answers whether a row stands for a subject at all: one of
   the conditions it is shown by holds - type=<kind> by any type of a value,
   so a composite type shows the rows of each of its kinds once, owner=<kind>
   by the object a part belongs to, any other <key>=<value> by that
   property of the subject - the panel of the window of the client interface
   (kinds.PaletteRow.Shown). */
function paletteShown(subject) {
  return row => !row.shown?.length || row.shown.some(condition => {
    const [key, value] = condition.split('=');
    return key === 'type' ? (subject.types || []).includes(value) : subject[key] === value;
  });
}

/* paletteFields answers the builder of the field of a row of a palette by
   its kind (kinds.PaletteRow), one for every subject - an object, a part of
   it, a constant: the name with its synonym following it, a text in every
   language, a plain text, «Открыть» of a module or a window, a flag, a
   list, a number, a switch, a list of objects or fields, a main form, the
   type, a value. context: project (languages), typeOffer, choices,
   cleanups() - where a field that follows the language of Studio leaves its
   end, typed() - after a keystroke, changed() - after anything else,
   openModule(role), windows - the windows the subject opens and
   openWindow(key), openFile(path), form(value) - the name of a main form
   and the path it opens at, choose(row, items) - the objects chosen for a
   row, null for none. A subject is {target, get(path), set(path, value),
   setName(value), check()}. */
function paletteFields(context) {
  let synonym = null;
  // A row is available while every property it needs is on: not false,
  // zero or empty, or the value named (kinds.PaletteRow.Needs).
  const available = (subject, row) => (row.needs || []).every(need => {
    const [path, wanted] = need.split('='), value = subject.get(path);
    if (wanted !== undefined) return value === wanted;
    return Array.isArray(value) ? value.length > 0 : !!value;
  });
  const clearing = (row, act, off) => {
    const clear = clearButton(`Очистить: ${row.title}`, act);
    clear.classList.add('field-button');
    clear.disabled = off;
    return clear;
  };
  // A list of objects or of fields in one line: their names by commas, «…»
  // choosing them and the cross clearing them; the fields - of the data
  // lock, of the input by string, the link by type - wait for their window,
  // which the inventory does not show.
  function listField(subject, row, value, off) {
    const items = Array.isArray(value) ? value : value ? [value] : [];
    const shown = node('input');
    shown.readOnly = true; shown.disabled = off;
    shown.value = row.field === 'objects' ? items.map(id => objectChoiceItem(context.choices?.[row.key], id)?.fullName || id).join(', ')
      : items.map(field => field.standard || field.attribute || [].concat(field.source || []).join('.')).join(', ');
    const pick = fieldChoiceButton(row.title, async () => {
      const chosen = await context.choose?.(row, items);
      if (chosen) { subject.set(row.key, chosen); context.changed(); }
    });
    pick.disabled = off || row.field === 'fields';
    return fieldWithButtons(shown, [pick, clearing(row, () => { subject.set(row.key, []); context.changed(); }, off || !items.length)]);
  }
  // A main form: its name, «…» choosing it - its window the inventory does
  // not show, unavailable until shot - the cross and the magnifier opening
  // it.
  function formField(subject, row, value, off) {
    const form = value ? context.form(value) : {name: ''};
    const shown = node('input');
    shown.readOnly = true; shown.value = form.name; shown.disabled = off;
    const pick = fieldChoiceButton(row.title, () => {});
    pick.disabled = true;
    const open = fieldButton('field-translations', `Открыть: ${row.title}`, () => context.openFile(form.path));
    open.disabled = off || !form.name;
    return fieldWithButtons(shown, [pick, clearing(row, () => { subject.set(row.key, ''); context.changed(); }, off || !form.name), open]);
  }
  const field = (subject, row) => {
    const target = subject.target;
    if (row.field === 'name') {
      return nameInput(target.name, value => { const before = target.name; subject.setName(value); synonym?.follow(before, value); context.typed(); }, subject.check);
    }
    if (row.field === 'localized') {
      const field = createLocalizedField(context.project, code => subject.get(row.key)?.[code],
        (language, value) => subject.set(row.key, setTranslation({...subject.get(row.key)}, language, value)), context.typed,
        row.lines ? {kind: 'area', rows: row.lines} : titleTextOptions);
      context.cleanups().push(field.destroy);
      if (row.key === 'title') synonym = field;
      return field;
    }
    // A namespace is shown and typed by its short name where it is the
    // platform's, and written whole (namespaceShown, namespaceTyped).
    if (row.field === 'namespace') {
      return textInput(namespaceShown(subject.get(row.key) || ''), value => { subject.set(row.key, namespaceTyped(value)); context.typed(); });
    }
    if (row.field === 'text') {
      const input = textInput(subject.get(row.key), value => { subject.set(row.key, value); context.typed(); });
      input.disabled = !!row.fixed;
      // A value the server checks when its field is left - the root of an
      // HTTP service (context.valueChecks, by the key of the row).
      if (context.valueChecks?.[row.key]) watchValue(input, context.valueChecks[row.key], () => ({value: input.value}));
      return input;
    }
    // A password kept apart from the project: dots when there is one; Studio
    // does not write the protected store yet (series 2.584, В4).
    if (row.field === 'password') {
      const shown = node('input');
      shown.type = 'password'; shown.readOnly = true; shown.disabled = true;
      shown.value = subject.get(row.key) ? '••••••••' : '';
      return shown;
    }
    // The headers of a WebSocket client, «Имя(значение)» by commas - a
    // secret one by dots -, «…» opening «Настройка заголовков»
    // (context.editHeaders), the cross clearing those that are not secret.
    if (row.field === 'headers') {
      const items = subject.get(row.key) || [], shown = node('input');
      shown.readOnly = true;
      shown.value = items.map(item => `${item.name}(${item.secret ? '•••' : item.value || ''})`).join(', ');
      const pick = fieldChoiceButton(row.title, async () => {
        const edited = await context.editHeaders?.(items);
        if (edited) { subject.set(row.key, edited); context.changed(); }
      });
      return fieldWithButtons(shown, [pick, clearing(row, () => { subject.set(row.key, items.filter(item => item.secret)); context.changed(); },
        !items.some(item => !item.secret))]);
    }
    // A type named by the schema: «string (http://www.w3.org/2001/XMLSchema)»,
    // «…» choosing it and the cross unavailable until the window of the
    // types of the packages (a type of an operation is never cleared:
    // web-servis.md, «Проверки»).
    if (row.field === 'xml-type' || row.field === 'xml-types') {
      const types = row.field === 'xml-types' ? subject.get(row.key) || [] : [subject.get(row.key)].filter(Boolean), shown = node('input');
      shown.readOnly = true; shown.disabled = !available(subject, row);
      shown.value = row.field === 'xml-types' ? types.map(xmlTypeName).join('; ') : types.filter(type => type.name).map(xmlTypeText).join(', ');
      // «…» chooses a type by chooseXMLType where the place gives the types
      // (context.xmlTypes) and the namespaces it offers (context.typeNamespaces);
      // a list of types - the members of a union - by editValueList, a row
      // by the same window; the cross clears a list.
      const pick = fieldChoiceButton(row.title, async () => {
        const chosen = row.field === 'xml-types' ? await chooseXMLTypes(types) : await chooseXMLType(types[0], context.typeNamespaces(), context.xmlTypes());
        if (chosen) { subject.set(row.key, chosen); context.changed(); }
      }), clear = clearButton(`Очистить: ${row.title}`, () => { subject.set(row.key, []); context.changed(); });
      pick.disabled = shown.disabled || !context.xmlTypes; clear.disabled = row.field !== 'xml-types' || shown.disabled || !types.length;
      clear.classList.add('field-button');
      return fieldWithButtons(shown, [pick, clear]);
    }
    // The XDTO packages of a web service: their namespaces - a package of
    // the project named by its own (context.packages) - «…» choosing them
    // in «Выбор пакетов XDTO», the cross clearing them.
    if (row.field === 'packages') {
      const items = subject.get(row.key) || [], shown = node('input'), off = !available(subject, row);
      shown.readOnly = true; shown.disabled = off;
      shown.value = items.map(item => item.package ? context.packages?.find(own => own.id === item.package)?.namespace || item.package : namespaceShown(item.namespace)).join(',');
      const pick = fieldChoiceButton(row.title, async () => {
        const chosen = await chooseXDTOPackages(items, context.platformNamespaces, context.packages);
        if (chosen) { subject.set(row.key, chosen); context.changed(); }
      });
      pick.disabled = off || !context.platformNamespaces;
      return fieldWithButtons(shown, [pick, clearing(row, () => { subject.set(row.key, []); context.changed(); }, off || !items.length)]);
    }
    // An address shown and not typed into: a page of the network opens in
    // the browser, a file is only shown.
    if (row.field === 'link') {
      const address = subject.get(row.key) || '', web = /^https?:\/\//i.test(address);
      const link = node(web ? 'a' : 'span', address, 'object-palette-link object-palette-address');
      link.title = address;
      if (web) { link.href = address; link.target = '_blank'; link.rel = 'noopener'; }
      return link;
    }
    // One object of the project by its identifier - a document's numerator
    // (dokument.md, «Палитра»): its name among what the window «Выбор
    // объекта» of the row offers (context.choices), the cross taking it
    // away; which window «…» opens the inventory does not show (series
    // 2.618, question): it stands unavailable.
    // A standard panel of the window of the client interface
    // (konfiguraciya.md, «Предсъёмка 28.10.2026»; predsyomka-1/42): the
    // panel by its caption (context.standardPanel), «…» choosing another
    // (context.chooseStandardPanel), no cross.
    if (row.field === 'standard-panel') {
      const shown = node('input');
      shown.readOnly = true; shown.value = context.standardPanel(subject.get(row.key));
      return fieldWithButtons(shown, [fieldChoiceButton(row.title, () => context.chooseStandardPanel())]);
    }
    if (row.field === 'object') {
      const id = subject.get(row.key), shown = node('input'), off = !available(subject, row);
      shown.readOnly = true; shown.disabled = off;
      shown.value = id ? objectChoiceItem(context.choices?.[row.key], id)?.name || id : '';
      const pick = fieldChoiceButton(row.title, () => {});
      pick.disabled = true;
      return fieldWithButtons(shown, [pick, clearing(row, () => { subject.set(row.key, undefined); context.changed(); }, off || !id)]);
    }
    // A picture: of the platform by its Russian name, of the project by
    // its own (metadata.PictureReference, PictureChoice); «…» chooses it in
    // «Выбор картинки», the cross takes it away. A picture of a file of the
    // object's own is shown by the file: its tabs «Из файла» are a question
    // of series 2.618, and «…» stands unavailable on it.
    if (row.field === 'picture') {
      const picture = subject.get(row.key), shown = node('input'), names = context.project?.pictures;
      shown.readOnly = true;
      shown.value = !picture ? '' : picture.standard ? names?.standard.find(item => item.name === picture.standard)?.russian || picture.standard
        : picture.common ? names?.common.find(item => item.id === picture.common)?.name || picture.common : picture.file || '';
      const pick = fieldChoiceButton(row.title, async () => {
        const chosen = await choosePicture(picture, names);
        if (chosen === undefined) return;
        subject.set(row.key, chosen || undefined); context.changed();
      });
      pick.disabled = !names || !!picture?.file || !!picture?.variants;
      return fieldWithButtons(shown, [pick, clearing(row, () => { subject.set(row.key, undefined); context.changed(); }, !picture)]);
    }
    if (row.field === 'module' || row.field === 'window') {
      const link = node('button', 'Открыть', 'object-palette-link');
      link.type = 'button';
      // A window the subject opens is among context.windows; those of the
      // standard attributes, characteristics, predefined items, additional
      // indexes, the help and the choice of a field are items of their own.
      link.disabled = row.field === 'window' && !context.windows?.includes(row.key);
      link.addEventListener('click', () => row.field === 'window' ? context.openWindow(row.key) : context.openModule(row.key));
      return link;
    }
    if (row.field === 'type') {
      const shown = node('input');
      shown.readOnly = true;
      shown.value = typePresentation(context.typeOffer, target.types);
      const pick = fieldChoiceButton('Редактирование типа данных', async () => {
        const types = await showTypeEditor(context.typeOffer, target.types);
        if (types) { subject.set('types', types); context.changed(); }
      });
      return fieldWithButtons(shown, [pick]);
    }
    const value = subject.get(row.key), off = !!row.fixed || !available(subject, row);
    const set = next => { if (subject.set(row.key, next) === false) return false; context.changed(); };
    let control;
    if (row.field === 'flag') {
      control = node('input');
      control.type = 'checkbox'; control.checked = !!value;
      control.addEventListener('change', () => set(control.checked));
    } else if (row.field === 'choice') {
      control = selectInput(row.choices, value || row.default || '', set, row.title);
    } else if (row.field === 'radio') {
      return radioFrame(row.title, row.choices, value || row.default || '', set, off);
    } else if (row.field === 'switch') {
      control = selectInput(row.choices, String(!!value), next => set(next === 'true'), row.title);
    } else if (row.field === 'number') {
      const options = {type: 'number', min: row.min || 0, ...(row.max ? {max: row.max} : {})};
      control = textInput(value ?? 0, set, row.limit ? limited(row.limit, options) : options);
    } else if (row.field === 'value') {
      // A value of the subject's type is entered by a field of that type,
      // which the inventory does not show yet: shown, not edited.
      control = node('input');
      control.readOnly = true; control.value = value?.data ?? '';
      control.disabled = true;
      return control;
    } else if (row.field === 'objects' || row.field === 'fields') {
      return listField(subject, row, value, off);
    } else if (row.field === 'form') {
      return formField(subject, row, value, off);
    } else return null;
    control.disabled = off;
    return control;
  };
  // The members of a union: «Редактирование списка значений», a row chosen
  // by the window of a type (paket-xdto.md, «Проверить пакет, экспорт
  // схемы, окно типа»).
  const chooseXMLTypes = types => editValueList(types, xmlTypeName, value => chooseXMLType(value, context.typeNamespaces(), context.xmlTypes()));
  // A value a node may leave not said (row.optional; paket-xdto.md,
  // «Палитра узла»): not said, the field shows the default greyed and ✓
  // says it - the default, or for a type the window choosing one, which is
  // an item of its own; said, the field is as any other, ✕ takes it back.
  function optional(subject, row) {
    const value = subject.get(row.key);
    if (value === undefined) {
      const shown = node('input', undefined, 'palette-unset');
      shown.readOnly = true; shown.disabled = true;
      shown.value = (row.choices || []).find(([key]) => key === row.default)?.[1] ?? row.default ?? '';
      // ✓ of a type opens the window choosing it.
      const say = fieldButton('apply', `Задать значение: ${row.title}`, async () => {
        if (row.field === 'xml-type' || row.field === 'xml-types') {
          const chosen = row.field === 'xml-types' ? await chooseXMLTypes([]) : await chooseXMLType(null, context.typeNamespaces(), context.xmlTypes());
          if (chosen) { subject.set(row.key, chosen); context.changed(); }
          return;
        }
        const given = {number: Number(row.default || 0), switch: row.default === 'true', choice: row.default || row.choices?.[0]?.[0]}[row.field];
        subject.set(row.key, given ?? '');
        context.changed();
      });
      say.disabled = (row.field === 'xml-type' || row.field === 'xml-types') && !context.xmlTypes;
      return fieldWithButtons(shown, [say]);
    }
    return fieldWithButtons(field(subject, row), [clearing(row, () => { subject.set(row.key, undefined); context.changed(); }, false)]);
  }
  return (subject, row) => row.optional ? optional(subject, row) : field(subject, row);
}

/* rowsPage is a page of the window of an object built of the palette's
   rows (kinds.WindowRows): each row of the page takes its field from the
   palette's row of the same key (palette, fields of paletteFields for
   subject) with the window's caption; a flag stands with its caption after
   it, a row of a module is a button opening it (openModule), a row of a
   window a button opening it where the kind has it (context.windows,
   context.openWindow - the palette's own), a row in a
   frame stands in a frame of its caption, a caption above stands above its
   field. */
function rowsPage(windowRows, palette, fields, subject, openModule, context = {}) {
  const page = node('div', undefined, 'object-window-fields');
  const paletteRows = (palette || []).flatMap(tab => tab.rows || []);
  let frame = null;
  for (const row of windowRows || []) {
    const base = paletteRows.find(item => item.key === row.key);
    if (!base) continue;
    let element;
    if (row.button) {
      element = node('button', row.title, 'object-window-button');
      element.type = 'button';
      // A button of a window opens it where the kind has it, as the link
      // of the palette does (paletteFields).
      const opens = base.field === 'window';
      element.disabled = opens && !context.windows?.includes(row.key);
      element.addEventListener('click', () => opens ? context.openWindow(row.key) : openModule(row.key));
    } else if (row.radio) {
      // A list of values drawn as the frame of radio buttons of its title
      // (snapshot dokument/03, «Тип номера»).
      element = fields(subject, {...base, title: row.title, field: 'radio'});
    } else {
      const control = fields(subject, {...base, title: row.title});
      if (base.field === 'flag') {
        element = node('label', undefined, 'catalog-check object-window-flag');
        element.append(control, node('span', row.title));
      } else element = labeledField(row.title, control, row.above ? 'catalog-field catalog-field-above' : 'catalog-field');
    }
    if (!row.frame) { frame = null; page.append(element); continue; }
    if (frame?.dataset.frame !== row.frame) {
      frame = node('fieldset', undefined, 'studio-frame object-window-fields');
      frame.dataset.frame = row.frame;
      frame.append(node('legend', row.frame));
      page.append(frame);
    }
    frame.append(element);
  }
  return page;
}

/* optionsPage is the page «Функциональные опции» of the window of an
   object (spravochnik.md, page 3; podsistema.md, «Окно»; snapshots
   okno-03, podsistema/05): «Функциональные опции, в которых участвует
   <вид>:» over the options of the project (places, MemberPlace) in the
   order of the tree, a box at each marked where the object is in its
   content (options, a Set the boxes change); changed() is called on every
   mark. The first row stands chosen, and the row chosen keeps the focus
   when the list is drawn anew. */
function optionsPage(places, options, changed, kind) {
  const page = node('div', undefined, 'object-window-fields');
  let list, current = places?.[0]?.id ?? null;
  const draw = () => {
    const fresh = checkList((places || []).map(place => ({key: place.id, caption: place.name, checked: options.has(place.id)})), current,
      (key, checked) => { if (checked) options.add(key); else options.delete(key); changed(); draw(); },
      key => { current = key; draw(); });
    fresh.classList.add('object-window-checks');
    if (list) { const focused = list.contains(document.activeElement); list.replaceWith(fresh); if (focused) fresh.focus(); }
    list = fresh;
  };
  draw();
  page.append(node('div', `Функциональные опции, в которых участвует ${kind}:`, 'object-window-caption'), list);
  return page;
}

/* subsystemsPage is the page «Подсистемы» of the window of an object, one
   for every kind (obshchee.md, «Окно объекта»; snapshots
   configurator-2026-10-27/series-2.584/): «Подсистемы, в которых участвует
   <вид>:» over the tree (subsystemsTree) - the prototype's web and HTTP
   services word it otherwise, ML has one form - the word of the kind the
   server's (source.memberWord, kinds.Descriptor.MemberWord). */
function subsystemsPage(source, marks, changed) {
  const page = node('div', undefined, 'object-window-fields');
  page.append(node('div', `Подсистемы, в которых участвует ${source.memberWord}:`, 'object-window-caption'), subsystemsTree(source.subsystems, marks, changed));
  return page;
}

/* subsystemsTree is the tree of the page «Подсистемы» of the window of an
   object (spravochnik.md, page 2; snapshot okno-02): the subsystems of the
   project by their parents (places, MemberPlace), a box at each marked
   where the object is in its content (marks, a Set the boxes change);
   a subsystem holding a marked one beneath stands open. changed() is
   called on every mark. */
function subsystemsTree(places, marks, changed) {
  const under = parent => (places || []).filter(place => (place.parent || null) === parent);
  const holds = place => marks.has(place.id) || under(place.id).some(holds);
  const branch = place => {
    const children = under(place.id).map(branch);
    return treeNode({icon: 'subsystem', caption: place.name, mark: markBox(place.name, marks, place.id, changed), children,
      open: children.length > 0 && under(place.id).some(holds)}).element;
  };
  const tree = node('div', undefined, 'object-window-tree object-window-tree-tall');
  tree.append(...under(null).map(branch));
  return tree;
}

/* windowHeadFields are the fields at the top of the window of an object
   without pages - a common picture, a style (obshchaya-kartinka.md, stil.md,
   «Окно»): the name, the synonym, the comment, and the subsystems holding
   it (source.subsystems) by commas with «…», unavailable until the window
   choosing them. object is the object edited; typed() is called on every
   key; the synonym's cleanup goes to cleanups. */
function windowHeadFields(source, object, typed, cleanups) {
  const setText = (key, value) => { if (typeof value === 'string' ? value : Object.keys(value || {}).length) object[key] = value; else delete object[key]; };
  const synonym = createLocalizedField(source, code => object.title?.[code], (language, value) => setText('title', setTranslation(object.title, language, value)), typed, titleTextOptions);
  cleanups.push(synonym.destroy);
  const name = nameInput(object.name, value => { const before = object.name; object.name = value; synonym.follow(before, value); typed(); },
    () => ({path: source.path, name: object.name}));
  const subsystems = node('input');
  subsystems.readOnly = true;
  subsystems.value = (source.subsystems || []).filter(place => place.member).map(place => place.name).join(',');
  const choose = fieldChoiceButton('Подсистемы', () => {});
  choose.disabled = true;
  return [labeledField('Имя:', name), labeledField('Синоним:', synonym),
    labeledField('Комментарий:', textInput(object.comment, value => { setText('comment', value); typed(); })),
    labeledField('Подсистемы:', fieldWithButtons(subsystems, [choose]))];
}

/* objectWindowEditor is the editor of the window of an object: of a kind
   whose pages are the palette's rows (source.pages, source.windowRows),
   «Подсистемы» the tree of the subsystems, and pages of its own the kind
   gives (own); or, where own gives draw(body), of a kind whose window has
   no pages - a common picture, a style (obshchaya-kartinka.md, stil.md,
   «Окно»): its title and a body draw fills (windowHeadFields at the top).
   The palette of the object, or of the part own chose, stands beside it,
   each showing what the other typed. target(source) is the object; own(api)
   answers {pages: {key: () => element}, title() - the title of the window
   where it is not «<вид> <имя>», part() - {subject, title, tabs} of
   the part chosen or null, reset(), draw(body), subject(source) - what the
   palette of the object edits, its own properties by default, context() -
   what its fields open beyond the window's (windows, openWindow), value
   (object) - what the window writes of the object} over api - {source(),
   object(), changed(), typed(), refresh(), show(page), palette(),
   cleanups()};
   valueChecks are the addresses checking a value of a row when its field
   is left (paletteFields). around holds the palette, its title, the opening
   of a module, the actions and the closing of the window. */
function objectWindowEditor(host, onChange, around, {target, own = () => ({pages: {}}), valueChecks}) {
  let source = null, frame = null, marks = new Set(), options = new Set(), cleanups = [];
  const paletteCleanups = [];
  const object = () => target(source);
  const title = () => kind.title?.() || `${source.kindTitle} ${object().name}`;
  // The part chosen in the window - of the kind's own pages or of the common
  // ones - one for the window; its palette stands beside it.
  let chosen = null;
  const same = (left, right) => !!left && !!right && left.kind === right.kind && left.container === right.container && left.id === right.id;
  const choose = choice => { chosen = choice; frame.refresh(); renderPalette(); };
  const focusPalette = () => around.palette?.querySelector('input')?.focus();
  // A row of a tree of parts: a click chooses its part, a double click
  // chooses it and puts the focus into its palette.
  function treeRow(icon, caption, choice, children = []) {
    const {element, row} = treeNode({icon, caption, children, open: true});
    row.classList.toggle('selected', same(chosen, choice));
    row.addEventListener('click', () => choose(choice));
    if (choice.id) row.addEventListener('dblclick', event => { event.stopPropagation(); choose(choice); focusPalette(); });
    return element;
  }
  const api = {source: () => source, object, changed, typed, marked: () => onChange(), refresh: () => frame.refresh(), show: key => frame.show?.(key),
    palette: () => renderPalette(), cleanups: () => cleanups, rowsPage: key => rowsOf(key),
    chosen: () => chosen, setChosen: choice => { chosen = choice; }, choose, treeRow, focusPalette, addTemplate: () => common.addTemplate()};
  const kind = own(api);
  // The common pages of a kind whose window has them (commonObjectPages);
  // «Вводится на основании» is the object's own list.
  const common = commonObjectPages({source: () => source, object, host, setBasedOn: list => { if (list.length) object().basedOn = list; else delete object().basedOn; },
    changed, marked: onChange, refresh: () => frame.refresh(), ...api, openFile: path => around.openFile?.(path)});
  function context(collect) {
    return {project: source, typed, changed, packages: source.packages, platformNamespaces: source.platformNamespaces, windows: [], cleanups: collect, valueChecks,
      openModule: role => around.openModule?.(role), ...kind.context?.()};
  }
  // Typing renames the window and the palette and leaves the field typed
  // into as it is; anything else draws both anew.
  function typed() {
    onChange();
    frame.setTitle(title());
    if (!host.contains(document.activeElement)) frame.refresh();
    if (!around.palette?.contains(document.activeElement)) renderPalette();
  }
  function changed() { onChange(); frame.refresh(); renderPalette(); }
  function renderPage(key) {
    cleanups.forEach(destroy => destroy());
    cleanups = [];
    if (kind.pages?.[key]) return kind.pages[key]();
    if (common.pages[key]) return common.pages[key]();
    // «Функциональные опции»: the options of the project, a box at each,
    // marked where the object is in its content (podsistema.md, «Окно»;
    // snapshot podsistema/05), written into the options as the
    // subsystems are.
    if (key === 'functional-options') return optionsPage(source.functionalOptions, options, onChange, source.memberWord);
    if (key === 'subsystems') return subsystemsPage(source, marks, onChange);
    return rowsOf(key);
  }
  // A page built of the palette's rows (kinds.WindowRows), each the field of
  // the subject of the palette.
  function rowsOf(key) {
    if (!source.windowRows?.[key]) return null;
    const own = context(() => cleanups);
    return rowsPage(source.windowRows[key], source.palette, paletteFields(own), kind.subject?.(source) || ownSubject(source, object()),
      role => around.openModule?.(role), own);
  }
  // The window of no pages: its title over the body draw fills.
  function pagelessWindow() {
    host.hidden = false;
    const element = node('div', undefined, 'object-window ' + kind.className), heading = node('div', title(), 'object-window-title');
    const body = node('div', undefined, kind.className + '-body');
    element.append(heading, body);
    host.replaceChildren(element);
    const refresh = () => { cleanups.forEach(destroy => destroy()); cleanups = []; body.replaceChildren(); kind.draw(body); };
    refresh();
    return {refresh, setTitle(text) { heading.textContent = text; }};
  }
  function renderPalette() {
    if (!source) return;
    const part = common.part() || kind.part?.();
    showObjectPalette(around, paletteCleanups, part ? part.title : object().name, part ? part.tabs : source.palette,
      part ? part.subject : kind.subject?.(source) || ownSubject(source, object()), context());
  }
  StudioLanguage.onChange(() => { if (source) { frame.refresh(); renderPalette(); } });
  return hostedEditor(host, () => { cleanups.forEach(destroy => destroy()); cleanups = []; if (source) clearObjectPalette(around, paletteCleanups); source = null; chosen = null; kind.reset?.(); }, {
    // Opened anew after «Сохранить» (keep), the window stays on its page with
    // the same part chosen.
    open(value, keep = false) {
      const page = keep && frame?.current ? frame.current() : 'main';
      if (source) clearObjectPalette(around, paletteCleanups);
      source = structuredClone(value);
      if (!keep) { chosen = null; kind.reset?.(); }
      kind.open?.(source);
      marks = new Set((source.subsystems || []).filter(place => place.member).map(place => place.id));
      options = new Set((source.functionalOptions || []).filter(place => place.member).map(place => place.id));
      common.open();
      frame = kind.draw ? pagelessWindow() : showObjectWindow(host, around, {title: title(), pages: source.pages, chosen: page, page: renderPage});
      renderPalette();
    },
    // The object and the subsystems marked, in one order so a mark taken
    // back leaves the window unchanged; a window of no pages marks none.
    value() {
      if (!source) return null;
      if (kind.value) return {...kind.value(structuredClone(object())), ...common.value()};
      // A kind of the page «Функциональные опции» writes the options marked;
      // its own subsystems are a property of its own, not marks of a page.
      if (source.pages?.some(page => page.key === 'functional-options')) return {...structuredClone(object()), functionalOptions: [...options].sort(), ...common.value()};
      return {...structuredClone(object()), subsystems: [...marks].sort(), ...common.value()};
    },
    // Shows the palette of the object itself, as a click on its node does.
    showPalette() {
      if (!source || !around.palette) return false;
      chosen = null;
      kind.reset?.();
      frame.refresh();
      renderPalette();
      return true;
    },
    // What the kind adds to the window: choosing a part a node of the tree
    // names, «Добавить» and «Удалить» of the tree inside the object.
    ...kind.methods,
  });
}

/* commonObjectPages are the pages the windows of objects of many kinds
   share, as the prototype draws them in each (spravochnik.md, pages 12-14;
   plan-obmena.md, dokument.md): «Обмен данными», «Ввод на основании» and
   «Права». around is {source() - what the window read (windowHead), object()
   - the object edited, setBasedOn(list) - writes its list «Вводится на
   основании», changed() - an edit drawing the window and the palette anew,
   marked() - an edit of a mark alone, refresh() - draws the window anew,
   host - where the window stands}. open() takes what a source read anew
   marks; value() is what the pages write with the object, by the pages the
   window has. */
function commonObjectPages(around) {
  const source = () => around.source(), has = key => (source().pages || []).some(page => page.key === key);
  // «Обмен данными»: the exchange plans of the project, a box and the icon
  // of a plan at each, marked where the plan registers the changes of the
  // object, with its autoregistration - «Разрешить» or «Запретить» - beside
  // a marked one; «Отметить все элементы» and «Снять отметку со всех
  // элементов» over them (spravochnik.md, page 14; snapshot okno-14). A plan
  // marked anew registers automatically, as a new object of the plan's own
  // window «Состав» does (plan-obmena.md). Written into the plans as the
  // subsystems are.
  let exchange = new Map(), currentPlan = null;
  // The autoregistration in the prototype's order, each with its picture
  // (snapshot configurator-2026-10-26/series-2.578/03; «Запретить» red - the
  // owner's decision of 26.10.2026).
  const autoRecords = [['deny', 'Запретить', 'auto-record-deny'], ['allow', 'Разрешить', 'auto-record-allow']];
  function exchangePage() {
    const page = node('div', undefined, 'object-window-fields'), places = source().exchangePlans || [];
    const mark = (id, marked) => { if (!marked) exchange.delete(id); else if (!exchange.has(id)) exchange.set(id, 'allow'); };
    const changed = () => { around.marked(); around.refresh(); };
    const head = node('div', undefined, 'object-window-list-head');
    head.append(...markAllButtons(marked => { for (const place of places) mark(place.id, marked); changed(); }));
    const table = node('table', undefined, 'object-window-table');
    const caption = node('tr');
    caption.append(node('th', 'План обмена'), node('th', 'Авторегистрация'));
    table.append(caption);
    for (const place of places) {
      const row = node('tr', undefined, place.id === currentPlan ? 'current' : ''), plan = node('td'), auto = node('td');
      const box = node('input');
      box.type = 'checkbox'; box.checked = exchange.has(place.id); box.className = 'object-choice-box';
      box.setAttribute('aria-label', place.name);
      box.addEventListener('change', () => { mark(place.id, box.checked); currentPlan = place.id; changed(); });
      const name = node('div', undefined, 'object-window-table-name');
      name.append(box, iconElement('exchange-plan'), node('span', place.name));
      plan.append(name);
      if (exchange.has(place.id)) {
        auto.append(listCell(autoRecords, exchange.get(place.id), value => { exchange.set(place.id, value); changed(); },
          'Авторегистрация: ' + place.name));
      }
      // A click makes the row current in place: a double click on its cell
      // opens the list of the cell, which a redraw would take away.
      row.addEventListener('click', event => {
        if (event.target.closest('input')) return;
        currentPlan = place.id;
        for (const other of table.querySelectorAll('tr')) other.classList.toggle('current', other === row);
      });
      row.append(plan, auto);
      table.append(row);
    }
    const frameOfTable = node('div', undefined, 'object-window-checks object-window-table-frame');
    frameOfTable.append(table);
    page.append(head, frameOfTable);
    return page;
  }
  // «Ввод на основании»: «Вводится на основании» - the object's own list of
  // bases - and «Является основанием для» - the objects whose lists name
  // it, written into them as the subsystems are - each with «Изменить»,
  // the window «Выбор объекта» of the eight kinds, and «Удалить»; below,
  // «Конструктор ввода на основании», unavailable (spravochnik.md, page 12;
  // snapshot okno-12). The object in one list is in the other too: it is
  // the basis of itself.
  let basisFor = [], currentBasis = {own: null, other: null};
  function basisName(id) {
    return objectChoiceItem(source().choices?.basedOn, id)?.fullName || (source().basisFor || []).find(place => place.id === id)?.name || id;
  }
  // The objects a list «Является основанием для» names stand in the order of
  // the tree, those it does not offer - the tables of external sources -
  // after them, as the server reads them.
  function inTreeOrder(ids) {
    const order = choiceOrder(source().choices?.basedOn || []), rank = id => order.includes(id) ? order.indexOf(id) : order.length;
    return [...ids].sort((a, b) => rank(a) - rank(b));
  }
  function setBasedOn(own, other) {
    around.setBasedOn(own);
    basisFor = inTreeOrder(other);
    around.changed();
  }
  // The object itself follows from one list into the other.
  function withSelf(list, holds) {
    const self = around.object().id, rest = list.filter(id => id !== self);
    return holds ? [...rest, self] : rest;
  }
  function basedOnPage() {
    const page = node('div', undefined, 'object-window-fields object-window-based-on'), own = around.object().basedOn || [], self = around.object().id;
    const block = (key, caption, list, set) => {
      if (!list.includes(currentBasis[key])) currentBasis[key] = list[0] ?? null;
      return objectList(caption, list.map(id => [id, basisName(id)]), currentBasis[key], id => { currentBasis[key] = id; around.refresh(); },
        async () => {
          const chosen = await chooseObjects('Выбор объекта', source().choices?.basedOn || [], list);
          // An object the window does not offer - a table of an external
          // source - stays where it is.
          if (chosen) set([...list.filter(id => !objectChoiceItem(source().choices?.basedOn, id)), ...chosen]);
        },
        id => set(list.filter(item => item !== id)));
    };
    const designer = node('button', 'Конструктор ввода на основании', 'object-window-designer');
    designer.type = 'button'; designer.disabled = true;
    page.append(...block('own', 'Вводится на основании:', own, list => setBasedOn(list, withSelf(basisFor, list.includes(self)))),
      ...block('other', 'Является основанием для:', basisFor, list => setBasedOn(withSelf(own, list.includes(self)), list)), designer);
    return page;
  }
  // «Права»: the roles of the project; the rights the role chosen gives on
  // the object, a box at each, marking a right marking the rights it needs
  // and clearing one clearing those needing it; its restrictions of access
  // to data on the right current (spravochnik.md, page 13; snapshot okno-13;
  // rol.md, «Права зависят друг от друга»). Each role is edited by the model
  // of the window of a role over what it says about the object alone, and
  // written with the object.
  let roleModels = new Map(), currentRole = null;
  const roleRight = {right: null, row: -1};
  const rightCaption = key => source().rightsSchema?.rights?.find(item => item.key === key)?.title || key;
  function rightsPage() {
    const page = node('div', undefined, 'object-window-fields object-window-rights'), roles = source().roles || [];
    const object = source().rightsSchema?.objects?.[0], model = roleModels.get(currentRole);
    page.append(node('div', 'Роли:', 'object-window-caption'),
      objectRows(roles.map(entry => [entry.role.id, entry.role.name]), currentRole, id => { currentRole = id; around.refresh(); }));
    const rights = model && object ? object.rights : [], members = object ? [{object}] : [];
    if (!rights.includes(roleRight.right)) { roleRight.right = rights[0] ?? null; roleRight.row = -1; }
    const redraw = () => { around.refresh(); around.host.querySelector('.object-window-rights .check-list')?.focus(); };
    const list = checkList(rights.map(right => ({key: right, caption: rightCaption(right), checked: model.groupMark(members, right) === 'all'})), roleRight.right,
      (right, enabled) => { model.setGroup(members, right, enabled); around.marked(); redraw(); },
      right => { roleRight.right = right; roleRight.row = -1; redraw(); });
    page.append(node('div', 'Права:', 'object-window-caption'), list);
    if (model) page.append(roleRestrictionSection({model, object, caption: rightCaption, title: item => localizedTitle(item, source().defaultLanguage),
      state: roleRight, changed: () => { around.marked(); around.refresh(); }}));
    return page;
  }
  // «Формы», «Команды», «Макеты»: a tree of the parts with its panel - the
  // forms of the object below the frame of the main forms, the commands
  // above the flag of the standard commands, the templates above the menu
  // «Конструкторы» (spravochnik.md, okno-08, okno-10, okno-11). A form opens
  // by «Изменить», a command - its module, as its own action is; commands
  // and templates move round the ends. Adding a template opens the
  // constructor of a template; adding forms and commands and deleting are
  // the items of their constructors, and a template opens in its editor -
  // each unavailable until its item. The part chosen is the window's
  // (around.chosen), its palette beside it.
  function partsTree(icon, caption, kind, items, open, move, add) {
    const chosen = around.chosen(), inside = chosen?.kind === kind ? chosen : null, on = !!inside;
    const bar = windowPanel([
      panelButton('add', 'Добавить', !!add, () => add()),
      panelButton('change', 'Изменить', on && !!open, () => open(inside.id)),
      panelButton('delete', 'Удалить', false, () => {}),
      panelButton('move-up', 'Переместить вверх', on && !!move, () => { move(inside.id, -1); around.changed(); }),
      panelButton('move-down', 'Переместить вниз', on && !!move, () => { move(inside.id, 1); around.changed(); }),
      panelButton('sort', 'Сортировать', false, () => {}),
    ]);
    const tree = node('div', undefined, 'object-window-tree object-window-tree-tall');
    tree.append(around.treeRow(icon, caption, {kind: `${kind}s`}, items.map(name => {
      const element = around.treeRow(icon, name, {kind, id: name});
      if (open) element.addEventListener('dblclick', () => open(name));
      return element;
    })));
    return [bar, tree];
  }
  function moveNamed(list) {
    return (name, by) => { const at = list.findIndex(item => item.name === name); if (at >= 0) moveInCircle(list, at, by); };
  }
  function formsPage() {
    const page = around.rowsPage('forms') || node('div', undefined, 'object-window-fields');
    page.append(...partsTree('form', 'Формы', 'form', (source().forms || []).map(form => form.name),
      name => around.openFile(source().forms.find(form => form.name === name).path)));
    return page;
  }
  function commandsPage() {
    const page = node('div', undefined, 'object-window-fields'), rows = around.rowsPage('commands');
    const commands = around.object().commands || [];
    page.append(...partsTree('command', 'Команды', 'command', commands.map(command => command.name),
      name => source().commandModules?.[name] && around.openFile(source().commandModules[name]), moveNamed(commands)), ...rows?.childNodes || []);
    return page;
  }
  // A template the constructor made (constructTemplate) stands last; the
  // content loaded from a file is written by «Сохранить» with it, by its
  // identifier.
  let contents = new Map();
  // «Добавить» of a template - on the page or on the branch «Макеты» of the
  // tree - opens the constructor of a template (maket.md); the name is
  // checked among the templates of the object before it is added, and the
  // template made is chosen.
  async function addTemplate() {
    const templates = () => around.object().templates || [];
    const check = draft => ({path: source().path, description: {...structuredClone(around.object()), templates: [...templates(),
      {id: crypto.randomUUID(), name: draft.name, kind: draft.kind}]}, at: ['templates', String(templates().length)]});
    const draft = await constructTemplate(source(), check, async draft => {
      const answer = await postJSON('/api/name', check(draft));
      if (answer.refusal) throw new Error(answer.refusal);
      return draft;
    });
    if (!draft || !source()) return;
    const template = {id: crypto.randomUUID(), name: draft.name, kind: draft.kind};
    if (Object.keys(draft.title || {}).length) template.title = draft.title;
    if (draft.comment) template.comment = draft.comment;
    (around.object().templates ||= []).push(template);
    if (draft.content) contents.set(template.id, draft.content);
    around.setChosen({kind: 'template', id: template.name});
    around.show('templates');
    around.changed();
  }
  function templatesPage() {
    const page = node('div', undefined, 'object-window-fields');
    const templates = around.object().templates || [];
    const designers = menuButton('Конструкторы', () => []);
    designers.disabled = true;
    designers.classList.add('object-window-button');
    page.append(...partsTree('template', 'Макеты', 'template', templates.map(template => template.name), null, moveNamed(templates), addTemplate), designers);
    return page;
  }
  // The palette of the template chosen: the rows of a template of an
  // object (kinds.PartPalette("template")).
  function part() {
    const chosen = around.chosen();
    if (chosen?.kind !== 'template' || !has('templates')) return null;
    const templates = around.object().templates || [], at = templates.findIndex(item => item.name === chosen.id), template = templates[at];
    if (!template) { around.setChosen(null); return null; }
    return {title: template.name, tabs: source().partPalettes?.template, subject: {target: template, owner: source().path.split('/')[1], get: path => pathValue(template, path),
      set: (path, value) => { if (value) setPathValue(template, path, value); else delete template[path]; },
      setName: value => { template.name = value; chosen.id = value; },
      check: () => ({path: source().path, description: structuredClone(around.object()), at: ['templates', String(at)]})}};
  }
  const pages = {exchange: exchangePage, 'based-on': basedOnPage, rights: rightsPage, forms: formsPage, commands: commandsPage, templates: templatesPage};
  return {
    pages,
    part,
    addTemplate,
    open() {
      contents = new Map();
      exchange = new Map((source().exchangePlans || []).filter(place => place.member).map(place => [place.id, place.autoRecord || '']));
      currentPlan = source().exchangePlans?.[0]?.id ?? null;
      basisFor = inTreeOrder(withSelf((source().basisFor || []).map(place => place.id), (around.object().basedOn || []).includes(around.object().id)));
      currentBasis = {own: null, other: null};
      roleModels = new Map((source().roles || []).map(entry => [entry.role.id, createRoleModel({role: entry.role, schema: source().rightsSchema})]));
      if (!roleModels.has(currentRole)) currentRole = source().roles?.[0]?.role.id ?? null;
    },
    // What the pages the window has write, in one order so a mark taken
    // back leaves the window unchanged.
    value() {
      const value = {};
      if (has('based-on')) value.basisFor = [...basisFor].sort();
      if (has('rights')) value.roles = (source().roles || []).map(entry => ({id: entry.role.id, revision: entry.revision,
        permission: roleModels.get(entry.role.id).value().objects.find(item => item.object === around.object().id) || null}));
      const loaded = [...contents].filter(([id]) => around.object().templates?.some(item => item.id === id)).map(([id, content]) => ({id, content}));
      if (loaded.length) value.templateContents = loaded;
      if (has('exchange')) value.exchangePlans = [...exchange].sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0).map(([id, autoRecord]) => ({id, autoRecord}));
      return value;
    },
  };
}

/* partsOfTwoLevels is a page of the window of an object (objectWindowEditor)
   holding parts of two levels - the operations of a web service and their
   parameters, the templates of an HTTP service and their methods: a panel
   adding a part of each level - the lower one under the part chosen - and
   the buttons parts share (partButtons), and the tree of the parts; the
   part chosen shows in the palette (source.partPalettes by its kind), its
   name checked by the model at its place in the object. spec is {list - the
   key of the upper parts in the object, children - of the lower ones in an
   upper part, upper and lower - {kind, icon, add: [icon, title], base, fields()}
   of a part of the level as the prototype makes it}. A new part stands last,
   named the next number of its base among its siblings, and is chosen. */
function partsOfTwoLevels(api, page, spec) {
  let chosen = null;
  const object = () => api.object();
  const uppers = () => object()[spec.list] ||= [];
  function chosenUpper() {
    const at = chosen ? uppers().findIndex(item => item.id === (chosen.upper ?? chosen.id)) : -1;
    return {upper: uppers()[at], at};
  }
  function chosenPart() {
    const {upper} = chosenUpper();
    return chosen?.kind === spec.lower.kind ? upper?.[spec.children]?.find(item => item.id === chosen.id) : upper;
  }
  function part() {
    const {upper, at} = chosenUpper(), chosenOne = chosenPart();
    if (!chosenOne) return null;
    const lowers = upper[spec.children] || [];
    const where = chosen.kind === spec.lower.kind ? [spec.list, String(at), spec.children, String(lowers.findIndex(item => item.id === chosen.id))] : [spec.list, String(at)];
    const source = api.source();
    return {title: chosenOne.name, tabs: source.partPalettes?.[chosen.kind],
      subject: {...ownSubject(source, chosenOne), check: () => ({path: source.path, description: object(), at: where})}};
  }
  function add(list, level, choice) {
    const added = {id: crypto.randomUUID(), name: numberedName(level.base, new Set(list.map(item => item.name.toLocaleLowerCase()))), title: {}, ...level.fields()};
    list.push(added);
    chosen = {...choice, kind: level.kind, id: added.id};
    api.changed();
  }
  function siblings() {
    return chosen?.kind === spec.lower.kind ? chosenUpper().upper?.[spec.children] || [] : uppers();
  }
  function render() {
    const element = node('div', undefined, 'object-window-fields'), on = !!chosenPart();
    const bar = windowPanel([
      panelButton(spec.upper.add[0], spec.upper.add[1], true, () => add(uppers(), spec.upper, {})),
      panelButton(spec.lower.add[0], spec.lower.add[1], on, () => {
        const {upper} = chosenUpper();
        if (upper) add(upper[spec.children] ||= [], spec.lower, {upper: upper.id});
      }),
      ...partButtons(on, {change: () => document.querySelector('#properties input')?.focus(),
        remove: () => { const list = siblings(), at = list.findIndex(item => item.id === chosen.id); if (at >= 0) list.splice(at, 1); chosen = null; api.changed(); },
        move: by => { const list = siblings(), at = list.findIndex(item => item.id === chosen.id); if (at >= 0) moveInCircle(list, at, by); api.changed(); }}),
    ]);
    const row = (icon, caption, choice, children = []) => {
      const {element: branch, row: line} = treeNode({icon, caption, children, open: true});
      line.classList.toggle('selected', !!chosen && chosen.kind === choice.kind && chosen.id === choice.id);
      line.addEventListener('click', () => { chosen = choice; api.refresh(); api.palette(); });
      return branch;
    };
    const tree = node('div', undefined, 'object-window-tree object-window-tree-tall');
    tree.append(...(object()[spec.list] || []).map(upper => row(spec.upper.icon, upper.name, {kind: spec.upper.kind, id: upper.id},
      (upper[spec.children] || []).map(lower => row(spec.lower.icon, lower.name, {kind: spec.lower.kind, id: lower.id, upper: upper.id})))));
    element.append(bar, tree);
    return element;
  }
  return {pages: {[page]: render}, part, reset() { chosen = null; }};
}

/* clearObjectPalette empties the palette of around (its element and its
   title) and runs the cleanups of its fields, emptying the list in place. */
function clearObjectPalette(around, cleanups) {
  cleanups.forEach(destroy => destroy());
  cleanups.length = 0;
  around.palette?.replaceChildren();
  around.paletteTitle?.('Свойства');
}

/* showObjectPalette draws in around.palette the palette of what title
   names: the caption «Свойства: <title>», the kind's tabs with the fields
   of paletteFields(context) for subject; the fields' cleanups are collected
   in cleanups, run first. */
function showObjectPalette(around, cleanups, title, tabs, subject, context) {
  if (!around.palette) return;
  clearObjectPalette(around, cleanups);
  around.paletteTitle?.(`Свойства: ${title}`);
  const fields = paletteFields({...context, cleanups: () => cleanups});
  around.palette.append(objectPalette(tabs || [], row => fields(subject, row), paletteShown(subject)));
}

/* paletteEditor is the editor of an object of a kind without a window: its
   double click, «Изменить» and «Свойства» open its palette, and the work
   area stays as it was (obshchee.md, «Двойной щелчок и „Изменить"»). target
   (source) is the object in what the server read, subject(source) what the
   palette edits, context(source) what its fields open and offer beyond the
   project, typing and redrawing. Typing renames the palette and leaves its
   fields as they are; anything else draws it anew. Without subject the
   palette edits the object's own properties by their keys - an empty value
   is not written - and opens none of its windows. */
function paletteEditor(host, onChange, around, {target, subject = source => ownSubject(source, target(source)), context = () => ({windows: []})}) {
  let source = null;
  const cleanups = [];
  function typed() { onChange(); around.paletteTitle?.(`Свойства: ${target(source).name}`); }
  function changed() { onChange(); render(); }
  function render() {
    if (!source) return;
    showObjectPalette(around, cleanups, target(source).name, source.palette, subject(source), {project: source, typed, changed, ...context(source, {changed})});
  }
  StudioLanguage.onChange(() => render());
  return hostedEditor(host, () => { if (source) clearObjectPalette(around, cleanups); source = null; }, {
    open(value) {
      if (source) clearObjectPalette(around, cleanups);
      source = structuredClone(value);
      render();
    },
    value() { return source && structuredClone(target(source)); },
    // The palette is all the object shows: a click on its node shows it.
    showPalette() {
      if (!source || !around.palette) return false;
      render();
      return true;
    },
    // A window the palette opens by a row - from the menu of the object
    // too («Состав» of a common attribute).
    openWindow(key) { if (source) context(source, {changed}).openWindow?.(key); },
  });
}

/* ownSubject is what a palette edits of an object whose rows are its own
   properties by their keys: an empty value is not written, the name is
   checked in its branch (watchName). */
function ownSubject(source, object) {
  return {target: object, get: key => object[key], set: (key, value) => { if (value) object[key] = value; else delete object[key]; },
    setName: value => { object.name = value; }, check: () => ({path: source.path, name: object.name})};
}

/* chooseFileContent asks the browser for a file, of the extensions in
   accept when given, and answers its name and its content in base64 - as
   the server takes a file in JSON - or null when none is chosen. */
function chooseFileContent(accept = '') {
  return new Promise(resolve => {
    const chooser = node('input');
    chooser.type = 'file';
    chooser.accept = accept;
    chooser.addEventListener('change', async () => {
      const file = chooser.files?.[0];
      if (!file) { resolve(null); return; }
      const bytes = new Uint8Array(await file.arrayBuffer());
      let text = '';
      for (let at = 0; at < bytes.length; at += 0x8000) text += String.fromCharCode(...bytes.subarray(at, at + 0x8000));
      resolve({name: file.name, content: btoa(text)});
    });
    chooser.click();
  });
}

/* objectWindow is the window of an object, one for every kind that has one,
   as the prototype's (docs/block-00/obshchee.md, «Окно объекта»): the title
   «<Вид> <Имя>»; the list of pages at the left, the page chosen beside it;
   at the bottom «Действия ▾», «<Назад», «Далее>», «Закрыть», «Справка» -
   «<Назад» unavailable on the first page, «Далее>» on the last. pages are
   the kind's ({key, title}, kinds.WindowPages), chosen the key shown first;
   page(key) builds a page anew, empty where nothing of it is built yet;
   actions() answers the items of «Действия» above the window's own three,
   «Изменить», «Скопировать» and «Удалить», which the window shows
   unavailable (spravochnik.md, «Меню «Действия» окна»); close() is
   «Закрыть», help the page of studioHelp «Справка» opens. Answers
   {element, show(key), refresh(), current(), setTitle(text)}: refresh()
   builds the page shown anew. */
function objectWindow({title, pages, chosen, page, actions, close, help}) {
  const element = node('div', undefined, 'object-window'), heading = node('div', title, 'object-window-title');
  const list = node('div', undefined, 'object-window-pages'), body = node('div', undefined, 'object-window-body');
  list.setAttribute('role', 'listbox');
  let current = pages.some(item => item.key === chosen) ? chosen : pages[0]?.key;
  const tabs = pages.map(item => {
    const tab = node('button', item.title, 'object-window-page');
    tab.type = 'button'; tab.dataset.page = item.key;
    tab.setAttribute('role', 'option');
    tab.addEventListener('click', () => show(item.key));
    return tab;
  });
  list.append(...tabs);
  const own = [['Изменить', () => {}, false, 'F2', 'change'], ['Скопировать', () => {}, false, 'F9', 'clone'], ['Удалить', () => {}, false, 'Del', 'delete']];
  const menu = menuButton('Действия', () => { const items = actions?.() || []; return items.length ? [...items, null, ...own] : own; });
  const back = node('button', '<Назад'), next = node('button', 'Далее>'), shut = node('button', 'Закрыть'), aid = node('button', 'Справка');
  for (const button of [back, next, shut, aid]) button.type = 'button';
  const step = by => { const at = pages.findIndex(item => item.key === current) + by; if (pages[at]) show(pages[at].key); };
  back.addEventListener('click', () => step(-1));
  next.addEventListener('click', () => step(1));
  shut.addEventListener('click', () => close?.());
  aid.addEventListener('click', () => showHelp(help));
  const bar = node('div', undefined, 'object-window-bar');
  bar.append(menu, back, next, shut, aid);
  const middle = node('div', undefined, 'object-window-middle');
  middle.append(list, body);
  element.append(heading, middle, bar);
  function refresh() {
    const at = pages.findIndex(item => item.key === current);
    for (const tab of tabs) tab.setAttribute('aria-selected', String(tab.dataset.page === current));
    back.disabled = at <= 0;
    next.disabled = at < 0 || at === pages.length - 1;
    body.replaceChildren(...[page(current)].filter(Boolean));
    body.dataset.page = current;
  }
  function show(key) { current = key; refresh(); }
  refresh();
  return {element, show, refresh, current: () => current, setTitle(text) { heading.textContent = text; }};
}

/* panelButton is a button of the panel over a tree of a window, its icon
   and its title; windowPanel is the panel. */
function panelButton(icon, title, enabled, act) {
  const button = iconButton(icon, title);
  button.disabled = !enabled;
  button.addEventListener('click', act);
  return button;
}
function windowPanel(buttons) {
  const bar = node('div', undefined, 'object-window-panel');
  bar.append(...buttons);
  return bar;
}

/* transferButtons is the prototype's column of «>» and «<» between two
   lists (the work area of the home page, the main section; snapshots
   series-2.595/41, 50): onward and back are [title, act, enabled]. Answers
   the column, of className. */
function transferButtons(className, onward, back) {
  const column = node('div', undefined, className);
  for (const [text, [title, act, enabled]] of [['>', onward], ['<', back]]) {
    const button = node('button', text);
    button.type = 'button';
    button.title = title;
    button.disabled = !enabled;
    button.addEventListener('click', act);
    column.append(button);
  }
  return column;
}

/* partButtons are the buttons a panel of a tree of parts shares, acting on
   the part chosen (on): «Изменить» (change), «Удалить» after the question
   of every deletion (remove), «Переместить вверх» and «вниз» (move(by)) -
   unavailable where there is no move yet - and «Сортировать», unavailable
   until the window «Сортировка метаданных» (the item of the map «порядок
   объектов»), where the panel has it (sort). titles name the buttons where
   the panel's prototype words them otherwise. */
function partButtons(on, {change, remove, move, sort = true, titles = {}}) {
  const title = key => titles[key] || {change: 'Изменить', remove: 'Удалить', up: 'Переместить вверх', down: 'Переместить вниз'}[key];
  return [
    panelButton('change', title('change'), on, change),
    panelButton('delete', title('remove'), on, async () => { if (await askDeleteObject()) remove(); }),
    panelButton('move-up', title('up'), on && !!move, () => move(-1)),
    panelButton('move-down', title('down'), on && !!move, () => move(1)),
    ...(sort ? [panelButton('sort', 'Сортировать', false, () => {})] : []),
  ];
}

/* showObjectWindow puts the window of an object (objectWindow) into host
   and shows it: «Действия» offers around.actions(), «Закрыть» is
   around.close(), «Справка» the page of the window of an object. Answers the
   window. */
function showObjectWindow(host, around, {title, pages, chosen, page}) {
  host.hidden = false;
  const frame = objectWindow({title, pages: pages || [], chosen, page,
    actions: () => around.actions?.() || [], close: () => around.close?.(), help: 'object-window'});
  host.replaceChildren(frame.element);
  return frame;
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
   button that chooses the row. A row with children opens and closes by its
   sign, by the keyboard and, where foldOnDoubleClick, by a double click; a
   single click on the row only chooses it (configurator 22.10.2026,
   STUDIO-SHELL.md: «Одиночный щелчок по узлу только выделяет его»). open is
   how it stands first, toggled(open) is told each time it is opened or
   closed by hand - not of the row drawn open, whose details the browser
   reports opened too, after the fact: a search that opens what holds a
   match would leave it all open once cleared (2.437, case 2в-25). mark is
   a box standing before the icon, as the prototype's window «Выбор
   объекта» draws it. Answers {element, row}: the element to put in the tree and the
   row to hang clicks and the menu on. Every tree of Studio is made of these;
   the trees of marks - the filter by subsystems, the fields of a
   restriction - are rows of restrictionTreeRow. */
function treeNode({icon, caption, children = [], open = false, toggled, foldOnDoubleClick = true, mark}) {
  const row = node(children.length ? 'summary' : 'div', undefined, 'studio-tree-row'), sign = node('span', undefined, 'studio-tree-sign');
  row.append(sign, ...(mark ? [mark] : []), iconElement(icon), typeof caption === 'string' ? node('span', caption) : caption);
  if (!children.length) {
    row.classList.add('studio-tree-leaf');
    return {element: row, row};
  }
  const element = node('details', undefined, 'studio-tree-node');
  element.open = open;
  // A click of the mouse (detail above 0) anywhere but on the sign does not
  // fold; the keyboard's click on the row does.
  row.addEventListener('click', event => { if (event.detail > 0 && !sign.contains(event.target)) event.preventDefault(); });
  // The double click comes to the row's details through the rows inside it
  // too: only its own row folds it, and a row whose own double click does
  // something else stops it before it comes here.
  if (foldOnDoubleClick) element.addEventListener('dblclick', event => {
    if (event.target.closest('.studio-tree-row') !== row || sign.contains(event.target)) return;
    element.open = !element.open;
  });
  let state = open;
  if (toggled) element.addEventListener('toggle', () => { if (element.open !== state) toggled(state = element.open); });
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
/* markAllButtons are the platform's pair of buttons over a list of boxes,
   drawn by their icons: «Отметить все элементы» and «Снять отметку со всех
   элементов», named so in Настройка списка and in the window of a role
   alike (configurator-2026-10-20/prava/33, 34, 52, 53; block-00/rol/03). mark(true) marks every box, mark(false)
   takes every mark away. Answers the two buttons. */
function markAllButtons(mark) {
  return [['check-all', 'Отметить все элементы', true], ['uncheck-all', 'Снять отметку со всех элементов', false]].map(([icon, text, enabled]) => {
    const button = iconButton(icon, text);
    button.addEventListener('click', () => mark(enabled));
    return button;
  });
}

function iconButton(name, title) {
  const button = document.createElement('button');
  button.type = 'button';
  button.className = 'icon-only';
  button.title = title;
  button.setAttribute('aria-label', title);
  button.append(iconElement(name));
  return button;
}

/* askCancelInput is the question of a value beyond the model's ceiling, the
   prototype's «В поле введены некорректные данные. Отменить ввод?», asked
   wherever such a value is entered (ML-STUDIO.md, «Поведение платформы»):
   true is «Да» - the value before comes back, false is «Нет» - the field
   stays to be put right. */
function askCancelInput() {
  return showDialog({title: 'В поле введены некорректные данные. Отменить ввод?', caption: questionCaption,
    explanation: 'Нажмите «Да» для отмены ввода или «Нет» для продолжения редактирования.',
    answers: [['yes', 'Да'], ['no', 'Нет']]}).then(answer => answer !== 'no');
}

/* chooseXDTOPackages is the window «Выбор пакетов XDTO» of a web service
   (docs/block-00/web-servis.md, «Окно», «Пространства платформы»; snapshots
   series-2.584/131-136): a box at each namespace of the platform
   (platform), then, indented, at each package of the project in the order
   of the tree by its namespace (packages, [{id, namespace}]); a marked one
   keeps its place. items are the packages chosen before (metadata.
   WebServicePackage); answers them on OK - those kept in their order, the
   ones marked anew after them in the order of the window - or null on
   «Отмена». */
async function chooseXDTOPackages(items, platform, packages) {
  const key = item => item.package ? 'package:' + item.package : 'namespace:' + item.namespace;
  const rows = [...platform.map(namespace => ({item: {namespace}, caption: namespaceShown(namespace)})),
    ...packages.map(own => ({item: {package: own.id}, caption: own.namespace, own: true}))];
  const marked = new Set(items.map(key));
  const content = node('div', undefined, 'xdto-packages-choice');
  for (const row of rows) {
    const line = node('label', undefined, 'xdto-packages-row' + (row.own ? ' xdto-packages-own' : '')), box = node('input');
    box.type = 'checkbox';
    box.checked = marked.has(key(row.item));
    box.addEventListener('change', () => { if (box.checked) marked.add(key(row.item)); else marked.delete(key(row.item)); });
    line.append(box, node('span', row.caption));
    content.append(line);
  }
  const answer = await showDialog({title: 'Выбор пакетов XDTO', content, className: 'xdto-packages-dialog', answers: [['ok', 'OK'], [null, 'Отмена']]});
  if (answer !== 'ok') return null;
  const kept = items.filter(item => marked.has(key(item)));
  const known = new Set(kept.map(key));
  return [...kept, ...rows.map(row => row.item).filter(item => marked.has(key(item)) && !known.has(key(item)))];
}

/* xmlTypeText is how a field shows a type named by the schema: «string
   (http://www.w3.org/2001/XMLSchema)», «UUID (data/core)». */
function xmlTypeText(type) {
  return `${type.name}${type.namespace ? ` (${namespaceShown(type.namespace)})` : ''}`;
}

/* readOnlyPalette draws the rows of a palette (kinds.PaletteTab) that shows
   and edits nothing - the palette of a node of the window viewing a
   description of a WS reference (ws-ssylka.md, «Окно»): each row its
   caption and its value in words, a value a node may leave not said
   (row.optional) marked ✓ where the description says it and shown as its
   default greyed where not. get(key) answers the value of a row. */
function readOnlyPalette(tabs, get) {
  const list = node('div', undefined, 'read-only-palette');
  for (const tab of tabs || []) for (const row of tab.rows) {
    const value = get(row.key), said = value !== undefined && value !== null && value !== '';
    let text = '';
    if (said) {
      if (row.field === 'switch') text = (row.choices || []).find(([key]) => key === String(!!value))?.[1] || '';
      else if (row.field === 'choice') text = (row.choices || []).find(([key]) => key === value)?.[1] || value;
      else if (row.field === 'xml-type') text = value.name ? xmlTypeText(value) : '';
      else if (row.field === 'namespace') text = namespaceShown(value);
      else if (row.field === 'xml-types') text = value.map(xmlTypeText).join(', ');
      else text = String(value);
    } else if (row.default !== undefined) text = (row.choices || []).find(([key]) => key === row.default)?.[1] ?? row.default;
    // A flag a node always has, left out, is false.
    else if (row.field === 'switch' && !row.optional) text = (row.choices || []).find(([key]) => key === 'false')?.[1] || '';
    const line = node('div', undefined, 'read-only-palette-row');
    const shown = node('span', text, 'read-only-palette-value' + (!said && row.optional ? ' palette-unset' : ''));
    if (said && row.optional) shown.prepend(iconElement('apply'));
    line.append(node('span', row.title, 'read-only-palette-caption'), shown);
    line.title = row.description || '';
    list.append(line);
  }
  return list;
}

/* editValueList is the platform's window «Редактирование списка
   значений» (paket-xdto.md, «Проверить пакет, экспорт схемы, окно типа»;
   snapshots series-2.603/19-21): «Список значений:» with «Добавить»,
   «Скопировать», «Изменить», «Удалить» and «Сохранить в файл» over the
   rows; OK, «Отмена», «Справка». «Добавить» puts an empty row after the
   current and makes it current, «Скопировать» a copy of the current;
   «…» of a row and «Изменить» choose its value by choose(value), which
   answers the value or null; «Удалить» takes the current away. What
   «Сохранить в файл» writes the inventory does not show (series 2.618,
   question): it stands unavailable. items are the values, text(value)
   writes one in its row. Answers the values on OK - rows left empty
   dropped - or null on «Отмена». */
async function editValueList(items, text, choose) {
  const rows = items.map(value => ({value}));
  let current = rows.length ? 0 : -1;
  const content = node('div', undefined, 'value-list-edit');
  const list = node('div', undefined, 'object-window-list value-list-edit-rows');
  list.setAttribute('role', 'listbox');
  const pick = async index => {
    const chosen = await choose(rows[index].value);
    if (chosen) rows[index].value = chosen;
    render();
  };
  const buttons = [
    panelButton('add', 'Добавить', true, () => { rows.splice(current + 1, 0, {value: null}); current += 1; render(); }),
    panelButton('copy', 'Скопировать', true, () => { rows.splice(current + 1, 0, {value: structuredClone(rows[current].value)}); current += 1; render(); }),
    panelButton('change', 'Изменить', true, () => pick(current)),
    panelButton('delete', 'Удалить', true, () => { rows.splice(current, 1); current = Math.min(current, rows.length - 1); render(); }),
    panelButton('save', 'Сохранить в файл', false, () => {}),
  ];
  const head = node('div', undefined, 'object-window-list-head');
  head.append(node('span', 'Список значений:'), ...buttons);
  function render() {
    for (const button of buttons.slice(1, 4)) button.disabled = current < 0;
    list.replaceChildren(...rows.map((row, index) => {
      const shown = node('input');
      shown.readOnly = true;
      shown.value = row.value ? text(row.value) : '';
      const line = node('div', undefined, 'object-window-list-row value-list-edit-row' + (index === current ? ' current' : ''));
      line.setAttribute('role', 'option');
      line.setAttribute('aria-selected', String(index === current));
      line.addEventListener('click', () => { if (current !== index) { current = index; render(); } });
      line.append(fieldWithButtons(shown, [fieldChoiceButton('Выбрать значение', () => pick(index))]));
      return line;
    }));
  }
  render();
  content.append(head, list);
  const answer = await showDialog({title: 'Редактирование списка значений', content, className: 'value-list-edit-dialog', help: 'value-list-edit',
    answers: [['ok', 'OK'], [null, 'Отмена']]});
  return answer === 'ok' ? rows.map(row => row.value).filter(Boolean) : null;
}

/* xmlTypeName writes a type named by the schema as the prototype writes a
   member of a union: «{namespace}name» (snapshot series-2.603/21), a
   namespace of the platform by its short name. */
function xmlTypeName(type) {
  return `{${namespaceShown(type.namespace)}}${type.name}`;
}

/* chooseXMLType is the window choosing a type named by the schema - of an
   operation and a parameter of a web service, of a node of an XDTO package
   (web-servis.md, «Выбор типа из палитры параметра»; paket-xdto.md, «Узлы
   редактора»; snapshots series-2.595/12-15, 63-65): titled «Выбор пакетов
   XDTO», as the prototype titles it, a tree of the namespaces by the
   alphabet of their URI, each holding its types (types, by namespace; a
   namespace whose types are not known holds none); the namespace of the
   type chosen before is open and the type marked; OK stands unavailable
   until a type is marked. Answers {namespace, name} on OK, null on
   «Отмена». */
async function chooseXMLType(current, namespaces, types) {
  let marked = current?.name ? {namespace: current.namespace, name: current.name} : null;
  const content = node('div', undefined, 'object-window-tree xml-type-choice');
  const fold = (one, other) => one.toLocaleLowerCase().localeCompare(other.toLocaleLowerCase());
  function render() {
    content.replaceChildren(...[...new Set(namespaces)].sort(fold).map(space => {
      const rows = (types[space] || []).map(name => {
        const {element, row} = treeNode({caption: name});
        row.classList.toggle('selected', marked?.namespace === space && marked?.name === name);
        row.addEventListener('click', () => { marked = {namespace: space, name}; render(); });
        return element;
      });
      return treeNode({caption: namespaceShown(space), children: rows, open: space === current?.namespace}).element;
    }));
  }
  render();
  const answer = await showDialog({title: 'Выбор пакетов XDTO', content, className: 'xml-type-choice-dialog',
    answers: [['ok', 'OK', {enabled: () => !!marked}], [null, 'Отмена']]});
  return answer === 'ok' ? marked : null;
}

/* chooseBorder is the window «Выбор рамки», one for every place a border is
   chosen - a value of a style, a style item, an element of a form (stil.md,
   «Досъёмка 27.10.2026»; snapshots series-2.584/033-049): «Из стиля» with
   the borders of the style, «Пример рамки», «Тип рамки» of eight (lines,
   metadata.BorderLines) and «Толщина» from 1 to the model's ceiling - 0 and
   6 are asked «Отменить ввод?» (textInput). «Нет рамки» keeps the
   thickness at 1 and shows «Нет просмотра». Marked «Из стиля», the window
   shows the type and thickness of the border of the style chosen and the
   thickness is unavailable. styles are the borders of the style - the
   platform's, then the project's - as {reference, caption, drawn: {line,
   width}}; value is the border chosen before (metadata.BorderValue) or
   none. Answers the border chosen on OK, null on «Отмена». How the type
   answers a click while «Из стиля» is marked the inventory does not show
   (series 2.595, question): the list does not take it. */
async function chooseBorder(value, styles, lines) {
  const same = (one, other) => JSON.stringify(one || null) === JSON.stringify(other || null);
  const fromStyle = value?.source === 'style';
  const chosen = {fromStyle, from: fromStyle ? value.from : styles[0]?.reference, line: !fromStyle && value?.line || 'none',
    width: !fromStyle && value?.width || 1};
  const content = node('div', undefined, 'border-choice');
  function render() {
    const style = styles.find(item => same(item.reference, chosen.from));
    const drawn = chosen.fromStyle ? style?.drawn || {line: 'none', width: 1} : {line: chosen.line, width: chosen.line === 'none' ? 1 : chosen.width};
    const mark = checkboxField('Из стиля', chosen.fromStyle, on => { chosen.fromStyle = on; render(); });
    const list = objectRows(styles.map((item, index) => [String(index), item.caption]), chosen.fromStyle && style ? String(styles.indexOf(style)) : null,
      picked => { chosen.from = styles[Number(picked)].reference; render(); });
    const sample = node('fieldset', undefined, 'studio-frame border-choice-sample');
    sample.append(node('legend', 'Пример рамки'));
    if (drawn.line === 'none') sample.append(node('div', 'Нет просмотра', 'border-choice-none'));
    else sample.append(borderSample(drawn));
    const types = objectRows(lines.map(item => [item.name, item.title]), drawn.line,
      picked => { if (chosen.fromStyle) return; chosen.line = picked; render(); });
    const thickness = wholeNumberInput(drawn.width, number => { chosen.width = number; render(); },
      {min: 1, max: StudioLimits.borderWidth, disabled: chosen.fromStyle || drawn.line === 'none'});
    const left = node('div', undefined, 'border-choice-left'), right = node('div', undefined, 'border-choice-right');
    left.append(mark, list, sample);
    right.append(node('div', 'Тип рамки', 'border-choice-caption'), types, labeledField('Толщина', thickness));
    content.replaceChildren(left, right);
  }
  render();
  const answer = await showDialog({title: 'Выбор рамки', content, side: true, className: 'border-choice-dialog', help: 'border-choice',
    answers: [['ok', 'OK'], [null, 'Отмена']]});
  if (answer !== 'ok') return null;
  if (chosen.fromStyle) return {source: 'style', from: structuredClone(chosen.from)};
  return {source: 'absolute', line: chosen.line, width: chosen.line === 'none' ? 1 : chosen.width};
}

/* borderSample draws a border of a type and a thickness (metadata.
   BorderValue): «Пример рамки» of «Выбор рамки» (snapshots series-2.584/
   042, 047) - a frame, double, raised or sunk, or a line under or over. */
function borderSample({line, width}) {
  const box = node('div', undefined, 'border-choice-box');
  const styles = {single: 'solid', double: 'double', embossed: 'outset', indented: 'inset', underline: 'solid', 'double-underline': 'double', overline: 'solid'};
  const side = {underline: 'Bottom', 'double-underline': 'Bottom', overline: 'Top'}[line] || '';
  box.style['border' + side] = `${styles[line] === 'double' ? Math.max(3, width) : width}px ${styles[line]} currentColor`;
  return box;
}

/* choosePicture is the window «Выбор картинки», one for every place a
   picture is chosen (docs/block-00/standartnye-kartinki.md, «Окно»;
   gruppa-komand.md, «Палитра»; snapshots standartnye-kartinki/19, 20):
   «Из библиотеки» with «Из проекта» - the common pictures of the
   project in the order of the tree, each with its image - and
   «Стандартные» - the pictures of the platform by their Russian names,
   the hidden ones left out; at the right the picture chosen with its
   size, under them «Очистить», «Сохранить», OK and «Отмена». The list
   marks no row until one is clicked, even one showing the picture set
   (snapshot 19); the tab of the picture set comes first, «Из
   конфигурации» when none is. «Очистить» takes the picture away and
   leaves the list as it is. The tabs «Из файла» and «Подбор из
   файла-коллекции», the arrows, «Добавить», «Изменить», «Удалить» and
   «Сохранить» wait for questions of series 2.618: the tabs are not shown,
   the buttons stand unavailable. value is a reference to a picture
   (metadata.PictureReference), pictures the names of the project's and
   the platform's (PictureChoice). Answers the reference chosen - the
   settings of its loading kept, as the prototype keeps them with no
   picture (METADATA-OBJECTS, «Ссылка без картинки»), null when nothing is
   left - or undefined after «Отмена». */
async function choosePicture(value, pictures) {
  let library = [];
  try { library = await (await fetch('/api/picture/library')).json(); } catch { library = []; }
  const images = new Map(library.map(item => [item.id, item]));
  const standard = new Map(pictures.standard.map(item => [item.name, item]));
  const chosen = {standard: value?.standard || '', common: value?.common || ''};
  let tab = chosen.standard ? 'standard' : 'common', marked = null;
  const imageOf = id => {
    const item = images.get(id);
    if (!item?.image) return null;
    const image = node('img');
    image.src = `/api/picture/image?path=${encodeURIComponent(item.path)}&name=${encodeURIComponent(item.image)}`;
    image.alt = item.name;
    return image;
  };
  const content = node('div', undefined, 'picture-choice');
  const inner = tabStrip([['common', 'Из проекта'], ['standard', 'Стандартные']], tab, key => { tab = key; marked = null; render(); });
  const list = node('div', undefined, 'picture-choice-list'), shown = node('div', undefined, 'picture-choice-image');
  const up = panelButton('move-up', 'Переместить вверх', false, () => {}), down = panelButton('move-down', 'Переместить вниз', false, () => {});
  const arrows = node('div', undefined, 'picture-choice-arrows');
  arrows.append(up, down);
  const below = node('div', undefined, 'picture-choice-buttons');
  for (const caption of ['Добавить', 'Изменить', 'Удалить']) {
    const button = node('button', caption);
    button.type = 'button'; button.disabled = true;
    below.append(button);
  }
  function render() {
    const rows = tab === 'common'
      ? pictures.common.map(item => [item.id, item.name, imageOf(item.id) || iconElement('')])
      : pictures.standard.filter(item => !item.hidden).map(item => [item.name, item.russian, iconElement(item.icon)]);
    list.replaceChildren(objectRows(rows, marked, key => {
      marked = key;
      Object.assign(chosen, tab === 'common' ? {standard: '', common: key} : {standard: key, common: ''});
      render();
    }));
    const drawn = chosen.common ? imageOf(chosen.common) : chosen.standard ? iconElement(standard.get(chosen.standard)?.icon || '') : null;
    const size = node('div', undefined, 'picture-choice-size');
    shown.replaceChildren(...drawn ? [drawn, size] : []);
    if (drawn?.tagName === 'IMG') drawn.addEventListener('load', () => { size.textContent = `${drawn.naturalWidth} x ${drawn.naturalHeight}`; });
    else if (drawn) size.textContent = '16 x 16';
  }
  render();
  const left = node('div', undefined, 'picture-choice-left');
  left.append(inner, list, below);
  const body = node('div', undefined, 'picture-choice-body');
  body.append(left, arrows, shown);
  content.append(tabStrip([['library', 'Из библиотеки']], 'library', () => {}), body);
  const any = () => !!(chosen.standard || chosen.common);
  const answer = await showDialog({title: 'Выбор картинки', content, className: 'picture-choice-dialog', answers: [
    ['clear', 'Очистить', {act: () => { chosen.standard = ''; chosen.common = ''; render(); }, enabled: any}],
    ['save', 'Сохранить', {act: () => {}, enabled: () => false}],
    ['ok', 'OK', {primary: () => true}], [null, 'Отмена']]});
  if (answer !== 'ok') return undefined;
  const reference = {...value || {}};
  delete reference.standard; delete reference.common;
  if (chosen.standard) reference.standard = chosen.standard;
  if (chosen.common) reference.common = chosen.common;
  return Object.keys(reference).length ? reference : null;
}

/* valueChoiceDialog shows a window choosing a value of a style - «Выбор
   цвета», «Выбор шрифта» - with OK and «Отмена», «Справка» only in its
   full kind (the simple kind of the palette of a style item has none,
   element-stilya.md); key names its class and its page of help. Answers
   whether OK was pressed. */
async function valueChoiceDialog(title, content, key, simple) {
  const answer = await showDialog({title, content, className: key + '-dialog', help: simple ? undefined : key, answers: [['ok', 'OK'], [null, 'Отмена']]});
  return answer === 'ok';
}

/* colorSwatches are the 40 samples under the palette of «Выбор цвета», four
   rows of ten from the left (stil.md, «Второй заход 27.10.2026»;
   configurator-2026-10-27/series-2.595/notes.md, В5). */
const colorSwatches = [
  '000000', '993300', '333300', '003300', '003366', '000080', '333399', '333333', '800000', 'FF6600',
  '808000', '008000', '008080', '0000FF', '666699', '808080', 'FF0000', 'FF9900', '99CC00', '339966',
  '33CCCC', '3366FF', '800080', '999999', 'FF00FF', 'FFCC00', 'FFFF00', '00FF00', '00FFFF', '00CCFF',
  '993366', 'C0C0C0', 'FF99CC', 'FFCC99', 'FFFF99', 'CCFFCC', 'CCFFFF', '99CCFF', 'CC99FF', 'FFFFFF',
].map(hex => '#' + hex);

/* colorNumbers is the three numbers of a colour written #RRGGBB;
   colorHex writes three numbers so. */
function colorNumbers(hex) {
  return [1, 3, 5].map(at => parseInt(hex.slice(at, at + 2), 16));
}

function colorHex(numbers) {
  return '#' + numbers.map(number => number.toString(16).padStart(2, '0')).join('').toUpperCase();
}

/* colorHSV and colorFromHSV turn three numbers to hue (0-360), saturation
   and value (0-1) and back - the coordinates of the palette of «Выбор
   цвета». */
function colorHSV([red, green, blue]) {
  const [r, g, b] = [red, green, blue].map(number => number / 255), top = Math.max(r, g, b), span = top - Math.min(r, g, b);
  let hue = 0;
  if (span) hue = top === r ? ((g - b) / span + 6) % 6 : top === g ? (b - r) / span + 2 : (r - g) / span + 4;
  return {hue: hue * 60, saturation: top ? span / top : 0, value: top};
}

function colorFromHSV({hue, saturation, value}) {
  const at = shift => { const k = (shift + hue / 60) % 6; return value - value * saturation * Math.max(0, Math.min(k, 4 - k, 1)); };
  return [at(5), at(3), at(1)].map(part => Math.round(part * 255));
}

/* chooseColor is the window «Выбор цвета», one for every place a colour is
   chosen (stil.md, «Окно», «Досъёмка» and «Второй заход 27.10.2026»;
   ML-STUDIO.md, «Окно стиля»; snapshots stil/07, series-2.584/024). Full,
   as the window of a style opens it: the tabs «Из стиля» (styles: the
   platform's 28 colours, then the project's) and «Web» (web: the web
   palette in the window's order), the palette - hue across, saturation
   down, the value on the bar beside it - «Красный», «Зеленый», «Синий»,
   «#», the 40 samples, OK, «Отмена», «Справка». Simple (options.simple),
   as the palette of a style item opens it (element-stilya.md,
   «Палитра»; snapshot series-2.595/05): one list, the colours of the style
   then the web palette, the three numbers, «Пример фона» and «Пример
   текста», OK and «Отмена». The tab «Windows», «Взять с экрана»,
   «Запомнить цвет» and the signs over the palette are not carried
   (ML-STUDIO.md). A row of a list is {key, caption, rgb, value} - rgb the
   colour it draws, #RRGGBB, or none where it is not known. A row answers
   its own value; the numbers, the palette, «#» and a sample answer the
   colour itself, even one some row draws too - the window then marks the
   first such row (obshchee.md, «Значения цвета, шрифта и рамки»). value is
   the colour chosen before (metadata.ColorValue) or none. Answers the
   colour chosen on OK, null on «Отмена». */
async function chooseColor(value, styles, web, options = {}) {
  const lists = options.simple ? [['all', [...styles, ...web]]] : [['style', styles], ['web', web]];
  const rows = lists.flatMap(([, list]) => list);
  const sameValue = (one, other) => JSON.stringify(one) === JSON.stringify(other);
  const opened = value && rows.find(row => sameValue(row.value, value));
  const chosen = {row: opened || null, rgb: value?.source === 'absolute' ? value.rgb : opened?.rgb || '#000000'};
  let tab = lists.find(([, list]) => list.includes(opened))?.[0] || lists[0][0];
  let hsv = colorHSV(colorNumbers(chosen.rgb));
  const content = node('div', undefined, 'color-choice' + (options.simple ? ' color-choice-simple' : ''));
  // Where the window marks a row: the one chosen, else the first of the tab
  // shown drawing the colour of the numbers.
  const marked = list => chosen.row && list.includes(chosen.row) ? chosen.row : chosen.row ? null : list.find(row => row.rgb === chosen.rgb) || null;
  function take(rgb, keepHSV) {
    chosen.row = null;
    chosen.rgb = rgb;
    if (!keepHSV) hsv = colorHSV(colorNumbers(rgb));
    render();
  }
  function listOf(list) {
    const current = marked(list), element = node('div', undefined, 'object-window-list color-choice-list');
    element.setAttribute('role', 'listbox');
    for (const row of list) {
      const line = node('div', undefined, 'object-window-list-row color-choice-row' + (row === current ? ' current' : ''));
      line.setAttribute('role', 'option');
      line.setAttribute('aria-selected', String(row === current));
      const sample = node('span', undefined, 'color-choice-sample');
      if (row.rgb) sample.style.background = row.rgb;
      line.append(sample, node('span', row.caption));
      line.title = row.caption;
      line.addEventListener('click', () => {
        chosen.row = row;
        if (row.rgb) { chosen.rgb = row.rgb; hsv = colorHSV(colorNumbers(row.rgb)); }
        render();
      });
      element.append(line);
    }
    return element;
  }
  function numbers() {
    const frame = node('div', undefined, 'color-choice-numbers');
    const parts = colorNumbers(chosen.rgb);
    for (const [index, caption] of ['Красный', 'Зеленый', 'Синий'].entries()) {
      frame.append(labeledField(caption + (options.simple ? ':' : ''), wholeNumberInput(parts[index], number => {
        const next = [...parts];
        next[index] = number;
        take(colorHex(next));
      }, {min: 0, max: 255})));
    }
    return frame;
  }
  // A drag over the palette or its bar: where the pointer goes, set(x, y)
  // in parts of the element's size.
  function dragged(element, set) {
    element.addEventListener('pointerdown', event => {
      const at = move => {
        const box = element.getBoundingClientRect();
        set(Math.min(1, Math.max(0, (move.clientX - box.left) / box.width)), Math.min(1, Math.max(0, (move.clientY - box.top) / box.height)));
      };
      element.setPointerCapture?.(event.pointerId);
      at(event);
      const follow = move => at(move);
      element.addEventListener('pointermove', follow);
      element.addEventListener('pointerup', () => element.removeEventListener('pointermove', follow), {once: true});
    });
    return element;
  }
  function palette() {
    const square = dragged(node('div', undefined, 'color-choice-palette'), (x, y) => {
      hsv = {...hsv, hue: x * 360, saturation: 1 - y, value: hsv.value || 1};
      take(colorHex(colorFromHSV(hsv)), true);
    });
    const spot = node('span', undefined, 'color-choice-spot');
    spot.style.left = `${hsv.hue / 360 * 100}%`;
    spot.style.top = `${(1 - hsv.saturation) * 100}%`;
    square.append(spot);
    const bar = dragged(node('div', undefined, 'color-choice-value'), (x, y) => {
      hsv = {...hsv, value: 1 - y};
      take(colorHex(colorFromHSV(hsv)), true);
    });
    bar.style.background = `linear-gradient(${colorHex(colorFromHSV({...hsv, value: 1}))}, #000)`;
    const arrow = node('span', undefined, 'color-choice-arrow');
    arrow.style.top = `${(1 - hsv.value) * 100}%`;
    bar.append(arrow);
    const frame = node('div', undefined, 'color-choice-picker');
    frame.append(square, bar);
    return frame;
  }
  function hexField() {
    const input = node('input');
    input.value = chosen.rgb.slice(1);
    input.maxLength = 6;
    input.setAttribute('aria-label', '#');
    input.addEventListener('change', async () => {
      if (/^[0-9a-f]{6}$/i.test(input.value)) { take('#' + input.value.toUpperCase()); return; }
      if (await askCancelInput()) input.value = chosen.rgb.slice(1); else input.focus();
    });
    const field = node('label', undefined, 'catalog-field color-choice-hex');
    field.append(node('span', '#'), input);
    return field;
  }
  function swatches() {
    const grid = node('div', undefined, 'color-choice-swatches');
    for (const rgb of colorSwatches) {
      const cell = node('button', undefined, 'color-choice-swatch' + (!chosen.row && rgb === chosen.rgb ? ' current' : ''));
      cell.type = 'button';
      cell.style.background = rgb;
      cell.title = colorNumbers(rgb).join(', ');
      cell.addEventListener('click', () => take(rgb));
      grid.append(cell);
    }
    return grid;
  }
  function render() {
    if (options.simple) {
      const background = node('div', 'Пример фона', 'color-choice-background'), text = node('div', 'Пример текста', 'color-choice-text');
      background.style.background = chosen.rgb;
      background.style.color = colorHSV(colorNumbers(chosen.rgb)).value > 0.6 ? '#000' : '#fff';
      text.style.color = chosen.rgb;
      const samples = node('div', undefined, 'color-choice-examples');
      samples.append(background, text);
      content.replaceChildren(listOf(lists[0][1]), numbers(), samples);
      return;
    }
    const tabs = tabStrip([['style', 'Из стиля'], ['web', 'Web']], tab, key => { tab = key; render(); });
    const left = node('div', undefined, 'color-choice-left');
    left.append(tabs, listOf(lists.find(([key]) => key === tab)[1]));
    const sample = node('div', undefined, 'color-choice-chosen');
    sample.style.background = chosen.rgb;
    const right = node('div', undefined, 'color-choice-right');
    right.append(sample, numbers(), hexField());
    const middle = node('div', undefined, 'color-choice-middle');
    middle.append(palette(), swatches());
    content.replaceChildren(left, middle, right);
  }
  render();
  if (!await valueChoiceDialog('Выбор цвета', content, 'color-choice', options.simple)) return null;
  return chosen.row ? structuredClone(chosen.row.value) : {source: 'absolute', rgb: chosen.rgb};
}

/* fontSizes are the sizes the lists of «Выбор шрифта» offer (stil.md,
   «Второй заход 27.10.2026»). */
const fontSizes = [8, 9, 10, 11, 12, 14, 16, 18, 20, 22, 24, 26, 28, 36, 48, 72];

/* fontFlags are the four switches of a font, in the order the windows and
   the text of a value name them (obshchee.md, «Значения цвета, шрифта и
   рамки»): [field of metadata.FontValue, caption, word of the text]. */
const fontFlags = [['bold', 'Полужирный', 'полужирный'], ['italic', 'Наклонный', 'наклонный'],
  ['underline', 'Подчеркнутый', 'подчеркнутый'], ['strikeout', 'Зачеркнутый', 'зачеркнутый']];

/* fontPresentation is how a field shows a font (metadata.FontValue) - the
   platform's rule, obshchee.md, «Значения цвета, шрифта и рамки»: what it
   is based on - «стиль: <элемент>» (base(value) names the item), the
   font of the system by its caption - or its face, then the size, the
   switches said on in the order of fontFlags, the scale where it is not a
   hundred: «стиль: Обычный шрифт текста, 12, полужирный, 150%», «Al Bayan,
   8, наклонный». base answers the caption of what value is based on, or
   '' where it is not known. */
function fontPresentation(value, base) {
  if (!value) return '';
  const parts = [];
  if (value.source === 'style' || value.source === 'system') {
    const caption = base(value);
    if (!caption) return '';
    parts.push(value.source === 'style' ? `стиль: ${caption}` : caption);
    if (value.face) parts.push(value.face);
  } else if (value.source === 'absolute') parts.push(value.face || '');
  else return '';
  if (value.size) parts.push(String(value.size));
  for (const [field, , word] of fontFlags) if (value[field]) parts.push(word);
  if (value.scale && value.scale !== 100) parts.push(`${value.scale}%`);
  return parts.join(', ');
}

/* chooseFont is the window «Выбор шрифта», one for every place a font is
   chosen (stil.md, «Окно», «Второй заход 27.10.2026»; ML-STUDIO.md, «Окно
   стиля»; snapshots stil/08, series-2.595/58). Full, as the window of a
   style opens it: «Шрифт стиля» with the fonts of the style (styles: the
   platform's five, then the project's, as {caption, value, drawn} - drawn
   the face and switches it is known to have), «Системный шрифт» with the
   fonts of the system (systems, {name, title}) - the two take each other
   away - the face, the size and the switches each with «Из стиля»,
   «Масштаб», the sample, OK, «Отмена», «Справка». A font of the style
   chosen marks the three «Из стиля» again and drops what was changed;
   the scale stays as it is (it has no «Из стиля»);
   with neither base the font is its own - Arial, 13 - and «Из стиля»
   stands unmarked and unavailable; unmarked, the face is empty with the
   faces of the system (faces) below, the size 13 with fontSizes, the
   switches unmarked. Simple (options.simple), as the palette of a style
   item opens it (element-stilya.md; snapshot series-2.595/02): one list -
   the fonts of the style in angle brackets, then the faces - the size
   with «<>» first, «Масштаб», the switches, the sample, OK and «Отмена»;
   a font of the style drops the switches and keeps the size and the
   scale, a face after it sets the size 8. value is the font chosen before
   (metadata.FontValue) or none. Answers the font chosen on OK, null on
   «Отмена». */
async function chooseFont(value, styles, systems, faces, options = {}) {
  const same = (one, other) => JSON.stringify(one) === JSON.stringify(other);
  const flagsOf = font => Object.fromEntries(fontFlags.map(([field]) => [field, !!font?.[field]]));
  const said = value && fontFlags.some(([field]) => value[field] !== undefined);
  const chosen = {
    style: value?.source === 'style' ? styles.find(row => same(row.value.from, value.from)) || null : null,
    system: value?.source === 'system' ? systems.find(row => row.name.toLowerCase() === value.system.toLowerCase()) || null : null,
    face: value?.face || '', size: value?.size || 0, flags: flagsOf(value), scale: value?.scale || 100,
  };
  chosen.base = chosen.style ? 'style' : chosen.system ? 'system' : value && value.source !== 'absolute' ? 'style' : 'own';
  if (chosen.base === 'style' && !chosen.style) chosen.style = styles[0];
  if (chosen.base === 'own') { chosen.face = chosen.face || 'Arial'; chosen.size = chosen.size || 13; }
  chosen.fromStyle = chosen.base === 'own' ? {face: false, size: false, flags: false}
    : {face: !value?.face, size: !value?.size, flags: !said};
  // The simple window has no «Из стиля»: its size «<>» is the size left out.
  if (options.simple) chosen.fromStyle = {face: !chosen.face, size: false, flags: false};
  const content = node('div', undefined, 'font-choice' + (options.simple ? ' font-choice-simple' : ''));
  const baseRow = () => chosen.base === 'style' ? chosen.style : chosen.base === 'system' ? chosen.system : null;
  // What the font is drawn with: what is chosen, else what its base is
  // known to have.
  function drawn() {
    const known = chosen.base === 'style' ? chosen.style?.drawn || {} : {};
    const face = chosen.fromStyle.face ? known.face || '' : chosen.face;
    const size = chosen.fromStyle.size ? known.size || 0 : chosen.size;
    const flags = chosen.fromStyle.flags ? flagsOf(known) : chosen.flags;
    return {face, size, flags, caption: face || baseRow()?.caption || baseRow()?.title || ''};
  }
  function rows(list, current, choose) {
    const element = objectRows(list.map((caption, index) => [String(index), caption]), String(current), key => { choose(Number(key)); render(); });
    element.classList.add('font-choice-list');
    return element;
  }
  function pickStyle(row) {
    chosen.base = 'style';
    chosen.style = row;
    if (options.simple) { chosen.flags = flagsOf(null); chosen.face = ''; chosen.fromStyle.face = true; return; }
    chosen.fromStyle = {face: true, size: true, flags: true};
    chosen.face = '';
    chosen.size = 0;
    chosen.flags = flagsOf(null);
  }
  function becomeOwn() {
    chosen.base = 'own';
    chosen.style = chosen.system = null;
    chosen.fromStyle = {face: false, size: false, flags: false};
    chosen.face = 'Arial';
    chosen.size = 13;
    chosen.flags = flagsOf(null);
  }
  function sizeField(disabled) {
    return textInput(disabled ? '' : String(chosen.size || (options.simple ? '' : 0)), number => {
      if (!(number > 0)) return false;
      chosen.size = number;
      chosen.fromStyle.size = false;
      render();
    }, {type: 'number', min: 1, disabled});
  }
  function sample() {
    const font = drawn(), frame = node('div', undefined, 'studio-frame font-choice-sample'), text = node('span', font.face ? font.face : font.caption);
    if (font.face) text.style.fontFamily = `"${font.face}"`;
    if (font.size) text.style.fontSize = `${Math.min(font.size, 48) * chosen.scale / 100}px`;
    if (font.flags.bold) text.style.fontWeight = 'bold';
    if (font.flags.italic) text.style.fontStyle = 'italic';
    text.style.textDecoration = [font.flags.underline && 'underline', font.flags.strikeout && 'line-through'].filter(Boolean).join(' ') || 'none';
    frame.append(text);
    return frame;
  }
  function flagsFrame() {
    const frame = node('fieldset', undefined, 'studio-frame font-choice-flags');
    frame.append(node('legend', 'Начертание'));
    const own = !chosen.fromStyle.flags, shown = own ? chosen.flags : drawn().flags;
    if (!options.simple) frame.append(checkboxField('Из стиля', chosen.fromStyle.flags, on => {
      chosen.fromStyle.flags = on;
      chosen.flags = flagsOf(null);
      render();
    }, 'catalog-check', chosen.base === 'own'));
    const grid = node('div', undefined, 'font-choice-flag-grid');
    for (const [field, caption] of fontFlags) grid.append(checkboxField(caption, shown[field], on => { chosen.flags[field] = on; render(); }, 'catalog-check', !own));
    frame.append(grid);
    return frame;
  }
  function scaleField() {
    const field = labeledField('Масштаб', wholeNumberInput(chosen.scale, number => { chosen.scale = number; render(); },
      {min: 0, max: StudioLimits.fontScale}), 'catalog-field font-choice-scale');
    field.append(node('span', '%'));
    return field;
  }
  function renderSimple() {
    const list = [...styles.map(row => `<${row.caption}>`), ...faces];
    const current = chosen.base === 'style' ? styles.indexOf(chosen.style) : faces.indexOf(chosen.face) >= 0 ? styles.length + faces.indexOf(chosen.face) : -1;
    const fonts = rows(list, current, index => {
      if (index < styles.length) { pickStyle(styles[index]); return; }
      if (chosen.base === 'style') chosen.size = 8;
      chosen.base = 'own';
      chosen.style = null;
      chosen.face = faces[index - styles.length];
      chosen.fromStyle.face = false;
    });
    const sizes = rows(['<>', ...fontSizes.map(String)], chosen.size ? fontSizes.indexOf(chosen.size) + 1 : 0, index => {
      chosen.size = index ? fontSizes[index - 1] : 0;
    });
    const left = node('div', undefined, 'font-choice-left'), right = node('div', undefined, 'font-choice-right');
    left.append(labeledField('Шрифт', fonts, 'catalog-field font-choice-column'), sample());
    right.append(labeledField('Размер', sizes, 'catalog-field font-choice-column'), scaleField(), flagsFrame());
    content.replaceChildren(left, right);
  }
  function render() {
    if (options.simple) { renderSimple(); return; }
    const own = chosen.base === 'own', font = drawn();
    const styleMark = checkboxField('Шрифт стиля', chosen.base === 'style', on => {
      if (on) pickStyle(chosen.style || styles[0]); else becomeOwn();
      render();
    });
    const styleList = rows(styles.map(row => row.caption), chosen.base === 'style' ? styles.indexOf(chosen.style) : -1, index => pickStyle(styles[index]));
    const systemMark = checkboxField('Системный шрифт', chosen.base === 'system', on => {
      if (on) { if (chosen.base === 'own') chosen.fromStyle = {face: true, size: true, flags: true}; chosen.base = 'system'; chosen.system = chosen.system || systems[0]; chosen.style = null; }
      else becomeOwn();
      render();
    });
    const systemList = rows(systems.map(row => row.title), chosen.base === 'system' ? systems.indexOf(chosen.system) : -1, index => {
      if (chosen.base === 'own') chosen.fromStyle = {face: true, size: true, flags: true};
      chosen.base = 'system';
      chosen.system = systems[index];
      chosen.style = null;
    });
    const faceMark = checkboxField('Из стиля', chosen.fromStyle.face, on => { chosen.fromStyle.face = on; chosen.face = ''; render(); }, 'catalog-check', own);
    const sizeMark = checkboxField('Из стиля', chosen.fromStyle.size, on => { chosen.fromStyle.size = on; chosen.size = on ? 0 : 13; render(); }, 'catalog-check', own);
    const faceInput = textInput(chosen.fromStyle.face ? font.face : chosen.face, text => { chosen.face = text; }, {disabled: chosen.fromStyle.face});
    faceInput.addEventListener('change', render);
    const faceList = rows(faces, chosen.fromStyle.face ? -1 : faces.indexOf(chosen.face), index => { chosen.face = faces[index]; });
    const sizeList = rows(fontSizes.map(String), chosen.fromStyle.size ? -1 : fontSizes.indexOf(chosen.size), index => { chosen.size = fontSizes[index]; });
    const left = node('div', undefined, 'font-choice-left');
    left.append(styleMark, styleList, systemMark, systemList, sample());
    const faceColumn = node('div', undefined, 'font-choice-face'), sizeColumn = node('div', undefined, 'font-choice-size');
    faceColumn.append(faceMark, faceInput, faceList);
    sizeColumn.append(sizeMark, sizeField(chosen.fromStyle.size), sizeList);
    const top = node('div', undefined, 'font-choice-top');
    top.append(faceColumn, sizeColumn);
    const right = node('div', undefined, 'font-choice-right');
    right.append(top, flagsFrame(), scaleField());
    content.replaceChildren(left, right);
  }
  render();
  if (!await valueChoiceDialog('Выбор шрифта', content, 'font-choice', options.simple)) return null;
  const result = chosen.base === 'style' ? {source: 'style', from: structuredClone(chosen.style.value.from)}
    : chosen.base === 'system' ? {source: 'system', system: chosen.system.name} : {source: 'absolute'};
  if (!chosen.fromStyle.face && chosen.face) result.face = chosen.face;
  if (!chosen.fromStyle.size && chosen.size) result.size = chosen.size;
  const based = chosen.base !== 'own' && !options.simple;
  if (!chosen.fromStyle.flags) for (const [field] of fontFlags) if (chosen.flags[field] || based) result[field] = chosen.flags[field];
  if (chosen.scale !== 100) result.scale = chosen.scale;
  return result;
}

/* systemFaces is the faces of the fonts of the system, as the server of
   Studio reads them once (/api/fonts; ML-STUDIO.md, «Окно стиля»): one
   list for every window choosing a font, fetched once. */
let systemFacesRead = null;
function systemFaces() {
  systemFacesRead ||= fetch('/api/fonts').then(response => response.ok ? response.json() : []).catch(() => []);
  return systemFacesRead;
}

/* typeEditorModel is the window «Редактирование типа данных» without its
   drawing, as the prototype's behaves (docs/block-00/obshchee.md, «Окно
   «Редактирование типа данных»», 23.10.2026). offer is the model's
   TypeOffer of the place: its rows, limits and the qualifiers of a type
   marked anew; types is the description the window opens on. A key names a
   row: the kind of its type, «arbitrary», «folder:<kind>» for a folder, and
   «<kind>:<id>» for a type naming an object. A folder holds the objects of
   its kind or, the enumerations of the platform, types naming none. An offer
   that is single - a field of an external source - has no «Составной тип
   данных», and a mark always takes the one before away. A row the place
   takes only by itself (TypeOfferRow.alone, and on a folder for the objects
   inside it) and «Произвольный» stand alone in a composite too:
   exclusion() names the question the window asks before such a mark. A type
   of the description the place does not offer - what the model takes beyond
   the window, a type of an earlier version - is kept as it is, after the
   types of the tree, until a mark that takes every other away takes it too.
   The window answers result(): {types} in the order of the tree, or
   {refusal}. */
function typeEditorModel(offer, types) {
  const rows = [];
  for (const [index, row] of offer.rows.entries()) {
    const key = row.arbitrary ? 'arbitrary' : row.kind || `folder:${row.objectKind || ''}:${index}`;
    const children = [
      ...(row.objects || []).map(object => ({key: `${row.objectKind}:${object.id}`, caption: object.name, icon: row.icon, checkable: true,
        type: {kind: row.objectKind, reference: object.id}, alone: !!row.alone})),
      ...(row.types || []).map(inside => ({key: inside.kind, caption: inside.name.russian, icon: inside.icon, checkable: true,
        type: {kind: inside.kind}, alone: !!inside.alone})),
    ];
    rows.push({key, caption: row.name.russian, icon: row.icon, checkable: !!(row.kind || row.arbitrary),
      type: row.kind ? {kind: row.kind} : null, children, alone: !!(row.alone || row.arbitrary)});
  }
  const order = rows.flatMap(row => [row, ...row.children]);
  const typeKey = type => type.reference ? `${type.kind}:${type.reference}` : type.kind;
  const marked = new Map(), drafts = new Map(), kept = [];
  for (const type of types) {
    if (order.some(row => row.key === typeKey(type))) marked.set(typeKey(type), structuredClone(type));
    else kept.push(structuredClone(type));
  }
  if (!types.length && rows.some(row => row.key === 'arbitrary')) marked.set('arbitrary', null);
  let composite = !offer.single && marked.size + kept.length > 1, search = '';
  const expanded = new Set(rows.filter(row => row.children.some(child => marked.has(child.key))).map(row => row.key));
  // The current row is the last one marked in the order of the tree, not the
  // first and not the one marked last (snapshots tip-95…106).
  let current = [...order].reverse().find(row => marked.has(row.key))?.key || order[0]?.key;
  const find = key => order.find(row => row.key === key);
  const standsAlone = key => !!find(key)?.alone;
  function fresh(row) {
    if (!row.type) return null;
    return drafts.get(row.key) || structuredClone(offer.newTypes?.[row.type.kind] || row.type);
  }
  function terms() {
    const words = search.trim().toLocaleLowerCase().split(/\s+/).filter(Boolean);
    return words.join('').length >= 2 ? words : [];
  }
  function matches(caption, words) {
    const lower = caption.toLocaleLowerCase();
    if (!words.every(word => lower.includes(word))) return null;
    return words.map(word => [lower.indexOf(word), word.length]).sort((a, b) => a[0] - b[0]);
  }
  return {
    get composite() { return composite; },
    get current() { return current; },
    get search() { return search; },
    /* visible is what the tree shows, row by row: {key, caption, icon,
       depth, checkable, checked, folds, open, highlights}. A search of two
       characters and more shows the rows whose caption holds every word of
       it, a type naming an object under its group unfolded, and hides the
       rest - the marked ones too. */
    visible() {
      const words = terms(), shown = [];
      for (const row of rows) {
        const own = words.length ? matches(row.caption, words) : [];
        const inside = row.children.flatMap(child => {
          const hit = words.length ? matches(child.caption, words) : [];
          return hit ? [[child, hit]] : [];
        });
        if (words.length && !own && !inside.length) continue;
        const folds = row.children.length > 0 && (!words.length || inside.length > 0);
        const open = folds && (words.length ? true : expanded.has(row.key));
        shown.push({key: row.key, caption: row.caption, icon: row.icon, depth: 0, checkable: row.checkable, checked: marked.has(row.key),
          folds, open, highlights: words.length && own ? own : []});
        if (open) for (const [child, hit] of (words.length ? inside : row.children.map(child => [child, []]))) {
          shown.push({key: child.key, caption: child.caption, icon: child.icon, depth: 1, checkable: true, checked: marked.has(child.key),
            folds: false, open: false, highlights: hit});
        }
      }
      return shown;
    },
    setSearch(text) { search = text; },
    fold(key, open) { if (open) expanded.add(key); else expanded.delete(key); },
    choose(key) { if (find(key)) current = key; },
    /* exclusion is the question the prototype asks before key is marked in
       a composite (snapshots series-2.531/12, series-2.535/10): «excludes-
       others» - the type stands alone and others are marked; «excludes-
       earlier» - a type marked before stands alone; null - none. */
    exclusion(key) {
      if (!composite || marked.has(key) || !find(key)?.checkable) return null;
      if (standsAlone(key) && marked.size + kept.length > 0) return 'excludes-others';
      if ([...marked.keys()].some(standsAlone)) return 'excludes-earlier';
      return null;
    },
    /* mark marks a type or takes it away, and makes its row current.
       Without «Составной тип данных» a mark takes the one before away; a
       type standing alone, or beside one marked before that does, takes
       every other away - the window has asked exclusion() first. */
    mark(key, checked) {
      const row = find(key);
      if (!row?.checkable) return;
      current = key;
      if (!checked) {
        if (marked.has(key) && key !== 'arbitrary') drafts.set(key, marked.get(key));
        marked.delete(key);
        return;
      }
      if (!composite || standsAlone(key) || [...marked.keys()].some(standsAlone)) {
        for (const [other, type] of marked) if (type) drafts.set(other, type);
        marked.clear();
        kept.length = 0;
      }
      marked.set(key, key === 'arbitrary' ? null : fresh(row));
    },
    /* setComposite: taking «Составной тип данных» away leaves the first type
       marked in the order of the tree, without asking, and makes it current. */
    setComposite(on) {
      if (offer.single) return;
      composite = on;
      if (on || marked.size + kept.length < 2) return;
      if (!marked.size) { kept.splice(1); return; }
      kept.length = 0;
      if (marked.size < 2) return;
      const first = order.find(row => marked.has(row.key));
      for (const [key, type] of marked) if (key !== first.key && type) drafts.set(key, type);
      for (const key of [...marked.keys()]) if (key !== first.key) marked.delete(key);
      current = first.key;
    },
    /* qualifiers are those of the current row: {kind, type, enabled}, the
       row unmarked showing them unavailable; null for a type without. */
    qualifiers() {
      const row = find(current);
      if (!row?.type || !['string', 'binary-data', 'number', 'date'].includes(row.type.kind)) return null;
      return {kind: row.type.kind, type: marked.get(current) || fresh(row), enabled: marked.has(current)};
    },
    /* setQualifier writes one qualifier of the current type and answers
       false for a value beyond the model's ceiling, which is not written.
       A fraction longer than the number lengthens the number, a number
       shorter than its fraction shortens the fraction; a string of length 0
       is unlimited, and a string longer than a fixed one may be is of
       variable length. Binary data takes what a string does, under a
       ceiling of its own. */
    setQualifier(field, value) {
      const type = marked.get(current);
      if (!type) return true;
      const limits = offer.limits;
      if (field === 'precision' || field === 'scale') {
        if (!Number.isInteger(value) || value < (field === 'precision' ? 1 : 0) || value > limits.numberDigits) return false;
        type[field] = value;
        if (field === 'scale' && (type.precision || 0) < value) type.precision = value;
        if (field === 'precision' && (type.scale || 0) > value) type.scale = value;
      } else if (field === 'length') {
        const ceiling = type.kind === 'binary-data' ? limits.binaryLength : limits.stringLength;
        if (!Number.isInteger(value) || value < 0 || value > ceiling) return false;
        type.length = value;
        if (!value || value > limits.fixedStringLength) delete type.fixedLength;
      } else if (field === 'unlimited') {
        type.length = value ? 0 : 10;
        if (value) delete type.fixedLength;
      } else if (field === 'fixedLength') {
        if (value && (!type.length || type.length > limits.fixedStringLength)) return true;
        if (value) type.fixedLength = true; else delete type.fixedLength;
      } else if (field === 'nonNegative') {
        if (value) type.nonNegative = true; else delete type.nonNegative;
      } else if (field === 'dateParts') {
        type.dateParts = value;
      }
      for (const name of ['scale', 'length']) if (type[name] === 0) delete type[name];
      return true;
    },
    result() {
      const chosen = [...order.filter(row => marked.has(row.key) && row.key !== 'arbitrary').map(row => marked.get(row.key)),
        ...kept.map(type => structuredClone(type))];
      if (!chosen.length && offer.required) return {refusal: 'Необходимо выбрать тип значения.'};
      return {types: chosen};
    },
  };
}

/* typePresentation is a type description as the palette shows it: its types
   by their names, a type naming an object with the object's name after a
   point, in the order of the description. */
function typePresentation(offer, types) {
  if (!types.length) return offer.rows.find(row => row.arbitrary)?.name.russian || '';
  const rows = offer.rows.flatMap(row => [row, ...(row.types || [])]);
  return types.map(type => {
    const row = rows.find(item => type.reference ? item.objectKind === type.kind : item.kind === type.kind);
    const name = row?.name.russian || type.kind;
    if (!type.reference) return name;
    const object = row?.objects?.find(item => item.id === type.reference);
    return `${name}.${object?.name || type.reference}`;
  }).join(', ');
}

/* typeExclusionQuestions are the prototype's questions of a mark that takes
   other types away, word for word (snapshots
   configurator-2026-10-24/series-2.531/12 and
   configurator-2026-10-25/series-2.535/10). */
const typeExclusionQuestions = {
  'excludes-others': 'Выбран тип, который не может быть включен в составной тип данных. Будут исключены остальные типы данных. Продолжить?',
  'excludes-earlier': 'Ранее был выбран тип, который не может быть включен в составной тип данных и будет исключен. Продолжить?',
};

/* showTypeEditor opens the window «Редактирование типа данных» on types,
   offering what offer holds, and answers the description chosen, or null
   for «Отмена». One window chooses every type of Studio. */
function showTypeEditor(offer, types) {
  const model = typeEditorModel(offer, types);
  /* mark marks a type, asking first what the prototype asks when the mark
     takes others away: «Да» marks, «Нет» leaves the marks as they were. */
  async function mark(key, checked) {
    const question = checked ? typeExclusionQuestions[model.exclusion(key)] : null;
    if (!question || await showDialog({title: question, answers: [['yes', 'Да'], ['no', 'Нет']]}) === 'yes') model.mark(key, checked);
    draw();
  }
  const content = node('div', undefined, 'type-editor');
  const composite = checkboxField('Составной тип данных', model.composite, on => { model.setComposite(on); draw(); }, 'type-editor-composite');
  const search = node('input', undefined, 'type-editor-search');
  search.type = 'search';
  search.placeholder = 'Поиск (Cmd+Option+M)';
  search.addEventListener('input', () => { model.setSearch(search.value); draw(); });
  search.addEventListener('keydown', event => {
    if (event.key !== 'Escape' || !search.value) return;
    event.preventDefault();
    search.value = '';
    model.setSearch('');
    draw();
  });
  const tree = node('div', undefined, 'type-editor-tree');
  tree.tabIndex = 0;
  tree.setAttribute('role', 'tree');
  const qualifiers = node('div', undefined, 'type-editor-qualifiers');
  content.append(...(offer.single ? [] : [composite]), search, tree, qualifiers);
  content.addEventListener('keydown', event => {
    if (event.code === 'KeyM' && event.altKey && (event.metaKey || event.ctrlKey)) { event.preventDefault(); search.focus(); }
  });
  tree.addEventListener('keydown', event => {
    const shown = model.visible(), at = shown.findIndex(row => row.key === model.current);
    if (markListKey(event, shown, model.current, key => { model.choose(key); draw(); }, (key, checked) => { mark(key, checked); })) return;
    if ((event.key === 'ArrowRight' || event.key === 'ArrowLeft') && at >= 0 && shown[at].folds) {
      event.preventDefault();
      model.fold(shown[at].key, event.key === 'ArrowRight');
      draw();
    }
  });
  function caption(row) {
    const text = node('span', undefined, 'type-editor-caption');
    let from = 0;
    for (const [start, length] of row.highlights) {
      if (start < from) continue;
      text.append(row.caption.slice(from, start), node('span', row.caption.slice(start, start + length), 'type-editor-match'));
      from = start + length;
    }
    text.append(row.caption.slice(from));
    return text;
  }
  function drawTree() {
    tree.replaceChildren();
    for (const row of model.visible()) {
      const line = node('div', undefined, 'type-editor-row' + (row.key === model.current ? ' current' : ''));
      line.style.setProperty('--depth', row.depth);
      line.setAttribute('role', 'treeitem');
      line.setAttribute('aria-selected', String(row.key === model.current));
      const sign = node('button', row.folds ? (row.open ? '⊖' : '⊕') : '', 'type-editor-sign');
      sign.type = 'button';
      sign.tabIndex = -1;
      if (row.folds) {
        sign.setAttribute('aria-label', `${row.open ? 'Свернуть' : 'Развернуть'} ${row.caption}`);
        sign.addEventListener('click', () => { model.fold(row.key, !row.open); draw(); });
      } else sign.disabled = true;
      line.append(sign);
      if (row.checkable) {
        const box = node('input');
        box.type = 'checkbox';
        box.checked = row.checked;
        box.tabIndex = -1;
        box.setAttribute('aria-label', row.caption);
        box.addEventListener('change', () => { mark(row.key, box.checked); });
        line.append(box);
      } else line.append(node('span', undefined, 'type-editor-nobox'));
      line.append(row.icon ? iconElement(row.icon) : node('span', undefined, 'type-editor-noicon'), caption(row));
      line.addEventListener('click', event => {
        if (event.target.tagName === 'INPUT' || event.target.closest('.type-editor-sign')) return;
        model.choose(row.key);
        draw();
        tree.focus();
      });
      tree.append(line);
    }
    tree.querySelector('.type-editor-row.current')?.scrollIntoView?.({block: 'nearest'});
  }
  /* A number field of a qualifier: the model takes it or refuses it, and a
     refused one asks textInput's question. */
  function numberField(label, field, value, enabled) {
    const input = textInput(String(value || 0), entered => {
      if (!model.setQualifier(field, entered)) return false;
      draw();
    }, {type: 'number', disabled: !enabled});
    input.classList.add('type-editor-number');
    return labeledField(label, input, 'type-editor-field');
  }
  function drawQualifiers() {
    qualifiers.replaceChildren();
    const shown = model.qualifiers();
    if (!shown) return;
    const {kind, type, enabled} = shown;
    if (kind === 'number') {
      qualifiers.append(numberField('Длина', 'precision', type.precision, enabled), numberField('Точность', 'scale', type.scale, enabled),
        checkboxField('Неотрицательное', !!type.nonNegative, on => { model.setQualifier('nonNegative', on); draw(); }, 'type-editor-check', !enabled));
    } else if (kind === 'string' || kind === 'binary-data') {
      const unlimited = !type.length, fixedAllowed = !unlimited && type.length <= offer.limits.fixedStringLength;
      const allowed = selectInput([['fixed', 'Фиксированная'], ['variable', 'Переменная']], type.fixedLength ? 'fixed' : 'variable',
        value => { model.setQualifier('fixedLength', value === 'fixed'); draw(); }, 'Допустимая длина');
      allowed.disabled = !enabled || !fixedAllowed;
      qualifiers.append(numberField('Длина', 'length', type.length, enabled), allowed,
        checkboxField('Неограниченная', unlimited, on => { model.setQualifier('unlimited', on); draw(); }, 'type-editor-check', !enabled));
    } else if (kind === 'date') {
      const parts = selectInput([['date', 'Дата'], ['time', 'Время'], ['date-time', 'Дата и время']], type.dateParts || 'date-time',
        value => { model.setQualifier('dateParts', value); draw(); }, 'Состав даты');
      parts.disabled = !enabled;
      qualifiers.append(labeledField('Состав даты', parts, 'type-editor-field'));
    }
  }
  function draw() {
    composite.querySelector('input').checked = model.composite;
    drawTree();
    drawQualifiers();
  }
  draw();
  return showDialog({title: 'Редактирование типа данных', content, className: 'type-editor-dialog',
    answers: [['ok', 'OK'], [null, 'Отмена']], check: key => (key === 'ok' ? model.result().refusal || '' : '')})
    .then(answer => (answer === 'ok' ? model.result().types : null));
}
