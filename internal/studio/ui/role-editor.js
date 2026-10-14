/* Role editor: explicit grants underneath, ordinary tree/checkboxes in Studio. */
/* Whether a restriction of the right names the fields it checks - one per
   set of fields, a field in one at most, and one more on «Прочие поля» - or
   the right takes one restriction, on every field. The rules are the
   server's (RestrictionRules), sent with the schema. */
function restrictsByFields(schema, right) {
  return !!(schema.restrictionRules || []).find(rule => rule.right === right)?.byFields;
}
function createRoleModel(source) {
  // Only the role is copied: the schema is read and never changed, and the
  // window «Все роли» builds a model for every role over one schema.
  const value = {...source, role: structuredClone(source.role)};
  value.role.objects ||= [];
  value.role.commands ||= [];
  const definitions = new Map(value.schema.objects.map(item => [item.id, item]));
  const fields = new Map(value.schema.objects.map(item => [item.id, new Map(item.fields.map(field => [field.key, field]))]));
  // The fields a restriction of reading may name, by object - see
  // RestrictionFields on the server.
  const restrictionFields = new Map(value.schema.objects.map(item => [item.id, new Set((item.restrictionFields||[]).map(field => field.key))]));
  let objects, fieldGrants;
  function reindex(){objects=new Map(value.role.objects.map(item=>[item.object,item]));fieldGrants=new Map(value.role.objects.map(item=>[item.object,new Map((item.fields||[]).map(field=>[field.field,field]))]));}
  reindex();
  function object(id) {
    let item = objects.get(id);
    if (!item) { item = {object:id, operations:[], fields:[]}; value.role.objects.push(item);objects.set(id,item);fieldGrants.set(id,new Map()); }
    item.operations ||= []; item.fields ||= [];
    return item;
  }
  function toggle(items, operation, enabled) {
    return enabled ? [...new Set([...items, operation])] : items.filter(item => item !== operation);
  }
  // A right has the prototype's two states: the box set grants it and the
  // box cleared denies it, both written - what is not written stands for the
  // role's flags (roleObjectRight, rolePartRight).
  function writeRight(holder, operation, enabled) {
    holder.operations = toggle(holder.operations || [], operation, enabled);
    const denied = toggle(holder.denied || [], operation, !enabled);
    if (denied.length) holder.denied = denied; else delete holder.denied;
  }
  const objectRight = (id, operation) => roleObjectRight(value.role, objects.get(id), operation);
  const partRight = (id, key, operation) => {
    const part = fields.get(id)?.get(key);
    return part ? rolePartRight(value.role, objects.get(id), fieldGrants.get(id)?.get(key), part, operation) : false;
  };
  // A right of a part is the part's alone: setting it touches neither the
  // object nor the other parts, as the prototype's editor does - whether a
  // part's right holds without the owner's is the role's flag of independent
  // rights, read when the rights act.
  function setField(id, key, operation, enabled) {
    if (!fields.get(id)?.get(key)?.operations.includes(operation)) return;
    const grant = object(id), index = fieldGrants.get(id);
    let target = index.get(key);
    if(!target){target={field:key,operations:[]};grant.fields.push(target);index.set(key,target);}
    writeRight(target,operation,enabled);
  }
  // A right of an object is the object's alone too: marking it gives none
  // to its attributes (owner's decision of 12.10.2026, by the help - the
  // attributes are set together only on the question the flag asks).
  function setObject(id, operation, enabled) {
    if (!definitions.get(id)?.rights.includes(operation)) return;
    writeRight(object(id), operation, enabled);
  }
  // Every right in force of every object and every part, by where it is.
  function rightsInForce() {
    const result = new Map();
    for (const definition of value.schema.objects) {
      for (const operation of definition.rights) result.set(`${definition.id} ${operation}`, objectRight(definition.id, operation));
      for (const part of definition.fields) for (const operation of part.operations)
        result.set(`${definition.id} ${operation} ${part.key}`, partRight(definition.id, part.key, operation));
    }
    return result;
  }
  // A flag changes what is not written. Whatever right it would turn over is
  // written first as it stood, so no right of an existing object or part
  // changes with the flag.
  function keepRightsAcross(change) {
    const before = rightsInForce();
    change();
    for (const [where, enabled] of rightsInForce()) {
      if (before.get(where) === enabled) continue;
      const [id, operation, key] = where.split(' ');
      if (key === undefined) writeRight(object(id), operation, before.get(where));
      else setField(id, key, operation, before.get(where));
    }
  }
  // The restrictions of access to data on one right of an object, in the
  // order the role writes them.
  const restrictionsOf = (id, right) => (objects.get(id)?.restrictions || []).filter(item => item.right === right);
  // Only a right restricted by fields takes them; a field the object does
  // not have is not taken.
  const byFields = right => restrictsByFields(value.schema, right);
  function restrictionEntry(id, right, restriction) {
    const entry = {right}, own = restrictionFields.get(id) || new Set();
    const chosen = byFields(right) ? (restriction.fields || []).filter(field => own.has(field)) : [];
    if (chosen.length) entry.fields = [...new Set(chosen)];
    if (restriction.condition) entry.condition = restriction.condition;
    return entry;
  }
  // A restriction is written inside the right it restricts, granted or
  // denied, as the prototype writes it: a right that stood for the role's
  // flags is written first as it stood, so adding a restriction turns no
  // right over.
  function putRestriction(id, right, index, restriction) {
    if (!definitions.get(id)?.restrictedRights?.includes(right)) return null;
    const grant = object(id);
    if (!grant.operations.includes(right) && !grant.denied?.includes(right)) writeRight(grant, right, objectRight(id, right));
    const entry = restrictionEntry(id, right, restriction);
    const list = grant.restrictions ||= [], same = restrictionsOf(id, right);
    if (index < same.length) list[list.indexOf(same[index])] = entry; else list.push(entry);
    return entry;
  }
  // Two restrictions of one right collide where the role cannot keep both
  // (validateRestrictions on the server): any two on a right not restricted
  // by fields, two on «Прочие поля», two naming one field. What the restriction would take from another: the fields they
  // share, or the other whole.
  function collision(right, entry, other) {
    if (!byFields(right)) return {whole: true, shared: []};
    const theirs = other.fields || [], ours = entry.fields || [];
    if (!ours.length) return theirs.length ? null : {whole: true, shared: []};
    const shared = theirs.filter(field => ours.includes(field));
    return !shared.length ? null : {whole: shared.length === theirs.length, shared};
  }
  // What a restriction put at index would collide with among the others of
  // the right.
  function restrictionClashes(id, right, index, restriction) {
    const entry = restrictionEntry(id, right, restriction);
    return restrictionsOf(id, right).flatMap((other, at) => {
      const clash = at === index ? null : collision(right, entry, other);
      return clash ? [{index: at, ...clash}] : [];
    });
  }
  // Writes restrictions on one right of an object at once, each at its
  // index or, past the end, as a new one, and each replaces what it collides
  // with: a field goes from the restriction that named it, a restriction
  // goes whole when nothing of it is left (an emptied one would stand for
  // «Прочие поля», which is not what was restricted). Changes are taken in
  // order, so of two that collide the later stays; one taken away whole
  // before its turn is not written, and a new one the right has already, as
  // it is, is not written twice. Answers what was taken from restrictions the
  // changes did not name, as they stood, for the question «Заменить?».
  function writeRestrictions(id, right, changes) {
    if (!definitions.get(id)?.restrictedRights?.includes(right)) return [];
    const before = restrictionsOf(id, right), named = new Set(changes.map(change => before[change.index]).filter(Boolean));
    const taken = [], same = (left, other) => left.condition === other.condition && JSON.stringify([...left.fields || []].sort()) === JSON.stringify([...other.fields || []].sort());
    for (const change of changes) {
      const target = before[change.index];
      const current = restrictionsOf(id, right);
      if (target && !current.includes(target)) continue;
      if (!target && current.some(other => same(other, restrictionEntry(id, right, change.restriction)))) continue;
      const entry = putRestriction(id, right, target ? current.indexOf(target) : current.length, change.restriction);
      named.add(entry);
      if (target) before[change.index] = entry;
      const grant = objects.get(id);
      for (const other of restrictionsOf(id, right)) {
        const clash = other === entry ? null : collision(right, entry, other);
        if (!clash) continue;
        if (!named.has(other)) taken.push({...structuredClone(other), ...clash});
        // A restriction loses fields in place, so it stays the one the
        // changes name.
        if (clash.whole) grant.restrictions = grant.restrictions.filter(item => item !== other);
        else other.fields = other.fields.filter(field => !clash.shared.includes(field));
      }
    }
    if (objects.get(id) && !objects.get(id).restrictions?.length) delete objects.get(id).restrictions;
    return taken;
  }
  function removeRestriction(id, right, index) {
    const grant = objects.get(id), target = restrictionsOf(id, right)[index];
    if (!target) return;
    grant.restrictions = grant.restrictions.filter(item => item !== target);
    if (!grant.restrictions.length) delete grant.restrictions;
  }
  // A template of the role's restrictions: its name, with the names of its
  // parameters in brackets where it takes any, and its text.
  function templateEntry(template) {
    const entry = {name: template.name.trim()};
    if (template.condition) entry.condition = template.condition;
    return entry;
  }
  function putTemplate(index, template) {
    const list = value.role.restrictionTemplates ||= [], entry = templateEntry(template);
    if (index < list.length) list[index] = entry; else list.push(entry);
  }
  // A template is known by its name proper, without regard to case and to
  // its parameters, as the server tells them apart
  // (validateRestrictionTemplates): a role keeps one of a name.
  const templateKey = name => name.split('(')[0].trim().toLocaleLowerCase();
  // Writes templates at once, as writeRestrictions writes restrictions: each
  // at its index or, past the end, as a new one, each replacing the others of
  // its name, the later of two staying; a new one the role has already, as
  // it is, is not written twice. Answers the templates taken that the
  // changes did not name.
  function writeTemplates(changes) {
    const before = [...value.role.restrictionTemplates || []], named = new Set(changes.map(change => before[change.index]).filter(Boolean));
    const taken = [];
    for (const change of changes) {
      const list = value.role.restrictionTemplates ||= [], target = before[change.index], entry = templateEntry(change.template);
      if (target && !list.includes(target)) continue;
      if (!target && list.some(other => other.name === entry.name && (other.condition || '') === (entry.condition || ''))) continue;
      if (target) { list[list.indexOf(target)] = entry; before[change.index] = entry; } else list.push(entry);
      named.add(entry);
      for (const other of [...list]) {
        if (other === entry || templateKey(other.name) !== templateKey(entry.name)) continue;
        if (!named.has(other)) taken.push(structuredClone(other));
        list.splice(list.indexOf(other), 1);
      }
    }
    if (!value.role.restrictionTemplates?.length) delete value.role.restrictionTemplates;
    return taken;
  }
  function removeTemplate(index) {
    const list = value.role.restrictionTemplates || [];
    list.splice(index, 1);
    if (!list.length) delete value.role.restrictionTemplates;
  }
  function clean() {
    const result = structuredClone(value.role);
    for (const item of result.objects) {
      item.operations ||= []; item.fields = (item.fields||[]).filter(field=>field.operations?.length || field.denied?.length);
    }
    result.objects = result.objects.filter(item => item.operations.length || item.denied?.length || item.fields.length);
    return result;
  }
  // Repair removes only what the configuration no longer has - an object, a
  // part of one, a restriction pointing at a missing field. A right the
  // object has no use for is carried, as the prototype carries it, and is
  // never repaired away.
  function repaired() {
    const result = clean();
    const parts = value.schema.parts || {};
    result.objects = result.objects.filter(item => parts[item.object]);
    for (const item of result.objects) {
      item.fields = item.fields.filter(field => parts[item.object].includes(field.field));
      // A restriction loses a field the object no longer has, and goes when
      // it had fields and none is left: emptied, it would stand for every
      // other field, which is not what was restricted.
      if (item.restrictions) {
        const own = restrictionFields.get(item.object) || new Set(), rights = definitions.get(item.object)?.rights || [];
        item.restrictions = item.restrictions.filter(restriction => rights.includes(restriction.right)).flatMap(restriction => {
          if (!restriction.fields?.length) return [restriction];
          const fields = restriction.fields.filter(field => own.has(field));
          return fields.length ? [{...restriction, fields}] : [];
        });
        if (!item.restrictions.length) delete item.restrictions;
      }
    }
    result.objects = result.objects.filter(item => item.operations.length || item.denied?.length || item.fields.length);
    result.commands = result.commands.filter(item => value.schema.forms.some(form => form.id === item.form && form.commands.some(command => command.id === item.command)));
    return result;
  }
  return {
    value: clean,
    setName(name) { value.role.name = name; },
    setTitle(language, title) { if(title.trim())value.role.title[language]=title;else delete value.role.title[language]; },
    setComment(comment) { value.role.comment = comment; },
    // The flag decides only what a new object is given, as in the prototype:
    // every right it stood for until now is written as it was before it
    // changes, so no right of an existing object turns over with it.
    setGrantNewObjectsByDefault(enabled) {
      if (!!value.role.grantNewObjectsByDefault === enabled) return;
      keepRightsAcross(() => { value.role.grantNewObjectsByDefault = enabled; });
    },
    // The prototype asks, when the flag for attributes changes, whether to
    // set or clear the rights on every attribute and table part: «Да» does,
    // «Нет» changes no right (help, «Редактирование роли»).
    setGrantNewFieldsByDefault(enabled, everyField = false) {
      if (!!value.role.grantNewFieldsByDefault === enabled) return;
      keepRightsAcross(() => { value.role.grantNewFieldsByDefault = enabled; });
      if (!everyField) return;
      for (const definition of value.schema.objects) for (const part of definition.fields)
        if (part.operations.includes('edit')) for (const operation of part.operations) setField(definition.id, part.key, operation, enabled);
    },
    setIndependentChildRights(enabled) { value.role.independentChildRights = enabled; },
    setObject, setField,
    hasObject: objectRight,
    hasField: partRight,
    setAll(id, enabled) { for(const operation of definitions.get(id)?.rights || [])setObject(id, operation, enabled); },
    setAllOfPart(id, key, enabled) { for(const operation of fields.get(id)?.get(key)?.operations || [])setField(id, key, operation, enabled); },
    // «Установить все права», «Снять все права» (help, «Редактирование
    // роли»): every box of the role - the rights of the configuration and of
    // every object, of their parts and of the commands of forms.
    setEverything(enabled) {
      for (const definition of value.schema.objects) {
        this.setAll(definition.id, enabled);
        for (const part of definition.fields) this.setAllOfPart(definition.id, part.key, enabled);
      }
      for (const form of value.schema.forms) for (const command of form.commands) this.setCommand(form.id, command.id, enabled);
    },
    hasCommand(form, command) { return value.role.commands.some(item => item.form === form && item.command === command); },
    setCommand(form, command, enabled) {
      if (!value.schema.forms.some(item => item.id === form && item.commands.some(item => item.id === command))) return;
      value.role.commands = value.role.commands.filter(item => item.form !== form || item.command !== command);
      if(enabled)value.role.commands.push({form,command});
    },
    restrictions: (id, right) => structuredClone(restrictionsOf(id, right)),
    restrictedByFields: byFields,
    putRestriction(id, right, index, restriction) { putRestriction(id, right, index, restriction); },
    restrictionClashes, writeRestrictions, removeRestriction,
    templates: () => structuredClone(value.role.restrictionTemplates || []),
    putTemplate, writeTemplates, removeTemplate,
    hasUnavailable() { return JSON.stringify(clean()) !== JSON.stringify(repaired()); },
    removeUnavailable() { value.role = repaired(); reindex(); },
  };
}

