'use strict';

/* The window of a web service, as the prototype's (docs/block-00/
   web-servis.md, «Окно»; snapshots web-servis/01-04, 09, 14): the window of
   an object of rows and subsystems (objectWindowEditor) with the page
   «Операции» of its own - the operations and their parameters
   (partsOfTwoLevels). A new operation is «Операция1», returning a string of
   XMLSchema, the lock managed, its procedure to be named; a new parameter
   «Параметр1», a string, «Входной» («Новый сервис»). «…» of «Пакеты XDTO»
   and of the types is unavailable until the windows choosing packages and
   types. */
function createWebServiceEditor(host, onChange, around = {}) {
  const xmlString = () => ({namespace: 'http://www.w3.org/2001/XMLSchema', name: 'string'});
  return objectWindowEditor(host, onChange, around, {target: source => source.service, own: api => partsOfTwoLevels(api, 'operations', {
    list: 'operations', children: 'parameters',
    upper: {kind: 'operation', icon: 'web-service-operation', add: ['add', 'Добавить операцию'], base: 'Операция',
      fields: () => ({procedure: '', returnType: xmlString(), dataLockControl: 'managed'})},
    lower: {kind: 'parameter', icon: 'parameter', add: ['attribute-add', 'Добавить параметр'], base: 'Параметр',
      fields: () => ({type: xmlString(), direction: 'in'})},
  })});
}
