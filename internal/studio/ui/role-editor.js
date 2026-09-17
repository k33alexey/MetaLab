/* Role editor: explicit grants underneath, ordinary tree/checkboxes in Studio. */
function createRoleModel(source) {
  const value = structuredClone(source);
  value.role.objects ||= [];
  value.role.commands ||= [];
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
    }
    result.objects = result.objects.filter(item => item.operations.length || item.fields.length);
    return result;
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
    }
    result.objects = result.objects.filter(item => item.operations.length || item.fields.length);
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
    hasUnavailable() { return JSON.stringify(clean()) !== JSON.stringify(repaired()); },
    removeUnavailable() { value.role = repaired(); reindex(); },
  };
}

function createRoleEditor(host, onChange) {
  let source, model, selected, panel, tree, warning, fieldsExpanded = false, titleFieldCleanup = null;
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
      warning=node('div',undefined,'role-warning');warning.append(node('span','В роли есть права на удалённые или изменённые объекты. '));const repair=node('button','Убрать недоступные права');repair.type='button';repair.addEventListener('click',()=>{model.removeUnavailable();changed();renderPanel();});warning.append(repair);warning.hidden=!model.hasUnavailable();host.append(warning);
      const body=node('div',undefined,'role-body'),sidebar=node('div',undefined,'role-sidebar'),search=node('input');search.type='search';search.placeholder='Поиск объекта';search.setAttribute('aria-label','Поиск объекта');search.addEventListener('input',()=>renderTree(search.value));tree=node('div',undefined,'role-tree');sidebar.append(search,tree);panel=node('section',undefined,'role-permissions');body.append(sidebar,panel);host.append(body);renderTree();renderPanel();
    },
    value(){return model?.value();},
    close(){clearTitleField();source=null;model=null;host.hidden=true;host.replaceChildren();},
    setDisabled(disabled){host.inert=disabled;},
  };
}
