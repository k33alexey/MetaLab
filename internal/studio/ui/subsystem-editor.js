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

/* showSubsystemCommandInterface is the window «Командный интерфейс» of a
   subsystem (podsistema.md, «Окно», «Командный интерфейс…», «Досъёмка
   28.10.2026»; snapshots podsistema/13, 14, series-2.603/42, 43): over the
   tree, «Переместить команду в другую группу команд», up, down - items of
   their own, unavailable - «Скрыть команды, невидимые по умолчанию» -
   a command whose box is off leaves the tree while it is pressed
   (konfiguraciya.md, В14 of series 2.595) - and
   «Отбор по ролям» - unavailable until its item; the tree «Команда —
   Видимость»: the groups of the section in the prototype's order (server,
   sectionGroups), a group with commands open, the commands of the content
   of the window - saved or not - by the alphabet of their presentation,
   each with the box of its visibility: the command interface's, or else
   the default of its kind (metadata.SectionLines). OK writes into the
   subsystem the visibility that differs from the default, and takes away
   what was said and is the default again unless roles are said for it;
   «Отмена» changes nothing. */
async function showSubsystemCommandInterface(api) {
  const object = api.object();
  const groups = await postJSON('/api/subsystem/section', {members: object.members || []});
  const identity = ['command', 'object', 'standard', 'written'];
  const same = (one, other) => identity.every(key => (one[key] || '') === (other[key] || ''));
  const said = object.commandInterface?.commandsVisibility || [];
  const lines = groups.flatMap(group => group.commands || []);
  const visible = new Map(lines.map(line => [line, said.find(item => same(item, line))?.visibility.common ?? line.visible]));
  const actions = {create: ': создать', 'create-folder': ': создать группу'};
  const caption = line => localizedTitle({title: line.title}, api.source().defaultLanguage) + (actions[line.standard] || '');
  let hidden = false;
  const tree = node('div', undefined, 'object-window-tree section-commands');
  const draw = () => {
    tree.replaceChildren(...groups.map(group => {
      const commands = (group.commands || []).filter(line => !hidden || visible.get(line))
        .sort((one, other) => caption(one).localeCompare(caption(other), 'ru'));
      const children = commands.map(line => {
        const box = node('input');
        box.type = 'checkbox';
        box.checked = visible.get(line);
        box.setAttribute('aria-label', 'Видимость: ' + caption(line));
        box.addEventListener('change', () => visible.set(line, box.checked));
        const {element, row} = treeNode({icon: line.icon, caption: caption(line)});
        row.append(box);
        return element;
      });
      return treeNode({icon: group.icon, caption: group.title, children, open: true}).element;
    }));
  };
  const hide = panelButton('filter-by-current-value', 'Скрыть команды, невидимые по умолчанию', true, () => {
    hidden = !hidden;
    hide.classList.toggle('pressed', hidden);
    draw();
  });
  const roles = selectInput([['', '<Не установлен>']], '', () => {}, 'Отбор по ролям');
  roles.disabled = true;
  const filter = node('label', undefined, 'section-commands-roles');
  filter.append(node('span', 'Отбор по ролям:'), roles);
  const bar = windowPanel([panelButton('move-item', 'Переместить команду в другую группу команд', false, () => {}),
    panelButton('move-up', 'Переместить вверх', false, () => {}), panelButton('move-down', 'Переместить вниз', false, () => {}), hide, filter]);
  const head = node('div', undefined, 'section-commands-head');
  head.append(node('span', 'Команда'), node('span', 'Видимость'));
  const content = node('div', undefined, 'section-commands-window');
  content.append(bar, head, tree);
  draw();
  if (await showDialog({title: 'Командный интерфейс', content, className: 'section-commands-dialog', help: 'subsystem-command-interface',
    answers: [['ok', 'OK'], [null, 'Отмена']]}) !== 'ok') return false;
  const kept = said.filter(item => !lines.some(line => same(item, line)));
  for (const line of lines) {
    const before = said.find(item => same(item, line)), now = visible.get(line);
    const command = Object.fromEntries(identity.filter(key => line[key]).map(key => [key, line[key]]));
    if (before?.visibility.roles?.length || now !== line.visible) kept.push({...command, visibility: {...before?.visibility, common: now}});
  }
  const commandInterface = {...object.commandInterface};
  if (kept.length) commandInterface.commandsVisibility = kept; else delete commandInterface.commandsVisibility;
  if (Object.keys(commandInterface).length) object.commandInterface = commandInterface; else delete object.commandInterface;
  api.changed();
  return true;
}
