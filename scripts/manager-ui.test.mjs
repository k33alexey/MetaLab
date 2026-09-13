import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const html = readFileSync(new URL('../internal/manager/ui/index.html', import.meta.url), 'utf8');
const source = [...html.matchAll(/<script>([\s\S]*?)<\/script>/g)].map(match => match[1]).join('\n');

function harness() {
  const nodes = new Map(), pending = [];
  function node() {
    const classes = new Set();
    return {
      value: '', textContent: '', children: [], checked: false, events: {},
      get selectedOptions() { return this.children.filter(child=>child.value===this.value); },
      classList: {
        add: value => classes.add(value), remove: value => classes.delete(value),
        toggle: (value, on) => on ? classes.add(value) : classes.delete(value),
        contains: value => classes.has(value),
      },
      addEventListener(name, callback) { this.events[name] = callback; },
      replaceChildren(...items) { this.children = items; this.textContent = ''; },
      append(...items) { this.children.push(...items); },
      reset() {}, setAttribute() {},
    };
  }
  function get(id) { if (!nodes.has(id)) nodes.set(id, node()); return nodes.get(id); }
  const context = vm.createContext({
    document: { querySelector: get, getElementById: id => get(`#${id}`), querySelectorAll: () => [...nodes.entries()].filter(([id]) => id.endsWith('-password')).map(([, value]) => value), createElement: node, createTextNode: value => value },
    fetch: (path,options) => new Promise(resolve => { pending.push({path,options,resolve}); }),
    Option: function(text,value) { this.textContent=text;this.value=value; },
    setInterval() {}, setTimeout() {}, console,
  });
  vm.runInContext(source, context); // Initial status request remains pending, no network or timers.
  const run = code => vm.runInContext(code, context);
  const reply = (path, body, occurrence = 0, status = 200) => {
    const index = pending.map((request,index)=>request.path===path?index:-1).filter(index=>index>=0)[occurrence] ?? -1;
    assert.notEqual(index, -1, `Missing request: ${path}`);
    pending.splice(index, 1)[0].resolve({ok: status>=200&&status<300, status, json: async () => body, text:async()=>String(body)});
  };
  return {run, reply, get, pending};
}

const signIn = `showManagerSession({id:'alice-session',login:'alice',platformAdministrator:true,metadataAdministrator:true,mustChangePassword:false})`;

const roleView = {projectId:'project',available:[{id:'reader',name:'Читатель',title:{ru:'Читатель'}},{id:'writer',name:'Редактор',title:{ru:'Редактор'}}],assignment:{projectId:'project',roleIds:['reader'],revision:3}};
const rolePath='/api/databases/base/application-roles/user';
async function openedRoles(h) {
  h.run(signIn);h.run(`permissionsDatabase='base'`);
  const opening=h.run(`openApplicationRoles({userId:'user',login:'Пользователь',appAccess:true})`);
  h.reply(rolePath,structuredClone(roleView));await opening;
}

