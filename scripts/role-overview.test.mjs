import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const script=readFileSync(new URL('../internal/studio/ui/role-overview.js',import.meta.url),'utf8');
const context=vm.createContext({structuredClone});
// Окно опирается на общие помощники Studio и на модель редактора роли, как в
// index.html, где studio-common.js и role-editor.js подключаются раньше.
vm.runInContext(readFileSync(new URL('../internal/studio/ui/studio-common.js',import.meta.url),'utf8'),context);
vm.runInContext(readFileSync(new URL('../internal/studio/ui/role-editor.js',import.meta.url),'utf8'),context);
vm.runInContext(script,context);
vm.runInContext('globalThis.StudioLanguage = StudioLanguage;',context);
const create=context.createRoleOverviewModel;
// Модель выполняется в отдельном realm виртуальной машины: её массивы и
// объекты не проходят по ссылочному сравнению deepStrictEqual, поэтому
// сверяем по значению.
const plain=value=>JSON.parse(JSON.stringify(value));

function fixture(){
  return {
    defaultLanguage:'ru',
    schema:{rights:[{key:'read',title:'Чтение'},{key:'insert',title:'Добавление'},{key:'update',title:'Изменение'},{key:'delete',title:'Удаление'},{key:'view',title:'Просмотр'},{key:'edit',title:'Редактирование'},{key:'input-by-string',title:'Ввод по строке'}],
      partGroups:[{key:'attributes',title:'Реквизиты'},{key:'commands',title:'Команды'}],
      objects:[
      {id:'goods',kind:'catalogs',name:'Товары',title:{ru:'Товары'},rights:['read','insert','update','delete','view','edit','input-by-string'],operations:['read','insert','update','delete'],
        fields:[{key:'ref',operations:['view','edit']},{key:'owner',title:{ru:'Владелец'},group:'attributes',operations:['view','edit']},{key:'price',title:{ru:'Цена'},group:'attributes',operations:['view','edit']},
          {key:'print',title:{ru:'Печать'},group:'commands',operations:['view']}],
        restrictionFields:[{key:'ref',title:{ru:'Ссылка'}},{key:'owner',title:{ru:'Владелец'}},{key:'price',title:{ru:'Цена'}}],
        restrictedRights:['read','insert','update','delete']},
      {id:'users',kind:'catalogs',name:'Пользователи',title:{ru:'Пользователи'},rights:['read','view'],operations:['read'],
        fields:[{key:'ref',operations:['view','edit']},{key:'person',title:{ru:'Физлицо'},group:'attributes',operations:['view','edit']}],
        restrictionFields:[{key:'ref',title:{ru:'Ссылка'}},{key:'person',title:{ru:'Физлицо'}}],
        restrictedRights:['read']},
    ],
    parts:{goods:['ref','owner','price','print'],users:['ref','person']},
    forms:[{id:'list',name:'ФормаСписка',title:{ru:'Форма списка'},commands:[{id:'post',name:'Провести',title:{ru:'Провести'}},{id:'copy',name:'Скопировать',title:{ru:'Скопировать'}}]}]},
    roles:[
      {path:'metadata/roles/a.yaml',revision:'ra',role:{id:'a',name:'Продавец',title:{ru:'Продавец'},objects:[
        {object:'goods',operations:['read','update'],fields:[{field:'ref',operations:['view']},{field:'owner',operations:['view']},{field:'print',operations:['view']}],
         restrictions:[{right:'read',fields:['owner'],condition:'ГДЕ Владелец = &ТекущийПользователь'}]},
      ]}},
      {path:'metadata/roles/b.yaml',revision:'rb',role:{id:'b',name:'Кладовщик',title:{ru:'Кладовщик'},objects:[]}},
    ],
  };
}
const goods=source=>({object:source.schema.objects[0]});
const roleValue=(model,index)=>plain(model.model(index).value());

