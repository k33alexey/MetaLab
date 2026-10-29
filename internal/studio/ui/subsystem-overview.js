'use strict';

/* «Все подсистемы» - the command of the branch «Подсистемы» and of the root
   (docs/block-00/podsistema.md, «Командный интерфейс, „Объекты
   подсистемы“, „Все подсистемы“», «Предсъёмка 28.10.2026», «Досъёмка
   28.10.2026, ночь»; snapshots series-2.603/51, 52, predsyomka-1/01-30,
   dosyomka-2639/20): a window of the work area, not modal.

   At the left above, «Подсистемы»: the tree of the subsystems in the
   command interface, by their names, in the order of the project tree -
   one not in it leaves the tree with the subsystems under it (question В1
   of series 2.642) - and over it the panel: «Добавить новый элемент» - a
   subsystem at the level of the current one, last among its parent's, its
   window opened -, the common buttons of a panel of parts (partButtons):
   «Изменить» opens the window of the current one, «Удалить» after the one
   question of a deletion, up and down by the one command of the order of
   the objects of the tree, unavailable until it (the item of the map
   «порядок объектов»); «Переместить подсистему» (moveSubsystem). The menu
   of a row is the menu of the subsystem in the project tree (around.menu),
   a double click «Изменить». At the opening the first row is current, and
   «Состав» and the command interface stay empty until a row is clicked.

   At the left below, «Состав» of the subsystem clicked: the objects it
   holds by their branches (heldComposition), closed; «Редактировать состав
   объектов метаданных, входящих в подсистему» - «Выбор объекта» of the
   project with the objects held marked, open down its first branch - and OK
   makes the content what is marked; «Удалить объект метаданных из состава
   подсистемы» takes the object chosen away with no question, unavailable on
   a branch, and the tree closes after it.

   At the right, «Командный интерфейс» of the subsystem clicked - the list
   of its section with its panel and «Отбор по ролям» (sectionCommandsPane),
   read again when its content changes.

   What the content and the command interface change stays in the window
   until «Сохранить» writes every subsystem changed at once (value()); the
   adding, the deletion and the moving change the project at once, as the
   tree does, and around refuses them while something is not saved. around
   is {changed(), open(row), create(parent), remove(row), move(row),
   menu(row, event, element) - element the row of the tree}. */
