/* The main menu of Studio (STUDIO-SHELL.md, «Главное меню и команды
   прототипа», «Верхней панели с кнопками нет»): the prototype's menus with
   the commands Studio makes, in the prototype's order, with its words, keys
   and icons as its menus show them (snapshots block-00/obolochka/02-27,
   otladka/02), and the owner's choices of 19.10.2026: «Проект» for the
   prototype's «Конфигурация» and no «1С» nor «конфигурация» in a text
   (ML-STUDIO.md, «„Проект" вместо „конфигурации"»). Its make-up is shown
   before its commands work: a command does nothing yet. The keys are the
   prototype's on the Mac; their one table, reassignable, is an item of
   block 3. Menus of an editor («Текст», «Форма», …) come with their
   editors.

   The menu is written as a table, a line to a command: a menu at the left
   edge, its commands two spaces in, a submenu's two more; a command is
   its text, its key, its icon and ✓ when it is on, divided by |; ---
   is a line between groups; a submenu ends with ▸. */
const studioMainMenuTable = `
Файл
  Закрыть                              |                     | file-close
  Сохранить данные                     | Cmd+S               | save
  ---
  Сравнить файлы...                    |                     | compare-files
  ---
  Выход
Правка
  Отменить                             | Cmd+Z               | undo
  Вернуть                              | Cmd+Y               | redo
  ---
  Вырезать                             | Cmd+X               | cut
  Копировать                           | Cmd+C               | copy
  Вставить                             | Cmd+V               | paste
  Очистить
  ---
  Выделить все                         | Cmd+A
  ---
  Найти...                             | Cmd+F               | find
  Найти следующий                      | F3                  | find-next
  Найти предыдущий                     | Shift+F3            | find-previous
  Заменить...                          | Cmd+H               | replace
  ---
  Глобальный поиск...                  | Cmd+Shift+F         | global-search
  Глобальная замена...
  ---
  Список закладок...                   | Cmd+Shift+F2        | bookmarks
  Редактирование текстов интерфейса... |                     | interface-texts
  ---
  Найти в дереве                       | Cmd+T               | find-in-tree
  Дополнительно                        | Option+Shift+Enter  | additional
  Свойства                             | Option+Enter        | properties
Проект
  Сохранить данные                     | F7                  | database-update
  ---
  Загрузить из XML...                  |                     | load-xml
  ---
  Git ▸                                |                     | git
    Получить                           |                     | git-pull
    Зафиксировать...                   |                     | git-commit
    Отправить                          |                     | git-push
    Ветки...                           |                     | git-branch
    Сравнить...                        |                     | git-diff
    ---
    Обновить дерево проекта            |                     | refresh
  ---
  Проверка модулей
  Проверка проекта...                  |                     | check-configuration
  ---
  Язык редактирования проекта...       |                     | language
Отладка
  Продолжить отладку                   | F5                  | debug-continue
  Остановить                           |                     | debug-pause
  Остановка по ошибке...
  ---
  Шагнуть в                            | F11                 | step-into
  Шагнуть через                        | F10                 | step-over
  Шагнуть из                           | Shift+F11           | step-out
  Идти до курсора                      | Shift+F10           | run-to-cursor
  Текущая строка                       |                     | current-line
  ---
  Точка останова                       | F9                  | breakpoint
  Точка останова с параметрами...      | Cmd+F9              | breakpoint-condition
  Включить/отключить точку останова    | Cmd+Shift+F9        | breakpoint-toggle
  Удалить все точки останова           |                     | breakpoints-delete
  Отключить все точки останова
  Список точек останова...             | Option+F9
  Сдвинуть точку останова вверх        | Option+Shift+Up     | breakpoint-up
  Сдвинуть точку останова вниз         | Option+Shift+Down   | breakpoint-down
  Вырезать точку останова              | Cmd+Option+X        | breakpoint-cut
  Скопировать точку останова           | Cmd+Option+K        | breakpoint-copy
  Вставить точку останова              | Cmd+Option+V        | breakpoint-paste
  ---
  Вычислить выражение...               | Shift+F9            | evaluate-expression
  Табло                                | Cmd+Option+W        | watch
  Локальные переменные                 |                     | watch
  Стек вызовов                         | Cmd+Option+C        | call-stack
  Замер производительности             |                     | performance-measurement
Сервис
  Открыть приложение                   | Cmd+Option+Shift+F5 | run
  ---
  Журнал регистрации...                |                     | event-log
  ---
  Шаблоны текста                       | Cmd+Shift+T         | text-templates
  ---
  Настройки...                         |                     | settings
Окна
  Новое окно                           |                     | window-new
  Горизонтально разделить окно         |                     | split-horizontal
  Вертикально разделить окно           |                     | split-vertical
  ---
  Закрыть
  Закрыть все
  ---
  Служебные окна ▸
    Служебные сообщения                | Cmd+Option+O        | service-messages
    Результаты поиска
    Терминал
    ---
    Панель состояния                   |                     |                         | ✓
    Панель окон                        |                     |                         | ✓
  ---
  Переместиться назад                  | Cmd+-               | navigate-back
  Переместиться вперед                 | Cmd+Shift+-         | navigate-forward
  ---
  Окна...                              |                     | windows
Справка
  Справка                              | F1                  | help
  Содержание справки                   | Shift+F1
  Индекс справки                       | Option+Shift+F1
  Поиск по справке                     | Option+F1
  ---
  Синтакс-помощник                     | Cmd+Shift+F1        | syntax-assistant
  Поиск в Синтакс-Помощнике            | Cmd+F1              | syntax-search
  ---
  О программе...                       |                     | about
`;