/* The caption of a folder of parts - Реквизиты, Табличные части, Команды. */
function rolePartGroupTitle(schema, group) { return schema.partGroups?.find(item => item.key === group)?.title || group; }

/* The tree of the configuration as both windows of rights show it - the
   role's editor and «Все роли»: the configuration itself at the top, then
   the objects kind by kind in the order the server lists them, each with its
   parts in folders and the objects inside it under their kind - a
   recalculation under its register, a table under its source - and the
   commands of forms last. A choice is {object}, {object, part} or {form};
   selection() answers the one made, choose(next) makes one. expanded keeps
   the keys of the nodes left open, so a redraw keeps them open. subsystems()
   answers the filter by subsystems the tree is under, or null: the
   configuration itself stays, a subsystem not passing stays only on the way
   to one that does. */
function createRoleTree({schema, kindTitles, title, expanded, selection, choose, subsystems = () => null}) {
  const element = node('div', undefined, 'role-tree');
  const kindTitle = kind => kindTitles?.[kind] || kind;
  const partGroupTitle = group => rolePartGroupTitle(schema, group);
  let filter = '';
  function treeButton(text, hint, className, isSelected, next) {
    const button = node('button', text, className + (isSelected ? ' selected' : ''));
    button.type = 'button'; if (hint) button.title = hint;
    button.addEventListener('click', event => { event.preventDefault(); choose(next); render(); });
    return button;
  }
  // A node with something beneath it opens and closes; its caption chooses it.
  function branch(key, caption, children, open) {
    const block = node('details', undefined, 'role-tree-branch');
    block.open = open || expanded.has(key);
    block.addEventListener('toggle', () => { if (block.open) expanded.add(key); else expanded.delete(key); });
    const summary = node('summary'); summary.append(caption); block.append(summary, ...children);
    return block;
  }
  function folder(key, caption, children, open) {
    const block = node('details', undefined, 'role-tree-folder');
    block.open = open || expanded.has(key);
    block.addEventListener('toggle', () => { if (block.open) expanded.add(key); else expanded.delete(key); });
    block.append(node('summary', caption), ...children);
    return block;
  }
  const matches = (item, needle) => !needle || `${item.name} ${title(item)}`.toLocaleLowerCase().includes(needle);
  // The nodes of one object: its parts in folders, then the objects inside it.
  // Null when neither the object nor anything in it matches the filter.
  function objectNode(item, inside, needle, shown) {
    const selected = selection();
    // An object the filter by subsystems leaves out keeps only the way to
    // what passes inside it, as a subsystem above a marked one.
    const passes = shown(item.id), own = passes && matches(item, needle), parts = new Map();
    for (const part of passes ? item.fields : []) {
      if (!parts.has(part.parent || '')) parts.set(part.parent || '', []);
      parts.get(part.parent || '').push(part);
    }
    const isPart = part => selected?.object?.id === item.id && selected?.part?.key === part.key;
    const partNode = (part, groupKey) => {
      const nested = (parts.get(part.key) || []).filter(child => own || matches(part, needle) || matches(child, needle));
      const caption = treeButton(title(part) || part.name, part.name, 'role-tree-item', isPart(part), {object: item, part});
      if (!(parts.get(part.key) || []).length) return caption;
      const children = nested.map(child => treeButton(title(child) || child.name, child.name, 'role-tree-item', isPart(child), {object: item, part: child}));
      return branch(`${item.id} ${groupKey} ${part.key}`, caption, children, !!needle);
    };
    const folders = [];
    for (const group of schema.partGroups || []) {
      const list = (parts.get('') || []).filter(part => part.group === group.key)
        .filter(part => own || matches(part, needle) || (parts.get(part.key) || []).some(child => matches(child, needle)));
      if (list.length) folders.push(folder(`${item.id} ${group.key}`, partGroupTitle(group.key), list.map(part => partNode(part, group.key)), !!needle));
    }
    for (const [kind, list] of inside.get(item.id) || []) {
      // What lies inside an object that matches is shown whole, as its parts are.
      const nodes = list.map(child => objectNode(child, inside, own ? '' : needle, shown)).filter(Boolean);
      if (nodes.length) folders.push(folder(`${item.id} ${kind}`, kindTitle(kind), nodes, !!needle || !passes));
    }
    if (!own && !folders.length) return null;
    const caption = treeButton(title(item) || item.name, item.name, 'role-tree-item', selected?.object?.id === item.id && !selected.part, {object: item});
    caption.dataset.object = item.id;
    if (!folders.length) return caption;
    return branch(item.id, caption, folders, (!!needle && !own) || !passes);
  }
  function render(next = filter) {
    filter = next;
    const needle = filter.toLocaleLowerCase().trim(), shown = subsystemFilterPasses(schema, subsystems());
    element.replaceChildren(); const groups = new Map(), inside = new Map();
    for (const item of schema.objects) {
      if (!item.owner) continue;
      if (!inside.has(item.owner)) inside.set(item.owner, new Map());
      const byKind = inside.get(item.owner);
      if (!byKind.has(item.kind)) byKind.set(item.kind, []);
      byKind.get(item.kind).push(item);
    }
    for (const item of schema.objects) {
      if (item.root) {
        if (matches(item, needle)) { const button = treeButton(title(item) || item.name, item.name, 'role-tree-item role-tree-root', selection()?.object?.id === item.id, {object: item}); button.dataset.object = item.id; element.append(button); }
        continue;
      }
      if (item.owner) continue;
      const entry = objectNode(item, inside, needle, shown);
      if (!entry) continue;
      let group = groups.get(item.kind);
      if (!group) { group = node('details'); group.open = true; group.append(node('summary', kindTitle(item.kind))); groups.set(item.kind, group); element.append(group); }
      group.append(entry);
    }
    const forms = schema.forms.filter(item => shown(item.id) && matches(item, needle));
    if (forms.length) { const group = node('details'); group.open = true; group.append(node('summary', 'Команды форм')); for (const form of forms) group.append(treeButton(title(form), form.name, 'role-tree-item', selection()?.form?.id === form.id, {form})); element.append(group); }
    if (!element.childElementCount) element.append(node('p', 'Нет подходящих объектов', 'muted'));
  }
  return {
    element, render,
    // Opens the objects an object lies inside, so choosing it shows it.
    reveal(object) { for (let owner = object.owner; owner; owner = schema.objects.find(item => item.id === owner)?.owner) expanded.add(owner); },
  };
}

