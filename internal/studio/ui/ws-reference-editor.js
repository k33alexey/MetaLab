'use strict';

/* The palette of a WS reference, which has no window to edit it in
   (docs/block-00/ws-ssylka.md): the name, the synonym, the comment, «URL
   источника» - the address the import wrote, a link nobody types into - and
   «WS-ссылка: Открыть», the window viewing the description
   (showWSReferenceView). around holds the palette and its title. */
function createWSReferenceEditor(host, onChange, around = {}) {
  return paletteEditor(host, onChange, around, {target: source => source.reference,
    context: source => ({windows: ['definition'], openWindow: () => showWSReferenceView(source.path)})});
}

/* showWSReferenceView is the window viewing the description of a WS
   reference, «WS-ссылка <Имя>: WS-ссылка [Только для чтения]»
   (ws-ssylka.md, «Окно»; snapshots ws-ssylka/04-09,
   configurator-2026-10-27/series-2.595/16-34): over Studio and not modal, a
   tree - «Модель данных», its packages as the editor of an XDTO package
   draws them (xdtoTree), and «Web-сервисы»: the services, their ports, the
   operations and their parameters - and the palette of the node chosen,
   which shows and edits nothing (readOnlyPalette); with no node chosen,
   the palette of the description. path is the description of the
   reference. */
async function showWSReferenceView(path) {
  const view = await (await fetch('/api/ws-reference/view?path=' + encodeURIComponent(path))).json();
  document.querySelector('dialog.ws-reference-view')?.remove();
  const window = document.createElement('dialog');
  window.className = 'ws-reference-view';
  const tree = node('div', undefined, 'object-window-tree ws-reference-view-tree'), palette = node('div', undefined, 'ws-reference-view-palette');
  let chosen = null;
  const leaf = (caption, kind, value, children = []) => ({caption, kind, value, children});
  const nodes = [
    leaf('Модель данных', 'folder', null, view.model.packages.map(item => ({...xdtoTree(item, {namespace: item.namespace}), content: item}))),
    leaf('Web-сервисы', 'folder', null, view.model.services.map(service => leaf(service.name, 'wsService', service, service.ports.map(port =>
      leaf(port.name, 'wsPort', port, port.operations.map(operation => leaf(operation.name, 'wsOperation', operation,
        operation.parameters.map(parameter => leaf(parameter.name, 'wsParameter', parameter))))))))),
  ];
  function paletteOf(item) {
    if (!item) return ['wsRoot', () => undefined, 'Описание определения Web-сервисов'];
    if (item.kind === 'package') return ['xdtoPackage', key => item.content[key], item.caption];
    if (item.palette) return [item.palette, key => item.subject.get(key), item.caption];
    if (item.value) return [item.kind, key => item.value[key], item.caption];
    return null;
  }
  // A branch stands open as it was left, and open down to the node chosen.
  const opened = new Set(nodes);
  const holds = (item, wanted) => item === wanted || item.children.some(child => holds(child, wanted));
  function draw(item) {
    const {element, row} = treeNode({caption: item.caption, children: item.children.map(draw), open: opened.has(item) || !!chosen && item !== chosen && holds(item, chosen),
      toggled: open => { if (open) opened.add(item); else opened.delete(item); }});
    row.classList.toggle('selected', item === chosen);
    row.addEventListener('click', () => { chosen = item; render(); });
    return element;
  }
  function render() {
    tree.replaceChildren(...nodes.map(draw));
    const shown = paletteOf(chosen);
    palette.replaceChildren(...(shown ? [node('div', 'Свойства: ' + shown[2], 'ws-reference-view-palette-title'), readOnlyPalette(view.nodePalettes[shown[0]], shown[1])] : []));
  }
  const close = node('button', 'Закрыть');
  close.type = 'button';
  close.addEventListener('click', () => window.close());
  const body = node('div', undefined, 'ws-reference-view-body');
  body.append(tree, palette);
  const actions = node('div', undefined, 'localized-title-actions');
  actions.append(close);
  window.append(node('h3', view.title), body, actions);
  window.addEventListener('close', () => window.remove());
  document.body.append(window);
  render();
  window.show();
  return window;
}

/* askWSDLAddress is the prototype's window «Введите URL импортируемого
   WSDL» (snapshot ws-ssylka/01): one line - an address of the network or a
   path to a file - with OK and «Отмена»; «Импорт WSDL…» opens it with the
   address before. Answers the address, or null when cancelled. */
async function askWSDLAddress(address = '') {
  let typed = address;
  const field = textInput(address, value => { typed = value; }, {placeholder: ''});
  field.classList.add('wsdl-address');
  const answer = await showDialog({title: 'Введите URL импортируемого WSDL', content: field, className: 'wsdl-address-dialog',
    answers: [['ok', 'ОК'], [null, 'Отмена']]});
  return answer === 'ok' ? typed : null;
}

/* importWSDL imports the description at the address the window gives: a new
   reference, or - with source - in place of that reference's description.
   A refusal is shown in the prototype's words and changes nothing. Answers
   the reference as its palette reads it, or null. */
async function importWSDL(source) {
  const address = await askWSDLAddress(source?.reference.locationUrl || '');
  if (address === null) return null;
  try {
    return await postJSON('/api/ws-reference/import', source ? {path: source.path, revision: source.revision, address} : {address});
  } catch (error) {
    await askChoice(String(error.message || error).trim(), '', [['ok', 'ОК']]);
    return null;
  }
}
