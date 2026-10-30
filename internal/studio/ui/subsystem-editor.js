'use strict';

/* The window of a subsystem, as the prototype's (docs/block-00/
   podsistema.md, «Окно»; snapshots podsistema/04-12): the window of an
   object of rows (objectWindowEditor) - «Основные» with the flags, the
   button «Командный интерфейс», the explanation and the picture,
   «Функциональные опции» - the options of the project marked where the
   subsystem is in their content - «Состав» and «Прочее» with the help -
   and its palette of three tabs, whose «Состав: Открыть» shows the page
   «Состав». «Командный интерфейс» - the button of «Основные» and the link
   of the palette - opens its window (showSubsystemCommandInterface); the
   picture is chosen by «Выбор картинки» (choosePicture). */
function createSubsystemEditor(host, onChange, around = {}) {
  return objectWindowEditor(host, onChange, around, {target: source => source.subsystem, own: api => {
    const opened = new Set();
    let chosen = null;
    return {pages: {composition: () => subsystemComposition(api, opened, () => chosen, id => { chosen = id; })},
      context: () => ({windows: ['members', 'commandInterface'],
        openWindow: key => key === 'commandInterface' ? showSubsystemCommandInterface(api) : api.show('composition')}), reset() { chosen = null; },
      // «Открыть командный интерфейс» of the menu of the tree, once the
      // window of the subsystem is open.
      methods: {openCommandInterface: () => showSubsystemCommandInterface(api)}};
  }});
}

/* subsystemComposition is the page «Состав» of the window of a subsystem
   (podsistema.md, «Окно»; snapshots podsistema/06, 08-12): above, the
   objects of the kinds the page offers (source.composition) by their
   branches - «Общие» first, numerators inside «Документы» - a box at each
   object, marked where the subsystem holds it; below, «Входящие в
   подсистему объекты:» - the objects held, by their branches - and ✕
   taking the one chosen away. An object marked anew is held last. A
   branch holding an object held stands open, and so does one opened by
   hand (opened, by title). */
function subsystemComposition(api, opened, chosen, choose) {
  const object = api.object(), held = new Set(object.members || []);
  const changed = () => {
    const before = object.members || [];
    object.members = [...before.filter(id => held.has(id)), ...[...held].filter(id => !before.includes(id))];
    if (!object.members.length) delete object.members;
    api.changed();
  };
  const holds = branch => (branch.objects || []).some(item => held.has(item.id)) || (branch.children || []).some(holds);
  const open = branch => opened.has(branch.title) || holds(branch);
  const offered = node('div', undefined, 'object-window-tree object-window-tree-tall subsystem-composition');
  offered.append(...compositionBranches(api.source().composition, true, open, opened,
    item => treeNode({icon: item.icon, caption: item.name, mark: markBox(item.name, held, item.id, changed)}).element));
  const below = node('div', undefined, 'object-window-tree subsystem-composition-held');
  below.append(...heldComposition(api.source().composition, held, open, opened, chosen(), id => { choose(id); api.refresh(); }));
  const remove = clearButton('Удалить из состава', () => { held.delete(chosen()); choose(null); changed(); });
  remove.disabled = !held.has(chosen());
  const caption = node('div', undefined, 'object-window-caption subsystem-composition-caption');
  caption.append(node('span', 'Входящие в подсистему объекты:'), remove);
  const page = node('div', undefined, 'object-window-fields');
  page.append(offered, caption, below);
  return page;
}

/* compositionBranches are the nodes of the branches of a tree of objects of
   «Выбор объекта» (objectChoiceTree): a branch stands whether it holds
   objects or not where every is set, as the branches of the project tree
   do, else only where it holds one; open(branch) says it stands open, and
   opened - the titles of the branches opened by hand - follows the hand.
   row(item) is the node of an object. */
function compositionBranches(branches, every, open, opened, row) {
  const branchNode = branch => {
    const children = [...(branch.children || []).flatMap(branchNode), ...(branch.objects || []).map(row)];
    if (!children.length && !every) return [];
    return [treeNode({icon: branch.icon, caption: branch.title, children, open: open(branch),
      toggled: state => { if (state) opened.add(branch.title); else opened.delete(branch.title); }}).element];
  };
  return branches.flatMap(branchNode);
}

/* heldComposition are the nodes of the objects a subsystem holds (held), by
   the branches of composition they are of - «Входящие в подсистему
   объекты:» of its window and «Состав» of «Все подсистемы» - the one
   chosen selected; a click on an object chooses it (choose(id)). */
function heldComposition(composition, held, open, opened, chosen, choose) {
  const heldBranches = branches => branches.map(branch => ({...branch, objects: (branch.objects || []).filter(item => held.has(item.id)),
    children: heldBranches(branch.children || [])}));
  return compositionBranches(heldBranches(composition), false, open, opened, item => {
    const {element, row} = treeNode({icon: item.icon, caption: item.name});
    row.classList.toggle('selected', item.id === chosen);
    row.addEventListener('click', () => choose(item.id));
    return element;
  });
}

