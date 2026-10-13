import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const script=readFileSync(new URL('../internal/studio/ui/role-overview.js',import.meta.url),'utf8');
const context=vm.createContext({structuredClone});
// Редактор опирается на общие помощники Studio, как в index.html, где
// studio-common.js подключается первым.
vm.runInContext(readFileSync(new URL('../internal/studio/ui/studio-common.js',import.meta.url),'utf8'),context);
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
    schema:{rights:[{key:'read',title:'Чтение'},{key:'insert',title:'Добавление'},{key:'update',title:'Изменение'},{key:'delete',title:'Удаление'}],objects:[
      {id:'goods',kind:'catalogs',name:'Товары',title:{ru:'Товары'},operations:['read','insert','update','delete'],
        fields:[{key:'ref',operations:['view','edit']},{key:'owner',title:{ru:'Владелец'},operations:['view','edit']},{key:'price',title:{ru:'Цена'},operations:['view','edit']},
          {key:'print',title:{ru:'Печать'},group:'commands',operations:['view']}],
        restrictionFields:[{key:'ref',title:{ru:'Ссылка'}},{key:'owner',title:{ru:'Владелец'}},{key:'price',title:{ru:'Цена'}}]},
      {id:'users',kind:'catalogs',name:'Пользователи',title:{ru:'Пользователи'},operations:['read'],
        fields:[{key:'ref',operations:['view','edit']},{key:'person',title:{ru:'Физлицо'},operations:['view','edit']}],
        restrictionFields:[{key:'ref',title:{ru:'Ссылка'}},{key:'person',title:{ru:'Физлицо'}}]},
    ],forms:[]},
    roles:[
      {path:'metadata/roles/a.yaml',role:{id:'a',name:'Продавец',title:{ru:'Продавец'},objects:[
        {object:'goods',operations:['read','update'],fields:[{field:'ref',operations:['view']},{field:'owner',operations:['view']},{field:'print',operations:['view']}],
         restrictions:[{right:'read',fields:['owner'],condition:'ГДЕ Владелец = &ТекущийПользователь'}]},
      ]}},
      {path:'metadata/roles/b.yaml',role:{id:'b',name:'Кладовщик',title:{ru:'Кладовщик'},objects:[]}},
    ],
  };
}

test('every role appears for the selected object, including one with no grants',()=>{
  const model=create(fixture()),rows=model.grants('goods');
  assert.deepEqual(plain(rows.map(row=>row.title)),['Продавец','Кладовщик']);
  assert.deepEqual(plain(rows[0].operations),{read:true,insert:false,update:true,delete:false});
  assert.equal(rows[0].granted,true);
  assert.equal(rows[1].granted,false);
  assert.deepEqual(plain(rows[1].operations),{read:false,insert:false,update:false,delete:false});
});

// A command the role may see is no field: counting it would show a role
// seeing more fields than the object has.
test('withheld fields are counted, not hidden behind the object checkmark',()=>{
  const model=create(fixture());
  assert.deepEqual(plain(model.grants('goods')[0].fields),{granted:2,total:3});
  assert.deepEqual(plain(model.grants('goods')[1].fields),{granted:0,total:3});
  assert.deepEqual(plain(model.grants('users')[0].fields),{granted:0,total:2});
});

test('an unknown object yields no rows instead of inventing them',()=>{
  assert.deepEqual(plain(create(fixture()).grants('missing')),[]);
});

test('a restriction is described by its fields and its condition as written',()=>{
  const rows=create(fixture()).restrictions('goods');
  assert.equal(rows.length,1);
  assert.deepEqual(plain(rows[0]),{path:'metadata/roles/a.yaml',role:'Продавец',operation:'read',
    field:'Владелец',condition:'ГДЕ Владелец = &ТекущийПользователь'});
  assert.deepEqual(plain(create(fixture()).restrictions('users')),[]);
});

// A restriction without fields is the one on «Прочие поля», as the prototype's
// editor names it; a field the object no longer has is shown by its key.
test('a restriction without fields is on the other fields',()=>{
  const source=fixture();
  source.roles[0].role.objects[0].restrictions=[{right:'read',fields:['price','gone']},{right:'update',condition:'ГДЕ ЛОЖЬ'}];
  const rows=create(source).restrictions('goods');
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
    assert.match(model.restrictions('goods')[0].field,/Owner/);
  } finally { context.StudioLanguage.set('ru'); }
});

// Дефект: окно показывало право по записи в роли, и роль с флагом «для
// новых объектов», которая пишет одни запреты (ПолныеПрава прототипа),
// выглядела ничего не выдающей.
test('a role with the flag for new objects is shown with what it does not deny',()=>{
  const source=fixture();
  source.roles.push({path:'metadata/roles/c.yaml',role:{id:'c',name:'Полные',title:{ru:'Полные'},grantNewObjectsByDefault:true,objects:[{object:'goods',denied:['delete']}]}});
  const row=create(source).grants('goods').find(item=>item.name==='Полные');
  assert.deepEqual(plain(row.operations),{read:true,insert:true,update:true,delete:false});
  assert.equal(row.granted,true);
});

// Every table part has its own Номер строки. Defect caught: a field of a
// table part named without its part, so the restrictions on the line numbers
// of two parts read the same.
test('a field of a table part is named after the part',()=>{
  const source=fixture();
  source.schema.objects[0].restrictionFields.push({key:'items',name:'Товары',title:{ru:'Товары'}},{key:'items.linenumber',name:'НомерСтроки',title:{ru:'Номер строки'},parent:'items'});
  source.roles[0].role.objects[0].restrictions=[{right:'read',fields:['items.linenumber','owner'],condition:'ГДЕ ИСТИНА'}];
  assert.equal(create(source).restrictions('goods')[0].field,'Товары.Номер строки, Владелец');
});