/* readMainMenu reads the table into menus: {text, items}, an item being
   [text, key, icon, checked], null for a line, or a submenu
   {text, icon, items}. */
function readMainMenu(table) {
  const menus = [], open = [];
  for (const line of table.split('\n')) {
    if (!line.trim()) continue;
    const depth = (line.length - line.trimStart().length) / 2;
    const [text, key = '', icon = '', mark = ''] = line.trim().split('|').map(cell => cell.trim());
    const item = !depth ? {text, items: []} : text === '---' ? null
      : text.endsWith(' ▸') ? {text: text.slice(0, -2), icon, items: []} : [text, key, icon, mark === '✓'];
    if (depth) open[depth - 1].items.push(item);
    else menus.push(item);
    if (item && !Array.isArray(item)) open[depth] = item;
  }
  return menus;
}
const studioMainMenu = readMainMenu(studioMainMenuTable);

/* createMainMenu draws the menu in host: a menu opens by a click on its
   name, and while one is open the pointer over another name opens that
   one; a submenu opens to the right of its row. chosen(path) is told the
   command clicked, its menus and its text joined by « → ». Escape and a
   click elsewhere close the menu. */
function createMainMenu(host, chosen) {
  host.setAttribute('role', 'menubar');
  let open = null;
  function close() {
    for (const list of document.querySelectorAll('.main-menu-list')) list.remove();
    for (const button of host.querySelectorAll('.main-menu-title')) button.classList.remove('selected');
    open = null;
  }
  // The lists open at level and deeper, closed when another opens there.
  function closeFrom(level) {
    for (const list of document.querySelectorAll('.main-menu-list')) if (Number(list.dataset.level) >= level) list.remove();
  }
  function list(items, x, y, path, level) {
    closeFrom(level);
    const menu = node('div', undefined, 'context-menu main-menu-list');
    menu.dataset.level = level;
    menu.setAttribute('role', 'menu');
    // The column of icons, as in the prototype's menus: a command without
    // an icon keeps its place, a command that is on shows a check there.
    const icons = items.some(item => item && (Array.isArray(item) ? item[2] || item[3] : item.icon));
    for (const item of items) {
      if (!item) { menu.append(node('div', undefined, 'context-menu-separator')); continue; }
      const sub = !Array.isArray(item), text = sub ? item.text : item[0];
      const row = node('button', text, 'context-menu-item');
      row.type = 'button';
      row.setAttribute('role', sub ? 'menuitem' : item[3] ? 'menuitemcheckbox' : 'menuitem');
      if (icons) row.prepend(!sub && item[3] ? node('span', '✓', 'main-menu-check') : iconElement(sub ? item.icon : item[2]));
      if (!sub && item[3]) row.setAttribute('aria-checked', 'true');
      if (sub) row.append(node('span', '▸', 'context-menu-key'));
      else if (item[1]) row.append(node('span', item[1], 'context-menu-key'));
      row.addEventListener('pointerenter', () => {
        if (sub) { const box = row.getBoundingClientRect(); list(item.items, box.right, box.top - 4, [...path, text], level + 1); }
        else closeFrom(level + 1);
      });
      row.addEventListener('click', () => { if (sub) return; close(); chosen([...path, text].join(' → ')); });
      menu.append(row);
    }
    document.body.append(menu);
    placeMenu(menu, x, y);
  }
  function show(menu, button) {
    close();
    open = menu;
    button.classList.add('selected');
    const box = button.getBoundingClientRect();
    list(menu.items, box.left, box.bottom, [menu.text], 0);
  }
  for (const menu of studioMainMenu) {
    const button = node('button', menu.text, 'main-menu-title');
    button.type = 'button';
    button.setAttribute('aria-haspopup', 'menu');
    button.addEventListener('click', () => { if (open === menu) close(); else show(menu, button); });
    button.addEventListener('pointerenter', () => { if (open && open !== menu) show(menu, button); });
    host.append(button);
  }
  addEventListener('mousedown', event => { if (open && !event.target.closest('.main-menu-list, .main-menu-title')) close(); }, true);
  addEventListener('keydown', event => { if (open && event.key === 'Escape') close(); });
  return {close};
}
