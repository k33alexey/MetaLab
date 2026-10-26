'use strict';

/* The window of an HTTP service, as the prototype's (docs/block-00/
   http-servis.md, «Окно»; snapshots http-servis/01, 02, 17, 18): the window
   of an object of rows and subsystems (objectWindowEditor) - «Основные» with
   the root, «Подсистемы», «Шаблоны URL», «Управление сеансами», «Прочее»
   with the button of the module - and its palette. The root is checked when
   its field is left, in the prototype's words («Некорректный корневой URL»,
   «Корневой URL должен быть заполнен»). «Шаблоны URL» stands empty until its
   item. */
function createHTTPServiceEditor(host, onChange, around = {}) {
  return objectWindowEditor(host, onChange, around, {target: source => source.service, valueChecks: {rootUrl: '/api/http-service/root'}});
}
