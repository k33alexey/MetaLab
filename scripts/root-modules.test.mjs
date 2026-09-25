import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const html=readFileSync(new URL('../internal/studio/ui/index.html',import.meta.url),'utf8');

test('the Studio page opens either module of the root from its own tree node',()=>{
  for(const match of html.matchAll(/<script>([\s\S]*?)<\/script>/g))new vm.Script(match[1]);
  // У корня два модуля, и открываются они одним путём: узел несёт и свой
  // адрес, и своё название, поэтому второй обработчик не нужен.
  assert.match(html,/node\.id==='session-module'\|\|node\.id==='application-module'/);
  // Открытие создаёт файл, поэтому идёт как мутация — с тем же заголовком,
  // что и остальные изменяющие запросы Studio.
  assert.match(html,/fetch\(`\/api\/root-module\/\$\{encodeURIComponent\(node\.id\)\}`,\{method:'POST',headers:\{'X-ML-CSRF':'1'\}\}\)/);
  // После создания дерево перечитывается: состояние узла меняется.
  const opener=html.slice(html.indexOf('async function openRootModule'),html.indexOf('// Узел группы «Роли»'));
  assert.match(opener,/await loadTree\(false\)/);
  assert.match(opener,/showSourceEditor\(currentFile\)/);
});
