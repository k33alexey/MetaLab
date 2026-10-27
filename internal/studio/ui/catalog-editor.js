'use strict';

/* Catalog editor: fields/checkboxes/lists instead of hand-edited YAML.
   The model mutates the object (a catalog, an exchange plan) in place (no internal clone) so every
   render function can read straight from `source` and stay in sync. */
function createCatalogModel(source, catalog = source.catalog) {
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
  return objectWindowEditor(host, onChange, around, {target: opened => opened.catalog, own: api => referenceWindow(api, onChange, around, host)});
}

/* The window of an exchange plan (docs/block-00/plan-obmena.md, «Окно»;
   snapshots plan-obmena/01, 02, 10-13): a catalog's without «Иерархия»,
   «Владельцы», «Нумерация» and «Обмен данными» - the pages its kind lists -
   the flag of a distributed base and «Состав» on «Основные», no type of
   the code on «Данные». */
function createExchangePlanEditor(host, onChange, around = {}) {
  return objectWindowEditor(host, onChange, around, {target: opened => opened.exchangePlan, own: api => referenceWindow(api, onChange, around, host)});
}

/* referenceWindow is what the window of an object stored as a reference
   with attributes and table parts - a catalog, an exchange plan - adds to
   the window of an object (objectWindowEditor): its own pages, the parts
   chosen in it and their palettes, what its fields open and offer, and the
   commands of the tree inside it. What differs between the kinds is their
   data: the pages the kind lists, the rows of its window and palette. */
