import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const html=readFileSync(new URL('../internal/studio/ui/index.html',import.meta.url),'utf8');

test('the Studio page opens a module of the root from its own tree node',()=>{
  for(const match of html.matchAll(/<script>([\s\S]*?)<\/script>/g))new vm.Script(match[1]);
  // Какой узел — модуль корня, говорит сервер видом узла: свой перечень
  // модулей на странице однажды забыл модуль внешнего соединения.
  assert.match(html,/node\.kind==='root-module'/);
  // Открытие создаёт файл, поэтому идёт как мутация — тем же запросом, что
  // и остальные изменяющие запросы Studio (studioRequest, POST по умолчанию).
  assert.match(html,/studioRequest\(`\/api\/root-module\/\$\{encodeURIComponent\(node\.id\)\}`\)/);
  // После создания дерево перечитывается: состояние узла меняется.
  const opener=html.slice(html.indexOf('async function openRootModule'),html.indexOf('// Узел группы «Роли»'));
  assert.match(opener,/await loadTree\(false\)/);
  assert.match(opener,/showSourceEditor\(currentFile\)/);
});