// Дефект: окно показывало только восемь исполняемых прав, а прототип —
// все права вида, и права «Просмотр», «Ввод по строке» нельзя было отметить.
test('the rows of an object are every right of its kind, in the order of its kind',()=>{
  const source=fixture(),model=create(source);
  assert.deepEqual(plain(model.rows(goods(source)).map(row=>row.key)),['read','insert','update','delete','view','edit','input-by-string']);
  assert.deepEqual(plain(model.rows({object:source.schema.objects[1]}).map(row=>row.key)),['read','view']);
  assert.deepEqual(plain(model.rows(null)),[]);
});

// Столбец — каждая роль, в том числе без единого права: вопрос окна — «кто
// дотянется до объекта», и пустой столбец отвечает на него так же.
test('every role is a column, one with no grants too, read as the role reads it',()=>{
  const source=fixture(),model=create(source),selection=goods(source);
  assert.deepEqual(plain(model.roles().map(role=>role.title)),['Продавец','Кладовщик']);
  assert.deepEqual(['read','insert','update','delete','view'].map(key=>model.has(0,selection,key)),[true,false,true,false,false]);
  assert.deepEqual(['read','update'].map(key=>model.has(1,selection,key)),[false,false]);
});

// Дефект: окно показывало право по записи в роли, и роль с флагом «для
// новых объектов», которая пишет одни запреты (ПолныеПрава прототипа),
// выглядела ничего не выдающей.
test('a role with the flag for new objects is shown with what it does not deny',()=>{
  const source=fixture();
  source.roles.push({path:'metadata/roles/c.yaml',revision:'rc',role:{id:'c',name:'Полные',title:{ru:'Полные'},grantNewObjectsByDefault:true,objects:[{object:'goods',denied:['delete']}]}});
  const model=create(source),selection=goods(source);
  assert.deepEqual(['read','insert','update','delete','input-by-string'].map(key=>model.has(2,selection,key)),[true,true,true,false,true]);
  assert.equal(model.roleState(2,selection),'some');
});

// Первая строка (справка): флажок ставит или снимает все права роли на
// объект. Дефект: правка задела бы другую роль или права частей объекта.
test('the first row sets or clears every right of one role on the object, and nothing else',()=>{
  const source=fixture(),model=create(source),selection=goods(source);
  assert.equal(model.roleState(0,selection),'some');
  assert.equal(model.roleState(1,selection),'none');
  model.setRole(1,selection,true);
  assert.equal(model.roleState(1,selection),'all');
  assert.deepEqual(plain(roleValue(model,1).objects),[{object:'goods',operations:['read','insert','update','delete','view','edit','input-by-string'],fields:[]}]);
  assert.deepEqual(roleValue(model,0).objects,plain(source.roles[0].role.objects));
  assert.deepEqual(plain(model.changes().map(change=>change.path)),['metadata/roles/b.yaml']);
  model.setRole(0,selection,false);
  assert.equal(model.roleState(0,selection),'none');
  const seller=roleValue(model,0).objects[0];
  assert.deepEqual(plain(seller.operations),[]);
  assert.deepEqual(plain(seller.denied),['read','insert','update','delete','view','edit','input-by-string']);
  // Права реквизитов и команд снятие прав объекта не трогает.
  assert.deepEqual(plain(seller.fields),plain(source.roles[0].role.objects[0].fields));
});

// Первая колонка (справка): флажок ставит или снимает право во всех ролях.
test('the first column sets or clears one right in every role, and no other right',()=>{
  const source=fixture(),model=create(source),selection=goods(source);
  assert.equal(model.rightState(selection,'read'),'some');
  assert.equal(model.rightState(selection,'delete'),'none');
  model.setRight(selection,'delete',true);
  assert.equal(model.rightState(selection,'delete'),'all');
  assert.deepEqual([0,1].map(index=>model.has(index,selection,'delete')),[true,true]);
  assert.deepEqual([0,1].map(index=>model.has(index,selection,'insert')),[false,false]);
  model.setRight(selection,'read',false);
  assert.equal(model.rightState(selection,'read'),'none');
  assert.equal(model.has(0,selection,'update'),true);
});

