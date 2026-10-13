import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const script=readFileSync(new URL('../internal/studio/ui/role-editor.js',import.meta.url),'utf8');
const context=vm.createContext({structuredClone});
// Редактор опирается на общие помощники Studio, как в index.html, где
// studio-common.js подключается первым.
vm.runInContext(readFileSync(new URL('../internal/studio/ui/studio-common.js',import.meta.url),'utf8'),context);
vm.runInContext(script,context);
const create=context.createRoleModel;
const plain=value=>JSON.parse(JSON.stringify(value));
function fixture(){return {role:{id:'role',name:'Продавец',title:{ru:'Продавец'}},schema:{rights:[{key:'read',title:'Чтение'},{key:'insert',title:'Добавление'},{key:'update',title:'Изменение'},{key:'delete',title:'Удаление'},{key:'view',title:'Просмотр'},{key:'edit',title:'Редактирование'},{key:'posting',title:'Проведение'}],objects:[{id:'base',root:true,rights:['administration','thin-client'],operations:[],fields:[],restrictionFields:[]},{id:'goods',rights:['read','insert','update','delete','view','edit','input-by-string'],restrictedRights:['read','insert','update','delete'],operations:['read','insert','update','delete'],fields:[{key:'ref',group:'standard-attributes',operations:['view','edit']},{key:'name',group:'attributes',operations:['view','edit']},{key:'part',group:'table-parts',operations:['view','edit']},{key:'column',group:'attributes',parent:'part',operations:['view','edit']},{key:'print',group:'commands',operations:['view']}],restrictionFields:[{key:'ref'},{key:'dataversion'},{key:'name'},{key:'part'},{key:'column',parent:'part'},{key:'part.linenumber',parent:'part'}]},{id:'other',rights:['read','view'],operations:['read'],fields:[],restrictionFields:[]}],parts:{base:[],goods:['column','name','part','print','ref'],other:[]},forms:[{id:'form',commands:[{id:'command'}]}]}};}

