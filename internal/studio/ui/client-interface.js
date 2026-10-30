'use strict';

/* «Интерфейс клиентского приложения» - the command of the menu of the root
   (docs/block-00/konfiguraciya.md, «Окна», «Досъёмка 27.10.2026»,
   «Предсъёмка 28.10.2026», «Досъёмка 29.10.2026, вечер»; snapshots
   konfiguraciya/23, series-2.595/53-57, 71, 72, predsyomka-1/40-44,
   dosyomka-2646/45-62): a window of the work area, not modal, with no OK -
   the interface it changes is written by «Сохранить».

   At the left the sides Верх, Низ, Лево, Право with their groups («Группа»)
   and panels - a panel named by the panel of the platform it shows,
   «Стандартная панель» while none is chosen, nothing for a panel the
   platform's list does not have; the nameless groups the prototype wraps a
   panel of a group in are not shown. Above them «Добавить (Ins)», «Изменить
   текущий элемент (F2)», «Удалить текущий (Del)», up and down. At the right
   the five panels of the platform, a mark at each one a side shows, and
   «Изменить текущий элемент (F2)» above them.

   What a command does to the interface the server says (behavior, Go): the
   window sends the interface it holds and shows what comes back - «Добавить»
   asks «Тип элемента» first; the panel added stands on the side, or in the
   group, chosen, the group at the end of the side. «Удалить» asks nothing;
   the next row becomes current. The arrows move a row among the rows of its
   side or group and are unavailable at its ends. The palette shows the row
   current: a panel «Стандартная панель» with «…» - «Выбор стандартной
   панели», OK unavailable on a panel placed already - a side and a group
   nothing; a panel of the list «Описание панели»: its name, and «Отображение»
   of the panel of sections. The menu of a row: Добавить, Изменить, Удалить;
   up, down; «Перейти» (F12) - the panel in the list becomes current;
   «Свойства». «Добавить» on a panel is unavailable until the prototype's
   place for it is shot (question of series 2.649). A panel of the list not
   placed is dragged onto a group of the tree: it stands at the end of the
   group, current (series-2.595/71); a placed one is not dragged, and a side
   and a panel take no drop - not shot (questions of series 2.667).

   around is {changed(), palette, paletteTitle}. */