/* subsystemCommands are the commands of the menu of a subsystem of its own
   (podsistema.md, «Команды»; snapshot podsistema/15): «Открыть командный
   интерфейс» - the window of the subsystem and its «Командный интерфейс»
   (actions.openInterface) - and «Справочная информация», unavailable until
   its window;
   «Объекты подсистемы» - the tree filtered by this subsystem alone, the
   objects of the subsystems beneath and above it not taken in
   (podsistema.md, «Командный интерфейс, „Объекты подсистемы“, „Все
   подсистемы“»; snapshots series-2.603/49, 50); «Добавить ▸» - «Подчиненная
   подсистема», «Подсистема»; «Переместить подсистему». node is the
   subsystem in the tree; actions are {openInterface(), create(make) - makes a subsystem by
   make() and opens it, moved() - draws the tree anew, filter(chosen) -
   puts the tree's filter by subsystems in force}. */
function subsystemCommands(node, actions) {
  return [['Открыть командный интерфейс', () => actions.openInterface()],
    ['Объекты подсистемы', () => actions.filter({chosen: new Set([node.id]), children: false, parents: false})], null,
    ['Добавить', [['Подчиненная подсистема', () => actions.create(() => postJSON('/api/subsystem/child', {parent: node.id}))],
      ['Подсистема', () => actions.create(() => postJSON('/api/subsystem', {}))]]], null,
    ['Переместить подсистему', async () => { if (await moveSubsystem(node.path, actions.projectName())) actions.moved(); }, true, '', 'move-subsystem'], null,
    ['Справочная информация', () => {}, false]];
}

/* moveSubsystem is «Переместить подсистему» (snapshot podsistema/16): the
   window «Выбор объекта» - the root of the project and the subsystems
   beneath it, by their parents - OK and «Отмена»; OK puts the subsystem
   described at path under the one chosen, or the root. A refusal - a name
   taken there, a subsystem of its own - is shown and moves nothing.
   Answers whether it moved. */
async function moveSubsystem(path, projectName) {
  const source = await (await fetch('/api/subsystem?path=' + encodeURIComponent(path))).json();
  const own = source.subsystem.id;
  let chosen = source.subsystem.parent || '';
  const content = node('div', undefined, 'object-window-tree object-choice-tree subsystem-move');
  const draw = () => {
    const under = parent => (source.subsystems || []).filter(place => (place.parent || '') === parent && place.id !== own);
    const branch = place => {
      const {element, row} = treeNode({icon: 'subsystem', caption: place.name, children: under(place.id).map(branch), open: true});
      row.classList.toggle('selected', place.id === chosen);
      row.addEventListener('click', () => { chosen = place.id; draw(); });
      return element;
    };
    const {element, row} = treeNode({icon: 'configuration', caption: projectName, children: under('').map(branch), open: true});
    row.classList.toggle('selected', chosen === '');
    row.addEventListener('click', () => { chosen = ''; draw(); });
    content.replaceChildren(element);
  };
  draw();
  if (await showDialog({title: 'Выбор объекта', content, className: 'subsystem-move-dialog', side: true, answers: [['ok', 'OK'], [null, 'Отмена']]}) !== 'ok') return false;
  try {
    await postJSON('/api/subsystem/move', {path, revision: source.revision, ...(chosen ? {parent: chosen} : {})});
    return true;
  } catch (error) {
    await askChoice(String(error.message || error).trim(), '', [['ok', 'ОК']]);
    return false;
  }
}

/* roleFilters are the values «Отбор по ролям» of the windows of a command
   interface had before, the last first - its list offers them again. */
const roleFilters = [];

