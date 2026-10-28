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
  // A branch of the objects offered stands whether it holds objects or not,
  // as the branches of the project tree do; one of the objects held only
  // where it holds one.
  const branchNode = (branch, row, every) => {
    const children = [...(branch.children || []).flatMap(inner => branchNode(inner, row, every)), ...(branch.objects || []).flatMap(row)];
    if (!children.length && !every) return [];
    return [treeNode({icon: branch.icon, caption: branch.title, children, open: opened.has(branch.title) || holds(branch),
      toggled: open => { if (open) opened.add(branch.title); else opened.delete(branch.title); }}).element];
  };
  const offered = node('div', undefined, 'object-window-tree object-window-tree-tall subsystem-composition');
  offered.append(...api.source().composition.flatMap(branch => branchNode(branch,
    item => [treeNode({icon: item.icon, caption: item.name, mark: markBox(item.name, held, item.id, changed)}).element], true)));
  const heldBranches = branches => branches.map(branch => ({...branch, objects: (branch.objects || []).filter(item => held.has(item.id)),
    children: heldBranches(branch.children || [])}));
  const below = node('div', undefined, 'object-window-tree subsystem-composition-held');
  below.append(...heldBranches(api.source().composition).flatMap(branch => branchNode(branch, item => {
    const {element, row} = treeNode({icon: item.icon, caption: item.name});
    row.classList.toggle('selected', item.id === chosen());
    row.addEventListener('click', () => { choose(item.id); api.refresh(); });
    return [element];
  }, false)));
  const remove = clearButton('Удалить из состава', () => { held.delete(chosen()); choose(null); changed(); });
  remove.disabled = !held.has(chosen());
  const caption = node('div', undefined, 'object-window-caption subsystem-composition-caption');
  caption.append(node('span', 'Входящие в подсистему объекты:'), remove);
  const page = node('div', undefined, 'object-window-fields');
  page.append(offered, caption, below);
  return page;
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
    ['Переместить подсистему', async () => { if (await moveSubsystem(node.path, actions.projectName())) actions.moved(); }], null,
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
  const groups = answer.groups, subsections = answer.subsections || [], projectRoles = answer.roles || [];
  const identity = ['command', 'object', 'standard', 'written'];
  const same = (one, other) => identity.every(key => (one[key] || '') === (other[key] || ''));
  const commandOf = line => Object.fromEntries(identity.filter(key => line[key]).map(key => [key, line[key]]));
  const groupKey = group => [group.standard, group.group, group.written].map(part => part || '').join('|');
  const said = object.commandInterface || {};
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
  let subsectionsTouched = false;
  const actions = {create: ': создать', 'create-folder': ': создать группу'};
  const caption = line => localizedTitle({title: line.title}, api.source().defaultLanguage) + (actions[line.standard] || '');
  const alphabet = (one, other) => caption(one).localeCompare(caption(other), 'ru');
  const commandsOf = key => {
    const members = lines.filter(line => place.get(line) === key);
    const manual = (orders.get(key) || []).filter(line => members.includes(line));
    const rest = members.filter(line => !manual.includes(line));
    const come = rest.filter(line => home.get(line) !== key).sort((one, other) => arrivals.indexOf(one) - arrivals.indexOf(other));
    return [...manual, ...rest.filter(line => home.get(line) === key).sort(alphabet), ...come];
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
  const columns = () => filtering.length ? projectRoles.filter(role => filtering.includes(role.id)) : projectRoles;
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
    content.style.setProperty('--roles-sight', sight + 'px');
    spacer.style.width = shown.length * roleWidth + 'px';
    scroller.hidden = shown.length <= rolesInSight;
    if (scroller.scrollLeft !== offset) scroller.scrollLeft = offset;
  };
  const roleState = (line, role) => {
    const said = roles.get(line)?.find(item => item.role === role.id);
    return said ? (said.visible ? 'on' : 'off') : 'default';
  };
  const roleBox = (line, role, cell) => {
    const box = node('span', undefined, 'role-visibility'), state = roleState(line, role);
    box.dataset.state = state;
    box.setAttribute('role', 'checkbox');
    box.setAttribute('aria-checked', {default: 'mixed', on: 'true', off: 'false'}[state]);
    box.setAttribute('aria-label', 'Видимость: ' + role.title + ': ' + caption(line));
    box.addEventListener('click', event => {
      event.stopPropagation();
      const said = [...(roles.get(line) || [])], at = said.findIndex(item => item.role === role.id);
      if (at < 0) said.push({role: role.id, visible: false});
      else if (!said[at].visible) said[at] = {role: role.id, visible: true};
      else said.splice(at, 1);
      roles.set(line, said.length ? said : undefined);
      current = {line};
      draw();
    });
    cell.append(box);
  };
  const chosen = (row, which) => {
    row.classList.toggle('selected', isCurrent(which));
    row.addEventListener('click', event => { if (!(event.target instanceof HTMLInputElement)) { current = which; draw(); } });
    row.addEventListener('contextmenu', event => { current = which; draw(); menu(event); });
  };
  const draw = () => {
    strips = [];
    const rolesHead = node('div', undefined, 'section-roles-head');
    rolesHead.append(node('span', 'Видимость по ролям'), strip((role, cell) => { cell.textContent = role.title; cell.title = role.title; }));
    head.replaceChildren(node('span', 'Команда', 'section-commands-caption'), node('span', 'Видимость', 'section-commands-visibility'), rolesHead);
    const rows = groups.map(group => {
      const key = groupKey(group.group);
      const lines = commandsOf(key).filter(opened);
      if (filtering.length && !lines.length) return null;
      const children = lines.filter(line => !hidden || visible.get(line)).map(line => {
        const box = node('input');
        box.type = 'checkbox';
        box.checked = visible.get(line);
        box.setAttribute('aria-label', 'Видимость: ' + caption(line));
        box.addEventListener('change', () => { visible.set(line, box.checked); current = {line}; draw(); });
        const {element, row} = treeNode({icon: line.icon, caption: caption(line)});
        row.append(box, strip((role, cell) => roleBox(line, role, cell)));
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
    draw();
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
    draw();
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
    draw();
  }
  function restore() {
    if (current?.group) { orders.delete(current.group); orderedGroups.add(current.group); }
    else if (current?.subsections) { subsectionOrder = null; subsectionsTouched = true; }
    draw();
  }
  const menu = event => showContextMenu(event, [
    ['Переместить команду', moveCommand, !!current?.line, '', 'move-item'],
    ['Переместить вверх', () => shift(-1), !!(current?.line || current?.subsection), 'Cmd+Shift+Up', 'move-up'],
    ['Переместить вниз', () => shift(1), !!(current?.line || current?.subsection), 'Cmd+Shift+Down', 'move-down'],
    null,
    ['Установить свойства по умолчанию', setDefault, !!(current?.line || current?.subsection), '', 'restore-values'],
    ['Восстановить автоматический порядок', restore, !!(current?.group || current?.subsections)],
  ]);
  tree.addEventListener('keydown', event => {
    if ((event.metaKey || event.ctrlKey) && event.shiftKey && (event.key === 'ArrowUp' || event.key === 'ArrowDown')) {
      event.preventDefault();
      shift(event.key === 'ArrowUp' ? -1 : 1);
    }
  });
  const hide = panelButton('filter-by-current-value', 'Скрыть команды, невидимые по умолчанию', true, () => {
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
  const bar = windowPanel([move, up, down, standard, hide, filter]);
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
  const content = node('div', undefined, 'section-commands-window');
  content.append(bar, head, tree, scroller);
  draw();
  if (await showDialog({title: 'Командный интерфейс', content, className: 'section-commands-dialog', help: 'subsystem-command-interface',
    answers: [['ok', 'OK'], [null, 'Отмена']]}) !== 'ok') return false;
  const commandInterface = {...said};
  const shown = line => place.get(line) !== home.get(line);
  const kept = visibilities.filter(item => !lineOf(item));
  for (const line of lines) {
    const now = visible.get(line), byRoles = roles.get(line);
    if (byRoles?.length || now !== line.visible || shown(line)) kept.push({...commandOf(line), visibility: {common: now, ...(byRoles?.length ? {roles: byRoles} : {})}});
  }
  commandInterface.commandsVisibility = kept;
  commandInterface.commandsPlacement = [...(said.commandsPlacement || []).filter(item => !placedLines.has(lineOf(item))),
    ...[...placedLines].filter(shown).map(line => ({...commandOf(line), group: byKey.get(place.get(line)).group, placement: 'auto'}))];
  if (orderedGroups.size) {
    const order = [...(said.commandsOrder || []).filter(item => !orderedGroups.has(groupKey(item.group))),
      ...[...orderedGroups].filter(key => orders.has(key)).flatMap(key => commandsOf(key).map(line => ({...commandOf(line), group: byKey.get(key).group})))];
    const seen = new Map(order.map(item => [groupKey(item.group), item.group]));
    commandInterface.commandsOrder = order;
    commandInterface.groupsOrder = [...seen.values()];
  }
  if (subsectionsTouched) commandInterface.subsystemsOrder = subsectionOrder ? subsectionsShown().map(item => item.id) : [];
  for (const key of ['commandsVisibility', 'commandsPlacement', 'commandsOrder', 'groupsOrder', 'subsystemsOrder']) {
    if (!commandInterface[key]?.length) delete commandInterface[key];
  }
  if (Object.keys(commandInterface).length) object.commandInterface = commandInterface; else delete object.commandInterface;
  api.changed();
  return true;
}