/* The filter by subsystems (help, «Отбор по подсистемам»): chosen holds the
   subsystems marked and, as '', «<Не входящие в подсистемы>»; children takes
   in what the subsystems beneath a marked one hold, parents what those above
   it hold. Answers whether an object passes: an object inside another passes
   with the one it lies in, a subsystem when it is marked itself, as the
   prototype's trees show the subsystems of the filter. Without a filter
   every object passes. */
function subsystemFilterPasses(schema, filter) {
  if (!filter) return () => true;
  const subsystems = schema.subsystems || {}, owners = new Map(schema.objects.map(item => [item.id, item.owner]));
  const above = id => { const result = []; for (let up = owners.get(id); up in subsystems; up = owners.get(up)) result.push(up); return result; };
  const marked = new Set();
  for (const id of Object.keys(subsystems)) {
    if (!filter.chosen.has(id)) { if (filter.children && above(id).some(up => filter.chosen.has(up))) marked.add(id); continue; }
    marked.add(id);
    if (filter.parents) for (const up of above(id)) marked.add(up);
  }
  const held = new Set([...marked].flatMap(id => subsystems[id])), anywhere = new Set(Object.values(subsystems).flat());
  return id => {
    if (id in subsystems) return marked.has(id);
    let top = id;
    while (!(top in subsystems) && owners.get(top)) top = owners.get(top);
    return held.has(top) || (filter.chosen.has('') && !anywhere.has(top));
  };
}

