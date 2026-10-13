/* «Все роли» (справка, «Все роли»): права выбранного объекта во всех ролях
   сразу и с правкой — права строками, роли столбцами; первая строка ставит
   или снимает все права роли, первая колонка — право во всех ролях; ниже —
   ограничения доступа выбранной роли. Две вкладки рядом — все ограничения
   и все шаблоны проекта — только для просмотра. */

/* Модель отделена от разметки и ничего не знает про DOM, поэтому
   проверяется тестами напрямую. Каждая роль правится своей моделью
   редактора роли (createRoleModel): что значит право, флаг, ограничение —
   знает она одна. */
function createRoleOverviewModel(source) {
  const entries = (source.roles || []).map(entry => {
    const model = createRoleModel({role: entry.role, schema: source.schema});
    return {path: entry.path, revision: entry.revision, role: entry.role, model, initial: JSON.stringify(model.value())};
  });
  const objects = new Map((source.schema.objects || []).map(item => [item.id, item]));
  function localized(item) { return localizedTitle(item, source.defaultLanguage, source.languages); }
  function objectTitle(id) {
    const object = objects.get(id);
    return object ? localized(object) : 'удалённый объект';
  }
  function describe(objectID, restriction) {
    return {operation: restriction.right, field: restrictionFieldsCaption(objects.get(objectID), restriction, localized), condition: restriction.condition || ''};
  }
  /* Поля объекта — части, на которые даются Просмотр и Редактирование;
     команды, операции и методы полями не считаются. */
  function fieldKeys(objectID) { return new Set((objects.get(objectID)?.fields || []).filter(field => field.operations.includes('edit')).map(field => field.key)); }
  function fieldSummary(grant, objectID) {
    const keys = fieldKeys(objectID), total = keys.size;
    if (!grant?.operations?.includes('read') || !total) return '—';
    const granted = Math.min((grant?.fields || []).filter(field => keys.has(field.field) && (field.operations || []).includes('view')).length, total);
    return granted >= total ? 'все' : `${granted} из ${total}`;
  }
  /* Строки окна по выбранному в дереве: права объекта, права части или
     команды формы, у каждой — как прочитать и записать её в роли. */
  function lines(selection) {
    if (selection?.form) {
      const form = selection.form;
      return form.commands.map(command => ({key: command.id, command,
        get: model => model.hasCommand(form.id, command.id), set: (model, enabled) => model.setCommand(form.id, command.id, enabled)}));
    }
    const object = selection?.object;
    if (!object) return [];
    if (selection.part) {
      const part = selection.part;
      return part.operations.map(right => ({key: right,
        get: model => model.hasField(object.id, part.key, right), set: (model, enabled) => model.setField(object.id, part.key, right, enabled)}));
    }
    return object.rights.map(right => ({key: right,
      get: model => model.hasObject(object.id, right), set: (model, enabled) => model.setObject(object.id, right, enabled)}));
  }
  // Флажок над многими: стоит, когда стоят все, снят, когда не стоит ни один,
  // иначе — третье состояние.
  function state(values) {
    const set = values.filter(Boolean).length;
    return !values.length || !set ? 'none' : set === values.length ? 'all' : 'some';
  }
  const at = index => entries[index]?.model;
  const roleOf = entry => entry.model.value();
  return {
    objects() { return source.schema.objects || []; },
    title(item) { return localized(item); },
    roleCount() { return entries.length; },
    roles() { return entries.map((entry, index) => ({index, path: entry.path, name: entry.role.name, title: localized(entry.role)})); },
    model: at,
    rows(selection) { return lines(selection).map(({key, command}) => ({key, command})); },
    has(index, selection, key) { const line = lines(selection).find(item => item.key === key); return !!line && line.get(at(index)); },
    set(index, selection, key, enabled) { const line = lines(selection).find(item => item.key === key); if (line && at(index)) line.set(at(index), enabled); },
    // Первая строка: все права роли на выбранное.
    roleState(index, selection) { return state(lines(selection).map(line => line.get(at(index)))); },
    setRole(index, selection, enabled) { for (const line of lines(selection)) line.set(at(index), enabled); },
    // Первая колонка: право во всех ролях.
    rightState(selection, key) { const line = lines(selection).find(item => item.key === key); return line ? state(entries.map(entry => line.get(entry.model))) : 'none'; },
    setRight(selection, key, enabled) { const line = lines(selection).find(item => item.key === key); if (line) for (const entry of entries) line.set(entry.model, enabled); },
    // Что сохранять: только изменённые роли, каждая — с ревизией, при
    // которой она прочитана.
    changes() {
      return entries.filter(entry => JSON.stringify(entry.model.value()) !== entry.initial)
        .map(entry => ({path: entry.path, expectedRevision: entry.revision, role: entry.model.value()}));
    },
    // Один список по всему проекту: здесь вопрос не «что с этим объектом»,
    // а «где вообще в конфигурации стоят ограничения» — перед выпуском это
    // единственный способ увидеть их все, не открывая роли по одной.
    allRestrictions() {
      const rows = [];
      for (const entry of entries) {
        for (const grant of roleOf(entry).objects || []) {
          if (!objects.has(grant.object)) continue;
          for (const restriction of grant.restrictions || []) {
            rows.push({path: entry.path, objectID: grant.object, object: objectTitle(grant.object),
              role: localized(entry.role), fields: fieldSummary(grant, grant.object), ...describe(grant.object, restriction)});
          }
        }
      }
      rows.sort((left, right) => left.object.localeCompare(right.object) || left.role.localeCompare(right.role));
      return rows;
    },
    // Шаблон показывается как он написан: имя с параметрами в скобках и
    // текст, в который ограничение подставляет значения параметров.
    allTemplates() {
      const rows = [];
      for (const entry of entries) {
        for (const template of roleOf(entry).restrictionTemplates || []) {
          rows.push({path: entry.path, role: localized(entry.role), name: template.name, condition: template.condition || ''});
        }
      }
      rows.sort((left, right) => left.role.localeCompare(right.role) || left.name.localeCompare(right.name));
      return rows;
    },
  };
}