/* showSubsystemCommandInterface is the window «Командный интерфейс» of a
   subsystem (podsistema.md, «Окно», «Командный интерфейс…», «Досъёмка
   28.10.2026», «Досъёмка 28.10.2026, поздний вечер»; snapshots podsistema/13,
   14, series-2.603/42, 43, vopr-2.636/07-28). Over the tree: «Переместить
   команду в другую группу команд», up, down, «Установить свойства по
   умолчанию», «Скрыть команды, невидимые по умолчанию» - a command whose box
   is off leaves the tree while it is pressed (konfiguraciya.md, В14 of series
   2.595) - and «Отбор по ролям» (below). The tree
   «Команда — Видимость»: the groups of the section in the prototype's order
   (server, sectionGroups), each command of the content of the window - saved
   or not - in the group its placement says, or else its own, with the box of
   its visibility: the command interface's, or else the default of its kind
   (metadata.SectionLines); under «Подразделы» the subsystems under it in the
   command interface (server, subsections).

   The order of a group is automatic - its own commands by the alphabet of
   their presentation, then those moved into it in the order they came - or
   manual, «(ручной порядок)», as its CommandsOrder lists them, the commands
   come later at its end; «Подразделы» alike by the alphabet or by
   SubsystemsOrder. The row clicked is current. On a command the four
   buttons act: the move offers the groups of the panel the command stands in
   by default (chooseCommandGroup) and puts it at the end of the group chosen;
   up and down swap it with its neighbour - not round the ends - and make its
   group manual; the default brings back its group, its place and its
   visibility, common and by roles. On a subsection up and down order
   «Подразделы» and the default is offered and does nothing known (question
   В2 of series 2.639). On a group or «Подразделы» the buttons are
   unavailable and the menu of the row offers «Восстановить автоматический
   порядок». The menu of a row is the buttons' and that; the keys
   Cmd+Shift+Up and Cmd+Shift+Down move.

   «Отбор по ролям» (obshchee.md, «Видимость по ролям»; snapshots
   series-2.603/44-47, vopr-2.636/02-06): a field of the roles chosen by
   their names, «<Не установлен>» while none, its list the values it had
   before (roleFilters), not cleared from the keyboard; «…» opens «Выбор
   объекта: Роль» - the project and its roles with boxes, the search
   (chooseObjects) - and OK without a role takes the filter away. The
   filter only narrows: a command stays that one of the roles opens
   (server, metadata.RoleDefinition.OpensSectionLine), a group without one
   goes, a subsection stays that they open something of, and «Подразделы»
   goes without one; the boxes stay the common visibility.

   The columns of the roles (obshchee.md, «Список команд», «Видимость по
   ролям»; snapshots vopr-2.636/01, 17, predsyomka-1/02, 03): under
   «Видимость по ролям» a column a role, by its synonym, in the order of
   the project - every role, or the roles of the filter; «Команда» stays
   where it is while they scroll under it. A command has a box in each:
   grey and marked while the role says nothing - the default, even with
   the common box off - and a click goes round, as the box of «Настройка
   видимости» does: off, on, the default again. Only the columns in sight
   are drawn - a project has a thousand roles. OK writes what the roles
   say into visibility.roles, in the order they were said.

   OK writes into the subsystem what the window changed: the visibility that
   differs from the default, and of a command standing out of its group
   always (the prototype's); the placement of a command moved or brought
   back; the whole order of a group ordered or brought back to automatic,
   GroupsOrder - the groups with an order - with it; the order of
   «Подразделы». What it did not change is written as it was; «Отмена»
   changes nothing. */
async function showSubsystemCommandInterface(api) {
  const object = api.object();
  const answer = await postJSON('/api/subsystem/section', {subsystem: object.id, members: object.members || []});
  const pane = sectionCommandsPane(answer, object.commandInterface || {}, api.source().defaultLanguage);
  if (!await commandInterfaceWindow(pane, 'subsystem-command-interface')) return false;
  const commandInterface = pane.value();
  if (commandInterface) object.commandInterface = commandInterface; else delete object.commandInterface;
  api.changed();
  return true;
}

/* commandInterfaceWindow is the modal window «Командный интерфейс» of a
   subsystem and of the root around the list pane (sectionCommandsPane): OK,
   «Отмена», «Справка» - the page help. Answers whether OK was pressed. */
async function commandInterfaceWindow(pane, help) {
  return await showDialog({title: 'Командный интерфейс', content: pane.element, className: 'section-commands-dialog', help,
    answers: [['ok', 'OK'], [null, 'Отмена']]}) === 'ok';
}

/* sectionActions is what follows the title of a standard command of a
   section by its command (metadata.SectionLine): «: создать» after the
   presentation of an object, «: создать группу» after the presentation of
   the list (podsistema.md, «Досъёмка 28.10.2026, поздний вечер»). */
const sectionActions = {create: ': создать', 'create-folder': ': создать группу'};

/* sectionCommandsPane is the list of the commands of the section of a
   subsystem with its panel - the body of the window «Командный интерфейс»
   (showSubsystemCommandInterface) and the right of «Все подсистемы»
   (createSubsystemOverview): answer is the server's (/api/subsystem/section),
   said the command interface the subsystem says now, language the one its
   titles are shown in, and changed() is called after every edit. The
   root's window has no move into another group (moves) and words its
   hiding otherwise (hideTitle) - snapshot series-2.595/36. The main section
   is its placed commands (content): add(line, key, first) puts a command in
   at the start or the end of a group, visible as the line says, current -
   one already in is made current - and remove() takes the current one out,
   the next becoming current, and «Удалить команду из основного раздела»
   (Del) of the menu does it (konfiguraciya.md, «Досъёмка 27.10.2026»,
   «Предсъёмка 28.10.2026»; snapshots series-2.595/69, 70,
   predsyomka-1/37); a command added stands where it was put until the
   window is opened again, the group staying automatic; every command is
   written with its placement and its visibility, as the configurations
   being moved write them. list names the setting of the list («Настройка
   списка...» of the menu of a row, commandListSettings) - its key and the
   width of «Команда». Answers {element, value(), add, remove, at()}:
   value is the command interface the list says - what OK of the window
   writes - or null where it says nothing; at() is the group the row
   current is or stands in, {key, panel}, or null. */
