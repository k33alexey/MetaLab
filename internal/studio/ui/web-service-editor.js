'use strict';

/* The window of a web service, as the prototype's (docs/block-00/
   web-servis.md, «Окно»; snapshots web-servis/01-04): the pages of the kind
   (source.pages) - «Основные», «Подсистемы», «Операции», «Управление
   сеансами», «Прочее» - built of the palette's rows (source.windowRows),
   «Подсистемы» the tree of the subsystems with boxes, «Операции» the tree of
   the operations and their parameters with its panel; the palette of the
   service, or of the operation or parameter chosen, beside it, each showing
   what the other typed. «…» of «Пакеты XDTO» and of the types is unavailable
   until the windows choosing packages and types. around holds the palette, its title, the opening of a
   module, the actions and the closing of the window. */
function createWebServiceEditor(host, onChange, around = {}) {
  let source = null, frame = null, marks = new Set(), cleanups = [], chosen = null;
  const paletteCleanups = [];
  const service = () => source.service;
  const title = () => `${source.kindTitle} ${service().name}`;
  // What the palette edits: the service, or the operation or parameter
  // chosen on «Операции», its name checked among its siblings by the model
  // (PartNameFault) at its place in the service.
  // The operation chosen, or the one of the parameter chosen, and where it
  // stands among the operations.
  function chosenOperation() {
    const operations = service().operations || [], at = chosen ? operations.findIndex(item => item.id === (chosen.operation ?? chosen.id)) : -1;
    return {operation: operations[at], at};
  }
  function subject() {
    const {operation, at} = chosenOperation();
    if (!operation) return ownSubject(source, service());
    const parameters = operation.parameters || [];
    const where = chosen.kind === 'parameter' ? ['operations', String(at), 'parameters', String(parameters.findIndex(item => item.id === chosen.id))] : ['operations', String(at)];
    const part = chosen.kind === 'parameter' ? parameters.find(item => item.id === chosen.id) : operation;
    return {...ownSubject(source, part), check: () => ({path: source.path, description: service(), at: where})};
  }
  function chosenPart() {
    const {operation} = chosenOperation();
    return chosen?.kind === 'parameter' ? operation?.parameters?.find(item => item.id === chosen.id) : operation;
  }
  // A new operation and parameter as the prototype makes them
  // (web-servis.md, «Новый сервис»): «Операция1», returning a string of
  // XMLSchema, the lock managed, its procedure to be named; «Параметр1», a
  // string, «Входной».
  const xmlString = () => ({namespace: 'http://www.w3.org/2001/XMLSchema', name: 'string'});
  // A new part stands last in its list, named the next number of base
  // among its siblings, and is chosen.
  function addPart(list, base, fields, choice) {
    const part = {id: crypto.randomUUID(), name: numberedName(base, new Set(list.map(item => item.name.toLocaleLowerCase()))), title: {}, ...fields};
    list.push(part);
    chosen = {...choice, id: part.id};
    changed();
  }
  function addOperation() {
    addPart(service().operations ||= [], 'Операция', {procedure: '', returnType: xmlString(), dataLockControl: 'managed'}, {kind: 'operation'});
  }
  function addParameter() {
    const {operation} = chosenOperation();
    if (operation) addPart(operation.parameters ||= [], 'Параметр', {type: xmlString(), direction: 'in'}, {kind: 'parameter', operation: operation.id});
  }
  // The list a chosen part stands in, to remove it from or move it in.
  function siblings() {
    return chosen?.kind === 'parameter' ? chosenOperation().operation?.parameters || [] : service().operations || [];
  }
  function renderOperationsPage() {
    const page = node('div', undefined, 'object-window-fields'), on = !!chosenPart();
    const bar = windowPanel([
      panelButton('add', 'Добавить операцию', true, addOperation),
      panelButton('attribute-add', 'Добавить параметр', on, addParameter),
      ...partButtons(on, {change: () => around.palette?.querySelector('input')?.focus(),
        remove: () => { const list = siblings(), at = list.findIndex(item => item.id === chosen.id); if (at >= 0) list.splice(at, 1); chosen = null; changed(); },
        move: by => { const list = siblings(), at = list.findIndex(item => item.id === chosen.id); if (at >= 0) moveInCircle(list, at, by); changed(); }}),
    ]);
    const row = (icon, caption, choice, children = []) => {
      const {element, row: line} = treeNode({icon, caption, children, open: true});
      line.classList.toggle('selected', !!chosen && chosen.kind === choice.kind && chosen.id === choice.id);
      line.addEventListener('click', () => { chosen = choice; frame.refresh(); renderPalette(); });
      return element;
    };
    const tree = node('div', undefined, 'object-window-tree object-window-tree-tall');
    tree.append(...(service().operations || []).map(operation => row('web-service-operation', operation.name, {kind: 'operation', id: operation.id},
      (operation.parameters || []).map(parameter => row('parameter', parameter.name, {kind: 'parameter', id: parameter.id, operation: operation.id})))));
    page.append(bar, tree);
    return page;
  }
  function context(collect) {
    return {project: source, typed, changed, packages: source.packages, windows: [], cleanups: collect,
      openModule: role => around.openModule?.(role)};
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
    if (key === 'subsystems') {
      // The caption above the tree is not shot for any kind but a catalog
      // (series 2.584, В2): the tree stands alone.
      const page = node('div', undefined, 'object-window-fields');
      page.append(subsystemsTree(source.subsystems, marks, onChange));
      return page;
    }
    if (key === 'operations') return renderOperationsPage();
    if (!source.windowRows?.[key]) return null;
    return rowsPage(source.windowRows[key], source.palette, paletteFields(context(() => cleanups)), subject(), role => around.openModule?.(role));
  }
  function renderPalette() {
    if (!source) return;
    const part = chosenPart();
    showObjectPalette(around, paletteCleanups, part ? part.name : service().name,
      part ? source.partPalettes?.[chosen.kind] : source.palette, subject(), context());
  }
  StudioLanguage.onChange(() => { if (source) { frame.refresh(); renderPalette(); } });
  return hostedEditor(host, () => { cleanups.forEach(destroy => destroy()); cleanups = []; if (source) clearObjectPalette(around, paletteCleanups); source = null; }, {
    open(value) {
      if (source) clearObjectPalette(around, paletteCleanups);
      source = structuredClone(value);
      chosen = null;
      marks = new Set((source.subsystems || []).filter(place => place.member).map(place => place.id));
      frame = showObjectWindow(host, around, {title: title(), pages: source.pages, chosen: 'main', page: renderPage});
      renderPalette();
    },
    // The service and the subsystems marked, in one order so a mark taken
    // back leaves the window unchanged.
    value() { return source && {...structuredClone(service()), subsystems: [...marks].sort()}; },
    showPalette() {
      if (!source || !around.palette) return false;
      chosen = null;
      frame.refresh();
      renderPalette();
      return true;
    },
  });
}
