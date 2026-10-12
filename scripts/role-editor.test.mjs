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
function fixture(){return {role:{id:'role',name:'Продавец',title:{ru:'Продавец'}},schema:{rights:[{key:'read',title:'Чтение'},{key:'insert',title:'Добавление'},{key:'update',title:'Изменение'},{key:'delete',title:'Удаление'},{key:'view',title:'Просмотр'},{key:'edit',title:'Редактирование'},{key:'posting',title:'Проведение'}],objects:[{id:'base',root:true,rights:['administration','thin-client'],operations:[],fields:[],policyFields:[]},{id:'goods',rights:['read','insert','update','delete','view','edit','input-by-string'],operations:['read','insert','update','delete'],fields:[{key:'ref',operations:['view']},{key:'name',operations:['view','edit']},{key:'part',operations:['view','edit']},{key:'column',parent:'part',operations:['view','edit']}],policyFields:[{key:'ref'},{key:'name'},{key:'part'},{key:'column',parent:'part'}]},{id:'other',rights:['read','view'],operations:['read'],fields:[],policyFields:[]}],parts:{base:[],goods:['column','name','part','ref'],other:[]},forms:[{id:'form',commands:[{id:'command'}]}]}};}

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
test('one object checkbox grants existing fields including table part columns',()=>{
  const model=create(fixture());model.setObject('goods','read',true);
  for(const key of ['ref','name','part','column'])assert.equal(model.hasField('goods',key,'view'),true);
  assert.equal(model.hasObject('other','read'),false);assert.equal(model.hasCommand('form','command'),false);
  model.setObject('goods','update',true);assert.equal(model.hasField('goods','name','edit'),true);assert.equal(model.hasField('goods','ref','edit'),false);
});
test('insert and update share field editing, removing both clears it',()=>{
  const model=create(fixture());model.setObject('goods','insert',true);model.setObject('goods','update',true);
  model.setObject('goods','insert',false);assert.equal(model.hasField('goods','name','edit'),true);
  model.setObject('goods','update',false);assert.equal(model.hasField('goods','name','edit'),false);
  const goods=model.value().objects[0];assert.deepEqual([...goods.operations],[]);assert.deepEqual([...goods.denied],['insert','update']);assert.equal(goods.fields.length,0);
});
test('field refinement stays separate and table part changes include children',()=>{
  const model=create(fixture());model.setObject('goods','read',true);model.setField('goods','name','view',false);
  assert.equal(model.hasObject('goods','read'),true);assert.equal(model.hasField('goods','column','view'),true);
  model.setField('goods','part','view',false);assert.equal(model.hasField('goods','column','view'),false);
  model.setField('goods','column','view',true);assert.equal(model.hasField('goods','part','view'),true);
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
  model.setObject('goods','update',true);assert.equal(model.hasField('goods','name','edit'),true);
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
  const model=create(fixture());model.setObject('goods','admin',true);model.setObject('missing','read',true);model.setField('goods','ref','edit',true);model.setField('goods','missing','view',true);
  assert.equal(model.value().objects.length,0);
});
test('bulk edits remain bounded for a large object',()=>{
  const source=fixture();source.schema.objects[1].fields=Array.from({length:9000},(_,index)=>({key:`field${index}`,operations:['view','edit']}));
  const model=create(source);model.setAll('goods',true);assert.equal(model.value().objects[0].fields.length,9000);
  model.setAll('goods',false);assert.equal(model.value().objects[0].fields.length,0);
});
test('access policies round-trip and survive cleaning even without operation grants',()=>{
  const model=create(fixture());
  model.addPolicy('goods',{operations:['read'],rule:{field:'name',operator:'in',values:[{kind:'string',data:'Основной'},{kind:'string',data:'Розничный'}]}});
  const value=model.value();
  assert.equal(value.objects.length,1);
  assert.equal(value.objects[0].policies.length,1);
  assert.equal(value.objects[0].policies[0].rule.operator,'in');
  assert.equal(value.objects[0].policies[0].rule.values.length,2);
  assert.equal(model.hasUnavailable(),false);
});
test('named templates are reusable and removing one removes the restrictions using it',()=>{
  const model=create(fixture());
  assert.equal(model.addTemplate('ПоСкладу',{field:'name',operator:'eq',parameter:'ТекущийСклад'}),true);
  assert.equal(model.addTemplate('ПоСкладу',{field:'name',operator:'eq',values:[]}),false,'duplicate name');
  model.addPolicy('goods',{operations:['read'],template:'ПоСкладу'});
  model.addPolicy('goods',{operations:['update'],template:'ПоСкладу'});
  assert.equal(model.policies('goods').length,2);
  model.removeTemplate('ПоСкладу');
  assert.equal(model.templates().length,0);
  assert.equal(model.policies('goods').length,0,'restrictions referencing a removed template must not survive');
});
test('policies on fields that no longer exist are reported and repaired',()=>{
  const source=fixture();
  source.role.objects=[{object:'goods',operations:['read'],fields:[],policies:[
    {operations:['read'],rule:{field:'name',operator:'eq',values:[{kind:'string',data:'x'}]}},
    {operations:['read'],rule:{field:'gone',operator:'eq',values:[{kind:'string',data:'x'}]}}]}];
  const model=create(source);
  assert.equal(model.hasUnavailable(),true);
  model.removeUnavailable();
  assert.equal(model.hasUnavailable(),false);
  const policies=model.policies('goods');
  assert.equal(policies.length,1);
  assert.equal(policies[0].rule.field,'name');
});
test('a restriction naming a template that was never declared is repaired away',()=>{
  const source=fixture();
  source.role.objects=[{object:'goods',operations:['read'],fields:[],policies:[{operations:['read'],template:'Отсутствует'}]}];
  const model=create(source);
  assert.equal(model.hasUnavailable(),true);
  model.removeUnavailable();
  assert.equal(model.policies('goods').length,0);
  assert.equal(model.value().objects.length,1,'the read grant itself stays');
});
test('a template with parameters is reusable and its arguments are checked on repair',()=>{
  const model=create(fixture());
  assert.equal(model.addTemplate('Своё',['Поле'],{field:'$Поле',operator:'eq',currentMlUser:true}),true);
  model.addPolicy('goods',{operations:['read'],template:'Своё',arguments:['name']});
  assert.equal(model.hasUnavailable(),false,'a template used with a real field must survive');
  model.setPolicy('goods',0,{operations:['read'],template:'Своё',arguments:['gone']});
  assert.equal(model.hasUnavailable(),true,'an argument naming a missing field must be reported');
  model.removeUnavailable();
  assert.equal(model.policies('goods').length,0);
});
test('a template invoked with the wrong number of fields is repaired away',()=>{
  const model=create(fixture());
  model.addTemplate('Своё',['Поле'],{field:'$Поле',operator:'eq',parameter:'П'});
  model.addPolicy('goods',{operations:['read'],template:'Своё'});
  assert.equal(model.hasUnavailable(),true,'a missing argument must be reported');
  model.removeUnavailable();
  assert.equal(model.policies('goods').length,0);
});
test('a subquery survives while the table it reads exists and is repaired away when it does not',()=>{
  const model=create(fixture());
  model.addPolicy('goods',{operations:['read'],rule:{field:'name',operator:'in',
    subquery:{object:'goods',field:'ref',where:[{field:'name',operator:'eq',parameter:'ТекущийПользователь'}]}}});
  assert.equal(model.hasUnavailable(),false);
  model.setPolicy('goods',0,{operations:['read'],rule:{field:'name',operator:'in',
    subquery:{object:'отсутствует',field:'ref',where:[]}}});
  assert.equal(model.hasUnavailable(),true,'a subquery over a removed object must be reported');
  model.removeUnavailable();
  assert.equal(model.policies('goods').length,0);
});
test('comment and default-grant flags round-trip through the model; only the flag for new objects writes what it stood for',()=>{
  const model=create(fixture());
  assert.equal(model.value().comment,undefined);assert.equal(model.value().grantNewObjectsByDefault,undefined);assert.equal(model.value().grantNewFieldsByDefault,undefined);assert.equal(model.value().independentChildRights,undefined);
  model.setComment('Только чтение справочников склада');model.setGrantNewObjectsByDefault(true);model.setGrantNewFieldsByDefault(true);model.setIndependentChildRights(true);
  const value=model.value();
  assert.equal(value.comment,'Только чтение справочников склада');assert.equal(value.grantNewObjectsByDefault,true);assert.equal(value.grantNewFieldsByDefault,true);assert.equal(value.independentChildRights,true);
  assert.deepEqual(value.objects.map(item=>`${item.object}:${item.denied.join(',')}:${item.fields.length}`),
    ['base:administration,thin-client:0','goods:read,insert,update,delete,view,edit,input-by-string:0','other:read,view:0']);
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