function createClientInterfaceEditor(host, around) {
  let source = null, value = null, initial = '', placed = new Set(), busy = false;
  let current = {side: 'top', path: []}, listed = null, palette = 'tree';
  const sides = [['top', 'Верх'], ['bottom', 'Низ'], ['left', 'Лево'], ['right', 'Право']];
  const cleanups = [], NotChosen = 'not-chosen', DraggedPanel = 'application/x-ml-standard-panel';
  const panelOf = id => source.panels.find(panel => panel.id === id);
  const same = (one, other) => one.side === other.side && JSON.stringify(one.path || []) === JSON.stringify(other.path || []);

  /* rows are the rows of the tree in their order: {place, kind, caption,
     depth, panel, index, count} - index and count among the rows of the
     same parent row, which the arrows keep to. */
  function rows() {
    const answer = [];
    for (const [side, caption] of sides) {
      answer.push({place: {side, path: []}, kind: 'side', caption, depth: 0});
      const children = (items, path, depth) => {
        const inner = [];
        items.forEach((item, at) => {
          const place = {side, path: [...path, at]};
          if (item.panel !== undefined) inner.push([{place, kind: 'panel', panel: item.panel, depth}]);
          else if (item.id) inner.push([{place, kind: 'group', caption: 'Группа', depth}, ...children(item.items || [], place.path, depth + 1).flat()]);
          else inner.push(...children(item.items || [], place.path, depth));
        });
        inner.forEach((block, index) => Object.assign(block[0], {index, count: inner.length}));
        return inner;
      };
      answer.push(...children(value[side] || [], [], 1).flat());
    }
    for (const row of answer) if (row.kind === 'panel') row.caption = row.panel === NotChosen ? 'Стандартная панель' : panelOf(row.panel)?.title || '';
    return answer;
  }
  const currentRow = () => rows().find(row => same(row.place, current)) || rows()[0];

  async function edit(operation, extra = {}) {
    if (busy) return null;
    busy = true;
    try {
      const response = await studioRequest('/api/project/client-interface/edit', {method: 'POST', body: {value, operation, at: current, ...extra}});
      if (!response.ok) throw new Error((await response.text()).trim());
      const edited = await response.json();
      value = edited.value;
      placed = new Set(edited.placed);
      return edited;
    } finally { busy = false; }
  }
  function changed() { draw(); around.changed(); }
  // The grey square of a panel a side shows (konfiguraciya/23).
  const placedMark = id => placed.has(id) ? Object.assign(node('span', undefined, 'client-interface-placed'), {title: 'Размещена'}) : undefined;

  async function add() {
    const row = currentRow();
    if (row.kind === 'panel') return;
    const kind = await chooseFromList({title: 'Тип элемента', items: [['add-panel', 'Стандартная панель'], ['add-group', 'Группа']], help: 'client-interface-element-type'});
    if (!kind) return;
    const edited = await edit(kind);
    if (!edited) return;
    current = edited.at;
    palette = 'tree';
    changed();
  }
  async function remove() {
    const all = rows(), at = all.findIndex(row => same(row.place, current)), row = all[at];
    if (!row || row.kind === 'side') return;
    // What goes with the row deleted is the rows below it deeper than it:
    // the row after them stands where it stood, or the row before.
    let next = at + 1;
    while (next < all.length && all[next].depth > row.depth) next++;
    if (!await edit('remove')) return;
    const left = rows();
    current = (next < all.length ? left[at] : left[at - 1]).place;
    changed();
  }
  async function shift(by) {
    const row = currentRow();
    if (row.kind === 'side' || row.index + by < 0 || row.index + by >= row.count) return;
    const edited = await edit('move', {by});
    if (!edited) return;
    current = edited.at;
    changed();
  }
  async function choosePanel() {
    const row = currentRow();
    if (row.kind !== 'panel') return;
    const items = source.panels.map(panel => [panel.id, panel.title, undefined, placedMark(panel.id)]);
    const chosen = await chooseFromList({title: 'Выбор стандартной панели', head: 'Стандартные панели', items,
      current: row.panel === NotChosen ? source.panels[0]?.id : row.panel, available: id => id === row.panel || !placed.has(id)});
    if (!chosen || chosen === row.panel) return;
    if (await edit('choose', {panel: chosen})) changed();
  }
  async function dropPanel(group, panel) {
    if (!panel || placed.has(panel)) return;
    current = group;
    const edited = await edit('drop-panel', {panel});
    if (!edited) return;
    current = edited.at;
    palette = 'tree';
    changed();
  }
  function goToList() {
    const row = currentRow();
    if (row.kind !== 'panel' || !panelOf(row.panel)) return;
    listed = row.panel;
    palette = 'list';
    draw();
    host.querySelector('.client-interface-list .object-window-list')?.focus();
  }

  // The palette of the row current or of the panel of the list current.
  function showPalette() {
    if (!around.palette) return;
    if (palette === 'list') {
      const panel = panelOf(listed);
      if (!panel) { clearObjectPalette(around, cleanups); return; }
      const definition = (value.panels || []).find(item => item.id === panel.id) || {};
      const subject = {target: {}, panel: panel.id, get: key => key === 'name' ? panel.name : definition.representation || '',
        set: async (key, next) => { if (await edit('representation', {representation: next})) changed(); }};
      showObjectPalette(around, cleanups, 'Описание панели', source.palettes.definition, subject, {changed() {}, typed() {}});
      return;
    }
    const row = currentRow();
    if (row.kind !== 'panel') { clearObjectPalette(around, cleanups); return; }
    const subject = {target: {}, get: () => row.panel, set() {}};
    showObjectPalette(around, cleanups, 'Стандартная панель', source.palettes.panel, subject, {changed() {}, typed() {},
      standardPanel: id => id === NotChosen ? 'Не выбрана' : panelOf(id)?.title || '', chooseStandardPanel: choosePanel});
  }

  function menu(event) {
    const row = currentRow(), element = row.kind !== 'side';
    showContextMenu(event, [
      ['Добавить', add, row.kind !== 'panel', 'Ins', 'add'],
      ['Изменить', () => { palette = 'tree'; showPalette(); }, true, 'F2', 'change'],
      ['Удалить', remove, element, 'Del', 'delete'],
      null,
      ['Переместить вверх', () => shift(-1), element && row.index > 0, 'Cmd+Shift+Up', 'move-up'],
      ['Переместить вниз', () => shift(1), element && row.index < row.count - 1, 'Cmd+Shift+Down', 'move-down'],
      null,
      ['Перейти', goToList, row.kind === 'panel' && !!panelOf(row.panel), 'F12'],
      null,
      ['Свойства', () => { palette = 'tree'; showPalette(); }, true, 'Option+Enter', 'properties'],
    ]);
  }

  function tree() {
    const row = currentRow(), element = row.kind !== 'side';
    const block = node('div', undefined, 'client-interface-tree');
    const panel = windowPanel([
      panelButton('add', 'Добавить (Ins)', row.kind !== 'panel', add),
      panelButton('change', 'Изменить текущий элемент (F2)', true, () => { palette = 'tree'; showPalette(); }),
      panelButton('delete', 'Удалить текущий (Del)', element, remove),
      panelButton('move-up', 'Переместить вверх', element && row.index > 0, () => shift(-1)),
      panelButton('move-down', 'Переместить вниз', element && row.index < row.count - 1, () => shift(1)),
    ]);
    const list = node('div', undefined, 'object-window-list client-interface-rows');
    list.tabIndex = 0;
    list.setAttribute('role', 'tree');
    for (const item of rows()) {
      const line = node('div', undefined, 'object-window-list-row client-interface-row client-interface-' + item.kind + (same(item.place, current) && palette === 'tree' ? ' current' : ''));
      line.style.paddingLeft = 4 + item.depth * 16 + 'px';
      line.setAttribute('role', 'treeitem');
      line.append(node('span', item.caption));
      line.addEventListener('mousedown', () => { current = item.place; palette = 'tree'; draw(); });
      line.addEventListener('contextmenu', event => { event.preventDefault(); current = item.place; palette = 'tree'; draw(); menu(event); });
      if (item.kind === 'group') {
        line.addEventListener('dragover', event => { if (event.dataTransfer.types.includes(DraggedPanel)) event.preventDefault(); });
        line.addEventListener('drop', event => { event.preventDefault(); dropPanel(item.place, event.dataTransfer.getData(DraggedPanel)); });
      }
      list.append(line);
    }
    list.addEventListener('keydown', event => {
      const all = rows(), at = all.findIndex(item => same(item.place, current));
      if (event.metaKey && event.shiftKey && (event.key === 'ArrowUp' || event.key === 'ArrowDown')) { event.preventDefault(); shift(event.key === 'ArrowUp' ? -1 : 1); }
      else if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
        event.preventDefault();
        const next = all[at + (event.key === 'ArrowUp' ? -1 : 1)];
        if (next) { current = next.place; palette = 'tree'; draw(); }
      }
      else if (event.key === 'Insert') { event.preventDefault(); add(); }
      else if (event.key === 'F2' || event.altKey && event.key === 'Enter') { event.preventDefault(); palette = 'tree'; showPalette(); }
      else if (event.key === 'Delete') { event.preventDefault(); remove(); }
      else if (event.key === 'F12') { event.preventDefault(); goToList(); }
    });
    block.append(panel, list);
    return block;
  }

  function panels() {
    const block = node('div', undefined, 'client-interface-list');
    const items = source.panels.map(panel => [panel.id, panel.title, undefined, placedMark(panel.id)]);
    const list = objectRows(items, palette === 'list' ? listed : null, id => { listed = id; palette = 'list'; draw(); });
    [...list.children].forEach((row, at) => {
      const id = source.panels[at].id;
      row.draggable = !placed.has(id);
      row.addEventListener('dragstart', event => event.dataTransfer.setData(DraggedPanel, id));
    });
    list.tabIndex = 0;
    list.addEventListener('keydown', event => {
      if (event.key === 'F2' || event.altKey && event.key === 'Enter') { event.preventDefault(); showPalette(); }
    });
    block.append(windowPanel([panelButton('change', 'Изменить текущий элемент (F2)', !!listed, () => { palette = 'list'; showPalette(); })]), list);
    return block;
  }

  function draw() {
    if (!value) return;
    const focused = ['.client-interface-rows', '.client-interface-list .object-window-list'].find(selector => host.querySelector(selector)?.contains(document.activeElement));
    host.replaceChildren(tree(), panels());
    if (focused) host.querySelector(focused)?.focus();
    showPalette();
  }

  /* open shows the interface read (the server's ClientInterfaceWindow);
     keep - after «Сохранить» - keeps the row current. */
  function open(read, keep = false) {
    source = structuredClone(read);
    value = source.value;
    placed = new Set(source.placed);
    if (!keep) { current = {side: 'top', path: []}; listed = null; palette = 'tree'; }
    initial = JSON.stringify(value);
    host.hidden = false;
    draw();
  }

  return hostedEditor(host, () => { source = null; value = null; clearObjectPalette(around, cleanups); }, {
    open,
    title() { return 'Интерфейс клиентского приложения'; },
    changed() { return !!value && JSON.stringify(value) !== initial; },
    value() { return {value, expectedRevision: source.revision}; },
  });
}
