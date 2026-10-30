'use strict';

/* «Рабочая область начальной страницы» - the command of the menu of the
   root (docs/block-00/konfiguraciya.md, «Окна», «Досъёмка 27.10.2026»,
   «Предсъёмка 28.10.2026»; snapshots konfiguraciya/18-20,
   series-2.595/41-49, predsyomka-1/33): a window of the work area, not
   modal, with no OK - the page it changes is written by «Сохранить».

   Two columns, «Левая колонка:» and «Правая колонка:», each with its panel
   - «Добавить новый элемент», «Добавить новый элемент копированием
   текущего», «Удалить текущий элемент», up and down - and its list «Форма»,
   «Высота», «Видимость»; «>» and «<» between them; below «Отображение
   командного интерфейса основного раздела» and «Шаблон начальной
   страницы».

   «Добавить» puts at the end of the column a row of no form, of the height
   10, visible to all, and opens «Выбор управляемой формы» (chooseOneObject)
   - «Отмена» takes the row away. «Скопировать» puts a copy of the current
   row at the end and opens the choice on its form; one form may stand
   twice. After OK the height of the row takes the focus. «Удалить» asks
   nothing; the row before becomes current. The arrows move the current row,
   nothing at the ends (moveWithin). The height is a whole number of six
   digits at most, 0 shown empty. The box «Видимость» is the common
   visibility; the link beside it - «Одинаковая», or «Различная» where a role
   sees the form otherwise (visibilityDiffers) - opens «Настройка видимости»
   (showVisibilitySettings). «>» moves the current row of the left to the end
   of the right, the next row of the left becoming current, and «<» back.
   «Одна колонка» hides the right column, the caption of the left and the
   moves; the forms of the right stay in the window and come back with two
   columns - a page of one column cannot keep them, and «Сохранить» writes
   the left only (question В3 of series 2.646). The menu of a row: Добавить
   (Ins), Скопировать (F9), Удалить (Del), up, down (Cmd+Shift+Up, Down) -
   the keys act on the list too.

   around is {changed()}, told of every change. */
