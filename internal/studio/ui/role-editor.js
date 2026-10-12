/* Role editor: explicit grants underneath, ordinary tree/checkboxes in Studio. */
function createRoleModel(source) {
  const value = structuredClone(source);
  value.role.objects ||= [];
  value.role.commands ||= [];
  value.role.policyTemplates ||= [];
  const definitions = new Map(value.schema.objects.map(item => [item.id, item]));
  const fields = new Map(value.schema.objects.map(item => [item.id, new Map(item.fields.map(field => [field.key, field]))]));
  // A restriction compares fields under its own keys, not the parts rights
  // are given on - see PolicyFields on the server.
  const policyFields = new Map(value.schema.objects.map(item => [item.id, new Map((item.policyFields||[]).map(field => [field.key, field]))]));
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
  function clean() {
    const result = structuredClone(value.role);
    for (const item of result.objects) {
      item.operations ||= []; item.fields = (item.fields||[]).filter(field=>field.operations?.length || field.denied?.length);
      item.policies = (item.policies||[]).filter(policy=>policy.operations?.length);
      if(!item.policies.length)delete item.policies;
    }
    result.objects = result.objects.filter(item => item.operations.length || item.denied?.length || item.fields.length || item.policies);
    if(!result.policyTemplates?.length)delete result.policyTemplates;
    return result;
  }
  // A policy is only meaningful while the rule it resolves to still points at a
  // field of that same object, so repairing drops the rule, not just the object.
  function resolveRule(role, policy) {
    if(policy.rule)return policy.rule;
    return (role.policyTemplates||[]).find(template=>template.name===policy.template)?.rule;
  }
  // A restriction survives repair only if everything it names still exists: the
  // field it restricts, the operations it narrows, the template it invokes with
  // the right number of fields, and the object a subquery reads.
  function policyStillApplies(role, objectID, definition, policy) {
    const own = policyFields.get(objectID);
    if (!policy.operations.every(operation => definition.operations.includes(operation))) return false;
    const template = policy.template ? (role.policyTemplates||[]).find(item => item.name === policy.template) : null;
    if (policy.template && !template) return false;
    if (template) {
      const parameters = template.parameters || [], supplied = policy.arguments || [];
      if (parameters.length !== supplied.length) return false;
      if (!supplied.every(field => own.has(field))) return false;
    }
    const rule = template ? template.rule : policy.rule;
    if (!rule) return false;
    const resolveField = field => {
      if (!field.startsWith('$')) return field;
      const at = (template?.parameters||[]).indexOf(field.slice(1));
      return at < 0 ? null : (policy.arguments||[])[at];
    };
    const field = resolveField(rule.field);
    if (!field || !own.has(field)) return false;
    if (!rule.subquery) return true;
    const source = policyFields.get(rule.subquery.object);
    if (!source || !source.has(rule.subquery.field)) return false;
    return (rule.subquery.where||[]).every(condition => {
      const conditionField = resolveField(condition.field);
      return conditionField && source.has(conditionField);
    });
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
      const definition = definitions.get(item.object);
      if (item.policies) {
        item.policies = definition ? item.policies.filter(policy => policyStillApplies(result, item.object, definition, policy)) : [];
        if(!item.policies.length)delete item.policies;
      }
    }
    result.objects = result.objects.filter(item => item.operations.length || item.denied?.length || item.fields.length || item.policies);
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
    templates() { return structuredClone(value.role.policyTemplates); },
    addTemplate(name, parameters, rule) {
      if(!name || value.role.policyTemplates.some(item=>item.name===name))return false;
      const template={name, rule}; if(parameters?.length)template.parameters=parameters;
      value.role.policyTemplates.push(template); return true;
    },
    setTemplateParameters(name, parameters) {
      const item=value.role.policyTemplates.find(entry=>entry.name===name); if(!item)return;
      if(parameters.length)item.parameters=parameters; else delete item.parameters;
    },
    setTemplateRule(name, rule) { const item=value.role.policyTemplates.find(entry=>entry.name===name); if(item)item.rule=rule; },
    // Removing a template also removes the restrictions that referenced it:
    // leaving them behind would produce a role that cannot be published.
    removeTemplate(name) {
      value.role.policyTemplates = value.role.policyTemplates.filter(item=>item.name!==name);
      for(const item of value.role.objects)if(item.policies)item.policies=item.policies.filter(policy=>policy.template!==name);
    },
    policies(id) { return structuredClone(objects.get(id)?.policies || []); },
    addPolicy(id, policy) { const item=object(id); item.policies ||= []; item.policies.push(policy); },
    setPolicy(id, index, policy) { const item=objects.get(id); if(item?.policies?.[index])item.policies[index]=policy; },
    removePolicy(id, index) { const item=objects.get(id); if(item?.policies)item.policies.splice(index,1); },
    hasUnavailable() { return JSON.stringify(clean()) !== JSON.stringify(repaired()); },
    removeUnavailable() { value.role = repaired(); value.role.policyTemplates ||= []; reindex(); },
  };
}