/* The window of the filter by subsystems (help, «Отбор по подсистемам»): the
   tree of subsystems with a box at each, «<Не входящие в подсистемы>» and the
   two flags. One window for every place that filters by subsystems. Without
   a filter in force it opens with everything marked and the subsystems
   beneath taken in, as the prototype's does. As a filter it is set with
   «Установить» or turned off; given apply - «Установить» or «Снять» of the
   role's rights by subsystems - it only chooses, without «Отключить».
   Answers the filter chosen, null for «Отключить», undefined when closed
   without either. */
async function chooseSubsystemFilter({schema, title, current, apply}) {
  const subsystems = schema.subsystems || {}, chosen = new Set(current ? current.chosen : [...Object.keys(subsystems), '']);
  const flags = current ? {children: !!current.children, parents: !!current.parents} : {children: true, parents: false};
  const mark = id => enabled => { if (enabled) chosen.add(id); else chosen.delete(id); };
  const beneath = new Map();
  for (const item of schema.objects) {
    if (!(item.id in subsystems)) continue;
    const parent = item.owner in subsystems ? item.owner : '';
    if (!beneath.has(parent)) beneath.set(parent, []);
    beneath.get(parent).push(item);
  }
  const tree = node('div', undefined, 'role-restriction-fields role-subsystem-tree');
  (function add(parent, depth) {
    for (const item of beneath.get(parent) || []) {
      const box = checkboxField(title(item) || item.name, chosen.has(item.id), mark(item.id), 'role-check');
      box.title = item.name; box.style.setProperty('--depth', depth);
      tree.append(box);
      add(item.id, depth + 1);
    }
  })('', 0);
  tree.append(checkboxField('<Не входящие в подсистемы>', chosen.has(''), mark(''), 'role-check'));
  const content = node('div', undefined, 'role-restriction-form');
  content.append(tree,
    checkboxField('Включать объекты подчиненных подсистем', flags.children, enabled => { flags.children = enabled; }, 'role-check'),
    checkboxField('Включать объекты родительских подсистем', flags.parents, enabled => { flags.parents = enabled; }, 'role-check'));
  const answer = await showDialog({title: 'Отбор по подсистемам', content, className: 'role-restriction-dialog', help: 'subsystem-filter',
    answers: apply ? [['ok', apply], [null, 'Отмена']] : [['ok', 'Установить'], ['off', 'Отключить'], [null, 'Отмена']],
    check: key => key === 'ok' && !chosen.size ? (apply ? 'Отметьте подсистемы.' : 'Отметьте подсистемы или нажмите «Отключить».') : ''});
  if (answer === 'off') return null;
  return answer === 'ok' ? {chosen, ...flags} : undefined;
}

