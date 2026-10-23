/* Role editor: explicit grants underneath, ordinary tree/checkboxes in Studio. */
/* Whether a restriction of the right names the fields it checks - one per
   set of fields, a field in one at most, and one more on «Прочие поля» - or
   the right takes one restriction, on every field. The rules are the
   server's (RestrictionRules), sent with the schema. */
function restrictsByFields(schema, right) {
  return !!(schema.restrictionRules || []).find(rule => rule.right === right)?.byFields;
}
/* The box over many: marked when every one is, cleared when none is, the
   third state otherwise - 'all', 'none', 'some'. */
function markOfMany(values) {
  const set = values.filter(Boolean).length;
  return !values.length || !set ? 'none' : set === values.length ? 'all' : 'some';
}
function createRoleModel(source) {
  // Only the role is copied: the schema is read and never changed, and the
  // window «Все роли» builds a model for every role over one schema.
  const value = {...source, role: structuredClone(source.role)};
  value.role.objects ||= [];
  const definitions = new Map(value.schema.objects.map(item => [item.id, item]));
  const fields = new Map(value.schema.objects.map(item => [item.id, new Map(item.fields.map(field => [field.key, field]))]));
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
  // Only a right restricted by fields takes them. A field the object does
  // not have now is taken as written: the windows pass only the fields they
  // show, and what the change does not touch - the condition changed alone,
  // a restriction copied - keeps the field, as the prototype keeps it
  // (ACCESS-RIGHTS, «Ограничение по полю, которого у объекта нет»).
  const byFields = right => restrictsByFields(value.schema, right);
  function restrictionEntry(id, right, restriction) {
    const entry = {right};
    const chosen = byFields(right) ? restriction.fields || [] : [];
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
  // part of one, a restriction of a right the object is not offered - and the rights
  // ML no longer keeps, putting the one standing for each in its place: the
  // thin and the web client become «Запуск приложения», and the rights on
  // commands of forms an earlier ML gave go, a role giving none (ACCESS-RIGHTS,
  // «Роли, сохранённые прежней версией ML»). A right the object has no use for
  // is carried, as the prototype carries it, and is never repaired away.
  function repaired() {
    const result = clean();
    const parts = value.schema.parts || {}, dropped = value.schema.droppedRights || {};
    result.objects = result.objects.filter(item => parts[item.object]);
    for (const item of result.objects) {
      // A right given through either right it replaces is given; one denied
      // through them is denied only if neither gives it.
      const kept = list => [...new Set((list || []).map(right => right in dropped ? dropped[right] : right).filter(Boolean))];
      if (item.operations.some(right => right in dropped) || item.denied?.some(right => right in dropped)) {
        item.operations = kept(item.operations);
        const denied = kept(item.denied).filter(right => !item.operations.includes(right));
        if (denied.length) item.denied = denied; else delete item.denied;
      }
      item.fields = item.fields.filter(field => parts[item.object].includes(field.field));
      // A restriction goes with the right it restricts. One naming a field
      // the object no longer has is kept as it is, as the prototype keeps
      // it, saying nothing (ACCESS-RIGHTS, «Ограничение по полю, которого у
      // объекта нет»): its window shows it on «Прочие поля».
      if (item.restrictions) {
        const rights = definitions.get(item.object)?.rights || [];
        item.restrictions = item.restrictions.filter(restriction => rights.includes(restriction.right));
        if (!item.restrictions.length) delete item.restrictions;
      }
    }
    result.objects = result.objects.filter(item => item.operations.length || item.denied?.length || item.fields.length);
    delete result.commands;
    return result;
  }
  return {
    value: clean,
    setName(name) { value.role.name = name; },
    setTitle(language, title) { value.role.title = setTranslation(value.role.title, language, title); },
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
    // A right over many - the parts of a folder, the objects of a kind
    // (members, roleGroupMembers): of those that have it, marked at all,
    // at none or at some; setting it sets it at every one of them. As the
    // prototype's window, marking a right marks the rights it needs and
    // clearing it clears the rights needing it, each member taking of them
    // the rights it has, as setObject and setField take no other
    // (schema.requires; block-00/rol «Права зависят друг от друга»).
    groupMark(members, operation) {
      return markOfMany(members.filter(member => roleMemberRights(member).includes(operation))
        .map(({object, part}) => part ? partRight(object.id, part.key, operation) : objectRight(object.id, operation)));
    },
    setGroup(members, operation, enabled) {
      for (const member of members) {
        const {object, part} = member, own = roleMemberRights(member), required = value.schema.requires || {};
        const tied = enabled ? required[operation] || [] : own.filter(right => required[right]?.includes(operation));
        for (const right of [operation, ...tied])
          if (part) setField(object.id, part.key, right, enabled); else setObject(object.id, right, enabled);
      }
    },
    setAllOfPart(id, key, enabled) { for(const operation of fields.get(id)?.get(key)?.operations || [])setField(id, key, operation, enabled); },
    // «Установить все права», «Снять все права» (help, «Редактирование
    // роли»): every box of the role - the rights of the configuration and of
    // every object and of their parts.
    setEverything(enabled) {
      for (const definition of value.schema.objects) {
        this.setAll(definition.id, enabled);
        for (const part of definition.fields) this.setAllOfPart(definition.id, part.key, enabled);
      }
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

/* The rights of one member of a group: of a part, or of an object. */
function roleMemberRights({object, part}) { return part ? part.operations : object.rights; }

/* What a folder or a branch of the tree of rights holds, as the prototype
   gives its rights over (pass of 20.10.2026, block-00/rol, «Права папки и
   ветви вида»): the parts of an object in a folder of parts, the objects
   inside an object of one kind - the recalculations of a register -, the
   objects of a kind in its branch; «Общие» holds nothing rights are given
   over. Only what lies right in it: the attributes of a table part are the
   table part's. group is the choice the tree makes - {object, partGroup},
   {object, kind}, {kind} or {common}. */
function roleGroupMembers(schema, group) {
  if (group.common) return [];
  if (group.partGroup) {
    const object = schema.objects.find(item => item.id === group.object);
    return (object?.fields || []).filter(part => !part.parent && part.group === group.partGroup).map(part => ({object, part}));
  }
  const owner = group.object || undefined;
  return schema.objects.filter(item => !item.root && item.kind === group.kind && item.owner === owner).map(object => ({object}));
}

/* The rights a group shows: every right one of its members has, in the
   order of the list of rights; an empty folder shows the rights it gives
   over what it would hold, as the prototype's (group.rights,
   schema.folders). */
function roleGroupRights(schema, members, group) {
  if (!members.length) return group.rights || [];
  return schema.rights.map(right => right.key).filter(key => members.some(member => roleMemberRights(member).includes(key)));
}

/* The caption of a folder of parts - Реквизиты, Табличные части, Команды. */
function rolePartGroupTitle(schema, group) { return schema.partGroups?.find(item => item.key === group)?.title || group; }

/* The tree of the configuration as both windows of rights show it - the
   role's editor and «Все роли» - laid out as the project tree is and as the
   prototype's (inventory of block 00, rol.md; snapshot
   configurator-2026-10-14/rol-otbor-kassa): the configuration itself at the
   top with everything beneath it, «Общие» holding the common kinds, the
   branches of the kinds in the order the server lists them, each in the
   place it gives them (kindPlaces: «Общие» - common.place -, the branch of another kind -
   sequences before the documents - or under the root), each object with its
   parts in folders and the objects inside it under their kind - a
   recalculation under its register, a table under its source. Every row is
   drawn as the project tree's (treeNode), with the icon the project tree
   gives the same thing: the kind's, the folder's of parts. A choice is
   {object} or {object, part}; with groups, a folder or a branch is chosen
   too, as {group} (roleGroupMembers), as the window of a role chooses them;
   selection() answers the one made, choose(next) makes one. expanded keeps
   the keys of the nodes left open, so a redraw keeps them open. subsystems()
   answers the filter by subsystems the tree is under, or null: the
   configuration itself stays, a subsystem not passing stays only on the way
   to one that does. source is the window's: the schema of rights, the
   captions, icons and places of the kinds, the branch «Общие». */
function createRoleTree({source, title, expanded, selection, choose, subsystems = () => null, groups = false}) {
  // What the tree is drawn from comes with the source of either window.
  const {schema, kindTitles, kindIcons, kindPlaces, common: commonBranch} = source;
  const element = node('div', undefined, 'role-tree');
  const kindTitle = kind => kindTitles?.[kind] || kind;
  const partGroupTitle = group => rolePartGroupTitle(schema, group);
  const partGroupIcon = group => schema.partGroups?.find(item => item.key === group)?.icon || '';
  let filter = '';
  function treeButton(text, hint, className, isSelected, next) {
    const button = node('button', text, className + (isSelected ? ' selected' : ''));
    button.type = 'button'; if (hint) button.title = hint;
    // The choice moves in place: a tree drawn anew would leave the second
    // click of a double click on a row no longer in it.
    button.addEventListener('click', event => {
      event.preventDefault(); choose(next);
      for (const item of element.querySelectorAll('.role-tree-item.selected')) item.classList.remove('selected');
      button.classList.add('selected');
    });
    return button;
  }
  // A node with something beneath it opens and closes and stays as it was
  // left across redraws, by its key in expanded: an object by its id, a
  // folder by the object's and its own, a branch by branchKey. An object's
  // caption is the button that chooses it; a folder's or a branch's only
  // names it. A filter by text opens what holds a match.
  const branchKey = kind => `branch ${kind}`;
  const foldable = (key, icon, caption, children, open) => treeNode({icon, caption, children, open: open || expanded.has(key),
    toggled: opened => { if (opened) expanded.add(key); else expanded.delete(key); }}).element;
  // The caption of a folder or a branch: with groups, the button that
  // chooses it, known by its key; otherwise it only names it.
  const groupCaption = (text, group) => {
    if (!groups) return text;
    const button = treeButton(text, '', 'role-tree-item role-tree-group', selection()?.group?.key === group.key, {group: {...group, caption: text}});
    button.dataset.group = group.key;
    return button;
  };
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
      const children = nested.map(child => treeNode({icon: partGroupIcon(child.group || groupKey),
        caption: treeButton(title(child) || child.name, child.name, 'role-tree-item', isPart(child), {object: item, part: child})}).element);
      return foldable(`${item.id} ${groupKey} ${part.key}`, partGroupIcon(groupKey), caption, children, !!needle);
    };
    // The folders of the kind in its order, every one even when empty, as
    // the prototype's (schema.folders); the parts of an inline one stand
    // right under the object. Under a filter by text only a folder holding a
    // match stays, unless the object matches itself. Objects inside of a
    // kind the folders do not name - a subsystem's - stand after them, in a
    // folder of their kind while there are any.
    const folders = [], named = new Set();
    const insideFolder = (kind, rights) => {
      named.add(kind);
      // What lies inside an object that matches is shown whole, as its parts are.
      const nodes = (inside.get(item.id)?.get(kind) || []).map(child => objectNode(child, inside, own ? '' : needle, shown)).filter(Boolean);
      const key = `${item.id} ${kind}`;
      if ((own && rights) || nodes.length) folders.push(foldable(key, kindIcons?.[kind], groupCaption(kindTitle(kind), {key, object: item.id, kind, rights: rights || []}), nodes, !!needle || !passes));
    };
    for (const folder of schema.folders?.[item.kind] || []) {
      if (folder.kind) { insideFolder(folder.kind, folder.rights); continue; }
      if (!passes) continue;
      const nodes = (parts.get('') || []).filter(part => part.group === folder.group)
        .filter(part => own || matches(part, needle) || (parts.get(part.key) || []).some(child => matches(child, needle)))
        .map(part => partNode(part, folder.group));
      const key = `${item.id} ${folder.group}`;
      if (folder.inline) folders.push(...nodes);
      else if (own || nodes.length) folders.push(foldable(key, partGroupIcon(folder.group), groupCaption(partGroupTitle(folder.group), {key, object: item.id, partGroup: folder.group, rights: folder.rights}), nodes, !!needle));
    }
    for (const kind of (inside.get(item.id) || new Map()).keys()) if (!named.has(kind)) insideFolder(kind, null);
    if (!own && !folders.length) return null;
    const caption = treeButton(title(item) || item.name, item.name, 'role-tree-item', selected?.object?.id === item.id && !selected.part, {object: item});
    caption.dataset.object = item.id;
    return foldable(item.id, kindIcons?.[item.kind], caption, folders, (!!needle && !own) || !passes);
  }
  // The branch of a kind, with the kind's rights for a branch holding no
  // object (schema.branches).
  const branchRights = new Map((schema.branches || []).map(item => [item.kind, item.rights]));
  const branch = (kind, children) => foldable(branchKey(kind), kindIcons?.[kind], groupCaption(kindTitle(kind), {key: branchKey(kind), kind, rights: branchRights.get(kind) || []}), children, !!filter.trim());
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
    let root = null;
    for (const item of schema.objects) {
      if (item.root) { root = item; continue; }
      if (item.owner) continue;
      const entry = objectNode(item, inside, needle, shown);
      if (!entry) continue;
      if (!groups.has(item.kind)) groups.set(item.kind, []);
      groups.get(item.kind).push(entry);
    }
    // The branches in their places: those standing in another's first in it,
    // then «Общие», then the rest under the root. Every kind rights are given
    // on has its branch, the empty ones too, as the prototype's (zapis.md
    // «Дерево прав», snapshot prava/19), while nothing filters the tree; a
    // filter keeps only the branches holding what passes it.
    const listed = (schema.branches || []).map(item => item.kind), showEmpty = !needle && !subsystems();
    const kinds = [...listed, ...[...groups.keys()].filter(kind => !listed.includes(kind))];
    const placeOf = kind => kindPlaces?.[kind] || '', inAnother = kind => placeOf(kind) && placeOf(kind) !== commonBranch?.place;
    const within = new Map(), common = [], top = [];
    for (const kind of kinds) {
      if (!inAnother(kind) || !(groups.has(kind) || showEmpty)) continue;
      if (!within.has(placeOf(kind))) within.set(placeOf(kind), []);
      within.get(placeOf(kind)).push(branch(kind, groups.get(kind) || []));
    }
    for (const kind of [...kinds, ...[...within.keys()].filter(kind => !kinds.includes(kind))]) {
      if (inAnother(kind)) continue;
      const children = [...(within.get(kind) || []), ...(groups.get(kind) || [])];
      if (!children.length && !showEmpty) continue;
      (placeOf(kind) && placeOf(kind) === commonBranch?.place ? common : top).push(branch(kind, children));
    }
    const commonKey = branchKey(commonBranch?.place);
    if (common.length) top.unshift(foldable(commonKey, commonBranch.icon, groupCaption(commonBranch.title, {key: commonKey, common: true}), common, !!needle));
    if (root && (top.length || matches(root, needle))) {
      const button = treeButton(title(root) || root.name, root.name, 'role-tree-item role-tree-root', selection()?.object?.id === root.id, {object: root});
      button.dataset.object = root.id;
      element.append(foldable(root.id, 'configuration', button, top, !!needle));
    } else element.append(...top);
    if (!element.childElementCount) element.append(node('p', 'Нет подходящих объектов', 'muted'));
  }
  return {
    element, render,
    // Opens everything an object lies inside, so choosing it shows it: the
    // objects and their folders, the branch of its kind and the one that
    // branch stands in, the configuration.
    reveal(object) {
      let top = object;
      for (let owner = object.owner; owner; owner = top.owner) {
        expanded.add(owner); expanded.add(`${owner} ${top.kind}`);
        top = schema.objects.find(item => item.id === owner);
        if (!top) return;
      }
      expanded.add(branchKey(top.kind));
      if (kindPlaces?.[top.kind]) expanded.add(branchKey(kindPlaces[top.kind]));
      const root = schema.objects.find(item => item.root);
      if (root) expanded.add(root.id);
    },
  };
}

/* openRoleTree sets the tree of rights as it stands when its window opens,
   as the prototype's (snapshots configurator-2026-10-20/menyu-01, -02, опись
   rol.md): only the configuration open and chosen, «Общие» and the
   branches of the kinds closed. It forgets what was left open in expanded
   and answers the choice to make. */
function openRoleTree(schema, expanded) {
  expanded.clear();
  const root = schema.objects.find(item => item.root);
  if (!root) return null;
  expanded.add(root.id);
  return {object: root};
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
  const {marked} = subsystemFilterMarks(schema, filter);
  const held = new Set([...marked].flatMap(id => subsystems[id])), anywhere = new Set(Object.values(subsystems).flat());
  return id => {
    if (id in subsystems) return marked.has(id);
    let top = id;
    while (!(top in subsystems) && owners.get(top)) top = owners.get(top);
    return held.has(top) || (filter.chosen.has('') && !anywhere.has(top));
  };
}

/* The subsystems a filter takes in (marked): those chosen, those beneath a
   chosen one by children, those above it by parents - the last also apart
   (parents); above lists the subsystems a subsystem lies in, nearest first. */
function subsystemFilterMarks(schema, filter) {
  const subsystems = schema.subsystems || {}, owners = new Map(schema.objects.map(item => [item.id, item.owner]));
  const above = id => { const result = []; for (let up = owners.get(id); up in subsystems; up = owners.get(up)) result.push(up); return result; };
  const marked = new Set(), parents = new Set();
  for (const id of Object.keys(subsystems)) {
    if (!filter.chosen.has(id)) { if (filter.children && above(id).some(up => filter.chosen.has(up))) marked.add(id); continue; }
    marked.add(id);
    if (filter.parents) for (const up of above(id)) { marked.add(up); parents.add(up); }
  }
  return {marked, parents, above};
}

/* The subsystems the tree of the configuration shows under a filter, as
   rows of its branch of subsystems (inventory of block 00, «Отбор по
   подсистемам», snapshots derevo/22-26): each the filter takes in, and each
   above one of those, as the path to it, though what it holds is not shown;
   one taken in as the parent of a chosen one stands with every subsystem
   beneath it, what they hold not shown either. A chosen one without
   «подчиненных» shows alone, with nothing beneath it, as the prototype's
   does. Without a filter, null: every row shows. */
function subsystemFilterRows(schema, filter) {
  if (!filter) return null;
  const {marked, parents, above} = subsystemFilterMarks(schema, filter), rows = new Set(marked);
  for (const id of Object.keys(schema.subsystems || {})) {
    const path = above(id);
    if (marked.has(id)) for (const up of path) rows.add(up);
    if (path.some(up => parents.has(up))) rows.add(id);
  }
  return rows;
}

/* The name of the configuration of a role's schema: its root object. */
function configurationName(schema) { return schema.objects?.find(item => item.root)?.name || ''; }

/* The window of the filter by subsystems (help, «Отбор по подсистемам»;
   snapshots obolochka/32, derevo/21): the configuration, by its name, at the
   root, the subsystems by their names beneath it in the order of the
   configuration, a nested one under its parent, which folds, and
   «<Не входящие в подсистемы>» last at the top; the two flags under the
   tree, their boxes at the right; the buttons in a column to the right of
   it. A subsystem is marked by itself, without those beneath it: the help
   takes them in by the flag «Включать объекты подчиненных подсистем». A row
   not marked shows the third state when one above it is marked - every row
   under the marked root too - or one beneath it, whatever the flags
   (snapshots derevo/24, 27). A click is one rule for every row, the root
   too (configurator-2026-10-20/otbor-01…08): at a row not marked, empty or
   in the third state, it marks the row and takes away the marks above it
   and beneath it on its branch, the marks of other branches staying; at a
   row marked it takes its mark alone away. One window for every place that
   filters by subsystems; root is the name of the configuration. Without a
   filter in force, or with one taking in every row, it opens with the root
   marked and the subsystems beneath taken in, as the prototype's does
   (derevo/27). As a filter it is set with «Установить» or turned off;
   given apply - «Установить» or «Снять» of the role's rights by
   subsystems - it only chooses, and the place of «Отключить» stays empty
   (snapshot rol-snyat-po-podsistemam-okno). «Установить» with nothing
   marked is no refusal: no filter, as «Отключить», or nothing to set
   (otbor-09…12). Answers the filter chosen, null for «Отключить»,
   undefined when closed without either. */
async function chooseSubsystemFilter({schema, root, current, apply}) {
  const subsystems = schema.subsystems || {}, every = [...Object.keys(subsystems), ''], top = '*';
  // The rows marked each by itself, top for the root: marked, it stands
  // for every row and no row is marked beside it.
  const chosen = new Set(!current || every.every(id => current.chosen.has(id)) ? [top] : every.filter(id => current.chosen.has(id)));
  const flags = current ? {children: !!current.children, parents: !!current.parents} : {children: true, parents: false};
  const beneath = new Map(), parentOf = new Map([['', top]]), under = new Map([[top, every]]);
  for (const item of schema.objects) {
    if (!(item.id in subsystems)) continue;
    const parent = item.owner in subsystems ? item.owner : '';
    if (!beneath.has(parent)) beneath.set(parent, []);
    beneath.get(parent).push(item);
    parentOf.set(item.id, parent || top);
  }
  // The rows above id, nearest first, up to the root; a ring of parents is
  // walked once.
  const above = id => { const rows = []; for (let up = parentOf.get(id); up && !rows.includes(up); up = parentOf.get(up)) rows.push(up); return rows; };
  const tree = node('div', undefined, 'role-restriction-fields role-fields-tree role-subsystem-tree'), boxes = [];
  // The marks are put on the boxes in place: a tree drawn anew would take
  // the focus away from the box just pressed.
  function sync() {
    for (const [input, id] of boxes) { input.checked = chosen.has(id); input.indeterminate = !input.checked && [...above(id), ...under.get(id) || []].some(other => chosen.has(other)); }
  }
  function click(id, enabled) {
    if (enabled) { for (const other of [...above(id), ...under.get(id) || []]) chosen.delete(other); chosen.add(id); }
    else chosen.delete(id);
    sync();
  }
  function choice(host, id, caption, depth, toggle) {
    const box = checkboxField(caption, false, enabled => click(id, enabled), 'role-check');
    host.append(restrictionTreeRow(depth, toggle, box));
    boxes.push([box.querySelector('input'), id]);
  }
  // Answers every subsystem beneath parent, at any depth.
  function add(host, parent, depth) {
    const all = [];
    for (const item of beneath.get(parent) || []) {
      if (!beneath.has(item.id)) { choice(host, item.id, item.name, depth); all.push(item.id); continue; }
      const inside = node('div'), folding = restrictionFold(item.name, [inside]);
      choice(host, item.id, item.name, depth, folding.toggle);
      host.append(inside);
      const rows = add(inside, item.id, depth + 1);
      under.set(item.id, rows);
      folding.fold(true);
      all.push(item.id, ...rows);
    }
    return all;
  }
  choice(tree, top, root, 0, null);
  add(tree, '', 1);
  choice(tree, '', '<Не входящие в подсистемы>', 1);
  sync();
  const content = node('div', undefined, 'role-restriction-form');
  content.append(tree,
    checkboxField('Включать объекты подчиненных подсистем', flags.children, enabled => { flags.children = enabled; }, 'role-check role-subsystem-flag'),
    checkboxField('Включать объекты родительских подсистем', flags.parents, enabled => { flags.parents = enabled; }, 'role-check role-subsystem-flag'));
  const answer = await showDialog({title: 'Отбор по подсистемам', content, className: 'role-restriction-dialog role-subsystem-dialog', help: 'subsystem-filter', side: true,
    answers: apply ? [['ok', apply], null, [null, 'Отмена']] : [['ok', 'Установить'], ['off', 'Отключить'], [null, 'Отмена']]});
  if (answer === 'off' || answer === 'ok' && !chosen.size && !apply) return null;
  if (answer !== 'ok' || !chosen.size) return undefined;
  return {chosen: chosen.has(top) ? new Set(every) : chosen, ...flags};
}

/* The button of the filter by subsystems: it opens the window and stays
   pressed while holder.subsystems, the filter, is in force; applied() is
   called after the filter changed. schema() answers what the window and
   the filter read, as of the click, root() the name of the configuration.
   button is the one to make so, the tree of the configuration giving its
   own; a button of the words is made without it. */
function subsystemFilterButton({schema, root, holder, applied, button = node('button', 'По подсистемам', 'role-subsystems')}) {
  button.type = 'button'; button.classList.add('role-subsystems');
  const sync = () => { button.classList.toggle('selected', !!holder.subsystems); button.setAttribute('aria-pressed', String(!!holder.subsystems)); };
  button.addEventListener('click', async () => {
    const next = await chooseSubsystemFilter({schema: schema(), root: root(), current: holder.subsystems});
    if (next === undefined) return;
    holder.subsystems = next; sync(); applied();
  });
  sync();
  // Whoever drops the filter by itself shows it so.
  button.syncPressed = sync;
  return button;
}

/* A list of rows and the commands of the prototype's context menu:
   Добавить, Изменить, Удалить. A row is chosen by a click and opened by a
   double click. chosen is the index of the row chosen, or a Set of them
   where several are chosen at once, as in the prototype's «Все ограничения
   доступа»: Ctrl adds a row or takes it away, Shift chooses the rows from the
   last one clicked. A cell is [text, className], or a cell made already.
   A command is [key, text, act, enabled, menuText, keyText]; act gets
   {row, column}, the row and the column of the cell clicked last - the
   current row and column of the prototype's lists. With actions the bar
   stands over the list, as the prototype's command bar, and starts with
   «Действия», a menu of every command in its order under its menuText with
   its keyText, an unavailable one shown so; a null command is a line
   between groups of the menu, and Ins adds and F2 changes, as the menu
   says. Enter and a double click change, Del removes. A command whose
   seventh element holds is in the menu only, as «Настройка списка...» is;
   the eighth names its icon: with actions the bar shows each command by its
   icon alone, its menuText the tooltip, a null a line between groups, and
   the menu draws the icon before the text (snapshot vse-ogr-01).
   layout (listLayout, studio-list.js) lays the list out by its setting:
   the cells of a row are then given in the order of the list's columns,
   and the column of the cell clicked last is the key of its column. */
function roleListTable(headings, rows, chosen, choose, commands, empty, {actions = false, layout = null} = {}) {
  const table=node('table',undefined,'role-list'),head=node('tr');
  if(layout)head.append(...layout.heading());else for(const text of headings)head.append(node('th',text));
  const thead=node('thead');thead.append(head);const body=node('tbody');
  const many=chosen instanceof Set,isChosen=index=>many?chosen.has(index):index===chosen;
  let anchor=many?[...chosen].at(-1)??-1:chosen,column=layout?layout.first():0,last=anchor;
  const current=()=>({row:isChosen(last)?last:-1,column});
  const run=name=>{const command=commands.find(item=>item?.[0]===name);if(command&&command[3]())command[2](current());};
  function click(index,event){
    last=index;
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
    if(layout)row.append(...layout.row(cells));else for(const cell of cells){if(cell instanceof Node){row.append(cell);continue;}const [text,className]=cell,item=node('td',text,className);item.title=text;row.append(item);}
    row.addEventListener('click',event=>{if(layout){const cell=event.target.closest?.('[data-column]');if(cell)column=cell.dataset.column;}else{const cell=event.target.closest?.('td');if(cell)column=[...row.children].indexOf(cell);}click(index,event);});
    row.addEventListener('dblclick',()=>{if(!isChosen(index))click(index);run('edit');});
    row.addEventListener('keydown',event=>{if(event.key==='Enter'||(actions&&event.key==='F2')){event.preventDefault();row.dispatchEvent(new MouseEvent('dblclick'));}if(event.key==='Delete'){event.preventDefault();if(!isChosen(index))click(index);run('remove');}});
    body.append(row);
  });
  if(!rows.length){const row=node('tr'),cell=node('td',empty,'muted');cell.colSpan=headings.length;row.append(cell);body.append(row);}
  table.append(thead,body);
  layout?.apply(table);
  const bar=node('div',undefined,'role-bulk'),buttons=[];
  if(actions){
    bar.append(menuButton('Действия',()=>commands.map(command=>command&&[command[4]||command[1],()=>command[2](current()),command[3](),command[5],command[7]])));
  }
  // On the bar of actions a line stands only between two groups that both
  // show a button.
  let line=false;
  for(const command of commands){
    if(!command){line=actions&&buttons.length>0;continue;}
    const [key,text,action,enabled,menuText,,menuOnly,icon]=command;if(menuOnly)continue;
    if(line){bar.append(node('span',undefined,'role-bulk-separator'));line=false;}
    const button=actions&&icon?iconButton(icon,menuText||text):node('button',text);button.type='button';button.dataset.command=key;button.addEventListener('click',()=>action(current()));buttons.push([button,enabled]);bar.append(button);
  }
  function syncCommands(){for(const [button,enabled] of buttons)button.disabled=!enabled();}
  syncCommands();
  const block=node('div');
  if(actions){
    bar.classList.add('role-bulk-top');block.append(bar,table);
    table.tabIndex=0;
    table.addEventListener('keydown',event=>{if(event.key==='Insert'){event.preventDefault();run('add');}});
  }else block.append(table,bar);
  return block;
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

/* The fold of a row of the trees of the windows of restrictions - the
   fields of a table part, the objects of a kind: ▸ or ▾ before the row,
   hiding or showing the rows beneath it. name is what the button names;
   rows, the rows beneath, may be filled after. Answers {toggle, fold(open)}. */
function restrictionFold(name, rows) {
  const toggle = node('button', '▸', 'role-fields-toggle');
  toggle.type = 'button';
  const fold = open => {
    toggle.textContent = open ? '▾' : '▸';
    toggle.setAttribute('aria-expanded', String(open));
    toggle.setAttribute('aria-label', `${open ? 'Свернуть' : 'Развернуть'} ${name}`);
    for (const row of rows) row.hidden = !open;
  };
  toggle.addEventListener('click', () => fold(toggle.getAttribute('aria-expanded') !== 'true'));
  return {toggle, fold};
}

/* A row of those trees at depth: the fold of the row, or an empty place
   where the row does not fold, and its box. */
function restrictionTreeRow(depth, toggle, box) {
  const row = node('div', undefined, 'role-fields-row');
  row.style.setProperty('--depth', depth);
  row.append(toggle || node('span', undefined, 'role-fields-toggle'), box);
  return row;
}

/* The fields of a restriction of reading, as the prototype's windows show
   them (help, form_RLFieldsListDlg; checked in the configurator 14.10.2026):
   one column by names, a table part folding under its own box with its fields
   beneath, «Отметить все» and «Снять все» over the list and «Прочие поля»
   under it. Marking a field takes «Прочие поля» away, marking «Прочие поля»
   takes every field away, taking the last field away marks «Прочие поля».
   chosen holds the keys marked; mixed - the keys marked grey, as the group
   «Изменить» shows a field some of its restrictions name and others not
   (configurator 22.10.2026, ACCESS-RIGHTS «Групповое «Изменить» состава
   полей»): a click on a grey box takes it away, marking «Прочие поля» takes
   the grey ones away too, and a grey one marks no field of its own, so
   «Прочие поля» stands marked beside it; marked(key, enabled) is told of
   every field the developer marks or takes away. taken(key) - a field another restriction
   names, which cannot be marked. One choice serves the window of one
   restriction, the group «Изменить» and the wizard. Answers the block with
   fill(fields) - the fields, {key, name, parent, hint}, parent the key of
   the table part; filled() - whether a field or «Прочие поля» is marked;
   disable(flag). */
function restrictionFieldsChoice({chosen, mixed = new Set(), taken = () => false, marked = () => {}}) {
  let others = !chosen.size, disabled = false, fields = [];
  const boxes = new Map(), list = node('div', undefined, 'role-restriction-fields role-fields-tree');
  // «Прочие поля» take the marks of the fields, and the window is told of
  // each, as when a field is taken away: «ОК» of the group waits for a
  // field marked again (configurator 22.10.2026, gruppovoe-dve-stroki-03).
  const othersBox = checkboxField('Прочие поля', others, enabled => {
    others = enabled;
    if (enabled) {
      const taken = [...chosen];
      chosen.clear(); mixed.clear();
      for (const key of taken) marked(key, false);
    }
    sync();
  }, 'role-check role-fields-others');
  function mark(key, enabled) {
    if (mixed.delete(key)) enabled = false;
    if (enabled) chosen.add(key); else chosen.delete(key);
    others = !chosen.size;
    marked(key, enabled);
    sync();
  }
  // The marks are put on the boxes in place: a list drawn anew would take
  // the focus away from the box just pressed.
  function sync() {
    for (const [key, input] of boxes) {
      input.checked = chosen.has(key); input.indeterminate = mixed.has(key); input.disabled = disabled || taken(key);
    }
    // Where the object has no field to restrict by - a register of
    // accumulation, accounting or calculation - «Прочие поля» stand marked
    // and unavailable (configurator 22.10.2026, pola-45…47).
    const input = othersBox.querySelector('input');
    input.checked = others; input.disabled = disabled || !fields.length;
    for (const button of block.querySelectorAll('.role-mark-all')) button.disabled = disabled;
  }
  // A table part folds: its fields are under it, hidden until it is opened.
  function row(field, depth, under) {
    const children = fields.filter(item => item.parent === field.key), inside = [];
    const folding = children.length ? restrictionFold(field.name, inside) : null;
    const box = checkboxField(field.name, false, enabled => mark(field.key, enabled), 'role-check');
    if (field.hint && field.hint !== field.name) box.title = field.hint;
    boxes.set(field.key, box.querySelector('input'));
    const line = restrictionTreeRow(depth, folding?.toggle, box);
    line.hidden = !!under;
    list.append(line);
    for (const child of children) inside.push(row(child, depth + 1, line));
    folding?.fold(false);
    return line;
  }
  const block = node('div', undefined, 'role-restriction-form role-fields-choice');
  block.append(markedListCaption('Поля:', enabled => {
    mixed.clear();
    if (enabled) for (const field of fields) { if (!taken(field.key)) chosen.add(field.key); } else chosen.clear();
    for (const field of fields) if (!taken(field.key)) marked(field.key, enabled);
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
      for (const key of [...mixed]) if (!keys.has(key)) mixed.delete(key);
      if (before && !chosen.size) others = true;
      list.replaceChildren(); boxes.clear();
      for (const field of fields) if (!field.parent || !keys.has(field.parent)) row(field, 0);
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

/* A restriction without a condition is written by no window of
   restrictions - the role's, the wizard and the group «Изменить» of «Все
   ограничения доступа» -, refused with the prototype's text (snapshot
   okno-roli-pustoe-ogranichenie; ACCESS-RIGHTS, «Пустое ограничение в окне
   роли»): the prototype's wizard writes one, ML none, as the owner decided
   on 22.10.2026. Answers the refusal, '' for a condition given. */
const emptyRestrictionRefusal=condition=>condition.trim()?'':'Нельзя записывать пустое ограничение доступа.';

/* The caption of the restrictions of access to data in both windows of
   rights, the role's and «Все роли», as the prototype's (block-00/rol;
   snapshot configurator-2026-10-20/prava/19). */
const roleRestrictionsCaption='Ограничения доступа к данным:';

/* The restrictions of access to data a role puts on the right current in
   the list of rights, as the prototype lists them under it (help,
   «Ограничение доступа к данным»; block-00/rol): the fields - «Поля» - or
   «Прочие поля», and the condition. A right restricted by fields takes one per set of
   fields, the others one each; a right no restriction may be put on, or a
   part, shows the list empty with its commands unavailable. model is the
   role's (createRoleModel); object is null for a part; state keeps the
   current right and the row chosen across redraws; changed() is called
   after the role changed, to redraw. */
function roleRestrictionSection({model, object, caption, title, state, changed}) {
  const section=node('section',undefined,'role-restrictions');
  section.append(node('h4',roleRestrictionsCaption));
  const right=object?.restrictedRights.includes(state.right)?state.right:null,list=right?model.restrictions(object.id,right):[];
  if(state.row>=list.length)state.row=-1;
  const others=list.some(item=>!item.fields?.length);
  const canAdd=()=>!!right&&(model.restrictedByFields(right)?!others||object.restrictionFields.some(field=>!list.some(item=>item.fields?.includes(field.key))):!list.length);
  const open=async index=>{if(await editRoleRestriction({model,object,right,index,caption,title})){state.row=index;changed();}};
  section.append(roleListTable(['Поля','Ограничение доступа'],list.map(item=>[[restrictionFieldsCaption(object,item,title)],[item.condition||'','role-condition']]),state.row,index=>{state.row=index;},[
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
    fields.fill(object.roleWindowFields.map(field=>({key:field.key,name:field.name,parent:field.parent,hint:title(field)})));
  }
  let condition=current.condition||'';
  const editor=queryTextField(condition,text=>{condition=text;},'Ограничение доступа');
  const content=restrictionWindowContent({fields,editor,change:{fields:true,condition:true},group:false});
  // A field another restriction names cannot be marked, so the only
  // collision left is a second one on «Прочие поля».
  const answer=await showDialog({title:`Ограничение доступа · ${caption(right)}`,content,className:'role-restriction-dialog',help:'role-restriction',
    answers:[['ok','ОК',{enabled:()=>!fields||fields.filled()}],[null,'Отмена']],
    check:()=>emptyRestrictionRefusal(condition)||(model.restrictionClashes(object.id,right,index,{fields:[...chosen]}).length?'Ограничение на прочие поля у этого права уже есть: отметьте поля.':'')});
  if(answer!=='ok')return false;
  model.putRestriction(object.id,right,index,{fields:[...chosen],condition});
  return true;
}

function createRoleEditor(host, onChange) {
  let source, model, selected, panel, tree, warning, description, titleFieldCleanup = null;
  // The tab shown - the rights or the templates of restrictions - and the
  // right current in the list of rights, described under the tree and with
  // its restrictions listed, with the row chosen in each list.
  let view = 'rights', templateRow = -1, rightsBody, templatesBody, viewTabs;
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
  function title(item) { return localizedTitle(item, source.defaultLanguage); }
  /* Подписи объектов и полей показываются на текущем языке Studio и
     меняются вместе с ним. */
  StudioLanguage.onChange(() => { if (!source) return; tree.render(); renderPanel(); if (view === 'templates') renderTemplates(); });
  function changed() { warning.hidden=!model.hasUnavailable(); onChange(); }
  function checkbox(text, checked, action, disabled=false) {
    const label=checkboxField(text,checked,enabled=>{const focus=label.firstChild.getAttribute('aria-label')||text;action(enabled);changed();renderPanel();for(const candidate of panel.querySelectorAll('input'))if((candidate.getAttribute('aria-label')||candidate.parentElement.textContent)===focus){candidate.focus();break;}},'role-check',disabled);
    return label;
  }
  /* The description of the current right under the tree, as the prototype
     shows it: what the right allows, and its name in the language in
     brackets; empty while the node chosen has no rights. */
  function describe(right){
    const known=right&&source.schema.rights.find(item=>item.key===right);
    description.replaceChildren(node('span','Описание:','role-description-caption'));
    if(known)description.append(node('span',`${known.description}\n(${known.name})`,'role-description-text'));
  }
  /* What the node chosen gives rights over, as the prototype's window
     shows it (pass of 20.10.2026, block-00/rol): an object or a part over
     itself alone; a folder or a branch over what it holds, a right marked at
     every one, cleared at none, grey at some; the box of a folder or of a
     branch sets or clears the right at everything it holds, the grey one
     setting it (the branch: pass after the series of 2.413, prava/20-23).
     Under a filter by subsystems a folder or a branch holds what it shows
     alone, its summary and its boxes theirs (pass of 21.10.2026,
     prava/45-48). Only an object takes restrictions of access to data. */
  function chosenNode(){
    const {object,part,group}=selected;
    if(group){
      const shown=subsystemFilterPasses(source.schema,treeFilter.subsystems);
      const members=roleGroupMembers(source.schema,group).filter(member=>member.part||shown(member.object.id)),owner=group.object&&source.schema.objects.find(item=>item.id===group.object);
      return {members,rights:roleGroupRights(source.schema,members,group),heading:[node('h3',group.caption),...owner?[node('p',title(owner)||owner.name,'muted')]:[]]};
    }
    const member={object,part},rights=roleMemberRights(member);
    return {members:[member],rights,restricted:part?null:object,
      heading:part?[node('h3',title(part)||part.name),node('p',`${title(object)||object.name} · ${rolePartGroupTitle(source.schema,part.group)}`,'muted')]:[node('h3',title(object)||object.name)],
      empty:rights.length?null:part?'Своих прав у этого узла нет: права даются на то, что в нём.':'Своих прав у объекта нет: права даются на его части.'};
  }
  function renderPanel() {
    panel.replaceChildren();
    if(!selected){panel.append(node('p','Выберите объект слева и отметьте разрешённые действия.','muted'));describe(null);return;}
    const {members,rights,heading,restricted=null,empty}=chosenNode();
    // Over the list, as the prototype's: «Права:» and the two buttons that
    // mark or clear every right of the node chosen, and nothing beside it.
    const head=node('div',undefined,'role-rights-head');
    head.append(node('span','Права:','role-rights-caption'),...markAllButtons(enabled=>{for(const right of rights)model.setGroup(members,right,enabled);changed();renderPanel();panel.querySelector('.check-list')?.focus();}));
    for(const button of head.querySelectorAll('button'))button.disabled=!rights.length;
    // The current right stays current on the next node that has it, as the
    // prototype's does; elsewhere the first right is.
    if(!rights.includes(restriction.right)){restriction.right=rights[0]||null;restriction.row=-1;}
    const mark=(right,enabled)=>{model.setGroup(members,right,enabled);changed();renderPanel();panel.querySelector('.check-list')?.focus();};
    const choose=right=>{restriction.right=right;restriction.row=-1;renderPanel();panel.querySelector('.check-list')?.focus();};
    panel.append(...heading);
    if(empty)panel.append(node('p',empty,'muted'));
    else panel.append(head,checkList(rights.map(right=>{const state=model.groupMark(members,right);return{key:right,caption:caption(right),checked:state==='all',mixed:state==='some'};}),restriction.right,mark,choose));
    panel.append(roleRestrictionSection({model,object:restricted,caption,title,state:restriction,changed:()=>{changed();renderPanel();}}));
    describe(restriction.right);
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
    const filter=await chooseSubsystemFilter({schema:source.schema,root:configurationName(source.schema),current:null,apply:enabled?'Установить':'Снять'});
    if(!filter)return;
    const passes=subsystemFilterPasses(source.schema,filter);
    for(const item of source.schema.objects)if(!item.root&&passes(item.id))model.setAll(item.id,enabled);
    changed();renderPanel();
  }
  function showView(next){view=next;rightsBody.hidden=view!=='rights';templatesBody.hidden=view!=='templates';viewTabs.choose(view);if(view==='templates')renderTemplates();}
  return hostedEditor(host, ()=>{clearTitleField();source=null;model=null;}, {
    open(value, preserveSelection=false) {
      clearTitleField();
      const previous=preserveSelection?selected:null;source=structuredClone(value);model=createRoleModel(source);selected=preserveSelection?null:openRoleTree(source.schema,expanded);if(previous?.group)selected={group:previous.group};else if(previous?.object){const object=source.schema.objects.find(item=>item.id===previous.object.id);const part=previous.part&&object?.fields.find(item=>item.key===previous.part.key);if(object)selected=part?{object,part}:{object};}host.hidden=false;host.replaceChildren();
      const identity=node('div',undefined,'role-identity'),label=node('label','Имя роли'),name=node('input');name.value=source.role.name;name.maxLength=128;name.addEventListener('input',()=>{model.setName(name.value);changed();});label.append(name);identity.append(label);
      const titleLabel=node('label','Заголовок'),titleField=createLocalizedField(source,code=>source.role.title[code],(code,value)=>model.setTitle(code,value),changed,titleTextOptions);
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
      warning=node('div',undefined,'role-warning');warning.append(node('span','В роли есть права на удалённые или изменённые объекты или права, которых в ML нет. '));const repair=node('button','Убрать недоступные права');repair.type='button';repair.addEventListener('click',()=>{model.removeUnavailable();changed();renderPanel();});warning.append(repair);warning.hidden=!model.hasUnavailable();host.append(warning);
      // The prototype's role window has two tabs: the rights and the
      // templates of restrictions.
      viewTabs=tabStrip([['rights','Права'],['templates','Шаблоны ограничений']],view,showView);
      host.append(viewTabs);
      const body=node('div',undefined,'role-body'),sidebar=node('div',undefined,'role-sidebar'),search=node('input');search.type='search';search.placeholder='Поиск объекта';search.setAttribute('aria-label','Поиск объекта');tree=createRoleTree({source,title,expanded,selection:()=>selected,choose:next=>{selected=next;renderPanel();},subsystems:()=>treeFilter.subsystems,groups:true});search.addEventListener('input',()=>tree.render(search.value));
      if(!preserveSelection)treeFilter.subsystems=null;
      const bar=node('div',undefined,'role-tree-bar'),actions=menuButton('Действия',()=>[['Установить все права',()=>{model.setEverything(true);changed();renderPanel();}],['Снять все права',()=>{model.setEverything(false);changed();renderPanel();}],['Установить по подсистемам...',()=>rightsBySubsystems(true)],['Снять по подсистемам...',()=>rightsBySubsystems(false)]]);
      bar.append(actions,subsystemFilterButton({schema:()=>source.schema,root:()=>configurationName(source.schema),holder:treeFilter,applied:()=>{tree.render(search.value);renderPanel();}}));
      description=node('div',undefined,'role-description');sidebar.append(bar,search,tree.element,description);panel=node('section',undefined,'role-permissions');body.append(sidebar,panel);rightsBody=body;templatesBody=node('section',undefined,'role-templates');host.append(body,templatesBody);
      if(!preserveSelection){view='rights';restriction.right=null;restriction.row=-1;templateRow=-1;}
      tree.render();renderPanel();showView(view);
    },
    // Переход из окна «Все роли» происходит по конкретному объекту, а не по
    // роли вообще: открывать роль на пустом месте значило бы заставить искать
    // в дереве то, что только что было на экране.
    selectObject(id){const object=source?.schema.objects.find(item=>item.id===id);if(!object)return;selected={object};showView('rights');tree.reveal(object);tree.render();renderPanel();},
    value(){return model?.value();},
  });
}