function sectionCommandsPane(answer, said, language, changed = () => {}, {moves = true, hideTitle = 'Скрыть команды, невидимые по умолчанию', content = false,
  list = {key: 'subsystem-commands'}} = {}) {
  const groups = answer.groups, subsections = answer.subsections || [], projectRoles = answer.roles || [];
  const listSettings = commandListSettings(list.key, projectRoles, list.width);
  let setting = listSettings.standard();
  const shownColumn = key => setting.columns.some(column => column.key === key && column.visible);
  const identity = ['command', 'object', 'standard', 'written'];
  const same = (one, other) => identity.every(key => (one[key] || '') === (other[key] || ''));
  const commandOf = line => Object.fromEntries(identity.filter(key => line[key]).map(key => [key, line[key]]));
  const groupKey = group => [group.standard, group.group, group.written].map(part => part || '').join('|');
  const visibilities = said.commandsVisibility || [];
  const lines = groups.flatMap(group => group.commands || []);
  const byKey = new Map(groups.map(group => [groupKey(group.group), group]));
  const home = new Map(lines.map(line => [line, groupKey(line.group)]));
  const lineOf = item => lines.find(line => same(line, item));
  const place = new Map(lines.map(line => [line, home.get(line)]));
  const arrivals = [];
  for (const item of said.commandsPlacement || []) {
    const line = lineOf(item), key = groupKey(item.group);
    if (line && byKey.has(key)) { place.set(line, key); arrivals.push(line); }
  }
  const visible = new Map(lines.map(line => [line, visibilities.find(item => same(item, line))?.visibility.common ?? line.visible]));
  const roles = new Map(lines.map(line => [line, visibilities.find(item => same(item, line))?.visibility.roles]));
  const orders = new Map();
  for (const item of said.commandsOrder || []) {
    const line = lineOf(item), key = groupKey(item.group);
    if (!line || !byKey.has(key)) continue;
    if (!orders.has(key)) orders.set(key, []);
    orders.get(key).push(line);
  }
  let subsectionOrder = said.subsystemsOrder?.length ? said.subsystemsOrder.map(id => subsections.find(item => item.id === id)).filter(Boolean) : null;
  const placedLines = new Set(), orderedGroups = new Set();
  // The commands added to the main section, at the start of their group
  // (first) or at its end, and those taken out of it.
  const added = new Set(), first = [], removedLines = [];
  const removedHere = item => removedLines.some(line => same(line, item));
  let subsectionsTouched = false;
  // A section of the root comes by its presentation already (server,
  // SectionSubsystem), a command by its title in every language.
  const caption = line => (typeof line.title === 'string' ? line.title : localizedTitle({title: line.title}, language)) + (sectionActions[line.standard] || '');
  const alphabet = (one, other) => caption(one).localeCompare(caption(other), 'ru');
  const commandsOf = key => {
    const members = lines.filter(line => place.get(line) === key);
    const manual = (orders.get(key) || []).filter(line => members.includes(line));
    const rest = members.filter(line => !manual.includes(line));
    const come = rest.filter(line => home.get(line) !== key || added.has(line)).sort((one, other) => arrivals.indexOf(one) - arrivals.indexOf(other));
    const leading = first.filter(line => come.includes(line));
    return [...manual, ...leading, ...rest.filter(line => home.get(line) === key && !added.has(line)).sort(alphabet),
      ...come.filter(line => !leading.includes(line))];
  };
  const subsectionsShown = () => {
    const manual = subsectionOrder || [];
    return [...manual, ...subsections.filter(item => !manual.includes(item)).sort((one, other) => one.title.localeCompare(other.title, 'ru'))];
  };
  let hidden = false, current = null, filtering = [];
  const opened = item => !filtering.length || (item.opens || []).some(role => filtering.includes(role));
  const isCurrent = which => !!current && Object.keys(which).every(key => current[key] === which[key]);
  const tree = node('div', undefined, 'object-window-tree section-commands');
  tree.tabIndex = 0;
  const roleWidth = 120, rolesInSight = 3;
  // The roles in the order and with the showing the setting of the list
  // gives them, narrowed by the filter.
  const columns = () => setting.columns.filter(column => column.visible && column.key.startsWith('role:'))
    .map(column => projectRoles.find(role => 'role:' + role.id === column.key)).filter(role => role && (!filtering.length || filtering.includes(role.id)));
  let strips = [], offset = 0;
  const strip = fill => {
    const element = node('div', undefined, 'section-roles-strip');
    strips.push({element, fill});
    return element;
  };
  const paint = () => {
    const shown = columns(), sight = Math.min(shown.length, rolesInSight) * roleWidth;
    offset = Math.max(0, Math.min(offset, shown.length * roleWidth - sight));
    const from = Math.floor(offset / roleWidth), to = Math.min(shown.length, Math.ceil((offset + sight) / roleWidth));
    for (const {element, fill} of strips) {
      const cells = [];
      for (let at = from; at < to; at++) {
        const cell = node('span', undefined, 'section-role-cell');
        cell.style.left = (at * roleWidth - offset) + 'px';
        fill(shown[at], cell);
        cells.push(cell);
      }
      element.replaceChildren(...cells);
    }
    windowContent.style.setProperty('--roles-sight', sight + 'px');
    spacer.style.width = shown.length * roleWidth + 'px';
    scroller.hidden = shown.length <= rolesInSight;
    if (scroller.scrollLeft !== offset) scroller.scrollLeft = offset;
  };
  const roleBox = (line, role, cell) => {
    cell.append(roleVisibilityBox(roleVisibilityState(roles.get(line), role.id), 'Видимость: ' + role.title + ': ' + caption(line), () => {
      const said = turnRoleVisibility(roles.get(line), role.id);
      roles.set(line, said.length ? said : undefined);
      current = {line};
      edited();
    }));
  };
  const chosen = (row, which) => {
    row.classList.toggle('selected', isCurrent(which));
    row.addEventListener('click', event => { if (!(event.target instanceof HTMLInputElement)) { current = which; draw(); } });
    row.addEventListener('contextmenu', event => { current = which; draw(); menu(event); });
  };
  const draw = () => {
    strips = [];
    const rolesHead = node('div', undefined, 'section-roles-head');
    rolesHead.append(...(shownColumn('by-roles') ? [node('span', 'Видимость по ролям')] : []), strip((role, cell) => { cell.textContent = role.title; cell.title = role.title; }));
    head.replaceChildren(node('span', 'Команда', 'section-commands-caption'), ...(shownColumn('visibility') ? [node('span', 'Видимость', 'section-commands-visibility')] : []), rolesHead);
    const rows = groups.map(group => {
      const key = groupKey(group.group);
      const lines = commandsOf(key).filter(opened);
      if (filtering.length && !lines.length) return null;
      const children = lines.filter(line => !hidden || visible.get(line)).map(line => {
        const box = node('input');
        box.type = 'checkbox';
        box.checked = visible.get(line);
        box.setAttribute('aria-label', 'Видимость: ' + caption(line));
        box.addEventListener('change', () => { visible.set(line, box.checked); current = {line}; edited(); });
        const {element, row} = treeNode({icon: line.icon, caption: caption(line)});
        row.append(...(shownColumn('visibility') ? [box] : []), strip((role, cell) => roleBox(line, role, cell)));
        chosen(row, {line});
        return element;
      });
      const {element, row} = treeNode({icon: group.icon, caption: group.title + (orders.has(key) ? ' (ручной порядок)' : ''), children, open: true});
      chosen(row, {group: key});
      return element;
    }).filter(Boolean);
    const under = subsectionsShown().filter(opened);
    if (under.length) {
      const children = under.map(item => {
        const {element, row} = treeNode({icon: 'subsystem', caption: item.title});
        chosen(row, {subsection: item});
        return element;
      });
      const {element, row} = treeNode({caption: 'Подразделы' + (subsectionOrder ? ' (ручной порядок)' : ''), children, open: true});
      chosen(row, {subsections: true});
      rows.push(element);
    }
    tree.replaceChildren(...rows);
    paint();
    const command = !!current?.line, movable = command || !!current?.subsection;
    move.disabled = !command;
    up.disabled = down.disabled = standard.disabled = !movable;
  };
  function edited() { draw(); changed(); }
  async function moveCommand() {
    const line = current?.line;
    if (!line) return;
    const panel = byKey.get(home.get(line)).panel;
    const to = await chooseCommandGroup(groups.filter(group => group.panel === panel).map(group => [groupKey(group.group), group.title]));
    if (to == null || to === place.get(line)) return;
    place.set(line, to);
    if (arrivals.includes(line)) arrivals.splice(arrivals.indexOf(line), 1);
    arrivals.push(line);
    placedLines.add(line);
    edited();
  }
  function shift(by) {
    if (current?.line) {
      const key = place.get(current.line), list = commandsOf(key);
      if (!moveWithin(list, list.indexOf(current.line), by)) return;
      orders.set(key, list);
      orderedGroups.add(key);
    } else if (current?.subsection) {
      const list = subsectionsShown();
      if (!moveWithin(list, list.indexOf(current.subsection), by)) return;
      subsectionOrder = list;
      subsectionsTouched = true;
    }
    edited();
  }
  function setDefault() {
    const line = current?.line;
    if (!line) return;
    place.set(line, home.get(line));
    placedLines.add(line);
    visible.set(line, line.visible);
    roles.set(line, undefined);
    for (const [key, list] of orders) {
      if (!list.includes(line)) continue;
      const rest = list.filter(item => item !== line);
      if (rest.length) orders.set(key, rest); else orders.delete(key);
      orderedGroups.add(key);
    }
    edited();
  }
  function restore() {
    if (current?.group) { orders.delete(current.group); orderedGroups.add(current.group); }
    else if (current?.subsections) { subsectionOrder = null; subsectionsTouched = true; }
    edited();
  }
  function add(line, key, atStart) {
    const known = lines.find(item => same(item, line));
    if (known) { current = {line: known}; draw(); return; }
    lines.push(line);
    home.set(line, groupKey(line.group));
    place.set(line, key);
    visible.set(line, line.visible);
    roles.set(line, undefined);
    added.add(line);
    arrivals.push(line);
    if (atStart) first.unshift(line);
    const at = removedLines.findIndex(item => same(item, line));
    if (at >= 0) removedLines.splice(at, 1);
    current = {line};
    edited();
  }
  function remove() {
    const line = current?.line;
    if (!line) return;
    const key = place.get(line), list = commandsOf(key), at = list.indexOf(line);
    lines.splice(lines.indexOf(line), 1);
    removedLines.push(line);
    for (const [group, order] of orders) {
      if (!order.includes(line)) continue;
      orders.set(group, order.filter(item => item !== line));
      orderedGroups.add(group);
    }
    const next = list[at + 1] || list[at - 1];
    current = next ? {line: next} : {group: key};
    edited();
  }
  const menu = event => showContextMenu(event, [
    ...(moves ? [['Переместить команду', moveCommand, !!current?.line, '', 'move-item']] : []),
    ['Переместить вверх', () => shift(-1), !!(current?.line || current?.subsection), 'Cmd+Shift+Up', 'move-up'],
    ['Переместить вниз', () => shift(1), !!(current?.line || current?.subsection), 'Cmd+Shift+Down', 'move-down'],
    null,
    ['Установить свойства по умолчанию', setDefault, !!(current?.line || current?.subsection), '', 'restore-values'],
    ['Восстановить автоматический порядок', restore, !!(current?.group || current?.subsections)],
    ...(content ? [null, ['Удалить команду из основного раздела', remove, !!current?.line, 'Del', 'delete']] : []),
    null,
    ['Настройка списка...', async () => {
      const next = await chooseListSettings(listSettings, setting, applied => { setting = applied; draw(); });
      if (next) { setting = next; draw(); StudioListSettings.save(listSettings.key, next); }
    }, true, '', 'list-settings'],
  ]);
  tree.addEventListener('keydown', event => {
    if ((event.metaKey || event.ctrlKey) && event.shiftKey && (event.key === 'ArrowUp' || event.key === 'ArrowDown')) {
      event.preventDefault();
      shift(event.key === 'ArrowUp' ? -1 : 1);
    } else if (content && event.key === 'Delete') {
      event.preventDefault();
      remove();
    }
  });
  const hide = panelButton('filter-by-current-value', hideTitle, true, () => {
    hidden = !hidden;
    hide.classList.toggle('pressed', hidden);
    draw();
  });
  const move = panelButton('move-item', 'Переместить команду в другую группу команд', false, moveCommand);
  const up = panelButton('move-up', 'Переместить вверх', false, () => shift(-1)), down = panelButton('move-down', 'Переместить вниз', false, () => shift(1));
  const standard = panelButton('restore-values', 'Установить свойства по умолчанию', false, setDefault);
  const filter = node('label', undefined, 'section-commands-roles');
  const filterRoles = () => {
    const names = new Map(projectRoles.map(role => [role.id, role.name]));
    const text = ids => ids.map(id => names.get(id) || id).join(', ');
    const choices = [['', '<Не установлен>'], ...roleFilters.map(ids => [ids.join(','), text(ids)])];
    const field = selectInput(choices, filtering.join(','), key => { filtering = key ? key.split(',') : []; draw(); }, 'Отбор по ролям');
    const choose = fieldChoiceButton('Выбор объекта: Роль', async () => {
      const marked = await chooseObjects('Выбор объекта: Роль', [{title: answer.project, icon: 'configuration',
        objects: projectRoles.map(role => ({id: role.id, name: role.name, icon: role.icon}))}], filtering, {open: true});
      if (!marked) return;
      filtering = marked;
      if (marked.length) {
        const at = roleFilters.findIndex(ids => ids.join(',') === marked.join(','));
        if (at >= 0) roleFilters.splice(at, 1);
        roleFilters.unshift(marked);
      }
      filter.replaceChildren(node('span', 'Отбор по ролям:'), filterRoles());
      draw();
    });
    return fieldWithButtons(field, [choose]);
  };
  filter.append(node('span', 'Отбор по ролям:'), filterRoles());
  const bar = windowPanel([...(moves ? [move] : []), up, down, standard, hide, filter]);
  const head = node('div', undefined, 'section-commands-head');
  const scroller = node('div', undefined, 'section-roles-scroll'), spacer = node('div');
  scroller.append(spacer);
  scroller.addEventListener('scroll', () => { offset = scroller.scrollLeft; paint(); });
  // The wheel and the trackpad sideways over the tree scroll the columns too.
  tree.addEventListener('wheel', event => {
    if (!event.deltaX || scroller.hidden) return;
    event.preventDefault();
    scroller.scrollLeft += event.deltaX;
  }, {passive: false});
  const windowContent = node('div', undefined, 'section-commands-window');
  windowContent.append(bar, head, tree, scroller);
  draw();
  // The setting the user kept, once read; the list opens where it says.
  StudioListSettings.load().then(() => {
    setting = listSettings.read(StudioListSettings.get(listSettings.key));
    draw();
    requestAnimationFrame(() => { tree.scrollTop = setting.opening === 'end' ? tree.scrollHeight : 0; });
  });
  const at = () => {
    const key = current?.group || (current?.line && place.get(current.line));
    return key ? {key, panel: byKey.get(key)?.panel} : null;
  };
  return {element: windowContent, value, add, remove, at};
  function value() {
  const commandInterface = {...said};
  const shown = line => content || place.get(line) !== home.get(line);
  const kept = visibilities.filter(item => !lineOf(item) && !removedHere(item));
  for (const line of lines) {
    const now = visible.get(line), byRoles = roles.get(line);
    if (byRoles?.length || now !== line.visible || shown(line)) kept.push({...commandOf(line), visibility: {common: now, ...(byRoles?.length ? {roles: byRoles} : {})}});
  }
  commandInterface.commandsVisibility = kept;
  const saidPlacement = line => (said.commandsPlacement || []).find(item => same(item, line))?.placement || 'auto';
  commandInterface.commandsPlacement = content
    ? [...(said.commandsPlacement || []).filter(item => !lineOf(item) && !removedHere(item)),
      ...lines.map(line => ({...commandOf(line), group: byKey.get(place.get(line)).group, placement: saidPlacement(line)}))]
    : [...(said.commandsPlacement || []).filter(item => !placedLines.has(lineOf(item))),
      ...[...placedLines].filter(shown).map(line => ({...commandOf(line), group: byKey.get(place.get(line)).group, placement: 'auto'}))];
  if (removedLines.length) commandInterface.commandsOrder = (said.commandsOrder || []).filter(item => !removedHere(item));
  if (orderedGroups.size) {
    const order = [...(commandInterface.commandsOrder || []).filter(item => !orderedGroups.has(groupKey(item.group))),
      ...[...orderedGroups].filter(key => orders.has(key)).flatMap(key => commandsOf(key).map(line => ({...commandOf(line), group: byKey.get(key).group})))];
    const seen = new Map(order.map(item => [groupKey(item.group), item.group]));
    commandInterface.commandsOrder = order;
    commandInterface.groupsOrder = [...seen.values()];
  }
  if (subsectionsTouched) commandInterface.subsystemsOrder = subsectionOrder ? subsectionsShown().map(item => item.id) : [];
  for (const key of ['commandsVisibility', 'commandsPlacement', 'commandsOrder', 'groupsOrder', 'subsystemsOrder']) {
    if (!commandInterface[key]?.length) delete commandInterface[key];
  }
  return Object.keys(commandInterface).length ? commandInterface : null;
  }
}

