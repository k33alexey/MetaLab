import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const script=readFileSync(new URL('../internal/studio/ui/project-editor.js',import.meta.url),'utf8');
const context=vm.createContext({structuredClone});
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
test('name, synonym and default language round-trip',()=>{
  const model=create(fixture());
  model.setName('НовоеИмя');model.setText('title','ru','Новый синоним');model.setDefaultLanguage('en');
  const saved=model.value();
  assert.equal(saved.name,'НовоеИмя');assert.equal(saved.title.ru,'Новый синоним');assert.equal(saved.defaultLanguage,'en');
});
// Синоним пишется по языку, и язык, в котором его стёрли, исчезает из текста
// целиком: пустая строка — это не перевод, а его отсутствие.
test('a text is stored per language and an emptied one disappears',()=>{
  const model=create(fixture());
  model.setText('title','en','Sales and warehouse');
  model.setText('copyright','ru','© Пример');
  let saved=model.value();
  assert.deepEqual(saved.title,{ru:'Продажи и склад',en:'Sales and warehouse'});
  assert.deepEqual(saved.copyright,{ru:'© Пример'});
  model.setText('title','en','');
  model.setText('copyright','ru','');
  saved=model.value();
  assert.deepEqual(saved.title,{ru:'Продажи и склад'});
  assert.equal('copyright' in saved,false);
});
// Поставщик и версия одинаковы на всех языках, поэтому это простые поля - и
// опустошённое поле пропадает, а не остаётся пустой строкой.
test('vendor and version are plain fields that vanish when emptied',()=>{
  const model=create(fixture());
  model.setField('vendor','Пример');model.setField('version','1.0.0.1');
  let saved=model.value();
  assert.equal(saved.vendor,'Пример');assert.equal(saved.version,'1.0.0.1');
  model.setField('vendor','');
  saved=model.value();
  assert.equal('vendor' in saved,false);
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
test('a new language gets a unique code and is independently editable',()=>{
  const model=create(fixture());
  const added=model.addLanguage();
  assert.notEqual(added.code,'en');assert.notEqual(added.code,'ru');
  model.setLanguageField(added.code,'name','Українська');model.setLanguageField(added.code,'title','Українська');
  const saved=model.value().languages.find(item=>item.code===added.code);
  assert.equal(saved.name,'Українська');assert.equal(saved.title,'Українська');
});
test('value() returns an independent clone',()=>{
  const source=fixture(),model=create(source);
  const value=model.value();value.languages.push({name:'x',title:'x',code:'x'});
  assert.equal(model.value().languages.length,2);
});
// Умолчание — ссылка на другой объект, и очищенное умолчание исчезает из
// корня, а не остаётся пустой строкой: «не задано» означает, что платформа
// возьмёт своё, а не что конфигурация назвала ничто.
test('a default is a reference that vanishes when cleared',()=>{
  const model=create(fixture());
  model.setField('defaultStyle','s1');
  model.setField('defaultReportForm','f1');
  let saved=model.value();
  assert.equal(saved.defaultStyle,'s1');assert.equal(saved.defaultReportForm,'f1');
  model.setField('defaultStyle','');
  saved=model.value();
  assert.equal('defaultStyle' in saved,false);
  assert.equal(saved.defaultReportForm,'f1');
});
// Роли перечислены в порядке конфигурации, роль не выдаётся дважды, а список
// без единой роли пропадает целиком.
test('default roles keep their order and are granted once',()=>{
  const model=create(fixture());
  model.setRoleGranted('defaultRoles','r1',true);model.setRoleGranted('defaultRoles','r2',true);model.setRoleGranted('defaultRoles','r1',true);
  assert.deepEqual(model.value().defaultRoles,['r2','r1']);
  model.setRoleGranted('defaultRoles','r2',false);
  assert.deepEqual(model.value().defaultRoles,['r1']);
  model.setRoleGranted('defaultRoles','r1',false);
  assert.equal('defaultRoles' in model.value(),false);
});
// Словарь поиска — пара «вид и объект»: один и тот же идентификатор у макета и
// у константы означает разные словари, и снятый последний словарь уносит
// список целиком.
test('a search dictionary is a kind and an object, not an identifier alone',()=>{
  const model=create(fixture());
  model.setDictionary('common-templates','d1',true);
  model.setDictionary('constants','d1',true);
  assert.deepEqual(model.value().additionalFullTextSearchDictionaries,
    [{kind:'common-templates',object:'d1'},{kind:'constants',object:'d1'}]);
  model.setDictionary('common-templates','d1',false);
  assert.deepEqual(model.value().additionalFullTextSearchDictionaries,[{kind:'constants',object:'d1'}]);
  model.setDictionary('constants','d1',false);
  assert.equal('additionalFullTextSearchDictionaries' in model.value(),false);
});
// Режим — слово из набора, и «как в ML» — это отсутствие свойства, а не пустая
// строка в файле.
test('a mode set back to the platform default disappears from the root',()=>{
  const model=create(fixture());
  model.setField('dataLockControl','managed');model.setField('scriptVariant','russian');
  assert.equal(model.value().dataLockControl,'managed');
  model.setField('dataLockControl','');
  const saved=model.value();
  assert.equal('dataLockControl' in saved,false);
  assert.equal(saved.scriptVariant,'russian');
});
// Флаг, снятый обратно, исчезает из корня целиком: ложь и отсутствие здесь —
// одно и то же, и писать её в файл значило бы писать пустоту словами.
test('a flag that is switched off disappears from the root',()=>{
  const model=create(fixture());
  model.setFlag('includeHelpInContents',true);
  assert.equal(model.value().includeHelpInContents,true);
  model.setFlag('includeHelpInContents',false);
  assert.equal('includeHelpInContents' in model.value(),false);
});
// Назначение, названное дважды, остаётся одним назначением, а список без
// единого назначения пропадает.
test('use purposes are named once and vanish together',()=>{
  const model=create(fixture());
  model.setPurpose('personal-computer',true);model.setPurpose('mobile-device',true);
  model.setPurpose('personal-computer',true);
  assert.deepEqual(model.value().usePurposes,['mobile-device','personal-computer']);
  model.setPurpose('mobile-device',false);model.setPurpose('personal-computer',false);
  assert.equal('usePurposes' in model.value(),false);
});
// Один и тот же список ролей с двумя разными полями: основные роли корня и
// роли ограничения автономного приложения не смешиваются.
test('roles are granted per field and never leak into another',()=>{
  const model=create(fixture());
  model.setRoleGranted('defaultRoles','r1',true);
  model.setRoleGranted('standaloneConfigurationRestrictionRoles','r2',true);
  const saved=model.value();
  assert.deepEqual(saved.defaultRoles,['r1']);
  assert.deepEqual(saved.standaloneConfigurationRestrictionRoles,['r2']);
});
// Список слов пишется по строке на слово, пустые строки не слова, и опустевший
// список исчезает из корня.
test('a list of words is written a line at a time',()=>{
  const model=create(fixture());
  model.setList('requiredMobilePermissions','Камера\n\n  Геолокация  \n');
  assert.deepEqual(model.value().requiredMobilePermissions,['Камера','Геолокация']);
  model.setList('requiredMobilePermissions','\n  \n');
  assert.equal('requiredMobilePermissions' in model.value(),false);
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