function referenceWindow(api, onChange, around, host) {
  let source = null, model = null;
  // The part chosen in the window is the builder's (api.chosen), one for
  // the window's every page.
  const pick = {get chosen() { return api.chosen(); }, set chosen(choice) { api.setChosen(choice); }};
  const catalog = () => api.object();
  const frame = {refresh: () => api.refresh(), show: key => api.show(key)};
  // A structural edit builds the page anew; typing does not - building the
  // field anew would drop its focus - it only renames what shows the name
  // (objectWindowEditor: changed, typed).
  const rerender = () => api.changed(), touch = () => api.typed(), renderPalette = () => api.palette();
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
    page.append(...nameAndSynonym(catalog(), value => model.setName(value), (language, value) => model.setTitle(language, value), undefined, api.cleanups()));
    page.append(labeledField('Комментарий', textInput(catalog().comment, value => { model.setComment(value); touch(); })));
    // The rows a kind puts after the comment - an exchange plan's flag of a
    // distributed base and «Состав» (kinds.WindowRows, «main»).
    if (source.windowRows?.main) page.append(...api.rowsPage('main').childNodes);
    page.append(node('div', undefined, 'object-window-gap'));
    for (const [key, caption, options] of presentations) {
      page.append(labeledField(caption, localized(code => catalog()[key]?.[code], (language, value) => model.setPresentation(key, language, value), api.cleanups(), options),
        'catalog-field catalog-field-above'));
    }
    return page;
  }

  // «Данные»: the lengths, the frames «Тип кода» and «Основное
  // представление», the trees of the attributes and of the table parts, each
  // with its panel, and the buttons of the windows of the standard
  // attributes, characteristics, common attributes and additional indexes -
  // unavailable until their windows are built (spravochnik.md, okno-06).
  const treeRow = api.treeRow, focusPalette = api.focusPalette;
  // The buttons a panel of parts shares (partButtons), acting on what is
  // chosen inside its tree.
  function chosenPartButtons(choice, remove, move) {
    return partButtons(!!choice?.id, {change: () => focusPalette(), remove: () => { remove(); pick.chosen = null; rerender(); },
      move: by => { move(by); rerender(); }});
  }
  function attributeChoice(container, attribute) { return {kind: 'attribute', container, id: attribute.id}; }
  function renderAttributesTree() {
    const folder = {kind: 'attributes'}, inside = pick.chosen?.kind === 'attribute' && pick.chosen.container === 'catalog' ? pick.chosen : null;
    const add = panelButton('add', 'Добавить', true, () => { const attribute = model.addAttribute('catalog'); pick.chosen = attributeChoice('catalog', attribute); rerender(); });
    const bar = windowPanel([add, ...chosenPartButtons(inside, () => model.removeAttribute('catalog', inside.id), by => model.moveAttribute('catalog', inside.id, by))]);
    const tree = node('div', undefined, 'object-window-tree');
    tree.append(treeRow('attribute', 'Реквизиты', folder, catalog().attributes.map(attribute => treeRow('attribute', attribute.name, attributeChoice('catalog', attribute)))));
    return [bar, tree];
  }
  function renderTablePartsTree() {
    const folder = {kind: 'tableparts'};
    // What is chosen inside the tree: a table part, or an attribute of one.
    const part = pick.chosen?.kind === 'tablepart' ? pick.chosen : null, inner = pick.chosen?.kind === 'attribute' && pick.chosen.container !== 'catalog' ? pick.chosen : null;
    const owner = part?.id || inner?.container;
    const addPart = panelButton('add', 'Добавить табличную часть', true, () => { const added = model.addTablePart(); pick.chosen = {kind: 'tablepart', id: added.id}; rerender(); });
    const addAttribute = panelButton('attribute-add', 'Добавить реквизит', !!owner, () => { const attribute = model.addAttribute(owner); pick.chosen = attributeChoice(owner, attribute); rerender(); });
    const buttons = inner
      ? chosenPartButtons(inner, () => model.removeAttribute(inner.container, inner.id), by => model.moveAttribute(inner.container, inner.id, by))
      : chosenPartButtons(part, () => model.removeTablePart(part.id), by => model.moveTablePart(part.id, by));
    const tree = node('div', undefined, 'object-window-tree object-window-tree-tall');
    tree.append(treeRow('tabular-section', 'Табличные части', folder, catalog().tableParts.map(item =>
      treeRow('tabular-section', item.name, {kind: 'tablepart', id: item.id}, item.attributes.map(attribute => treeRow('attribute', attribute.name, attributeChoice(item.id, attribute)))))));
    return [windowPanel([addPart, addAttribute, ...buttons]), tree];
  }
  function renderDataPage() {
    const page = node('div', undefined, 'object-window-fields');
    const code = catalog().code;
    page.append(numberField('Длина кода', code.length, value => model.setCodeLength(value), 'code'));
    page.append(numberField('Длина наименования', catalog().descriptionLength, value => model.setDescriptionLength(value), 'description'));
    const frames = node('div', undefined, 'object-window-frames');
    // «Тип кода» stands where the kind's palette has it: a catalog's, not an
    // exchange plan's (plan-obmena/02).
    if ((source.palette || []).some(tab => tab.rows?.some(row => row.key === 'code.type'))) {
      frames.append(radioFrame('Тип кода', [['number', 'Число'], ['string', 'Строка']], code.type, value => { model.setCodeType(value); rerender(); }, !code.length));
    }
    frames.append(
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
  function objectName(property, id) { return objectChoiceItem(source.choices?.[property], id)?.fullName || id; }
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
    page.append(...objectList('Список владельцев справочника:', owners.map(id => [id, objectName('owners', id)]), currentOwner,
      id => { currentOwner = id; frame.refresh(); }, chooseOwners,
      id => { model.setOwners(owners.filter(owner => owner !== id)); rerender(); }), choiceField('Использование подчинения:', [['to-items', 'Элементам'], ['to-folders', 'Группам'], ['to-folders-and-items', 'Группам и элементам']],
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
  // A page built of the palette's rows (kinds.WindowRows): each row is the
  // palette's field of its property under the window's caption - beside
  // it, or above it - a flag with its box at the right, a module or a
  // window opened by a button, the rows of one frame inside it
  // (spravochnik.md, okno-09, okno-15).
  function renderRowsPage(key) { return api.rowsPage(key); }

  // «Подсистемы»: the subsystems of the project by their parents, a box at
  // each, marked where the catalog is in its content (spravochnik.md, page
  // 2; snapshot okno-02). The marks are written into the subsystems by
  // «Сохранить» with the catalog (SaveCatalogWindow); a subsystem holding a
  // marked one beneath stands open.
  let membership = new Set();
  function renderSubsystemsPage() {
    const page = node('div', undefined, 'object-window-fields');
    page.append(node('div', `Подсистемы, в которых участвует ${source.kindTitle.toLowerCase()}:`, 'object-window-caption'), subsystemsTree(source.subsystems, membership, onChange));
    return page;
  }
  // «Функциональные опции»: the options of the project in a list of boxes
  // with a current row, marked where the catalog itself is in its content
  // (spravochnik.md, page 3; snapshot okno-03); written into the options as
  // the subsystems are.
  let options = new Set();
  function renderOptionsPage() {
    return optionsPage(source.functionalOptions, options, onChange, source.kindTitle.toLowerCase());
  }
  /* What a palette edits: the catalog or a part of it chosen in the window
     (an attribute, a table part), each read and written by the key of a row
     and named by its own rules (paletteFields). */
  function catalogSubject() {
    return {target: catalog(), get: path => model.get(path), set: (path, value) => model.set(path, value),
      setName: value => model.setName(value), check: () => ({path: source.path, name: catalog().name})};
  }
  function attributeSubject(container, attribute, way) {
    return valueSubject(attribute, model.typeOffer, types => model.setAttributeTypes(container, attribute.id, types), {
      owner: container === 'catalog' ? 'catalogs' : 'tablePart',
      setName: value => model.setAttributeName(container, attribute.id, value),
      check: () => ({path: source.path, description: model.value(), at: way})});
  }
  function tablePartSubject(part, way) {
    return {target: part, owner: 'catalogs', get: path => pathValue(part, path), set: (path, value) => setPathValue(part, path, value),
      setName: value => model.setTablePartName(part.id, value), check: () => ({path: source.path, description: model.value(), at: way})};
  }
  // The part chosen in the window and its palette (objectWindowEditor,
  // kind.part): an attribute, a table part, a template; none - the
  // catalog's own palette.
  function part() {
    const parts = catalog().tableParts, partAt = id => parts.findIndex(item => item.id === id);
    if (pick.chosen?.kind === 'attribute') {
      const list = model.attributeList(pick.chosen.container), attribute = list?.find(item => item.id === pick.chosen.id);
      if (!attribute) { pick.chosen = null; return null; }
      const at = ['attributes', String(list.indexOf(attribute))];
      return {title: attribute.name, tabs: source.partPalettes?.attribute,
        subject: attributeSubject(pick.chosen.container, attribute, pick.chosen.container === 'catalog' ? at : ['tableParts', String(partAt(pick.chosen.container)), ...at])};
    }
    if (pick.chosen?.kind === 'tablepart') {
      const item = parts.find(entry => entry.id === pick.chosen.id);
      if (!item) { pick.chosen = null; return null; }
      return {title: item.name, tabs: source.partPalettes?.tablePart, subject: tablePartSubject(item, ['tableParts', String(partAt(item.id))])};
    }
    return null;
  }

  return {
    pages: {main: renderMainPage, subsystems: renderSubsystemsPage, 'functional-options': renderOptionsPage, hierarchy: renderHierarchyPage, owners: renderOwnersPage,
      data: renderDataPage, numbering: renderNumberingPage},
    open(opened) {
      source = opened;
      model = createCatalogModel(source, api.object());
      const marked = places => new Set((places || []).filter(place => place.member).map(place => place.id));
      membership = marked(source.subsystems);
      options = marked(source.functionalOptions);
    },
    subject: () => catalogSubject(),
    part,
    // What the fields of the palette of the catalog and its parts offer:
    // the types, the windows of choice, the forms and files they open.
    context: () => ({typeOffer: model.typeOffer, choices: source.choices, openFile: path => around.openFile?.(path),
      form: name => ({name, path: source.forms?.find(form => form.name === name)?.path}),
      choose: async (row, items) => {
        if (row.key === 'owners') { await chooseOwners(); return null; }
        return await chooseObjects('Выбор объекта', source.choices?.[row.key] || [], items);
      }}),
    // The catalog and the subsystems and options marked on its pages, in one
    // order so a mark taken back leaves the window unchanged; the common
    // pages add theirs (objectWindowEditor).
    value: () => ({...model.value(), subsystems: [...membership].sort(), functionalOptions: [...options].sort()}),
    methods: {
    // Chooses the attribute or table part a node of the tree names
    // (`attribute:<id>` or `tablepart:<id>`) on the page «Данные», so a click
    // on the configuration tree finds it inside the open window.
    select(kind, id) {
      if (!source) return false;
      if (kind === 'attribute' && catalog().attributes.some(item => item.id === id)) pick.chosen = {kind: 'attribute', container: 'catalog', id};
      else if (kind === 'tablepart' && catalog().tableParts.some(item => item.id === id)) pick.chosen = {kind: 'tablepart', id};
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
      if (id.endsWith(':templates')) return {kind: 'template'};
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
      if (context.kind === 'template') { api.addTemplate(); return; }
      if (context.kind === 'tablepart' && !context.container) {
        pick.chosen = {kind: 'tablepart', id: model.addTablePart().id};
      } else {
        const container = context.container || 'catalog', attribute = model.addAttribute(container);
        if (!attribute) return;
        pick.chosen = attributeChoice(container, attribute);
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
      pick.chosen = null;
      rerender();
    },
  }};
}