test('a part of an object is edited by its own rights, the object left as it was',()=>{
  const source=fixture(),model=create(source),object=source.schema.objects[0];
  const selection={object,part:object.fields[2]};
  assert.deepEqual(plain(model.rows(selection).map(row=>row.key)),['view','edit']);
  assert.equal(model.rightState(selection,'view'),'none');
  model.setRight(selection,'view',true);
  assert.equal(model.rightState(selection,'view'),'all');
  assert.deepEqual(plain(roleValue(model,1).objects),[{object:'goods',operations:[],fields:[{field:'price',operations:['view']}]}]);
  assert.deepEqual(plain(roleValue(model,0).objects[0].operations),['read','update']);
});

test('the commands of a form are rows too',()=>{
  const source=fixture(),model=create(source),selection={form:source.schema.forms[0]};
  assert.deepEqual(plain(model.rows(selection).map(row=>row.key)),['post','copy']);
  model.setRole(0,selection,true);
  assert.equal(model.roleState(0,selection),'all');
  assert.deepEqual(plain(roleValue(model,0).commands),[{form:'list',command:'post'},{form:'list',command:'copy'}]);
  assert.equal(model.roleState(1,selection),'none');
});

// Сохраняются только изменённые роли, каждая со своей ревизией. Дефект:
// окно пишет все роли разом и затирает роль, изменённую вне Studio, хотя её
// здесь не трогали.
test('only the roles changed in the window are saved, each with the revision it was read at',()=>{
  const source=fixture(),model=create(source),selection=goods(source);
  assert.deepEqual(plain(model.changes()),[]);
  model.set(1,selection,'view',true);
  const changes=plain(model.changes());
  assert.deepEqual(changes.map(change=>[change.path,change.expectedRevision]),[['metadata/roles/b.yaml','rb']]);
  assert.deepEqual(changes[0].role.objects,[{object:'goods',operations:['view'],fields:[]}]);
});

test('a restriction put through a role of the window is listed among all restrictions',()=>{
  const source=fixture(),model=create(source);
  model.model(1).putRestriction('users','read',0,{fields:[],condition:'ГДЕ ИСТИНА'});
  assert.deepEqual(plain(model.allRestrictions().map(row=>`${row.object}/${row.role}/${row.field}/${row.condition}`)),
    ['Пользователи/Кладовщик/Прочие поля/ГДЕ ИСТИНА','Товары/Продавец/Владелец/ГДЕ Владелец = &ТекущийПользователь']);
  assert.deepEqual(plain(model.changes().map(change=>change.path)),['metadata/roles/b.yaml']);
});

// A restriction without fields is the one on «Прочие поля», as the prototype's
// editor names it; a field the object no longer has is shown by its key.
test('a restriction without fields is on the other fields',()=>{
  const source=fixture();
  source.roles[0].role.objects[0].restrictions=[{right:'read',fields:['price','gone']},{right:'update',condition:'ГДЕ ЛОЖЬ'}];
  const rows=create(source).allRestrictions();
  assert.deepEqual(plain(rows.map(row=>[row.operation,row.field,row.condition])),[['read','Цена, gone',''],['update','Прочие поля','ГДЕ ЛОЖЬ']]);
});

test('the project-wide list carries every restriction of every role, ordered',()=>{
  const source=fixture();
  source.roles[1].role.objects=[{object:'users',operations:['read'],
    restrictions:[{right:'read',condition:'ГДЕ Физлицо <> НЕОПРЕДЕЛЕНО'}]}];
  const rows=create(source).allRestrictions();
  assert.deepEqual(plain(rows.map(row=>`${row.object}/${row.role}/${row.operation}`)),
    ['Пользователи/Кладовщик/read','Товары/Продавец/read']);
  assert.equal(rows[1].fields,'2 из 3');
  assert.equal(rows[0].fields,'0 из 2');
  assert.equal(rows[1].objectID,'goods');
});