test('Studio inline script remains valid JavaScript',()=>{
  const html=readFileSync(new URL('../internal/studio/ui/index.html',import.meta.url),'utf8');
  for(const match of html.matchAll(/<script>([\s\S]*?)<\/script>/g))new vm.Script(match[1]);
  assert.match(html,/\/api\/role/);assert.match(html,/id="role-create-dialog"/);assert.match(html,/id="tree-add"/);
});
test('empty role has no implicit grants and model never mutates input',()=>{
  const source=fixture(),model=create(source);assert.equal(model.hasObject('goods','read'),false);
  model.setObject('goods','read',true);assert.equal(source.role.objects,undefined);
  const value=model.value();value.objects[0].operations=[];assert.equal(model.hasObject('goods','read'),true);
});
// Дефект: отметка права объекта проставляла права на все его реквизиты; у
// прототипа права на реквизиты ставятся вместе только по вопросу при смене
// флага «для реквизитов» (решение владельца 12.10.2026).
test('marking an object gives its parts nothing',()=>{
  const model=create(fixture());model.setObject('goods','read',true);model.setObject('goods','update',true);
  for(const key of ['ref','name','part','column'])for(const right of ['view','edit'])assert.equal(model.hasField('goods',key,right),false,`${key} ${right}`);
  assert.equal(model.hasField('goods','print','view'),false);assert.equal(model.value().objects[0].fields.length,0);
  assert.equal(model.hasObject('other','read'),false);assert.equal(model.hasCommand('form','command'),false);
});
// Дефект: снятая галочка части только убирала разрешение - у прототипа у
// права части два состояния, как у права объекта, и снятое пишется запретом.
test('a part keeps its rights in two states, written apart from the object and the other parts',()=>{
  const model=create(fixture());model.setField('goods','name','view',true);model.setField('goods','name','edit',false);
  assert.equal(model.hasField('goods','name','view'),true);assert.equal(model.hasField('goods','name','edit'),false);
  const name=model.value().objects[0].fields.find(field=>field.field==='name');
  assert.deepEqual([...name.operations],['view']);assert.deepEqual([...name.denied],['edit']);
  assert.equal(model.hasObject('goods','read'),false);assert.equal(model.hasField('goods','ref','view'),false);
  model.setField('goods','print','view',true);assert.equal(model.hasField('goods','print','view'),true);
});
// Дефект: отметка табличной части проставляла права её колонкам, а отметка
// колонки - самой части; у прототипа права подчинённого связаны с правами
// владельца флагом независимых прав при исполнении, а не правкой.
test('a table part and its columns are marked each on its own',()=>{
  const model=create(fixture());model.setField('goods','part','view',true);
  assert.equal(model.hasField('goods','column','view'),false);
  model.setField('goods','column','edit',true);model.setField('goods','part','view',false);
  assert.equal(model.hasField('goods','column','edit'),true);assert.equal(model.hasField('goods','part','edit'),false);
});
test('all grants and reset apply only to the selected object',()=>{
  const model=create(fixture());model.setObject('other','read',true);model.setAll('goods',true);
  for(const op of ['read','insert','update','delete'])assert.equal(model.hasObject('goods',op),true);
  model.setAll('goods',false);assert.equal(model.hasObject('other','read'),true);
  const goods=model.value().objects.find(item=>item.object==='goods');
  assert.deepEqual([...goods.denied],['read','insert','update','delete','view','edit','input-by-string']);assert.deepEqual([...goods.operations],[]);
});
test('unknown grants survive ordinary editing and are repaired only explicitly',()=>{
  const source=fixture();source.role.objects=[{object:'gone',operations:['read'],fields:[]},{object:'goods',operations:['read','posting'],denied:['delete'],fields:[{field:'gone',operations:['view']},{field:'ref',operations:['view','edit']},{field:'name',denied:['view']}]}];source.role.commands=[{form:'form',command:'gone'}];
  const model=create(source);assert.equal(model.hasUnavailable(),true);model.setName('НовоеИмя');assert.equal(model.value().objects.length,2);
  model.removeUnavailable();assert.equal(model.hasUnavailable(),false);assert.equal(model.value().objects.length,1);assert.equal(model.hasObject('goods','read'),true);assert.equal(model.hasField('goods','ref','view'),true);
  // Repair removes what the configuration lost, never a right: posting on an
  // object that is never posted, editing a field the editor shows read-only,
  // and every denial stay as the prototype keeps them.
  const goods=model.value().objects[0];
  assert.deepEqual([...goods.operations],['read','posting']);assert.deepEqual([...goods.denied],['delete']);
  assert.equal(model.hasField('goods','ref','edit'),true);assert.deepEqual([...goods.fields.find(field=>field.field==='name').denied],['view']);
  assert.equal(goods.fields.some(field=>field.field==='gone'),false);
  model.setObject('goods','update',true);assert.equal(model.hasField('goods','name','edit'),false);assert.equal(model.hasField('goods','name','view'),false);
});
test('granting a denied right takes the denial back, so a right has one state',()=>{
  const source=fixture();source.role.objects=[{object:'goods',denied:['read'],fields:[{field:'name',denied:['view']}]}];
  const model=create(source);assert.equal(model.hasObject('goods','read'),false);
  model.setField('goods','name','view',true);
  const name=model.value().objects[0].fields.find(field=>field.field==='name');
  assert.deepEqual([...name.operations],['view']);assert.equal(name.denied,undefined);
  model.setObject('goods','read',true);assert.equal(model.value().objects[0].denied,undefined);
});
test('a part only denied is kept when the role is cleaned',()=>{
  const source=fixture();source.role.objects=[{object:'goods',fields:[{field:'name',denied:['view']}]}];
  assert.equal(create(source).value().objects[0].fields.length,1);
});
test('commands are scoped and never grant data access',()=>{
  const model=create(fixture());model.setCommand('form','command',true);model.setCommand('other','command',true);
  assert.equal(model.value().commands.length,1);assert.equal(model.value().objects.length,0);
  model.setCommand('form','command',false);assert.equal(model.value().commands.length,0);
});
test('unsupported operations and fields cannot be granted through controls',()=>{
  const model=create(fixture());model.setObject('goods','admin',true);model.setObject('missing','read',true);model.setField('goods','print','edit',true);model.setField('goods','missing','view',true);
  assert.equal(model.value().objects.length,0);
});
// Дефект: смена флага «для реквизитов» на объекте с тысячами реквизитов
// искала каждую запись перебором и висла.
test('bulk edits remain bounded for a large object',()=>{
  const source=fixture();source.schema.objects[1].fields=Array.from({length:9000},(_,index)=>({key:`field${index}`,group:'attributes',operations:['view','edit']}));
  const model=create(source),started=Date.now();model.setGrantNewFieldsByDefault(true,true);
  assert.equal(model.value().objects[0].fields.length,9000);assert.equal(model.hasField('goods','field8999','edit'),true);
  model.setGrantNewFieldsByDefault(false,true);assert.equal(model.value().objects[0].fields.length,9000);assert.equal(model.hasField('goods','field0','view'),false);
  assert.ok(Date.now()-started<2000,`took ${Date.now()-started} ms`);
});
// What the editor is not asked to change it keeps. Defect caught: a save from the editor dropping a role's
// restrictions or templates, and a right cleared with its restriction lost.
test('restrictions and templates survive editing as written',()=>{
  const source=fixture();
  source.role.restrictionTemplates=[{name:'ПоЗначениям(Таблица)',condition:'ГДЕ #Параметр(1)'}];
  source.role.objects=[{object:'goods',operations:['read','update'],fields:[],restrictions:[
    {right:'read',fields:['ref','dataversion']},{right:'read',condition:'#ПоЗначениям("Склад")'},{right:'update',condition:'ГДЕ ЛОЖЬ'}]}];
  const model=create(source);
  model.setObject('goods','read',false);
  model.setObject('goods','insert',true);
  const value=plain(model.value());
  assert.deepEqual(value.restrictionTemplates,source.role.restrictionTemplates);
  assert.deepEqual(value.objects[0].restrictions,source.role.objects[0].restrictions);
  assert.deepEqual(value.objects[0].denied,['read'],'a cleared right is denied and keeps its restriction');
  assert.equal(model.hasUnavailable(),false);
});
// Defect caught: a restriction naming a field the object lost kept on, so the
// role could not be saved; or, emptied of fields, turned into a restriction
// of every other field.
test('a restriction loses a field the object no longer has, and goes when none is left',()=>{
  const source=fixture();
  source.role.objects=[{object:'goods',operations:['read'],fields:[],restrictions:[
    {right:'read',fields:['ref','gone']},{right:'read',fields:['lost'],condition:'ГДЕ ЛОЖЬ'},{right:'read',condition:'ГДЕ ИСТИНА'}]}];
  const model=create(source);
  assert.equal(model.hasUnavailable(),true);
  model.removeUnavailable();
  assert.equal(model.hasUnavailable(),false);
  assert.deepEqual(plain(model.value().objects[0].restrictions),[{right:'read',fields:['ref']},{right:'read',condition:'ГДЕ ИСТИНА'}]);
});
test('comment and default-grant flags round-trip through the model; a flag writes what it stood for',()=>{
  const model=create(fixture());
  assert.equal(model.value().comment,undefined);assert.equal(model.value().grantNewObjectsByDefault,undefined);assert.equal(model.value().grantNewFieldsByDefault,undefined);assert.equal(model.value().independentChildRights,undefined);
  model.setComment('Только чтение справочников склада');model.setGrantNewObjectsByDefault(true);model.setGrantNewFieldsByDefault(true);model.setIndependentChildRights(true);
  const value=model.value();
  assert.equal(value.comment,'Только чтение справочников склада');assert.equal(value.grantNewObjectsByDefault,true);assert.equal(value.grantNewFieldsByDefault,true);assert.equal(value.independentChildRights,true);
  assert.deepEqual(value.objects.map(item=>`${item.object}:${item.denied.join(',')}:${item.fields.length}`),
    ['base:administration,thin-client:0','goods:read,insert,update,delete,view,edit,input-by-string:1','other:read,view:0']);
  // The command stood for the flag for new objects too, and is written as it stood.
  assert.deepEqual([...value.objects[1].fields[0].denied],['view']);assert.equal(value.objects[1].fields[0].field,'print');
  model.setGrantNewObjectsByDefault(false);assert.equal(model.value().grantNewObjectsByDefault,false);
});

