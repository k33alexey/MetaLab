import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

// The window «Редактирование типа данных» without its drawing
// (typeEditorModel, studio-common.js). Every expectation is the prototype's,
// written out from the opis (docs/block-00/obshchee.md, «Окно
// «Редактирование типа данных»», snapshots configurator-2026-10-23/tip-*),
// not read from the model under test.
const context=vm.createContext({structuredClone,crypto});
vm.runInContext(readFileSync(new URL('../internal/studio/ui/studio-common.js',import.meta.url),'utf8'),context);
const {typeEditorModel,typePresentation}=context;
const plain=value=>JSON.parse(JSON.stringify(value));

function offer({required=true,arbitrary=false}={}){return {
  place:'stored-field',required,limits:{stringLength:1024,fixedStringLength:100,numberDigits:32},
  newTypes:{string:{kind:'string',length:10},number:{kind:'number',precision:10},date:{kind:'date',dateParts:'date'}},
  rows:[...(arbitrary?[{arbitrary:true,name:{russian:'Произвольный'}}]:[]),
    {kind:'number',name:{russian:'Число'}},{kind:'string',name:{russian:'Строка'}},{kind:'date',name:{russian:'Дата'}},{kind:'boolean',name:{russian:'Булево'}},
    {kind:'catalog-ref',name:{russian:'СправочникСсылка'},objectKind:'catalog',objects:[{id:'c1',name:'АвансовыйОтчетПрисоединенныеФайлы'},{id:'c2',name:'Валюты'}]},
    {kind:'document-ref',name:{russian:'ДокументСсылка'},objectKind:'document',objects:[{id:'d1',name:'ПокупкаПродажаВалюты'}]},
    {kind:'business-process-ref',name:{russian:'БизнесПроцессСсылка'},objectKind:'business-process',objects:[]},
    {name:{russian:'ОпределяемыйТип'},objectKind:'defined-type',objects:[{id:'t1',name:'GLN'}]},
    {kind:'any-ref',name:{russian:'ЛюбаяСсылка'}}]};}
const captions=model=>plain(model.visible().map(row=>row.caption));

// Defect caught: two types marked without «Составной тип данных».
test('without a composite type a mark takes the one before away',()=>{
  const model=typeEditorModel(offer(),[{kind:'string',length:10}]);
  model.mark('number',true);
  assert.deepEqual(plain(model.result().types),[{kind:'number',precision:10}]);
  assert.equal(model.current,'number');
});

// Defect caught: a click on the caption marking the type.
test('choosing a row makes it current and marks nothing; its qualifiers show unavailable',()=>{
  const model=typeEditorModel(offer(),[{kind:'number',precision:10}]);
  model.choose('date');
  assert.equal(model.current,'date');
  assert.deepEqual(plain(model.result().types),[{kind:'number',precision:10}]);
  assert.deepEqual(plain(model.qualifiers()),{kind:'date',type:{kind:'date',dateParts:'date'},enabled:false});
  model.choose('boolean');
  assert.equal(model.qualifiers(),null);
});

// Defects caught: taking «Составной» away asking, or leaving the last marked
// rather than the first in the order of the tree.
test('taking the composite type away leaves the first type of the tree marked and current',()=>{
  const model=typeEditorModel(offer(),[{kind:'string',length:10},{kind:'number',precision:10},{kind:'catalog',reference:'c1'},{kind:'catalog-ref'}]);
  assert.equal(model.composite,true);
  model.setComposite(false);
  assert.deepEqual(plain(model.result().types),[{kind:'number',precision:10}]);
  assert.equal(model.current,'number');
});

// Defect caught: the window opening on the first type of the description, or
// with the group of a marked object folded.
test('the window opens on the last type marked in the order of the tree, its group unfolded',()=>{
  const model=typeEditorModel(offer(),[{kind:'catalog',reference:'c1'},{kind:'string',length:10},{kind:'number',precision:10},{kind:'document',reference:'d1'}]);
  assert.equal(model.current,'document:d1');
  const shown=model.visible();
  assert.equal(shown.find(row=>row.key==='catalog-ref').open,true);
  assert.equal(shown.find(row=>row.key==='document-ref').open,true);
  assert.equal(shown.find(row=>row.key==='business-process-ref').folds,false);
});

