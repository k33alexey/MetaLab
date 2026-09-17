'use strict';

/* Catalog editor: fields/checkboxes/lists instead of hand-edited YAML.
   The model mutates `source.catalog` in place (no internal clone) so every
   render function can read straight from `source` and stay in sync. */
function createCatalogModel(source) {
  const catalog = source.catalog;
  catalog.attributes ||= [];
  catalog.tableParts ||= [];
  const typeKinds = [
    ['string', 'Строка'], ['number', 'Число'], ['boolean', 'Булево'], ['date', 'Дата'], ['uuid', 'УникальныйИдентификатор'],
    ['enumeration', 'Перечисление'], ['defined-type', 'Определяемый тип'], ['catalog', 'СправочникСсылка'], ['document', 'ДокументСсылка'],
  ];
  function referenceChoices(kind) {
    return {enumeration: source.typeChoices.enumerations, 'defined-type': source.typeChoices.definedTypes, catalog: source.typeChoices.catalogs, document: source.typeChoices.documents}[kind] || [];
  }
  function attributeList(container) {
    if (container === 'catalog') return catalog.attributes;
    return catalog.tableParts.find(part => part.id === container)?.attributes;
  }
  function usedNames(exclude) {
    const names = new Set([catalog.name.toLocaleLowerCase()]);
    for (const attribute of catalog.attributes) if (attribute.id !== exclude) names.add(attribute.name.toLocaleLowerCase());
    for (const part of catalog.tableParts) {
      if (part.id !== exclude) names.add(part.name.toLocaleLowerCase());
      for (const attribute of part.attributes) if (attribute.id !== exclude) names.add(attribute.name.toLocaleLowerCase());
    }
    return names;
  }
  function uniqueName(base) {
    const used = usedNames();
    let index = 1, name = base;
    while (used.has(name.toLocaleLowerCase())) name = base + (++index);
    return name;
  }
  return {
    value() { return structuredClone(catalog); },
    typeKinds, referenceChoices, attributeList,
    setName(name) { catalog.name = name; },
    setTitle(language, text) { catalog.title ||= {}; if (text.trim()) catalog.title[language] = text; else delete catalog.title[language]; },
    setCodeType(type) { catalog.code.type = type; },
    setCodeLength(length) { catalog.code.length = length; },
    setCodeAuto(auto) { catalog.code.auto = auto; },
    setCodeUnique(unique) { catalog.code.unique = unique; },
    setDescriptionLength(length) { catalog.descriptionLength = length; },
    addAttribute(container) {
      const list = attributeList(container);
      if (!list) return null;
      const attribute = {id: crypto.randomUUID(), name: uniqueName('НовыйРеквизит'), title: {[source.defaultLanguage]: 'Новый реквизит'}, types: [{kind: 'string', length: 50}]};
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
      if (!attribute) return;
      attribute.title ||= {};
      if (text.trim()) attribute.title[language] = text; else delete attribute.title[language];
    },
    setAttributeRequired(container, id, required) { const attribute = attributeList(container)?.find(item => item.id === id); if (attribute) attribute.required = required; },
    setAttributeIndexed(container, id, indexed) { const attribute = attributeList(container)?.find(item => item.id === id); if (attribute) attribute.indexed = indexed; },
    addType(container, id) {
      const attribute = attributeList(container)?.find(item => item.id === id);
      if (!attribute || attribute.types.length >= 32) return;
      attribute.types.push({kind: 'string', length: 50});
    },
    removeType(container, id, index) {
      const attribute = attributeList(container)?.find(item => item.id === id);
      if (!attribute || attribute.types.length <= 1) return;
      attribute.types.splice(index, 1);
    },
    setTypeKind(container, id, index, kind) {
      const attribute = attributeList(container)?.find(item => item.id === id);
      if (!attribute?.types[index]) return;
      const type = {kind};
      if (kind === 'string') type.length = 50;
      else if (kind === 'number') { type.precision = 15; type.scale = 2; }
      else if (['enumeration', 'defined-type', 'catalog', 'document'].includes(kind)) type.reference = referenceChoices(kind)[0]?.id;
      attribute.types[index] = type;
    },
    setTypeField(container, id, index, field, fieldValue) {
      const attribute = attributeList(container)?.find(item => item.id === id);
      if (!attribute?.types[index]) return;
      attribute.types[index][field] = fieldValue;
    },
    addTablePart() {
      const part = {id: crypto.randomUUID(), name: uniqueName('НоваяТабличнаяЧасть'), title: {[source.defaultLanguage]: 'Новая табличная часть'}, attributes: []};
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
      if (!part) return;
      part.title ||= {};
      if (text.trim()) part.title[language] = text; else delete part.title[language];
    },
  };
}