// Дефект: редактор показывал неуказанное право невыданным и у роли с флагом
// «для новых объектов», где прототип читает его выданным (ПолныеПрава erp не
// пишет «Чтение» ни у одного из 565 справочников, а читает их все).
test('a right the role does not write is the flag for new objects, as the prototype reads it',()=>{
  const source=fixture();source.role.grantNewObjectsByDefault=true;source.role.objects=[{object:'goods',denied:['delete']}];
  const model=create(source);
  assert.equal(model.hasObject('goods','read'),true);assert.equal(model.hasObject('other','view'),true);assert.equal(model.hasObject('base','administration'),true);
  assert.equal(model.hasObject('goods','delete'),false,'a denial stays a denial');
});
// Дефект: снятая галочка только убирала разрешение, и при флаге «для новых
// объектов» право оставалось выданным - у прототипа два состояния, и
// снятое пишется запретом.
test('clearing a box denies the right in so many words',()=>{
  const source=fixture();source.role.grantNewObjectsByDefault=true;
  const model=create(source);model.setObject('goods','read',false);
  assert.equal(model.hasObject('goods','read'),false);
  assert.deepEqual([...model.value().objects[0].denied],['read']);
  model.setObject('goods','read',true);assert.equal(model.value().objects[0].denied,undefined);assert.deepEqual([...model.value().objects[0].operations],['read']);
});
// Дефект: смена флага «для новых объектов» молча переворачивала все
// незаписанные права существующих объектов; по справке флаг решает только,
// что получит новый объект.
test('changing the flag for new objects turns over no right of an existing object',()=>{
  const source=fixture();source.role.objects=[{object:'goods',operations:['read'],denied:['delete']}];
  const model=create(source);const before=()=>source.schema.objects.flatMap(item=>item.rights.map(right=>`${item.id}.${right}=${model.hasObject(item.id,right)}`));
  const was=before();
  model.setGrantNewObjectsByDefault(true);assert.deepEqual(before(),was);assert.equal(model.value().grantNewObjectsByDefault,true);
  model.setGrantNewObjectsByDefault(false);assert.deepEqual(before(),was);
  model.setGrantNewObjectsByDefault(false);assert.deepEqual(before(),was,'setting the flag it already has changes nothing');
});
// Дефект: у корня конфигурации и у объектов без прав, которые исполняет
// платформа, редактор не давал ничего отметить; право не из перечня вида
// не принимается.
test('the configuration and every right of the kind are offered, a right outside the kind is not',()=>{
  const model=create(fixture());
  model.setObject('base','administration',true);model.setObject('goods','input-by-string',true);
  assert.equal(model.hasObject('base','administration'),true);assert.equal(model.hasObject('goods','input-by-string'),true);
  model.setObject('base','read',true);model.setObject('other','input-by-string',true);
  assert.equal(model.value().objects.find(item=>item.object==='base').operations.includes('read'),false);
  assert.equal(model.value().objects.some(item=>item.object==='other'),false);
});
// Дефект: незаписанное право части читалось невыданным при любых флагах.
// По выгрузкам: реквизит, не записанный в роли, при флаге «для реквизитов…»
// имеет право объекта (443 тыс. записанных запретов), без флага - нет;
// команды, операции и методы роли с флагом «для новых объектов» пишут
// только запреты, без флага - только разрешения.
test('a right the role does not write on a part is read by the flags, as the exports show',()=>{
  const source=fixture();source.role.objects=[{object:'goods',operations:['view'],denied:['edit']}];
  let model=create(source);
  assert.equal(model.hasField('goods','name','view'),false,'without the flag for attributes a field has no right');
  assert.equal(model.hasField('goods','print','view'),false,'without the flag for new objects a command has no right');
  source.role.grantNewFieldsByDefault=true;model=create(source);
  assert.equal(model.hasField('goods','name','view'),true,'the object gives View');assert.equal(model.hasField('goods','column','edit'),false,'the object denies Edit');
  assert.equal(model.hasField('goods','print','view'),false,'a command does not follow the flag for attributes');
  source.role.grantNewObjectsByDefault=true;source.role.grantNewFieldsByDefault=false;model=create(source);
  assert.equal(model.hasField('goods','print','view'),true);assert.equal(model.hasField('goods','name','view'),false);
});
// Дефект: ответ «Нет» на вопрос при смене флага «для реквизитов» молча
// переворачивал права реквизитов, которые флаг определял; по справке «Нет»
// не меняет прав.
test('the flag for attributes answered «Нет» changes no right in force',()=>{
  const source=fixture();source.role.objects=[{object:'goods',operations:['read','view'],fields:[{field:'name',denied:['view']}]}];
  const model=create(source);
  const all=()=>source.schema.objects.flatMap(item=>[...item.rights.map(right=>`${item.id}.${right}=${model.hasObject(item.id,right)}`),
    ...item.fields.flatMap(part=>part.operations.map(right=>`${item.id}.${part.key}.${right}=${model.hasField(item.id,part.key,right)}`))]);
  const was=all();
  model.setGrantNewFieldsByDefault(true,false);assert.deepEqual(all(),was);assert.equal(model.value().grantNewFieldsByDefault,true);
  model.setGrantNewFieldsByDefault(false,false);assert.deepEqual(all(),was);
  model.setGrantNewObjectsByDefault(true);assert.deepEqual(all(),was,'nor does the flag for new objects, parts included');
});
// Дефект: ответ «Да» не ставил прав на реквизиты или ставил их и командам;
// по справке «Да» ставит или снимает права на все реквизиты и табличные части.
test('the flag for attributes answered «Да» sets or clears every field and table part, and nothing else',()=>{
  const model=create(fixture());model.setGrantNewFieldsByDefault(true,true);
  for(const key of ['ref','name','part','column'])for(const right of ['view','edit'])assert.equal(model.hasField('goods',key,right),true,`${key} ${right}`);
  assert.equal(model.hasField('goods','print','view'),false,'a command is no attribute');assert.equal(model.hasObject('goods','read'),false);
  model.setGrantNewFieldsByDefault(false,true);
  for(const key of ['ref','name','part','column'])assert.equal(model.hasField('goods',key,'view'),false,key);
});

