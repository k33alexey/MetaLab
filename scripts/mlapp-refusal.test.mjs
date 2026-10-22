import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

// Страница ML App открывает базу запросом app-bootstrap. Отказ, который
// сервер пишет для человека (JSON с «error»: роль, сохранённая прежней
// версией ML, — роль и право, 2.449), страница показывает как есть; любой
// другой — «База недоступна». В vm нет DOM: веб-компоненты получают пустую
// основу, а load исполняется на заготовке оболочки без конструктора.
// Ловит: страница, которая берёт общий текст вместо слов сервера (в
// негативном контроле закрытия блока 2в такую поломку не ловил ни один тест).
const source=readFileSync(new URL('../internal/mlapp/ui/app.js',import.meta.url),'utf8');

function shellAnswering(response){
  const context=vm.createContext({
    HTMLElement:class{},
    customElements:{define(){}},
    document:{documentElement:{}},
    location:{pathname:'/app/db-1'},
    fetch:async()=>response,
  });
  vm.runInContext(source+'\nglobalThis.Shell=MLAppShell;',context);
  const shell=Object.create(context.Shell.prototype);
  shell.shown=null;
  shell.renderError=message=>{shell.shown=message;};
  return shell;
}

const answer=(status,type,body)=>({ok:false,status,headers:{get:name=>name==='Content-Type'?type:null},json:async()=>JSON.parse(body)});

test('отказ, написанный сервером для человека, страница показывает его словами',async()=>{
  const text='Роль «Кладовщик» даёт право «Тонкий клиент», которого ML не хранит.';
  const shell=shellAnswering(answer(409,'application/json; charset=utf-8',JSON.stringify({error:text})));
  await shell.load();
  assert.equal(shell.shown,text);
});

test('прочий отказ — «База недоступна», вход — «Требуется вход»',async()=>{
  for(const [response,expected] of [
    [answer(409,'text/plain; charset=utf-8','ML App metadata unavailable'),'База недоступна'],
    [answer(409,'application/json','не JSON'),'База недоступна'],
    [answer(409,'application/json','{}'),'База недоступна'],
    [answer(401,'text/plain',''),'Требуется вход'],
  ]){
    const shell=shellAnswering(response);
    await shell.load();
    assert.equal(shell.shown,expected);
  }
});
