/* «Настройка списка» — the standard mechanism of the lists of Studio, one
   for every list that has the command (ML-STUDIO.md, «Настройка списка —
   стандартный механизм»; help, form_customizegriddlg; checked in the
   configurator 18.10.2026, docs/block-00/vse-ogranicheniya-dostupa.md): which
   columns a list shows and in what order, where each column stands against
   the one before it, how wide and how high it is, whether it follows the
   window, where the list opens and how it is ordered. The setting is the
   user's, kept for each list apart and between the runs of Studio
   (StudioListSettings, list_settings.go). */

/* The choices of the window, [value, caption]: the place of a column
   against the one before it (help, ПоложениеКолонки), whether its size
   changes (ИзменениеРазмераКолонки), where the list opens
   (НачальноеОтображениеСписка). */
const listColumnLocations = [['new', 'Новая колонка'], ['next-row', 'На следующей строке'], ['same-column', 'В той же колонке']];
const listColumnResizes = [['change', 'Изменять'], ['keep', 'Не изменять']];
const listOpenings = [['beginning', 'В начало списка'], ['end', 'В конец списка'], ['auto', 'Автоматически']];
/* The height of a column is from 1 to 10 lines, its width from 1 character
   with no bound above: the window of the prototype refuses 0 and 11 lines
   and 0 characters, and takes eleven nines of characters
   (snapshots configurator-2026-10-19/shabl-21…24). */
const listColumnHeightLimit = 10;

/* createListSettings is the setting of one list, apart from the page:
   definition names the list (key, the key it is kept under), its columns
   in their standard order — {key, title} and what differs from the
   standard column: width 14 characters, height 1 line, no autoheight, a new
   column, its size changing (the configurator's columns, snapshots
   vse-ogr-13, vse-ogr-15) — where it opens (opening), what «Автоматически»
   means for it (autoOpening: help, НачальноеОтображениеСписка.Авто — the end
   for the lists ordered by date, the beginning for the rest) and its
   standard order (sort: {column, descending}). A setting is
   {columns: [{key, visible, location, resize, width, height, autoHeight}],
   opening, sort}. */
function createListSettings(definition) {
  const columns = definition.columns.map(column => ({key: column.key, title: column.title, visible: true,
    location: column.location || 'new', resize: column.resize || 'change', width: column.width || 14, height: column.height || 1, autoHeight: !!column.autoHeight}));
  const byKey = new Map(columns.map(column => [column.key, column]));
  const standardSort = {column: definition.sort?.column ?? columns[0]?.key, descending: !!definition.sort?.descending};
  const among = (choices, value) => choices.some(([key]) => key === value);
  const size = (value, standard, limit = Infinity) => Number.isInteger(value) && value >= 1 && value <= limit ? value : standard;
  const own = column => ({key: column.key, visible: column.visible, location: column.location, resize: column.resize, width: column.width, height: column.height, autoHeight: column.autoHeight});
  function standardColumns() { return columns.map(own); }
  return {
    key: definition.key,
    title(key) { return byKey.get(key)?.title ?? key; },
    standard() { return {columns: standardColumns(), opening: definition.opening || 'auto', sort: {...standardSort}}; },
    // «Стандартная настройка» (help: it takes back every setting of the
    // user): the columns and where the list opens; the order of the list is
    // not the window's and stays (snapshot vse-ogr-19).
    restored(settings) { return {...this.standard(), sort: {...settings.sort}}; },
    // A kept setting as the list has it now: a column the list no longer
    // has is dropped, one it gained stands at the end as standard, and a
    // value the window could not have given is the standard one — the file
    // is the user's and may be anything.
    read(saved) {
      const result = this.standard();
      if (!saved || typeof saved !== 'object') return result;
      const seen = new Set(), kept = [];
      for (const item of Array.isArray(saved.columns) ? saved.columns : []) {
        const standard = byKey.get(item?.key);
        if (!standard || seen.has(item.key)) continue;
        seen.add(item.key);
        kept.push({key: item.key, visible: typeof item.visible === 'boolean' ? item.visible : true,
          location: among(listColumnLocations, item.location) ? item.location : standard.location,
          resize: among(listColumnResizes, item.resize) ? item.resize : standard.resize,
          width: size(item.width, standard.width), height: size(item.height, standard.height, listColumnHeightLimit),
          autoHeight: typeof item.autoHeight === 'boolean' ? item.autoHeight : standard.autoHeight});
      }
      result.columns = [...kept, ...columns.filter(column => !seen.has(column.key)).map(own)];
      if (among(listOpenings, saved.opening)) result.opening = saved.opening;
      if (byKey.has(saved.sort?.column)) result.sort = {column: saved.sort.column, descending: saved.sort.descending === true};
      return result;
    },
    // The composite columns of the shown ones (help, form_customizegriddlg,
    // ПоложениеКолонки): a new column stands to the right, one on the next
    // row starts a line under the one before it, one in the same column
    // stands on the line of the one before it. The first shown column starts
    // the first composite one whatever its place. Answers the composite
    // columns, each a list of lines, each a list of columns.
    groups(settings) {
      const groups = [];
      for (const column of settings.columns.filter(item => item.visible)) {
        const group = groups.at(-1);
        if (!group || column.location === 'new') groups.push([[column]]);
        else if (column.location === 'next-row') group.push([column]);
        else group.at(-1).push(column);
      }
      return groups;
    },
    // A click on the heading of a column orders the list by it, from the
    // least; a click on the heading it is ordered by turns the order round
    // (snapshots vse-ogr-10, vse-ogr-11).
    sortedBy(settings, key) {
      const descending = settings.sort.column === key ? !settings.sort.descending : false;
      return {...settings, sort: {column: key, descending}};
    },
    // The rows in the order of the setting, by the text valueOf(row, key)
    // shows in the column; rows with the same text keep the order they came
    // in, the list's own, in either direction (the sort is stable).
    sorted(rows, settings, valueOf) {
      const {column, descending} = settings.sort, text = row => String(valueOf(row, column) ?? '');
      return [...rows].sort((left, right) => (descending ? -1 : 1) * text(left).localeCompare(text(right)));
    },
    // The row a list of count rows opens on: the first, the last, or as
    // «Автоматически» means for this list; -1 for an empty list.
    openingRow(settings, count) {
      if (!count) return -1;
      const opening = settings.opening === 'auto' ? definition.autoOpening || 'beginning' : settings.opening;
      return opening === 'end' ? count - 1 : 0;
    },
  };
}

