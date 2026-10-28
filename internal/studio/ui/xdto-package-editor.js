'use strict';

/* The editor of an XDTO package, as the prototype's (docs/block-00/
   paket-xdto.md, «Окно — редактор пакета», «Узлы редактора»; snapshots
   paket-xdto/02-04, series-2.584/139-178): the window «XDTO пакет <Имя>:
   Пакет», a panel - «Добавить ▾» by the node chosen, «Удалить» asking
   «Удалить элемент?», up and down round the ends, «Проверить пакет» -
   unavailable until the panel «Служебные сообщения» it writes to - and
   «Экспорт XML-схемы…» of the body as the editor holds it (exportXDTOSchema)
   - and the tree of the body of
   the package (source.content, metadata.XDTOContent): the root «Пакет
   (<пространство>)», a folder a kind of node appearing with its first node -
   «Директивы импорта», «Свойства», «Типы значений», «Типы объектов»; under a
   value type «Определения типов», «Образец», «Перечисление»; under an
   object type its properties; under a property its definition of a type.
   The node chosen shows in the palette (source.nodePalettes by its kind),
   every value but a namespace said by ✓ (paletteFields, row.optional). A new
   node is named by the prototype: «Свойство1», «ТипЗначения1»,
   «ТипОбъекта1», «НовыйЭлемент1», an import of
   «http://www.imported-package.org». Every node and every item of
   «Добавить ▾» bears the sign of its kind (xdtoIcons; ICONS.md, «Редактор
   пакета XDTO»). */
function createXDTOPackageEditor(host, onChange, around = {}) {
  return objectWindowEditor(host, onChange, around, {target: source => source.package,
    own: api => {
      // The node chosen, by its path; null - none: the palette is the
      // package's own, as a click on it in the tree of the project shows.
      let chosen = null;
      const content = () => api.source().content;
      const draw = body => {
        const tree = node('div', undefined, 'object-window-tree object-window-tree-tall xdto-tree');
        const root = xdtoTree(content(), api.object());
        const current = chosen && xdtoFind(root, chosen);
        tree.append(xdtoRow(root, current, picked => { chosen = picked.path; api.refresh(); api.palette(); }));
        body.append(xdtoPanel(current || root, content(), api, path => { chosen = path; }), tree);
      };
      const part = () => {
        const root = xdtoTree(content(), api.object()), current = chosen && xdtoFind(root, chosen);
        if (!current || current.kind === 'folder') return null;
        if (current.kind === 'package') {
          const object = api.object(), body = content();
          return {title: current.caption, tabs: api.source().nodePalettes.xdtoPackage, subject: xdtoSubject(key => key === 'namespace' ? object : body)};
        }
        return {title: current.caption, tabs: api.source().nodePalettes[current.palette], subject: current.subject};
      };
      // «Пакет: Открыть» of the palette chooses the root of the tree.
      // A type of a node is chosen among every namespace - XML Schema, SOAP,
      // the platform's, the packages of the project - this package's types
      // as the editor holds them (paket-xdto.md, «Узлы редактора»).
      const xmlTypes = () => ({...api.source().types, [api.object().namespace]: [...(content().valueTypes || []), ...(content().objectTypes || [])]
        .map(item => item.name).filter(Boolean).sort((one, other) => one.toLocaleLowerCase().localeCompare(other.toLocaleLowerCase()))});
      const context = () => ({windows: ['content'], openWindow: () => { chosen = []; api.refresh(); api.palette(); }, xmlTypes,
        typeNamespaces: () => ['http://schemas.xmlsoap.org/soap/envelope/', ...api.source().platformNamespaces, ...Object.keys(xmlTypes())]});
      return {className: 'xdto-window', draw, part, context, reset() { chosen = null; },
        title: () => `XDTO пакет ${api.object().name}: Пакет`,
        value: object => ({...object, content: structuredClone(content())})};
    }});
}

/* xdtoSubject is what the palette of a node edits: the value of a key in
   the object holder(key) answers. A value set to undefined is taken away -
   not said - and false, 0 and '' are values said (row.optional). */
