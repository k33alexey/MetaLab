import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const script=readFileSync(new URL('../internal/studio/ui/role-overview.js',import.meta.url),'utf8');
const context=vm.createContext({structuredClone});
vm.runInContext(script,context);
const create=context.createRoleOverviewModel;
// Модель выполняется в отдельном realm виртуальной машины: её массивы и
// объекты не проходят по ссылочному сравнению deepStrictEqual, поэтому
// сверяем по значению.
const plain=value=>JSON.parse(JSON.stringify(value));

function fixture(){
  return {
    defaultLanguage:'ru',
    schema:{objects:[
      {id:'goods',kind:'catalogs',name:'Товары',title:{ru:'Товары'},operations:['read','create','update','delete'],
        fields:[{key:'ref',operations:['read']},{key:'owner',title:{ru:'Владелец'},operations:['read','update']},{key:'price',title:{ru:'Цена'},operations:['read','update']}]},
      {id:'users',kind:'catalogs',name:'Пользователи',title:{ru:'Пользователи'},operations:['read'],
        fields:[{key:'ref',operations:['read']},{key:'person',title:{ru:'Физлицо'},operations:['read']}]},
    ],forms:[]},
    roles:[
      {path:'metadata/roles/a.yaml',role:{id:'a',name:'Продавец',title:{ru:'Продавец'},objects:[
        {object:'goods',operations:['read','update'],fields:[{field:'ref',operations:['read']},{field:'owner',operations:['read']}],
         policies:[{operations:['read'],rule:{field:'owner',operator:'eq',parameter:'ТекущийПользователь'}}]},
      ]}},
      {path:'metadata/roles/b.yaml',role:{id:'b',name:'Кладовщик',title:{ru:'Кладовщик'},objects:[]}},
    ],
  };
}

test('every role appears for the selected object, including one with no grants',()=>{
  const model=create(fixture()),rows=model.grants('goods');
  assert.deepEqual(plain(rows.map(row=>row.title)),['Продавец','Кладовщик']);
  assert.deepEqual(plain(rows[0].operations),{read:true,create:false,update:true,delete:false});
  assert.equal(rows[0].granted,true);
  assert.equal(rows[1].granted,false);
  assert.deepEqual(plain(rows[1].operations),{read:false,create:false,update:false,delete:false});
});

test('withheld fields are counted, not hidden behind the object checkmark',()=>{
  const model=create(fixture());
  assert.deepEqual(plain(model.grants('goods')[0].fields),{granted:2,total:3});
  assert.deepEqual(plain(model.grants('goods')[1].fields),{granted:0,total:3});
  assert.deepEqual(plain(model.grants('users')[0].fields),{granted:0,total:2});
});

test('an unknown object yields no rows instead of inventing them',()=>{
  assert.deepEqual(plain(create(fixture()).grants('missing')),[]);
});

test('a restriction is described by field, condition and where it came from',()=>{
  const rows=create(fixture()).restrictions('goods');
  assert.equal(rows.length,1);
  assert.deepEqual(plain(rows[0]),{path:'metadata/roles/a.yaml',role:'Продавец',operation:'read',
    origin:'своё правило',field:'Владелец',condition:'Равно параметр сеанса ТекущийПользователь'});
  assert.deepEqual(plain(create(fixture()).restrictions('users')),[]);
});

test('one restriction on several operations becomes one row per operation',()=>{
  const source=fixture();
  source.roles[0].role.objects[0].policies=[{operations:['read','update'],rule:{field:'price',operator:'in',values:[{kind:'number',data:'10'},{kind:'number',data:'20'}]}}];
  const rows=create(source).restrictions('goods');
  assert.deepEqual(plain(rows.map(row=>row.operation)),['read','update']);
  assert.equal(rows[0].condition,'В списке «10», «20»');
});

test('a template restriction is shown resolved, with its arguments substituted',()=>{
  const source=fixture();
  source.roles[0].role.policyTemplates=[{name:'ПоВладельцу',parameters:['Поле'],
    rule:{field:'$Поле',operator:'eq',parameter:'ТекущийПользователь'}}];
  source.roles[0].role.objects[0].policies=[{operations:['read'],template:'ПоВладельцу',arguments:['price']}];
  const [row]=create(source).restrictions('goods');
  assert.equal(row.field,'Цена');
  assert.equal(row.condition,'Равно параметр сеанса ТекущийПользователь');
  assert.equal(row.origin,'шаблон ПоВладельцу');
});

test('a subquery reads its field titles from the object it selects from',()=>{
  const source=fixture();
  source.roles[0].role.objects[0].policies=[{operations:['read'],rule:{field:'owner',operator:'in',
    subquery:{object:'users',field:'person',where:[{field:'ref',operator:'eq',parameter:'ТекущийПользователь'}]}}}];
  const [row]=create(source).restrictions('goods');
  assert.equal(row.condition,'В списке «Физлицо» из «Пользователи», где Ссылка Равно параметр сеанса ТекущийПользователь');
});

test('a broken template is named as broken rather than read as no restriction',()=>{
  const source=fixture();
  source.roles[0].role.objects[0].policies=[{operations:['read'],template:'Нет',arguments:[]}];
  assert.match(create(source).restrictions('goods')[0].error,/не объявлен ролью/);
  source.roles[0].role.policyTemplates=[{name:'Нет',parameters:['Поле'],rule:{field:'$Поле',operator:'eq',values:[{kind:'string',data:'x'}]}}];
  assert.match(create(source).restrictions('goods')[0].error,/ожидает полей: 1/);
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