function createCatalogEditor(host, onChange) {
  let source, model, selected = {kind: 'catalog'}, titleFieldCleanups = [];
  function node(tag, text, className) {
    const element = document.createElement(tag);
    if (text !== undefined) element.textContent = text;
    if (className) element.className = className;
    return element;
  }
  function title(item) { return item.title?.[source.defaultLanguage] || Object.values(item.title || {}).find(Boolean) || item.name; }
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
    const wrapper = node('label', undefined, 'catalog-field'), input = node('input');
    wrapper.append(node('span', label));
    input.type = options.type || 'text';
    input.value = value ?? '';
    if (options.min !== undefined) input.min = options.min;
    if (options.max !== undefined) input.max = options.max;
    if (options.maxLength) input.maxLength = options.maxLength;
    if (options.type === 'number') {
      input.addEventListener('change', () => { apply(Number(input.value)); rerender(); });
    } else {
      input.addEventListener('input', () => { apply(input.value); touch(); });
    }
    wrapper.append(input);
    return wrapper;
  }
  function selectField(label, value, choices, apply) {
    const wrapper = node('label', undefined, 'catalog-field'), select = node('select');
    wrapper.append(node('span', label));
    for (const [key, text] of choices) {
      const option = node('option', text);
      option.value = key;
      option.selected = key === value;
      select.append(option);
    }
    select.addEventListener('change', () => { apply(select.value); rerender(); });
    wrapper.append(select);
    return wrapper;
  }
  function checkboxField(label, checked, apply) {
    const wrapper = node('label', undefined, 'catalog-check'), input = node('input');
    input.type = 'checkbox'; input.checked = checked;
    input.addEventListener('change', () => { apply(input.checked); rerender(); });
    wrapper.append(input, node('span', label));
    return wrapper;
  }
  function localizedTitleField(target, setTitle) {
    const wrapper = node('label', undefined, 'catalog-field');
    wrapper.append(node('span', 'Заголовок'));
    const field = createLocalizedTitleField(source.languages, code => target.title?.[code], (code, value) => setTitle(code, value), touch);
    titleFieldCleanups.push(field.destroy);
    wrapper.append(field);
    return wrapper;
  }
  function renderTypeRow(container, attribute, type, index) {
    const row = node('div', undefined, 'catalog-type-row');
    row.append(selectField('Тип', type.kind, model.typeKinds, kind => model.setTypeKind(container, attribute.id, index, kind)));
    if (type.kind === 'string') {
      row.append(textField('Длина', type.length, value => model.setTypeField(container, attribute.id, index, 'length', value), {type: 'number', min: 1, max: 100000}));
    } else if (type.kind === 'number') {
      row.append(textField('Точность', type.precision, value => model.setTypeField(container, attribute.id, index, 'precision', value), {type: 'number', min: 1, max: 38}));
      row.append(textField('Масштаб', type.scale, value => model.setTypeField(container, attribute.id, index, 'scale', value), {type: 'number', min: 0, max: 20}));
    } else if (['enumeration', 'defined-type', 'catalog', 'document'].includes(type.kind)) {
      const choices = model.referenceChoices(type.kind).map(item => [item.id, item.title?.[source.defaultLanguage] || item.name]);
      row.append(selectField('Ссылка на', type.reference, choices, value => model.setTypeField(container, attribute.id, index, 'reference', value)));
    }
    if (attribute.types.length > 1) {
      const remove = node('button', 'Удалить тип');
      remove.type = 'button';
      remove.addEventListener('click', () => { model.removeType(container, attribute.id, index); rerender(); });
      row.append(remove);
    }
    return row;
  }
  function renderAttributePanel(container, attribute) {
    const panel = node('div', undefined, 'catalog-detail');
    panel.append(node('h3', 'Реквизит'));
    panel.append(textField('Имя', attribute.name, value => model.setAttributeName(container, attribute.id, value), {maxLength: 128}));
    panel.append(localizedTitleField(attribute, (language, value) => model.setAttributeTitle(container, attribute.id, language, value)));
    panel.append(checkboxField('Обязательный', !!attribute.required, value => model.setAttributeRequired(container, attribute.id, value)));
    panel.append(checkboxField('Индексировать', !!attribute.indexed, value => model.setAttributeIndexed(container, attribute.id, value)));
    const types = node('div', undefined, 'catalog-types');
    types.append(node('div', 'Типы значения', 'catalog-section-title'));
    for (const [index, type] of attribute.types.entries()) types.append(renderTypeRow(container, attribute, type, index));
    const addType = node('button', '+ Тип');
    addType.type = 'button';
    addType.disabled = attribute.types.length >= 32;
    addType.addEventListener('click', () => { model.addType(container, attribute.id); rerender(); });
    types.append(addType);
    panel.append(types);
    return panel;
  }
  function renderAttributeList(container, attributes) {
    const list = node('div', undefined, 'catalog-list');
    for (const attribute of attributes) {
      const button = node('button', title(attribute), 'catalog-list-item');
      button.type = 'button';
      button.classList.toggle('selected', selected.kind === 'attribute' && selected.container === container && selected.id === attribute.id);
      button.addEventListener('click', () => { selected = {kind: 'attribute', container, id: attribute.id}; rerender(); });
      list.append(button);
    }
    const add = node('button', '+ Реквизит');
    add.type = 'button';
    add.addEventListener('click', () => { const attribute = model.addAttribute(container); if (attribute) selected = {kind: 'attribute', container, id: attribute.id}; rerender(); });
    list.append(add);
    return list;
  }
  function renderTablePartPanel(part) {
    const panel = node('div', undefined, 'catalog-detail');
    panel.append(node('h3', 'Табличная часть'));
    panel.append(textField('Имя', part.name, value => model.setTablePartName(part.id, value), {maxLength: 128}));
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
    const attributes = node('details', undefined, 'catalog-group');
    attributes.open = true;
    attributes.append(node('summary', 'Реквизиты'));
    attributes.append(renderAttributeList('catalog', source.catalog.attributes));
    sidebar.append(attributes);
    const tableParts = node('details', undefined, 'catalog-group');
    tableParts.open = true;
    tableParts.append(node('summary', 'Табличные части'));
    const partList = node('div', undefined, 'catalog-list');
    for (const part of source.catalog.tableParts) {
      const button = node('button', title(part), 'catalog-list-item');
      button.type = 'button';
      button.classList.toggle('selected', selected.kind === 'tablepart' && selected.id === part.id);
      button.addEventListener('click', () => { selected = {kind: 'tablepart', id: part.id}; rerender(); });
      partList.append(button);
    }
    const addPart = node('button', '+ Табличная часть');
    addPart.type = 'button';
    addPart.addEventListener('click', () => { const part = model.addTablePart(); selected = {kind: 'tablepart', id: part.id}; rerender(); });
    partList.append(addPart);
    tableParts.append(partList);
    sidebar.append(tableParts);
    return sidebar;
  }
  function renderObjectPanel() {
    const panel = node('div', undefined, 'catalog-detail');
    panel.append(node('h3', 'Свойства справочника'));
    panel.append(node('div', `UUID: ${source.catalog.id}`, 'catalog-uuid'));
    panel.append(textField('Имя', source.catalog.name, value => model.setName(value), {maxLength: 128}));
    panel.append(localizedTitleField(source.catalog, (language, value) => model.setTitle(language, value)));
    const code = node('div', undefined, 'catalog-code');
    code.append(node('div', 'Код', 'catalog-section-title'));
    code.append(selectField('Тип кода', source.catalog.code.type, [['string', 'Строка'], ['number', 'Число']], value => model.setCodeType(value)));
    code.append(textField('Длина кода', source.catalog.code.length, value => model.setCodeLength(value), {type: 'number', min: 1, max: 128}));
    code.append(checkboxField('Автонумерация', !!source.catalog.code.auto, value => model.setCodeAuto(value)));
    code.append(checkboxField('Проверять уникальность', !!source.catalog.code.unique, value => model.setCodeUnique(value)));
    panel.append(code);
    panel.append(textField('Длина наименования', source.catalog.descriptionLength, value => model.setDescriptionLength(value), {type: 'number', min: 1, max: 1048576}));
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
  return {
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
    close() { clearTitleFields(); source = null; model = null; host.hidden = true; host.replaceChildren(); },
    setDisabled(disabled) { host.inert = disabled; },
    // Selects the attribute or table part named by a tree-node fragment
    // (`attribute:<id>` or `tablepart:<id>`), so a click on the main
    // configuration tree jumps straight to it inside the open editor.
    selectFragment(fragment) {
      if (!source || !fragment) return;
      const [kind, id] = fragment.split(':');
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
  };
}