test('project role controls preserve explicit selections without adding privileges',async()=>{
  const h=harness();await openedRoles(h);
  assert.equal(h.get('#application-role-user').textContent,'Пользователь');
  assert.equal(h.run('applicationRoleState.choices[0].input.checked'),true);
  assert.equal(h.run('applicationRoleState.choices[1].input.checked'),false);
  assert.equal(h.get('#application-role-save').disabled,false);
});
test('role responses cannot reappear after logout or replace a different user',async()=>{
  const h=harness();h.run(signIn);h.run(`permissionsDatabase='base'`);
  const old=h.run(`openApplicationRoles({userId:'user',login:'Первый',appAccess:true})`);
  const fresh=h.run(`openApplicationRoles({userId:'next',login:'Второй',appAccess:true})`);
  h.reply(rolePath,roleView);await old;
  assert.equal(h.get('#application-role-user').textContent,'Второй');assert.equal(h.get('#application-role-choices').children.length,0);
  h.run('showManagerSession(null)');h.reply('/api/databases/base/application-roles/next',roleView);await fresh;
  assert.equal(h.get('#application-role-choices').children.length,0);assert.equal(h.get('#application-role-form').classList.contains('hidden'),true);
});
test('saving roles freezes choices and sends only IDs with the captured revision',async()=>{
  const h=harness();await openedRoles(h);h.run('applicationRoleState.choices[1].input.checked=true');
  const save=h.run('saveApplicationRoles({preventDefault(){}})');
  const request=h.pending.find(item=>item.path===rolePath);
  assert.deepEqual(JSON.parse(request.options.body),{projectId:'project',expectedRevision:3,roleIds:['reader','writer']});
  assert.equal(h.get('#application-role-choices').inert,true);
  await h.run('saveApplicationRoles({preventDefault(){}})');assert.equal(h.pending.filter(item=>item.path===rolePath).length,1);
  h.reply(rolePath,{projectId:'project',roleIds:['reader','writer'],revision:4});await save;
  assert.equal(h.run('applicationRoleState.view.assignment.revision'),4);assert.equal(h.get('#application-role-choices').inert,false);
});
test('stale role saves require reload and never automatically overwrite the server',async()=>{
  const h=harness();await openedRoles(h);
  const save=h.run('saveApplicationRoles({preventDefault(){}})');h.reply(rolePath,'conflict',0,409);await save;
  assert.equal(h.get('#application-role-save').disabled,true);assert.equal(h.get('#application-role-choices').inert,false);
  await h.run('saveApplicationRoles({preventDefault(){}})');assert.equal(h.pending.filter(item=>item.path===rolePath).length,0);
});
test('failed role saves retain choices for a deliberate retry',async()=>{
  const h=harness();await openedRoles(h);h.run('applicationRoleState.choices[1].input.checked=true');
  const save=h.run('saveApplicationRoles({preventDefault(){}})');h.reply(rolePath,'denied',0,403);await save;
  assert.equal(h.run('applicationRoleState.choices[1].input.checked'),true);assert.equal(h.get('#application-role-save').disabled,false);
  assert.equal(h.run('applicationRoleState.view.assignment.revision'),3);
});
test('a late save response cannot modify the next role editor',async()=>{
  const h=harness();await openedRoles(h);const save=h.run('saveApplicationRoles({preventDefault(){}})');
  const opening=h.run(`openApplicationRoles({userId:'next',login:'Второй',appAccess:true})`);
  h.reply(rolePath,{revision:99});await save;
  assert.equal(h.get('#application-role-user').textContent,'Второй');assert.equal(h.get('#application-role-save').disabled,true);
  h.reply('/api/databases/base/application-roles/next',roleView);await opening;
  assert.equal(h.run('applicationRoleState.view.assignment.revision'),3);
});
test('missing published roles remain visible until explicitly removed',async()=>{
  const h=harness();h.run(signIn);h.run(`permissionsDatabase='base'`);
  const open=h.run(`openApplicationRoles({userId:'user',login:'Пользователь',appAccess:true})`);
  h.reply(rolePath,{...roleView,assignment:{projectId:'project',roleIds:['removed'],revision:3}});await open;
  assert.equal(h.run('applicationRoleState.choices.length'),3);
  assert.equal(h.run(`applicationRoleState.choices.find(choice=>choice.id==='removed').input.checked`),true);
});

test('late personal data never reappears after logout', async () => {
  const {run, reply, get} = harness();
  run(signIn);
  const request = run('refreshManagerUsers()');
  run('showManagerSession(null)');
  reply('/api/manager/users', [{id:'alice',login:'alice'}]);
  await request;
  assert.equal(get('#manager-user-list').children.length, 0);
  assert.equal(get('#manager-users').classList.contains('hidden'), true);
  assert.equal(get('#manager-user').textContent, 'MetaLab');
});

test('late heartbeat cannot restore a logged-out Manager session', async () => {
  const {run, reply, get} = harness();
  run(signIn);
  const request = run('refreshManagerSession()');
  run('showManagerSession(null)');
  reply('/api/manager/session', {id:'alice-session',login:'alice',platformAdministrator:true});
  await request;
  assert.equal(get('#manager-user').textContent, 'MetaLab');
  assert.equal(get('#manager-login').classList.contains('hidden'), false);
});

test('role changes invalidate responses without losing the current identity', async () => {
  const {run, reply, get} = harness();
  run(signIn);
  const request = run(`managerItems('/api/databases')`);
  run(`showManagerSession({id:'alice-session',login:'alice',metadataAdministrator:true,platformAdministrator:false,mustChangePassword:false})`);
  reply('/api/databases', [{name:'Private administrator database'}]);
  assert.equal(await request, null);
  assert.equal(get('#manager-user').textContent, 'alice');
  assert.equal(get('#manager-users').classList.contains('hidden'), true);
});

test('mandatory password change hides operational controls', () => {
  const {run, get} = harness();
  run(`showManagerSession({id:'temporary',login:'alice',metadataAdministrator:true,platformAdministrator:true,mustChangePassword:true})`);
  for (const id of ['databases','studio-section','manager-users']) assert.equal(get(`#${id}`).classList.contains('hidden'), true);
  assert.equal(get('#manager-password').classList.contains('hidden'), false);
});

test('editing an existing assignment fills all three independent flags', async () => {
  const {run,reply,get}=harness();
  run(signIn);
  const request=run(`openPermissions({id:'database',name:'My database'})`);
  reply('/api/databases/database/permissions',[{login:'developer',appAccess:false,studioAccess:true,databaseAdministrator:false}]);
  await request;
  get('#permissions-current').children[0].children[0].events.click();
  assert.equal(get('#permissions-login').value,'developer');
  assert.equal(get('#permissions-app').checked,false);
  assert.equal(get('#permissions-studio').checked,true);
  assert.equal(get('#permissions-admin').checked,false);
});

test('periodic refresh never erases a password while it is being typed', () => {
  const {run,get}=harness();
  get('#manager-login-password').value='partly typed password';
  run('showManagerSession(null)');
  assert.equal(get('#manager-login-password').value,'partly typed password');
  const temporary=`showManagerSession({id:'temporary',login:'alice',metadataAdministrator:true,platformAdministrator:false,mustChangePassword:true})`;
  run(temporary);
  get('#manager-new-password').value='partly typed new password';
  run(temporary);
  assert.equal(get('#manager-new-password').value,'partly typed new password');
  run('showManagerSession(null)');
  assert.equal(get('#manager-new-password').value,'');
});

