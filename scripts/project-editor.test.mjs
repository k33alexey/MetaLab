import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const script=readFileSync(new URL('../internal/studio/ui/project-editor.js',import.meta.url),'utf8');
const context=vm.createContext({structuredClone});
// Редактор опирается на общие помощники Studio, как в index.html, где
// studio-common.js подключается первым.
vm.runInContext(readFileSync(new URL('../internal/studio/ui/studio-common.js',import.meta.url),'utf8'),context);
vm.runInContext(script,context);
const create=context.createProjectModel;
function fixture(){return {configuration:{format:1,id:'p1',name:'SalesDemo',title:{ru:'Продажи и склад'},defaultLanguage:'ru',languages:[
  {name:'English',title:{en:'English'},code:'en'},{name:'Русский',title:{ru:'Русский'},code:'ru'},
]}};}

test('Studio inline script remains valid JavaScript',()=>{
  const html=readFileSync(new URL('../internal/studio/ui/index.html',import.meta.url),'utf8');
  for(const match of html.matchAll(/<script>([\s\S]*?)<\/script>/g))new vm.Script(match[1]);
  assert.match(html,/\/api\/project-configuration/);
});
const subject=configuration=>context.rootSubject(configuration);
// Палитра корня правит описание по ключам строк: имя, синоним по языкам,
// основной язык — по его коду, хотя палитра видит идентификатор языка.
test('name, synonym and the main language round-trip through the palette',()=>{
  const configuration=fixture().configuration;configuration.languages[0].id='l-en';configuration.languages[1].id='l-ru';
  const root=subject(configuration);
  root.setName('НовоеИмя');root.set('title',{ru:'Новый синоним'});
  assert.equal(root.get('defaultLanguage'),'l-ru');
  root.set('defaultLanguage','l-en');
  assert.equal(configuration.name,'НовоеИмя');assert.deepEqual(configuration.title,{ru:'Новый синоним'});assert.equal(configuration.defaultLanguage,'en');
  // Язык, которого нет, основной язык не меняет: без основного языка корень не живёт.
  root.set('defaultLanguage','l-xx');
  assert.equal(configuration.defaultLanguage,'en');
});
// Текст пишется по языку, и стёртый целиком исчезает из корня: пустая
// строка — это не перевод, а его отсутствие.
test('an emptied text disappears from the root',()=>{
  const configuration=fixture().configuration,root=subject(configuration);
  root.set('copyright',{ru:'© Пример'});
  assert.deepEqual(configuration.copyright,{ru:'© Пример'});
  root.set('copyright',{});
  assert.equal('copyright' in configuration,false);
});
test('English cannot be removed or edited through the model',()=>{
  const model=create(fixture());
  model.setLanguageField('en','name','Hacked');
  model.removeLanguage('en');
  const saved=model.value();
  assert.equal(saved.languages.length,2);
  assert.equal(saved.languages.find(item=>item.code==='en').name,'English');
});
test('removing the current default language falls back to another configured one',()=>{
  const model=create(fixture());
  model.setDefaultLanguage('ru');
  model.removeLanguage('ru');
  const saved=model.value();
  assert.equal(saved.languages.length,1);
  assert.equal(saved.defaultLanguage,'en');
});
test('value() returns an independent clone',()=>{
  const source=fixture(),model=create(source);
  const value=model.value();value.languages.push({name:'x',title:'x',code:'x'});
  assert.equal(model.value().languages.length,2);
});
// Поставщик — простое поле, умолчание — ссылка, флаг — отметка: опустошённое,
// очищенное и снятое исчезают из корня, а не остаются пустой строкой, null
// или ложью — «не задано» означает своё у платформы, а не ссылку в никуда.
test('an emptied field, a cleared default and an unmarked flag vanish',()=>{
  const configuration=fixture().configuration,root=subject(configuration);
  root.set('vendor','Пример');root.set('defaultStyle','s1');root.set('includeHelpInContents',true);root.set('dataLockControl','managed');
  assert.equal(configuration.vendor,'Пример');assert.equal(configuration.defaultStyle,'s1');assert.equal(configuration.includeHelpInContents,true);
  root.set('vendor','');root.set('defaultStyle',undefined);root.set('includeHelpInContents',false);
  for(const key of ['vendor','defaultStyle','includeHelpInContents'])assert.equal(key in configuration,false,key);
  assert.equal(configuration.dataLockControl,'managed');
});
// Роли идут в том порядке, в каком их отметили в окне выбора, а пустой
// список пропадает целиком.
test('the main roles are kept as chosen and an empty list vanishes',()=>{
  const configuration=fixture().configuration,root=subject(configuration);
  root.set('defaultRoles',['r2','r1']);
  assert.deepEqual(configuration.defaultRoles,['r2','r1']);
  root.set('defaultRoles',[]);
  assert.equal('defaultRoles' in configuration,false);
});
// Словарь поиска — пара «вид и объект»: палитра видит идентификаторы, а
// корень держит пары; снятый словарь уходит вместе со своим видом, и
// последний снятый уносит список.
test('a search dictionary is a kind and an object, not an identifier alone',()=>{
  const configuration=fixture().configuration,root=subject(configuration);
  configuration.additionalFullTextSearchDictionaries=[{kind:'common-templates',object:'d1'},{kind:'constants',object:'d2'}];
  assert.deepEqual(root.get('additionalFullTextSearchDictionaries'),['d1','d2']);
  root.set('additionalFullTextSearchDictionaries',['d2']);
  assert.deepEqual(configuration.additionalFullTextSearchDictionaries,[{kind:'constants',object:'d2'}]);
  root.set('additionalFullTextSearchDictionaries',[]);
  assert.equal('additionalFullTextSearchDictionaries' in configuration,false);
});
// Синоним языка локализован: язык называется на каждом языке проекта, и
// стёртый перевод исчезает, а не остаётся пустой строкой. Английский не
// правится вовсе.
test('a language synonym is written per language',()=>{
  const model=create(fixture());
  model.setLanguageTitle('ru','en','Russian');
  model.setLanguageTitle('ru','ru','Русский');
  let saved=model.value().languages.find(item=>item.code==='ru');
  assert.deepEqual(saved.title,{en:'Russian',ru:'Русский'});
  model.setLanguageTitle('ru','en','');
  saved=model.value().languages.find(item=>item.code==='ru');
  assert.deepEqual(saved.title,{ru:'Русский'});
  model.setLanguageTitle('en','ru','Взломанный');
  assert.deepEqual(model.value().languages.find(item=>item.code==='en').title,{en:'English'});
});
// Комментарий языка — обычное поле: опустошённое, оно пропадает.
test('a language comment vanishes when emptied',()=>{
  const model=create(fixture());
  model.setLanguageField('ru','comment','Язык учёта');
  assert.equal(model.value().languages.find(item=>item.code==='ru').comment,'Язык учёта');
  model.setLanguageField('ru','comment','');
  assert.equal('comment' in model.value().languages.find(item=>item.code==='ru'),false);
});
// Порядок объектов палитра корня не показывает и не правит, но хранит его то
// же описание корня: правка палитры не должна его терять (2.519).
test('the order of the objects survives an edit of the palette',()=>{
  const configuration=fixture().configuration;
  configuration.objectsOrder={catalogs:['c3','c1','c2']};
  subject(configuration).set('vendor','MetaLab');
  assert.deepEqual(configuration.objectsOrder,{catalogs:['c3','c1','c2']});
});
// Окно «Командный интерфейс» пишет порядок и видимость разделов в то же
// описание; пустые — не пишутся.
test('the sections of the command interface are written and emptied',()=>{
  const model=create(fixture());
  model.setSections(['s1'],[{subsystem:'s1',common:false}]);
  assert.deepEqual(model.value().subsystemsOrder,['s1']);
  model.setSections([],[]);
  assert.equal('subsystemsOrder' in model.value(),false);assert.equal('subsystemsVisibility' in model.value(),false);
});
