import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const script=readFileSync(new URL('../internal/studio/ui/catalog-editor.js',import.meta.url),'utf8');
const context=vm.createContext({structuredClone,crypto});
vm.runInContext(script,context);
const create=context.createCatalogModel;
function fixture(){return {
  catalog:{format:1,id:'cat',name:'Товары',title:{ru:'Товары'},code:{type:'string',length:9,auto:true,unique:true},descriptionLength:250,attributes:[],tableParts:[]},
  typeChoices:{enumerations:[{id:'enum1',name:'ВидыНоменклатуры',title:{ru:'Виды номенклатуры'}}],definedTypes:[],catalogs:[{id:'cat2',name:'Контрагенты',title:{ru:'Контрагенты'}}],documents:[]},
  languages:[{code:'ru',title:'Русский'},{code:'en',title:'English'}],defaultLanguage:'ru',
};}

test('Studio inline script remains valid JavaScript',()=>{
  const html=readFileSync(new URL('../internal/studio/ui/index.html',import.meta.url),'utf8');
  for(const match of html.matchAll(/<script>([\s\S]*?)<\/script>/g))new vm.Script(match[1]);
  assert.match(html,/\/api\/catalog/);assert.match(html,/id="catalog-create-dialog"/);assert.match(html,/id="tree-add"/);
});
test('new attribute gets a unique name and a default single string(50) type',()=>{
  const source=fixture(),model=create(source);
  const first=model.addAttribute('catalog'),second=model.addAttribute('catalog');
  assert.notEqual(first.name,second.name);
  assert.equal(first.types.length,1);assert.equal(first.types[0].kind,'string');assert.equal(first.types[0].length,50);
  assert.equal(model.value().attributes.length,2);
});
test('model mutates source in place but value() returns an independent clone',()=>{
  const source=fixture(),model=create(source);
  model.addAttribute('catalog');
  assert.equal(source.catalog.attributes.length,1);
  const value=model.value();value.attributes.push({});
  assert.equal(source.catalog.attributes.length,1);
});
test('attribute name, title and flags round-trip',()=>{
  const model=create(fixture());
  const attribute=model.addAttribute('catalog');
  model.setAttributeName('catalog',attribute.id,'Артикул');
  model.setAttributeTitle('catalog',attribute.id,'ru','Артикул товара');
  model.setAttributeTitle('catalog',attribute.id,'en','');
  model.setAttributeRequired('catalog',attribute.id,true);
  model.setAttributeIndexing('catalog',attribute.id,'index-with-additional-order');
  const saved=model.value().attributes[0];
  assert.equal(saved.name,'Артикул');assert.deepEqual(saved.title,{ru:'Артикул товара'});
  assert.equal(saved.required,true);assert.equal(saved.indexing,'index-with-additional-order');
});
test('an unindexed field says nothing instead of saying dont-index',()=>{
  const model=create(fixture());
  const attribute=model.addAttribute('catalog');
  model.setAttributeIndexing('catalog',attribute.id,'index');
  model.setAttributeIndexing('catalog',attribute.id,'dont-index');
  assert.equal('indexing' in model.value().attributes[0],false);
});
test('a composite type can be added and the last type cannot be removed',()=>{
  const model=create(fixture());
  const attribute=model.addAttribute('catalog');
  model.addType('catalog',attribute.id);
  assert.equal(model.value().attributes[0].types.length,2);
  model.removeType('catalog',attribute.id,0);
  assert.equal(model.value().attributes[0].types.length,1);
  model.removeType('catalog',attribute.id,0);
  assert.equal(model.value().attributes[0].types.length,1);
});
test('switching type kind replaces incompatible fields with sensible defaults',()=>{
  const model=create(fixture());
  const attribute=model.addAttribute('catalog');
  model.setTypeKind('catalog',attribute.id,0,'number');
  assert.deepEqual(model.value().attributes[0].types[0],{kind:'number',precision:15,scale:2});
  model.setTypeKind('catalog',attribute.id,0,'catalog');
  assert.deepEqual(model.value().attributes[0].types[0],{kind:'catalog',reference:'cat2'});
});
test('table parts own their own attribute list, addressed by container id',()=>{
  const model=create(fixture());
  const part=model.addTablePart();
  const attribute=model.addAttribute(part.id);
  assert.equal(model.value().tableParts[0].attributes.length,1);
  assert.equal(model.value().attributes.length,0);
  model.setTablePartName(part.id,'Товары');
  model.setTablePartTitle(part.id,'ru','Товары в документе');
  const saved=model.value().tableParts[0];
  assert.equal(saved.name,'Товары');assert.deepEqual(saved.title,{ru:'Товары в документе'});
  model.removeAttribute(part.id,attribute.id);
  assert.equal(model.value().tableParts[0].attributes.length,0);
});
test('removing a table part removes its attributes with it',()=>{
  const model=create(fixture());
  const part=model.addTablePart();
  model.addAttribute(part.id);
  model.removeTablePart(part.id);
  assert.equal(model.value().tableParts.length,0);
});
test('object-level name, title and code settings round-trip',()=>{
  const model=create(fixture());
  model.setName('Номенклатура');model.setTitle('en','Items');model.setTitle('ru','');
  model.setCodeType('number');model.setCodeLength(11);model.setCodeAuto(false);model.setCodeUnique(false);model.setDescriptionLength(500);
  const saved=model.value();
  assert.equal(saved.name,'Номенклатура');assert.deepEqual(saved.title,{en:'Items'});
  assert.deepEqual(saved.code,{type:'number',length:11,auto:false,unique:false});
  assert.equal(saved.descriptionLength,500);
});