function createHomePageEditor(host, around) {
  let source = null, page = null, initial = '';
  const current = {left: 0, right: 0};
  const columns = [['left', 'Левая колонка:'], ['right', 'Правая колонка:']];
  const templates = [['one-column', 'Одна колонка'], ['two-columns-equal-width', 'Две колонки одинаковой ширины'],
    ['two-columns-variable-width', 'Две колонки разной ширины (2:1)']];
  const places = [['top', 'Сверху'], ['bottom', 'Снизу'], ['', 'Не использовать']];
  const oneColumn = () => page.template === 'one-column';
  const formName = id => source.names[id] || id || '';

  function changed() { draw(); around.changed(); }

  // «Отмена» takes the row added away and makes current again the row that
  // was before the command (konfiguraciya.md, «Досъёмка 29.10.2026,
  // вечер»).
  async function chooseForm(side, at, before) {
    const row = page[side][at];
    const form = await chooseOneObject('Выбор управляемой формы', source.forms, row.form);
    if (!form) { page[side].splice(at, 1); current[side] = Math.max(0, Math.min(before, page[side].length - 1)); changed(); return; }
    row.form = form;
    changed();
    host.querySelector(`.home-page-${side} .home-page-row.current .home-page-height`)?.focus();
  }
  function add(side, copy) {
    const list = page[side], copied = list[current[side]], before = current[side];
    if (copy && !copied) return;
    list.push(copy ? structuredClone(copied) : {form: '', height: 10, visibility: {common: true}});
    current[side] = list.length - 1;
    draw();
    chooseForm(side, list.length - 1, before);
  }
  function remove(side) {
    const list = page[side];
    if (!list.length) return;
    list.splice(current[side], 1);
    current[side] = Math.max(0, current[side] - 1);
    changed();
  }
  function shift(side, by) {
    if (!moveWithin(page[side], current[side], by)) return;
    current[side] += by;
    changed();
  }
  function carry(from, to) {
    const list = page[from];
    if (!list.length) return;
    page[to].push(...list.splice(current[from], 1));
    current[from] = Math.min(current[from], Math.max(0, list.length - 1));
    changed();
  }

  function menu(side, event) {
    const on = page[side].length > 0;
    showContextMenu(event, [
      ['Добавить', () => add(side, false), true, 'Ins', 'add'],
      ['Скопировать', () => add(side, true), on, 'F9', 'clone'],
      ['Удалить', () => remove(side), on, 'Del', 'delete'],
      ['Переместить вверх', () => shift(side, -1), on, 'Cmd+Shift+Up', 'move-up'],
      ['Переместить вниз', () => shift(side, 1), on, 'Cmd+Shift+Down', 'move-down'],
    ]);
  }

  function heightCell(row) {
    const input = node('input', undefined, 'home-page-height');
    input.inputMode = 'numeric';
    input.maxLength = 6;
    input.value = row.height ? String(row.height) : '';
    input.setAttribute('aria-label', 'Высота');
    input.addEventListener('input', () => {
      input.value = input.value.replace(/\D/g, '').slice(0, 6);
      const height = Number(input.value || 0);
      if (height) row.height = height; else delete row.height;
      around.changed();
    });
    input.addEventListener('change', () => { input.value = row.height ? String(row.height) : ''; });
    return input;
  }

  function column(side, caption) {
    const list = page[side], on = list.length > 0;
    const block = node('div', undefined, 'home-page-column home-page-' + side);
    const panel = windowPanel([
      panelButton('add', 'Добавить новый элемент', true, () => add(side, false)),
      panelButton('clone', 'Добавить новый элемент копированием текущего', on, () => add(side, true)),
      panelButton('delete', 'Удалить текущий элемент', on, () => remove(side)),
      panelButton('move-up', 'Переместить текущий элемент вверх', on, () => shift(side, -1)),
      panelButton('move-down', 'Переместить текущий элемент вниз', on, () => shift(side, 1)),
    ]);
    const table = node('div', undefined, 'home-page-list');
    table.tabIndex = 0;
    table.append(node('div', undefined, 'home-page-head'));
    table.lastChild.append(node('span', 'Форма'), node('span', 'Высота'), node('span', 'Видимость'));
    list.forEach((row, at) => {
      const line = node('div', undefined, 'home-page-row' + (at === current[side] ? ' current' : ''));
      const name = node('span', formName(row.form), 'home-page-form');
      name.title = formName(row.form);
      const box = node('input');
      box.type = 'checkbox';
      box.checked = row.visibility.common;
      box.setAttribute('aria-label', 'Видимость: ' + formName(row.form));
      box.addEventListener('change', () => { row.visibility.common = box.checked; current[side] = at; changed(); });
      const link = node('a', visibilityDiffers(row.visibility) ? 'Различная' : 'Одинаковая', 'home-page-roles');
      link.href = '#';
      link.addEventListener('click', async event => {
        event.preventDefault();
        current[side] = at;
        const visibility = await showVisibilitySettings(row.visibility, source.roles);
        if (visibility) { row.visibility = visibility; changed(); } else draw();
      });
      const visible = node('span', undefined, 'home-page-visibility');
      visible.append(box, link);
      line.append(name, heightCell(row), visible);
      line.addEventListener('mousedown', () => { if (current[side] !== at) { current[side] = at; for (const other of table.querySelectorAll('.home-page-row')) other.classList.toggle('current', other === line); } });
      line.addEventListener('contextmenu', event => { current[side] = at; draw(); menu(side, event); });
      table.append(line);
    });
    table.addEventListener('contextmenu', event => { if (event.target === table) menu(side, event); });
    table.addEventListener('keydown', event => {
      if (event.target.tagName === 'INPUT' && event.key !== 'F9') return;
      if (event.metaKey && event.shiftKey && (event.key === 'ArrowUp' || event.key === 'ArrowDown')) { event.preventDefault(); shift(side, event.key === 'ArrowUp' ? -1 : 1); }
      else if (event.key === 'Insert') { event.preventDefault(); add(side, false); }
      else if (event.key === 'F9') { event.preventDefault(); add(side, true); }
      else if (event.key === 'Delete') { event.preventDefault(); remove(side); }
    });
    block.append(...(oneColumn() ? [] : [node('div', caption, 'home-page-caption')]), panel, table);
    return block;
  }

  function draw() {
    const body = node('div', undefined, 'home-page-columns' + (oneColumn() ? ' one-column' : ''));
    body.append(column('left', columns[0][1]));
    if (!oneColumn()) {
      const moves = transferButtons('home-page-moves', ['Перенести в правую колонку', () => carry('left', 'right'), page.left.length > 0],
        ['Перенести в левую колонку', () => carry('right', 'left'), page.right.length > 0]);
      body.append(moves, column('right', columns[1][1]));
    }
    const settings = node('div', undefined, 'home-page-settings');
    settings.append(
      labeledField('Отображение командного интерфейса основного раздела:', selectInput(places, page.mainSectionCommands === 'do-not-use' ? '' : page.mainSectionCommands || '',
        value => { if (value) page.mainSectionCommands = value; else delete page.mainSectionCommands; changed(); }), 'home-page-field'),
      labeledField('Шаблон начальной страницы:', selectInput(templates, page.template, value => { page.template = value; changed(); }), 'home-page-field'));
    host.replaceChildren(body, settings);
  }

  // value is the page the window says: the right column of a page of one
  // column left out, as the model keeps none.
  function value() {
    const said = {template: page.template, left: page.left, right: oneColumn() ? [] : page.right,
      ...(page.mainSectionCommands ? {mainSectionCommands: page.mainSectionCommands} : {})};
    if (!said.left.length) delete said.left;
    if (!said.right.length) delete said.right;
    return said;
  }

  /* open shows the page read (the server's HomePageWindow); keep - after
     «Сохранить» - keeps the rows current. */
  function open(read, keep = false) {
    source = structuredClone(read);
    page = source.page;
    page.left = page.left || [];
    page.right = page.right || [];
    for (const row of [...page.left, ...page.right]) row.visibility = row.visibility || {common: false};
    if (!keep) { current.left = 0; current.right = 0; }
    initial = JSON.stringify(value());
    host.hidden = false;
    draw();
  }

  return hostedEditor(host, () => { source = null; page = null; }, {
    open,
    title() { return 'Рабочая область начальной страницы'; },
    changed() { return !!page && JSON.stringify(value()) !== initial; },
    value() { return {value: value(), expectedRevision: source.revision}; },
  });
}