/* StudioListSettings holds the settings of the lists the user has set up,
   read from the server once (load) and written there on every change
   (save). A list read before they came, or without them, is standard.
   problem(text) is told what did not save. */
const StudioListSettings = (() => {
  let lists = {}, loading = null;
  return {
    problem: () => {},
    load() {
      if (!loading) {
        loading = fetch('/api/settings/lists')
          .then(async response => { if (!response.ok) throw new Error((await response.text()).trim()); return response.json(); })
          .then(answer => { lists = answer.lists || {}; })
          .catch(error => { this.problem(`Настройки списков не прочитаны: ${error.message}`); });
      }
      return loading;
    },
    get(key) { return lists[key]; },
    async save(key, settings) {
      lists[key] = settings;
      try {
        const response = await studioRequest('/api/settings/list', {method: 'PUT', body: {key, settings}});
        if (!response.ok) throw new Error((await response.text()).trim());
      } catch (error) {
        this.problem(`Настройка списка не сохранена: ${error.message}`);
      }
    },
  };
})();

/* listLayout lays a list of roleListTable out by its setting: the composite
   columns side by side, a column changing its size sharing the width of the
   window with the others by its width, one not changing it standing at its
   width in characters (help, form_customizegriddlg); the lines of a
   composite column one under another; a cell as high as its height in
   lines, or with autoheight as high as its text up to that, the text cut
   with an ellipsis on its last line (snapshot vse-ogr-12). A click on the
   heading of a column is onSort(key); the edge of the heading of a column
   that changes its size is drawn by the mouse, and the setting with its new
   width is onSettings(settings). Answers heading() — the cells of the
   heading — row(cells) — the cells of a row, cells given in the order of the
   list's columns, each [text, className] or what to put in the cell —
   apply(table), first() — the key of the first column shown. */