/* commandListSettings is the setting of the list of commands of every
   window of a command interface (obshchee.md, «Список команд»; snapshots
   predsyomka-1/05-17, 32, 39), key apart for each: «Команда» twice, always
   shown - a new column of width (40, of «Все подсистемы» 25) and one on the
   next row, 27 - «Видимость» and «Видимость по ролям», new columns of 8,
   these four of autoheight; then a column a role of the project, by its
   synonym, in the order of the project, the first on the next row, the
   rest in the same column, 87 wide. The list opens at its beginning. */
function commandListSettings(key, roles, width = 40) {
  return createListSettings({key, opening: 'beginning', columns: [
    {key: 'command', title: 'Команда', width, autoHeight: true, fixed: true},
    {key: 'command-next', title: 'Команда', location: 'next-row', width: 27, autoHeight: true, fixed: true},
    {key: 'visibility', title: 'Видимость', width: 8, autoHeight: true},
    {key: 'by-roles', title: 'Видимость по ролям', width: 8, autoHeight: true},
    ...roles.map((role, at) => ({key: 'role:' + role.id, title: role.title, location: at ? 'same-column' : 'next-row', width: 87})),
  ]});
}

/* showProjectCommandInterface is the window «Командный интерфейс» of the
   root (konfiguraciya.md, «Окна», «Досъёмка 27.10.2026», «Предсъёмка
   28.10.2026»; snapshots konfiguraciya/16, 17, series-2.595/36-40, 66-68):
   the list of every window of a command interface (sectionCommandsPane)
   with one group, «Разделы» - the subsystems of the root in the command
   interface, by their synonyms (server, ProjectSections), visible by
   default - and no move into another group; «Скрыть невидимые по
   умолчанию». configuration is the root as its editor holds it; OK gives
   set(order, visibility) the order of the sections and their visibility,
   common and by roles - project.Project SubsystemsOrder and
   SubsystemsVisibility - what the list does not show kept as it was;
   «Отмена» changes nothing. Answers whether OK was pressed. */