// Defects caught: the set marking the objects inside it, a set and an
// object of its kind not standing together, a description out of the order
// of the tree.
test('a set and an object of its kind stand together, and the description follows the tree',()=>{
  const model=typeEditorModel(offer(),[{kind:'number',precision:10}]);
  model.setComposite(true);
  model.mark('catalog-ref',true);
  model.fold('catalog-ref',true);
  assert.equal(model.visible().find(row=>row.key==='catalog:c1').checked,false);
  model.mark('catalog:c1',true);
  model.mark('string',true);
  assert.deepEqual(plain(model.result().types),[{kind:'number',precision:10},{kind:'string',length:10},{kind:'catalog-ref'},{kind:'catalog',reference:'c1'}]);
  assert.equal(typePresentation(offer(),model.result().types),'Число, Строка, СправочникСсылка, СправочникСсылка.АвансовыйОтчетПрисоединенныеФайлы');
});

// Defects caught: a number of 0 or 33 digits written, the fraction and the
// number out of step.
test('the qualifiers of a number keep the ceilings and each other',()=>{
  const model=typeEditorModel(offer(),[{kind:'number',precision:10}]);
  for(const value of [0,33,38,99])assert.equal(model.setQualifier('precision',value),false);
  assert.equal(model.setQualifier('scale',33),false);
  assert.equal(model.setQualifier('precision',32),true);
  model.setQualifier('precision',10);
  model.setQualifier('scale',11);
  assert.deepEqual(plain(model.qualifiers().type),{kind:'number',precision:11,scale:11});
  model.setQualifier('precision',5);
  assert.deepEqual(plain(model.qualifiers().type),{kind:'number',precision:5,scale:5});
});

// Defects caught: a string of length 0 left of fixed length, 1025 written,
// «Фиксированная» at a length over 100 - the prototype's window lets it
// through, Studio does not (PLATFORM-MODEL.md, «Выбор типа»).
test('the qualifiers of a string: unlimited at 0, back to 10, fixed only up to 100',()=>{
  const model=typeEditorModel(offer(),[{kind:'string',length:10}]);
  model.setQualifier('fixedLength',true);
  model.setQualifier('length',0);
  assert.deepEqual(plain(model.qualifiers().type),{kind:'string'});
  model.setQualifier('unlimited',false);
  assert.deepEqual(plain(model.qualifiers().type),{kind:'string',length:10});
  assert.equal(model.setQualifier('length',1025),false);
  model.setQualifier('length',101);
  model.setQualifier('fixedLength',true);
  assert.deepEqual(plain(model.qualifiers().type),{kind:'string',length:101});
  model.setQualifier('length',100);
  model.setQualifier('fixedLength',true);
  model.setQualifier('length',200);
  assert.deepEqual(plain(model.qualifiers().type),{kind:'string',length:200});
});

// Defects caught: a search of one letter filtering, the words searched in the
// whole name of a type rather than in its row, the marked rows kept in sight.
test('the search: from two characters, every word in one row, the group of a match unfolded',()=>{
  const model=typeEditorModel(offer(),[{kind:'string',length:10}]);
  model.setSearch('В');
  assert.equal(captions(model).length,9);
  model.setSearch('Вал спр');
  assert.deepEqual(captions(model),[]);
  model.setSearch('ссыл спр');
  assert.deepEqual(captions(model),['СправочникСсылка','БизнесПроцессСсылка']);
  assert.equal(model.visible()[0].folds,false);
  model.setSearch('Валюты');
  const shown=model.visible();
  assert.deepEqual(plain(shown.map(row=>row.caption)),['СправочникСсылка','Валюты','ДокументСсылка','ПокупкаПродажаВалюты']);
  assert.deepEqual(plain(shown[0].highlights),[]);
  assert.deepEqual(plain(shown[3].highlights),[[14,6]]);
  model.setSearch('');
  assert.equal(captions(model).length,9);
});

// Defect caught: «ОК» with nothing marked closing where a type is required,
// and refusing where it is not.
test('nothing marked is refused only where a type is required',()=>{
  const model=typeEditorModel(offer(),[{kind:'string',length:10}]);
  model.mark('string',false);
  assert.equal(model.result().refusal,'Необходимо выбрать тип значения.');
  const free=typeEditorModel(offer({required:false}),[]);
  assert.deepEqual(plain(free.result()),{types:[]});
});

