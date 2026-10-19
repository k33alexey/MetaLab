import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

// Модель настройки списка (studio-list.js) не знает про DOM и проверяется
// напрямую; помощники Studio подключаются раньше, как в index.html.
const context=vm.createContext({structuredClone});
vm.runInContext(readFileSync(new URL('../internal/studio/ui/studio-common.js',import.meta.url),'utf8'),context);
vm.runInContext(readFileSync(new URL('../internal/studio/ui/studio-list.js',import.meta.url),'utf8'),context);
const plain=value=>JSON.parse(JSON.stringify(value));

// Список как «Все ограничения доступа»: ожидания ниже выписаны по снимкам
// vse-ogr-12…19, а не взяты из этого же описания.
function list(){
  return context.createListSettings({key:'probe',opening:'end',autoOpening:'beginning',sort:{column:'object'},columns:[
    {key:'object',title:'Объект'},{key:'role',title:'Роль'},{key:'condition',title:'Ограничение доступа',height:5,autoHeight:true}]});
}

// Дефект: стандартная колонка без ширины 14, высоты 1, места «Новая колонка»
// и «Изменять» — у прототипа так стоит у каждой (vse-ogr-13, vse-ogr-15).
test('a column stands as the configurator shows it unless the list says otherwise',()=>{
  const standard=plain(list().standard());
  assert.deepEqual(standard.columns[0],{key:'object',visible:true,location:'new',resize:'change',width:14,height:1,autoHeight:false});
  assert.deepEqual(standard.columns[2],{key:'condition',visible:true,location:'new',resize:'change',width:14,height:5,autoHeight:true});
  assert.equal(standard.opening,'end');
  assert.deepEqual(standard.sort,{column:'object',descending:false});
});

// Дефект: кривой файл пользователя ломает список, исчезнувшая колонка
// остаётся, новая не появляется, а значения вне выбора окна принимаются.
test('a kept setting is read as the list has it now',()=>{
  const read=plain(list().read({columns:[
    {key:'condition',visible:false,location:'next-row',resize:'keep',width:30,height:2,autoHeight:false},
    {key:'gone',visible:true},{key:'condition',visible:true},
    {key:'object',visible:'yes',location:'aside',resize:'stretch',width:0,height:1.5,autoHeight:'no'}],
    opening:'sideways',sort:{column:'gone',descending:true}}));
  assert.deepEqual(read.columns.map(column=>column.key),['condition','object','role']);
  assert.deepEqual(read.columns[0],{key:'condition',visible:false,location:'next-row',resize:'keep',width:30,height:2,autoHeight:false});
  assert.deepEqual(read.columns[1],{key:'object',visible:true,location:'new',resize:'change',width:14,height:1,autoHeight:false});
  assert.equal(read.opening,'end');
  assert.deepEqual(read.sort,{column:'object',descending:false});
  for(const broken of [null,'text',42,{columns:'x'}])assert.deepEqual(plain(list().read(broken)),plain(list().standard()));
  assert.deepEqual(plain(list().read({opening:'auto',sort:{column:'role',descending:true}})).sort,{column:'role',descending:true});
});

// Дефект: составные колонки разложены не по справке ПоложениеКолонки —
// «На следующей строке» не начинает линию под предыдущей, «В той же
// колонке» не встаёт рядом, скрытая колонка занимает место.
test('the shown columns make up the composite ones by their places',()=>{
  const model=list(),settings=model.standard();
  settings.columns[1].location='next-row';
  settings.columns[2].location='same-column';
  assert.deepEqual(plain(model.groups(settings).map(group=>group.map(line=>line.map(column=>column.key)))),[[['object'],['role','condition']]]);
  settings.columns[0].visible=false;
  assert.deepEqual(plain(model.groups(settings).map(group=>group.map(line=>line.map(column=>column.key)))),[[['role','condition']]]);
  settings.columns[0].visible=true;settings.columns[0].location='same-column';settings.columns[1].location='new';
  assert.deepEqual(plain(model.groups(settings).map(group=>group.map(line=>line.map(column=>column.key)))),[[['object']],[['role','condition']]]);
});

// Дефект: повторный щелчок по заголовку не обращает порядок, щелчок по
// другой колонке оставляет обратный порядок (vse-ogr-10, vse-ogr-11).
test('a click on a heading orders by it and a second one turns the order round',()=>{
  const model=list(),first=model.standard();
  const byRole=model.sortedBy(first,'role');
  assert.deepEqual(plain(byRole.sort),{column:'role',descending:false});
  assert.deepEqual(plain(model.sortedBy(byRole,'role').sort),{column:'role',descending:true});
  assert.deepEqual(plain(model.sortedBy(model.sortedBy(byRole,'role'),'object').sort),{column:'object',descending:false});
  assert.deepEqual(plain(first.sort),{column:'object',descending:false});
});

// Дефект: строки с одинаковым текстом теряют порядок списка (по объекту и
// роли), обратный порядок переворачивает и их (vse-ogr-18: по праву, внутри
// права — по объекту).
test('rows of the same text keep the order of the list',()=>{
  const model=list(),rows=[{id:1,role:'Б'},{id:2,role:'А'},{id:3,role:'Б'},{id:4,role:'А'}];
  const settings=model.sortedBy(model.standard(),'role');
  assert.deepEqual(plain(model.sorted(rows,settings,(row,column)=>row[column]).map(row=>row.id)),[2,4,1,3]);
  assert.deepEqual(plain(model.sorted(rows,model.sortedBy(settings,'role'),(row,column)=>row[column]).map(row=>row.id)),[1,3,2,4]);
});

// Дефект: «Стандартная настройка» не возвращает колонки или сбрасывает
// порядок списка, которого в окне нет (vse-ogr-19).
test('the standard setting takes the columns back and keeps the order',()=>{
  const model=list(),settings=model.sortedBy(model.standard(),'role');
  settings.columns.reverse();settings.columns[0].visible=false;settings.columns[0].height=9;settings.opening='beginning';
  const restored=plain(model.restored(settings));
  assert.deepEqual(restored.columns,plain(model.standard().columns));
  assert.equal(restored.opening,'end');
  assert.deepEqual(restored.sort,{column:'role',descending:false});
});

// Дефект: список открывается не на той строке — «В конец списка» не на
// последней (vse-ogr-18), «Автоматически» не так, как велит список.
test('the list opens on the row its setting names',()=>{
  const model=list(),settings=model.standard();
  assert.equal(model.openingRow(settings,5),4);
  assert.equal(model.openingRow({...settings,opening:'beginning'},5),0);
  assert.equal(model.openingRow({...settings,opening:'auto'},5),0);
  assert.equal(model.openingRow(settings,0),-1);
  const dated=context.createListSettings({key:'dated',opening:'auto',autoOpening:'end',columns:[{key:'date',title:'Дата'}]});
  assert.equal(dated.openingRow(dated.standard(),3),2);
});
