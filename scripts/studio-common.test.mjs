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
const {StudioLanguage,createLocalizedTitleField}=context;

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
      set maxLength(value){this._maxLength=value;},
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