async function showProjectCommandInterface(configuration, language, set) {
  const answer = await (await fetch('/api/project/sections')).json();
  const group = {standard: 'sections'};
  const lines = (answer.subsections || []).map(item => ({object: item.id, title: item.title, icon: 'subsystem', visible: true, opens: item.opens, group}));
  const said = {
    commandsVisibility: (configuration.subsystemsVisibility || []).map(item => ({object: item.subsystem,
      visibility: {common: item.common, ...(item.roles?.length ? {roles: item.roles} : {})}})),
    commandsOrder: (configuration.subsystemsOrder || []).map(id => ({object: id, group})),
  };
  const pane = sectionCommandsPane({...answer, groups: [{group, title: 'Разделы', panel: 'sections', commands: lines}], subsections: []}, said, language,
    () => {}, {moves: false, hideTitle: 'Скрыть невидимые по умолчанию', list: {key: 'root-commands'}});
  if (!await commandInterfaceWindow(pane, 'project-command-interface')) return false;
  const value = pane.value() || {};
  set((value.commandsOrder || []).map(item => item.object), (value.commandsVisibility || []).map(item => ({subsystem: item.object,
    common: item.visibility.common, ...(item.visibility.roles ? {roles: item.visibility.roles} : {})})));
  return true;
}

/* showMainSectionCommandInterface is the window «Командный интерфейс
   основного раздела» of the root (konfiguraciya.md, «Окна», «Досъёмка
   27.10.2026», «Предсъёмка 28.10.2026»; snapshots konfiguraciya/21, 22,
   series-2.595/50-52, 69, 70, predsyomka-1/34-38), modal. At the left
   «Доступные команды:» - the project open, its branches holding the
   objects that give a section a command, by their names, and under each
   its commands by their titles (server, mainSectionAvailable); no menu.
   Between, «>» and «<». At the right the list of every window of a command
   interface (sectionCommandsPane) over the commands placed in the main
   section, «Переместить команду» first. «>» puts the command chosen at the
   left in: a command to create one at the start of «Панель
   действий.Создать», unseen, whatever is chosen at the right; a list at the
   end of the group of the navigation panel chosen at the right - its own
   group where none is - visible; any other command at the end of its own
   group, as its kind shows it (question В2 of series 2.646). «<» and Del
   take the command current out. OK writes the command interface into the
   project's file of it (/api/project/main-section) at the revision read:
   the window has no editor behind it to hold the change till «Сохранить».
   Answers the error of the writing, or null; «Отмена» writes nothing. */
