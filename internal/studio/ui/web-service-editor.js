'use strict';

/* The window of a web service, as the prototype's (docs/block-00/
   web-servis.md, «Окно»; snapshots web-servis/01-04, 09, 14): the window of
   an object of rows and subsystems (objectWindowEditor) with the page
   «Операции» of its own - the tree of the operations and their parameters
   with its panel; the palette of the operation or parameter chosen beside
   it. «…» of «Пакеты XDTO» and of the types is unavailable until the windows
   choosing packages and types. */
function createWebServiceEditor(host, onChange, around = {}) {
  return objectWindowEditor(host, onChange, around, {target: source => source.service, own: webServiceOperations});
}

/* webServiceOperations is the page «Операции» of the window of a web
   service over the window's api (objectWindowEditor). */
function webServiceOperations(api) {
  let chosen = null;
  const service = () => api.source().service;
  // The operation chosen, or the one of the parameter chosen, and where it
  // stands among the operations.
  function chosenOperation() {
    const operations = service().operations || [], at = chosen ? operations.findIndex(item => item.id === (chosen.operation ?? chosen.id)) : -1;
    return {operation: operations[at], at};
  }
  function chosenPart() {
    const {operation} = chosenOperation();
    return chosen?.kind === 'parameter' ? operation?.parameters?.find(item => item.id === chosen.id) : operation;
  }
  // The palette of the part chosen, its name checked among its siblings by
  // the model (PartNameFault) at its place in the service.
  function part() {
    const {operation, at} = chosenOperation(), chosenOne = chosenPart();
    if (!chosenOne) return null;
    const parameters = operation.parameters || [];
    const where = chosen.kind === 'parameter' ? ['operations', String(at), 'parameters', String(parameters.findIndex(item => item.id === chosen.id))] : ['operations', String(at)];
    const source = api.source();
    return {title: chosenOne.name, tabs: source.partPalettes?.[chosen.kind],
      subject: {...ownSubject(source, chosenOne), check: () => ({path: source.path, description: service(), at: where})}};
  }
  // A new operation and parameter as the prototype makes them
  // (web-servis.md, «Новый сервис»): «Операция1», returning a string of
  // XMLSchema, the lock managed, its procedure to be named; «Параметр1», a
  // string, «Входной». A new part stands last, named the next number of its
  // base among its siblings, and is chosen.
  const xmlString = () => ({namespace: 'http://www.w3.org/2001/XMLSchema', name: 'string'});
  function addPart(list, base, fields, choice) {
    const added = {id: crypto.randomUUID(), name: numberedName(base, new Set(list.map(item => item.name.toLocaleLowerCase()))), title: {}, ...fields};
    list.push(added);
    chosen = {...choice, id: added.id};
    api.changed();
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
      ...partButtons(on, {change: () => document.querySelector('#properties input')?.focus(),
        remove: () => { const list = siblings(), at = list.findIndex(item => item.id === chosen.id); if (at >= 0) list.splice(at, 1); chosen = null; api.changed(); },
        move: by => { const list = siblings(), at = list.findIndex(item => item.id === chosen.id); if (at >= 0) moveInCircle(list, at, by); api.changed(); }}),
    ]);
    const row = (icon, caption, choice, children = []) => {
      const {element, row: line} = treeNode({icon, caption, children, open: true});
      line.classList.toggle('selected', !!chosen && chosen.kind === choice.kind && chosen.id === choice.id);
      line.addEventListener('click', () => { chosen = choice; api.refresh(); api.palette(); });
      return element;
    };
    const tree = node('div', undefined, 'object-window-tree object-window-tree-tall');
    tree.append(...(service().operations || []).map(operation => row('web-service-operation', operation.name, {kind: 'operation', id: operation.id},
      (operation.parameters || []).map(parameter => row('parameter', parameter.name, {kind: 'parameter', id: parameter.id, operation: operation.id})))));
    page.append(bar, tree);
    return page;
  }
  return {pages: {operations: renderOperationsPage}, part, reset() { chosen = null; }};
}