function xdtoSubject(holder) {
  return {target: holder('name'), get: key => holder(key)[key],
    set: (key, value) => { const object = holder(key); if (value === undefined) delete object[key]; else object[key] = value; }};
}

/* xdtoListSubject edits an item of a list of plain values - an import, a
   pattern - by the key of its row. */
function xdtoListSubject(list, index, key) {
  return {target: {}, get: wanted => wanted === key ? list[index] : undefined, set: (wanted, value) => { if (wanted === key) list[index] = value ?? ''; }};
}

/* xdtoIcons are the signs of the nodes by their kind, as the prototype's
   tree and its «Добавить ▾» draw them: a definition of a type under a value
   type is drawn as a value type, under a property as the ball of a
   property with the square below. */
const xdtoIcons = {package: 'xdto-package', import: 'xdto-import', property: 'xdto-property', valueType: 'xdto-value-type',
  objectType: 'xdto-object-type', pattern: 'xdto-pattern', enumeration: 'xdto-enumeration', valueTypeDef: 'xdto-value-type',
  typeDef: 'xdto-property-type-definition'};

/* xdtoTree is the tree of the body of a package: nodes {kind, caption,
   icon, path, children, palette, subject, list, index, adds} - kind package,
   folder, import, property, valueType, objectType, typeDef, pattern,
   enumeration; path the keys and indexes from the body down to it; list
   and index where it stands among its siblings; adds what «Добавить» adds
   under it (xdtoAdds by the container it belongs to). A folder bears the
   sign of what it holds, «Определения типов» a T with two squares. */
function xdtoTree(content, object) {
  const folder = (path, key, caption, items, make, adds, icon) => !items?.length ? [] : [{kind: 'folder', caption, icon, path: [...path, key], adds,
    children: items.map((item, index) => make(item, [...path, key, index], items, index))}];
  const plain = (kind, palette, key, adds) => (item, path, list, index) => ({kind, caption: kind === 'enumeration' ? item.value : item, icon: xdtoIcons[kind], path, list, index, palette,
    subject: kind === 'enumeration' ? xdtoSubject(() => item) : xdtoListSubject(list, index, key), adds, children: []});
  const property = (item, path, list, index) => {
    const own = {kind: 'property', caption: item.name || item.ref?.name || '', icon: xdtoIcons.property, path, list, index, palette: 'xdtoProperty', subject: xdtoSubject(() => item),
      adds: {owner: item, kinds: ['typeDef']}, children: []};
    if (item.typeDef) own.children.push(definition(item.typeDef, [...path, 'typeDef'], own));
    return own;
  };
  const valueAdds = value => ({owner: value, kinds: ['valueTypeDef', 'pattern', 'enumeration']});
  const objectAdds = object => ({owner: object, kinds: ['innerProperty']});
  const valueChildren = (value, path) => [
    ...folder(path, 'typeDefs', 'Определения типов', value.typeDefs, (inner, at, list, index) => ({...definition(inner, at), icon: xdtoIcons.valueTypeDef, list, index}),
      valueAdds(value), 'xdto-type-definitions'),
    ...folder(path, 'patterns', 'Образец', value.patterns, plain('pattern', 'xdtoPattern', 'value', valueAdds(value)), valueAdds(value), xdtoIcons.pattern),
    ...folder(path, 'enumerations', 'Перечисление', value.enumerations, plain('enumeration', 'xdtoEnumeration', 'value', valueAdds(value)), valueAdds(value), xdtoIcons.enumeration),
  ];
  const objectChildren = (object, path) => (object.properties || []).map((inner, index) => property(inner, [...path, 'properties', index], object.properties, index));
  const definition = (inner, path, owner) => inner.object
    ? {kind: 'typeDef', caption: 'Определение типа', icon: xdtoIcons.typeDef, path, palette: 'xdtoObjectType', subject: xdtoSubject(() => inner.object), adds: objectAdds(inner.object),
      owner, children: objectChildren(inner.object, [...path, 'object'])}
    : {kind: 'typeDef', caption: 'Определение типа', icon: xdtoIcons.typeDef, path, palette: 'xdtoValueType', subject: xdtoSubject(() => inner.value ||= {}), adds: valueAdds(inner.value ||= {}),
      owner, children: valueChildren(inner.value, [...path, 'value'])};
  const named = (kind, palette, adds, children) => (item, path, list, index) => ({kind, caption: item.name || '', icon: xdtoIcons[kind], path, list, index, palette,
    subject: xdtoSubject(() => item), adds: adds(item), children: children(item, path)});
  const rootAdds = {owner: content, kinds: ['import', 'property', 'valueType', 'objectType']};
  return {kind: 'package', caption: `Пакет (${object.namespace || ''})`, icon: xdtoIcons.package, path: [], adds: rootAdds, children: [
    ...folder([], 'imports', 'Директивы импорта', content.imports, plain('import', 'xdtoImport', 'namespace', rootAdds), rootAdds, xdtoIcons.import),
    ...folder([], 'properties', 'Свойства', content.properties, property, rootAdds, xdtoIcons.property),
    ...folder([], 'valueTypes', 'Типы значений', content.valueTypes, named('valueType', 'xdtoValueType', valueAdds, valueChildren), rootAdds, xdtoIcons.valueType),
    ...folder([], 'objectTypes', 'Типы объектов', content.objectTypes, named('objectType', 'xdtoObjectType', objectAdds, objectChildren), rootAdds, xdtoIcons.objectType),
  ]};
}

