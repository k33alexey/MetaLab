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
const {StudioLanguage,createLocalizedField,localizedTitle,languageTitle,uniqueName,textInput}=context;

// A minimal DOM stub: just enough createElement/append/dataset surface for
// studio-common.js to build its field and dialog, without a real DOM.
function makeDocumentStub(){
  function makeElement(tag){
    const listeners={};
    return {
      tagName:tag,children:[],dataset:{},style:{},classList:{add(){},toggle(){}},validity:{valid:true},
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
  return {createElement:makeElement,createElementNS:(namespace,tag)=>makeElement(tag),createTextNode:text=>({text}),body:{append(){}}};
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

test('the field shows the current language and edits it in place',()=>{
  StudioLanguage.set('ru');
  const title={ru:'Товары',en:'Products'};
  const field=createLocalizedField({languages:[{code:'ru'},{code:'en'}],defaultLanguage:'ru'},code=>title[code],(code,value)=>title[code]=value,()=>{});
  const input=field.children[0];
  assert.equal(input.value,'Товары');
  input.value='Изменено';
  input.dispatch('input');
  assert.equal(title.ru,'Изменено');
  field.destroy();
});

// Лупа появляется, когда языков больше одного (решение владельца
// 19.10.2026): одному языку переводить не на что. Дефект: лупа у проекта с
// одним языком или её нет у проекта с двумя.
test('the magnifier stands inside the field while there is more than one language',()=>{
  const lupa=languages=>{
    const field=createLocalizedField({languages,defaultLanguage:'ru'},()=>'',()=>{},()=>{});
    const [input,tray]=field.children;
    assert.equal(input.tagName,'input');
    field.destroy();
    return tray.children.map(button=>button.title);
  };
  assert.deepEqual(lupa([{code:'ru'}]),[]);
  assert.deepEqual(lupa([{code:'ru'},{code:'en'}]),['Строки на разных языках']);
});

test('switching StudioLanguage refreshes every mounted field',()=>{
  StudioLanguage.set('ru');
  const title={ru:'Товары',en:'Products'};
  const field=createLocalizedField({languages:[{code:'ru'},{code:'en'}],defaultLanguage:'ru'},code=>title[code],(code,value)=>title[code]=value,()=>{});
  const input=field.children[0];
  assert.equal(input.value,'Товары');
  StudioLanguage.set('en');
  assert.equal(input.value,'Products');
  StudioLanguage.set('ru');
  field.destroy();
});

// Подпись показывается на текущем языке Studio, а без перевода на нём —
// именем, не переводом на другом языке (правило платформы «нет перевода на
// языке — имя», решение владельца 19.10.2026). Дефекты: переключатель языка
// Studio на подписи не действует; без перевода на языке Studio берётся язык
// по умолчанию или первый попавшийся перевод.
test('a title follows the Studio language, else it is the name',()=>{
  StudioLanguage.set('en');
  assert.equal(localizedTitle({name:'Goods',title:{ru:'Товары',en:'Products'}},'ru'),'Products');
  assert.equal(localizedTitle({name:'Goods',title:{en:'  ',ru:'Товары'}},'ru'),'Goods');
  assert.equal(localizedTitle({name:'Goods',title:{ru:'Товары',de:'Waren'}},'ru'),'Goods');
  assert.equal(localizedTitle({name:'Goods',title:{}},'ru'),'Goods');
  StudioLanguage.set('ru');
  assert.equal(localizedTitle({name:'Goods',title:{ru:'Товары',en:'Products'}},'ru'),'Товары');
});

// Язык называется на самом себе, а не на текущем языке Studio и не на языке
// по умолчанию: список, где один язык написан на другом, читается как ошибка.
// Язык называется синонимом на языке Studio, как прототип подписывает
// строки окна «Строки на разных языках» (ERP: «Російська» при украинском
// основном языке), а без синонима на нём — именем, по правилу платформы
// «нет перевода на языке — имя» (решения владельца 19.10.2026). Дефекты: язык
// назван на себе самом, не следует смене языка Studio, назван переводом на
// другом языке.
test('a language is named in the language of Studio, else by its name, else by its code',()=>{
  StudioLanguage.set('ru');
  assert.equal(languageTitle({code:'en',name:'English',title:{ru:'Английский',en:'English (US)'}},'ru'),'Английский');
  StudioLanguage.set('en');
  assert.equal(languageTitle({code:'en',name:'English',title:{ru:'Английский',en:'English (US)'}},'ru'),'English (US)');
  StudioLanguage.set('ru');
  assert.equal(languageTitle({code:'en',name:'English',title:{en:'English (US)',de:'Englisch'}},'ru'),'English');
  assert.equal(languageTitle({code:'en',name:'English',title:{ru:'  '}},'ru'),'English');
  assert.equal(languageTitle({code:'en',name:'',title:null},'ru'),'en');
});

// Имя сверяется без учёта регистра, как имена конфигурации.
test('a unique name skips taken names whatever their case',()=>{
  assert.equal(uniqueName('Поле',new Set()),'Поле');
  assert.equal(uniqueName('Поле',new Set(['поле','поле2'])),'Поле3');
});

// Предел длины ставится, только если он назван: у комментария его нет.
// Число применяется по завершении ввода и числом, текст — на каждой букве.
test('a text control limits only when told to and applies numbers as numbers',async()=>{
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
  await new Promise(resolve=>setTimeout(resolve));
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

// Ловит запрос Studio, ушедший без заголовка X-ML-CSRF или с телом не в JSON:
// сервер отвечает на такой 403 или 415 (validateStudioMutation,
// decodeStudioJSON), а заголовок прежде выписывался у каждого из двадцати
// вызовов. И postJSON, отдавший разбор ответа с ошибкой как успех.
test('a request of Studio carries the CSRF header and its body as JSON, postJSON throws the trimmed text of a refusal',async()=>{
  const sent=[];
  context.fetch=async(path,init)=>{sent.push({path,init});return init.method==='DELETE'?{ok:false,text:async()=>'  отказ \n'}:{ok:true,json:async()=>({done:true})};};
  await context.studioRequest('/api/a');
  assert.equal(sent[0].init.method,'POST');
  assert.deepEqual({...sent[0].init.headers},{'X-ML-CSRF':'1'});
  assert.equal(sent[0].init.body,undefined);
  const answer=await context.postJSON('/api/b',{name:'Склад'},{method:'PUT'});
  assert.deepEqual({...answer},{done:true});
  assert.equal(sent[1].init.method,'PUT');
  assert.deepEqual({...sent[1].init.headers},{'X-ML-CSRF':'1','Content-Type':'application/json'});
  assert.equal(sent[1].init.body,'{"name":"Склад"}');
  await assert.rejects(context.postJSON('/api/c',{},{method:'DELETE'}),error=>error.message==='отказ');
  delete context.fetch;
});

// Ловит перевод из одних пробелов, оставленный в тексте: сервер такой
// отвергает при записи (validateTitle), а редактор проекта прежде снимал
// только совсем пустой.
test('a blank translation is taken away, a translation with text is written',()=>{
  const text=context.setTranslation(undefined,'ru','Склад');
  assert.deepEqual({...text},{ru:'Склад'});
  assert.equal(context.setTranslation(text,'en','  \n'),text);
  assert.deepEqual({...context.setTranslation(text,'ru',' ')},{});
});

// Ловит текст из имени не по описи (obshchee.md, «Текст из имени»; снимки
// obshchee/sinonim-*, naimenovanie-iz-imeni-*): слова — с заглавной после
// строчной, со второго — со строчной. Где описи нет — цифра, «_», заглавная
// без строчной после (вопрос В1 серии 2.559), — текста нет.
test('the text of a name parts its words, and makes none where the inventory says nothing',()=>{
  for(const [name,text] of [['ТестовыйСправочникОписи','Тестовый справочник описи'],['ВтороеИмя','Второе имя'],['ПервоеИмя','Первое имя'],
    ['Товары','Товары'],['товарыСклада','товары склада'],['',''],['СтавкаНДС',''],['Реквизит1',''],['Вид_операции',''],['СчетА','']])
    assert.equal(context.textFromName(name),text,name);
});

// Ловит текст, который не следует за именем, или следует и после правки
// руками, или пишется не на том языке, что показывает поле.
test('the text follows its name until it is put right by hand, in the language the field shows',()=>{
  StudioLanguage.set('ru');
  const title={en:'Goods'};
  const field=createLocalizedField({languages:[{code:'ru'},{code:'en'}],defaultLanguage:'ru'},code=>title[code],(code,value)=>context.setTranslation(title,code,value),()=>{});
  field.follow('','ПервоеИмя');
  assert.equal(title.ru,'Первое имя');assert.equal(field.children[0].value,'Первое имя');
  field.follow('ПервоеИмя','ВтороеИмя');
  assert.equal(title.ru,'Второе имя');
  field.follow('ВтороеИмя','ВтороеИмяН');
  assert.equal('ru' in title,false,'a name with no text takes the text the name made away');
  field.follow('ВтороеИмяН','ВтороеИмяНовое');
  assert.equal(title.ru,'Второе имя новое');
  title.ru='Своё';
  field.follow('ВтороеИмяНовое','ТретьеИмя');
  assert.equal(title.ru,'Своё');assert.equal(title.en,'Goods');
  delete title.en;
  StudioLanguage.set('en');
  field.follow('','ПервоеИмя');
  assert.equal(title.en,'Первое имя');assert.equal(title.ru,'Своё');
  StudioLanguage.set('ru');
  field.destroy();
});

// fontPresentation writes a font as the platform's field does (obshchee.md,
// «Значения цвета, шрифта и рамки»; snapshots series-2.595/03, 04, 59, 60).
// Defects caught: a part out of the prototype's order, a switch said off
// written, a scale of a hundred written, a base not named.
test('fontPresentation: the base or the face, the size, the switches on in order, a scale not of a hundred',()=>{
  const {fontPresentation}=context;
  const base=value=>({style:'Обычный шрифт текста',system:'Шрифт диалогов и меню'}[value.source]);
  assert.equal(fontPresentation({source:'style',from:{standard:'NormalTextFont'}},base),'стиль: Обычный шрифт текста');
  assert.equal(fontPresentation({source:'style',from:{standard:'NormalTextFont'},size:12,bold:true,italic:false,scale:150},base),
    'стиль: Обычный шрифт текста, 12, полужирный, 150%');
  assert.equal(fontPresentation({source:'absolute',face:'Al Bayan',size:8,strikeout:true,underline:true,italic:true,bold:true,scale:150},base),
    'Al Bayan, 8, полужирный, наклонный, подчеркнутый, зачеркнутый, 150%');
  assert.equal(fontPresentation({source:'system',system:'DefaultGUIFont',size:10,bold:true,scale:100},base),'Шрифт диалогов и меню, 10, полужирный');
  assert.equal(fontPresentation({source:'style',from:{item:'x'}},()=>''),'');
});

// A namespace of the platform is shown by its short name in every place -
// a type in a field, a member of a union - and taken typed short or whole,
// written whole (ML-STUDIO.md, «Пространства платформы — коротким именем»).
// The defects caught: an address shown whole where its short name is known,
// a short name typed written as typed, and a key of the object's prototype
// («constructor») taken for a namespace of the platform.
test('a namespace of the platform is shown and typed by its short name',()=>{
  vm.runInContext('Object.assign(PlatformNamespaces, {"http://v8.1c.ru/8.1/data/core": "data/core"})',context);
  const {namespaceShown,namespaceTyped,xmlTypeText,xmlTypeName}=context;
  assert.equal(xmlTypeText({namespace:'http://v8.1c.ru/8.1/data/core',name:'UUID'}),'UUID (data/core)');
  assert.equal(xmlTypeName({namespace:'http://v8.1c.ru/8.1/data/core',name:'UUID'}),'{data/core}UUID');
  assert.equal(xmlTypeText({namespace:'http://www.sample-package.org',name:'Тип'}),'Тип (http://www.sample-package.org)');
  assert.equal(namespaceShown('constructor'),'constructor');
  assert.equal(namespaceTyped(' data/core '),'http://v8.1c.ru/8.1/data/core');
  assert.equal(namespaceTyped('http://v8.1c.ru/8.1/data/core'),'http://v8.1c.ru/8.1/data/core');
  assert.equal(namespaceTyped('urn:example'),'urn:example');
});

// A letter typed in the tree of «Выбор объекта» goes to the next row on it,
// round, the letters not gathered (obshchee.md, «Видимость по ролям»;
// snapshots dosyomka-2653/20 to 24 - the rows of the roles of the ERP copy,
// the root current at first). The defects caught: the letters gathered into
// a word («Ба» staying on «Базовые…»), the search starting from the first
// row and not after the current one, no round to the first, the case taken
// into account, a row of another alphabet's letter taken, a letter no row
// begins with moving the current row.
test('a letter typed in «Выбор объекта» goes to the next row on it',()=>{
  const {rowByLetter}=context;
  const rows=['BASERP','АдминистраторСистемы','Администрирование','АдминистрированиеЗарплатаКадры','БазовыеПраваБИД','БазовыеПраваБП','БазовыеПраваБРО'];
  assert.equal(rowByLetter(rows,0,'Б'),4);
  assert.equal(rowByLetter(rows,4,'а'),1);
  assert.equal(rowByLetter(rows,4,'б'),5);
  assert.equal(rowByLetter(rows,5,'Б'),6);
  assert.equal(rowByLetter(rows,6,'б'),4);
  assert.equal(rowByLetter(rows,3,'b'),0);
  assert.equal(rowByLetter(rows,2,'я'),2);
  assert.equal(rowByLetter(rows,-1,'а'),1);
  assert.equal(rowByLetter([],-1,'а'),-1);
});
