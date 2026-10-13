/* Role editor: explicit grants underneath, ordinary tree/checkboxes in Studio. */
function createRoleModel(source) {
  const value = structuredClone(source);
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
  // A restriction is written inside the right it restricts, granted or
  // denied, as the prototype writes it: a right that stood for the role's
  // flags is written first as it stood, so adding a restriction turns no
  // right over. Only reading is restricted by fields; a field the object
  // does not have is not taken.
  function putRestriction(id, right, index, restriction) {
    if (!definitions.get(id)?.restrictedRights?.includes(right)) return;
    const grant = object(id);
    if (!grant.operations.includes(right) && !grant.denied?.includes(right)) writeRight(grant, right, objectRight(id, right));
    const entry = {right}, own = restrictionFields.get(id) || new Set();
    const chosen = right === 'read' ? (restriction.fields || []).filter(field => own.has(field)) : [];
    if (chosen.length) entry.fields = [...new Set(chosen)];
    if (restriction.condition) entry.condition = restriction.condition;
    const list = grant.restrictions ||= [], same = restrictionsOf(id, right);
    if (index < same.length) list[list.indexOf(same[index])] = entry; else list.push(entry);
  }
  function removeRestriction(id, right, index) {
    const grant = objects.get(id), target = restrictionsOf(id, right)[index];
    if (!target) return;
    grant.restrictions = grant.restrictions.filter(item => item !== target);
    if (!grant.restrictions.length) delete grant.restrictions;
  }
  // A template of the role's restrictions: its name, with the names of its
  // parameters in brackets where it takes any, and its text.
  function putTemplate(index, template) {
    const list = value.role.restrictionTemplates ||= [], entry = {name: template.name.trim()};
    if (template.condition) entry.condition = template.condition;
    if (index < list.length) list[index] = entry; else list.push(entry);
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
    hasCommand(form, command) { return value.role.commands.some(item => item.form === form && item.command === command); },
    setCommand(form, command, enabled) {
      if (!value.schema.forms.some(item => item.id === form && item.commands.some(item => item.id === command))) return;
      value.role.commands = value.role.commands.filter(item => item.form !== form || item.command !== command);
      if(enabled)value.role.commands.push({form,command});
    },
    restrictions: (id, right) => structuredClone(restrictionsOf(id, right)),
    putRestriction, removeRestriction,
    templates: () => structuredClone(value.role.restrictionTemplates || []),
    putTemplate, removeTemplate,
    hasUnavailable() { return JSON.stringify(clean()) !== JSON.stringify(repaired()); },
    removeUnavailable() { value.role = repaired(); reindex(); },
  };
}

