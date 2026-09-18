/* «Все роли»: один объект — сразу все роли. Только просмотр; правка остаётся
   в редакторе конкретной роли, куда отсюда можно перейти по имени роли. */

/* Модель отделена от разметки: она отвечает на два вопроса окна — кто и что
   может делать с выбранным объектом и каким условием это сужено — и ничего
   не знает про DOM, поэтому проверяется тестами напрямую. */
function createRoleOverviewModel(source) {
  const entries = source.roles || [];
  const objects = new Map((source.schema.objects || []).map(item => [item.id, item]));
  const standard = {ref:'Ссылка',code:'Код',description:'Наименование',deletionmark:'Пометка удаления',version:'Версия',predefined:'Предопределённый',predefineddataname:'Имя предопределённых данных',number:'Номер',date:'Дата',posted:'Проведён',recordid:'Идентификатор записи',period:'Период',recorder:'Регистратор',linenumber:'Номер строки',active:'Активность',movementkind:'Вид движения',value:'Значение',order:'Порядок'};
  const operators = {eq:'Равно',ne:'Не равно',in:'В списке','not-in':'Не в списке'};
  function localized(item) {
    return item.title?.[source.defaultLanguage] || Object.values(item.title || {}).find(Boolean) || standard[item.key] || item.name;
  }
  function objectTitle(id) {
    const object = objects.get(id);
    return object ? localized(object) : 'удалённый объект';
  }
  // Поле шаблона не привязано к объекту: шаблон живёт в роли и применяется к
  // разным объектам, поэтому его имя ищется по всей конфигурации.
  let shared = null;
  function anyFieldTitle(key) {
    if (!shared) {
      shared = new Map();
      for (const object of objects.values()) for (const field of object.fields) if (!shared.has(field.key)) shared.set(field.key, localized(field));
    }
    return shared.get(key) || standard[key] || key;
  }
  function fieldTitle(objectID, key) {
    if (!objectID) return anyFieldTitle(key);
    const field = objects.get(objectID)?.fields.find(item => item.key === key);
    return field ? localized(field) : (standard[key] || key);
  }
  // Подстановка параметров шаблона повторяет серверную: поле правила и поля
  // условий подзапроса, записанные как «$Имя», берутся из аргументов
  // ограничения по позиции параметра.
  function substitute(rule, bound) {
    const resolve = field => {
      if (!field.startsWith('$')) return field;
      const argument = bound.get(field.slice(1));
      return argument || null;
    };
    const field = resolve(rule.field);
    if (!field) return null;
    const result = {...rule, field};
    if (rule.subquery) {
      const where = [];
      for (const condition of rule.subquery.where || []) {
        const inner = resolve(condition.field);
        if (!inner) return null;
        where.push({...condition, field: inner});
      }
      result.subquery = {...rule.subquery, where};
    }
    return result;
  }
  function operand(rule, objectID) {
    if (rule.parameter) return `параметр сеанса ${rule.parameter}`;
    if (rule.subquery) {
      const inner = rule.subquery.object;
      const where = (rule.subquery.where || []).map(condition =>
        `${fieldTitle(inner, condition.field)} ${operators[condition.operator] || condition.operator} ${operand(condition, inner)}`);
      const head = `«${fieldTitle(inner, rule.subquery.field)}» из «${objectTitle(inner)}»`;
      return where.length ? `${head}, где ${where.join(' и ')}` : head;
    }
    const values = (rule.values || []).map(item => `«${item.data}»`);
    return values.length ? values.join(', ') : 'значение не задано';
  }
  // Ограничение показывается только если его правило действительно
  // разрешается: нераскрытый шаблон — это не «нет ограничения», а
  // неработающая роль, и окно должно назвать это прямо.
  function describe(role, objectID, policy) {
    let rule = policy.rule;
    if (policy.template) {
      const template = (role.policyTemplates || []).find(item => item.name === policy.template);
      if (!template) return {error: `шаблон «${policy.template}» не объявлен ролью`};
      const parameters = template.parameters || [], supplied = policy.arguments || [];
      if (parameters.length !== supplied.length) return {error: `шаблон «${policy.template}» ожидает полей: ${parameters.length}`};
      rule = substitute(template.rule, new Map(parameters.map((name, index) => [name, supplied[index]])));
      if (!rule) return {error: `шаблон «${policy.template}» ссылается на неизвестный параметр`};
    }
    if (!rule) return {error: 'правило не задано'};
    return {field: fieldTitle(objectID, rule.field), condition: `${operators[rule.operator] || rule.operator} ${operand(rule, objectID)}`};
  }
  function grantOf(entry, objectID) { return (entry.role.objects || []).find(item => item.object === objectID); }
  function readableFields(objectID) { return (objects.get(objectID)?.fields || []).filter(field => field.operations.includes('read')).length; }
  function fieldSummary(grant, objectID) {
    const total = readableFields(objectID);
    if (!grant?.operations?.includes('read') || !total) return '—';
    const granted = Math.min((grant.fields || []).filter(field => (field.operations || []).includes('read')).length, total);
    return granted >= total ? 'все' : `${granted} из ${total}`;
  }
  return {
    objects() { return source.schema.objects || []; },
    title(item) { return localized(item); },
    roleCount() { return entries.length; },
    // Строка на каждую роль, включая роли без единого права: вопрос окна —
    // «кто дотянется до объекта», и пустая строка отвечает на него так же
    // содержательно, как отмеченная.
    grants(objectID) {
      const object = objects.get(objectID);
      if (!object) return [];
      const readable = object.fields.filter(field => field.operations.includes('read')).length;
      return entries.map(entry => {
        const grant = grantOf(entry, objectID), operations = {};
        for (const operation of object.operations) operations[operation] = !!grant?.operations?.includes(operation);
        const granted = (grant?.fields || []).filter(field => (field.operations || []).includes('read')).length;
        return {
          path: entry.path, name: entry.role.name, title: localized(entry.role),
          operations, granted: Object.values(operations).some(Boolean),
          fields: {granted: Math.min(granted, readable), total: readable},
        };
      });
    },
    // Один список по всему проекту: здесь вопрос не «что с этим объектом»,
    // а «где вообще в конфигурации стоят ограничения» — перед выпуском это
    // единственный способ увидеть их все, не открывая роли по одной.
    allRestrictions() {
      const rows = [];
      for (const entry of entries) {
        for (const grant of entry.role.objects || []) {
          if (!objects.has(grant.object)) continue;
          for (const policy of grant.policies || []) {
            const described = describe(entry.role, grant.object, policy);
            for (const operation of policy.operations || []) {
              rows.push({path: entry.path, objectID: grant.object, object: objectTitle(grant.object),
                role: localized(entry.role), operation, fields: fieldSummary(grant, grant.object),
                origin: policy.template ? `шаблон ${policy.template}` : 'своё правило', ...described});
            }
          }
        }
      }
      rows.sort((left, right) => left.object.localeCompare(right.object) || left.role.localeCompare(right.role));
      return rows;
    },
    // Шаблон показывается как он написан — с «$Имя» на месте параметра:
    // именно незаполненность и делает его переиспользуемым, а конкретное
    // поле подставляет уже ограничение, и оно видно в таблице выше.
    allTemplates() {
      const rows = [];
      for (const entry of entries) {
        for (const template of entry.role.policyTemplates || []) {
          const rule = template.rule;
          rows.push({path: entry.path, role: localized(entry.role), name: template.name,
            parameters: (template.parameters || []).map(name => '$' + name).join(', '),
            field: fieldTitle(null, rule.field),
            condition: `${operators[rule.operator] || rule.operator} ${operand(rule, null)}`});
        }
      }
      rows.sort((left, right) => left.role.localeCompare(right.role) || left.name.localeCompare(right.name));
      return rows;
    },
    restrictions(objectID) {
      const rows = [];
      for (const entry of entries) {
        for (const policy of grantOf(entry, objectID)?.policies || []) {
          const described = describe(entry.role, objectID, policy);
          for (const operation of policy.operations || []) {
            rows.push({path: entry.path, role: localized(entry.role), operation,
              origin: policy.template ? `шаблон ${policy.template}` : 'своё правило', ...described});
          }
        }
      }
      return rows;
    },
  };
}

