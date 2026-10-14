import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const script=readFileSync(new URL('../internal/studio/ui/studio-common.js',import.meta.url),'utf8');
const context=vm.createContext({document:makeDocumentStub(),structuredClone,crypto});
vm.runInContext(script,context);
// StudioLanguage is declared `const` at script scope — visible to later
// classic <script> tags in a real browser, but not exposed as a property of
// the vm context on its own. Expose it explicitly for this test file.
vm.runInContext('globalThis.StudioLanguage = StudioLanguage;',context);
const {StudioLanguage,createLocalizedTitleField,localizedTitle,languageTitle,uniqueName,textInput}=context;

// A minimal DOM stub: just enough createElement/append/dataset surface for
// studio-common.js to build its field and dialog, without a real DOM.
function makeDocumentStub(){
  function makeElement(tag){
    const listeners={};
    return {
      tagName:tag,children:[],dataset:{},style:{},
      set textContent(value){this._text=value;},get textContent(){return this._text;},
      set value(value){this._value=value;},get value(){return this._value;},
      set type(value){this._type=value;},get type(){return this._type;},
      set title(value){this._title=value;},get title(){return this._title;},
      set className(value){this._class=value;},get className(){return this._class;},
      set method(value){this._method=value;},
      set maxLength(value){this._maxLength=value;},get maxLength(){return this._maxLength;},
      setAttribute(name,value){this.attributes={...this.attributes,[name]:value};},
      append(...nodes){this.children.push(...nodes);},
      addEventListener(name,handler){(listeners[name]||=[]).push(handler);},
      dispatch(name){for(const handler of listeners[name]||[])handler();},
      remove(){},
      showModal(){},
    };
  }
  return {createElement:makeElement,createTextNode:text=>({text}),body:{append(){}}};
}

test('StudioLanguage: init only sets the default once, set() notifies listeners',()=>{
  StudioLanguage.init('ru');
  assert.equal(StudioLanguage.get(),'ru');
  StudioLanguage.init('en'); // must not override an already-initialized language
  assert.equal(StudioLanguage.get(),'ru');
  let seen=null;
  const unsubscribe=StudioLanguage.onChange(code=>seen=code);
  StudioLanguage.set('uk');
  assert.equal(StudioLanguage.get(),'uk');
  assert.equal(seen,'uk');
  unsubscribe();
  StudioLanguage.set('en');
  assert.equal(seen,'uk'); // unsubscribed listener must not fire again
  StudioLanguage.set('ru'); // restore for the remaining tests below
});

test('collapsed field shows the current language and edits it in place',()=>{
  StudioLanguage.set('ru');
  const title={ru:'Товары',en:'Products'};
  const field=createLocalizedTitleField([{code:'ru',title:'Русский'},{code:'en',title:'English'}],code=>title[code],(code,value)=>title[code]=value,()=>{});
  const input=field.children[0];
  assert.equal(input.value,'Товары');
  input.value='Изменено';
  input.dispatch('input');
  assert.equal(title.ru,'Изменено');
  field.destroy();
});

test('the expand button is hidden for a single-language project',()=>{
  const field=createLocalizedTitleField([{code:'ru',title:'Русский'}],()=>'',()=>{},()=>{});
  assert.equal(field.children.length,1); // just the input, no magnifying-glass button
  field.destroy();
});

test('switching StudioLanguage refreshes every mounted field',()=>{
  StudioLanguage.set('ru');
  const title={ru:'Товары',en:'Products'};
  const field=createLocalizedTitleField([{code:'ru',title:'Русский'},{code:'en',title:'English'}],code=>title[code],(code,value)=>title[code]=value,()=>{});
  const input=field.children[0];
  assert.equal(input.value,'Товары');
  StudioLanguage.set('en');
  assert.equal(input.value,'Products');
  StudioLanguage.set('ru');
  field.destroy();
});