function createSubsystemOverview(host, around) {
  let source = null, rows = [], current = null, clicked = false, chosen = null, pane = null, reading = 0;
  const edited = new Set(), closed = new Set(), opened = new Set();
  const rowOf = id => rows.find(row => row.subsystem.id === id);
  const parentOf = row => row.subsystem.parent || '';
  // A subsystem stands in the tree when it and every subsystem above it are
  // in the command interface.
  const shown = row => row.subsystem.includeInCommandInterface && (!row.subsystem.parent || shown(rowOf(row.subsystem.parent) || {subsystem: {}}));
  const shownRows = () => rows.filter(shown);
  const changedRow = row => { edited.add(row.subsystem.id); around.changed(); };

  const tree = node('div', undefined, 'object-window-tree subsystem-overview-tree');
  const composition = node('div', undefined, 'object-window-tree subsystem-overview-composition');
  const commands = node('div', undefined, 'subsystem-overview-commands');
  const panel = node('div'), compositionPanel = node('div');

  function drawTree() {
    const branch = row => {
      const children = shownRows().filter(other => parentOf(other) === row.subsystem.id).map(branch);
      const {element, row: line} = treeNode({icon: 'subsystem', caption: row.subsystem.name, children, open: !closed.has(row.subsystem.id),
        toggled: open => { if (open) closed.delete(row.subsystem.id); else closed.add(row.subsystem.id); }});
      line.classList.toggle('selected', row.subsystem.id === current);
      line.addEventListener('click', () => choose(row.subsystem.id));
      line.addEventListener('dblclick', () => around.open(row));
      line.addEventListener('contextmenu', event => { choose(row.subsystem.id); around.menu(row, event, line); });
      return element;
    };
    tree.replaceChildren(...shownRows().filter(row => !row.subsystem.parent).map(branch));
    const row = rowOf(current), on = !!row;
    panel.replaceChildren(windowPanel([
      panelButton('add', 'Добавить новый элемент', true, () => around.create(row ? parentOf(row) : '')),
      ...partButtons(on, {change: () => around.open(row), remove: () => around.remove(row), sort: false,
        titles: {change: 'Изменить (или открыть) текущий элемент', remove: 'Удалить текущий элемент', up: 'Переместить текущий элемент вверх',
          down: 'Переместить текущий элемент вниз'}}),
      panelButton('move-subsystem', 'Переместить подсистему', on, () => around.move(row)),
    ]));
  }

  function drawComposition() {
    const row = clicked ? rowOf(current) : null;
    const held = new Set(row?.subsystem.members || []);
    composition.replaceChildren(...(row ? heldComposition(source.composition, held, branch => opened.has(branch.title), opened, chosen,
      id => { chosen = id; drawComposition(); }) : []));
    compositionPanel.replaceChildren(windowPanel([
      panelButton('change', 'Редактировать состав объектов метаданных, входящих в подсистему', !!row, () => editComposition(row)),
      panelButton('delete', 'Удалить объект метаданных из состава подсистемы', !!row && held.has(chosen), () => {
        row.subsystem.members = row.subsystem.members.filter(id => id !== chosen);
        if (!row.subsystem.members.length) delete row.subsystem.members;
        chosen = null; opened.clear();
        changedRow(row); drawComposition(); readCommands();
      }),
    ]));
  }

  async function editComposition(row) {
    const before = row.subsystem.members || [];
    const marked = await chooseObjects('Выбор объекта', [{title: source.project, icon: 'configuration', children: source.composition}], before, {first: true});
    if (!marked) return;
    const members = [...before.filter(id => marked.includes(id)), ...marked.filter(id => !before.includes(id))];
    if (members.length === before.length && members.every((id, at) => id === before[at])) return;
    if (members.length) row.subsystem.members = members; else delete row.subsystem.members;
    if (!members.includes(chosen)) chosen = null;
    changedRow(row); drawComposition(); readCommands();
  }

  // The command interface is read anew for the content the window holds; a
  // reading overtaken by another click is dropped.
  async function readCommands() {
    const row = clicked ? rowOf(current) : null, mine = ++reading;
    if (!row) { pane = null; commands.replaceChildren(); return; }
    const answer = await postJSON('/api/subsystem/section', {subsystem: row.subsystem.id, members: row.subsystem.members || []});
    if (mine !== reading) return;
    pane = sectionCommandsPane(answer, row.subsystem.commandInterface || {}, source.defaultLanguage, () => {
      const said = pane.value();
      if (said) row.subsystem.commandInterface = said; else delete row.subsystem.commandInterface;
      changedRow(row);
    });
    commands.replaceChildren(pane.element);
  }

  function choose(id) {
    if (current === id && clicked) return;
    current = id; clicked = true; chosen = null; opened.clear();
    drawTree(); drawComposition(); readCommands();
  }

  function draw() {
    const caption = text => node('div', text, 'subsystem-overview-caption');
    const left = node('div', undefined, 'subsystem-overview-left'), right = node('div', undefined, 'subsystem-overview-right');
    left.append(caption('Подсистемы'), panel, tree, caption('Состав'), compositionPanel, composition);
    right.append(caption('Командный интерфейс'), commands);
    host.replaceChildren(left, right);
    drawTree(); drawComposition(); readCommands();
  }

  /* open shows the subsystems read (the server's SubsystemOverview); keep -
     after «Сохранить» - keeps the row clicked, else the first row is current
     and nothing is shown of it until it is clicked (snapshots
     predsyomka-1/01, 19, 23). */
  function open(value, keep = false) {
    source = structuredClone(value);
    rows = source.subsystems;
    edited.clear();
    if (!keep || !rowOf(current) || !shown(rowOf(current))) { current = shownRows()[0]?.subsystem.id ?? null; clicked = false; chosen = null; }
    host.hidden = false;
    draw();
  }

  return hostedEditor(host, () => { source = null; rows = []; current = null; clicked = false; pane = null; edited.clear(); }, {
    open,
    title() { return 'Все подсистемы'; },
    // The subsystems changed, for saving at once.
    value() { return rows.filter(row => edited.has(row.subsystem.id)).map(({path, revision, subsystem}) => ({path, revision, subsystem})); },
  });
}