function createRoleOverview(host, openRole) {
  let source, model, selected = null, tree, panel, tab = 'roles', body, filter = '';
  const operations = {read:'Чтение',view:'Просмотр',create:'Добавление',update:'Изменение',delete:'Удаление',post:'Проведение','undo-posting':'Отмена проведения','totals-control':'Управление итогами'};
  const kinds = {constants:'Константы',enumerations:'Перечисления',catalogs:'Справочники',documents:'Документы','information-registers':'Регистры сведений','accumulation-registers':'Регистры накопления'};
  function node(tag, text, className) {
    const element = document.createElement(tag);
    if (text !== undefined) element.textContent = text;
    if (className) element.className = className;
    return element;
  }
  function renderTree(query = '') {
    tree.replaceChildren();
    const groups = new Map(), needle = query.toLocaleLowerCase().trim();
    for (const item of model.objects()) {
      if (!`${item.name} ${model.title(item)}`.toLocaleLowerCase().includes(needle)) continue;
      let group = groups.get(item.kind);
      if (!group) { group = node('details'); group.open = true; group.append(node('summary', kinds[item.kind] || item.kind)); groups.set(item.kind, group); tree.append(group); }
      const button = node('button', model.title(item), 'role-tree-item');
      button.type = 'button'; button.title = item.name; button.dataset.object = item.id;
      if (selected?.id === item.id) button.classList.add('selected');
      button.addEventListener('click', () => { selected = item; renderTree(query); renderPanel(); });
      group.append(button);
    }
    if (!tree.childElementCount) tree.append(node('p', 'Нет подходящих объектов', 'muted'));
  }
  function renderGrants(object) {
    const block = node('section', undefined, 'overview-block');
    block.append(node('h4', 'Все права'));
    const rows = model.grants(object.id);
    if (!rows.length) { block.append(node('p', 'В проекте нет ролей.', 'muted')); return block; }
    const table = node('table'), head = node('tr');
    head.append(node('th', 'Роль'));
    for (const operation of object.operations) head.append(node('th', operations[operation] || operation));
    head.append(node('th', 'Реквизиты'));
    table.append(head);
    for (const row of rows) {
      const line = node('tr');
      if (!row.granted) line.className = 'overview-empty';
      const name = node('td'), link = node('button', row.title, 'overview-role');
      link.type = 'button'; link.title = row.name;
      link.addEventListener('click', () => openRole(row.path, object.id));
      name.append(link); line.append(name);
      for (const operation of object.operations) {
        const cell = node('td', row.operations[operation] ? '✓' : '·', row.operations[operation] ? 'overview-yes' : 'overview-no');
        cell.setAttribute('aria-label', `${row.title} — ${operations[operation] || operation} — ${row.operations[operation] ? 'разрешено' : 'нет'}`);
        line.append(cell);
      }
      // Реквизиты показываются числом, а не галкой: право на объект без части
      // его реквизитов — это другое право, и по галке это не видно.
      const fields = !row.operations.read || !row.fields.total ? '—'
        : row.fields.granted >= row.fields.total ? 'все' : `${row.fields.granted} из ${row.fields.total}`;
      line.append(node('td', fields));
      table.append(line);
    }
    block.append(table);
    return block;
  }
  function renderRestrictions(object) {
    const block = node('section', undefined, 'overview-block');
    block.append(node('h4', 'РЛС'));
    const rows = model.restrictions(object.id);
    if (!rows.length) { block.append(node('p', 'Ограничений на строки этого объекта нет ни в одной роли.', 'muted')); return block; }
    block.append(node('p', 'Строки, не прошедшие правило, не видны и не изменяются. Ограничения разных ролей объединяются по ИЛИ: роль без ограничения открывает объект целиком.', 'muted'));
    const table = node('table'), head = node('tr');
    head.append(node('th', 'Роль'), node('th', 'Право'), node('th', 'Поле'), node('th', 'Условие'), node('th', 'Источник'));
    table.append(head);
    for (const row of rows) {
      const line = node('tr');
      const name = node('td'), link = node('button', row.role, 'overview-role');
      link.type = 'button';
      link.addEventListener('click', () => openRole(row.path, object.id));
      name.append(link); line.append(name);
      line.append(node('td', operations[row.operation] || row.operation));
      if (row.error) {
        const cell = node('td', row.error, 'overview-broken');
        cell.colSpan = 2; line.append(cell);
      } else line.append(node('td', row.field), node('td', row.condition, 'overview-condition'));
      line.append(node('td', row.origin));
      table.append(line);
    }
    block.append(table);
    return block;
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
    const rows = model.allRestrictions().filter(row => matches(row, [row.object, row.role, row.field, row.condition, row.origin]));
    if (!rows.length) { block.append(node('p', model.allRestrictions().length ? 'Ничего не найдено.' : 'В проекте нет ни одного ограничения доступа.', 'muted')); return block; }
    block.append(node('p', 'Каждая строка — одно ограничение одной роли на одно право. Поля показывают, сколько реквизитов объекта эта роль вообще читает: ограничение сужает строки, права на реквизиты — столбцы.', 'muted'));
    const table = node('table'), head = node('tr');
    head.append(node('th', 'Объект'), node('th', 'Роль'), node('th', 'Право'), node('th', 'Поля'), node('th', 'Правило доступа'), node('th', 'Источник'));
    table.append(head);
    for (const row of rows) {
      const line = node('tr');
      line.append(node('td', row.object));
      line.append(roleLink(row.role, row.path, row.objectID));
      line.append(node('td', operations[row.operation] || row.operation), node('td', row.fields));
      if (row.error) line.append(node('td', row.error, 'overview-broken'));
      else line.append(node('td', `${row.field} ${row.condition}`, 'overview-condition'));
      line.append(node('td', row.origin));
      table.append(line);
    }
    block.append(table);
    return block;
  }
  function renderAllTemplates() {
    const block = node('section', undefined, 'overview-block');
    const all = model.allTemplates();
    const rows = all.filter(row => matches(row, [row.role, row.name, row.parameters, row.field, row.condition]));
    if (!rows.length) { block.append(node('p', all.length ? 'Ничего не найдено.' : 'Ни одна роль не объявила шаблонов политик.', 'muted')); return block; }
    block.append(node('p', 'Шаблон принадлежит роли и переиспользуется её ограничениями. Параметр «$Имя» стоит там, где ограничение подставит поле своего объекта.', 'muted'));
    const table = node('table'), head = node('tr');
    head.append(node('th', 'Роль'), node('th', 'Наименование'), node('th', 'Параметры'), node('th', 'Правило'));
    table.append(head);
    for (const row of rows) {
      const line = node('tr');
      line.append(roleLink(row.role, row.path));
      line.append(node('td', row.name), node('td', row.parameters || '—'));
      line.append(node('td', `${row.field} ${row.condition}`, 'overview-condition'));
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
      search.addEventListener('input', () => { filter = search.value; renderTree(filter); });
      tree = node('div', undefined, 'overview-tree'); sidebar.append(search, tree);
      panel = node('section', undefined, 'overview-panel');
      body.append(sidebar, panel);
      body.className = 'overview-body';
      renderTree(filter); renderPanel();
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
    if (!selected) { panel.append(node('p', 'Выберите объект слева, чтобы увидеть его права и ограничения по всем ролям сразу.', 'muted')); return; }
    panel.append(node('h3', model.title(selected)));
    panel.append(renderGrants(selected), renderRestrictions(selected));
  }
  function open(value) {
      source = structuredClone(value); model = createRoleOverviewModel(source);
      const previous = selected?.id;
      selected = model.objects().find(item => item.id === previous) || null;
      host.hidden = false; host.replaceChildren();
      const header = node('div', undefined, 'overview-header');
      const strip = node('div', undefined, 'overview-tabs');
      for (const [key, text] of [['roles', 'Все роли'], ['restrictions', 'Ограничения доступа'], ['templates', 'Шаблоны политик']]) {
        const button = node('button', text, 'overview-tab');
        button.type = 'button';
        if (tab === key) button.classList.add('selected');
        button.setAttribute('aria-pressed', String(tab === key));
        button.addEventListener('click', () => { if (tab === key) return; tab = key; filter = ''; open(source); });
        strip.append(button);
      }
      header.append(strip, node('span', `Ролей в проекте: ${model.roleCount()}`, 'muted'));
      host.append(header);
      body = node('div'); host.append(body);
      renderBody();
  }
  return {
    open,
    close() { source = null; model = null; host.hidden = true; host.replaceChildren(); },
    setDisabled(disabled) { host.inert = disabled; },
  };
}
