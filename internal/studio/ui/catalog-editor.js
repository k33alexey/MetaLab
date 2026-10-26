'use strict';

/* pathValue and setPathValue read and write a property of a description by
   its key, dotted for one inside another - code.length, presentation.mask -
   as the palette names it. A list, a text or a set of translations left
   empty is not written at all. */
function pathValue(object, path) {
  return path.split('.').reduce((value, key) => value?.[key], object);
}
function setPathValue(object, path, value) {
  const keys = path.split('.'), last = keys.pop();
  let target = object;
  for (const key of keys) target = target[key] ||= {};
  const empty = value === '' || (Array.isArray(value) ? !value.length : value && typeof value === 'object' && !Object.keys(value).length);
  if (empty) delete target[last]; else target[last] = value;
}

/* Catalog editor: fields/checkboxes/lists instead of hand-edited YAML.
   The model mutates `source.catalog` in place (no internal clone) so every
   render function can read straight from `source` and stay in sync. */
function createCatalogModel(source) {
  const catalog = source.catalog;
  catalog.attributes ||= [];
  catalog.tableParts ||= [];
  // What the window «Редактирование типа данных» offers for a field the
  // database stores, as the server reads it off the model (TypeOffer).
  const typeOffer = source.typeOffer;
  function attributeList(container) {
    if (container === 'catalog') return catalog.attributes;
    return catalog.tableParts.find(part => part.id === container)?.attributes;
  }
  function attributeOf(container, id) { return attributeList(container)?.find(item => item.id === id); }
  // The names taken in one branch: a new part is named apart from its
  // siblings, the check of a typed name is the server's (watchName).
  function names(list) {
    return new Set(list.map(item => item.name.toLocaleLowerCase()));
  }
  // A text of the object nobody has written - in no language - is not
  // written at all.
  function setText(key, value) {
    if (typeof value === 'string' ? value : Object.keys(value).length) catalog[key] = value; else delete catalog[key];
  }
  return {
    value() { return structuredClone(catalog); },
    typeOffer, attributeList,
    setName(name) { catalog.name = name; },
    setTitle(language, text) { catalog.title = setTranslation(catalog.title, language, text); },
    setComment(text) { setText('comment', text); },
    // A presentation or the explanation, by its key in the description.
    setPresentation(key, language, text) { setText(key, setTranslation(catalog[key], language, text)); },
    setDefaultPresentation(presentation) { catalog.defaultPresentation = presentation; },
    // A setting of the hierarchy: enabled, kind, foldersOnTop, limitLevels,
    // levelCount. Switched on, a hierarchy of no kind becomes one of folders
    // and items and a limit of no count one of two levels - what the
    // prototype's window shows of a catalog that never had one (spravochnik.md,
    // okno-04, «Умолчания»): the model takes neither empty.
    setHierarchy(key, value) {
      const hierarchy = catalog.hierarchy ||= {};
      hierarchy[key] = value;
      if (key === 'enabled' && value && !hierarchy.kind) hierarchy.kind = 'folders-and-items';
      if (key === 'limitLevels' && value && !(hierarchy.levelCount >= 1)) hierarchy.levelCount = 2;
    },
    setOwners(owners) { if (owners.length) catalog.owners = owners; else delete catalog.owners; },
    // A property by its key in the description, dotted for one inside
    // another - code.length, hierarchy.enabled - as the palette names it.
    get(path) { return pathValue(catalog, path); },
    // Sets a property by its key: the settings of the hierarchy by their own
    // rule (setHierarchy), the rest by setPathValue.
    set(path, value) {
      const keys = path.split('.');
      if (keys[0] === 'hierarchy') { this.setHierarchy(keys[1], value); return; }
      setPathValue(catalog, path, value);
    },
    setSubordination(subordination) { catalog.subordination = subordination; },
    setCodeSeries(series) { catalog.code.series = series; },
    setCodeType(type) { catalog.code.type = type; },
    setCodeLength(length) { catalog.code.length = length; },
    setCodeAuto(auto) { catalog.code.auto = auto; },
    setCodeUnique(unique) { catalog.code.unique = unique; },
    setDescriptionLength(length) { catalog.descriptionLength = length; },
    addAttribute(container) {
      const list = attributeList(container);
      if (!list) return null;
      const attribute = {id: crypto.randomUUID(), name: numberedName('Реквизит', names(list)), title: {}, types: [structuredClone(typeOffer.newTypes.string)]};
      list.push(attribute);
      return attribute;
    },
    removeAttribute(container, id) {
      const list = attributeList(container);
      const index = list?.findIndex(item => item.id === id) ?? -1;
      if (index >= 0) list.splice(index, 1);
    },
    moveAttribute(container, id, by) {
      const list = attributeList(container), index = list?.findIndex(item => item.id === id) ?? -1;
      if (index >= 0) moveInCircle(list, index, by);
    },
    setAttributeName(container, id, name) { const attribute = attributeOf(container, id); if (attribute) attribute.name = name; },
    setAttributeTitle(container, id, language, text) { const attribute = attributeOf(container, id); if (attribute) attribute.title = setTranslation(attribute.title, language, text); },
    setAttributeTypes(container, id, types) { const attribute = attributeOf(container, id); if (attribute) attribute.types = types; },
    addTablePart() {
      const part = {id: crypto.randomUUID(), name: numberedName('ТабличнаяЧасть', names(catalog.tableParts)), title: {}, attributes: []};
      catalog.tableParts.push(part);
      return part;
    },
    removeTablePart(id) {
      const index = catalog.tableParts.findIndex(part => part.id === id);
      if (index >= 0) catalog.tableParts.splice(index, 1);
    },
    moveTablePart(id, by) {
      const index = catalog.tableParts.findIndex(part => part.id === id);
      if (index >= 0) moveInCircle(catalog.tableParts, index, by);
    },
    setTablePartName(id, name) { const part = catalog.tableParts.find(item => item.id === id); if (part) part.name = name; },
    setTablePartTitle(id, language, text) {
      const part = catalog.tableParts.find(item => item.id === id);
      if (part) part.title = setTranslation(part.title, language, text);
    },
  };
}

