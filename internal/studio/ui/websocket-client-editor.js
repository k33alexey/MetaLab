'use strict';

/* The palette of a WebSocket client, which has no window (docs/block-00/
   websocket-klient.md, «Палитра»; snapshots websocket-klient/09, 11): the
   name, the synonym, the comment, two flags, the module, the address, the
   user, the password by dots, the headers - «…» opening «Настройка
   заголовков» -, two flags of the system and the timeout. The password and
   the value of a secret header lie in the protected store of the system:
   the password typed goes with the client as newPassword, which the server
   writes there (WebSocketClientEdit); a secret header is shown and not
   changed. A timeout of 0 is written as typed - zero is not thirty (the
   model). around holds the palette, its title and the opening of a module. */
function createWebSocketClientEditor(host, onChange, around = {}) {
  return paletteEditor(host, onChange, around, {
    target: source => source.client,
    subject: source => {
      const own = ownSubject(source, source.client);
      return {...own, set: (key, value) => { if (key === 'timeout') source.client.timeout = value; else own.set(key, value); },
        setPassword: value => { source.client.newPassword = value; }, typedPassword: () => source.client.newPassword};
    },
    context: () => ({windows: [], openModule: role => around.openModule?.(role), editHeaders: showHeadersWindow}),
  });
}

/* showHeadersWindow is the prototype's window «Настройка заголовков»
   (snapshot websocket-klient/06): the table «Заголовок — Значение» with
   «Добавить», «Скопировать», «Удалить», «Переместить вверх», «Переместить
   вниз» over it, OK and «Отмена». A secret header stands with its value by
   dots and is neither changed nor taken away here. Answers the headers, or
   null when cancelled. */
async function showHeadersWindow(headers) {
  const rows = structuredClone(headers || []);
  let current = rows.length ? 0 : -1;
  const content = node('div', undefined, 'headers-window');
  const table = node('table', undefined, 'headers-table');
  function cell(row, key) {
    const td = node('td');
    if (row.secret) {
      td.textContent = key === 'value' ? '•••' : row.name;
      return td;
    }
    const input = textInput(row[key] || '', value => { row[key] = value; });
    input.addEventListener('focus', () => { current = rows.indexOf(row); paint(); });
    td.append(input);
    return td;
  }
  const remove = () => { rows.splice(current, 1); current = Math.min(current, rows.length - 1); draw(); };
  const move = by => { moveInCircle(rows, current, by); current = (current + by + rows.length) % rows.length; draw(); };
  const bar = windowPanel([
    panelButton('add', 'Добавить', true, () => { rows.push({name: '', value: ''}); current = rows.length - 1; draw(); }),
    panelButton('clone', 'Скопировать', false, () => { rows.splice(current + 1, 0, {name: rows[current].name, value: rows[current].value}); current += 1; draw(); }),
    panelButton('delete', 'Удалить', false, remove),
    panelButton('move-up', 'Переместить вверх', false, () => move(-1)),
    panelButton('move-down', 'Переместить вниз', false, () => move(1)),
  ]);
  function paint() {
    const chosen = rows[current], buttons = bar.querySelectorAll('button');
    buttons[1].disabled = !chosen || chosen.secret;
    buttons[2].disabled = !chosen || chosen.secret;
    buttons[3].disabled = !chosen;
    buttons[4].disabled = !chosen;
    [...table.querySelectorAll('tbody tr')].forEach((tr, at) => tr.classList.toggle('selected', at === current));
  }
  function draw() {
    const head = node('thead'), body = node('tbody'), titles = node('tr');
    titles.append(node('th', 'Заголовок'), node('th', 'Значение'));
    head.append(titles);
    for (const row of rows) {
      const tr = node('tr');
      tr.append(cell(row, 'name'), cell(row, 'value'));
      tr.addEventListener('click', () => { current = rows.indexOf(row); paint(); });
      body.append(tr);
    }
    table.replaceChildren(head, body);
    paint();
  }
  draw();
  content.append(bar, table);
  const answer = await showDialog({title: 'Настройка заголовков', content, className: 'headers-dialog', answers: [['ok', 'ОК'], [null, 'Отмена']]});
  return answer === 'ok' ? rows.filter(row => row.secret || row.name || row.value).map(row => row.secret ? row : {name: row.name, value: row.value}) : null;
}