function createRoleOverview(host, openRole, onChange) {
  let source, model, selected = null, tree, panel, tab = 'roles', body, filter = '';
  // Роль, выбранная щелчком по её столбцу: её ограничения — под таблицей
  // (справка: «Для редактирования ограничения необходимо сначала выбрать
  // роль»), с правом и строкой, выбранными в списке.
  let chosenRole = null;
  const restriction = {right: null, row: -1}, expanded = new Set();
  /* Подписи прав — с сервера, из тех же перечней, что у редактора роли. */
  const caption = key => source.schema.rights?.find(item => item.key === key)?.title || key;
  const title = item => model.title(item);
  /* Подписи показываются на текущем языке Studio и меняются вместе с ним. */
  StudioLanguage.onChange(() => { if (!source) return; renderBody(); });
  function changed() { onChange?.(); renderPanel(); }
  const roleIndex = () => model.roles().findIndex(role => role.path === chosenRole);
  // Флажок с третьим состоянием: стоит у всех, ни у кого, у части.
  function stateBox(state, label, action) {
    const box = node('input');
    box.type = 'checkbox'; box.checked = state === 'all'; box.indeterminate = state === 'some';
    box.setAttribute('aria-label', label);
    box.addEventListener('change', () => { action(box.checked); changed(); });
    return box;
  }
  function rowCaption(row) { return row.command ? model.title(row.command) || row.command.name : caption(row.key); }
  function renderGrid() {
    const roles = model.roles(), rows = model.rows(selected);
    if (!roles.length) return node('p', 'В проекте нет ролей.', 'muted');
    if (!rows.length) return node('p', selected.form ? 'У формы нет команд.' : selected.part ? 'Своих прав у этого узла нет: права даются на то, что в нём.' : 'Своих прав у объекта нет: права даются на его части.', 'muted');
    const scroll = node('div', undefined, 'overview-grid'), table = node('table'), head = node('tr');
    head.append(node('th', selected.form ? 'Команда' : 'Право', 'overview-right'));
    for (const role of roles) {
      const cell = node('th', undefined, role.path === chosenRole ? 'overview-chosen' : undefined), button = node('button', role.title, 'overview-role');
      button.type = 'button'; button.title = role.name;
      button.addEventListener('click', () => { chosenRole = role.path; restriction.row = -1; renderPanel(); });
      cell.append(button); head.append(cell);
    }
    const thead = node('thead'), tbody = node('tbody');
    thead.append(head);
    const all = node('tr', undefined, 'overview-all'), allCaption = node('th', selected.form ? 'Все команды' : 'Все права', 'overview-right');
    all.append(allCaption);
    for (const role of roles) {
      const cell = node('td', undefined, role.path === chosenRole ? 'overview-chosen' : undefined);
      cell.append(stateBox(model.roleState(role.index, selected), `${role.title} — все`, enabled => model.setRole(role.index, selected, enabled)));
      all.append(cell);
    }
    tbody.append(all);
    for (const row of rows) {
      const line = node('tr'), text = rowCaption(row), name = node('th', undefined, 'overview-right'), label = node('label');
      label.append(stateBox(model.rightState(selected, row.key), `${text} — во всех ролях`, enabled => model.setRight(selected, row.key, enabled)), node('span', text));
      name.append(label); line.append(name);
      for (const role of roles) {
        const cell = node('td', undefined, role.path === chosenRole ? 'overview-chosen' : undefined), box = node('input');
        box.type = 'checkbox'; box.checked = model.has(role.index, selected, row.key);
        box.setAttribute('aria-label', `${role.title} — ${text}`);
        box.addEventListener('change', () => { model.set(role.index, selected, row.key, box.checked); chosenRole = role.path; changed(); });
        // Щелчок по клетке выбирает роль, а по праву с ограничениями — и
        // его ограничения под таблицей.
        cell.addEventListener('click', event => {
          if (chosenRole === role.path && restriction.right === row.key) return;
          chosenRole = role.path;
          if (selected.object?.restrictedRights?.includes(row.key) && !selected.part) { restriction.right = row.key; restriction.row = -1; }
          if (event.target !== box) renderPanel();
        });
        cell.append(box); line.append(cell);
      }
      tbody.append(line);
    }
    table.append(thead, tbody); scroll.append(table);
    return scroll;
  }
  function renderRestrictions(object) {
    const index = roleIndex();
    if (index < 0) {
      const block = node('section', undefined, 'role-restrictions');
      block.append(node('h4', 'Ограничение доступа к данным'), node('p', 'Выберите роль — щелчком по её столбцу, — чтобы увидеть и изменить её ограничения.', 'muted'));
      return block;
    }
    const role = model.roles()[index];
    const section = roleRestrictionSection({model: model.model(index), object, caption, title, state: restriction, changed});
    const heading = node('div', undefined, 'overview-chosen-role'), link = node('button', 'Открыть роль', 'overview-role');
    link.type = 'button';
    link.addEventListener('click', () => openRole(role.path, object.id));
    heading.append(node('span', `Роль: ${role.title}`), link);
    section.querySelector('h4').after(heading);
    return section;
  }
  function roleLink(text, path, objectID) {
    const cell = node('td'), link = node('button', text, 'overview-role');
    link.type = 'button';
    link.addEventListener('click', () => openRole(path, objectID));
    cell.append(link);
    return cell;
  }
  function matches(row, columns) {
    if (!filter.trim()) return true;
    const needle = filter.toLocaleLowerCase().trim();
    return columns.some(value => (value || '').toLocaleLowerCase().includes(needle));
  }
  // «Все ограничения» первого окна 1С — это две таблицы; Studio показывает их
  // как две вкладки одной полосы вместе с «Всеми ролями», потому что
  // отдельных окон здесь нет и вкладка дешевле лишнего представления.
  function renderAllRestrictions() {
    const block = node('section', undefined, 'overview-block');
    const rows = model.allRestrictions().filter(row => matches(row, [row.object, row.role, row.field, row.condition]));
    if (!rows.length) { block.append(node('p', model.allRestrictions().length ? 'Ничего не найдено.' : 'В проекте нет ни одного ограничения доступа.', 'muted')); return block; }
    block.append(node('p', 'Каждая строка — одно ограничение одной роли на одно право. «Реквизиты» — сколько реквизитов объекта эта роль вообще читает: ограничение сужает записи, права на реквизиты — столбцы.', 'muted'));
    const table = node('table'), head = node('tr');
    head.append(node('th', 'Объект'), node('th', 'Роль'), node('th', 'Право'), node('th', 'Реквизиты'), node('th', 'Поля'), node('th', 'Условие'));
    table.append(head);
    for (const row of rows) {
      const line = node('tr');
      line.append(node('td', row.object));
      line.append(roleLink(row.role, row.path, row.objectID));
      line.append(node('td', caption(row.operation)), node('td', row.fields));
      line.append(node('td', row.field), node('td', row.condition, 'overview-condition'));
      table.append(line);
    }
    block.append(table);
    return block;
  }
  function renderAllTemplates() {
    const block = node('section', undefined, 'overview-block');
    const all = model.allTemplates();
    const rows = all.filter(row => matches(row, [row.role, row.name, row.condition]));
    if (!rows.length) { block.append(node('p', all.length ? 'Ничего не найдено.' : 'Ни одна роль не объявила шаблонов ограничений.', 'muted')); return block; }
    block.append(node('p', 'Шаблон принадлежит роли: условие её ограничения вызывает его как #Имя("значение", ...).', 'muted'));
    const table = node('table'), head = node('tr');
    head.append(node('th', 'Роль'), node('th', 'Имя'), node('th', 'Текст'));
    table.append(head);
    for (const row of rows) {
      const line = node('tr');
      line.append(roleLink(row.role, row.path));
      line.append(node('td', row.name), node('td', row.condition, 'overview-condition'));
      table.append(line);
    }
    block.append(table);
    return block;
  }
  function renderBody() {
    body.replaceChildren();
    if (tab === 'roles') {
      const sidebar = node('div', undefined, 'overview-sidebar'), search = node('input');
      search.type = 'search'; search.placeholder = 'Поиск объекта'; search.setAttribute('aria-label', 'Поиск объекта');
      search.value = filter;
      tree = createRoleTree({schema: source.schema, kindTitles: source.kindTitles, title, expanded, selection: () => selected, choose: next => { selected = next; renderPanel(); }});
      search.addEventListener('input', () => { filter = search.value; tree.render(filter); });
      sidebar.append(search, tree.element);
      panel = node('section', undefined, 'overview-panel');
      body.append(sidebar, panel);
      body.className = 'overview-body';
      tree.render(filter); renderPanel();
      return;
    }
    body.className = 'overview-list';
    const search = node('input');
    search.type = 'search'; search.placeholder = 'Поиск по таблице'; search.setAttribute('aria-label', 'Поиск по таблице');
    search.value = filter;
    // Перерисовывается только сама таблица: поле поиска остаётся тем же
    // элементом, иначе каждая набранная буква уводила бы из него фокус.
    const list = node('div');
    const fill = () => list.replaceChildren(tab === 'restrictions' ? renderAllRestrictions() : renderAllTemplates());
    search.addEventListener('input', () => { filter = search.value; fill(); });
    fill();
    body.append(search, list);
  }
  function renderPanel() {
    panel.replaceChildren();
    if (!selected) { panel.append(node('p', 'Выберите объект слева: его права во всех ролях можно отметить здесь сразу.', 'muted')); return; }
    const item = selected.form || selected.object;
    panel.append(node('h3', selected.part ? `${model.title(item) || item.name} · ${model.title(selected.part) || selected.part.name}` : model.title(item) || item.name));
    panel.append(renderGrid());
    if (selected.object && !selected.part && selected.object.restrictedRights?.length) panel.append(renderRestrictions(selected.object));
  }
  // Выбранное в дереве и выбранная роль переживают сохранение: окно
  // открывается заново с тем, что записано, на том же месте.
  function reselect() {
    if (!selected) return null;
    if (selected.form) { const form = source.schema.forms.find(item => item.id === selected.form.id); return form ? {form} : null; }
    const object = model.objects().find(item => item.id === selected.object.id);
    if (!object) return null;
    const part = selected.part && object.fields.find(item => item.key === selected.part.key);
    return part ? {object, part} : selected.part ? null : {object};
  }
  function open(value, keep = false) {
      source = structuredClone(value); model = createRoleOverviewModel(source);
      selected = reselect();
      if (!keep) { expanded.clear(); restriction.right = null; restriction.row = -1; }
      if (!model.roles().some(role => role.path === chosenRole)) chosenRole = null;
      host.hidden = false; host.replaceChildren();
      const header = node('div', undefined, 'overview-header');
      const strip = node('div', undefined, 'overview-tabs');
      for (const [key, text] of [['roles', 'Все роли'], ['restrictions', 'Ограничения доступа'], ['templates', 'Шаблоны ограничений']]) {
        const button = node('button', text, 'overview-tab');
        button.type = 'button';
        if (tab === key) button.classList.add('selected');
        button.setAttribute('aria-pressed', String(tab === key));
        button.dataset.tab = key;
        button.addEventListener('click', () => { if (tab === key) return; tab = key; filter = ''; drawFrame(); });
        strip.append(button);
      }
      header.append(strip, node('span', `Ролей в проекте: ${model.roleCount()}`, 'muted'));
      host.append(header);
      body = node('div'); host.append(body);
      renderBody();
  }
  // Смена вкладки перерисовывает окно, не трогая правок: модели ролей остаются.
  function drawFrame() {
    for (const button of host.querySelectorAll('.overview-tab')) {
      const on = button.dataset.tab === tab;
      button.classList.toggle('selected', on); button.setAttribute('aria-pressed', String(on));
    }
    renderBody();
  }
  return {
    open,
    // Роли, изменённые в окне, — для сохранения одним запросом.
    value() { return model?.changes() || []; },
    close() { source = null; model = null; host.hidden = true; host.replaceChildren(); },
    setDisabled(disabled) { host.inert = disabled; },
  };
}