/* The window of a catalog, the prototype's (docs/block-00/spravochnik.md,
   «Окно редактирования»): the frame of every object's window (objectWindow)
   with the pages the kind lists (source.pages). Built so far: «Основные»,
   «Данные» and the two flags of «Нумерация»; a page whose item of the map is
   not done yet is listed and empty. around holds what the window stands
   among: palette - where the properties of the attribute or table part
   chosen in the window stand, as the prototype's palette shows them;
   actions() - the commands of the object's menu «Действия» offers; close() -
   «Закрыть». */
function createCatalogEditor(host, onChange, around = {}) {
  let source, model, frame, chosen = null, cleanups = [], paletteCleanups = [];
  const catalog = () => source.catalog;
  /* Подписи в окне показываются на текущем языке Studio и меняются вместе
     с ним. */
  StudioLanguage.onChange(() => { if (source) { frame.refresh(); renderPalette(); } });
  // A structural edit builds the page anew; typing does not - building the
  // field anew would drop its focus - it only renames what shows the name.
  function rerender() { onChange(); frame.refresh(); renderPalette(); }
  // An edit in the palette draws the window anew and one in the window the
  // palette: each shows what the other typed, and the one typed into keeps
  // its focus. An edit of neither - in the window «Строки на разных языках»
  // - draws both.
  function touch() {
    onChange();
    frame.setTitle(windowTitle());
    const inPalette = !!around.palette?.contains(document.activeElement), inWindow = host.contains(document.activeElement);
    if (!inWindow) frame.refresh();
    if (!inPalette) renderPalette();
  }
  function windowTitle() { return `${source.kindTitle} ${catalog().name}`; }
  function localized(get, set, collect, options = titleTextOptions) {
    const field = createLocalizedField(source, get, set, touch, options);
    collect.push(field.destroy);
    return field;
  }
  // The field of a name: at is the way to the part in the catalog, none for
  // the catalog's own name.
  function nameField(value, apply, at) {
    return labeledField('Имя', nameInput(value, value => { apply(value); touch(); },
      () => at ? {path: source.path, description: model.value(), at} : {path: source.path, name: catalog().name}));
  }
  // The name of the catalog or of a part and its synonym, which follows the
  // name while it is not put right by hand (field.follow).
  function nameAndSynonym(target, setName, setTitle, at, collect) {
    const synonym = localized(code => target.title?.[code], setTitle, collect);
    const name = nameField(target.name, value => { const before = target.name; setName(value); synonym.follow(before, value); }, at);
    return [name, labeledField('Синоним', synonym)];
  }
  function numberField(label, value, apply, field) {
    return labeledField(label, textInput(value, value => { apply(value); rerender(); }, limited(field, {type: 'number', min: 0})));
  }

  // «Основные»: the name, the synonym and the comment; below them the four
  // presentations and the explanation, each caption above its field, each a
  // text in every language (spravochnik.md, okno-01).
  const presentations = [
    ['objectPresentation', 'Представление объекта'], ['extendedObjectPresentation', 'Расширенное представление объекта'],
    ['listPresentation', 'Представление списка'], ['extendedListPresentation', 'Расширенное представление списка'],
    ['explanation', 'Пояснение', {kind: 'area', rows: 3}],
  ];
  function renderMainPage() {
    const page = node('div', undefined, 'object-window-fields');
    page.append(...nameAndSynonym(catalog(), value => model.setName(value), (language, value) => model.setTitle(language, value), undefined, cleanups));
    page.append(labeledField('Комментарий', textInput(catalog().comment, value => { model.setComment(value); touch(); })));
    page.append(node('div', undefined, 'object-window-gap'));
    for (const [key, caption, options] of presentations) {
      page.append(labeledField(caption, localized(code => catalog()[key]?.[code], (language, value) => model.setPresentation(key, language, value), cleanups, options),
        'catalog-field catalog-field-above'));
    }
    return page;
  }

  // «Данные»: the lengths, the frames «Тип кода» and «Основное
  // представление», the trees of the attributes and of the table parts, each
  // with its panel, and the buttons of the windows of the standard
  // attributes, characteristics, common attributes and additional indexes -
  // unavailable until their windows are built (spravochnik.md, okno-06).
  function same(left, right) { return !!left && !!right && left.kind === right.kind && left.container === right.container && left.id === right.id; }
  function choose(choice) { chosen = choice; frame.refresh(); renderPalette(); }
  function treeRow(icon, caption, choice, children = []) {
    const {element, row} = treeNode({icon, caption, children, open: true});
    row.classList.toggle('selected', same(chosen, choice));
    row.addEventListener('click', () => choose(choice));
    if (choice.id) row.addEventListener('dblclick', event => { event.stopPropagation(); choose(choice); focusPalette(); });
    return element;
  }
  function panelButton(icon, title, enabled, act) {
    const button = iconButton(icon, title);
    button.disabled = !enabled;
    button.addEventListener('click', act);
    return button;
  }
  // The buttons a panel shares: change, delete, up and down act on what is
  // chosen inside its tree, sorting waits for the window «Сортировка
  // метаданных» (the item of the map «порядок объектов»).
  function partButtons(choice, remove, move) {
    const on = !!choice?.id;
    return [
      panelButton('change', 'Изменить', on, () => focusPalette()),
      panelButton('delete', 'Удалить', on, async () => { if (await askDeleteObject()) { remove(); chosen = null; rerender(); } }),
      panelButton('move-up', 'Переместить вверх', on, () => { move(-1); rerender(); }),
      panelButton('move-down', 'Переместить вниз', on, () => { move(1); rerender(); }),
      panelButton('sort', 'Сортировать', false, () => {}),
    ];
  }
  function panel(buttons) {
    const bar = node('div', undefined, 'object-window-panel');
    bar.append(...buttons);
    return bar;
  }
  function attributeChoice(container, attribute) { return {kind: 'attribute', container, id: attribute.id}; }
  function renderAttributesTree() {
    const folder = {kind: 'attributes'}, inside = chosen?.kind === 'attribute' && chosen.container === 'catalog' ? chosen : null;
    const add = panelButton('add', 'Добавить', true, () => { const attribute = model.addAttribute('catalog'); chosen = attributeChoice('catalog', attribute); rerender(); });
    const bar = panel([add, ...partButtons(inside, () => model.removeAttribute('catalog', inside.id), by => model.moveAttribute('catalog', inside.id, by))]);
    const tree = node('div', undefined, 'object-window-tree');
    tree.append(treeRow('attribute', 'Реквизиты', folder, catalog().attributes.map(attribute => treeRow('attribute', attribute.name, attributeChoice('catalog', attribute)))));
    return [bar, tree];
  }
  function renderTablePartsTree() {
    const folder = {kind: 'tableparts'};
    // What is chosen inside the tree: a table part, or an attribute of one.
    const part = chosen?.kind === 'tablepart' ? chosen : null, inner = chosen?.kind === 'attribute' && chosen.container !== 'catalog' ? chosen : null;
    const owner = part?.id || inner?.container;
    const addPart = panelButton('add', 'Добавить табличную часть', true, () => { const added = model.addTablePart(); chosen = {kind: 'tablepart', id: added.id}; rerender(); });
    const addAttribute = panelButton('attribute-add', 'Добавить реквизит', !!owner, () => { const attribute = model.addAttribute(owner); chosen = attributeChoice(owner, attribute); rerender(); });
    const buttons = inner
      ? partButtons(inner, () => model.removeAttribute(inner.container, inner.id), by => model.moveAttribute(inner.container, inner.id, by))
      : partButtons(part, () => model.removeTablePart(part.id), by => model.moveTablePart(part.id, by));
    const tree = node('div', undefined, 'object-window-tree object-window-tree-tall');
    tree.append(treeRow('tabular-section', 'Табличные части', folder, catalog().tableParts.map(item =>
      treeRow('tabular-section', item.name, {kind: 'tablepart', id: item.id}, item.attributes.map(attribute => treeRow('attribute', attribute.name, attributeChoice(item.id, attribute)))))));
    return [panel([addPart, addAttribute, ...buttons]), tree];
  }
  function renderDataPage() {
    const page = node('div', undefined, 'object-window-fields');
    const code = catalog().code;
    page.append(numberField('Длина кода', code.length, value => model.setCodeLength(value), 'code'));
    page.append(numberField('Длина наименования', catalog().descriptionLength, value => model.setDescriptionLength(value), 'description'));
    const frames = node('div', undefined, 'object-window-frames');
    frames.append(
      radioFrame('Тип кода', [['number', 'Число'], ['string', 'Строка']], code.type, value => { model.setCodeType(value); rerender(); }, !code.length),
      radioFrame('Основное представление', [['as-code', 'В виде кода'], ['as-description', 'В виде наименования']], catalog().defaultPresentation,
        value => { model.setDefaultPresentation(value); rerender(); }));
    page.append(frames, ...renderAttributesTree(), ...renderTablePartsTree());
    const windows = node('div', undefined, 'object-window-buttons');
    for (const caption of ['Стандартные реквизиты', 'Характеристики', 'Общие реквизиты', 'Дополнительные индексы']) {
      const button = node('button', caption);
      button.type = 'button'; button.disabled = true;
      windows.append(button);
    }
    page.append(windows);
    return page;
  }

  function flag(caption, checked, apply, disabled) {
    return checkboxField(caption, checked, value => { apply(value); rerender(); }, 'catalog-check object-window-flag', disabled);
  }
  function choiceField(caption, choices, value, apply, disabled, className = 'catalog-field catalog-field-above') {
    const select = selectInput(choices, value || '', value => { apply(value); rerender(); }, caption);
    select.disabled = disabled;
    return labeledField(caption, select, className);
  }

  // «Иерархия»: the flag «Иерархический справочник» and under it the kind,
  // folders on top, the limit and the count of levels - unavailable without
  // the flag, the count without the limit, each showing its value
  // (spravochnik.md, okno-04; snapshot ierarhiya-vklyuchena).
  function renderHierarchyPage() {
    const page = node('div', undefined, 'object-window-fields'), hierarchy = catalog().hierarchy || {}, off = !hierarchy.enabled;
    const inner = node('div', undefined, 'object-window-fields object-window-inner');
    inner.append(
      choiceField('Вид иерархии:', [['folders-and-items', 'Иерархия групп и элементов'], ['items', 'Иерархия элементов']], hierarchy.kind,
        value => model.setHierarchy('kind', value), off),
      flag('Размещать группы сверху', !!hierarchy.foldersOnTop, value => model.setHierarchy('foldersOnTop', value), off),
      flag('Ограничение количества уровней иерархии', !!hierarchy.limitLevels, value => model.setHierarchy('limitLevels', value), off),
      labeledField('Количество уровней иерархии', textInput(hierarchy.levelCount ?? 0, value => { model.setHierarchy('levelCount', value); rerender(); },
        limited('levels', {type: 'number', min: 1, disabled: off || !hierarchy.limitLevels}))));
    page.append(flag('Иерархический справочник', !!hierarchy.enabled, value => model.setHierarchy('enabled', value)), inner);
    return page;
  }

  // «Владельцы»: the list of owners by their full names, «Изменить» opening
  // the window «Выбор объекта» of the five kinds that may own a catalog and
  // «Удалить» taking the owner chosen in the list away; the use of
  // subordination below, unavailable without owners (spravochnik.md,
  // okno-05). The owners chosen are taken only when the model takes them:
  // the server answers the prototype's refusal - predefined items, the
  // catalog itself, a ring - and nothing is written (snapshots vladelcy-otkaz-*).
  let currentOwner = null;
  // The full name of an object the window «Выбор объекта» of the property
  // offers, Справочник.Контрагенты.
  function objectName(property, id) {
    const find = branches => { for (const branch of branches || []) { const item = branch.objects?.find(object => object.id === id) || find(branch.children); if (item) return item; } return null; };
    return find(source.choices?.[property])?.fullName || id;
  }
  async function chooseOwners() {
    const owners = await chooseObjects('Выбор объекта', source.choices?.owners || [], catalog().owners || []);
    if (!owners) return;
    let answer;
    try { answer = await postJSON(`${source.editor}/owners`, {path: source.path, object: {...model.value(), owners}}); } catch (error) { answer = {refusal: String(error.message || error)}; }
    if (answer.refusal) { await askChoice(answer.refusal, '', [['ok', 'ОК']]); return; }
    model.setOwners(owners);
    rerender();
  }
  function renderOwnersPage() {
    const page = node('div', undefined, 'object-window-fields object-window-owners'), owners = catalog().owners || [];
    if (!owners.includes(currentOwner)) currentOwner = owners[0] ?? null;
    const head = node('div', undefined, 'object-window-list-head');
    const change = iconButton('change', 'Изменить'), remove = iconButton('delete', 'Удалить');
    change.addEventListener('click', chooseOwners);
    remove.disabled = !owners.length;
    remove.addEventListener('click', () => { model.setOwners(owners.filter(id => id !== currentOwner)); rerender(); });
    head.append(node('span', 'Список владельцев справочника:'), change, remove);
    const list = node('div', undefined, 'object-window-list');
    list.setAttribute('role', 'listbox');
    for (const id of owners) {
      const row = node('div', objectName('owners', id), 'object-window-list-row' + (id === currentOwner ? ' current' : ''));
      row.setAttribute('role', 'option');
      row.setAttribute('aria-selected', String(id === currentOwner));
      row.addEventListener('click', () => { currentOwner = id; frame.refresh(); });
      row.addEventListener('dblclick', chooseOwners);
      list.append(row);
    }
    page.append(head, list, choiceField('Использование подчинения:', [['to-items', 'Элементам'], ['to-folders', 'Группам'], ['to-folders-and-items', 'Группам и элементам']],
      catalog().subordination, value => model.setSubordination(value), !owners.length));
    return page;
  }

  // «Нумерация»: the two flags and the frame «Серии кодов», all unavailable
  // at a code of no length (spravochnik.md, okno-07; snapshot
  // novyy-numeraciya).
  function renderNumberingPage() {
    const page = node('div', undefined, 'object-window-fields'), code = catalog().code, off = !code.length;
    page.append(
      flag('Автонумерация', !!code.auto, value => model.setCodeAuto(value), off),
      flag('Контроль уникальности', !!code.unique, value => model.setCodeUnique(value), off),
      radioFrame('Серии кодов', [['whole', 'Во всем справочнике'], ['within-subordination', 'В пределах подчинения'],
        ['within-owner-subordination', 'В пределах подчинения владельцу']], code.series, value => { model.setCodeSeries(value); rerender(); }, off));
    return page;
  }
  const builtPages = {main: renderMainPage, hierarchy: renderHierarchyPage, owners: renderOwnersPage, data: renderDataPage, numbering: renderNumberingPage};
  function renderPage(key) {
    cleanups.forEach(destroy => destroy());
    cleanups = [];
    return builtPages[key]?.() || null;
  }

  function clearPalette() {
    paletteCleanups.forEach(destroy => destroy());
    paletteCleanups = [];
    around.palette?.replaceChildren();
    around.paletteTitle?.('Свойства');
  }
  /* What a palette edits: the catalog or a part of it chosen in the window
     (an attribute, a table part), each read and written by the key of a row
     and named by its own rules. owner and type are what a row of a part is
     shown by (kinds.PaletteRow.Shown). */
  function catalogSubject() {
    return {target: catalog(), get: path => model.get(path), set: (path, value) => model.set(path, value),
      setName: value => model.setName(value), check: () => ({path: source.path, name: catalog().name})};
  }
  // The kind of the one type of an attribute as its palette tells them
  // apart - string, number, reference - or composite.
  function typeClass(types) {
    if (types?.length !== 1) return 'composite';
    return types[0].reference ? 'reference' : types[0].kind;
  }
  // A qualifier of the one type of an attribute, read off it as the window
  // of the type shows it.
  function qualifierOf(types, field) {
    const type = types?.[0] || {};
    return field === 'unlimited' ? !type.length : type[field];
  }
  function attributeSubject(container, attribute, way) {
    return {target: attribute, owner: container === 'catalog' ? 'catalogs' : 'tablePart', type: typeClass(attribute.types),
      get: path => path.startsWith('type.') ? qualifierOf(attribute.types, path.slice(5)) : pathValue(attribute, path),
      // A qualifier is written by the model of the window of the type, by
      // its rules and ceilings: a value beyond one is refused (false).
      set(path, value) {
        if (!path.startsWith('type.')) { setPathValue(attribute, path, value); return true; }
        const types = typeEditorModel(model.typeOffer, attribute.types);
        if (types.setQualifier(path.slice(5), value) === false) return false;
        model.setAttributeTypes(container, attribute.id, types.result().types);
        return true;
      },
      setName: value => model.setAttributeName(container, attribute.id, value),
      check: () => ({path: source.path, description: model.value(), at: way})};
  }
  function tablePartSubject(part, way) {
    return {target: part, owner: 'catalogs', get: path => pathValue(part, path), set: (path, value) => setPathValue(part, path, value),
      setName: value => model.setTablePartName(part.id, value), check: () => ({path: source.path, description: model.value(), at: way})};
  }
  // A row stands where one of the conditions it is shown by holds.
  function shownFor(subject) {
    return row => !row.shown?.length || row.shown.some(condition => condition === `type=${subject.type}` || condition === `owner=${subject.owner}`);
  }
  // The field of a row of the palette by its kind (kinds.PaletteRow): the
  // name with the synonym following it, a text in every language, a plain
  // text, «Открыть» of a module or a window, a flag, a list, a number, a
  // switch, a list of objects or fields, a main form, the type of a part, a
  // value; null for a row the palette has no field for.
  function paletteControl(subject, row) {
    const target = subject.target;
    if (row.field === 'name') {
      return nameInput(target.name, value => { const before = target.name; subject.setName(value); paletteSynonym?.follow(before, value); touch(); }, subject.check);
    }
    if (row.field === 'localized') {
      const field = localized(code => subject.get(row.key)?.[code], (language, value) => subject.set(row.key, setTranslation({...subject.get(row.key)}, language, value)),
        paletteCleanups, row.lines ? {kind: 'area', rows: row.lines} : titleTextOptions);
      if (row.key === 'title') paletteSynonym = field;
      return field;
    }
    if (row.field === 'text') return textInput(subject.get(row.key), value => { subject.set(row.key, value); touch(); });
    if (row.field === 'module' || row.field === 'window') {
      const link = node('button', 'Открыть', 'object-palette-link');
      link.type = 'button';
      // The windows of the standard attributes, characteristics,
      // predefined items, additional indexes, the help and the choice of a
      // field are items of their own.
      link.disabled = row.field === 'window';
      link.addEventListener('click', () => around.openModule?.(row.key));
      return link;
    }
    if (row.field === 'type') {
      const shown = node('input');
      shown.readOnly = true;
      shown.value = typePresentation(model.typeOffer, target.types);
      const pick = fieldChoiceButton('Редактирование типа данных', async () => {
        const types = await showTypeEditor(model.typeOffer, target.types);
        if (types) { subject.set('types', types); rerender(); }
      });
      return fieldWithButtons(shown, [pick]);
    }
    const value = subject.get(row.key), off = !available(subject, row);
    const set = next => { if (subject.set(row.key, next) === false) return false; rerender(); };
    let control;
    if (row.field === 'flag') {
      control = node('input');
      control.type = 'checkbox'; control.checked = !!value;
      control.addEventListener('change', () => set(control.checked));
    } else if (row.field === 'choice') {
      control = selectInput(row.choices, value || row.default || '', set, row.title);
    } else if (row.field === 'switch') {
      control = selectInput(row.choices, String(!!value), next => set(next === 'true'), row.title);
    } else if (row.field === 'number') {
      const options = {type: 'number', min: row.min || 0, ...(row.max ? {max: row.max} : {})};
      control = textInput(value ?? 0, set, row.limit ? limited(row.limit, options) : options);
    } else if (row.field === 'value') {
      // A value of the part's type is entered by a field of that type,
      // which the inventory does not show yet: shown, not edited (В8).
      control = node('input');
      control.readOnly = true; control.value = value?.data ?? '';
      control.disabled = true;
      return control;
    } else if (row.field === 'objects' || row.field === 'fields') {
      return listControl(subject, row, value, off);
    } else if (row.field === 'form') {
      return formControl(row, value || '', off);
    } else return null;
    control.disabled = off;
    return control;
  }
  // A row is available while every property it needs is on: not false,
  // zero or empty, or the value named (kinds.PaletteRow.Needs).
  function available(subject, row) {
    return (row.needs || []).every(need => {
      const [path, wanted] = need.split('='), value = subject.get(path);
      if (wanted !== undefined) return value === wanted;
      return Array.isArray(value) ? value.length > 0 : !!value;
    });
  }
  // A main form, as the palette shows it: its name, «…» choosing it among
  // the forms - its window the inventory does not show, unavailable until
  // shot - the cross clearing it and the magnifier opening it.
  function formControl(row, name, off) {
    const shown = node('input');
    shown.readOnly = true; shown.value = name; shown.disabled = off;
    const pick = fieldChoiceButton(row.title, () => {});
    pick.disabled = true;
    const clear = clearButton(`Очистить: ${row.title}`, () => { model.set(row.key, ''); rerender(); });
    clear.classList.add('field-button');
    clear.disabled = off || !name;
    const open = fieldButton('field-translations', `Открыть: ${row.title}`, () => around.openFile?.(source.path.replace(/object\.yaml$/, `forms/${name}/form.yaml`)));
    open.disabled = off || !name;
    return fieldWithButtons(shown, [pick, clear, open]);
  }
  // A list of objects or of fields in one line, as the palette shows it:
  // their names by commas, «…» choosing them and the cross clearing them.
  // The owners are chosen as on the page «Владельцы» - with the model's
  // refusal; the bases plainly; the fields - of the data lock, of the input
  // by string, the link by type - wait for their window, which the inventory
  // does not show.
  function listControl(subject, row, value, off) {
    const items = Array.isArray(value) ? value : value ? [value] : [];
    const shown = node('input');
    shown.readOnly = true;
    shown.value = row.field === 'objects' ? items.map(id => objectName(row.key, id)).join(', ')
      : items.map(field => field.standard || field.attribute || [].concat(field.source || []).join('.')).join(', ');
    const pick = fieldChoiceButton(row.title, async () => {
      if (row.key === 'owners') { await chooseOwners(); return; }
      const chosen = await chooseObjects('Выбор объекта', source.choices?.[row.key] || [], items);
      if (chosen) { subject.set(row.key, chosen); rerender(); }
    });
    pick.disabled = off || row.field === 'fields';
    const clear = clearButton(`Очистить: ${row.title}`, () => { subject.set(row.key, []); rerender(); });
    clear.classList.add('field-button');
    clear.disabled = off || !items.length;
    shown.disabled = off;
    return fieldWithButtons(shown, [pick, clear]);
  }
  let paletteSynonym = null;
  function renderPalette() {
    if (!around.palette || !source) return;
    clearPalette();
    paletteSynonym = null;
    const parts = catalog().tableParts, partAt = id => parts.findIndex(item => item.id === id);
    let subject, tabs, title;
    if (chosen?.kind === 'attribute') {
      const list = model.attributeList(chosen.container), attribute = list?.find(item => item.id === chosen.id);
      if (!attribute) { chosen = null; return; }
      const at = ['attributes', String(list.indexOf(attribute))];
      subject = attributeSubject(chosen.container, attribute, chosen.container === 'catalog' ? at : ['tableParts', String(partAt(chosen.container)), ...at]);
      tabs = source.partPalettes?.attribute; title = attribute.name;
    } else if (chosen?.kind === 'tablepart') {
      const part = parts.find(item => item.id === chosen.id);
      if (!part) { chosen = null; return; }
      subject = tablePartSubject(part, ['tableParts', String(partAt(part.id))]);
      tabs = source.partPalettes?.tablePart; title = part.name;
    } else {
      subject = catalogSubject(); tabs = source.palette; title = catalog().name;
    }
    around.paletteTitle?.(`Свойства: ${title}`);
    around.palette.append(objectPalette(tabs || [], row => paletteControl(subject, row), shownFor(subject)));
  }
  function focusPalette() { around.palette?.querySelector('input')?.focus(); }

  return hostedEditor(host, () => { cleanups.forEach(destroy => destroy()); cleanups = []; if (source) clearPalette(); source = null; model = null; }, {
    // Opened anew after «Сохранить» (keep), the window stays on its page
    // with the same part chosen.
    open(value, keep = false) {
      const page = keep && frame ? frame.current() : 'main';
      if (source) clearPalette();
      if (!keep) chosen = null;
      source = structuredClone(value);
      model = createCatalogModel(source);
      host.hidden = false;
      frame = objectWindow({title: windowTitle(), pages: source.pages || [], chosen: page, page: renderPage,
        actions: () => around.actions?.() || [], close: () => around.close?.(), help: 'object-window'});
      host.replaceChildren(frame.element);
      renderPalette();
    },
    value() { return model?.value(); },
    // Shows the palette of the catalog itself, as a click on its node of the
    // tree does: nothing in the window stays chosen. Answers whether the
    // palette is the window's.
    showPalette() {
      if (!source || !around.palette) return false;
      chosen = null;
      frame.refresh();
      renderPalette();
      return true;
    },
    // Chooses the attribute or table part a node of the tree names
    // (`attribute:<id>` or `tablepart:<id>`) on the page «Данные», so a click
    // on the configuration tree finds it inside the open window.
    select(kind, id) {
      if (!source) return false;
      if (kind === 'attribute' && catalog().attributes.some(item => item.id === id)) chosen = {kind: 'attribute', container: 'catalog', id};
      else if (kind === 'tablepart' && catalog().tableParts.some(item => item.id === id)) chosen = {kind: 'tablepart', id};
      else return false;
      frame.show('data');
      renderPalette();
      return true;
    },
    // What «Добавить» on a node of the tree inside this catalog adds:
    // Реквизиты or an attribute - a sibling attribute in the same container
    // (the catalog, or the enclosing table part); Табличные части - a table
    // part; a table part - an attribute inside it. None for another node.
    addContextOf(node) {
      const id = node.id || '', fragment = node.fragment || '';
      if (id.endsWith(':attributes')) return {kind: 'attribute', container: 'catalog'};
      if (id.endsWith(':table-parts')) return {kind: 'tablepart'};
      if (fragment.startsWith('attribute:')) return {kind: 'attribute', container: 'catalog'};
      if (fragment.startsWith('tablepart:')) return {kind: 'attribute', container: fragment.slice('tablepart:'.length)};
      return null;
    },
    // What «Удалить» on a node of the tree inside this catalog deletes: an
    // attribute of the catalog or of a table part, or a table part.
    deleteContextOf(node) {
      const id = node.id || '', fragment = node.fragment || '';
      if (fragment.startsWith('attribute:')) return {kind: 'attribute', container: 'catalog', id: fragment.slice('attribute:'.length)};
      if (!fragment.startsWith('tablepart:')) return null;
      if (!id.includes(':')) return {kind: 'tablepart', id};
      const [container, attribute] = id.split(':');
      return {kind: 'attribute', container, id: attribute};
    },
    // Adds an attribute or a table part by «Добавить» of the tree, where the
    // tree's selection puts it (addContextOf), as the window's own panels
    // add them, and chooses it on the page «Данные».
    addFromTreeContext(context) {
      if (!source) return;
      if (context.kind === 'tablepart' && !context.container) {
        chosen = {kind: 'tablepart', id: model.addTablePart().id};
      } else {
        const container = context.container || 'catalog', attribute = model.addAttribute(container);
        if (!attribute) return;
        chosen = attributeChoice(container, attribute);
      }
      frame.show('data');
      rerender();
    },
    // Deletes an attribute or a table part by «Удалить» of the tree, which
    // has asked its question already.
    deleteFromTreeContext(context) {
      if (!source) return;
      if (context.kind === 'tablepart') model.removeTablePart(context.id);
      else if (context.kind === 'attribute') model.removeAttribute(context.container, context.id);
      else return;
      chosen = null;
      rerender();
    },
  });
}
