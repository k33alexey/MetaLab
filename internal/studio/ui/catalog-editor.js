'use strict';

/* Catalog editor: fields/checkboxes/lists instead of hand-edited YAML.
   The model mutates `source.catalog` in place (no internal clone) so every
   render function can read straight from `source` and stay in sync. */
function createCatalogModel(source) {
  const catalog = source.catalog;
  catalog.attributes ||= [];
  catalog.tableParts ||= [];
  // The platform indexes a field in one of three ways, not two. What the third
  // one adds over the second its help does not say, so it is offered, stored
  // and carried on unchanged.
  const indexingModes = [
    ['dont-index', 'Не индексировать'], ['index', 'Индексировать'],
    ['index-with-additional-order', 'Индексировать с доп. упорядочиванием'],
  ];
  // What the window «Редактирование типа данных» offers for a field the
  // database stores, as the server reads it off the model (TypeOffer).
  const typeOffer = source.typeOffer;
  function attributeList(container) {
    if (container === 'catalog') return catalog.attributes;
    return catalog.tableParts.find(part => part.id === container)?.attributes;
  }
  // The names taken in one branch: a new part is named apart from its
  // siblings, the check of a typed name is the server's (watchName).
  function names(list) {
    return new Set(list.map(item => item.name.toLocaleLowerCase()));
  }
  return {
    value() { return structuredClone(catalog); },
    typeOffer, indexingModes, attributeList,
    setName(name) { catalog.name = name; },
    setTitle(language, text) { catalog.title = setTranslation(catalog.title, language, text); },
    setCodeType(type) { catalog.code.type = type; },
    setCodeLength(length) { catalog.code.length = length; },
    setCodeAuto(auto) { catalog.code.auto = auto; },
    setCodeUnique(unique) { catalog.code.unique = unique; },
    setDescriptionLength(length) { catalog.descriptionLength = length; },
    addAttribute(container) {
      const list = attributeList(container);
      if (!list) return null;
      const attribute = {id: crypto.randomUUID(), name: uniqueName('НовыйРеквизит', names(list)), title: {[source.defaultLanguage]: 'Новый реквизит'}, types: [structuredClone(typeOffer.newTypes.string)]};
      list.push(attribute);
      return attribute;
    },
    removeAttribute(container, id) {
      const list = attributeList(container);
      const index = list?.findIndex(item => item.id === id) ?? -1;
      if (index >= 0) list.splice(index, 1);
    },
    setAttributeName(container, id, name) { const attribute = attributeList(container)?.find(item => item.id === id); if (attribute) attribute.name = name; },
    setAttributeTitle(container, id, language, text) {
      const attribute = attributeList(container)?.find(item => item.id === id);
      if (attribute) attribute.title = setTranslation(attribute.title, language, text);
    },
    setAttributeFillChecking(container, id, checked) {
      const attribute = attributeList(container)?.find(item => item.id === id);
      if (!attribute) return;
      if (checked) attribute.fillChecking = 'show-error'; else delete attribute.fillChecking;
    },
    setAttributeIndexing(container, id, indexing) {
      const attribute = attributeList(container)?.find(item => item.id === id);
      if (!attribute) return;
      if (indexing === 'dont-index') delete attribute.indexing; else attribute.indexing = indexing;
    },
    setAttributeTypes(container, id, types) {
      const attribute = attributeList(container)?.find(item => item.id === id);
      if (attribute) attribute.types = types;
    },
    addTablePart() {
      const part = {id: crypto.randomUUID(), name: uniqueName('НоваяТабличнаяЧасть', names(catalog.tableParts)), title: {[source.defaultLanguage]: 'Новая табличная часть'}, attributes: []};
      catalog.tableParts.push(part);
      return part;
    },
    removeTablePart(id) {
      const index = catalog.tableParts.findIndex(part => part.id === id);
      if (index >= 0) catalog.tableParts.splice(index, 1);
    },
    setTablePartName(id, name) { const part = catalog.tableParts.find(item => item.id === id); if (part) part.name = name; },
    setTablePartTitle(id, language, text) {
      const part = catalog.tableParts.find(item => item.id === id);
      if (part) part.title = setTranslation(part.title, language, text);
    },
  };
}

