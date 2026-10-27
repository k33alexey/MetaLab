'use strict';

/* The window of a web service, as the prototype's (docs/block-00/
   web-servis.md, «Окно»; snapshots web-servis/01-04, 09, 14): the window of
   an object of rows and subsystems (objectWindowEditor) with the page
   «Операции» of its own - the operations and their parameters
   (partsOfTwoLevels). A new operation is «Операция1», returning a string of
   XMLSchema, the lock managed, its procedure to be named; a new parameter
   «Параметр1», a string, «Входной» («Новый сервис»). «…» of «Пакеты XDTO»
   opens «Выбор пакетов XDTO» (chooseXDTOPackages); «…» of a type the
   window choosing it (chooseXMLType). */
function createWebServiceEditor(host, onChange, around = {}) {
  const xmlString = () => ({namespace: 'http://www.w3.org/2001/XMLSchema', name: 'string'});
  // A type is chosen among XML Schema and the packages the service uses, by
  // their namespaces (web-servis.md, «Выбор типа из палитры параметра»).
  const typeContext = api => ({xmlTypes: () => api.source().types, typeNamespaces: () => ['http://www.w3.org/2001/XMLSchema',
    ...(api.object().packages || []).map(item => item.package ? api.source().packages.find(own => own.id === item.package)?.namespace : item.namespace).filter(Boolean)]});
  return objectWindowEditor(host, onChange, around, {target: source => source.service, own: api => ({...partsOfTwoLevels(api, 'operations', {
    list: 'operations', children: 'parameters',
    upper: {kind: 'operation', icon: 'web-service-operation', add: ['web-service-operation-add', 'Добавить операцию'], base: 'Операция',
      fields: () => ({procedure: '', returnType: xmlString(), dataLockControl: 'managed'})},
    lower: {kind: 'parameter', icon: 'parameter', add: ['parameter-add', 'Добавить параметр'], base: 'Параметр',
      fields: () => ({type: xmlString(), direction: 'in'})},
  }), context: () => typeContext(api)})});
}
