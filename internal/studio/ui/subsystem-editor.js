'use strict';

/* The window of a subsystem, as the prototype's (docs/block-00/
   podsistema.md, «Окно»; snapshots podsistema/04-12): the window of an
   object of rows (objectWindowEditor) - «Основные» with the flags, the
   button «Командный интерфейс», the explanation and the picture,
   «Функциональные опции» - the options of the project marked where the
   subsystem is in their content - «Состав» and «Прочее» with the help -
   and its palette of three tabs, whose «Состав: Открыть» shows the page
   «Состав». The window «Командный интерфейс» and «Выбор картинки» are
   items of their own: their button and «…» stand unavailable. */
function createSubsystemEditor(host, onChange, around = {}) {
  return objectWindowEditor(host, onChange, around, {target: source => source.subsystem, own: api => {
    const opened = new Set();
    let chosen = null;
    return {pages: {composition: () => subsystemComposition(api, opened, () => chosen, id => { chosen = id; })},
      context: () => ({windows: ['members'], openWindow: () => api.show('composition')}), reset() { chosen = null; }};
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