/* The button of the filter by subsystems: it opens the window and stays
   pressed while holder.subsystems, the filter, is in force; applied() is
   called after the filter changed. */
function subsystemFilterButton({schema, title, holder, applied}) {
  const button = node('button', 'По подсистемам', 'role-subsystems');
  button.type = 'button';
  const sync = () => { button.classList.toggle('selected', !!holder.subsystems); button.setAttribute('aria-pressed', String(!!holder.subsystems)); };
  button.addEventListener('click', async () => {
    const next = await chooseSubsystemFilter({schema, title, current: holder.subsystems});
    if (next === undefined) return;
    holder.subsystems = next; sync(); applied();
  });
  sync();
  return button;
}

/* A list of rows and the commands of the prototype's context menu:
   Добавить, Изменить, Удалить. A row is chosen by a click and opened by a
   double click. chosen is the index of the row chosen, or a Set of them
   where several are chosen at once, as in the prototype's «Все ограничения
   доступа»: Ctrl adds a row or takes it away, Shift chooses the rows from the
   last one clicked. A cell is [text, className], or a cell made already. */
function roleListTable(headings, rows, chosen, choose, commands, empty) {
  const table=node('table',undefined,'role-list'),head=node('tr');
  for(const text of headings)head.append(node('th',text));
  const thead=node('thead');thead.append(head);const body=node('tbody');
  const many=chosen instanceof Set,isChosen=index=>many?chosen.has(index):index===chosen;
  let anchor=many?[...chosen].at(-1)??-1:chosen;
  function click(index,event){
    if(!many){chosen=index;choose(index);}
    else{
      if(event?.ctrlKey||event?.metaKey){if(!chosen.delete(index))chosen.add(index);anchor=index;}
      else if(event?.shiftKey&&anchor>=0){chosen.clear();for(let at=Math.min(anchor,index);at<=Math.max(anchor,index);at++)chosen.add(at);}
      else{chosen.clear();chosen.add(index);anchor=index;}
      choose(chosen);
    }
    [...body.children].forEach((other,at)=>other.classList.toggle('selected',isChosen(at)));
    syncCommands();
  }
  rows.forEach((cells,index)=>{
    const row=node('tr',undefined,isChosen(index)?'selected':undefined);row.tabIndex=0;
    for(const cell of cells){if(cell instanceof Node){row.append(cell);continue;}const [text,className]=cell,item=node('td',text,className);item.title=text;row.append(item);}
    row.addEventListener('click',event=>click(index,event));
    row.addEventListener('dblclick',()=>{if(!isChosen(index))click(index);commands.find(([key])=>key==='edit')?.[2]();});
    row.addEventListener('keydown',event=>{if(event.key==='Enter'){event.preventDefault();row.dispatchEvent(new MouseEvent('dblclick'));}if(event.key==='Delete'){event.preventDefault();if(!isChosen(index))click(index);commands.find(([key])=>key==='remove')?.[2]();}});
    body.append(row);
  });
  if(!rows.length){const row=node('tr'),cell=node('td',empty,'muted');cell.colSpan=headings.length;row.append(cell);body.append(row);}
  table.append(thead,body);
  const bar=node('div',undefined,'role-bulk'),buttons=[];
  for(const [key,text,action,enabled] of commands){const button=node('button',text);button.type='button';button.dataset.command=key;button.addEventListener('click',action);buttons.push([button,enabled]);bar.append(button);}
  function syncCommands(){for(const [button,enabled] of buttons)button.disabled=!enabled();}
  syncCommands();
  const block=node('div');block.append(table,bar);return block;
}

/* The caption of a list in the windows of restrictions, with «Отметить все»
   and «Снять все» over it, as the prototype has them: mark(true) marks every
   item, mark(false) takes every mark away. */
function markedListCaption(caption, mark, extra = []) {
  const heading = node('div', undefined, 'overview-wizard-caption'), commands = node('span', undefined, 'role-list-marks');
  for (const [text, enabled] of [['Отметить все', true], ['Снять все', false]]) {
    const button = node('button', text, 'role-mark-all');
    button.type = 'button';
    button.addEventListener('click', () => mark(enabled));
    commands.append(button);
  }
  commands.append(...extra);
  heading.append(node('span', caption, 'role-restriction-caption'), commands);
  return heading;
}

/* The fields of a restriction of reading, as the prototype's windows show
   them (help, form_RLFieldsListDlg; checked in the configurator 14.10.2026):
   one column by names, a table part folding under its own box with its fields
   beneath, «Отметить все» and «Снять все» over the list and «Прочие поля»
   under it. Marking a field takes «Прочие поля» away, marking «Прочие поля»
   takes every field away, taking the last field away marks «Прочие поля».
   chosen holds the keys marked; taken(key) - a field another restriction
   names, which cannot be marked. One choice serves the window of one
   restriction, the group «Изменить» and the wizard. Answers the block with
   fill(fields) - the fields, {key, name, parent, hint}, parent the key of
   the table part; filled() - whether a field or «Прочие поля» is marked;
   disable(flag). */