// Defect caught: «Произвольный» standing beside a type.
test('«Произвольный» is no type and stands alone',()=>{
  const model=typeEditorModel(offer({required:false,arbitrary:true}),[]);
  assert.equal(model.visible()[0].checked,true);
  model.setComposite(true);
  model.mark('number',true);
  assert.equal(model.visible()[0].checked,false);
  model.mark('arbitrary',true);
  assert.deepEqual(plain(model.result().types),[]);
  assert.equal(typePresentation(offer({arbitrary:true}),[]),'Произвольный');
});

// Defects caught: a type the place takes only by itself put beside another
// without the prototype's question (snapshots series-2.531/12,
// series-2.535/10), so the save refuses what the window built; the question
// asked where nothing is taken away - outside a composite, or beside types
// that may stand together; the objects of a folder marked alone standing
// beside a type; «Произвольный» marked before not asking. The questions are
// the prototype's: a type standing alone beside others - «excludes-others»;
// a type beside one standing alone - «excludes-earlier»; «Да» leaves the new
// type alone.
test('a type that stands only by itself asks before it takes the others away',()=>{
  const place=offer({required:false,arbitrary:true});
  place.rows.push({kind:'value-storage',name:{russian:'ХранилищеЗначения'},alone:true});
  place.rows.push({name:{russian:'ВнешнийИсточникДанныхТаблицаСсылка'},objectKind:'external-data-source-table',alone:true,
    objects:[{id:'x1',name:'Склад.Товары'}]});
  const model=typeEditorModel(place,[{kind:'string',length:10},{kind:'number',precision:10}]);
  assert.equal(model.composite,true);
  assert.equal(model.exclusion('date'),null);
  assert.equal(model.exclusion('value-storage'),'excludes-others');
  assert.equal(model.exclusion('external-data-source-table:x1'),'excludes-others');
  model.mark('value-storage',true);
  assert.deepEqual(plain(model.result().types),[{kind:'value-storage'}]);
  assert.equal(model.exclusion('number'),'excludes-earlier');
  assert.equal(model.exclusion('value-storage'),null);
  model.mark('number',true);
  assert.deepEqual(plain(model.result().types),[{kind:'number',precision:10}]);
  model.mark('arbitrary',true);
  assert.equal(model.exclusion('string'),'excludes-earlier');
  const single=typeEditorModel(place,[{kind:'string',length:10}]);
  assert.equal(single.composite,false);
  assert.equal(single.exclusion('value-storage'),null);
});

// Defects caught: a type of the description the place does not offer - what
// the model takes beyond the window, a type of an earlier version - dropped
// without a word on «ОК», as it was until 2.555 (review 2.554, А9); kept
// through a mark that takes every other away.
test('a type the place does not offer is kept until a mark takes the others away',()=>{
  const model=typeEditorModel(offer(),[{kind:'string',length:10},{kind:'vanished-type',name:'Старый'}]);
  assert.equal(model.composite,true);
  assert.deepEqual(plain(model.result().types),[{kind:'string',length:10},{kind:'vanished-type',name:'Старый'}]);
  model.mark('number',true);
  assert.deepEqual(plain(model.result().types),[{kind:'number',precision:10},{kind:'string',length:10},{kind:'vanished-type',name:'Старый'}]);
  model.setComposite(false);
  assert.deepEqual(plain(model.result().types),[{kind:'number',precision:10}]);
  const alone=typeEditorModel(offer(),[{kind:'vanished-type',name:'Старый'}]);
  assert.deepEqual(plain(alone.result().types),[{kind:'vanished-type',name:'Старый'}]);
  alone.mark('date',true);
  assert.deepEqual(plain(alone.result().types),[{kind:'date',dateParts:'date'}]);
});

// Defect caught: qualifiers forgotten when a type is unmarked and marked again.
test('a type marked again keeps the qualifiers it had',()=>{
  const model=typeEditorModel(offer(),[{kind:'string',length:25}]);
  model.mark('number',true);
  model.mark('string',true);
  assert.deepEqual(plain(model.result().types),[{kind:'string',length:25}]);
});
