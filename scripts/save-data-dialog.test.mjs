import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const html=readFileSync(new URL('../internal/studio/ui/index.html',import.meta.url),'utf8');

test('the save-data click asks for nothing and only then shows what needs confirming',()=>{
  for(const match of html.matchAll(/<script>([\s\S]*?)<\/script>/g))new vm.Script(match[1]);
  assert.match(html,/id="save-data-dialog"/);
  // Первый запрос идёт БЕЗ согласий: подтверждать нечего, пока сервер не сказал, что именно.
  assert.match(html,/postSaveData\(\{objectLoss:false,valueRewrite:false\}\)/);
  // Подтверждения раздельные — это и есть суть подпункта.
  assert.match(html,/objectLoss:saveDataObjectInput\.checked,valueRewrite:saveDataValueInput\.checked/);
  // Кнопка недоступна, пока не отмечено каждое требуемое согласие.
  const updater=html.slice(html.indexOf('function updateSaveDataSubmit'),html.indexOf('saveDataObjectInput.addEventListener'));
  assert.match(updater,/!saveDataObjectLoss\.hidden&&!saveDataObjectInput\.checked/);
  assert.match(updater,/!saveDataValueRewrite\.hidden&&!saveDataValueInput\.checked/);
  // Три группы: удаление, изменение значений и то, что может не примениться.
  for(const impact of ['object_loss','value_rewrite','may_fail'])assert.ok(html.includes(impact),impact);
});