// A restriction is written inside its right, granted or denied, as the
// prototype writes it. Defect caught: a restriction added on a right the role
// does not write - one that stands for the role's flag - left a role the
// server refuses to save («restricts a right the role neither grants nor
// denies»), or wrote the right granted where the flag had it denied.
test('a restriction added on an unwritten right writes the right as it stood',()=>{
  for(const flag of [false,true]){
    const source=fixture();source.role.grantNewObjectsByDefault=flag;
    const model=create(source);
    model.putRestriction('goods','insert',0,{condition:'ГДЕ ИСТИНА'});
    const goods=plain(model.value()).objects.find(item=>item.object==='goods');
    assert.deepEqual(flag?goods.operations:goods.denied,['insert'],`flag ${flag}`);
    assert.equal(model.hasObject('goods','insert'),flag,'the right in force does not turn over');
    assert.deepEqual(goods.restrictions,[{right:'insert',condition:'ГДЕ ИСТИНА'}]);
  }
});
// Defect caught: fields kept on a right other than reading, a field the
// object does not have, the same field twice; a restriction on an object
// that keeps no data.
test('only reading is restricted by fields, and only by the object\'s own',()=>{
  const model=create(fixture());
  model.putRestriction('goods','read',0,{fields:['ref','gone','ref','part.linenumber'],condition:'ГДЕ 1'});
  model.putRestriction('goods','update',0,{fields:['ref'],condition:'ГДЕ 2'});
  model.putRestriction('other','read',0,{condition:'ГДЕ 3'});
  model.putRestriction('base','read',0,{condition:'ГДЕ 4'});
  const value=plain(model.value());
  assert.deepEqual(value.objects.find(item=>item.object==='goods').restrictions,[{right:'read',fields:['ref','part.linenumber'],condition:'ГДЕ 1'},{right:'update',condition:'ГДЕ 2'}]);
  assert.equal(value.objects.some(item=>item.object==='other'||item.object==='base'),false);
});
// The list of a right is numbered within the right. Defect caught: the
// change or the removal of the second restriction of reading touched a
// restriction of another right that stood between them in the role.
test('a restriction is changed and removed by its place among those of its right',()=>{
  const source=fixture();
  source.role.objects=[{object:'goods',operations:['read','update'],fields:[],restrictions:[
    {right:'read',fields:['ref'],condition:'А'},{right:'update',condition:'Б'},{right:'read',condition:'В'}]}];
  const model=create(source);
  assert.deepEqual(plain(model.restrictions('goods','read')).map(item=>item.condition),['А','В']);
  model.putRestriction('goods','read',1,{condition:'Г'});
  assert.deepEqual(plain(model.value().objects[0].restrictions).map(item=>item.condition),['А','Б','Г']);
  model.removeRestriction('goods','read',1);model.removeRestriction('goods','read',5);
  assert.deepEqual(plain(model.value().objects[0].restrictions).map(item=>item.condition),['А','Б']);
  model.removeRestriction('goods','read',0);model.removeRestriction('goods','update',0);
  assert.equal(model.value().objects[0].restrictions,undefined,'no empty list is written');
  assert.deepEqual(plain(model.value().objects[0].operations),['read','update'],'removing a restriction keeps the right');
});
// Defect caught: a template saved with the blanks around its name, an empty
// list written after the last template goes, a change landing on another.
test('templates are added, changed and removed in place',()=>{
  const model=create(fixture());
  model.putTemplate(0,{name:' Первый ',condition:'ГДЕ 1'});
  assert.deepEqual(plain(model.templates()),[{name:'Первый',condition:'ГДЕ 1'}]);
  model.putTemplate(1,{name:'Второй(Поле)',condition:''});
  model.putTemplate(0,{name:'Первый',condition:'ГДЕ 2'});
  assert.deepEqual(plain(model.value().restrictionTemplates),[{name:'Первый',condition:'ГДЕ 2'},{name:'Второй(Поле)'}]);
  model.removeTemplate(0);
  assert.deepEqual(plain(model.templates()),[{name:'Второй(Поле)'}]);
  model.removeTemplate(0);
  assert.equal(model.value().restrictionTemplates,undefined);
});
