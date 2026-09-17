import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const script=readFileSync(new URL('../internal/studio/ui/project-editor.js',import.meta.url),'utf8');
const context=vm.createContext({structuredClone});
vm.runInContext(script,context);
const create=context.createProjectModel;
function fixture(){return {manifest:{format:1,id:'p1',name:'SalesDemo',title:'Продажи и склад',defaultLanguage:'ru',languages:[
  {name:'English',title:'English',code:'en'},{name:'Русский',title:'Русский',code:'ru'},
]}};}

test('Studio inline script remains valid JavaScript',()=>{
  const html=readFileSync(new URL('../internal/studio/ui/index.html',import.meta.url),'utf8');
  for(const match of html.matchAll(/<script>([\s\S]*?)<\/script>/g))new vm.Script(match[1]);
  assert.match(html,/\/api\/project-manifest/);
});
test('name, title and default language round-trip',()=>{
  const model=create(fixture());
  model.setName('НовоеИмя');model.setTitle('Новый заголовок');model.setDefaultLanguage('en');
  const saved=model.value();
  assert.equal(saved.name,'НовоеИмя');assert.equal(saved.title,'Новый заголовок');assert.equal(saved.defaultLanguage,'en');
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