function listLayout(list, settings, {onSort, onSettings}) {
  let current = settings, table = null;
  const groups = () => list.groups(current);
  const columnIndex = new Map(list.standard().columns.map((column, index) => [column.key, index]));
  const fixed = group => group.every(line => line.every(column => column.resize === 'keep'));
  const width = group => Math.max(...group.map(line => line.reduce((sum, column) => sum + column.width, 0)));
  function apply(target = table) {
    table = target;
    if (!table) return;
    table.classList.add('settable-list');
    const all = groups();
    table.style.setProperty('--list-columns', all.map(group => fixed(group) ? `${width(group)}ch` : `minmax(0, ${width(group)}fr)`).join(' ') || 'minmax(0, 1fr)');
    all.forEach((group, at) => group.forEach((line, row) =>
      table.style.setProperty(`--list-line-${at}-${row}`, line.map(column => `minmax(0, ${column.width}fr)`).join(' '))));
  }
  function lines(at, group, fill) {
    const cell = node(fill.tag), block = [];
    group.forEach((line, row) => {
      const part = node('div', undefined, 'list-line');
      part.style.gridTemplateColumns = `var(--list-line-${at}-${row})`;
      for (const column of line) part.append(fill(column));
      block.push(part);
    });
    cell.append(...block);
    return cell;
  }
  function resizing(handle, column) {
    handle.addEventListener('click', event => event.stopPropagation());
    handle.addEventListener('pointerdown', event => {
      event.preventDefault(); event.stopPropagation();
      const heading = handle.parentElement, start = event.clientX, shown = heading.getBoundingClientRect().width;
      let next = column.width;
      handle.setPointerCapture?.(event.pointerId);
      const widen = width => {
        next = width;
        current = {...current, columns: current.columns.map(item => item.key === column.key ? {...item, width} : item)};
        apply();
      };
      // The edge follows the mouse: a column that changes its size shares
      // the width with the others, so its width in characters is found by
      // what it then measures, in a few steps, to the nearest character.
      const move = moved => {
        const wanted = Math.max(1, shown + moved.clientX - start);
        for (let step = 0; step < 4; step++) {
          const measured = heading.getBoundingClientRect().width, width = Math.max(1, Math.round(next * wanted / Math.max(measured, 1)));
          if (width === next) break;
          widen(width);
        }
      };
      const up = () => {
        handle.removeEventListener('pointermove', move); handle.removeEventListener('pointerup', up);
        if (next !== column.width) onSettings(current);
      };
      handle.addEventListener('pointermove', move); handle.addEventListener('pointerup', up);
    });
  }
  return {
    apply,
    first() { return groups()[0]?.[0]?.[0]?.key; },
    heading() {
      return groups().map((group, at) => {
        const fill = column => {
          const heading = node('div', list.title(column.key), 'list-heading');
          heading.dataset.column = column.key;
          if (current.sort.column === column.key) heading.dataset.sort = current.sort.descending ? 'descending' : 'ascending';
          heading.addEventListener('click', () => onSort(column.key));
          if (column.resize === 'change') {
            const handle = node('span', undefined, 'list-resize');
            handle.setAttribute('aria-hidden', 'true');
            resizing(handle, column);
            heading.append(handle);
          }
          return heading;
        };
        fill.tag = 'th';
        return lines(at, group, fill);
      });
    },
    row(cells) {
      return groups().map((group, at) => {
        const fill = column => {
          const value = cells[columnIndex.get(column.key)], cell = node('div', undefined, `list-cell ${column.autoHeight ? 'list-auto' : 'list-fixed'}`);
          cell.dataset.column = column.key;
          cell.style.setProperty('--lines', column.height);
          if (value instanceof Node) cell.append(value);
          else {
            const [text, className] = value;
            cell.textContent = text;
            cell.title = text;
            if (className) cell.classList.add(className);
          }
          return cell;
        };
        fill.tag = 'td';
        return lines(at, group, fill);
      });
    },
  };
}

/* chooseListSettings is the window «Настройка списка» over the setting of
   list (snapshots vse-ogr-12…16, shabl-03): «Колонки» — the columns with
   the box of their showing, moved up and down, «Стандартная настройка»,
   every box marked or none, and for the column chosen its place, whether
   its size changes, its width, height and autoheight; «Прочее» — where the
   list opens. «Применить» gives the setting to applied(settings) and keeps
   the window open; it is available while the window holds what is not yet
   applied. Answers the setting OK closed the window with, or null. */
