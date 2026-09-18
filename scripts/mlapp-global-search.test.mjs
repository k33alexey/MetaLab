import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

// app.js определяет веб-компоненты; в vm нет DOM, поэтому выполняем только
// чистые функции подбора, вырезав их из файла по границам объявлений.
const source=readFileSync(new URL('../internal/mlapp/ui/app.js',import.meta.url),'utf8');
const start=source.indexOf('function globalSearchEntries');
const end=source.indexOf('class MLCommandBar');
assert.ok(start>0&&end>start,'функции глобального поиска не найдены в app.js');
const context=vm.createContext({});
vm.runInContext(source.slice(start,end)+'\nglobalThis.entries=globalSearchEntries;globalThis.matches=globalSearchMatches;',context);
const {entries,matches}=context;
// Функции исполняются в отдельном realm виртуальной машины: их массивы не
// проходят по ссылочному сравнению deepStrictEqual, поэтому сверяем по значению.
const plain=value=>JSON.parse(JSON.stringify(value));

const navigation=[
  {id:'home',title:'Начальная страница'},
  {id:'catalogs/Товары',kind:'catalogs',kindTitle:'Справочник',name:'Товары',title:'Товары',operations:['read','create','update']},
  {id:'catalogs/Контрагенты',kind:'catalogs',kindTitle:'Справочник',name:'Контрагенты',title:'Контрагенты',operations:['read']},
  {id:'documents/ПродажаТоваров',kind:'documents',kindTitle:'Документ',name:'ПродажаТоваров',title:'Продажа товаров',operations:['read','create']},
];

test('пустой запрос не предлагает ничего',()=>{
  assert.deepEqual(plain(matches(navigation,'')),[]);
  assert.deepEqual(plain(matches(navigation,'   ')),[]);
});

test('ищет по названию вида объекта, а не по данным',()=>{
  const found=plain(matches(navigation,'Товар').map(item=>`${item.title} · ${item.caption}`));
  assert.deepEqual(found,[
    'Товары · Справочник',
    'Товары: создать · Справочник · команда',
    'Продажа товаров · Документ',
    'Продажа товаров: создать · Документ · команда',
  ]);
});

test('совпадение с начала имени идёт раньше совпадения в середине',()=>{
  const found=matches(navigation,'товар');
  assert.equal(found[0].title,'Товары');
  assert.ok(found.findIndex(item=>item.title==='Продажа товаров')>0);
});

test('«создать» предлагается только там, где право есть',()=>{
  const commands=[...entries(navigation)].filter(item=>item.action==='create').map(item=>item.item.name);
  assert.deepEqual(plain(commands),['Товары','ПродажаТоваров']);
  assert.equal(matches(navigation,'Контрагент').length,1);
});

test('начальная страница и объекты без адреса в поиск не попадают',()=>{
  assert.equal(entries(navigation).some(item=>item.item.id==='home'),false);
  assert.deepEqual(plain(matches([{id:'x',title:'Без вида'}],'без')),[]);
});

test('регистр и подстрока по имени метаданных тоже находятся',()=>{
  assert.equal(matches(navigation,'ПРОДАЖАТОВАРОВ')[0].title,'Продажа товаров');
  assert.equal(matches(navigation,'нет такого').length,0);
});

test('выдача ограничена, чтобы список не превращался в дерево конфигурации',()=>{
  const many=Array.from({length:50},(_,index)=>({id:`catalogs/О${index}`,kind:'catalogs',kindTitle:'Справочник',name:`Объект${index}`,title:`Объект ${index}`,operations:['read','create']}));
  assert.equal(matches(many,'объект').length,12);
  assert.equal(matches(many,'объект',3).length,3);
});