function createRoleEditor(host, onChange) {
  let source, model, selected, panel, tree, warning, titleFieldCleanup = null;
  // The tab shown - the rights or the templates of restrictions - and the
  // right whose restrictions are listed, with the row chosen in each list.
  let view = 'rights', restrictionRight = null, restrictionRow = -1, templateRow = -1, rightsBody, templatesBody;
  // The nodes of the tree left open, by key, so a redraw keeps them open.
  const expanded = new Set();
  function clearTitleField() { titleFieldCleanup?.(); titleFieldCleanup = null; }
  /* Подписи прав, видов и стандартных полей приходят с сервера, из
     единственных перечней: свой словарь здесь разошёлся бы с ними при первой
     правке одного из двух. */
  const caption = key => source.schema.rights.find(item => item.key === key)?.title || key;
  const kindTitle = kind => source.kindTitles?.[kind] || kind;
  function title(item) { return localizedTitle(item, source.defaultLanguage, source.languages); }
  /* Подписи объектов и полей показываются на текущем языке Studio и
     меняются вместе с ним. */
  StudioLanguage.onChange(() => { if (!source) return; renderTree(currentFilter()); renderPanel(); if (view === 'templates') renderTemplates(); });
  function changed() { warning.hidden=!model.hasUnavailable(); onChange(); }
  function checkbox(text, checked, action, disabled=false) {
    const label=checkboxField(text,checked,enabled=>{const focus=label.firstChild.getAttribute('aria-label')||text;action(enabled);changed();renderPanel();for(const candidate of panel.querySelectorAll('input'))if((candidate.getAttribute('aria-label')||candidate.parentElement.textContent)===focus){candidate.focus();break;}},'role-check',disabled);
    return label;
  }
  // The tree is the configurator's: the configuration itself at the top, then
  // the objects kind by kind in the order the server lists them, each with
  // its parts in folders and the objects inside it under their kind - a
  // recalculation under its register, a table under its source.
  const partGroupTitle = group => source.schema.partGroups?.find(item => item.key === group)?.title || group;
  function treeButton(text, hint, className, isSelected, choose) {
    const button = node('button', text, className + (isSelected ? ' selected' : ''));
    button.type = 'button'; if (hint) button.title = hint;
    button.addEventListener('click', event => { event.preventDefault(); choose(); renderTree(currentFilter()); renderPanel(); });
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
  const currentFilter = () => host.querySelector('.role-sidebar input')?.value || '';
  const matches = (item, filter) => !filter || `${item.name} ${title(item)}`.toLocaleLowerCase().includes(filter);
  // The nodes of one object: its parts in folders, then the objects inside it.
  // Null when neither the object nor anything in it matches the filter.
  function objectNode(item, inside, filter) {
    const isSelected = selected?.object?.id === item.id;
    const choose = part => () => { selected = part ? {object: item, part} : {object: item}; };
    const own = matches(item, filter), parts = new Map();
    for (const part of item.fields) {
      if (!parts.has(part.parent || '')) parts.set(part.parent || '', []);
      parts.get(part.parent || '').push(part);
    }
    const partNode = (part, groupKey) => {
      const nested = (parts.get(part.key) || []).filter(child => own || matches(part, filter) || matches(child, filter));
      const chosen = selected?.object?.id === item.id && selected?.part?.key === part.key;
      const caption = treeButton(title(part) || part.name, part.name, 'role-tree-item', chosen, choose(part));
      if (!(parts.get(part.key) || []).length) return caption;
      const children = nested.map(child => treeButton(title(child) || child.name, child.name, 'role-tree-item', selected?.object?.id === item.id && selected?.part?.key === child.key, choose(child)));
      return branch(`${item.id} ${groupKey} ${part.key}`, caption, children, !!filter);
    };
    const folders = [];
    for (const group of source.schema.partGroups || []) {
      const list = (parts.get('') || []).filter(part => part.group === group.key)
        .filter(part => own || matches(part, filter) || (parts.get(part.key) || []).some(child => matches(child, filter)));
      if (list.length) folders.push(folder(`${item.id} ${group.key}`, partGroupTitle(group.key), list.map(part => partNode(part, group.key)), !!filter));
    }
    for (const [kind, list] of inside.get(item.id) || []) {
      // What lies inside an object that matches is shown whole, as its parts are.
      const nodes = list.map(child => objectNode(child, inside, own ? '' : filter)).filter(Boolean);
      if (nodes.length) folders.push(folder(`${item.id} ${kind}`, kindTitle(kind), nodes, !!filter));
    }
    if (!own && !folders.length) return null;
    const caption = treeButton(title(item) || item.name, item.name, 'role-tree-item', isSelected && !selected.part, choose(null));
    caption.dataset.object = item.id;
    if (!folders.length) return caption;
    return branch(item.id, caption, folders, !!filter && !own);
  }
  function renderTree(filter = '') {
    filter = filter.toLocaleLowerCase().trim();
    tree.replaceChildren(); const groups = new Map(), inside = new Map();
    for (const item of source.schema.objects) {
      if (!item.owner) continue;
      if (!inside.has(item.owner)) inside.set(item.owner, new Map());
      const byKind = inside.get(item.owner);
      if (!byKind.has(item.kind)) byKind.set(item.kind, []);
      byKind.get(item.kind).push(item);
    }
    for (const item of source.schema.objects) {
      if (item.root) {
        if (matches(item, filter)) { const button = treeButton(title(item) || item.name, item.name, 'role-tree-item role-tree-root', selected?.object?.id === item.id, () => { selected = {object: item}; }); button.dataset.object = item.id; tree.append(button); }
        continue;
      }
      if (item.owner) continue;
      const entry = objectNode(item, inside, filter);
      if (!entry) continue;
      let group = groups.get(item.kind);
      if (!group) { group = node('details'); group.open = true; group.append(node('summary', kindTitle(item.kind))); groups.set(item.kind, group); tree.append(group); }
      group.append(entry);
    }
    const forms = source.schema.forms.filter(item => matches(item, filter));
    if (forms.length) { const group = node('details'); group.open = true; group.append(node('summary', 'Команды форм')); for (const form of forms) group.append(treeButton(title(form), form.name, 'role-tree-item', selected?.form?.id === form.id, () => { selected = {form}; })); tree.append(group); }
    if (!tree.childElementCount) tree.append(node('p', 'Нет подходящих объектов', 'muted'));
  }
  function renderPanel() {
    panel.replaceChildren();
    if(!selected){panel.append(node('p','Выберите объект слева и отметьте разрешённые действия.','muted'));return;}
    if(selected.form){const form=selected.form;panel.append(node('h3',title(form)),node('p','Доступ к команде не заменяет права на используемые данные.','muted'));for(const command of form.commands)panel.append(checkbox(title(command),model.hasCommand(form.id,command.id),enabled=>model.setCommand(form.id,command.id,enabled)));return;}
    const object=selected.object;
    const bulk=(set)=>{const all=node('div',undefined,'role-bulk');for(const [text,enabled] of [['Разрешить всё',true],['Снять все права',false]]){const button=node('button',text);button.type='button';button.addEventListener('click',()=>{set(enabled);changed();renderPanel();});all.append(button);}return all;};
    if(selected.part){
      const part=selected.part;
      panel.append(node('h3',title(part)||part.name),node('p',`${title(object)||object.name} · ${partGroupTitle(part.group)}`,'muted'));
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
    if(object.restrictedRights?.length)panel.append(restrictionSection(object));
  }
  // A list of rows with two columns and the commands of the prototype's
  // context menu: Добавить, Изменить, Удалить. A row is chosen by a click and
  // opened by a double click.
  function listTable(headings, rows, chosen, choose, commands, empty) {
    const table=node('table',undefined,'role-list'),head=node('tr');
    for(const text of headings)head.append(node('th',text));
    const thead=node('thead');thead.append(head);const body=node('tbody');
    rows.forEach((cells,index)=>{
      const row=node('tr',undefined,index===chosen?'selected':undefined);row.tabIndex=0;
      for(const [text,className] of cells){const cell=node('td',text,className);cell.title=text;row.append(cell);}
      row.addEventListener('click',()=>{choose(index);for(const other of body.children)other.classList.toggle('selected',other===row);syncCommands();});
      row.addEventListener('dblclick',()=>{choose(index);commands.find(([key])=>key==='edit')?.[2]();});
      row.addEventListener('keydown',event=>{if(event.key==='Enter'){event.preventDefault();row.dispatchEvent(new MouseEvent('dblclick'));}if(event.key==='Delete'){event.preventDefault();choose(index);commands.find(([key])=>key==='remove')?.[2]();}});
      body.append(row);
    });
    if(!rows.length){const row=node('tr'),cell=node('td',empty,'muted');cell.colSpan=headings.length;row.append(cell);body.append(row);}
    table.append(thead,body);
    const bar=node('div',undefined,'role-bulk'),buttons=[];
    for(const [key,text,action,enabled] of commands){const button=node('button',text);button.type='button';button.addEventListener('click',action);buttons.push([button,enabled]);bar.append(button);}
    function syncCommands(){for(const [button,enabled] of buttons)button.disabled=!enabled();}
    syncCommands();
    const block=node('div');block.append(table,bar);return block;
  }
  // The restrictions of access to data on a right of the object, as the
  // prototype lists them under the rights (help, «Ограничение доступа к
  // данным»): the field or «Прочие поля», and the condition. Reading takes
  // one per set of fields, the other rights one each.
  function restrictionSection(object) {
    const section=node('section',undefined,'role-restrictions');
    section.append(node('h4','Ограничение доступа к данным'));
    if(!object.restrictedRights.includes(restrictionRight)){restrictionRight=object.restrictedRights[0];restrictionRow=-1;}
    const tabs=node('div',undefined,'role-restriction-rights');
    for(const right of object.restrictedRights){
      const count=model.restrictions(object.id,right).length,button=node('button',count?`${caption(right)} (${count})`:caption(right),right===restrictionRight?'selected':undefined);
      button.type='button';button.setAttribute('aria-pressed',String(right===restrictionRight));
      button.addEventListener('click',()=>{restrictionRight=right;restrictionRow=-1;renderPanel();});
      tabs.append(button);
    }
    section.append(tabs);
    const right=restrictionRight,list=model.restrictions(object.id,right);
    if(restrictionRow>=list.length)restrictionRow=-1;
    const others=list.some(item=>!item.fields?.length);
    const canAdd=()=>right==='read'?!others||object.restrictionFields.some(field=>!list.some(item=>item.fields?.includes(field.key))):!list.length;
    const open=index=>editRestriction(object,right,index);
    section.append(listTable(['Поле','Ограничение доступа'],list.map(item=>[[restrictionFieldsCaption(object,item,title)],[item.condition||'','role-condition']]),restrictionRow,index=>{restrictionRow=index;},[
      ['add','Добавить',()=>open(list.length),canAdd],
      ['edit','Изменить',()=>{if(restrictionRow>=0)open(restrictionRow);},()=>restrictionRow>=0],
      ['remove','Удалить',()=>{if(restrictionRow<0)return;model.removeRestriction(object.id,right,restrictionRow);restrictionRow=-1;changed();renderPanel();},()=>restrictionRow>=0],
    ],'Ограничений нет'));
    return section;
  }
  // The window of one restriction: the fields it checks, for reading, and
  // its condition in the query language (help, «Ограничение доступа»).
  async function editRestriction(object,right,index){
    const list=model.restrictions(object.id,right),current=list[index]||{right},content=node('div',undefined,'role-restriction-form');
    const chosen=new Set(current.fields||[]);
    if(right==='read'){
      const taken=new Set(list.flatMap((item,other)=>other===index?[]:item.fields||[]));
      const fieldsLabel=node('div','Поля — ни одно не отмечено: «Прочие поля»','role-restriction-caption'),fields=node('div',undefined,'role-restriction-fields');
      for(const field of object.restrictionFields){
        const box=checkboxField(field.parent?(title(field)||field.name):(title(field)||field.name),chosen.has(field.key),enabled=>{if(enabled)chosen.add(field.key);else chosen.delete(field.key);},'role-check',taken.has(field.key));
        if(field.parent)box.classList.add('role-restriction-child');
        box.title=field.name;fields.append(box);
      }
      content.append(fieldsLabel,fields);
    }
    let condition=current.condition||'';
    content.append(node('div','Ограничение доступа','role-restriction-caption'),queryTextField(condition,text=>{condition=text;},'Ограничение доступа'));
    const othersTaken=list.some((item,other)=>other!==index&&!item.fields?.length);
    const answer=await showDialog({title:`Ограничение доступа · ${caption(right)}`,content,className:'role-restriction-dialog',answers:[['ok','ОК'],[null,'Отмена']],
      check:()=>right==='read'&&!chosen.size&&othersTaken?'Ограничение на прочие поля у этого права уже есть: отметьте поля.':''});
    if(answer!=='ok')return;
    model.putRestriction(object.id,right,index,{fields:[...chosen],condition});
    restrictionRow=index;changed();renderPanel();
  }
  // The templates of the role's restrictions (help, «Редактирование роли»,
  // «Шаблоны ограничений»): a name, with its parameters in brackets, and a
  // text the condition of a restriction takes in place of #Имя(...).
  function renderTemplates(){
    templatesBody.replaceChildren();
    const list=model.templates();
    if(templateRow>=list.length)templateRow=-1;
    templatesBody.append(node('p','Имя шаблона — имя по правилам языка, в скобках можно перечислить параметры: Шаблон(Поле, Пользователь). В тексте ограничения шаблон вызывается как #Шаблон("значение", ...).','muted'));
    templatesBody.append(listTable(['Имя','Текст шаблона'],list.map(item=>[[item.name],[item.condition||'','role-condition']]),templateRow,index=>{templateRow=index;},[
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
      const body=node('div',undefined,'role-body'),sidebar=node('div',undefined,'role-sidebar'),search=node('input');search.type='search';search.placeholder='Поиск объекта';search.setAttribute('aria-label','Поиск объекта');search.addEventListener('input',()=>renderTree(search.value));tree=node('div',undefined,'role-tree');sidebar.append(search,tree);panel=node('section',undefined,'role-permissions');body.append(sidebar,panel);rightsBody=body;templatesBody=node('section',undefined,'role-templates');host.append(body,templatesBody);
      if(!preserveSelection){view='rights';restrictionRight=null;restrictionRow=-1;templateRow=-1;}
      renderTree();renderPanel();showView(view);
    },
    // Переход из окна «Все роли» происходит по конкретному объекту, а не по
    // роли вообще: открывать роль на пустом месте значило бы заставить искать
    // в дереве то, что только что было на экране.
    selectObject(id){const object=source?.schema.objects.find(item=>item.id===id);if(!object)return;selected={object};showView('rights');for(let owner=object.owner;owner;owner=source.schema.objects.find(item=>item.id===owner)?.owner)expanded.add(owner);renderTree();renderPanel();},
    value(){return model?.value();},
    close(){clearTitleField();source=null;model=null;host.hidden=true;host.replaceChildren();},
    setDisabled(disabled){host.inert=disabled;},
  };
}