async function showMainSectionCommandInterface() {
  const response = await fetch('/api/project/main-section');
  if (!response.ok) return new Error((await response.text()).trim());
  const answer = await response.json();
  const language = answer.defaultLanguage;
  const caption = line => localizedTitle({title: line.title}, language) + (sectionActions[line.standard] || '');
  const pane = sectionCommandsPane({...answer, subsections: []}, answer.commandInterface || {}, language, () => {},
    {hideTitle: 'Скрыть невидимые по умолчанию', content: true, list: {key: 'main-section-commands'}});
  let chosen = null;
  const tree = node('div', undefined, 'object-window-tree main-section-available');
  const keyOf = group => [group.standard, group.group, group.written].map(part => part || '').join('|');
  const object = item => treeNode({icon: item.icon, caption: item.name, children: (answer.commands[item.id] || []).map(line => {
    const {element, row} = treeNode({icon: line.icon, caption: caption(line)});
    row.addEventListener('click', () => {
      chosen = line;
      for (const other of tree.querySelectorAll('.studio-tree-row.selected')) other.classList.remove('selected');
      row.classList.add('selected');
    });
    return element;
  })}).element;
  const branch = group => treeNode({icon: group.icon, caption: group.title,
    children: [...(group.children || []).map(branch), ...(group.objects || []).map(object)]}).element;
  tree.replaceChildren(treeNode({icon: 'configuration', caption: answer.project, children: answer.available.map(branch), open: true}).element);
  const put = () => {
    if (!chosen) return;
    const line = {...chosen, visible: chosen.visible};
    if (line.standard === 'create' || line.standard === 'create-folder') { pane.add(line, keyOf({standard: 'actions-panel-create'}), true); return; }
    const at = pane.at();
    const navigation = line.standard === 'open-list' && at?.panel === 'navigation' ? at.key : keyOf(line.group);
    pane.add({...line, visible: line.standard === 'open-list' ? true : line.visible}, navigation, false);
  };
  const moves = transferButtons('main-section-moves', ['Добавить команду в основной раздел', put, true],
    ['Удалить команду из основного раздела', () => pane.remove(), true]);
  const available = node('div', undefined, 'main-section-left');
  available.append(node('div', 'Доступные команды:', 'main-section-caption'), tree);
  const content = node('div', undefined, 'main-section');
  content.append(available, moves, pane.element);
  if (await showDialog({title: 'Командный интерфейс основного раздела', content, className: 'section-commands-dialog main-section-dialog',
    help: 'main-section-command-interface', answers: [['ok', 'OK'], [null, 'Отмена']]}) !== 'ok') return null;
  const saved = await studioRequest('/api/project/main-section', {method: 'PUT', body: {value: pane.value() || {}, expectedRevision: answer.revision}});
  if (saved.status === 409) return new Error('Командный интерфейс основного раздела изменён вне Studio. Откройте окно заново и повторите правку.');
  return saved.ok ? null : new Error((await saved.text()).trim());
}