async function chooseListSettings(list, settings, applied) {
  let draft = structuredClone(settings), given = structuredClone(settings), chosen = 0;
  const same = (left, right) => JSON.stringify(left) === JSON.stringify(right);
  const columnsPage = node('div', undefined, 'list-settings-page studio-tab-page'), otherPage = node('div', undefined, 'list-settings-page studio-tab-page');
  const pages = {columns: columnsPage, other: otherPage};
  const show = key => { for (const [name, page] of Object.entries(pages)) page.hidden = name !== key; };
  const tabs = tabStrip([['columns', 'Колонки'], ['other', 'Прочее']], 'columns', show);
  const bar = node('div', undefined, 'list-settings-bar');
  const up = iconButton('move-up', 'Переместить вверх'), down = iconButton('move-down', 'Переместить вниз'), standard = node('button', 'Стандартная настройка');
  standard.type = 'button';
  const move = by => {
    const to = chosen + by;
    if (to < 0 || to >= draft.columns.length) return;
    const [column] = draft.columns.splice(chosen, 1);
    draft.columns.splice(to, 0, column);
    chosen = to; fill();
  };
  up.addEventListener('click', () => move(-1));
  down.addEventListener('click', () => move(1));
  standard.addEventListener('click', () => { draft = list.restored(draft); chosen = 0; fill(); });
  bar.append(up, down, standard);
  const columnList = node('div', undefined, 'list-settings-columns'), side = node('div', undefined, 'list-settings-side');
  const marks = node('div', undefined, 'list-settings-marks');
  marks.append(...markAllButtons(visible => { for (const column of draft.columns) column.visible = visible; fill(); }));
  const group = node('fieldset', undefined, 'list-settings-column');
  side.append(marks, group);
  const body = node('div', undefined, 'list-settings-body');
  body.append(columnList, side);
  columnsPage.append(bar, body);
  const opening = selectInput(listOpenings, draft.opening, value => { draft.opening = value; }, 'При открытии переходить');
  otherPage.append(labeledField('При открытии переходить:', opening, 'list-settings-field list-settings-inline'));
  // The list of columns is drawn anew only when it changes as a whole;
  // choosing a column marks it in place, so that the box clicked keeps
  // its change.
  function fill() {
    columnList.replaceChildren(...draft.columns.map((column, index) => {
      const line = checkboxField(list.title(column.key), column.visible, visible => { column.visible = visible; }, 'list-settings-item');
      // A click on the caption chooses the column; only the box shows or
      // hides it.
      line.addEventListener('click', event => {
        if (event.target.tagName !== 'INPUT') event.preventDefault();
        if (chosen !== index) choose(index);
      });
      return line;
    }));
    choose(chosen);
  }
  function choose(index) {
    chosen = index;
    [...columnList.children].forEach((line, at) => line.classList.toggle('selected', at === chosen));
    up.disabled = chosen <= 0; down.disabled = chosen >= draft.columns.length - 1;
    const column = draft.columns[chosen];
    group.replaceChildren(node('legend', 'Колонка'));
    if (column) {
      const number = (value, apply, label, limit = Infinity) => {
        // What is not a whole number from 1 up to the limit is not taken:
        // the field shows again the last value taken.
        let taken = value;
        const input = textInput(value, entered => {
          if (Number.isInteger(entered) && entered >= 1 && entered <= limit) { taken = entered; apply(entered); } else input.value = taken;
        }, {type: 'number', min: 1, ...(limit < Infinity ? {max: limit} : {})});
        input.setAttribute('aria-label', label);
        return input;
      };
      group.append(
        labeledField('Положение:', selectInput(listColumnLocations, column.location, value => { column.location = value; }, 'Положение'), 'list-settings-field'),
        labeledField('Изменение размера:', selectInput(listColumnResizes, column.resize, value => { column.resize = value; }, 'Изменение размера'), 'list-settings-field'),
        labeledField('Ширина:', number(column.width, value => { column.width = value; }, 'Ширина'), 'list-settings-field'),
        labeledField('Высота:', number(column.height, value => { column.height = value; }, 'Высота', listColumnHeightLimit), 'list-settings-field'),
        checkboxField('Автовысота', column.autoHeight, enabled => { column.autoHeight = enabled; }, 'list-settings-check'));
    }
    opening.value = draft.opening;
  }
  fill();
  show('columns');
  const content = node('div', undefined, 'list-settings');
  content.append(tabs, columnsPage, otherPage);
  const answer = await showDialog({title: 'Настройка списка', content, className: 'list-settings-dialog', help: 'list-settings',
    answers: [['ok', 'ОК'], [null, 'Отмена'], ['apply', 'Применить', {act: () => { given = structuredClone(draft); applied(structuredClone(draft)); }, enabled: () => !same(draft, given)}]]});
  // «Отмена» takes back what «Применить» gave as well: the list returns to
  // the setting the window opened with (snapshot
  // configurator-2026-10-19/shabl-26).
  if (answer !== 'ok' && !same(given, settings)) applied(structuredClone(settings));
  return answer === 'ok' ? draft : null;
}
