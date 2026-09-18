/* Role editor: explicit grants underneath, ordinary tree/checkboxes in Studio. */
function createRoleModel(source) {
  const value = structuredClone(source);
  value.role.objects ||= [];
  value.role.commands ||= [];
  value.role.policyTemplates ||= [];
  const definitions = new Map(value.schema.objects.map(item => [item.id, item]));
  const fields = new Map(value.schema.objects.map(item => [item.id, new Map(item.fields.map(field => [field.key, field]))]));
  const children = new Map(value.schema.objects.map(item => {
    const result = new Map();
    for(const field of item.fields)if(field.parent){if(!result.has(field.parent))result.set(field.parent,[]);result.get(field.parent).push(field.key);}
    return [item.id,result];
  }));
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
  function assignField(id, key, operation, enabled) {
    if (!fields.get(id)?.get(key)?.operations.includes(operation)) return;
    const grant = object(id), index = fieldGrants.get(id);
    let target = index.get(key);
    if(!target){target={field:key,operations:[]};grant.fields.push(target);index.set(key,target);}
    target.operations=toggle(target.operations,operation,enabled);
  }
  function setField(id, key, operation, enabled) {
    const field = fields.get(id)?.get(key);
    if (!field?.operations.includes(operation)) return;
    const keys = [key, ...(children.get(id).get(key)||[])];
    if (enabled && field.parent) keys.push(field.parent);
    for (const fieldKey of keys) {
      assignField(id,fieldKey,operation,enabled);
    }
  }
  function setObject(id, operation, enabled) {
    const definition = definitions.get(id);
    if (!definition?.operations.includes(operation)) return;
    const grant = object(id);
    grant.operations = toggle(grant.operations, operation, enabled);
    const fieldOperation = operation === 'read' ? 'read' : ['create','update'].includes(operation) ? 'update' : null;
    if (!fieldOperation) return;
    // Removing Create must not remove field editing still needed by Update.
    if (!enabled && fieldOperation === 'update' && grant.operations.some(item => ['create','update'].includes(item))) return;
    for (const field of definition.fields) assignField(id, field.key, fieldOperation, enabled);
  }
  function clean() {
    const result = structuredClone(value.role);
    for (const item of result.objects) {
      item.operations ||= []; item.fields = (item.fields||[]).filter(field=>field.operations.length);
      item.policies = (item.policies||[]).filter(policy=>policy.operations?.length);
      if(!item.policies.length)delete item.policies;
    }
    result.objects = result.objects.filter(item => item.operations.length || item.fields.length || item.policies);
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
    const own = fields.get(objectID);
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
    const source = fields.get(rule.subquery.object);
    if (!source || !source.has(rule.subquery.field)) return false;
    return (rule.subquery.where||[]).every(condition => {
      const conditionField = resolveField(condition.field);
      return conditionField && source.has(conditionField);
    });
  }
  function repaired() {
    const result = clean();
    result.objects = result.objects.filter(item => definitions.has(item.object));
    for (const item of result.objects) {
      const definition = definitions.get(item.object);
      item.operations = item.operations.filter(operation => definition.operations.includes(operation));
      item.fields = item.fields.filter(field => fields.get(item.object).has(field.field));
      for (const field of item.fields) {
        const descriptor = fields.get(item.object).get(field.field);
        field.operations = field.operations.filter(operation => descriptor.operations.includes(operation));
      }
      item.fields = item.fields.filter(field => field.operations.length);
      if (item.policies) {
        item.policies = item.policies.filter(policy => policyStillApplies(result, item.object, definition, policy));
        if(!item.policies.length)delete item.policies;
      }
    }
    result.objects = result.objects.filter(item => item.operations.length || item.fields.length || item.policies);
    result.commands = result.commands.filter(item => value.schema.forms.some(form => form.id === item.form && form.commands.some(command => command.id === item.command)));
    return result;
  }
  return {
    value: clean,
    setName(name) { value.role.name = name; },
    setTitle(language, title) { if(title.trim())value.role.title[language]=title;else delete value.role.title[language]; },
    setComment(comment) { value.role.comment = comment; },
    setGrantNewObjectsByDefault(enabled) { value.role.grantNewObjectsByDefault = enabled; },
    setGrantNewFieldsByDefault(enabled) { value.role.grantNewFieldsByDefault = enabled; },
    setObject, setField,
    hasObject(id, operation) { return objects.get(id)?.operations?.includes(operation) || false; },
    hasField(id, key, operation) { return fieldGrants.get(id)?.get(key)?.operations.includes(operation) || false; },
    setAll(id, enabled) { for(const operation of definitions.get(id)?.operations || [])setObject(id, operation, enabled); },
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
  let source, model, selected, panel, tree, warning, templates, fieldsExpanded = false, policiesExpanded = false, titleFieldCleanup = null;
  function clearTitleField() { titleFieldCleanup?.(); titleFieldCleanup = null; }
  const operations = {read:'Чтение',create:'Добавление',update:'Изменение',delete:'Удаление',post:'Проведение','undo-posting':'Отмена проведения'};
  const kinds = {constants:'Константы',enumerations:'Перечисления',catalogs:'Справочники',documents:'Документы','information-registers':'Регистры сведений','accumulation-registers':'Регистры накопления'};
  const standard = {ref:'Ссылка',code:'Код',description:'Наименование',deletionmark:'Пометка удаления',version:'Версия',predefined:'Предопределённый',predefineddataname:'Имя предопределённых данных',number:'Номер',date:'Дата',posted:'Проведён',recordid:'Идентификатор записи',period:'Период',recorder:'Регистратор',linenumber:'Номер строки',active:'Активность',movementkind:'Вид движения',value:'Значение',order:'Порядок'};
  function node(tag, text, className) {
    const element = document.createElement(tag);
    if(text !== undefined)element.textContent=text;
    if(className)element.className=className;
    return element;
  }
  function title(item) { return item.title?.[source.defaultLanguage] || Object.values(item.title || {}).find(Boolean) || standard[item.key] || item.name; }
  function changed() { warning.hidden=!model.hasUnavailable(); onChange(); }
  function checkbox(text, checked, action, disabled=false) {
    const label=node('label',undefined,'role-check'), input=node('input');
    input.type='checkbox'; input.checked=checked; input.disabled=disabled;
    input.addEventListener('change',()=>{const focus=input.getAttribute('aria-label')||text;action(input.checked);changed();renderPanel();for(const candidate of panel.querySelectorAll('input'))if((candidate.getAttribute('aria-label')||candidate.parentElement.textContent)===focus){candidate.focus();break;}});
    label.append(input,node('span',text)); return label;
  }
  const operators = {eq:'Равно',ne:'Не равно',in:'В списке','not-in':'Не в списке'};
  function select(options, current, action, label) {
    const element=node('select');
    for(const [key,text] of options){const option=node('option',text);option.value=key;element.append(option);}
    element.value=current; if(label)element.setAttribute('aria-label',label);
    element.addEventListener('change',()=>{action(element.value);changed();renderPanel();});
    return element;
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
      ...(rule.parameter||rule.subquery?{}:{values:parseOperand(operandText(rule),operator)})}),'Оператор'));
    const names=source.schema.sessionParameters||[];
    const kind=rule.subquery?'subquery':(rule.parameter?'parameter':'values');
    const kinds=[['values','Значение'],...(names.length?[['parameter','Параметр сеанса']]:[]),
      ...(allowSubquery&&source.schema.objects.length?[['subquery','Подзапрос']]:[])];
    row.append(select(kinds,kind,chosen=>{
      if(chosen==='parameter')apply({field:rule.field,operator:rule.operator,parameter:names[0]});
      else if(chosen==='subquery'){
        const object=source.schema.objects[0];
        apply({field:rule.field,operator:'in',subquery:{object:object.id,field:object.fields[0]?.key||'ref',where:[]}});
      } else apply({...blankRule(rule.field),operator:rule.operator});
    },'Вид операнда'));
    if(kind==='parameter'){
      row.append(select(names.map(name=>[name,name]),rule.parameter,parameter=>apply({...rule,parameter}),'Параметр сеанса'));
      return row;
    }
    if(kind==='subquery'){
      const block=node('div',undefined,'role-subquery');block.append(row);
      const chosen=source.schema.objects.find(item=>item.id===rule.subquery.object)||source.schema.objects[0];
      const line=node('div',undefined,'role-rule');
      line.append(node('span','из','muted'));
      line.append(select(source.schema.objects.map(item=>[item.id,title(item)]),rule.subquery.object,id=>{
        const next=source.schema.objects.find(item=>item.id===id);
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
  function objectFieldOptions(object) { return object.fields.map(field=>[field.key,(field.parent?'↳ ':'')+title(field)]); }
  function allFieldOptions() {
    const result=new Map();
    for(const object of source.schema.objects)for(const field of object.fields)if(!result.has(field.key))result.set(field.key,title(field));
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
      head.append(select(object.operations.map(operation=>[operation,operations[operation]]),policy.operations[0],
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
  function renderTree(filter='') {
    tree.replaceChildren(); const groups=new Map();
    for(const item of source.schema.objects) {
      if(!`${item.name} ${title(item)}`.toLocaleLowerCase().includes(filter.toLocaleLowerCase().trim()))continue;
      let group=groups.get(item.kind);
      if(!group){group=node('details');group.open=true;group.append(node('summary',kinds[item.kind]||item.kind));groups.set(item.kind,group);tree.append(group);}
      const button=node('button',title(item),'role-tree-item');button.type='button';button.title=item.name;button.dataset.object=item.id;
      button.addEventListener('click',()=>{selected={object:item};fieldsExpanded=false;renderPanel();});group.append(button);
    }
    const forms=source.schema.forms.filter(item=>`${item.name} ${title(item)}`.toLocaleLowerCase().includes(filter.toLocaleLowerCase().trim()));
    if(forms.length){const group=node('details');group.open=true;group.append(node('summary','Команды форм'));for(const form of forms){const button=node('button',title(form),'role-tree-item');button.type='button';button.addEventListener('click',()=>{selected={form};renderPanel();});group.append(button);}tree.append(group);}
    if(!tree.childElementCount)tree.append(node('p','Нет подходящих объектов','muted'));
  }
  function renderPanel() {
    panel.replaceChildren();
    if(!selected){panel.append(node('p','Выберите объект слева и отметьте разрешённые действия.','muted'));return;}
    if(selected.form){const form=selected.form;panel.append(node('h3',title(form)),node('p','Доступ к команде не заменяет права на используемые данные.','muted'));for(const command of form.commands)panel.append(checkbox(title(command),model.hasCommand(form.id,command.id),enabled=>model.setCommand(form.id,command.id,enabled)));return;}
    const object=selected.object;panel.append(node('h3',title(object)));
    const actions=node('div',undefined,'role-operation-list');
    for(const operation of object.operations)actions.append(checkbox(operations[operation],model.hasObject(object.id,operation),enabled=>model.setObject(object.id,operation,enabled)));
    panel.append(actions);
    const all=node('div',undefined,'role-bulk');
    for(const [text,enabled] of [['Разрешить всё',true],['Снять все права',false]]){const button=node('button',text);button.type='button';button.addEventListener('click',()=>{model.setAll(object.id,enabled);changed();renderPanel();});all.append(button);}
    panel.append(all,node('p','Чтение и изменение проставляются сразу на существующие реквизиты. При необходимости уточните их ниже.','muted'));
    const fields=node('details',undefined,'role-fields');fields.open=fieldsExpanded;fields.addEventListener('toggle',()=>fieldsExpanded=fields.open);fields.append(node('summary','Реквизиты и табличные части'));
    const table=node('table'),head=node('tr');head.append(node('th','Реквизит'),node('th','Чтение'),node('th','Изменение'));table.append(head);
    for(const field of object.fields){const row=node('tr');row.append(node('td',(field.parent?'↳ ':'')+title(field)));for(const operation of ['read','update']){const cell=node('td');cell.append(checkbox('',model.hasField(object.id,field.key,operation),enabled=>model.setField(object.id,field.key,operation,enabled),!field.operations.includes(operation)));cell.querySelector('input').setAttribute('aria-label',`${title(field)} — ${operations[operation]}`);row.append(cell);}table.append(row);}
    fields.append(table);panel.append(fields);
    panel.append(renderPolicies(object));
  }
  return {
    open(value, preserveSelection=false) {
      clearTitleField();
      const previous=preserveSelection?selected:null,expanded=preserveSelection&&fieldsExpanded;source=structuredClone(value);model=createRoleModel(source);selected=null;if(previous?.object){const object=source.schema.objects.find(item=>item.id===previous.object.id);if(object)selected={object};}if(previous?.form){const form=source.schema.forms.find(item=>item.id===previous.form.id);if(form)selected={form};}fieldsExpanded=expanded;host.hidden=false;host.replaceChildren();
      const identity=node('div',undefined,'role-identity'),label=node('label','Имя роли'),name=node('input');name.value=source.role.name;name.maxLength=128;name.addEventListener('input',()=>{model.setName(name.value);changed();});label.append(name);identity.append(label);
      const titleLabel=node('label','Заголовок'),titleField=createLocalizedTitleField(source.languages,code=>source.role.title[code],(code,value)=>model.setTitle(code,value),changed);
      titleFieldCleanup=titleField.destroy;titleLabel.append(titleField);identity.append(titleLabel);
      const commentLabel=node('label','Комментарий'),comment=node('textarea');comment.value=source.role.comment||'';comment.maxLength=4000;comment.rows=2;
      comment.addEventListener('change',()=>{model.setComment(comment.value);changed();});commentLabel.append(comment);identity.append(commentLabel);
      identity.append(checkbox('Устанавливать права для новых объектов',!!source.role.grantNewObjectsByDefault,enabled=>model.setGrantNewObjectsByDefault(enabled)));
      identity.append(checkbox('Устанавливать права для реквизитов и табличных частей по умолчанию',!!source.role.grantNewFieldsByDefault,enabled=>model.setGrantNewFieldsByDefault(enabled)));
      host.append(identity);
      templates=node('details',undefined,'role-templates');host.append(templates);
      warning=node('div',undefined,'role-warning');warning.append(node('span','В роли есть права на удалённые или изменённые объекты. '));const repair=node('button','Убрать недоступные права');repair.type='button';repair.addEventListener('click',()=>{model.removeUnavailable();changed();renderPanel();});warning.append(repair);warning.hidden=!model.hasUnavailable();host.append(warning);
      const body=node('div',undefined,'role-body'),sidebar=node('div',undefined,'role-sidebar'),search=node('input');search.type='search';search.placeholder='Поиск объекта';search.setAttribute('aria-label','Поиск объекта');search.addEventListener('input',()=>renderTree(search.value));tree=node('div',undefined,'role-tree');sidebar.append(search,tree);panel=node('section',undefined,'role-permissions');body.append(sidebar,panel);host.append(body);renderTree();renderTemplates();renderPanel();
    },
    // Переход из окна «Все роли» происходит по конкретному объекту, а не по
    // роли вообще: открывать роль на пустом месте значило бы заставить искать
    // в дереве то, что только что было на экране.
    selectObject(id){const object=source?.schema.objects.find(item=>item.id===id);if(!object)return;selected={object};fieldsExpanded=false;renderTree();renderPanel();},
    value(){return model?.value();},
    close(){clearTitleField();source=null;model=null;host.hidden=true;host.replaceChildren();},
    setDisabled(disabled){host.inert=disabled;},
  };
}