// Подпись показывается на текущем языке Studio, а без перевода на нём — по
// цепочке проекта (язык по умолчанию, затем порядок языков), и только потом —
// именем. Дефект: редактор берёт язык по умолчанию и первый попавшийся
// перевод, а переключатель языка Studio на подписи не действует.
test('a title follows the Studio language, then the project chain, then the name',()=>{
  const languages=[{code:'ru'},{code:'de'},{code:'en'}];
  StudioLanguage.set('en');
  assert.equal(localizedTitle({name:'Goods',title:{ru:'Товары',en:'Products'}},'ru',languages),'Products');
  assert.equal(localizedTitle({name:'Goods',title:{en:'  ',ru:'Товары'}},'ru',languages),'Товары');
  assert.equal(localizedTitle({name:'Goods',title:{en:'',de:'Waren',ar:'بضائع'}},'ru',languages),'Waren');
  assert.equal(localizedTitle({name:'Goods',title:{}},'ru',languages),'Goods');
  StudioLanguage.set('ru');
  assert.equal(localizedTitle({name:'Goods',title:{ru:'Товары',en:'Products'}},'ru',languages),'Товары');
});

// Язык называется на самом себе, а не на текущем языке Studio и не на языке
// по умолчанию: список, где один язык написан на другом, читается как ошибка.
test('a language is named on itself, then by its name, then by its code',()=>{
  const languages=[{code:'ru'},{code:'en'}];
  StudioLanguage.set('ru');
  assert.equal(languageTitle({code:'en',name:'English',title:{ru:'Английский',en:'English (US)'}},'ru',languages),'English (US)');
  assert.equal(languageTitle({code:'en',name:'English',title:{}},'ru',languages),'English');
  assert.equal(languageTitle({code:'en',name:'',title:null},'ru',languages),'en');
});

// Имя сверяется без учёта регистра, как имена конфигурации.
test('a unique name skips taken names whatever their case',()=>{
  assert.equal(uniqueName('Поле',new Set()),'Поле');
  assert.equal(uniqueName('Поле',new Set(['поле','поле2'])),'Поле3');
});

// Предел длины ставится, только если он назван: у комментария его нет.
// Число применяется по завершении ввода и числом, текст — на каждой букве.
test('a text control limits only when told to and applies numbers as numbers',()=>{
  let applied;
  const free=textInput('x',value=>applied=value);
  assert.equal(free.maxLength,undefined);
  free.value='abc';free.dispatch('input');
  assert.equal(applied,'abc');
  assert.equal(textInput('x',()=>{},{maxLength:255}).maxLength,255);
  const number=textInput(5,value=>applied=value,{type:'number'});
  number.value='7';number.dispatch('input');
  assert.equal(applied,'abc');
  number.dispatch('change');
  assert.equal(applied,7);
});

// Every window that offers «Справка» names a page of studioHelp, and the page
// has a heading and text: a key without a page would throw on the button,
// a misspelt one would never be noticed until pressed.
test('every help key a window passes has a page in studioHelp',async()=>{
  const {readdirSync}=await import('node:fs');
  const help=vm.createContext({});
  vm.runInContext(readFileSync(new URL('../internal/studio/ui/studio-help.js',import.meta.url),'utf8')+';globalThis.studioHelp=studioHelp;',help);
  const folder=new URL('../internal/studio/ui/',import.meta.url),keys=new Set();
  for(const name of readdirSync(folder).filter(name=>name.endsWith('.js')))
    for(const match of readFileSync(new URL(name,folder),'utf8').matchAll(/\bhelp:\s*'([^']+)'/g))keys.add(match[1]);
  assert.ok(keys.size>=4,`help keys found: ${[...keys]}`);
  for(const key of keys){
    const page=help.studioHelp[key];
    assert.ok(page,`no page for ${key}`);
    assert.ok(page.title&&page.text.length&&page.text.every(Boolean),`empty page ${key}`);
  }
});
