import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const script=readFileSync(new URL('../internal/studio/ui/project-editor.js',import.meta.url),'utf8');
const context=vm.createContext({structuredClone});
vm.runInContext(script,context);
const create=context.createProjectModel;
function fixture(){return {configuration:{format:1,id:'p1',name:'SalesDemo',title:{ru:'Продажи и склад'},defaultLanguage:'ru',languages:[
  {name:'English',title:'English',code:'en'},{name:'Русский',title:'Русский',code:'ru'},
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
  model.setRoleGranted('r1',true);model.setRoleGranted('r2',true);model.setRoleGranted('r1',true);
  assert.deepEqual(model.value().defaultRoles,['r2','r1']);
  model.setRoleGranted('r2',false);
  assert.deepEqual(model.value().defaultRoles,['r1']);
  model.setRoleGranted('r1',false);
  assert.equal('defaultRoles' in model.value(),false);
});
