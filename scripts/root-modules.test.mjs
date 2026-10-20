import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const html=readFileSync(new URL('../internal/studio/ui/index.html',import.meta.url),'utf8');

test('the Studio page opens a module of the root from the command of its menu',()=>{
  for(const match of html.matchAll(/<script>([\s\S]*?)<\/script>/g))new vm.Script(match[1]);
  // Какая команда открывает модуль корня, говорит сервер полем команды:
  // свой перечень модулей на странице однажды забыл модуль внешнего
  // соединения.
  const opener=html.slice(html.indexOf('async function openModule'),html.indexOf('// «Все роли» и «Все ограничения доступа» — не файлы'));
  assert.match(opener,/if\(!module\.root\)return openFile\(module\.path\)/);
  // Открытие создаёт файл, поэтому идёт как мутация — тем же запросом, что
  // и остальные изменяющие запросы Studio (studioRequest, POST по умолчанию).
  assert.match(opener,/studioRequest\(`\/api\/root-module\/\$\{encodeURIComponent\(module\.root\)\}`\)/);
  assert.match(opener,/showSourceEditor\(currentFile\)/);
});
