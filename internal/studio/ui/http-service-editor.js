'use strict';

/* The window of an HTTP service, as the prototype's (docs/block-00/
   http-servis.md, «Окно»; snapshots http-servis/01, 02, 09, 13, 17, 18): the
   window of an object of rows and subsystems (objectWindowEditor) -
   «Основные» with the root, «Подсистемы», «Шаблоны URL», «Управление
   сеансами», «Прочее» with the button of the module - and its palette.
   «Шаблоны URL» holds the templates and their methods (partsOfTwoLevels): a
   new template is «ШаблонURL1» of «/*», a new method «Метод1» of GET with
   its handler to be named («Новые объекты»). The root is checked when its
   field is left, in the prototype's words («Некорректный корневой URL»,
   «Корневой URL должен быть заполнен»). */
function createHTTPServiceEditor(host, onChange, around = {}) {
  return objectWindowEditor(host, onChange, around, {target: source => source.service, valueChecks: {rootUrl: '/api/http-service/root'},
    own: api => partsOfTwoLevels(api, 'templates', {
      list: 'templates', children: 'methods',
      upper: {kind: 'template', icon: 'url-template', add: ['add', 'Добавить шаблон'], base: 'ШаблонURL', fields: () => ({template: '/*'})},
      lower: {kind: 'method', icon: 'http-method', add: ['attribute-add', 'Добавить метод'], base: 'Метод', fields: () => ({method: 'GET', handler: ''})},
    })});
}