function createCatalogEditor(host, onChange) {
  let source, model, selected = {kind: 'catalog'}, titleFieldCleanups = [];
  function title(item) { return localizedTitle(item, source.defaultLanguage); }
  /* Подписи в списках показываются на текущем языке Studio и меняются вместе
     с ним. */
  StudioLanguage.onChange(() => { if (source) renderBody(); });
  // Structural edits (add/remove, selection change, checkboxes, selects) can
  // afford a full re-render. Free-text typing cannot — recreating the input
  // on every keystroke would drop focus and the cursor position, so a text
  // edit only refreshes the sidebar (names may have changed) and leaves the
  // focused detail panel alone.
  function rerender() { onChange(); renderBody(); }
  function touch() { onChange(); refreshSidebar(); }
  function refreshSidebar() {
    const body = host.querySelector('.catalog-body');
    if (!body || !body.firstChild) return;
    body.replaceChild(renderSidebar(), body.firstChild);
  }
  function textField(label, value, apply, options = {}) {
    return labeledField(label, textInput(value, value => { apply(value); if (options.type === 'number') rerender(); else touch(); }, options));
  }
  // The field of a name: at is the way to the part in the catalog, none for
  // the catalog's own name.
  function nameField(value, apply, at) {
    const input = textInput(value, value => { apply(value); touch(); }, {maxLength: 128});
    watchName(input, () => at ? {path: source.path, description: model.value(), at} : {path: source.path, name: source.catalog.name});
    return labeledField('Имя', input);
  }
  // The way to an attribute: of the catalog, or of one of its table parts.
  function attributeWay(container, attribute) {
    const list = model.attributeList(container), at = ['attributes', String(list.indexOf(attribute))];
    return container === 'catalog' ? at : ['tableParts', String(source.catalog.tableParts.findIndex(part => part.id === container)), ...at];
  }
  function selectField(label, value, choices, apply) {
    return labeledField(label, selectInput(choices, value, value => { apply(value); rerender(); }));
  }
  function checkField(label, checked, apply) {
    return checkboxField(label, checked, value => { apply(value); rerender(); });
  }
  function localizedTitleField(target, setTitle) {
    const field = createLocalizedField(source, code => target.title?.[code], setTitle, touch, titleTextOptions);
    titleFieldCleanups.push(field.destroy);
    return labeledField('Заголовок', field);
  }
  // The type of an attribute is a field of its presentation with «…», which
  // opens the window «Редактирование типа данных» (showTypeEditor), as the
  // palette of the prototype has it.
  function renderTypeField(container, attribute) {
    const shown = node('input');
    shown.readOnly = true;
    shown.value = typePresentation(model.typeOffer, attribute.types);
    const choose = fieldChoiceButton('Редактирование типа данных', async () => {
      const types = await showTypeEditor(model.typeOffer, attribute.types);
      if (!types) return;
      model.setAttributeTypes(container, attribute.id, types);
      rerender();
    });
    return labeledField('Тип', fieldWithButtons(shown, [choose]));
  }
  function renderAttributePanel(container, attribute) {
    const panel = node('div', undefined, 'catalog-detail');
    panel.append(node('h3', 'Реквизит'));
    panel.append(nameField(attribute.name, value => model.setAttributeName(container, attribute.id, value), attributeWay(container, attribute)));
    panel.append(localizedTitleField(attribute, (language, value) => model.setAttributeTitle(container, attribute.id, language, value)));
    panel.append(checkField('Проверять заполнение', attribute.fillChecking === 'show-error',
      value => model.setAttributeFillChecking(container, attribute.id, value)));
    panel.append(selectField('Индексирование', attribute.indexing || 'dont-index', model.indexingModes,
      value => model.setAttributeIndexing(container, attribute.id, value)));
    panel.append(renderTypeField(container, attribute));
    return panel;
  }
  // A list of the sidebar: a button per item, the one chosen marked, and a
  // button that adds one. choice(item) is what choosing the item selects;
  // add() adds one and answers its choice, or null when none was added.
  function choiceList(items, choice, addText, add) {
    const list = node('div', undefined, 'catalog-list');
    const same = (left, right) => left.kind === right.kind && left.container === right.container && left.id === right.id;
    for (const item of items) {
      const button = node('button', title(item), 'catalog-list-item');
      button.type = 'button';
      button.classList.toggle('selected', same(selected, choice(item)));
      button.addEventListener('click', () => { selected = choice(item); rerender(); });
      list.append(button);
    }
    const more = node('button', addText);
    more.type = 'button';
    more.addEventListener('click', () => { const next = add(); if (next) selected = next; rerender(); });
    list.append(more);
    return list;
  }
  function renderAttributeList(container, attributes) {
    const choice = attribute => ({kind: 'attribute', container, id: attribute.id});
    return choiceList(attributes, choice, '+ Реквизит', () => { const attribute = model.addAttribute(container); return attribute && choice(attribute); });
  }
  function renderTablePartPanel(part) {
    const panel = node('div', undefined, 'catalog-detail');
    panel.append(node('h3', 'Табличная часть'));
    panel.append(nameField(part.name, value => model.setTablePartName(part.id, value), ['tableParts', String(source.catalog.tableParts.indexOf(part))]));
    panel.append(localizedTitleField(part, (language, value) => model.setTablePartTitle(part.id, language, value)));
    panel.append(node('div', 'Реквизиты табличной части', 'catalog-section-title'));
    panel.append(renderAttributeList(part.id, part.attributes));
    return panel;
  }
  function renderSidebar() {
    const sidebar = node('div', undefined, 'catalog-sidebar');
    const objectButton = node('button', 'Справочник · свойства', 'catalog-list-item');
    objectButton.type = 'button';
    objectButton.classList.toggle('selected', selected.kind === 'catalog');
    objectButton.addEventListener('click', () => { selected = {kind: 'catalog'}; rerender(); });
    sidebar.append(objectButton);
    // The two folders stand as the project tree's do, with their icons.
    const partChoice = part => ({kind: 'tablepart', id: part.id});
    sidebar.append(
      treeNode({icon: 'attribute', caption: 'Реквизиты', open: true, children: [renderAttributeList('catalog', source.catalog.attributes)]}).element,
      treeNode({icon: 'tabular-section', caption: 'Табличные части', open: true,
        children: [choiceList(source.catalog.tableParts, partChoice, '+ Табличная часть', () => partChoice(model.addTablePart()))]}).element);
    return sidebar;
  }
  function renderObjectPanel() {
    const panel = node('div', undefined, 'catalog-detail');
    panel.append(node('h3', 'Свойства справочника'));
    panel.append(node('div', `UUID: ${source.catalog.id}`, 'catalog-uuid'));
    panel.append(nameField(source.catalog.name, value => model.setName(value)));
    panel.append(localizedTitleField(source.catalog, (language, value) => model.setTitle(language, value)));
    const code = node('div', undefined, 'catalog-code');
    code.append(node('div', 'Код', 'catalog-section-title'));
    code.append(selectField('Тип кода', source.catalog.code.type, [['string', 'Строка'], ['number', 'Число']], value => model.setCodeType(value)));
    code.append(textField('Длина кода', source.catalog.code.length, value => model.setCodeLength(value), {type: 'number', min: 0, max: 50}));
    code.append(checkField('Автонумерация', !!source.catalog.code.auto, value => model.setCodeAuto(value)));
    code.append(checkField('Проверять уникальность', !!source.catalog.code.unique, value => model.setCodeUnique(value)));
    panel.append(code);
    panel.append(textField('Длина наименования', source.catalog.descriptionLength, value => model.setDescriptionLength(value), {type: 'number', min: 0, max: 150}));
    return panel;
  }
  function clearTitleFields() { titleFieldCleanups.forEach(destroy => destroy()); titleFieldCleanups = []; }
  function renderBody() {
    const body = host.querySelector('.catalog-body');
    if (!body) return;
    clearTitleFields();
    body.replaceChildren();
    body.append(renderSidebar());
    let panel;
    if (selected.kind === 'attribute') {
      const attribute = model.attributeList(selected.container)?.find(item => item.id === selected.id);
      if (attribute) panel = renderAttributePanel(selected.container, attribute);
      else { selected = {kind: 'catalog'}; panel = renderObjectPanel(); }
    } else if (selected.kind === 'tablepart') {
      const part = source.catalog.tableParts.find(item => item.id === selected.id);
      if (part) panel = renderTablePartPanel(part);
      else { selected = {kind: 'catalog'}; panel = renderObjectPanel(); }
    } else {
      panel = renderObjectPanel();
    }
    body.append(panel);
  }
  return hostedEditor(host, () => { clearTitleFields(); source = null; model = null; }, {
    open(value) {
      clearTitleFields();
      source = structuredClone(value);
      model = createCatalogModel(source);
      selected = {kind: 'catalog'};
      host.hidden = false;
      host.replaceChildren();
      const body = node('div', undefined, 'catalog-body');
      host.append(body);
      renderBody();
    },
    value() { return model?.value(); },
    // Selects the attribute or table part a node of the tree names
    // (`attribute:<id>` or `tablepart:<id>`), so a click on the main
    // configuration tree jumps straight to it inside the open editor.
    select(kind, id) {
      if (!source) return;
      if (kind === 'attribute' && source.catalog.attributes.some(item => item.id === id)) {
        selected = {kind: 'attribute', container: 'catalog', id};
        rerender();
      } else if (kind === 'tablepart' && source.catalog.tableParts.some(item => item.id === id)) {
        selected = {kind: 'tablepart', id};
        rerender();
      }
    },
    // Adds a new attribute or table part on behalf of the tree's generic
    // "Add" toolbar button, positioned by where the tree selection was
    // (Реквизиты/an attribute -> new sibling attribute in the same
    // container; Табличные части/a table part -> new attribute inside it,
    // or a new table part) — same effect as this editor's own "+ Реквизит"
    // / "+ Табличная часть" buttons, just triggered from the tree.
    addFromTreeContext(context) {
      if (!source) return;
      if (context.kind === 'tablepart' && !context.container) {
        const part = model.addTablePart();
        selected = {kind: 'tablepart', id: part.id};
        rerender();
        return;
      }
      const container = context.container || 'catalog';
      const attribute = model.addAttribute(container);
      if (attribute) {
        selected = {kind: 'attribute', container, id: attribute.id};
        rerender();
      }
    },
    // Removes the attribute or table part on behalf of the tree's generic
    // "Delete" toolbar button — the only way to delete these now that this
    // editor no longer carries its own duplicate "Удалить" buttons.
    deleteFromTreeContext(context) {
      if (!source) return;
      if (context.kind === 'tablepart') {
        model.removeTablePart(context.id);
      } else if (context.kind === 'attribute') {
        model.removeAttribute(context.container, context.id);
      } else {
        return;
      }
      selected = {kind: 'catalog'};
      rerender();
    },
  });
}
