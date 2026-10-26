'use strict';

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
  function renderRowsPage(key) {
    const page = node('div', undefined, 'object-window-fields'), fields = fieldsOfPalette(() => cleanups), subject = catalogSubject();
    const paletteRows = (source.palette || []).flatMap(tab => tab.rows || []);
    let frame = null;
    for (const row of source.windowRows[key]) {
      const base = paletteRows.find(item => item.key === row.key);
      if (!base) continue;
      let element;
      if (row.button) {
        element = node('button', row.title, 'object-window-button');
        element.type = 'button';
        element.disabled = base.field === 'window';
        element.addEventListener('click', () => around.openModule?.(row.key));
      } else {
        const control = fields(subject, {...base, title: row.title});
        if (base.field === 'flag') {
          element = node('label', undefined, 'catalog-check object-window-flag');
          element.append(control, node('span', row.title));
        } else element = labeledField(row.title, control, row.above ? 'catalog-field catalog-field-above' : 'catalog-field');
      }
      if (!row.frame) { frame = null; page.append(element); continue; }
      if (frame?.dataset.frame !== row.frame) {
        frame = node('fieldset', undefined, 'studio-frame object-window-fields');
        frame.dataset.frame = row.frame;
        frame.append(node('legend', row.frame));
        page.append(frame);
      }
      frame.append(element);
    }
    return page;
  }
  // «Формы», «Команды», «Макеты»: a tree of the parts with its panel - the
  // forms of the object below the frame of the main forms, the commands
  // above the flag of the standard commands, the templates above the menu
  // «Конструкторы» (spravochnik.md, okno-08, okno-10, okno-11). A form opens
  // by «Изменить», a command - its module, as its own action is; commands
  // and templates move round the ends. Adding and deleting are the items of
  // the constructors of a form, of a command and of a template; a template
  // opens in its editor - each unavailable until its item.
  function partsTree(icon, caption, kind, items, open, move) {
    const inside = chosen?.kind === kind ? chosen : null, on = !!inside;
    const bar = panel([
      panelButton('add', 'Добавить', false, () => {}),
      panelButton('change', 'Изменить', on && !!open, () => open(inside.id)),
      panelButton('delete', 'Удалить', false, () => {}),
      panelButton('move-up', 'Переместить вверх', on && !!move, () => { move(inside.id, -1); rerender(); }),
      panelButton('move-down', 'Переместить вниз', on && !!move, () => { move(inside.id, 1); rerender(); }),
      panelButton('sort', 'Сортировать', false, () => {}),
    ]);
    const tree = node('div', undefined, 'object-window-tree object-window-tree-tall');
    tree.append(treeRow(icon, caption, {kind: `${kind}s`}, items.map(name => {
      const element = treeRow(icon, name, {kind, id: name});
      if (open) element.addEventListener('dblclick', () => open(name));
      return element;
    })));
    return [bar, tree];
  }
  function moveNamed(list) {
    return (name, by) => { const at = list.findIndex(item => item.name === name); if (at >= 0) moveInCircle(list, at, by); };
  }
  function renderFormsPage() {
    const page = renderRowsPage('forms');
    page.append(...partsTree('form', 'Формы', 'form', (source.forms || []).map(form => form.name),
      name => around.openFile?.(source.forms.find(form => form.name === name).path)));
    return page;
  }
  function renderCommandsPage() {
    const page = node('div', undefined, 'object-window-fields'), rows = renderRowsPage('commands');
    const commands = catalog().commands || [];
    page.append(...partsTree('command', 'Команды', 'command', commands.map(command => command.name),
      name => source.commandModules?.[name] && around.openFile?.(source.commandModules[name]), moveNamed(commands)), ...rows.childNodes);
    return page;
  }
  function renderTemplatesPage() {
    const page = node('div', undefined, 'object-window-fields');
    const templates = catalog().templates || [];
    const designers = menuButton('Конструкторы', () => []);
    designers.disabled = true;
    designers.classList.add('object-window-button');
    page.append(...partsTree('template', 'Макеты', 'template', templates.map(template => template.name), null, moveNamed(templates)), designers);
    return page;
  }
  // «Подсистемы»: the subsystems of the project by their parents, a box at
  // each, marked where the catalog is in its content (spravochnik.md, page
  // 2; snapshot okno-02). The marks are written into the subsystems by
  // «Сохранить» with the catalog (SaveCatalogWindow); a subsystem holding a
  // marked one beneath stands open.
  let membership = new Set();
  function renderSubsystemsPage() {
    const page = node('div', undefined, 'object-window-fields');
    const places = source.subsystems || [], under = parent => places.filter(place => (place.parent || null) === parent);
    const holds = place => membership.has(place.id) || under(place.id).some(holds);
    const branch = place => {
      const children = under(place.id).map(branch);
      return treeNode({icon: 'subsystem', caption: place.name, mark: markBox(place.name, membership, place.id, onChange), children,
        open: children.length > 0 && under(place.id).some(holds)}).element;
    };
    const tree = node('div', undefined, 'object-window-tree object-window-tree-tall');
    tree.append(...under(null).map(branch));
    page.append(node('div', 'Подсистемы, в которых участвует справочник:', 'object-window-caption'), tree);
    return page;
  }
  // «Функциональные опции»: the options of the project in a list of boxes
  // with a current row, marked where the catalog itself is in its content
  // (spravochnik.md, page 3; snapshot okno-03); written into the options as
  // the subsystems are.
  let options = new Set(), currentOption = null;
  function renderOptionsPage() {
    const page = node('div', undefined, 'object-window-fields'), places = source.functionalOptions || [];
    let list;
    const draw = () => {
      const fresh = checkList(places.map(place => ({key: place.id, caption: place.name, checked: options.has(place.id)})), currentOption,
        (key, checked) => { if (checked) options.add(key); else options.delete(key); onChange(); draw(); },
        key => { currentOption = key; draw(); });
      fresh.classList.add('object-window-checks');
      if (list) { const focused = list.contains(document.activeElement); list.replaceWith(fresh); if (focused) fresh.focus(); }
      list = fresh;
    };
    draw();
    page.append(node('div', 'Функциональные опции, в которых участвует справочник:', 'object-window-caption'), list);
    return page;
  }
  // «Обмен данными»: the exchange plans of the project, a box and the icon
  // of a plan at each, marked where the plan registers the changes of the
  // catalog, with its autoregistration - «Разрешить» or «Запретить» - beside
  // a marked one; «Отметить все элементы» and «Снять отметку со всех
  // элементов» over them (spravochnik.md, page 14; snapshot okno-14). A plan
  // marked anew registers automatically, as a new object of the plan's own
  // window «Состав» does (plan-obmena.md). Written into the plans as the
  // subsystems are.
  let exchange = new Map(), currentPlan = null;
  // The autoregistration in the prototype's order, each with its picture
  // (snapshot configurator-2026-10-26/series-2.578/03; «Запретить» red - the
  // owner's decision of 26.10.2026).
  const autoRecords = [['deny', 'Запретить', 'auto-record-deny'], ['allow', 'Разрешить', 'auto-record-allow']];
  function renderExchangePage() {
    const page = node('div', undefined, 'object-window-fields'), places = source.exchangePlans || [];
    const mark = (id, marked) => { if (!marked) exchange.delete(id); else if (!exchange.has(id)) exchange.set(id, 'allow'); };
    const changed = () => { onChange(); frame.refresh(); };
    const head = node('div', undefined, 'object-window-list-head');
    head.append(...markAllButtons(marked => { for (const place of places) mark(place.id, marked); changed(); }));
    const table = node('table', undefined, 'object-window-table');
    const caption = node('tr');
    caption.append(node('th', 'План обмена'), node('th', 'Авторегистрация'));
    table.append(caption);
    for (const place of places) {
      const row = node('tr', undefined, place.id === currentPlan ? 'current' : ''), plan = node('td'), auto = node('td');
      const box = node('input');
      box.type = 'checkbox'; box.checked = exchange.has(place.id); box.className = 'object-choice-box';
      box.setAttribute('aria-label', place.name);
      box.addEventListener('change', () => { mark(place.id, box.checked); currentPlan = place.id; changed(); });
      const name = node('div', undefined, 'object-window-table-name');
      name.append(box, iconElement('exchange-plan'), node('span', place.name));
      plan.append(name);
      if (exchange.has(place.id)) {
        auto.append(listCell(autoRecords, exchange.get(place.id), value => { exchange.set(place.id, value); onChange(); frame.refresh(); },
          'Авторегистрация: ' + place.name));
      }
      // A click makes the row current in place: a double click on its cell
      // opens the list of the cell, which a redraw would take away.
      row.addEventListener('click', event => {
        if (event.target.closest('input')) return;
        currentPlan = place.id;
        for (const other of table.querySelectorAll('tr')) other.classList.toggle('current', other === row);
      });
      row.append(plan, auto);
      table.append(row);
    }
    const frameOfTable = node('div', undefined, 'object-window-checks object-window-table-frame');
    frameOfTable.append(table);
    page.append(head, frameOfTable);
    return page;
  }
  // «Ввод на основании»: «Вводится на основании» - the catalog's own list of
  // bases - and «Является основанием для» - the objects whose lists name
  // it, written into them as the subsystems are - each with «Изменить»,
  // the window «Выбор объекта» of the eight kinds, and «Удалить»; below,
  // «Конструктор ввода на основании», unavailable (spravochnik.md, page 12;
  // snapshot okno-12). The catalog in one list is in the other too: it is
  // the basis of itself.
  let basisFor = [], currentBasis = {own: null, other: null};
  function basisName(id) {
    return objectChoiceItem(source.choices?.basedOn, id)?.fullName || (source.basisFor || []).find(place => place.id === id)?.name || id;
  }
  // The objects a list «Является основанием для» names stand in the order of
  // the tree, those it does not offer - the tables of external sources -
  // after them, as the server reads them.
  function inTreeOrder(ids) {
    const order = choiceOrder(source.choices?.basedOn || []), rank = id => order.includes(id) ? order.indexOf(id) : order.length;
    return [...ids].sort((a, b) => rank(a) - rank(b));
  }
  function setBasedOn(own, other) {
    model.set('basedOn', own);
    basisFor = inTreeOrder(other);
    rerender();
  }
  // The catalog itself follows from one list into the other.
  function withSelf(list, holds) {
    const self = catalog().id, rest = list.filter(id => id !== self);
    return holds ? [...rest, self] : rest;
  }
  function renderBasedOnPage() {
    const page = node('div', undefined, 'object-window-fields object-window-based-on'), own = catalog().basedOn || [], self = catalog().id;
    const block = (key, caption, list, set) => {
      if (!list.includes(currentBasis[key])) currentBasis[key] = list[0] ?? null;
      return objectList(caption, list.map(id => [id, basisName(id)]), currentBasis[key], id => { currentBasis[key] = id; frame.refresh(); },
        async () => {
          const chosen = await chooseObjects('Выбор объекта', source.choices?.basedOn || [], list);
          // An object the window does not offer - a table of an external
          // source - stays where it is.
          if (chosen) set([...list.filter(id => !objectChoiceItem(source.choices?.basedOn, id)), ...chosen]);
        },
        id => set(list.filter(item => item !== id)));
    };
    const designer = node('button', 'Конструктор ввода на основании', 'object-window-designer');
    designer.type = 'button'; designer.disabled = true;
    page.append(...block('own', 'Вводится на основании:', own, list => setBasedOn(list, withSelf(basisFor, list.includes(self)))),
      ...block('other', 'Является основанием для:', basisFor, list => setBasedOn(withSelf(own, list.includes(self)), list)), designer);
    return page;
  }
  // «Права»: the roles of the project; the rights the role chosen gives on
  // the catalog, a box at each, marking a right marking the rights it needs
  // and clearing one clearing those needing it; its restrictions of access
  // to data on the right current (spravochnik.md, page 13; snapshot okno-13;
  // rol.md, «Права зависят друг от друга»). Each role is edited by the model
  // of the window of a role over what it says about the catalog alone, and
  // written with the catalog.
  let roleModels = new Map(), currentRole = null;
  const roleRight = {right: null, row: -1};
  const rightCaption = key => source.rightsSchema?.rights?.find(item => item.key === key)?.title || key;
  function renderRightsPage() {
    const page = node('div', undefined, 'object-window-fields object-window-rights'), roles = source.roles || [];
    const object = source.rightsSchema?.objects?.[0], model = roleModels.get(currentRole);
    page.append(node('div', 'Роли:', 'object-window-caption'),
      objectRows(roles.map(entry => [entry.role.id, entry.role.name]), currentRole, id => { currentRole = id; frame.refresh(); }));
    const rights = model && object ? object.rights : [], members = object ? [{object}] : [];
    if (!rights.includes(roleRight.right)) { roleRight.right = rights[0] ?? null; roleRight.row = -1; }
    const redraw = () => { frame.refresh(); host.querySelector('.object-window-rights .check-list')?.focus(); };
    const list = checkList(rights.map(right => ({key: right, caption: rightCaption(right), checked: model.groupMark(members, right) === 'all'})), roleRight.right,
      (right, enabled) => { model.setGroup(members, right, enabled); onChange(); redraw(); },
      right => { roleRight.right = right; roleRight.row = -1; redraw(); });
    page.append(node('div', 'Права:', 'object-window-caption'), list);
    if (model) page.append(roleRestrictionSection({model, object, caption: rightCaption, title: item => localizedTitle(item, source.defaultLanguage),
      state: roleRight, changed: () => { onChange(); frame.refresh(); }}));
    return page;
  }
  const builtPages = {main: renderMainPage, subsystems: renderSubsystemsPage, 'functional-options': renderOptionsPage, exchange: renderExchangePage, 'based-on': renderBasedOnPage, rights: renderRightsPage, hierarchy: renderHierarchyPage, owners: renderOwnersPage, data: renderDataPage,
    numbering: renderNumberingPage, forms: renderFormsPage, commands: renderCommandsPage, templates: renderTemplatesPage};
  function renderPage(key) {
    cleanups.forEach(destroy => destroy());
    cleanups = [];
    if (builtPages[key]) return builtPages[key]();
    return source.windowRows?.[key] ? renderRowsPage(key) : null;
  }

  function clearPalette() {
    paletteCleanups.forEach(destroy => destroy());
    paletteCleanups = [];
    around.palette?.replaceChildren();
    around.paletteTitle?.('Свойства');
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
  // The fields of the palette of the catalog and its parts: what they open,
  // the windows of choice they offer, and how an edit is shown.
  function fieldsOfPalette(collect = () => paletteCleanups) {
    return paletteFields({project: source, typeOffer: model.typeOffer, choices: source.choices, cleanups: collect,
      typed: touch, changed: rerender, openModule: role => around.openModule?.(role), openFile: path => around.openFile?.(path),
      form: name => ({name, path: source.forms?.find(form => form.name === name)?.path}),
      choose: async (row, items) => {
        if (row.key === 'owners') { await chooseOwners(); return null; }
        return await chooseObjects('Выбор объекта', source.choices?.[row.key] || [], items);
      }});
  }
  function renderPalette() {
    if (!around.palette || !source) return;
    clearPalette();
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
    const fields = fieldsOfPalette();
    around.palette.append(objectPalette(tabs || [], row => fields(subject, row), paletteShown(subject)));
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
      const marked = places => new Set((places || []).filter(place => place.member).map(place => place.id));
      membership = marked(source.subsystems);
      options = marked(source.functionalOptions);
      currentOption = source.functionalOptions?.[0]?.id ?? null;
      exchange = new Map((source.exchangePlans || []).filter(place => place.member).map(place => [place.id, place.autoRecord || '']));
      currentPlan = source.exchangePlans?.[0]?.id ?? null;
      basisFor = inTreeOrder(withSelf((source.basisFor || []).map(place => place.id), (source.catalog.basedOn || []).includes(source.catalog.id)));
      currentBasis = {own: null, other: null};
      roleModels = new Map((source.roles || []).map(entry => [entry.role.id, createRoleModel({role: entry.role, schema: source.rightsSchema})]));
      if (!roleModels.has(currentRole)) currentRole = source.roles?.[0]?.role.id ?? null;
      host.hidden = false;
      frame = objectWindow({title: windowTitle(), pages: source.pages || [], chosen: page, page: renderPage,
        actions: () => around.actions?.() || [], close: () => around.close?.(), help: 'object-window'});
      host.replaceChildren(frame.element);
      renderPalette();
    },
    // The catalog and the subsystems and options marked on its pages, in one
    // order so a mark taken back leaves the window unchanged.
    value() { return model && {...model.value(), subsystems: [...membership].sort(), functionalOptions: [...options].sort(),
      basisFor: [...basisFor].sort(),
      roles: (source.roles || []).map(entry => ({id: entry.role.id, revision: entry.revision,
        permission: roleModels.get(entry.role.id).value().objects.find(item => item.object === source.catalog.id) || null})),
      exchangePlans: [...exchange].sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0).map(([id, autoRecord]) => ({id, autoRecord}))}; },
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
