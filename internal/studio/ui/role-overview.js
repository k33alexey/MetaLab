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
  function localized(item) { return localizedTitle(item, source.defaultLanguage); }
  function objectTitle(id) {
    const object = objects.get(id);
    return object ? localized(object) : 'удалённый объект';
  }
  function describe(objectID, restriction) {
    return {operation: restriction.right, field: restrictionFieldsCaption(objects.get(objectID), restriction, localized), condition: restriction.condition || ''};
  }
  /* Строки окна по выбранному в дереве: права объекта или права части, у
     каждой — как прочитать и записать её в роли. */
  function lines(selection) {
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
    roles: roleList,
    model: at,
    rows(selection) { return lines(selection).map(({key}) => ({key})); },
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
    // условию. Поля — не «любое из отмеченных», как у других списков:
    // ограничение должно называть все отмеченные поля, свои сверх них не
    // мешают, «Прочие поля» не проходят (сверено в конфигураторе
    // 18.10.2026, docs/block-00/vse-ogranicheniya-dostupa.md).
    allRestrictions(filter = null) {
      const rows = restrictionRows();
      if (!filter) return rows;
      const pathsOf = new Map();
      const fieldPassed = row => {
        if (!filter.paths?.size) return true;
        if (!pathsOf.has(row.objectID)) pathsOf.set(row.objectID, fieldPaths(row.objectID));
        const named = new Set(row.keys.map(key => pathsOf.get(row.objectID).get(key)));
        return [...filter.paths].every(path => named.has(path));
      };
      return rows.filter(row => among(filter.objects, row.objectID) && among(filter.roles, row.path) && among(filter.rights, row.operation)
        && fieldPassed(row) && holds(row.condition, filter.condition));
    },
    // Что предлагает окно отбора ограничений (сверено в конфигураторе
    // 14.10.2026): только встречающееся в ограничениях, а не всё в
    // конфигурации. Объекты — в порядке дерева, роли — в порядке ролей окна,
    // права — в порядке прав, поля — по объектам в порядке их полей, без
    // «Прочих полей»; от отмеченных объектов список полей не зависит.
    restrictionFilterChoices,
    // «Отбор по значению в текущей колонке» (сверено в конфигураторе
    // 18.10.2026, docs/block-00/vse-ogranicheniya-dostupa.md) правит условие
    // колонки текущей ячейки в том же отборе, что окно, другие не трогает.
    // Списки — переключателем: значения ячейки уже в условии — они
    // снимаются, нет — добавляются; у ячейки с несколькими полями снимаются
    // все её поля. «Прочие поля» отбора не меняют. Наименование шаблона — на
    // равенство, текст — «содержит» весь текст ячейки; повтор снимает. Без
    // единого условия отбора нет — ответ null.
    byValue(filter, row, column) {
      const next = {...filter};
      const toggle = (key, values) => {
        const current = [...filter?.[key] || []];
        next[key] = new Set(values.some(value => current.includes(value)) ? current.filter(value => !values.includes(value)) : [...current, ...values]);
      };
      if (column === 'object') toggle('objects', [row.objectID]);
      else if (column === 'role') toggle('roles', [row.path]);
      else if (column === 'operation') toggle('rights', [row.operation]);
      else if (column === 'field') {
        if (!row.keys.length) return filter;
        const pathOf = fieldPaths(row.objectID);
        toggle('paths', row.keys.map(key => pathOf.get(key)));
      } else if (column === 'name' || column === 'condition') next[column] = filter?.[column] === row[column] ? '' : row[column];
      else return filter;
      return activeFilter(next);
    },
    activeFilter,
    // Строка «Отбор:» под списком (сверено в конфигураторе 14.10.2026):
    // условия через «; », в каждом отмеченное — в порядке списка окна.
    restrictionFilterText(filter) {
      if (!filter) return '';
      const choices = restrictionFilterChoices(), titles = new Map(restrictable().map(item => [item.id, item.title]));
      return [
        marked('Объекты', [...titles.keys()].filter(id => filter.objects?.has(id)).map(id => titles.get(id))),
        marked('Роли', roleList().filter(role => filter.roles?.has(role.path)).map(role => role.title)),
        marked('Права', rightKeys().filter(key => filter.rights?.has(key)).map(rightTitle)),
        marked('Поля', choices.fields.filter(choice => filter.paths?.has(choice.path)).map(choice => choice.caption)),
        valueText('Ограничение доступа содержит', filter.condition),
      ].filter(Boolean).join('; ');
    },
    templateFilterText(filter) {
      if (!filter) return '';
      return [
        marked('Роли', roleList().filter(role => filter.roles?.has(role.path)).map(role => role.title)),
        valueText('Наименование', filter.name),
        valueText('Текст шаблона содержит', filter.condition),
      ].filter(Boolean).join('; ');
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
      return rightKeys().filter(key => items.some(item => item.rights.includes(key)));
    },
    restrictionFieldChoices, chosenFieldKeys, restrictable,
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
      return perform(trial, modelOf => writePlaces(modelOf, rows, undefined, row => ({index: row.index,
        restriction: {fields: change.fields ? change.fields(row) : row.keys, condition: change.condition ?? row.condition}})));
    },
    // «Копировать шаблоны ограничений»: выбранные шаблоны — в отмеченные
    // роли, как ограничения (справка, form_CopyRowLevelInstrDlg); шаблон с
    // тем же наименованием в роли заменяется — об этом окно спрашивает.
    copyTemplates(rows, roles, trial) {
      return perform(trial, modelOf => roles.flatMap(role => takenTemplates(role, modelOf(role).writeTemplates(rows.map(row => ({template: {name: row.name, condition: row.condition}}))))));
    },
    copyRestrictions(rows, roles, trial) {
      return perform(trial, modelOf => roles.flatMap(role => writePlaces(modelOf, rows, role, row => ({restriction: {fields: row.keys, condition: row.condition}}))));
    },
    removeRestrictions(rows) {
      for (const row of fromLast(rows)) at(row.roleIndex)?.removeRestriction(row.objectID, row.operation, row.index);
    },
    editTemplates(rows, change, trial) {
      return perform(trial, modelOf => byRole(rows).flatMap(([role, group]) =>
        takenTemplates(role, modelOf(role).writeTemplates(group.map(row => ({index: row.index, template: {name: change.name ?? row.name, condition: change.condition ?? row.condition}}))))));
    },
    addTemplate(roles, template, trial) {
      return perform(trial, modelOf => roles.flatMap(role => takenTemplates(role, modelOf(role).writeTemplates([{template}]))));
    },
    removeTemplates(rows) {
      for (const row of fromLast(rows)) at(row.roleIndex)?.removeTemplate(row.index);
    },
  };
  function roleList() { return entries.map((entry, index) => ({index, path: entry.path, name: entry.role.name, title: localized(entry.role)})); }
  function rightKeys() { return (source.schema.rights || []).map(right => right.key); }
  function rightTitle(key) { return source.schema.rights?.find(right => right.key === key)?.title || key; }
  // Объекты, на права которых ставятся ограничения, — первый шаг мастера
  // «Новое ограничение доступа» и окно отбора, по видам в порядке дерева;
  // вложенный назван вместе с тем, в чём лежит.
  function restrictable() {
    return (source.schema.objects || []).filter(item => item.restrictedRights?.length).map(item => {
      let caption = localized(item) || item.name;
      for (let owner = objects.get(item.owner); owner; owner = objects.get(owner.owner)) caption = `${localized(owner) || owner.name}.${caption}`;
      return {id: item.id, kind: item.kind, title: caption, rights: item.restrictedRights};
    });
  }
  // Все ограничения всех ролей, по объекту и роли.
  function restrictionRows() {
    const rows = [];
    entries.forEach((entry, roleIndex) => {
      for (const grant of roleOf(entry).objects || []) {
        if (!objects.has(grant.object)) continue;
        const counted = new Map();
        for (const restriction of grant.restrictions || []) {
          const index = counted.get(restriction.right) || 0;
          counted.set(restriction.right, index + 1);
          rows.push({path: entry.path, roleIndex, index, objectID: grant.object, object: objectTitle(grant.object),
            role: localized(entry.role), keys: restriction.fields || [], ...describe(grant.object, restriction)});
        }
      }
    });
    return rows.sort((left, right) => left.object.localeCompare(right.object) || left.role.localeCompare(right.role));
  }
  // Поля, которые отмечаются у ограничений нескольких объектов сразу: все
  // поля всех объектов, одно и то же поле узнаётся по имени, у поля
  // табличной части — вместе с именем части. keys — ключ поля у тех
  // объектов, где оно есть, caption — подпись, у поля части — через точку
  // после подписи части. Отмеченное поле достаётся только объекту, у
  // которого оно есть (владелец 13.10.2026, проверено в конфигураторе).
  function restrictionFieldChoices(objectIDs) {
    const choices = new Map();
    for (const id of new Set(objectIDs)) {
      const paths = fieldPaths(id), own = objects.get(id)?.restrictionFields || [];
      const named = field => localized(field) || field.name;
      for (const field of own) {
        const path = paths.get(field.key), part = field.parent && own.find(item => item.key === field.parent);
        if (!choices.has(path)) choices.set(path, {path, field, caption: part ? `${named(part)}.${named(field)}` : named(field), keys: new Map()});
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
  // Отбор с условием — как есть, без единого условия — null: отмеченного
  // нет (пустые списки, пустые наименование и текст), отбора тоже.
  function activeFilter(filter) {
    return Object.values(filter || {}).some(item => item instanceof Set ? item.size > 0 : typeof item === 'string' && item !== '') ? filter : null;
  }
  // Условие строки «Отбор:»: подпись и отмеченное через запятую, значение —
  // в одну строку; пустое условия не даёт.
  function marked(label, titles) { return titles.length ? `${label}: ${titles.join(', ')}` : ''; }
  function oneLine(value) { return String(value).replace(/\s+/g, ' ').trim(); }
  function valueText(label, value) { return value ? `${label}: ${oneLine(value)}` : ''; }
  function restrictionFilterChoices() {
    const rows = restrictionRows(), used = new Set(rows.map(row => row.objectID)), paths = new Set();
    for (const row of rows) { const pathOf = fieldPaths(row.objectID); for (const key of row.keys) paths.add(pathOf.get(key)); }
    const items = restrictable().filter(item => used.has(item.id));
    return {objects: items, roles: roleList().filter(role => rows.some(row => row.path === role.path)),
      rights: rightKeys().filter(key => rows.some(row => row.operation === key)),
      fields: restrictionFieldChoices(items.map(item => item.id)).filter(choice => paths.has(choice.path))};
  }
  // Ключи отмеченных полей у объекта; null — у объекта нет ни одного из
  // отмеченных: мастер такому объекту ограничения по этим полям не даёт,
  // групповое «Изменить» даёт ему «Прочие поля». Ничего не отмечено —
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
  // собираются по роли, объекту и праву в том порядке, в каком выбраны, и
  // entry(row) пишется в роль toRole, а без неё — в роль самой строки (тогда
  // и собираются по роли). Ответ — забранное (taken).
  function writePlaces(modelOf, rows, toRole, entry) {
    const groups = new Map();
    for (const row of rows) {
      const key = `${toRole === undefined ? row.roleIndex : ''} ${row.objectID} ${row.operation}`;
      if (!groups.has(key)) groups.set(key, []);
      groups.get(key).push(row);
    }
    return [...groups.values()].flatMap(group => {
      const role = toRole ?? group[0].roleIndex, {objectID, operation} = group[0];
      return taken(role, objectID, operation, modelOf(role).writeRestrictions(objectID, operation, group.map(entry)));
    });
  }
  function byRole(rows) {
    const groups = new Map();
    for (const row of rows) { if (!groups.has(row.roleIndex)) groups.set(row.roleIndex, []); groups.get(row.roleIndex).push(row); }
    return [...groups];
  }
  // Удаление нескольких строк — с последней: номера ещё не удалённых не
  // сдвигаются.
  function fromLast(rows) { return [...rows].sort((left, right) => right.index - left.index); }
  // Шаблоны, которые запись забрала у роли, — строки вопроса «Заменить?».
  function takenTemplates(role, templates) { return templates.map(template => ({role: localized(entries[role].role), template: template.name})); }
  function taken(role, objectID, right, restrictions) {
    return restrictions.map(restriction => ({role: localized(entries[role].role), object: objectTitle(objectID), operation: right,
      field: restrictionFieldsCaption(objects.get(objectID), restriction.whole ? restriction : {fields: restriction.shared}, localized), whole: restriction.whole}));
  }
}

// Окна прототипа над всеми ролями, по ключу — подпись окна.
const overviewWindows = {roles: 'Все роли', restrictions: 'Все ограничения доступа'};

// Списки окна «Все ограничения доступа», по закладке: колонки прототипа в его
// порядке и их стандартная настройка (сверено в конфигураторе 18.10.2026,
// docs/block-00/vse-ogranicheniya-dostupa.md, снимки vse-ogr-12, vse-ogr-15,
// shabl-03): ширина 14 символов, высота в строку; у «Ограничения доступа»
// высота 5 с автовысотой. Список открывается в конце, «Автоматически» — в
// начале (он не упорядочен по дате), ограничения упорядочены по объекту,
// шаблоны — по роли (снимки vse-ogr-10, shabl-01).
const overviewLists = {
  restrictions: createListSettings({key: 'all-restrictions', opening: 'end', autoOpening: 'beginning', sort: {column: 'object'}, columns: [
    {key: 'object', title: 'Объект'}, {key: 'role', title: 'Роль'}, {key: 'operation', title: 'Право'}, {key: 'field', title: 'Поля'},
    {key: 'condition', title: 'Ограничение доступа', height: 5, autoHeight: true}]}),
  // «Роль» и «Наименование» — стандартные, «Текст шаблона» — 38 знаков, до
  // 10 строк по тексту (снимки configurator-2026-10-19/shabl-19, -20).
  templates: createListSettings({key: 'all-templates', opening: 'end', autoOpening: 'beginning', sort: {column: 'role'}, columns: [
    {key: 'role', title: 'Роль'}, {key: 'name', title: 'Наименование'},
    {key: 'condition', title: 'Текст шаблона', width: 38, height: 10, autoHeight: true}]}),
};

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
  function rowCaption(row) { return caption(row.key); }
  function renderGrid() {
    const roles = model.roles(), rows = model.rows(selected);
    if (!roles.length) return node('p', 'В проекте нет ролей.', 'muted');
    if (!rows.length) return node('p', selected.part ? 'Своих прав у этого узла нет: права даются на то, что в нём.' : 'Своих прав у объекта нет: права даются на его части.', 'muted');
    const scroll = node('div', undefined, 'overview-grid'), table = node('table'), head = node('tr');
    head.append(node('th', 'Право', 'overview-right'));
    for (const role of roles) {
      const cell = node('th', undefined, role.path === chosenRole ? 'overview-chosen' : undefined), button = node('button', role.title, 'overview-role');
      button.type = 'button'; button.title = role.name;
      button.addEventListener('click', () => { chosenRole = role.path; restriction.row = -1; renderPanel(); });
      cell.append(button); head.append(cell);
    }
    const thead = node('thead'), tbody = node('tbody');
    thead.append(head);
    const all = node('tr', undefined, 'overview-all'), allCaption = node('th', 'Все права', 'overview-right');
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
  // Роль в ячейке списка — ссылкой, открывающей роль.
  function roleLink(text, path, objectID) {
    const link = node('button', text, 'overview-role');
    link.type = 'button';
    link.addEventListener('click', () => openRole(path, objectID));
    return link;
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
  const chosenOf = rows => [...listChosen].sort((left, right) => left - right).map(index => rows[index]).filter(Boolean);
  // Настройка списков окна (overviewLists): прочитанная у пользователя, null
  // — ещё не прочитана. Настройки приходят с сервера после открытия
  // страницы, и окно, открытое раньше, показывается по ним заново.
  const listSettings = {restrictions: null, templates: null}, shownRows = {restrictions: [], templates: []};
  StudioListSettings.load().then(() => {
    listSettings.restrictions = listSettings.templates = null;
    if (source && tab !== 'roles') renderBody();
  });
  function settingsOf(key) { return listSettings[key] ??= overviewLists[key].read(StudioListSettings.get(overviewLists[key].key)); }
  // Строка списка узнаётся по месту своего ограничения или шаблона, а не по
  // номеру: выбранные остаются выбранными и после смены порядка.
  const rowKey = row => `${row.path}\n${row.objectID ?? ''}\n${row.operation ?? ''}\n${row.index}`;
  let keptRows = null, opening = false, revealing = false;
  // Новая настройка списка key: запоминается у пользователя и сразу
  // показывается.
  function setListSettings(key, next) {
    keptRows = new Set(chosenOf(shownRows[key]).map(rowKey));
    listSettings[key] = next;
    StudioListSettings.save(overviewLists[key].key, next);
    refill();
  }
  // Текст ячейки колонки column, по которому список упорядочивается.
  const columnText = {
    restrictions: (row, column) => column === 'operation' ? caption(row.operation) : row[column],
    templates: (row, column) => row[column],
  };
  // Строки списка key в порядке его настройки. Окно, только что открытое,
  // встаёт на строку, куда велит «При открытии переходить» (снимок
  // vse-ogr-18: в конце списка, последняя строка текущая).
  function arranged(key, rows) {
    const list = overviewLists[key], settings = settingsOf(key), sorted = list.sorted(rows, settings, columnText[key]);
    shownRows[key] = sorted;
    if (opening) {
      const at = list.openingRow(settings, sorted.length);
      listChosen = new Set(at < 0 ? [] : [at]);
      opening = false; revealing = true;
    } else if (keptRows) listChosen = new Set(sorted.flatMap((row, index) => keptRows.has(rowKey(row)) ? [index] : []));
    keptRows = null;
    return sorted;
  }
  // Раскладка списка key по его настройке: щелчок по заголовку колонки
  // упорядочивает по ней, перетаскивание края заголовка меняет ширину.
  function layoutOf(key) {
    const list = overviewLists[key];
    return listLayout(list, settingsOf(key), {onSort: column => setListSettings(key, list.sortedBy(settingsOf(key), column)), onSettings: next => setListSettings(key, next)});
  }
  // Команды «Действия» списка в порядке прототипа, группами через черту, с
  // клавишами (сверено в конфигураторе 14 и 18.10.2026,
  // docs/block-00/vse-ogranicheniya-dostupa.md): правка, копирование,
  // отбор, настройка. Без строки недоступны правка, удаление, копирование и
  // отбор по значению; «Отключить отбор» и «Настройка списка...» доступны
  // всегда. «Отбор по значению в текущей колонке» берёт ячейку, щёлкнутую
  // последней. «Настройка списка...» — только в меню: кнопки на панели у
  // неё нет (снимок vse-ogr-01). Удаление спрашивает removing — [вопрос об
  // одной строке, о нескольких]. «Вывести список...» — свой пункт карты
  // (блок 11). Значки команд — стандартные картинки прототипа (снимок
  // vse-ogr-01, ICONS.md): на панели команда показана только значком.
  function listCommands(key, rows, {add, canAdd, edit, copy, copyText, remove, removing, choose}) {
    const some = () => listChosen.size > 0;
    return [
      ['add', 'Добавить', add, canAdd, '', 'Ins', false, 'add'],
      ['edit', 'Изменить', () => { if (some()) edit(chosenOf(rows)); }, some, '', 'F2', false, 'change'],
      ['remove', 'Удалить', async () => {
        if (!some()) return;
        const chosen = chosenOf(rows);
        if (await askChoice(removing[chosen.length > 1 ? 1 : 0], '', [['yes', 'Да'], [null, 'Нет']]) !== 'yes') return;
        remove(chosen); listChanged();
      }, some, '', 'Del', false, 'delete'],
      null,
      ['copy', 'Копировать', () => { if (some()) copy(chosenOf(rows)); }, some, copyText, '', false, 'clone'],
      null,
      ['filter', 'Установить отбор', choose, () => true, 'Установить отбор...', '', false, 'filter-and-sort'],
      ['by-value', 'Отбор по значению', ({row, column}) => {
        if (!rows[row]) return;
        const next = model.byValue(listFilter[key], rows[row], column);
        if (next !== listFilter[key]) setListFilter(key, next);
      }, some, 'Отбор по значению в текущей колонке', '', false, 'filter-by-current-value'],
      ['unfilter', 'Отключить отбор', () => setListFilter(key, null), () => true, '', '', false, 'clear-filter'],
      null,
      ['settings', 'Настройка списка', async () => {
        const next = await chooseListSettings(overviewLists[key], settingsOf(key), applied => setListSettings(key, applied));
        if (next) setListSettings(key, next);
      }, () => true, 'Настройка списка...', '', true, 'list-settings'],
    ];
  }
  // Под списком — строка «Отбор:» с условиями действующего отбора.
  const filterLine = text => node('p', `Отбор: ${text}`, 'overview-filter-line');
  // Закладка окна — список key: пояснение, сам список с выбором окна, его
  // командами (listCommands) и раскладкой по настройке, под ним — строка
  // «Отбор:». cells(row) — ячейки строки в порядке колонок списка.
  function listBlock(key, intro, rows, cells, commands, empty, filterText) {
    const block = node('section', undefined, 'overview-block');
    block.append(node('p', intro, 'muted'),
      roleListTable([], rows.map(cells), listChosen, chosen => { listChosen = chosen; }, listCommands(key, rows, commands), empty, {actions: true, layout: layoutOf(key)}),
      filterLine(filterText));
    return block;
  }
  function renderAllRestrictions() {
    const rows = arranged('restrictions', model.allRestrictions(listFilter.restrictions));
    return listBlock('restrictions', 'Каждая строка — одно ограничение одной роли на одно право. Несколько строк выбираются с Ctrl или Shift.', rows,
      row => [[row.object], roleLink(row.role, row.path, row.objectID), [caption(row.operation)], [row.field], [row.condition, 'overview-condition']], {
        add: () => newRestriction(), canAdd: () => model.roleCount() > 0 && model.restrictable().length > 0,
        edit: editRestrictions, copy: copyRestrictions, copyText: 'Копировать ограничения доступа', remove: chosen => model.removeRestrictions(chosen),
        removing: ['Удалить ограничение доступа?', 'Удалить ограничения доступа?'],
        choose: () => filterRestrictions(),
      }, model.allRestrictions().length ? 'Ничего не найдено.' : 'В проекте нет ни одного ограничения доступа.', model.restrictionFilterText(listFilter.restrictions));
  }
  function renderAllTemplates() {
    const all = model.allTemplates(), rows = arranged('templates', model.allTemplates(listFilter.templates));
    return listBlock('templates', all.length ? 'Шаблон принадлежит роли: условие её ограничения вызывает его как #Имя("значение", ...). Несколько строк выбираются с Ctrl или Shift.' : 'Ни одна роль не объявила шаблонов ограничений.', rows,
      row => [roleLink(row.role, row.path), [row.name], [row.condition, 'overview-condition']], {
        add: () => addTemplate(), canAdd: () => model.roleCount() > 0,
        edit: editTemplates, copy: copyTemplates, copyText: 'Копировать шаблоны ограничений', remove: chosen => model.removeTemplates(chosen),
        removing: ['Удалить шаблон?', 'Удалить шаблоны?'],
        choose: () => filterTemplates(),
      }, all.length ? 'Ничего не найдено.' : 'Шаблонов нет', model.templateFilterText(listFilter.templates));
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
  // не правится. Объект, у которого нет ни одного из отмеченных полей,
  // получает «Прочие поля», а не сохраняет свои; «ОК» недоступна, пока
  // окно ничего не меняет (сверено в конфигураторе 14.10.2026, ACCESS-RIGHTS
  // «Правка ограничений группой»).
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
    // Поля пишутся только праву, ограничиваемому по полям (restrictionEntry
    // роли); у прочих они пусты, и сравнение «меняет ли окно» их не задевает.
    const fieldsOf = row => restrictsByFields(source.schema, row.operation) ? model.chosenFieldKeys(choices, chosen, row.objectID) ?? [] : row.keys;
    const sameKeys = (left, right) => left.length === right.length && left.every(key => right.includes(key));
    const changes = () => rows.some(row => (change.fields && !sameKeys(fieldsOf(row), row.keys)) || (change.condition && condition !== row.condition));
    const answer = await showDialog({title: 'Ограничение доступа', explanation: rows.length > 1 ? `Выбрано ограничений: ${rows.length}.` : '', content,
      className: 'role-restriction-dialog', help: 'role-restriction-group',
      answers: [['ok', 'ОК', {enabled: () => (!change.fields || fields.filled()) && changes()}], [null, 'Отмена']]});
    if (answer !== 'ok' || !changes()) return;
    const edit = {
      fields: change.fields ? fieldsOf : undefined,
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
  // Роли, отмечаемые флажками: куда копировать, кому добавить шаблон; роли
  // except не предлагаются.
  function roleChoice(chosen, except = new Set()) {
    const list = node('div', undefined, 'role-restriction-fields');
    for (const role of model.roles()) if (!except.has(role.index)) list.append(checkboxField(role.title, chosen.has(role.index), mark(chosen, role.index), 'role-check'));
    return list;
  }
  const rightBoxes = (keys, chosen, after) => keys.map(key => checkboxField(caption(key), chosen.has(key), mark(chosen, key, after), 'role-check'));
  // Объекты деревом, как у прототипа (сверено в конфигураторе 14.10.2026,
  // ACCESS-RIGHTS «Мастер, шаг 1»): флажок у корня конфигурации, у каждого
  // вида и у объекта, виды свёрнуты; и кнопка «По подсистемам». Флажок над
  // объектами показывает, отмечены все, часть или ни один: щелчок по снятому
  // отмечает все, по отмеченному целиком или частично — снимает все. Одно
  // дерево у мастера и у окна отбора; offered — какие объекты предлагаются.
  // holder.subsystems — отбор по подсистемам; объект, скрытый им, из выбора
  // уходит: в работу идёт то, что видно в списке. refiltered — после смены
  // отбора, до перерисовки; after — после каждой перемены выбора. Ответ —
  // список, кнопка, fill, рисующий список заново, и mark(enabled), отмечающий
  // или снимающий все видимые.
  function objectChoice(chosenObjects, holder, {after = () => {}, offered = () => model.restrictable(), refiltered = () => {}} = {}) {
    const objectList = node('div', undefined, 'role-restriction-fields role-fields-tree role-wizard-objects');
    let boxes = [], groups = [], shown = [];
    // Отметки ставятся на месте: список, нарисованный заново, забрал бы
    // фокус у флажка, по которому щёлкнули.
    function sync() {
      for (const [input, id] of boxes) input.checked = chosenObjects.has(id);
      for (const [input, items] of groups) {
        const marked = items.filter(item => chosenObjects.has(item.id)).length;
        input.checked = marked > 0 && marked === items.length; input.indeterminate = marked > 0 && marked < items.length;
      }
    }
    function markAll(items, enabled) { for (const item of items) if (enabled) chosenObjects.add(item.id); else chosenObjects.delete(item.id); }
    function groupBox(caption, items) {
      const box = checkboxField(caption, false, () => { markAll(items, !items.some(item => chosenObjects.has(item.id))); sync(); after(); }, 'role-check');
      groups.push([box.querySelector('input'), items]);
      return box;
    }
    function line(depth, toggle, box) {
      const row = restrictionTreeRow(depth, toggle, box);
      objectList.append(row);
      return row;
    }
    function fillObjects() {
      const passes = subsystemFilterPasses(source.schema, holder.subsystems), kinds = new Map();
      shown = [];
      for (const item of offered()) {
        if (!passes(item.id)) { chosenObjects.delete(item.id); continue; }
        if (!kinds.has(item.kind)) kinds.set(item.kind, []);
        kinds.get(item.kind).push(item); shown.push(item);
      }
      objectList.replaceChildren(); boxes = []; groups = [];
      if (!kinds.size) { objectList.append(node('p', 'Ни один объект не входит в отмеченные подсистемы.', 'muted')); return; }
      const root = source.schema.objects?.find(item => item.root);
      line(0, null, groupBox(root ? title(root) || root.name : 'Конфигурация', shown));
      for (const [kind, items] of kinds) {
        const rows = [], caption = source.kindTitles?.[kind] || kind, folding = restrictionFold(caption, rows);
        line(1, folding.toggle, groupBox(caption, items));
        for (const item of items) {
          const box = checkboxField(item.title, false, mark(chosenObjects, item.id, () => { sync(); after(); }), 'role-check');
          boxes.push([box.querySelector('input'), item.id]);
          rows.push(line(2, null, box));
        }
        folding.fold(false);
      }
      sync();
    }
    // Отбор выбирается в своём окне, уже после щелчка, который окно с
    // деревом слышало: перемену ему сообщает событие change списка.
    const button = subsystemFilterButton({schema: () => source.schema, root: () => configurationName(source.schema), holder, applied: () => {
      refiltered(); fillObjects(); after(); objectList.dispatchEvent(new Event('change', {bubbles: true}));
    }});
    fillObjects();
    return {list: objectList, button, fill: fillObjects, mark(enabled) { markAll(shown, enabled); sync(); after(); }};
  }
  // Копирование выбранного в отмеченные роли (справка,
  // form_CopyRowLevelInstrDlg; сверено в конфигураторе 18.10.2026): у
  // ограничений и у шаблонов одно окно — «Из ролей:» (роли выбранных строк)
  // и «В роли:» флажками без них, «ОК» недоступна, пока роли не отмечены.
  // apply(roles) записывает копию и отвечает false, если окно надо открыть
  // снова с теми же отметками.
  async function copyToRoles(rows, title, apply) {
    const from = new Set(rows.map(row => row.roleIndex)), roles = new Set();
    for (;;) {
      const sources = node('div', undefined, 'role-restriction-fields role-copy-sources'), targets = node('div', undefined, 'role-restriction-form');
      for (const role of model.roles()) if (from.has(role.index)) sources.append(node('div', `✓ ${role.title}`, 'role-check'));
      const list = roleChoice(roles, from);
      targets.append(markedListCaption('В роли:', enabled => {
        for (const role of model.roles()) if (!from.has(role.index)) { if (enabled) roles.add(role.index); else roles.delete(role.index); }
        for (const input of list.querySelectorAll('input')) input.checked = enabled;
      }), list);
      const content = node('div', undefined, 'role-copy-grid'), left = node('div', undefined, 'role-restriction-form');
      left.append(node('div', 'Из ролей:', 'role-restriction-caption'), sources);
      content.append(left, targets);
      const answer = await showDialog({title, content, className: 'role-restriction-dialog', help: 'role-copy',
        answers: [['ok', 'ОК', {enabled: () => roles.size > 0}], [null, 'Отмена']]});
      if (answer !== 'ok' || await apply([...roles])) return;
    }
  }
  const copyRestrictions = rows => copyToRoles(rows, 'Копировать ограничения', async roles => {
    await applyReplacing(trial => model.copyRestrictions(rows, roles, trial), restrictionLine);
    return true;
  });
  // Шаблон того же наименования в роли: «Да» заменяет его и копирует
  // остальные, «Нет» не копирует ничего, «Отмена» возвращает к окну.
  const copyTemplates = rows => copyToRoles(rows, 'Копировать шаблоны ограничений', async roles => {
    if (model.copyTemplates(rows, roles, true).length) {
      const answer = await askChoice('Шаблоны с таким наименованием уже существуют в выбранных ролях. Заменять существующие шаблоны?', '',
        [['yes', 'Да'], ['no', 'Нет'], [null, 'Отмена']]);
      if (answer === null) return false;
      if (answer !== 'yes') return true;
    }
    model.copyTemplates(rows, roles, false);
    listChanged();
    return true;
  });
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
    // Смена отбора по подсистемам — «Установить» или «Отключить» — снимает
    // отметки всех объектов, и видимых тоже; роли остаются (сверено в
    // конфигураторе 14.10.2026).
    const objectsOffered = objectChoice(chosenObjects, wizardFilter, {refiltered: () => chosenObjects.clear()}), roleList = roleChoice(roles);
    first.append(markedListCaption('Объекты:', enabled => objectsOffered.mark(enabled), [objectsOffered.button]), objectsOffered.list,
      markedListCaption('Роли:', enabled => {
        for (const role of model.roles()) if (enabled) roles.add(role.index); else roles.delete(role.index);
        for (const input of roleList.querySelectorAll('input')) input.checked = enabled;
      }), roleList);
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
    // прототипа; на первом шаге «ОК» недоступна, «Далее >» — пока не отмечен
    // ни один объект (сверено 14.10.2026; прототип после сброса отметок
    // сменой отбора оставляет её доступной и ведёт на пустой второй шаг —
    // его ошибка, ACCESS-RIGHTS).
    const answer = await showDialog({title: 'Новое ограничение доступа', content, className: 'role-restriction-dialog', help: 'role-restriction-wizard', answers: [
      ['back', '< Назад', {act: () => { step = 1; show(); }, enabled: () => step === 2}],
      ['next', 'Далее >', {act: () => { step = 2; fillSecond(); show(); }, enabled: () => step === 1 && chosenObjects.size > 0, primary: () => step === 1}],
      ['ok', 'ОК', {enabled: () => step === 2 && rights.size > 0 && (!fieldsChosen() || fields.filled()), primary: () => step === 2}],
      [null, 'Отмена']],
    check: key => key === 'next' && !roles.size ? 'Отметьте роли.' : ''});
    if (answer !== 'ok') return;
    // Только отмеченные поля, если отмечено право, ограничиваемое по полям:
    // без него поля ничего не значат.
    const chosenPaths = fieldsChosen() ? paths : new Set();
    await applyReplacing(trial => model.addRestrictions({objectIDs: [...chosenObjects], roles: [...roles], rights: [...rights], paths: chosenPaths, condition}, trial), restrictionLine);
  }
  // Список окна отбора: подпись с «Отметить все» и «Снять все» над
  // флажками. keys — что предлагается, по порядку; chosen — отмеченное;
  // label(key) — подпись флажка.
  function filterList(caption, keys, chosen, label) {
    const list = node('div', undefined, 'role-restriction-fields'), block = node('div', undefined, 'role-filter-list');
    const fill = () => list.replaceChildren(...keys.map(key => checkboxField(label(key), chosen.has(key), mark(chosen, key), 'role-check')));
    block.append(markedListCaption(caption, enabled => { for (const key of keys) if (enabled) chosen.add(key); else chosen.delete(key); fill(); }), list);
    fill();
    return block;
  }
  // Отмеченное при открытии окна: по действующему отбору, а без условия в
  // списке — всё (сверено в конфигураторе 14.10.2026).
  const opened = (current, keys) => new Set(current?.size ? keys.filter(key => current.has(key)) : keys);
  // Условие списка: отмечено всё или не отмечено ничего — условия нет.
  function narrowed(chosen, keys) {
    const kept = keys.filter(key => chosen.has(key));
    return new Set(kept.length < keys.length ? kept : []);
  }
  // Окно «Отбор» списка key: «ОК» ставит отбор value(), «Отмена» ничего не
  // меняет; без единого условия отбора нет.
  async function filterWindow(key, content, help, value, className = '') {
    const answer = await showDialog({title: 'Отбор', content, className: `role-restriction-dialog ${className}`, help, answers: [['ok', 'ОК'], [null, 'Отмена']]});
    if (answer === 'ok') setListFilter(key, model.activeFilter(value()));
  }
  // Окно отбора ограничений (справка, form_RolesGroupEditFilterDlg; сверено
  // в конфигураторе 14.10.2026): объекты (с «По подсистемам»), поля, роли,
  // права — только встречающиеся в ограничениях, — и текст ограничения.
  async function filterRestrictions() {
    const current = listFilter.restrictions, offered = model.restrictionFilterChoices();
    const keys = {objects: offered.objects.map(item => item.id), roles: offered.roles.map(role => role.path), rights: offered.rights, paths: offered.fields.map(choice => choice.path)};
    const draft = {objects: opened(current?.objects, keys.objects), roles: opened(current?.roles, keys.roles), rights: opened(current?.rights, keys.rights),
      paths: opened(current?.paths, keys.paths), condition: current?.condition || '', subsystems: current?.subsystems ?? null};
    const objects = objectChoice(draft.objects, draft, {offered: () => offered.objects}), objectBlock = node('div', undefined, 'role-filter-list');
    objectBlock.append(markedListCaption('Объект:', enabled => objects.mark(enabled)), objects.list);
    const roleTitles = new Map(offered.roles.map(role => [role.path, role.title])), fieldCaptions = new Map(offered.fields.map(choice => [choice.path, choice.caption]));
    const text = textInput(draft.condition, value => { draft.condition = value; }, {kind: 'area', rows: 4});
    text.setAttribute('aria-label', 'Ограничение доступа');
    const textBlock = node('div', undefined, 'role-filter-list');
    textBlock.append(node('div', 'Ограничение доступа:', 'role-restriction-caption'), text);
    const grid = node('div', undefined, 'role-filter-grid'), right = node('div', undefined, 'role-filter-column');
    right.append(filterList('Права:', keys.rights, draft.rights, caption), textBlock);
    grid.append(objectBlock, filterList('Поля:', keys.paths, draft.paths, path => fieldCaptions.get(path)), filterList('Роли:', keys.roles, draft.roles, path => roleTitles.get(path)), right);
    const content = node('div', undefined, 'role-restriction-form');
    content.append(grid, objects.button);
    await filterWindow('restrictions', content, 'role-restriction-filter', () => ({objects: narrowed(draft.objects, keys.objects), roles: narrowed(draft.roles, keys.roles),
      rights: narrowed(draft.rights, keys.rights), paths: narrowed(draft.paths, keys.paths), condition: draft.condition, subsystems: draft.subsystems}), 'role-filter-dialog');
  }
  // Окно отбора шаблонов (справка, form_TemplateEditFilterDlg; сверено в
  // конфигураторе 14.10.2026): роли — все роли конфигурации, наименование —
  // выпадающий список всех наименований с очисткой, на равенство, фрагмент
  // текста шаблона.
  async function filterTemplates() {
    const current = listFilter.templates, names = model.templateNames(), roles = model.roles(), keys = roles.map(role => role.path);
    const draft = {roles: opened(current?.roles, keys), name: names.includes(current?.name) ? current.name : '', condition: current?.condition || ''};
    const roleTitles = new Map(roles.map(role => [role.path, role.title]));
    const name = clearableSelect(names.map(item => [item, item]), draft.name, value => { draft.name = value; }, 'Наименование');
    const text = textInput(draft.condition, value => { draft.condition = value; });
    text.setAttribute('aria-label', 'Текст шаблона');
    const content = node('div', undefined, 'role-restriction-form');
    content.append(filterList('Роли:', keys, draft.roles, path => roleTitles.get(path)), node('div', 'Наименование:', 'role-restriction-caption'), name,
      node('div', 'Текст шаблона:', 'role-restriction-caption'), text);
    await filterWindow('templates', content, 'role-template-filter', () => ({roles: narrowed(draft.roles, keys), name: draft.name, condition: draft.condition}));
  }
  const templateLine = item => `${item.role} · ${item.template}`;
  // Окно шаблонов (справка, form_EditRowTemplatesDlg): у одинаковых во
  // всех выбранных шаблонах наименования и текста флажок изменения стоит,
  // у разных — снят, поле пусто; правка поля ставит флажок. При изменении
  // (editing) флажки стоят и у одного шаблона, каждый над своим полем, а
  // подписи — с двоеточием, как у прототипа (снимок shabl-16); окну
  // добавления шаблона флажки не нужны.
  function templateForm(rows, editing = false) {
    const content = node('div', undefined, 'role-restriction-form'), state = {};
    const name = common(rows.map(row => row.name)), text = common(rows.map(row => row.condition));
    state.name = name ?? ''; state.condition = text ?? '';
    state.changeName = name !== null; state.changeCondition = text !== null;
    const nameFlag = checkboxField('Изменять наименование', state.changeName, enabled => { state.changeName = enabled; }, 'role-check');
    const textFlag = checkboxField('Изменять текст', state.changeCondition, enabled => { state.changeCondition = enabled; }, 'role-check');
    const nameInput = textInput(state.name, value => { state.name = value; state.changeName = true; nameFlag.querySelector('input').checked = true; });
    nameInput.setAttribute('aria-label', 'Наименование');
    const editor = queryTextField(state.condition, value => { state.condition = value; state.changeCondition = true; textFlag.querySelector('input').checked = true; }, 'Текст шаблона');
    if (editing) content.append(nameFlag, node('div', 'Наименование:', 'role-restriction-caption'), nameInput,
      textFlag, node('div', 'Текст шаблона:', 'role-restriction-caption'), editor);
    else content.append(node('div', 'Наименование', 'role-restriction-caption'), nameInput, node('div', 'Текст шаблона', 'role-restriction-caption'), editor);
    return {content, state};
  }
  const templateNameCheck = state => () => state.changeName && !state.name.trim() ? 'Укажите наименование шаблона.' : '';
  async function editTemplates(rows) {
    const {content, state} = templateForm(rows, true);
    const answer = await showDialog({title: rows.length > 1 ? 'Изменение шаблонов ограничений' : 'Изменение шаблона ограничений',
      explanation: rows.length > 1 ? `Выбрано шаблонов: ${rows.length}.` : '', content, className: 'role-restriction-dialog', side: true, help: 'role-template-edit',
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
    listChosen = new Set(); opening = true;
    fill();
    body.append(list);
    if (revealing) { revealing = false; list.querySelector('tbody tr.selected')?.scrollIntoView({block: 'nearest'}); }
  }
  function renderPanel() {
    panel.replaceChildren();
    if (!selected) { panel.append(node('p', 'Выберите объект слева: его права во всех ролях можно отметить здесь сразу.', 'muted')); return; }
    const item = selected.object;
    panel.append(node('h3', selected.part ? `${model.title(item) || item.name} · ${model.title(selected.part) || selected.part.name}` : model.title(item) || item.name));
    panel.append(renderGrid());
    if (selected.object && !selected.part && selected.object.restrictedRights?.length) panel.append(renderRestrictions(selected.object));
  }
  // Выбранное в дереве и выбранная роль переживают сохранение: окно
  // открывается заново с тем, что записано, на том же месте.
  function reselect() {
    if (!selected) return null;
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
  return hostedEditor(host, () => { source = null; model = null; }, {
    open,
    show,
    title() { return overviewWindows[view]; },
    // Роли, изменённые в окне, — для сохранения одним запросом.
    value() { return model?.changes() || []; },
  });
}
