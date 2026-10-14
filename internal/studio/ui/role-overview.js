/* «Все роли» (справка, «Все роли»): права выбранного объекта во всех ролях
   сразу и с правкой — права строками, роли столбцами; первая строка ставит
   или снимает все права роли, первая колонка — право во всех ролях; ниже —
   ограничения доступа выбранной роли. Второе окно того же вида — «Все
   ограничения доступа» — с закладками «Ограничения доступа» и «Шаблоны
   ограничений» и командами прототипа над несколькими строками сразу:
   изменить группой, копировать в роли, удалить (справка, «Все ограничения
   доступа»). Оба окна правят одни и те же модели ролей. */

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
    // Строка знает, где лежит её ограничение: роль, объект, право и номер
    // среди ограничений этого права, — по ним его правят команды окна.
    // filter — отбор окна (справка, form_RolesGroupEditFilterDlg), null —
    // без отбора: объекты, роли (пути), права, поля (пути полей) и фрагмент
    // текста ограничения. Строка проходит, когда проходит по каждому
    // условию.
    allRestrictions(filter = null) {
      const rows = [];
      entries.forEach((entry, roleIndex) => {
        for (const grant of roleOf(entry).objects || []) {
          if (!objects.has(grant.object)) continue;
          const counted = new Map();
          for (const restriction of grant.restrictions || []) {
            const index = counted.get(restriction.right) || 0;
            counted.set(restriction.right, index + 1);
            rows.push({path: entry.path, roleIndex, index, objectID: grant.object, object: objectTitle(grant.object),
              role: localized(entry.role), fields: fieldSummary(grant, grant.object), keys: restriction.fields || [], ...describe(grant.object, restriction)});
          }
        }
      });
      rows.sort((left, right) => left.object.localeCompare(right.object) || left.role.localeCompare(right.role));
      if (!filter) return rows;
      const pathsOf = new Map();
      const fieldPassed = row => {
        if (!filter.paths?.size) return true;
        if (!pathsOf.has(row.objectID)) pathsOf.set(row.objectID, fieldPaths(row.objectID));
        return row.keys.some(key => filter.paths.has(pathsOf.get(row.objectID).get(key)));
      };
      return rows.filter(row => among(filter.objects, row.objectID) && among(filter.roles, row.path) && among(filter.rights, row.operation)
        && fieldPassed(row) && holds(row.condition, filter.condition));
    },
    // Шаблон показывается как он написан: имя с параметрами в скобках и
    // текст, в который ограничение подставляет значения параметров.
    // filter (справка, form_TemplateEditFilterDlg): роли (пути),
    // наименование — на равенство, фрагмент текста шаблона.
    allTemplates(filter = null) {
      const rows = [];
      entries.forEach((entry, roleIndex) => {
        (roleOf(entry).restrictionTemplates || []).forEach((template, index) => {
          rows.push({path: entry.path, roleIndex, index, role: localized(entry.role), name: template.name, condition: template.condition || ''});
        });
      });
      rows.sort((left, right) => left.role.localeCompare(right.role) || left.name.localeCompare(right.name));
      if (!filter) return rows;
      return rows.filter(row => among(filter.roles, row.path) && (!filter.name || row.name === filter.name) && holds(row.condition, filter.condition));
    },
    // Наименования всех шаблонов конфигурации — выпадающий список отбора.
    templateNames() {
      return [...new Set(entries.flatMap(entry => (roleOf(entry).restrictionTemplates || []).map(template => template.name)))].sort((left, right) => left.localeCompare(right));
    },
    // Права, которые ограничиваются хотя бы у одного из объектов, в порядке
    // прав вида.
    restrictedRights(items) {
      return (source.schema.rights || []).map(right => right.key).filter(key => items.some(item => item.rights.includes(key)));
    },
    restrictionFieldChoices, chosenFieldKeys,
    // Объекты, на права которых ставятся ограничения, — первый шаг мастера
    // «Новое ограничение доступа», по видам в порядке дерева; вложенный
    // назван вместе с тем, в чём лежит.
    restrictable() {
      return (source.schema.objects || []).filter(item => item.restrictedRights?.length).map(item => {
        let caption = localized(item) || item.name;
        for (let owner = objects.get(item.owner); owner; owner = objects.get(owner.owner)) caption = `${localized(owner) || owner.name}.${caption}`;
        return {id: item.id, kind: item.kind, title: caption, rights: item.restrictedRights};
      });
    },
    // «Новое ограничение доступа» (справка, form_NewRowLevelRightDlg): одно
    // условие — каждому выбранному объекту в каждой выбранной роли на каждое
    // отмеченное право, которое у объекта ограничивается (право без
    // ограничений writeRestrictions не пишет). Поля — только у права,
    // ограничиваемого по полям (restrictsByFields): объект получает те из отмеченных, что у него есть, а без
    // единого из них ограничения чтения не получает. Ответ — забранное, как у
    // команд списка.
    addRestrictions({objectIDs, roles, rights, paths, condition}, trial) {
      const choices = restrictionFieldChoices(objectIDs);
      return perform(trial, modelOf => roles.flatMap(role => objectIDs.flatMap(objectID =>
        rights.flatMap(right => {
          const fields = restrictsByFields(source.schema, right) ? chosenFieldKeys(choices, paths, objectID) : [];
          return fields ? taken(role, objectID, right, modelOf(role).writeRestrictions(objectID, right, [{restriction: {fields, condition}}])) : [];
        }))));
    },
    // Команды окна над выбранными строками (справка, «Все ограничения
    // доступа»). Каждая с trial прогоняется на копиях ролей и ничего не
    // меняет: так до записи известно, что она заберёт у ограничений и
    // шаблонов, которых не называла, — об этом окно спрашивает «Заменить?».
    // Ответ — забранное, по строке на роль, объект и право.
    editRestrictions(rows, change, trial) {
      return perform(trial, modelOf => byPlace(rows, (role, objectID, right, group) =>
        taken(role, objectID, right, modelOf(role).writeRestrictions(objectID, right, group.map(row => ({index: row.index,
          restriction: {fields: change.fields ? change.fields(row) : row.keys, condition: change.condition ?? row.condition}}))))));
    },
    copyRestrictions(rows, roles, trial) {
      return perform(trial, modelOf => roles.flatMap(role => byPlace(rows, (_, objectID, right, group) =>
        taken(role, objectID, right, modelOf(role).writeRestrictions(objectID, right, group.map(row => ({restriction: {fields: row.keys, condition: row.condition}})))), true)));
    },
    removeRestrictions(rows) {
      for (const row of [...rows].sort((left, right) => right.index - left.index)) at(row.roleIndex)?.removeRestriction(row.objectID, row.operation, row.index);
    },
    editTemplates(rows, change, trial) {
      return perform(trial, modelOf => byRole(rows).flatMap(([role, group]) =>
        modelOf(role).writeTemplates(group.map(row => ({index: row.index, template: {name: change.name ?? row.name, condition: change.condition ?? row.condition}})))
          .map(template => ({role: localized(entries[role].role), template: template.name}))));
    },
    addTemplate(roles, template, trial) {
      return perform(trial, modelOf => roles.flatMap(role => modelOf(role).writeTemplates([{template}])
        .map(other => ({role: localized(entries[role].role), template: other.name}))));
    },
    removeTemplates(rows) {
      for (const row of [...rows].sort((left, right) => right.index - left.index)) at(row.roleIndex)?.removeTemplate(row.index);
    },
  };
  // Поля, которые отмечаются у ограничений нескольких объектов сразу: все
  // поля всех объектов, одно и то же поле узнаётся по имени, у поля
  // табличной части — вместе с именем части. keys — ключ поля у тех
  // объектов, где оно есть. Отмеченное поле достаётся только объекту, у
  // которого оно есть (владелец 13.10.2026, проверено в конфигураторе).
  function restrictionFieldChoices(objectIDs) {
    const choices = new Map();
    for (const id of new Set(objectIDs)) {
      const paths = fieldPaths(id);
      for (const field of objects.get(id)?.restrictionFields || []) {
        const path = paths.get(field.key);
        if (!choices.has(path)) choices.set(path, {path, field, keys: new Map()});
        choices.get(path).keys.set(id, field.key);
      }
    }
    return [...choices.values()];
  }
  // Путь поля ограничения объекта по его ключу: имя, у поля табличной
  // части — через точку после имени части.
  function fieldPaths(objectID) {
    const own = objects.get(objectID)?.restrictionFields || [], byKey = new Map(own.map(field => [field.key, field]));
    return new Map(own.map(field => [field.key, field.parent ? `${byKey.get(field.parent)?.name}.${field.name}` : field.name]));
  }
  // Условия отбора: в списке — «одно из отмеченных», ничего не отмечено —
  // условия нет; текст — фрагмент без учёта регистра, пустой условия не
  // ставит.
  function among(chosen, value) { return !chosen?.size || chosen.has(value); }
  function holds(text, fragment) { return !fragment || (text || '').toLocaleLowerCase().includes(fragment.toLocaleLowerCase()); }
  // Ключи отмеченных полей у объекта; null — у объекта нет ни одного из
  // отмеченных: пустой состав стал бы «Прочими полями», а прототип такому
  // объекту ограничения по этим полям не даёт. Ничего не отмечено —
  // «Прочие поля» у каждого.
  function chosenFieldKeys(choices, paths, objectID) {
    const keys = choices.filter(choice => paths.has(choice.path)).map(choice => choice.keys.get(objectID)).filter(Boolean);
    return paths.size && !keys.length ? null : keys;
  }
  function perform(trial, operation) {
    const models = entries.map(entry => trial ? createRoleModel({role: entry.model.value(), schema: source.schema}) : entry.model);
    return operation(index => models[index]);
  }
  // Ограничения одного права пишутся вместе (writeRestrictions): строки
  // собираются по роли, объекту и праву в том порядке, в каком выбраны.
  function byPlace(rows, write, anyRole = false) {
    const groups = new Map();
    for (const row of rows) {
      const key = `${anyRole ? '' : row.roleIndex} ${row.objectID} ${row.operation}`;
      if (!groups.has(key)) groups.set(key, []);
      groups.get(key).push(row);
    }
    return [...groups.values()].flatMap(group => write(group[0].roleIndex, group[0].objectID, group[0].operation, group));
  }
  function byRole(rows) {
    const groups = new Map();
    for (const row of rows) { if (!groups.has(row.roleIndex)) groups.set(row.roleIndex, []); groups.get(row.roleIndex).push(row); }
    return [...groups];
  }
  function taken(role, objectID, right, restrictions) {
    return restrictions.map(restriction => ({role: localized(entries[role].role), object: objectTitle(objectID), operation: right,
      field: restrictionFieldsCaption(objects.get(objectID), restriction.whole ? restriction : {fields: restriction.shared}, localized), whole: restriction.whole}));
  }
}

