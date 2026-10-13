/* «Все роли» (справка, «Все роли»): права выбранного объекта во всех ролях
   сразу и с правкой — права строками, роли столбцами; первая строка ставит
   или снимает все права роли, первая колонка — право во всех ролях; ниже —
   ограничения доступа выбранной роли. Две вкладки рядом — все ограничения
   и все шаблоны проекта — с командами прототипа над несколькими строками
   сразу: изменить группой, копировать в роли, удалить (справка, «Все
   ограничения доступа»). */

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
    allRestrictions() {
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
      return rows;
    },
    // Шаблон показывается как он написан: имя с параметрами в скобках и
    // текст, в который ограничение подставляет значения параметров.
    allTemplates() {
      const rows = [];
      entries.forEach((entry, roleIndex) => {
        (roleOf(entry).restrictionTemplates || []).forEach((template, index) => {
          rows.push({path: entry.path, roleIndex, index, role: localized(entry.role), name: template.name, condition: template.condition || ''});
        });
      });
      rows.sort((left, right) => left.role.localeCompare(right.role) || left.name.localeCompare(right.name));
      return rows;
    },
    // Поля, которые отмечаются у ограничений нескольких объектов сразу: все
    // поля всех объектов, одно и то же поле узнаётся по имени, у поля
    // табличной части — вместе с именем части. keys — ключ поля у тех
    // объектов, где оно есть. Отмеченное поле достаётся только объекту, у
    // которого оно есть (владелец 13.10.2026, проверено в конфигураторе).
    restrictionFieldChoices(objectIDs) {
      const choices = new Map();
      for (const id of new Set(objectIDs)) {
        const own = objects.get(id)?.restrictionFields || [], byKey = new Map(own.map(field => [field.key, field]));
        for (const field of own) {
          const path = field.parent ? `${byKey.get(field.parent)?.name}.${field.name}` : field.name;
          if (!choices.has(path)) choices.set(path, {path, field, keys: new Map()});
          choices.get(path).keys.set(id, field.key);
        }
      }
      return [...choices.values()];
    },
    // Ключи отмеченных полей у объекта; null — у объекта нет ни одного из
    // отмеченных: пустой состав стал бы «Прочими полями», а прототип такому
    // объекту ограничения по этим полям не даёт. Ничего не отмечено —
    // «Прочие поля» у каждого.
    chosenFieldKeys(choices, paths, objectID) {
      const keys = choices.filter(choice => paths.has(choice.path)).map(choice => choice.keys.get(objectID)).filter(Boolean);
      return paths.size && !keys.length ? null : keys;
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
  // Строки выбираются по одной или несколько сразу — с Ctrl и Shift, — и
  // команды списка действуют на все выбранные (справка, «Все ограничения
  // доступа»).
  let listChosen = new Set(), refill = () => {};
  function listChanged() { listChosen = new Set(); onChange?.(); refill(); }
  function restrictionRows() { return model.allRestrictions().filter(row => matches(row, [row.object, row.role, row.field, row.condition])); }
  function templateRows() { return model.allTemplates().filter(row => matches(row, [row.role, row.name, row.condition])); }
  const chosenOf = rows => [...listChosen].sort((left, right) => left - right).map(index => rows[index]).filter(Boolean);
  function renderAllRestrictions() {
    const block = node('section', undefined, 'overview-block');
    const rows = restrictionRows();
    if (!rows.length) { block.append(node('p', model.allRestrictions().length ? 'Ничего не найдено.' : 'В проекте нет ни одного ограничения доступа.', 'muted')); return block; }
    block.append(node('p', 'Каждая строка — одно ограничение одной роли на одно право. «Реквизиты» — сколько реквизитов объекта эта роль вообще читает: ограничение сужает записи, права на реквизиты — столбцы. Несколько строк выбираются с Ctrl или Shift.', 'muted'));
    const some = () => listChosen.size > 0;
    block.append(roleListTable(['Объект', 'Роль', 'Право', 'Реквизиты', 'Поля', 'Условие'],
      rows.map(row => [[row.object], roleLink(row.role, row.path, row.objectID), [caption(row.operation)], [row.fields], [row.field], [row.condition, 'overview-condition']]),
      listChosen, chosen => { listChosen = chosen; }, [
        ['edit', 'Изменить', () => { if (some()) editRestrictions(chosenOf(rows)); }, some],
        ['copy', 'Копировать', () => { if (some()) copyRestrictions(chosenOf(rows)); }, some],
        ['remove', 'Удалить', () => { if (!some()) return; model.removeRestrictions(chosenOf(rows)); listChanged(); }, some],
      ], ''));
    return block;
  }
  function renderAllTemplates() {
    const block = node('section', undefined, 'overview-block');
    const all = model.allTemplates(), rows = templateRows();
    block.append(node('p', all.length ? 'Шаблон принадлежит роли: условие её ограничения вызывает его как #Имя("значение", ...). Несколько строк выбираются с Ctrl или Shift.' : 'Ни одна роль не объявила шаблонов ограничений.', 'muted'));
    const some = () => listChosen.size > 0;
    block.append(roleListTable(['Роль', 'Наименование', 'Текст шаблона'],
      rows.map(row => [roleLink(row.role, row.path), [row.name], [row.condition, 'overview-condition']]),
      listChosen, chosen => { listChosen = chosen; }, [
        ['add', 'Добавить', () => addTemplate(), () => model.roleCount() > 0],
        ['edit', 'Изменить', () => { if (some()) editTemplates(chosenOf(rows)); }, some],
        ['remove', 'Удалить', () => { if (!some()) return; model.removeTemplates(chosenOf(rows)); listChanged(); }, some],
      ], all.length ? 'Ничего не найдено.' : 'Шаблонов нет'));
    return block;
  }
  // Значение, общее для всех выбранных, или null, когда они расходятся.
  function common(values) { return values.every(value => value === values[0]) ? values[0] : null; }
  // Флажок «Изменять …» над полем окна группового изменения: при
  // расходящихся значениях снят, и поле до его установки не правится
  // (справка, form_EditRowLevelRightDlg).
  function changeFlag(label, checked, controls, apply) {
    const field = checkboxField(label, checked, enabled => { apply(enabled); for (const control of controls()) control.disabled = !enabled; }, 'role-check');
    for (const control of controls()) control.disabled = !checked;
    return field;
  }
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
  async function editRestrictions(rows) {
    const content = node('div', undefined, 'role-restriction-form'), reading = rows.filter(row => row.operation === 'read');
    let changeFields = false, changeCondition = false, chosen = new Set();
    const choices = model.restrictionFieldChoices(reading.map(row => row.objectID));
    if (reading.length) {
      const pathsOf = row => choices.filter(choice => row.keys.includes(choice.keys.get(row.objectID))).map(choice => choice.path).sort().join('\n');
      const same = common(reading.map(pathsOf)), boxes = [];
      changeFields = same !== null && reading.every(row => row.keys.length === (same ? same.split('\n').length : 0));
      if (changeFields) chosen = new Set(same ? same.split('\n') : []);
      const fields = node('div', undefined, 'role-restriction-fields');
      for (const choice of choices) {
        const box = checkboxField(title(choice.field) || choice.field.name, chosen.has(choice.path), enabled => { if (enabled) chosen.add(choice.path); else chosen.delete(choice.path); }, 'role-check');
        if (choice.field.parent) box.classList.add('role-restriction-child');
        box.title = choice.path; boxes.push(box.querySelector('input')); fields.append(box);
      }
      content.append(changeFlag('Изменять состав полей', changeFields, () => boxes, enabled => { changeFields = enabled; }),
        node('div', 'Поля — ни одно не отмечено: «Прочие поля»', 'role-restriction-caption'), fields);
    }
    const text = common(rows.map(row => row.condition));
    changeCondition = text !== null;
    let condition = text ?? '';
    const editor = queryTextField(condition, value => { condition = value; }, 'Ограничение доступа');
    content.append(changeFlag('Изменять ограничение', changeCondition, () => [editor.textarea], enabled => { changeCondition = enabled; }), node('div', 'Ограничение доступа', 'role-restriction-caption'), editor);
    const answer = await showDialog({title: 'Ограничение доступа', explanation: rows.length > 1 ? `Выбрано ограничений: ${rows.length}.` : '', content, className: 'role-restriction-dialog', answers: [['ok', 'ОК'], [null, 'Отмена']]});
    if (answer !== 'ok' || (!changeFields && !changeCondition)) return;
    const fieldsOf = row => model.chosenFieldKeys(choices, chosen, row.objectID) ?? row.keys;
    const change = {
      fields: changeFields ? row => fieldsOf(row) : undefined,
      condition: changeCondition ? condition : undefined,
    };
    await applyReplacing(trial => model.editRestrictions(rows, change, trial), restrictionLine);
  }
  // Роли, отмечаемые флажками: куда копировать ограничения, кому добавить
  // шаблон.
  function roleChoice(chosen) {
    const list = node('div', undefined, 'role-restriction-fields');
    for (const role of model.roles()) list.append(checkboxField(role.title, chosen.has(role.index), enabled => { if (enabled) chosen.add(role.index); else chosen.delete(role.index); }, 'role-check'));
    return list;
  }
  async function copyRestrictions(rows) {
    const roles = new Set(), content = node('div', undefined, 'role-restriction-form');
    content.append(node('div', 'Роли, в которые копируются ограничения', 'role-restriction-caption'), roleChoice(roles));
    const answer = await showDialog({title: 'Копировать ограничения', explanation: `Выбрано ограничений: ${rows.length}.`, content, className: 'role-restriction-dialog',
      answers: [['ok', 'ОК'], [null, 'Отмена']], check: () => roles.size ? '' : 'Отметьте роли.'});
    if (answer !== 'ok') return;
    await applyReplacing(trial => model.copyRestrictions(rows, [...roles], trial), restrictionLine);
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
    const search = node('input');
    search.type = 'search'; search.placeholder = 'Поиск по таблице'; search.setAttribute('aria-label', 'Поиск по таблице');
    search.value = filter;
    // Перерисовывается только сама таблица: поле поиска остаётся тем же
    // элементом, иначе каждая набранная буква уводила бы из него фокус.
    const list = node('div');
    const fill = () => list.replaceChildren(tab === 'restrictions' ? renderAllRestrictions() : renderAllTemplates());
    refill = fill;
    search.addEventListener('input', () => { filter = search.value; listChosen = new Set(); fill(); });
    listChosen = new Set();
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