function restrictionFieldsChoice({chosen, taken = () => false}) {
  let others = !chosen.size, disabled = false, fields = [];
  const boxes = new Map(), list = node('div', undefined, 'role-restriction-fields role-fields-tree');
  const othersBox = checkboxField('Прочие поля', others, enabled => {
    others = enabled;
    if (enabled) chosen.clear();
    sync();
  }, 'role-check role-fields-others');
  function mark(key, enabled) {
    if (enabled) chosen.add(key); else chosen.delete(key);
    others = !chosen.size;
    sync();
  }
  // The marks are put on the boxes in place: a list drawn anew would take
  // the focus away from the box just pressed.
  function sync() {
    for (const [key, input] of boxes) { input.checked = chosen.has(key); input.disabled = disabled || taken(key); }
    const input = othersBox.querySelector('input');
    input.checked = others; input.disabled = disabled;
    for (const button of block.querySelectorAll('.role-mark-all')) button.disabled = disabled;
  }
  // A table part folds: its fields are under it, hidden until it is opened.
  function row(field, depth, under) {
    const line = node('div', undefined, 'role-fields-row'), children = fields.filter(item => item.parent === field.key), inside = [];
    let fold = null;
    line.style.setProperty('--depth', depth);
    line.hidden = !!under;
    if (children.length) {
      const toggle = node('button', '▸', 'role-fields-toggle');
      toggle.type = 'button';
      fold = open => {
        toggle.textContent = open ? '▾' : '▸';
        toggle.setAttribute('aria-expanded', String(open));
        toggle.setAttribute('aria-label', `${open ? 'Свернуть' : 'Развернуть'} ${field.name}`);
        for (const child of inside) child.hidden = !open;
      };
      toggle.addEventListener('click', () => fold(toggle.getAttribute('aria-expanded') !== 'true'));
      line.append(toggle);
    } else line.append(node('span', undefined, 'role-fields-toggle'));
    const box = checkboxField(field.name, false, enabled => mark(field.key, enabled), 'role-check');
    if (field.hint && field.hint !== field.name) box.title = field.hint;
    boxes.set(field.key, box.querySelector('input'));
    line.append(box);
    list.append(line);
    for (const child of children) inside.push(row(child, depth + 1, line));
    fold?.(false);
    return line;
  }
  const block = node('div', undefined, 'role-restriction-form role-fields-choice');
  block.append(markedListCaption('Поля:', enabled => {
    if (enabled) for (const field of fields) { if (!taken(field.key)) chosen.add(field.key); } else chosen.clear();
    others = !chosen.size;
    sync();
  }), list, othersBox);
  return Object.assign(block, {
    fill(next) {
      // A field that is gone takes its mark with it; the last one gone marks
      // «Прочие поля», as taking it away would.
      fields = next;
      const keys = new Set(fields.map(field => field.key)), before = chosen.size;
      for (const key of [...chosen]) if (!keys.has(key)) chosen.delete(key);
      if (before && !chosen.size) others = true;
      list.replaceChildren(); boxes.clear();
      for (const field of fields) if (!field.parent || !keys.has(field.parent)) row(field, 0);
      if (!fields.length) list.append(node('p', 'Полей для ограничения нет.', 'muted'));
      sync();
    },
    filled: () => chosen.size > 0 || others,
    disable(flag) { disabled = flag; sync(); },
  });
}

/* The window of a restriction's fields and condition, one for the window of
   one restriction and for the group «Изменить» (help, form_EditRowLevelRightDlg):
   «Изменять состав полей» and «Изменять ограничение» over the fields on the
   left and the condition on the right. In the window of one restriction both
   are marked and cannot be taken away; in the group they change only what is
   marked. fields is a restrictionFieldsChoice, or null where the right is not
   restricted by fields. change.fields and change.condition hold whether each
   is changed; group - whether they can be taken away. */
function restrictionWindowContent({fields, editor, change, group}) {
  const content = node('div', undefined, 'role-restriction-form role-restriction-window');
  const flag = (label, key, controls) => {
    const box = checkboxField(label, change[key], enabled => { change[key] = enabled; controls(enabled); }, 'role-check', !group);
    controls(change[key]);
    return box;
  };
  const left = node('div', undefined, 'role-restriction-form'), right = node('div', undefined, 'role-restriction-form');
  if (fields) left.append(flag('Изменять состав полей', 'fields', enabled => fields.disable(!enabled)), fields);
  right.append(flag('Изменять ограничение', 'condition', enabled => { editor.textarea.disabled = !enabled; }),
    node('div', 'Ограничение доступа:', 'role-restriction-caption'), editor);
  if (fields) content.append(left);
  else content.classList.add('role-restriction-single');
  content.append(right);
  return content;
}

/* The restrictions of access to data a role puts on the rights of an object,
   as the prototype lists them under the rights (help, «Ограничение доступа к
   данным»): the field or «Прочие поля», and the condition. A right restricted
   by fields takes one per set of fields, the others one each. model is the role's
   (createRoleModel); state keeps the right shown and the row chosen across
   redraws; changed() is called after the role changed, to redraw. */
function roleRestrictionSection({model, object, caption, title, state, changed}) {
  const section=node('section',undefined,'role-restrictions');
  section.append(node('h4','Ограничение доступа к данным'));
  if(!object.restrictedRights.includes(state.right)){state.right=object.restrictedRights[0];state.row=-1;}
  const tabs=node('div',undefined,'role-restriction-rights');
  for(const right of object.restrictedRights){
    const count=model.restrictions(object.id,right).length,button=node('button',count?`${caption(right)} (${count})`:caption(right),right===state.right?'selected':undefined);
    button.type='button';button.setAttribute('aria-pressed',String(right===state.right));
    button.addEventListener('click',()=>{state.right=right;state.row=-1;changed();});
    tabs.append(button);
  }
  section.append(tabs);
  const right=state.right,list=model.restrictions(object.id,right);
  if(state.row>=list.length)state.row=-1;
  const others=list.some(item=>!item.fields?.length);
  const canAdd=()=>model.restrictedByFields(right)?!others||object.restrictionFields.some(field=>!list.some(item=>item.fields?.includes(field.key))):!list.length;
  const open=async index=>{if(await editRoleRestriction({model,object,right,index,caption,title})){state.row=index;changed();}};
  section.append(roleListTable(['Поле','Ограничение доступа'],list.map(item=>[[restrictionFieldsCaption(object,item,title)],[item.condition||'','role-condition']]),state.row,index=>{state.row=index;},[
    ['add','Добавить',()=>open(list.length),canAdd],
    ['edit','Изменить',()=>{if(state.row>=0)open(state.row);},()=>state.row>=0],
    ['remove','Удалить',()=>{if(state.row<0)return;model.removeRestriction(object.id,right,state.row);state.row=-1;changed();},()=>state.row>=0],
  ],'Ограничений нет'));
  return section;
}

/* The window of one restriction: the fields it checks, where the right is
   restricted by fields, and its condition in the query language (help,
   «Ограничение доступа»), laid out as the group «Изменить» with both flags
   marked. Answers whether the restriction was put in the role. */
async function editRoleRestriction({model,object,right,index,caption,title}){
  const list=model.restrictions(object.id,right),current=list[index]||{right};
  const chosen=new Set(current.fields||[]);
  let fields=null;
  if(model.restrictedByFields(right)){
    const taken=new Set(list.flatMap((item,other)=>other===index?[]:item.fields||[]));
    fields=restrictionFieldsChoice({chosen,taken:key=>taken.has(key)});
    fields.fill(object.restrictionFields.map(field=>({key:field.key,name:field.name,parent:field.parent,hint:title(field)})));
  }
  let condition=current.condition||'';
  const editor=queryTextField(condition,text=>{condition=text;},'Ограничение доступа');
  const content=restrictionWindowContent({fields,editor,change:{fields:true,condition:true},group:false});
  // A field another restriction names cannot be marked, so the only
  // collision left is a second one on «Прочие поля».
  const answer=await showDialog({title:`Ограничение доступа · ${caption(right)}`,content,className:'role-restriction-dialog',help:'role-restriction',
    answers:[['ok','ОК',{enabled:()=>!fields||fields.filled()}],[null,'Отмена']],
    check:()=>model.restrictionClashes(object.id,right,index,{fields:[...chosen]}).length?'Ограничение на прочие поля у этого права уже есть: отметьте поля.':''});
  if(answer!=='ok')return false;
  model.putRestriction(object.id,right,index,{fields:[...chosen],condition});
  return true;
}