const databases = [
  {id:'one',name:'Продажи',mode:'primary',state:'running',owner:{userId:'alice',login:'Алиса'}},
  {id:'two',name:'Продажи — отладка',mode:'debug',state:'stopped',owner:{userId:'bob',login:'Борис'}},
  {id:'three',name:'Архив',mode:'debug',state:'error'},
  {id:'four',name:'Продажи — тест',mode:'debug',state:'stopped',owner:{userId:'alice',login:'Алиса'}},
].map(item=>({...item,permissions:{studio:true},connection:{host:'localhost',port:5432,database:item.id}}));

test('name, mode, state and stable owner ID filters are combined', () => {
  const {run,get}=harness();run(signIn);
  run(`managerDatabases=${JSON.stringify(databases)}`);
  const ids=()=>JSON.parse(run('JSON.stringify(filteredDatabases(managerDatabases).map(item=>item.id))'));
  assert.deepEqual(ids(),['one','two','three','four']);
  get('#database-search').value=' ПРОДАЖИ ';
  get('#database-filter').value='debug';
  get('#database-state-filter').value='stopped';
  get('#database-owner-filter').value='alice';
  assert.deepEqual(ids(),['four']);
  get('#database-owner-filter').value='bob';assert.deepEqual(ids(),['two']);
  get('#database-owner-filter').value='Борис';assert.deepEqual(ids(),[]);
  for(const id of ['database-search','database-filter','database-state-filter'])get(`#${id}`).value='';
  get('#database-owner-filter').value='unassigned';assert.deepEqual(ids(),['three']);
});

test('rendering offers only visible owners and clearly reports no matches', () => {
  const {run,get,pending}=harness();run(signIn);
  run(`managerDatabases=${JSON.stringify(databases)};renderDatabases()`);
  assert.deepEqual(get('#database-owner-filter').children.map(option=>option.value),['','unassigned','alice','bob']);
  assert.equal(get('#database-list').children.length,4);
  const requestCount=pending.length;
  get('#database-search').value='Ничего';
  get('#database-search').events.input();
  assert.equal(pending.length,requestCount,'typing must not refetch PostgreSQL-backed data');
  assert.equal(get('#database-count').textContent,'Показано 0 из 4');
  assert.equal(get('#database-empty').classList.contains('hidden'),false);
  assert.equal(get('#database-empty').textContent,'По выбранным фильтрам базы не найдены.');
  get('#database-filters-reset').events.click();
  assert.equal(get('#database-count').textContent,'Показано 4 из 4');
  assert.equal(get('#database-empty').classList.contains('hidden'),true);
});

test('owner selection survives refresh, and disappearing owners do not broaden results', () => {
  const {run,get}=harness();run(signIn);
  run(`managerDatabases=${JSON.stringify(databases)};renderDatabases()`);
  get('#database-owner-filter').value='bob';
  get('#database-state-filter').value='stopped';
  run('renderDatabases()');assert.equal(get('#database-count').textContent,'Показано 1 из 4');
  run(`managerDatabases=${JSON.stringify(databases.filter(item=>item.owner?.userId!=='bob'))};renderDatabases()`);
  assert.equal(get('#database-owner-filter').value,'bob');
  assert.equal(get('#database-owner-filter').selectedOptions[0].textContent,'Борис');
  assert.equal(get('#database-state-filter').value,'stopped');
  assert.equal(get('#database-count').textContent,'Показано 0 из 3');
});

test('logout clears cached databases, owners and filter preferences', () => {
  const {run,get}=harness();run(signIn);
  run(`managerDatabases=${JSON.stringify(databases)};renderDatabases()`);
  get('#database-owner-filter').value='bob';get('#database-search').value='Продажи';
  run('showManagerSession(null)');
  assert.equal(run('managerDatabases.length'),0);
  assert.equal(get('#database-owner-filter').value,'');assert.equal(get('#database-search').value,'');
  assert.deepEqual(get('#database-owner-filter').children.map(option=>option.value),['','unassigned']);
  assert.equal(get('#database-list').children.length,0);
  assert.equal(get('#database-count').textContent,'');
});

test('a late registry response cannot restore revoked databases', async () => {
  const {run,reply,get}=harness();run(signIn);
  const older=run('refreshDatabases()'),newer=run('refreshDatabases()');
  reply('/api/databases',[],1);
  await new Promise(setImmediate);
  reply('/api/studio/sessions',[]);
  await newer;
  reply('/api/databases',databases);
  await new Promise(setImmediate);
  reply('/api/studio/sessions',[]);
  await older;
  assert.equal(run('managerDatabases.length'),0);
  assert.equal(get('#database-count').textContent,'Показано 0 из 0');
  assert.equal(get('#database-empty').textContent,'Нет доступных баз.');
});
