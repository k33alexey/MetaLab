import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const script=readFileSync(new URL('../internal/studio/ui/role-editor.js',import.meta.url),'utf8');
const context=vm.createContext({structuredClone});
vm.runInContext(script,context);
const create=context.createRoleModel;
function fixture(){return {role:{id:'role',name:'Продавец',title:{ru:'Продавец'}},schema:{objects:[{id:'goods',operations:['read','create','update','delete'],fields:[{key:'ref',operations:['read']},{key:'name',operations:['read','update']},{key:'part',operations:['read','update']},{key:'column',parent:'part',operations:['read','update']}]},{id:'other',operations:['read'],fields:[]}],forms:[{id:'form',commands:[{id:'command'}]}]}};}

test('Studio inline script remains valid JavaScript',()=>{
  const html=readFileSync(new URL('../internal/studio/ui/index.html',import.meta.url),'utf8');
  for(const match of html.matchAll(/<script>([\s\S]*?)<\/script>/g))new vm.Script(match[1]);
  assert.match(html,/\/api\/role/);assert.match(html,/id="role-create"/);
});
test('empty role has no implicit grants and model never mutates input',()=>{
  const source=fixture(),model=create(source);assert.equal(model.hasObject('goods','read'),false);
  model.setObject('goods','read',true);assert.equal(source.role.objects,undefined);
  const value=model.value();value.objects[0].operations=[];assert.equal(model.hasObject('goods','read'),true);
});
test('one object checkbox grants existing fields including table part columns',()=>{
  const model=create(fixture());model.setObject('goods','read',true);
  for(const key of ['ref','name','part','column'])assert.equal(model.hasField('goods',key,'read'),true);
  assert.equal(model.hasObject('other','read'),false);assert.equal(model.hasCommand('form','command'),false);
  model.setObject('goods','update',true);assert.equal(model.hasField('goods','name','update'),true);assert.equal(model.hasField('goods','ref','update'),false);
});
test('create and update share field editing, removing both clears it',()=>{
  const model=create(fixture());model.setObject('goods','create',true);model.setObject('goods','update',true);
  model.setObject('goods','create',false);assert.equal(model.hasField('goods','name','update'),true);
  model.setObject('goods','update',false);assert.equal(model.hasField('goods','name','update'),false);assert.equal(model.value().objects.length,0);
});
test('field refinement stays separate and table part changes include children',()=>{
  const model=create(fixture());model.setObject('goods','read',true);model.setField('goods','name','read',false);
  assert.equal(model.hasObject('goods','read'),true);assert.equal(model.hasField('goods','column','read'),true);
  model.setField('goods','part','read',false);assert.equal(model.hasField('goods','column','read'),false);
  model.setField('goods','column','read',true);assert.equal(model.hasField('goods','part','read'),true);
});
test('all grants and reset apply only to the selected object',()=>{
  const model=create(fixture());model.setObject('other','read',true);model.setAll('goods',true);
  for(const op of ['read','create','update','delete'])assert.equal(model.hasObject('goods',op),true);
  model.setAll('goods',false);assert.equal(model.hasObject('other','read'),true);assert.equal(model.value().objects.length,1);
});
test('unknown grants survive ordinary editing and are repaired only explicitly',()=>{
  const source=fixture();source.role.objects=[{object:'gone',operations:['read'],fields:[]},{object:'goods',operations:['read','post'],fields:[{field:'gone',operations:['read']},{field:'ref',operations:['read','update']}]}];source.role.commands=[{form:'form',command:'gone'}];
  const model=create(source);assert.equal(model.hasUnavailable(),true);model.setName('НовоеИмя');assert.equal(model.value().objects.length,2);
  model.removeUnavailable();assert.equal(model.hasUnavailable(),false);assert.equal(model.value().objects.length,1);assert.equal(model.hasObject('goods','read'),true);assert.equal(model.hasField('goods','ref','read'),true);assert.equal(model.hasField('goods','ref','update'),false);
  model.setObject('goods','update',true);assert.equal(model.hasField('goods','name','update'),true);
});
test('commands are scoped and never grant data access',()=>{
  const model=create(fixture());model.setCommand('form','command',true);model.setCommand('other','command',true);
  assert.equal(model.value().commands.length,1);assert.equal(model.value().objects.length,0);
  model.setCommand('form','command',false);assert.equal(model.value().commands.length,0);
});
test('unsupported operations and fields cannot be granted through controls',()=>{
  const model=create(fixture());model.setObject('goods','admin',true);model.setObject('missing','read',true);model.setField('goods','ref','update',true);model.setField('goods','missing','read',true);
  assert.equal(model.value().objects.length,0);
});
test('bulk edits remain bounded for a large object',()=>{
  const source=fixture();source.schema.objects[0].fields=Array.from({length:9000},(_,index)=>({key:`field${index}`,operations:['read','update']}));
  const model=create(source);model.setAll('goods',true);assert.equal(model.value().objects[0].fields.length,9000);
  model.setAll('goods',false);assert.equal(model.value().objects.length,0);
});