function createRoleEditor(host, onChange) {
  let source, model, selected, panel, tree, warning, titleFieldCleanup = null;
  // The tab shown - the rights or the templates of restrictions - and the
  // right whose restrictions are listed, with the row chosen in each list.
  let view = 'rights', templateRow = -1, rightsBody, templatesBody;
  const restriction = {right: null, row: -1};
  // The filter by subsystems of the tree of objects, while the role is open.
  const treeFilter = {subsystems: null};
  // The nodes of the tree left open, by key, so a redraw keeps them open.
  const expanded = new Set();
  function clearTitleField() { titleFieldCleanup?.(); titleFieldCleanup = null; }
  /* Подписи прав, видов и стандартных полей приходят с сервера, из
     единственных перечней: свой словарь здесь разошёлся бы с ними при первой
     правке одного из двух. */
  const caption = key => source.schema.rights.find(item => item.key === key)?.title || key;
  function title(item) { return localizedTitle(item, source.defaultLanguage, source.languages); }
  /* Подписи объектов и полей показываются на текущем языке Studio и
     меняются вместе с ним. */
  StudioLanguage.onChange(() => { if (!source) return; tree.render(); renderPanel(); if (view === 'templates') renderTemplates(); });
  function changed() { warning.hidden=!model.hasUnavailable(); onChange(); }
  function checkbox(text, checked, action, disabled=false) {
    const label=checkboxField(text,checked,enabled=>{const focus=label.firstChild.getAttribute('aria-label')||text;action(enabled);changed();renderPanel();for(const candidate of panel.querySelectorAll('input'))if((candidate.getAttribute('aria-label')||candidate.parentElement.textContent)===focus){candidate.focus();break;}},'role-check',disabled);
    return label;
  }
  function renderPanel() {
    panel.replaceChildren();
    if(!selected){panel.append(node('p','Выберите объект слева и отметьте разрешённые действия.','muted'));return;}
    if(selected.form){const form=selected.form;panel.append(node('h3',title(form)),node('p','Доступ к команде не заменяет права на используемые данные.','muted'));for(const command of form.commands)panel.append(checkbox(title(command),model.hasCommand(form.id,command.id),enabled=>model.setCommand(form.id,command.id,enabled)));return;}
    const object=selected.object;
    const bulk=(set)=>{const all=node('div',undefined,'role-bulk');for(const [text,enabled] of [['Разрешить всё',true],['Снять все права',false]]){const button=node('button',text);button.type='button';button.addEventListener('click',()=>{set(enabled);changed();renderPanel();});all.append(button);}return all;};
    if(selected.part){
      const part=selected.part;
      panel.append(node('h3',title(part)||part.name),node('p',`${title(object)||object.name} · ${rolePartGroupTitle(source.schema,part.group)}`,'muted'));
      if(!part.operations.length){panel.append(node('p','Своих прав у этого узла нет: права даются на то, что в нём.','muted'));return;}
      const actions=node('div',undefined,'role-operation-list');
      for(const operation of part.operations)actions.append(checkbox(caption(operation),model.hasField(object.id,part.key,operation),enabled=>model.setField(object.id,part.key,operation,enabled)));
      panel.append(actions,bulk(enabled=>model.setAllOfPart(object.id,part.key,enabled)));
      return;
    }
    panel.append(node('h3',title(object)||object.name));
    if(object.rights.length){
      const actions=node('div',undefined,'role-operation-list');
      for(const operation of object.rights)actions.append(checkbox(caption(operation),model.hasObject(object.id,operation),enabled=>model.setObject(object.id,operation,enabled)));
      panel.append(actions,bulk(enabled=>model.setAll(object.id,enabled)));
    } else panel.append(node('p','Своих прав у объекта нет: права даются на его части.','muted'));
    if(object.restrictedRights?.length)panel.append(roleRestrictionSection({model,object,caption,title,state:restriction,changed:()=>{changed();renderPanel();}}));
  }
  // The templates of the role's restrictions (help, «Редактирование роли»,
  // «Шаблоны ограничений»): a name, with its parameters in brackets, and a
  // text the condition of a restriction takes in place of #Имя(...).
  function renderTemplates(){
    templatesBody.replaceChildren();
    const list=model.templates();
    if(templateRow>=list.length)templateRow=-1;
    templatesBody.append(node('p','Имя шаблона — имя по правилам языка, в скобках можно перечислить параметры: Шаблон(Поле, Пользователь). В тексте ограничения шаблон вызывается как #Шаблон("значение", ...).','muted'));
    templatesBody.append(roleListTable(['Имя','Текст шаблона'],list.map(item=>[[item.name],[item.condition||'','role-condition']]),templateRow,index=>{templateRow=index;},[
      ['add','Добавить',()=>editTemplate(list.length),()=>true],
      ['edit','Изменить',()=>{if(templateRow>=0)editTemplate(templateRow);},()=>templateRow>=0],
      ['remove','Удалить',()=>{if(templateRow<0)return;model.removeTemplate(templateRow);templateRow=-1;changed();renderTemplates();},()=>templateRow>=0],
    ],'Шаблонов нет'));
  }
  async function editTemplate(index){
    const current=model.templates()[index]||{name:'',condition:''},content=node('div',undefined,'role-restriction-form');
    let name=current.name,condition=current.condition||'';
    const nameInput=node('input');nameInput.value=name;nameInput.setAttribute('aria-label','Наименование');nameInput.addEventListener('input',()=>{name=nameInput.value;});
    content.append(node('div','Наименование','role-restriction-caption'),nameInput,node('div','Текст шаблона','role-restriction-caption'),queryTextField(condition,text=>{condition=text;},'Текст шаблона'));
    const answer=await showDialog({title:'Шаблон ограничения',content,className:'role-restriction-dialog',answers:[['ok','ОК'],[null,'Отмена']],check:()=>name.trim()?'':'Укажите наименование шаблона.'});
    if(answer!=='ok')return;
    model.putTemplate(index,{name,condition});templateRow=index;changed();renderTemplates();
  }
  // «Установить по подсистемам...», «Снять по подсистемам...» (help,
  // «Редактирование роли»): every right of each object of the subsystems
  // chosen, and «Просмотр» of those subsystems; the configuration itself
  // keeps its rights.
  async function rightsBySubsystems(enabled){
    const filter=await chooseSubsystemFilter({schema:source.schema,title,current:null,apply:enabled?'Установить':'Снять'});
    if(!filter)return;
    const passes=subsystemFilterPasses(source.schema,filter);
    for(const item of source.schema.objects)if(!item.root&&passes(item.id))model.setAll(item.id,enabled);
    changed();renderPanel();
  }
  function showView(next){view=next;rightsBody.hidden=view!=='rights';templatesBody.hidden=view!=='templates';for(const button of host.querySelectorAll('.role-tabs button'))button.setAttribute('aria-selected',String(button.dataset.view===view));if(view==='templates')renderTemplates();}
  return {
    open(value, preserveSelection=false) {
      clearTitleField();
      const previous=preserveSelection?selected:null;if(!preserveSelection)expanded.clear();source=structuredClone(value);model=createRoleModel(source);selected=null;if(previous?.object){const object=source.schema.objects.find(item=>item.id===previous.object.id);const part=previous.part&&object?.fields.find(item=>item.key===previous.part.key);if(object)selected=part?{object,part}:{object};}if(previous?.form){const form=source.schema.forms.find(item=>item.id===previous.form.id);if(form)selected={form};}host.hidden=false;host.replaceChildren();
      const identity=node('div',undefined,'role-identity'),label=node('label','Имя роли'),name=node('input');name.value=source.role.name;name.maxLength=128;name.addEventListener('input',()=>{model.setName(name.value);changed();});label.append(name);identity.append(label);
      const titleLabel=node('label','Заголовок'),titleField=createLocalizedTitleField(source.languages,code=>source.role.title[code],(code,value)=>model.setTitle(code,value),changed);
      titleFieldCleanup=titleField.destroy;titleLabel.append(titleField);identity.append(titleLabel);
      const commentLabel=node('label','Комментарий'),comment=node('textarea');comment.value=source.role.comment||'';comment.maxLength=4000;comment.rows=2;
      comment.addEventListener('change',()=>{model.setComment(comment.value);changed();});commentLabel.append(comment);identity.append(commentLabel);
      identity.append(checkbox('Устанавливать права для новых объектов',!!source.role.grantNewObjectsByDefault,enabled=>model.setGrantNewObjectsByDefault(enabled)));
      // The prototype asks before the flag for attributes changes whether to
      // set or clear the rights on every attribute and table part.
      const fieldsFlag=checkboxField('Устанавливать права для реквизитов и табличных частей по умолчанию',!!source.role.grantNewFieldsByDefault,async enabled=>{
        const answer=await askChoice(enabled?'Установить права на все реквизиты и табличные части?':'Снять права со всех реквизитов и табличных частей?',
          '«Нет» меняет только флаг: права, которые уже действуют, останутся как были.',[['yes','Да'],['no','Нет'],[null,'Отмена']]);
        if(answer===null){fieldsFlag.querySelector('input').checked=!enabled;return;}
        model.setGrantNewFieldsByDefault(enabled,answer==='yes');changed();renderPanel();
      },'role-check');
      identity.append(fieldsFlag);
      identity.append(checkbox('Независимые права подчиненных объектов',!!source.role.independentChildRights,enabled=>model.setIndependentChildRights(enabled)));
      host.append(identity);
      warning=node('div',undefined,'role-warning');warning.append(node('span','В роли есть права на удалённые или изменённые объекты. '));const repair=node('button','Убрать недоступные права');repair.type='button';repair.addEventListener('click',()=>{model.removeUnavailable();changed();renderPanel();});warning.append(repair);warning.hidden=!model.hasUnavailable();host.append(warning);
      // The prototype's role window has two tabs: the rights and the
      // templates of restrictions.
      const tabs=node('div',undefined,'role-tabs');tabs.setAttribute('role','tablist');
      for(const [key,text] of [['rights','Права'],['templates','Шаблоны ограничений']]){const button=node('button',text);button.type='button';button.dataset.view=key;button.setAttribute('role','tab');button.addEventListener('click',()=>showView(key));tabs.append(button);}
      host.append(tabs);
      const body=node('div',undefined,'role-body'),sidebar=node('div',undefined,'role-sidebar'),search=node('input');search.type='search';search.placeholder='Поиск объекта';search.setAttribute('aria-label','Поиск объекта');tree=createRoleTree({schema:source.schema,kindTitles:source.kindTitles,title,expanded,selection:()=>selected,choose:next=>{selected=next;renderPanel();},subsystems:()=>treeFilter.subsystems});search.addEventListener('input',()=>tree.render(search.value));
      if(!preserveSelection)treeFilter.subsystems=null;
      const bar=node('div',undefined,'role-tree-bar'),actions=node('button','Действия');actions.type='button';actions.setAttribute('aria-haspopup','menu');
      actions.addEventListener('click',event=>showContextMenu(event,[['Установить все права',()=>{model.setEverything(true);changed();renderPanel();}],['Снять все права',()=>{model.setEverything(false);changed();renderPanel();}],['Установить по подсистемам...',()=>rightsBySubsystems(true)],['Снять по подсистемам...',()=>rightsBySubsystems(false)]]));
      bar.append(actions,subsystemFilterButton({schema:source.schema,title,holder:treeFilter,applied:()=>tree.render(search.value)}));
      sidebar.append(bar,search,tree.element);panel=node('section',undefined,'role-permissions');body.append(sidebar,panel);rightsBody=body;templatesBody=node('section',undefined,'role-templates');host.append(body,templatesBody);
      if(!preserveSelection){view='rights';restriction.right=null;restriction.row=-1;templateRow=-1;}
      tree.render();renderPanel();showView(view);
    },
    // Переход из окна «Все роли» происходит по конкретному объекту, а не по
    // роли вообще: открывать роль на пустом месте значило бы заставить искать
    // в дереве то, что только что было на экране.
    selectObject(id){const object=source?.schema.objects.find(item=>item.id===id);if(!object)return;selected={object};showView('rights');tree.reveal(object);tree.render();renderPanel();},
    value(){return model?.value();},
    close(){clearTitleField();source=null;model=null;host.hidden=true;host.replaceChildren();},
    setDisabled(disabled){host.inert=disabled;},
  };
}
