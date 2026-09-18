import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const source=readFileSync(new URL('../internal/mlapp/ui/app.js',import.meta.url),'utf8');
const start=source.indexOf('// Панель открытых окон ML App');
const end=source.indexOf('class MLCommandBar');
assert.ok(start>0&&end>start,'модель окон не найдена в app.js');
const context=vm.createContext({});
vm.runInContext(source.slice(start,end)+'\nglobalThis.api={MAX_WINDOWS,windowKey,storedWindows,restoreWindows};',context);
const {MAX_WINDOWS,windowKey,storedWindows,restoreWindows}=context.api;
const plain=value=>JSON.parse(JSON.stringify(value));

test('одно и то же открывается в одном окне, а новые записи — в разных',()=>{
  const goods={kind:'catalogs',name:'Товары',title:'Товары'};
  assert.equal(windowKey({kind:'list',object:goods}),windowKey({kind:'list',object:goods}));
  assert.notEqual(windowKey({kind:'list',object:goods}),windowKey({kind:'object',object:goods,reference:'42'}));
  assert.equal(windowKey({kind:'object',object:goods,reference:'42'}),windowKey({kind:'object',object:goods,reference:'42'}));
  // Две ещё не записанные записи — это два разных окна: общей ссылки у них нет.
  assert.notEqual(windowKey({kind:'object',object:goods,reference:'new',id:'w1'}),windowKey({kind:'object',object:goods,reference:'new',id:'w2'}));
  assert.equal(windowKey({kind:'home'}),'home');
});

test('после F5 сохраняется адрес окна, а не его содержимое',()=>{
  const windows=[
    {id:'home',kind:'home',title:'Начальная страница',object:null,reference:null,form:{huge:true}},
    {id:'w1',kind:'list',title:'Товары',object:{kind:'catalogs',name:'Товары',title:'Товары',id:'x'},reference:null,form:{rows:[1,2,3]},listState:{cursor:'abc'},modified:false},
    {id:'w2',kind:'object',title:'Товары',object:{kind:'catalogs',name:'Товары',title:'Товары',id:'x'},reference:'42',objectState:{fields:{}},modified:true},
  ];
  const stored=plain(storedWindows(windows,'w2'));
  assert.equal(stored.activeId,'w2');
  assert.deepEqual(stored.windows.map(item=>item.id),['home','w1','w2']);
  assert.equal(stored.windows[2].modified,true);
  for(const item of stored.windows){
    assert.equal('form' in item,false,'содержимое формы сохраняться не должно');
    assert.equal('listState' in item,false);
    assert.equal('objectState' in item,false);
  }
});

test('восстановление отбрасывает то, что нельзя открыть',()=>{
  const restored=plain(restoreWindows({activeId:'w9',windows:[
    {id:'home',kind:'home',title:'Начальная страница'},
    {id:'w1',kind:'list',title:'Товары',object:{kind:'catalogs',name:'Товары'}},
    {id:'w2',kind:'list',title:'Без адреса'},
    {id:'w3',kind:'object',title:'Без ссылки',object:{kind:'catalogs',name:'Товары'}},
    {id:'w1',kind:'list',title:'Дубль',object:{kind:'catalogs',name:'Товары'}},
    null,
  ]}));
  assert.deepEqual(restored.windows.map(item=>item.id),['home','w1']);
  // Активное окно, которого не осталось, заменяется первым уцелевшим.
  assert.equal(restored.activeId,'home');
});

test('пустое и испорченное хранилище не ломают панель',()=>{
  for(const stored of [null,undefined,{},{windows:'нет'},{windows:[]}]){
    const restored=plain(restoreWindows(stored));
    assert.deepEqual(restored.windows,[]);
    assert.equal(restored.activeId,null);
  }
});

test('восстанавливается не больше предела окон',()=>{
  const many=Array.from({length:MAX_WINDOWS+5},(_,index)=>({id:`w${index}`,kind:'list',title:`Окно ${index}`,object:{kind:'catalogs',name:`О${index}`}}));
  const restored=plain(restoreWindows({activeId:'w0',windows:many}));
  assert.equal(restored.windows.length,MAX_WINDOWS);
});

test('предел — около десяти окон',()=>{
  assert.equal(MAX_WINDOWS,10);
});

// Баннер обновления: ML App не перезагружает открытую форму сам — в ней может
// быть несохранённая работа.
test('обновление предлагается, а не навязывается',()=>{
  const shell=source.slice(source.indexOf('startPublicationWatch(databaseId) {'),source.indexOf('showUpdateBanner() {'));
  assert.match(shell,/30000/,'отметка публикации опрашивается отдельно от частого опроса сеанса');
  assert.doesNotMatch(shell,/location\.reload\(\)/,'сама по себе страница не перезагружается');
  const banner=source.slice(source.indexOf('showUpdateBanner() {'),source.indexOf('startSessionMonitor(databaseId) {'));
  assert.match(banner,/Обновить/);
  assert.match(banner,/Позже/);
  assert.match(banner,/location\.reload\(\)/,'перезагрузка только по кнопке');
});