/* xdtoFind is the node of the tree at path, or null. */
function xdtoFind(root, path) {
  const key = JSON.stringify(path);
  const walk = item => JSON.stringify(item.path) === key ? item : item.children.map(walk).find(Boolean) || null;
  return walk(root);
}

/* xdtoRow draws a node of the tree with what lies under it, open, the node
   chosen marked. */
function xdtoRow(item, current, choose) {
  const {element, row} = treeNode({icon: item.icon, caption: item.caption, children: item.children.map(child => xdtoRow(child, current, choose)), open: true});
  row.classList.toggle('selected', item === current);
  row.addEventListener('click', () => choose(item));
  return element;
}

/* xdtoAddable are the nodes «Добавить ▾» offers, in the prototype's order,
   with their captions and how each is made (paket-xdto.md, «Узлы
   редактора»): under the root an import, a property, a value type, an
   object type; under a value type a definition of a type, a pattern, an
   enumeration; under an object type a property; under a property its one
   definition of a type, asked «Выберите тип объекта». Each item bears the
   sign of what it adds (xdtoIcons by the key), «Образец» too, which the
   prototype's menu leaves without one though its tree draws it. */
const xdtoAddable = {
  import: ['Директива импорта', owner => (owner.imports ||= []).push('http://www.imported-package.org') - 1, 'imports'],
  property: ['Свойство', owner => xdtoNamed(owner.properties ||= [], 'Свойство'), 'properties'],
  valueType: ['Тип значения', owner => xdtoNamed(owner.valueTypes ||= [], 'ТипЗначения'), 'valueTypes'],
  objectType: ['Тип объекта', owner => xdtoNamed(owner.objectTypes ||= [], 'ТипОбъекта'), 'objectTypes'],
  innerProperty: ['Свойство', owner => xdtoNamed(owner.properties ||= [], 'Свойство'), 'properties'],
  valueTypeDef: ['Определение типа', owner => (owner.typeDefs ||= []).push({value: {}}) - 1, 'typeDefs'],
  pattern: ['Образец', owner => (owner.patterns ||= []).push(numberedName('НовыйЭлемент', new Set(owner.patterns.map(item => item.toLocaleLowerCase())))) - 1, 'patterns'],
  enumeration: ['Перечисление', owner => (owner.enumerations ||= []).push({value: numberedName('НовыйЭлемент',
    new Set(owner.enumerations.map(item => (item.value || '').toLocaleLowerCase())))}) - 1, 'enumerations'],
};

/* xdtoNamed adds a node named the next number of base among list and
   answers its index. */
function xdtoNamed(list, base) {
  return list.push({name: numberedName(base, new Set(list.filter(item => item.name).map(item => item.name.toLocaleLowerCase())))}) - 1;
}

/* xdtoPanel is the panel over the tree. «Добавить ▾» adds under the
   container of the node chosen and chooses what it added (choose(path));
   a property holding its definition of a type offers it unavailable. */