// Окна прототипа над всеми ролями, по ключу — подпись окна.
const overviewWindows = {roles: 'Все роли', restrictions: 'Все ограничения доступа'};

function createRoleOverview(host, openRole, onChange) {
  let source, model, selected = null, tree, panel, tab = 'roles', body, filter = '';
  // Окно и его закладки (справка: «Все роли» и «Все ограничения доступа» —
  // два окна, обе команды контекстного меню ветви «Роли» и корня дерева).
  let view = 'roles';
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
  // «Все ограничения доступа» — две закладки: ограничения и шаблоны. Строки выбираются по одной или несколько сразу — с Ctrl и Shift, — и
  // команды списка действуют на все выбранные (справка, «Все ограничения
  // доступа»).
  let listChosen = new Set(), refill = () => {};
  function listChanged() { listChosen = new Set(); onChange?.(); refill(); }
  // Отбор списка — свой у каждой закладки, живёт, пока открыто окно; null —
  // отбора нет. Отбор правок не делает: перерисовывается только список.
  const noFilters = () => ({restrictions: null, templates: null});
  let listFilter = noFilters();
  function setListFilter(key, value) { listFilter[key] = value; listChosen = new Set(); refill(); }
  const filterCommands = (key, choose) => [
    ['filter', 'Установить отбор', choose, () => true],
    ['unfilter', 'Отключить отбор', () => setListFilter(key, null), () => !!listFilter[key]],
  ];
  const chosenOf = rows => [...listChosen].sort((left, right) => left - right).map(index => rows[index]).filter(Boolean);
  function renderAllRestrictions() {
    const block = node('section', undefined, 'overview-block');
    const rows = model.allRestrictions(listFilter.restrictions);
    block.append(node('p', 'Каждая строка — одно ограничение одной роли на одно право. «Реквизиты» — сколько реквизитов объекта эта роль вообще читает: ограничение сужает записи, права на реквизиты — столбцы. Несколько строк выбираются с Ctrl или Shift.', 'muted'));
    const some = () => listChosen.size > 0;
    block.append(roleListTable(['Объект', 'Роль', 'Право', 'Реквизиты', 'Поля', 'Условие'],
      rows.map(row => [[row.object], roleLink(row.role, row.path, row.objectID), [caption(row.operation)], [row.fields], [row.field], [row.condition, 'overview-condition']]),
      listChosen, chosen => { listChosen = chosen; }, [
        ['add', 'Добавить', () => newRestriction(), () => model.roleCount() > 0 && model.restrictable().length > 0],
        ['edit', 'Изменить', () => { if (some()) editRestrictions(chosenOf(rows)); }, some],
        ['copy', 'Копировать', () => { if (some()) copyRestrictions(chosenOf(rows)); }, some],
        ['remove', 'Удалить', () => { if (!some()) return; model.removeRestrictions(chosenOf(rows)); listChanged(); }, some],
        ...filterCommands('restrictions', () => filterRestrictions()),
      ], model.allRestrictions().length ? 'Ничего не найдено.' : 'В проекте нет ни одного ограничения доступа.'));
    return block;
  }
  function renderAllTemplates() {
    const block = node('section', undefined, 'overview-block');
    const all = model.allTemplates(), rows = model.allTemplates(listFilter.templates);
    block.append(node('p', all.length ? 'Шаблон принадлежит роли: условие её ограничения вызывает его как #Имя("значение", ...). Несколько строк выбираются с Ctrl или Shift.' : 'Ни одна роль не объявила шаблонов ограничений.', 'muted'));
    const some = () => listChosen.size > 0;
    block.append(roleListTable(['Роль', 'Наименование', 'Текст шаблона'],
      rows.map(row => [roleLink(row.role, row.path), [row.name], [row.condition, 'overview-condition']]),
      listChosen, chosen => { listChosen = chosen; }, [
        ['add', 'Добавить', () => addTemplate(), () => model.roleCount() > 0],
        ['edit', 'Изменить', () => { if (some()) editTemplates(chosenOf(rows)); }, some],
        ['remove', 'Удалить', () => { if (!some()) return; model.removeTemplates(chosenOf(rows)); listChanged(); }, some],
        ...filterCommands('templates', () => filterTemplates()),
      ], all.length ? 'Ничего не найдено.' : 'Шаблонов нет'));
    return block;
  }
  // Значение, общее для всех выбранных, или null, когда они расходятся.
  function common(values) { return values.every(value => value === values[0]) ? values[0] : null; }
  // Сначала операция прогоняется на копиях ролей: что она заберёт у
  // ограничений и шаблонов, которых не называла, перечисляется, и «Да»
  // заменяет их (владелец 13.10.2026), «Отмена» не меняет ничего.
  async function applyReplacing(operation, line) {
    const taken = operation(true);
    if (taken.length) {
      const list = node('ul', undefined, 'overview-taken');
      for (const item of taken) list.append(node('li', line(item)));
      const answer = await showDialog({title: 'Заменить?', explanation: 'Это уже стоит в ролях и будет заменено:', content: list, answers: [['yes', 'Да'], [null, 'Отмена']]});
      if (answer !== 'yes') return;
    }
    operation(false);
    listChanged();
  }
  const restrictionLine = item => `${item.role} · ${item.object} · ${caption(item.operation)}: ${item.field}${item.whole ? '' : ' (поле переходит к новому ограничению)'}`;
  // Окно группового «Изменить» (справка, form_EditRowLevelRightDlg): флажки
  // «Изменять …» при расходящихся значениях сняты, и до их установки поле
  // не правится.
  async function editRestrictions(rows) {
    const reading = rows.filter(row => restrictsByFields(source.schema, row.operation));
    const change = {fields: false, condition: false};
    let chosen = new Set(), fields = null;
    const choices = model.restrictionFieldChoices(reading.map(row => row.objectID));
    if (reading.length) {
      const pathsOf = row => choices.filter(choice => row.keys.includes(choice.keys.get(row.objectID))).map(choice => choice.path).sort().join('\n');
      const same = common(reading.map(pathsOf));
      change.fields = same !== null && reading.every(row => row.keys.length === (same ? same.split('\n').length : 0));
      if (change.fields) chosen = new Set(same ? same.split('\n') : []);
      fields = restrictionFieldsChoice({chosen});
      fields.fill(choiceFields(choices));
    }
    const text = common(rows.map(row => row.condition));
    change.condition = text !== null;
    let condition = text ?? '';
    const editor = queryTextField(condition, value => { condition = value; }, 'Ограничение доступа');
    const content = restrictionWindowContent({fields, editor, change, group: true});
    const answer = await showDialog({title: 'Ограничение доступа', explanation: rows.length > 1 ? `Выбрано ограничений: ${rows.length}.` : '', content,
      className: 'role-restriction-dialog', help: 'role-restriction-group',
      answers: [['ok', 'ОК', {enabled: () => !change.fields || fields.filled()}], [null, 'Отмена']]});
    if (answer !== 'ok' || (!change.fields && !change.condition)) return;
    const fieldsOf = row => model.chosenFieldKeys(choices, chosen, row.objectID) ?? row.keys;
    const edit = {
      fields: change.fields ? row => fieldsOf(row) : undefined,
      condition: change.condition ? condition : undefined,
    };
    await applyReplacing(trial => model.editRestrictions(rows, edit, trial), restrictionLine);
  }
  // Поля ограничения (restrictionFieldChoices) для restrictionFieldsChoice:
  // ключ — путь, поле табличной части — под её путём.
  const choiceFields = choices => choices.map(choice => ({key: choice.path, name: choice.field.name,
    parent: choice.field.parent ? choice.path.slice(0, choice.path.indexOf('.')) : '', hint: title(choice.field)}));
  // Флажок, отмечающий key в наборе chosen; after — после каждой отметки.
  const mark = (chosen, key, after = () => {}) => enabled => { if (enabled) chosen.add(key); else chosen.delete(key); after(); };
  // Роли, отмечаемые флажками: куда копировать ограничения, кому добавить
  // шаблон, по каким отбирать.
  function roleChoice(chosen) {
    const list = node('div', undefined, 'role-restriction-fields');
    for (const role of model.roles()) list.append(checkboxField(role.title, chosen.has(role.index), mark(chosen, role.index), 'role-check'));
    return list;
  }
  // Поля ограничения флажками (restrictionFieldChoices): поле табличной
  // части — под ней с отступом, путь — во всплывающей подсказке; отмеченные
  // пути — в chosen.
  function fieldBoxes(choices, chosen) {
    return choices.map(choice => {
      const box = checkboxField(title(choice.field) || choice.field.name, chosen.has(choice.path), mark(chosen, choice.path), 'role-check');
      if (choice.field.parent) box.classList.add('role-restriction-child');
      box.title = choice.path;
      return box;
    });
  }
  const rightBoxes = (keys, chosen, after) => keys.map(key => checkboxField(caption(key), chosen.has(key), mark(chosen, key, after), 'role-check'));
  // Объекты с ограничиваемыми правами по видам, с флажком «все» у вида и
  // кнопкой «По подсистемам» — у мастера и у окна отбора. holder.subsystems —
  // отбор по подсистемам; объект, скрытый им, из выбора уходит: в работу
  // идёт то, что видно в списке. after — после каждой перемены выбора.
  function objectChoice(chosenObjects, holder, after = () => {}) {
    const objectList = node('div', undefined, 'role-restriction-fields role-wizard-objects');
    function fillObjects() {
      const passes = subsystemFilterPasses(source.schema, holder.subsystems), groups = new Map();
      for (const item of model.restrictable()) {
        if (!passes(item.id)) { chosenObjects.delete(item.id); continue; }
        if (!groups.has(item.kind)) groups.set(item.kind, []);
        groups.get(item.kind).push(item);
      }
      objectList.replaceChildren();
      if (!groups.size) objectList.append(node('p', 'Ни один объект не входит в отмеченные подсистемы.', 'muted'));
      for (const [kind, items] of groups) {
        const boxes = [], group = node('label', undefined, 'role-check role-wizard-kind'), all = node('input');
        all.type = 'checkbox'; all.setAttribute('aria-label', `${source.kindTitles?.[kind] || kind} — все`);
        const sync = () => { const marked = items.filter(item => chosenObjects.has(item.id)).length; all.checked = marked === items.length; all.indeterminate = marked > 0 && marked < items.length; };
        all.addEventListener('change', () => { for (const item of items) if (all.checked) chosenObjects.add(item.id); else chosenObjects.delete(item.id); for (const box of boxes) box.checked = all.checked; sync(); after(); });
        group.append(all, node('span', source.kindTitles?.[kind] || kind));
        objectList.append(group);
        for (const item of items) {
          const box = checkboxField(item.title, chosenObjects.has(item.id), mark(chosenObjects, item.id, () => { sync(); after(); }), 'role-check');
          box.classList.add('role-restriction-child');
          boxes.push(box.querySelector('input')); objectList.append(box);
        }
        sync();
      }
    }
    const heading = node('div', undefined, 'overview-wizard-caption'), block = node('div', undefined, 'role-restriction-form');
    heading.append(node('span', 'Объекты', 'role-restriction-caption'), subsystemFilterButton({schema: source.schema, title, holder, applied: () => { fillObjects(); after(); }}));
    block.append(heading, objectList);
    fillObjects();
    return block;
  }
  async function copyRestrictions(rows) {
    const roles = new Set(), content = node('div', undefined, 'role-restriction-form');
    content.append(node('div', 'Роли, в которые копируются ограничения', 'role-restriction-caption'), roleChoice(roles));
    const answer = await showDialog({title: 'Копировать ограничения', explanation: `Выбрано ограничений: ${rows.length}.`, content, className: 'role-restriction-dialog',
      answers: [['ok', 'ОК'], [null, 'Отмена']], check: () => roles.size ? '' : 'Отметьте роли.'});
    if (answer !== 'ok') return;
    await applyReplacing(trial => model.copyRestrictions(rows, [...roles], trial), restrictionLine);
  }
  // Отбор мастера по подсистемам живёт, пока открыто окно.
  const wizardFilter = {subsystems: null};
  // Мастер «Новое ограничение доступа» (справка, form_NewRowLevelRightDlg):
  // на первом шаге — объекты, с отбором «По подсистемам», и роли; на втором —
  // права, поля чтения и текст ограничения. «Назад» возвращает к первому
  // шагу с тем, что на нём отмечено.
  async function newRestriction() {
    const chosenObjects = new Set(), roles = new Set(), rights = new Set(), paths = new Set();
    let step = 1, condition = '', offered = [];
    const content = node('div', undefined, 'role-restriction-form');
    const first = node('div', undefined, 'role-restriction-form'), second = node('div', undefined, 'role-restriction-form role-restriction-window');
    first.append(objectChoice(chosenObjects, wizardFilter), node('div', 'Роли', 'role-restriction-caption'), roleChoice(roles));
    // Второй шаг строится по выбранному на первом: права слева — те, что
    // ограничиваются хотя бы у одного из объектов; справа поля всех объектов
    // с ограничением по полям, «Прочие поля» и текст ограничения. Отмеченное,
    // которого больше нет, снимается. Поля доступны, пока отмечено право,
    // ограничиваемое по полям.
    const byFields = right => restrictsByFields(source.schema, right), fieldsChosen = () => [...rights].some(byFields);
    const rightList = node('div', undefined, 'role-restriction-fields role-fields-tree'), fields = restrictionFieldsChoice({chosen: paths});
    const fillRights = () => { rightList.replaceChildren(...rightBoxes(offered, rights, () => fields.disable(!fieldsChosen()))); fields.disable(!fieldsChosen()); };
    const editor = queryTextField(condition, value => { condition = value; }, 'Ограничение доступа');
    const left = node('div', undefined, 'role-restriction-form'), right = node('div', undefined, 'role-restriction-form');
    left.append(markedListCaption('Права:', enabled => { for (const key of offered) if (enabled) rights.add(key); else rights.delete(key); fillRights(); }), rightList);
    right.append(fields, node('div', 'Ограничение доступа:', 'role-restriction-caption'), editor);
    second.append(left, right);
    function fillSecond() {
      const items = model.restrictable().filter(item => chosenObjects.has(item.id));
      offered = model.restrictedRights(items);
      for (const key of [...rights]) if (!offered.includes(key)) rights.delete(key);
      fields.fill(choiceFields(model.restrictionFieldChoices(items.filter(item => item.rights.some(byFields)).map(item => item.id))));
      fillRights();
    }
    const show = () => { first.hidden = step !== 1; second.hidden = step !== 2; };
    content.append(first, second);
    show();
    // Кнопки мастера видны всегда, ненужная на шаге недоступна, как у
    // прототипа; на первом шаге «ОК» недоступна (сверено 14.10.2026).
    const answer = await showDialog({title: 'Новое ограничение доступа', content, className: 'role-restriction-dialog', help: 'role-restriction-wizard', answers: [
      ['back', '< Назад', {act: () => { step = 1; show(); }, enabled: () => step === 2}],
      ['next', 'Далее >', {act: () => { step = 2; fillSecond(); show(); }, enabled: () => step === 1, primary: () => step === 1}],
      ['ok', 'ОК', {enabled: () => step === 2 && rights.size > 0 && (!fieldsChosen() || fields.filled()), primary: () => step === 2}],
      [null, 'Отмена']],
    check: key => key === 'next' ? (!chosenObjects.size ? 'Отметьте объекты.' : !roles.size ? 'Отметьте роли.' : '') : ''});
    if (answer !== 'ok') return;
    // Только отмеченные поля, если отмечено право, ограничиваемое по полям:
    // без него поля ничего не значат.
    const chosenPaths = fieldsChosen() ? paths : new Set();
    await applyReplacing(trial => model.addRestrictions({objectIDs: [...chosenObjects], roles: [...roles], rights: [...rights], paths: chosenPaths, condition}, trial), restrictionLine);
  }
  // Роли отбора хранятся путями: номер роли после сохранения может смениться.
  const rolePaths = indexes => new Set([...indexes].map(index => model.roles()[index]?.path).filter(Boolean));
  const roleIndexes = paths => new Set(model.roles().filter(role => paths?.has(role.path)).map(role => role.index));
  // Окно отбора ограничений (справка, form_RolesGroupEditFilterDlg): объекты
  // (с «По подсистемам»), роли, права, поля и текст ограничения. Поля — те,
  // что есть у отмеченных объектов с правом, ограничиваемым по полям, а пока
  // объекты не отмечены, — у всех таких.
  async function filterRestrictions() {
    const current = listFilter.restrictions;
    const draft = {objects: new Set(current?.objects), roles: roleIndexes(current?.roles), rights: new Set(current?.rights), paths: new Set(current?.paths),
      condition: current?.condition || '', subsystems: current?.subsystems ?? null};
    const fieldList = node('div', undefined, 'role-restriction-fields');
    function fillFields() {
      const items = model.restrictable().filter(item => item.rights.some(right => restrictsByFields(source.schema, right)));
      const marked = items.filter(item => draft.objects.has(item.id));
      const choices = model.restrictionFieldChoices((marked.length ? marked : items).map(item => item.id));
      for (const path of [...draft.paths]) if (!choices.some(choice => choice.path === path)) draft.paths.delete(path);
      fieldList.replaceChildren(...fieldBoxes(choices, draft.paths));
      if (!choices.length) fieldList.append(node('p', 'У объектов нет полей для ограничения чтения.', 'muted'));
    }
    const rightList = node('div', undefined, 'role-restriction-fields');
    rightList.append(...rightBoxes(model.restrictedRights(model.restrictable()), draft.rights));
    const text = textInput(draft.condition, value => { draft.condition = value; });
    text.setAttribute('aria-label', 'Ограничение доступа');
    const content = node('div', undefined, 'role-restriction-form');
    content.append(objectChoice(draft.objects, draft, fillFields), node('div', 'Роли', 'role-restriction-caption'), roleChoice(draft.roles),
      node('div', 'Права', 'role-restriction-caption'), rightList, node('div', 'Поля', 'role-restriction-caption'), fieldList,
      node('div', 'Ограничение доступа', 'role-restriction-caption'), text);
    fillFields();
    const answer = await showDialog({title: 'Отбор', content, className: 'role-restriction-dialog', answers: [['ok', 'ОК'], [null, 'Отмена']]});
    if (answer !== 'ok') return;
    const value = {...draft, roles: rolePaths(draft.roles)};
    setListFilter('restrictions', value.objects.size || value.roles.size || value.rights.size || value.paths.size || value.condition ? value : null);
  }
  // Окно отбора шаблонов (справка, form_TemplateEditFilterDlg): роли,
  // наименование из всех шаблонов конфигурации — на равенство, фрагмент
  // текста шаблона.
  async function filterTemplates() {
    const current = listFilter.templates, names = model.templateNames();
    const draft = {roles: roleIndexes(current?.roles), name: names.includes(current?.name) ? current.name : '', condition: current?.condition || ''};
    const name = selectInput([['', ''], ...names.map(item => [item, item])], draft.name, value => { draft.name = value; }, 'Наименование');
    const text = textInput(draft.condition, value => { draft.condition = value; });
    text.setAttribute('aria-label', 'Текст шаблона');
    const content = node('div', undefined, 'role-restriction-form');
    content.append(node('div', 'Роли', 'role-restriction-caption'), roleChoice(draft.roles), node('div', 'Наименование', 'role-restriction-caption'), name,
      node('div', 'Текст шаблона', 'role-restriction-caption'), text);
    const answer = await showDialog({title: 'Установка отбора шаблонов ограничений', content, className: 'role-restriction-dialog', answers: [['ok', 'ОК'], [null, 'Отмена']]});
    if (answer !== 'ok') return;
    const value = {...draft, roles: rolePaths(draft.roles)};
    setListFilter('templates', value.roles.size || value.name || value.condition ? value : null);
  }
  const templateLine = item => `${item.role} · ${item.template}`;
  // Окно шаблонов (справка, form_EditRowTemplatesDlg): у одинаковых во
  // всех выбранных шаблонах наименования и текста флажок изменения стоит,
  // у разных — снят, поле пусто; правка поля ставит флажок.
  function templateForm(rows) {
    const content = node('div', undefined, 'role-restriction-form'), state = {};
    const name = common(rows.map(row => row.name)), text = common(rows.map(row => row.condition));
    state.name = name ?? ''; state.condition = text ?? '';
    state.changeName = name !== null; state.changeCondition = text !== null;
    const nameFlag = checkboxField('Изменять наименование', state.changeName, enabled => { state.changeName = enabled; }, 'role-check');
    const textFlag = checkboxField('Изменять текст', state.changeCondition, enabled => { state.changeCondition = enabled; }, 'role-check');
    const nameInput = textInput(state.name, value => { state.name = value; state.changeName = true; nameFlag.querySelector('input').checked = true; });
    nameInput.setAttribute('aria-label', 'Наименование');
    const editor = queryTextField(state.condition, value => { state.condition = value; state.changeCondition = true; textFlag.querySelector('input').checked = true; }, 'Текст шаблона');
    content.append(node('div', 'Наименование', 'role-restriction-caption'), nameInput, ...(rows.length > 1 ? [nameFlag] : []),
      node('div', 'Текст шаблона', 'role-restriction-caption'), editor, ...(rows.length > 1 ? [textFlag] : []));
    return {content, state};
  }
  const templateNameCheck = state => () => state.changeName && !state.name.trim() ? 'Укажите наименование шаблона.' : '';
  async function editTemplates(rows) {
    const {content, state} = templateForm(rows);
    const answer = await showDialog({title: 'Изменение шаблонов ограничений', explanation: rows.length > 1 ? `Выбрано шаблонов: ${rows.length}.` : '', content, className: 'role-restriction-dialog',
      answers: [['ok', 'ОК'], [null, 'Отмена']], check: templateNameCheck(state)});
    if (answer !== 'ok' || (!state.changeName && !state.changeCondition)) return;
    const change = {name: state.changeName ? state.name : undefined, condition: state.changeCondition ? state.condition : undefined};
    await applyReplacing(trial => model.editTemplates(rows, change, trial), templateLine);
  }
  async function addTemplate() {
    const {content, state} = templateForm([{name: '', condition: ''}]), roles = new Set();
    content.append(node('div', 'Роли, которым добавляется шаблон', 'role-restriction-caption'), roleChoice(roles));
    const answer = await showDialog({title: 'Шаблон ограничения', content, className: 'role-restriction-dialog', answers: [['ok', 'ОК'], [null, 'Отмена']],
      check: () => !state.name.trim() ? 'Укажите наименование шаблона.' : roles.size ? '' : 'Отметьте роли.'});
    if (answer !== 'ok') return;
    await applyReplacing(trial => model.addTemplate([...roles], {name: state.name, condition: state.condition}, trial), templateLine);
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
    const list = node('div');
    const fill = () => list.replaceChildren(tab === 'restrictions' ? renderAllRestrictions() : renderAllTemplates());
    refill = fill;
    listChosen = new Set();
    fill();
    body.append(list);
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
  function open(value, keep = false, next = view) {
      source = structuredClone(value); model = createRoleOverviewModel(source);
      if (!keep || next !== view) listFilter = noFilters();
      if (next !== view) { view = next; filter = ''; }
      selected = reselect();
      if (!keep) { expanded.clear(); restriction.right = null; restriction.row = -1; }
      if (!model.roles().some(role => role.path === chosenRole)) chosenRole = null;
      host.hidden = false;
      drawWindow();
  }
  // Переход в другое окно правок не трогает: модели ролей остаются.
  function show(next) {
      if (view === next) return;
      view = next; filter = ''; listFilter = noFilters();
      drawWindow();
  }
  function drawWindow() {
      tab = view === 'roles' ? 'roles' : tab === 'templates' ? 'templates' : 'restrictions';
      host.dataset.view = view;
      host.replaceChildren();
      const header = node('div', undefined, 'overview-header');
      const strip = node('div', undefined, 'overview-tabs');
      for (const [key, text] of view === 'roles' ? [] : [['restrictions', 'Ограничения доступа'], ['templates', 'Шаблоны ограничений']]) {
        const button = node('button', text, 'overview-tab');
        button.type = 'button';
        if (tab === key) button.classList.add('selected');
        button.setAttribute('aria-pressed', String(tab === key));
        button.dataset.tab = key;
        button.addEventListener('click', () => { if (tab === key) return; tab = key; drawFrame(); });
        strip.append(button);
      }
      header.append(strip, node('span', `Ролей в проекте: ${model.roleCount()}`, 'muted'));
      host.append(header);
      body = node('div'); host.append(body);
      renderBody();
  }
  // Смена закладки перерисовывает окно, не трогая правок: модели ролей остаются.
  function drawFrame() {
    for (const button of host.querySelectorAll('.overview-tab')) {
      const on = button.dataset.tab === tab;
      button.classList.toggle('selected', on); button.setAttribute('aria-pressed', String(on));
    }
    renderBody();
  }
  return {
    open,
    show,
    title() { return overviewWindows[view]; },
    // Роли, изменённые в окне, — для сохранения одним запросом.
    value() { return model?.changes() || []; },
    close() { source = null; model = null; host.hidden = true; host.replaceChildren(); },
    setDisabled(disabled) { host.inert = disabled; },
  };
}