function createRoleEditor(host, onChange) {
  let source, model, selected, panel, tree, warning, templates, policiesExpanded = false, titleFieldCleanup = null;
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
  StudioLanguage.onChange(() => { if (!source) return; renderTree(currentFilter()); renderTemplates(); renderPanel(); });
  function changed() { warning.hidden=!model.hasUnavailable(); onChange(); }
  function checkbox(text, checked, action, disabled=false) {
    const label=checkboxField(text,checked,enabled=>{const focus=label.firstChild.getAttribute('aria-label')||text;action(enabled);changed();renderPanel();for(const candidate of panel.querySelectorAll('input'))if((candidate.getAttribute('aria-label')||candidate.parentElement.textContent)===focus){candidate.focus();break;}},'role-check',disabled);
    return label;
  }
  const operators = {eq:'Равно',ne:'Не равно',in:'В списке','not-in':'Не в списке'};
  function select(options, current, action, label) {
    return selectInput(options,current,value=>{action(value);changed();renderPanel();},label);
  }
  // Literal operands are entered as one line: a single value for Равно/Не равно,
  // a comma-separated list for В списке. Session parameters are picked from the
  // project's own list instead, so a typo cannot reach publication.
  function operandText(rule) { return (rule.values||[]).map(item=>item.data).join(', '); }
  function parseOperand(text, operator) {
    const parts=text.split(',').map(item=>item.trim()).filter(Boolean);
    const values=(operator==='in'||operator==='not-in')?parts:parts.slice(0,1);
    return values.map(data=>({kind:'string',data}));
  }
  // A rule compares a field with literal values, a session parameter, or the set
  // of values found in another table. The third form is the one real
  // restrictions use, and it is why nested subqueries are refused: one level
  // expresses the pattern and keeps the cost of a policy predictable.
  function ruleEditor(rule, fieldOptions, apply, allowSubquery=true) {
    const row=node('div',undefined,'role-rule');
    row.append(select(fieldOptions,rule.field,field=>apply({...rule,field}),'Поле ограничения'));
    const listOnly=!!rule.subquery;
    const operatorChoices=listOnly?Object.entries(operators).filter(([key])=>key==='in'||key==='not-in'):Object.entries(operators);
    row.append(select(operatorChoices,rule.operator,operator=>apply({...rule,operator,
      ...(rule.parameter||rule.subquery||rule.currentMlUser?{}:{values:parseOperand(operandText(rule),operator)})}),'Оператор'));
    const names=source.schema.sessionParameters||[];
    // The ML user is an operand of its own, not a session parameter: session
    // parameters are the configuration's, and ТекущийПользователь among them is
    // whatever the configuration's session module puts there.
    const kind=rule.subquery?'subquery':(rule.parameter?'parameter':(rule.currentMlUser?'mluser':'values'));
    const kinds=[['values','Значение'],...(names.length?[['parameter','Параметр сеанса']]:[]),['mluser','Пользователь ML'],
      ...(allowSubquery&&queryable().length?[['subquery','Подзапрос']]:[])];
    row.append(select(kinds,kind,chosen=>{
      if(chosen==='parameter')apply({field:rule.field,operator:rule.operator,parameter:names[0]});
      else if(chosen==='mluser')apply({field:rule.field,operator:rule.operator,currentMlUser:true});
      else if(chosen==='subquery'){
        const object=queryable()[0];
        apply({field:rule.field,operator:'in',subquery:{object:object.id,field:object.fields[0]?.key||'ref',where:[]}});
      } else apply({...blankRule(rule.field),operator:rule.operator});
    },'Вид операнда'));
    if(kind==='parameter'){
      row.append(select(names.map(name=>[name,name]),rule.parameter,parameter=>apply({...rule,parameter}),'Параметр сеанса'));
      return row;
    }
    if(kind==='mluser')return row;
    if(kind==='subquery'){
      const block=node('div',undefined,'role-subquery');block.append(row);
      const chosen=queryable().find(item=>item.id===rule.subquery.object)||queryable()[0];
      const line=node('div',undefined,'role-rule');
      line.append(node('span','из','muted'));
      line.append(select(queryable().map(item=>[item.id,title(item)]),rule.subquery.object,id=>{
        const next=queryable().find(item=>item.id===id);
        apply({...rule,subquery:{object:id,field:next.fields[0]?.key||'ref',where:[]}});
      },'Объект подзапроса'));
      line.append(node('span','по полю','muted'));
      line.append(select(objectFieldOptions(chosen),rule.subquery.field,
        field=>apply({...rule,subquery:{...rule.subquery,field}}),'Поле подзапроса'));
      block.append(line);
      (rule.subquery.where||[]).forEach((condition,index)=>{
        const nested=ruleEditor(condition,objectFieldOptions(chosen),next=>{
          const where=[...rule.subquery.where];where[index]=next;
          apply({...rule,subquery:{...rule.subquery,where}});
        },false);
        const remove=node('button','Убрать условие');remove.type='button';
        remove.addEventListener('click',()=>{
          const where=rule.subquery.where.filter((_,at)=>at!==index);
          apply({...rule,subquery:{...rule.subquery,where}});changed();renderPanel();renderTemplates();
        });
        nested.append(remove);block.append(nested);
      });
      const add=node('button','Добавить условие подзапроса');add.type='button';
      add.addEventListener('click',()=>{
        const where=[...(rule.subquery.where||[]),blankRule(chosen.fields[0]?.key||'ref')];
        apply({...rule,subquery:{...rule.subquery,where}});changed();renderPanel();renderTemplates();
      });
      block.append(add);
      return block;
    }
    const operand=node('input');operand.value=operandText(rule);operand.placeholder=(rule.operator==='in'||rule.operator==='not-in')?'Значения через запятую':'Значение';
    operand.setAttribute('aria-label','Значение ограничения');
    operand.addEventListener('change',()=>{apply({...rule,values:parseOperand(operand.value,rule.operator)});changed();renderPanel();});
    row.append(operand);
    return row;
  }
  // A new rule starts with one empty value rather than none: a rule with no
  // operand at all is refused on save, and the editor must not be able to build
  // something the project cannot store.
  function blankRule(field) { return {field, operator:'eq', values:[{kind:'string',data:''}]}; }
  // A subquery reads a table: an object with fields a restriction compares.
  function queryable() { return source.schema.objects.filter(item=>item.policyFields?.length); }
  function objectFieldOptions(object) { return (object.policyFields||[]).map(field=>[field.key,(field.parent?'↳ ':'')+title(field)]); }
  function allFieldOptions() {
    const result=new Map();
    for(const object of source.schema.objects)for(const field of object.policyFields||[])if(!result.has(field.key))result.set(field.key,title(field));
    return [...result];
  }
  function renderTemplates() {
    templates.replaceChildren();
    templates.append(node('summary','Шаблоны политик'));
    templates.append(node('p','Правило, названное один раз и переиспользуемое в нескольких ограничениях этой роли.','muted'));
    const options=allFieldOptions();
    if(!options.length){templates.append(node('p','Нет объектов с реквизитами.','muted'));return;}
    for(const template of model.templates()) {
      const block=node('div',undefined,'role-template');
      const head=node('div',undefined,'role-template-head');head.append(node('strong',template.name));
      const remove=node('button','Удалить');remove.type='button';remove.setAttribute('aria-label',`Удалить шаблон ${template.name}`);
      remove.addEventListener('click',()=>{model.removeTemplate(template.name);changed();renderPanel();renderTemplates();});
      head.append(remove);block.append(head);
      const parameterLine=node('div',undefined,'role-rule'),parameters=node('input');
      parameters.value=(template.parameters||[]).join(', ');parameters.placeholder='Параметры через запятую';
      parameters.setAttribute('aria-label',`Параметры шаблона ${template.name}`);
      parameters.addEventListener('change',()=>{
        model.setTemplateParameters(template.name,parameters.value.split(',').map(item=>item.trim()).filter(Boolean));
        changed();renderTemplates();renderPanel();
      });
      parameterLine.append(node('span','Параметры','muted'),parameters);block.append(parameterLine);
      // A parameter is used by naming it where a field would go, so it belongs
      // in the same list the field select offers.
      const templateFields=[...(template.parameters||[]).map(name=>['$'+name,'$'+name]),...options];
      block.append(ruleEditor(template.rule,templateFields,rule=>{model.setTemplateRule(template.name,rule);renderTemplates();}));
      templates.append(block);
    }
    const add=node('div',undefined,'role-template-add'),name=node('input');
    name.placeholder='Имя шаблона';name.maxLength=128;name.setAttribute('aria-label','Имя нового шаблона политики');
    const button=node('button','Добавить шаблон');button.type='button';
    button.addEventListener('click',()=>{
      if(!model.addTemplate(name.value.trim(),[],blankRule(options[0][0])))return;
      name.value='';changed();renderTemplates();renderPanel();
    });
    add.append(name,button);templates.append(add);
  }
  function renderPolicies(object) {
    const block=node('details',undefined,'role-policies');block.open=policiesExpanded;
    block.addEventListener('toggle',()=>policiesExpanded=block.open);
    block.append(node('summary','Ограничения доступа к строкам'));
    block.append(node('p','Ограничение сужает уже выданные права: строки, не прошедшие правило, не видны и не изменяются. Ограничения разных ролей объединяются по ИЛИ.','muted'));
    const options=objectFieldOptions(object);
    if(!options.length){block.append(node('p','У объекта нет реквизитов, по которым можно ограничить строки.','muted'));return block;}
    const declared=model.templates(),names=declared.map(item=>item.name);
    const arity=name=>(declared.find(item=>item.name===name)?.parameters||[]).length;
    const list=model.policies(object.id);
    list.forEach((policy,index)=>{
      const row=node('div',undefined,'role-policy');
      const head=node('div',undefined,'role-policy-head');
      head.append(select(object.operations.map(operation=>[operation,caption(operation)]),policy.operations[0],
        operation=>model.setPolicy(object.id,index,{...policy,operations:[operation]}),'Право, которое ограничивается'));
      head.append(select([['rule','Своё правило'],...(names.length?[['template','Шаблон политики']]:[])],policy.template?'template':'rule',
        kind=>model.setPolicy(object.id,index,kind==='template'
          ?{operations:policy.operations,template:names[0],arguments:Array(arity(names[0])).fill(options[0][0])}
          :{operations:policy.operations,rule:blankRule(options[0][0])}),'Источник правила'));
      const remove=node('button','Удалить');remove.type='button';remove.setAttribute('aria-label',`Удалить ограничение ${index+1}`);
      remove.addEventListener('click',()=>{model.removePolicy(object.id,index);changed();renderPanel();});
      head.append(remove);row.append(head);
      if(policy.template!==undefined){
        row.append(select(names.map(name=>[name,name]),policy.template,
          template=>model.setPolicy(object.id,index,{...policy,template,arguments:Array(arity(template)).fill(options[0][0])}),'Шаблон политики'));
        // One field per declared parameter: the template says WHAT to check, the
        // restriction says on which column of this particular object.
        const declaredParameters=(declared.find(item=>item.name===policy.template)?.parameters)||[];
        declaredParameters.forEach((parameter,at)=>{
          const line=node('div',undefined,'role-rule');
          line.append(node('span','$'+parameter,'muted'));
          line.append(select(options,(policy.arguments||[])[at]||options[0][0],field=>{
            const values=[...(policy.arguments||[])];values[at]=field;
            model.setPolicy(object.id,index,{...policy,arguments:values});
          },`Поле для параметра ${parameter}`));
          row.append(line);
        });
      }
      else row.append(ruleEditor(policy.rule,options,rule=>model.setPolicy(object.id,index,{...policy,rule})));
      block.append(row);
    });
    const add=node('button','Добавить ограничение');add.type='button';
    add.addEventListener('click',()=>{
      model.addPolicy(object.id,{operations:[object.operations[0]],rule:blankRule(options[0][0])});
      policiesExpanded=true;changed();renderPanel();
    });
    block.append(add);
    return block;
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
    // A restriction narrows a right this platform acts on.
    if(object.operations.length)panel.append(renderPolicies(object));
  }
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
      templates=node('details',undefined,'role-templates');host.append(templates);
      warning=node('div',undefined,'role-warning');warning.append(node('span','В роли есть права на удалённые или изменённые объекты. '));const repair=node('button','Убрать недоступные права');repair.type='button';repair.addEventListener('click',()=>{model.removeUnavailable();changed();renderPanel();});warning.append(repair);warning.hidden=!model.hasUnavailable();host.append(warning);
      const body=node('div',undefined,'role-body'),sidebar=node('div',undefined,'role-sidebar'),search=node('input');search.type='search';search.placeholder='Поиск объекта';search.setAttribute('aria-label','Поиск объекта');search.addEventListener('input',()=>renderTree(search.value));tree=node('div',undefined,'role-tree');sidebar.append(search,tree);panel=node('section',undefined,'role-permissions');body.append(sidebar,panel);host.append(body);renderTree();renderTemplates();renderPanel();
    },
    // Переход из окна «Все роли» происходит по конкретному объекту, а не по
    // роли вообще: открывать роль на пустом месте значило бы заставить искать
    // в дереве то, что только что было на экране.
    selectObject(id){const object=source?.schema.objects.find(item=>item.id===id);if(!object)return;selected={object};for(let owner=object.owner;owner;owner=source.schema.objects.find(item=>item.id===owner)?.owner)expanded.add(owner);renderTree();renderPanel();},
    value(){return model?.value();},
    close(){clearTitleField();source=null;model=null;host.hidden=true;host.replaceChildren();},
    setDisabled(disabled){host.inert=disabled;},
  };
}