function xdtoPanel(current, content, api, choose) {
  const adds = current.adds;
  const containerPath = item => item.kind === 'folder' ? item.path.slice(0, -1) : ['package', 'property', 'valueType', 'objectType', 'typeDef'].includes(item.kind) ? item.path : item.path.slice(0, -2);
  const ownerPath = (current.kind === 'typeDef' && current.adds.kinds.includes('innerProperty')) ? [...current.path, 'object']
    : current.kind === 'typeDef' ? [...current.path, 'value'] : containerPath(current);
  const items = () => adds.kinds.map(key => {
    if (key === 'typeDef') return ['Определение типа', async () => {
      const answer = await askChoice('Выберите тип объекта', '', [['object', '{http://v8.1c.ru/8.1/xdto}ObjectType'], ['value', 'Тип значения'], [null, 'Отмена']]);
      if (!answer) return;
      adds.owner.typeDef = answer === 'object' ? {object: {}} : {value: {}};
      choose([...current.path, 'typeDef']);
      api.changed();
    }, !adds.owner.typeDef, undefined, xdtoIcons.typeDef];
    const [caption, make, list] = xdtoAddable[key];
    return [caption, () => {
      const index = make(adds.owner);
      choose([...ownerPath, list, index]);
      api.changed();
    }, true, undefined, xdtoIcons[key === 'innerProperty' ? 'property' : key]];
  });
  const listed = !!current.list;
  const remove = async () => {
    if (await askChoice('Удалить элемент?', '', [['yes', 'Да'], ['no', 'Нет']]) !== 'yes') return;
    if (current.list) current.list.splice(current.index, 1);
    else xdtoRemoveAt(content, current.path);
    choose([]);
    api.changed();
  };
  // A node moves among its siblings round the ends, as the prototype moves
  // every part (moveInCircle).
  const move = by => {
    choose([...current.path.slice(0, -1), moveInCircle(current.list, current.index, by)]);
    api.changed();
  };
  const removable = listed || current.kind === 'typeDef';
  return windowPanel([
    menuButton('Добавить', items, 'object-window-add'),
    panelButton('delete', 'Удалить', removable, remove),
    panelButton('move-up', 'Переместить вверх', listed, () => move(-1)),
    panelButton('move-down', 'Переместить вниз', listed, () => move(1)),
    panelButton('check-package', 'Проверить пакет', false, () => {}),
    panelButton('save', 'Экспорт XML-схемы...', true, () => exportXDTOSchema(api.source().path, content)),
  ]);
}

/* xdtoRemoveAt takes away the value at path - the definition of a type of
   a property - from the body. */
function xdtoRemoveAt(content, path) {
  let holder = content;
  for (const key of path.slice(0, -1)) holder = holder[key];
  delete holder[path.at(-1)];
}

/* exportXDTOSchema is «Экспорт XML-схемы…» of a package (paket-xdto.md,
   «Проверить пакет, экспорт схемы, окно типа»; snapshots series-2.603/26,
   27): the schema of the body given - the editor's, saved or not - or, with
   none, of the body on disk (the command of the tree). A package with
   issues is not exported: «При экспорте схемы XML были обнаружены ошибки.
   Экспорт невозможен»; else the file is saved as the browser saves a
   download, named as the prototype's dialog names it, «Без названия.xsd»,
   and «Экспорт схемы XML успешно завершен». The issues themselves go to
   «Служебные сообщения» with «Проверить пакет». Answers whether the schema
   was written. */
async function exportXDTOSchema(path, content) {
  const answer = await postJSON('/api/xdto-package/schema', {path, content});
  if (!answer) return false;
  if (answer.issues?.length) {
    await showDialog({title: 'При экспорте схемы XML были обнаружены ошибки. Экспорт невозможен', caption: questionCaption, answers: [['ok', 'OK']]});
    return false;
  }
  const link = node('a');
  link.href = `data:application/xml;base64,${answer.schema}`;
  link.download = 'Без названия.xsd';
  link.click();
  await showDialog({title: 'Экспорт схемы XML успешно завершен', caption: questionCaption, answers: [['ok', 'OK']]});
  return true;
}