test('a restriction on an object that no longer exists is left out of the list',()=>{
  const source=fixture();
  source.roles[0].role.objects.push({object:'gone',operations:['read'],restrictions:[{right:'read',condition:'ГДЕ ЛОЖЬ'}]});
  assert.equal(create(source).allRestrictions().length,1);
});

test('templates are listed per role with their names and texts as written',()=>{
  const source=fixture();
  source.roles[0].role.restrictionTemplates=[{name:'ПоВладельцу(Поле)',condition:'ГДЕ #Параметр(1) = &ТекущийПользователь'}];
  source.roles[1].role.restrictionTemplates=[{name:'ПоЦене',condition:'ГДЕ Цена В (10)'}];
  const rows=create(source).allTemplates();
  assert.deepEqual(plain(rows.map(row=>[row.role,row.name,row.condition])),[
    ['Кладовщик','ПоЦене','ГДЕ Цена В (10)'],
    ['Продавец','ПоВладельцу(Поле)','ГДЕ #Параметр(1) = &ТекущийПользователь'],
  ]);
  assert.deepEqual(plain(create(fixture()).allTemplates()),[]);
});

test('the view file keeps the model free of the DOM',()=>{
  const model=script.slice(script.indexOf('function createRoleOverviewModel'),script.indexOf('function createRoleOverview('));
  assert.equal(/document\.|window\./.test(model),false);
});

test('the Studio page loads the overview and opens it from the roles group node',()=>{
  const html=readFileSync(new URL('../internal/studio/ui/index.html',import.meta.url),'utf8');
  for(const match of html.matchAll(/<script>([\s\S]*?)<\/script>/g))new vm.Script(match[1]);
  assert.match(html,/\/ui\/role-overview\.js/);
  assert.match(html,/\/ui\/role-overview\.css/);
  assert.match(html,/id="role-overview"/);
  assert.match(html,/fetch\('\/api\/roles'\)/);
  // Окно правит роли: «Сохранить» пишет изменённые роли одним запросом.
  assert.match(html,/endpoint='\/api\/roles';payload=\{roles:roleOverview\.value\(\)\}/);
  assert.ok(html.indexOf('/ui/role-editor.js')<html.indexOf('/ui/role-overview.js'));
  assert.match(html,/node\.id==='metadata\/roles'/);
});

test('the overview offers all three tables of the roles group node',()=>{
  const view=script.slice(script.indexOf('function createRoleOverview('));
  for(const label of ['Все роли','Ограничения доступа','Шаблоны ограничений'])assert.ok(view.includes(label),label);
});

// Подписи обзора показываются на текущем языке Studio, как в каждом
// редакторе. Дефект: свой выбор перевода — язык по умолчанию и первый
// попавшийся, мимо переключателя языка Studio.
test('object and field titles follow the Studio language',()=>{
  const source=fixture();
  source.languages=[{code:'ru'},{code:'en'}];
  source.schema.objects[0].title={ru:'Товары',en:'Goods'};
  source.schema.objects[0].restrictionFields[1].title={ru:'Владелец',en:'Owner'};
  context.StudioLanguage.set('en');
  try {
    const model=create(source);
    assert.equal(model.title(model.objects().find(item=>item.id==='goods')),'Goods');
    assert.match(model.allRestrictions()[0].field,/Owner/);
  } finally { context.StudioLanguage.set('ru'); }
});

// Every table part has its own Номер строки. Defect caught: a field of a
// table part named without its part, so the restrictions on the line numbers
// of two parts read the same.
test('a field of a table part is named after the part',()=>{
  const source=fixture();
  source.schema.objects[0].restrictionFields.push({key:'items',name:'Товары',title:{ru:'Товары'}},{key:'items.linenumber',name:'НомерСтроки',title:{ru:'Номер строки'},parent:'items'});
  source.roles[0].role.objects[0].restrictions=[{right:'read',fields:['items.linenumber','owner'],condition:'ГДЕ ИСТИНА'}];
  assert.equal(create(source).allRestrictions()[0].field,'Товары.Номер строки, Владелец');
});
