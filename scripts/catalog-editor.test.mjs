import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const script=readFileSync(new URL('../internal/studio/ui/catalog-editor.js',import.meta.url),'utf8');
const context=vm.createContext({structuredClone,crypto});
// Редактор опирается на общие помощники Studio, как в index.html, где
// studio-common.js подключается первым.
vm.runInContext(readFileSync(new URL('../internal/studio/ui/studio-common.js',import.meta.url),'utf8'),context);
vm.runInContext(script,context);
const create=context.createCatalogModel;
function fixture(){return {
  catalog:{format:1,id:'cat',name:'Товары',title:{ru:'Товары'},code:{type:'string',length:9,auto:true,unique:true},descriptionLength:250,attributes:[],tableParts:[]},
  typeOffer:{place:'stored-field',required:true,limits:{stringLength:1024,fixedStringLength:100,numberDigits:32},
    newTypes:{string:{kind:'string',length:10},number:{kind:'number',precision:10},date:{kind:'date',dateParts:'date'}},
    rows:[{kind:'number',name:{russian:'Число'}},{kind:'string',name:{russian:'Строка'}},
      {kind:'catalog-ref',name:{russian:'СправочникСсылка'},objectKind:'catalog',objects:[{id:'cat2',name:'Контрагенты',title:{ru:'Контрагенты'}}]}]},
  languages:[{code:'ru',title:'Русский'},{code:'en',title:'English'}],defaultLanguage:'ru',
};}

test('Studio inline script remains valid JavaScript',()=>{
  const html=readFileSync(new URL('../internal/studio/ui/index.html',import.meta.url),'utf8');
  for(const match of html.matchAll(/<script>([\s\S]*?)<\/script>/g))new vm.Script(match[1]);
  // Окно объекта даёт дерево (Node.Editor), а не папка вида на странице.
  assert.doesNotMatch(html,/metadata\/(catalogs|roles)|isCatalog|isRole\b|isRole[^O]/,'the page guesses a kind by its folder');
  assert.match(html,/\/api\/catalog/);assert.doesNotMatch(html,/id="catalog-create-dialog"/,"«Добавить» asks no name");assert.match(html,/id="tree-add"/);
});
test('new attribute gets a unique name and the type a new field takes, from the model',()=>{
  const source=fixture(),model=create(source);
  const first=model.addAttribute('catalog'),second=model.addAttribute('catalog');
  assert.notEqual(first.name,second.name);
  assert.deepEqual(JSON.parse(JSON.stringify(first.types)),[{kind:'string',length:10}]);
  assert.equal(model.value().attributes.length,2);
});
test('model mutates source in place but value() returns an independent clone',()=>{
  const source=fixture(),model=create(source);
  model.addAttribute('catalog');
  assert.equal(source.catalog.attributes.length,1);
  const value=model.value();value.attributes.push({});
  assert.equal(source.catalog.attributes.length,1);
});
test('attribute name and title round-trip',()=>{
  const model=create(fixture());
  const attribute=model.addAttribute('catalog');
  model.setAttributeName('catalog',attribute.id,'Артикул');
  model.setAttributeTitle('catalog',attribute.id,'ru','Артикул товара');
  model.setAttributeTitle('catalog',attribute.id,'en','');
  const saved=model.value().attributes[0];
  assert.equal(saved.name,'Артикул');assert.deepEqual(saved.title,{ru:'Артикул товара'});
});
test('a property is written by its dotted key, and left out when emptied',()=>{
  const attribute={name:'Артикул'};
  context.setPathValue(attribute,'presentation.mask','999');
  context.setPathValue(attribute,'fillChecking','show-error');
  assert.equal(context.pathValue(attribute,'presentation.mask'),'999');
  assert.equal(attribute.fillChecking,'show-error');
  context.setPathValue(attribute,'presentation.tooltip',{});
  context.setPathValue(attribute,'choice.parameters',[]);
  context.setPathValue(attribute,'comment','');
  assert.deepEqual(JSON.parse(JSON.stringify(attribute)),{name:'Артикул',presentation:{mask:'999'},fillChecking:'show-error',choice:{}});
  assert.equal(context.pathValue(attribute,'filling.value.data'),undefined);
});
test('the type the window chose replaces the attribute type, and the presentation names it',()=>{
  const model=create(fixture());
  const attribute=model.addAttribute('catalog');
  model.setAttributeTypes('catalog',attribute.id,[{kind:'number',precision:10},{kind:'catalog',reference:'cat2'}]);
  assert.deepEqual(JSON.parse(JSON.stringify(model.value().attributes[0].types)),[{kind:'number',precision:10},{kind:'catalog',reference:'cat2'}]);
  assert.equal(context.typePresentation(model.typeOffer,model.value().attributes[0].types),'Число, СправочникСсылка.Контрагенты');
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
test('an attribute and a table part move round the ends of their list, as the prototype moves them',()=>{
  const model=create(fixture());
  const [first,second,third]=[model.addAttribute('catalog'),model.addAttribute('catalog'),model.addAttribute('catalog')];
  const order=()=>model.value().attributes.map(item=>item.id);
  model.moveAttribute('catalog',first.id,-1);
  assert.deepEqual(order(),[second.id,third.id,first.id]);
  model.moveAttribute('catalog',first.id,1);
  assert.deepEqual(order(),[first.id,second.id,third.id]);
  model.moveAttribute('catalog',second.id,1);
  assert.deepEqual(order(),[first.id,third.id,second.id]);
  const one=model.addTablePart(),two=model.addTablePart();
  model.moveTablePart(two.id,1);
  assert.deepEqual(model.value().tableParts.map(item=>item.id),[two.id,one.id]);
});
test('the comment, the presentations and the explanation are written and left out when emptied',()=>{
  const model=create(fixture());
  model.setComment('для склада');model.setPresentation('objectPresentation','ru','Товар');model.setPresentation('explanation','en','Goods');
  model.setDefaultPresentation('as-code');
  let saved=model.value();
  assert.equal(saved.comment,'для склада');assert.deepEqual(saved.objectPresentation,{ru:'Товар'});assert.deepEqual(saved.explanation,{en:'Goods'});
  assert.equal(saved.defaultPresentation,'as-code');
  model.setComment('');model.setPresentation('objectPresentation','ru','');
  saved=model.value();
  assert.equal('comment' in saved,false);assert.equal('objectPresentation' in saved,false);
});
