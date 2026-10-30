'use strict';

/* The window of a common picture, as the prototype's (docs/block-00/
   obshchaya-kartinka.md, «Окно»; snapshot obshchaya-kartinka/02-okno): no
   pages - the name, the synonym, the comment, the subsystems and the two
   flags of availability at the top; below, the picture at the left and its
   buttons at the right; «Тип картинки» under it. The palette beside it shows
   the same name, synonym, comment and flags (source.palette). An image
   chosen or cleared, and a set of variants turned on, edited or off, is the
   window's until «Сохранить», which writes it with the description (ML-
   STUDIO.md, «Набор вариантов»): «Включить набор вариантов» makes the image
   the one variant, «Малая (ldpi)», with no question; «Выключить» leaves the
   image of one variant (setImage); «Выбрать из файла» and «Очистить» put an
   image or none in place of a set. A set shows the variant for the density
   of the screen (screenVariant) and the type zip. The buttons of the
   transparency stand unavailable: they are an item of their own. The
   prototype's «Редактировать» is here only for a set, which it opens the
   editor of (showPictureSetEditor): Studio has no editor of pictures, a
   picture is loaded from a file and saved to one (the owner's decision of
   15.10.2026; question В1 of series 2.631). around holds the palette and
   its title. */
function createPictureEditor(host, onChange, around = {}) {
  // What the window gives the picture (pictureChoice), and the picture as
  // the server read it.
  const choice = {image: null, set: null};
  let source = null;
  const picture = () => source.picture;
  function setText(key, value) {
    if (typeof value === 'string' ? value : Object.keys(value || {}).length) picture()[key] = value; else delete picture()[key];
  }
  function setFlag(key, value) {
    if (value) picture()[key] = true; else delete picture()[key];
    // Unavailable for appearance is unavailable for choice too: the help
    // makes the one editable only while the other is on.
    if (key === 'availableForChoice' && !value) delete picture().availableForAppearance;
  }
  function draw(body, api) {
    const {typed, changed} = api;
    body.append(...windowHeadFields(source, picture(), typed, api.cleanups(), api.subsystems()),
      checkboxField('Доступность для выбора', !!picture().availableForChoice, value => { setFlag('availableForChoice', value); changed(); }),
      checkboxField('Доступность для оформления', !!picture().availableForAppearance, value => { setFlag('availableForAppearance', value); changed(); },
        'catalog-check', !picture().availableForChoice));
    const folder = {...source, imageAddress: name => `/api/picture/image?path=${encodeURIComponent(source.path)}&name=${encodeURIComponent(name)}`};
    const view = pictureChoice(choice, folder, picture().name, changed), buttons = node('div', undefined, 'picture-window-buttons');
    for (const [caption, enabled, act] of view.commands(true)) {
      const element = node('button', caption);
      element.type = 'button';
      element.disabled = !enabled();
      element.addEventListener('click', act);
      buttons.append(element);
    }
    const middle = node('div', undefined, 'picture-window-middle');
    middle.append(view.image, buttons);
    body.append(middle, view.kind);
  }
  return objectWindowEditor(host, onChange, around, {target: opened => opened.picture, own: api => ({className: 'picture-window',
    draw: body => { source = api.source(); draw(body, api); },
    subject: opened => ({target: opened.picture, get: key => opened.picture[key], set: (key, value) => key === 'comment' ? setText(key, value) : setFlag(key, value),
      setName: value => { opened.picture.name = value; }, check: () => ({path: opened.path, name: opened.picture.name})}),
    context: () => ({windows: ['picture'], openWindow: () => host.querySelector('input')?.focus()}),
    reset: () => { choice.image = null; choice.set = null; },
    value: object => pictureChoiceValue(choice, object)})});
}

/* pictureChoice is the picture a window chooses and its commands, one for
   the window of a common picture (obshchaya-kartinka.md, «Окно»; snapshot
   obshchaya-kartinka/02-okno) and for «Выбор картинки» of the picture of
   the main section (konfiguraciya.md, «Досъёмка 30.10.2026»; snapshots
   dosyomka-2651/07, 08). choice is what the window gives - image, an image
   chosen ({format, content}), of the folder ({file}) or none ({}); set, the
   rows of a set; null for each is what the folder has - kept until the
   window writes it (ML-STUDIO.md, «Набор вариантов»): «Включить набор
   вариантов» makes the image the one variant, «Малая (ldpi)», with no
   question; «Выключить» leaves the image of one variant (setImage);
   «Выбрать из файла» and «Очистить» put an image or none in place of a set.
   A set shows the variant for the density of the screen (screenVariant) and
   the type zip. The buttons of the transparency stand unavailable: they are
   an item of their own. The prototype's «Редактировать» is here only for a
   set, which it opens the editor of (showPictureSetEditor): Studio has no
   editor of pictures, a picture is loaded from a file and saved to one (the
   owner's decision of 15.10.2026; question В1 of series 2.631). folder is
   the server's: images, set, formats, variantChoices, and imageAddress(name)
   of an image of it; name names the picture; changed() follows a change.
   Answers {image, kind, commands(saving)}: the image drawn, «Тип картинки»
   under it, and the commands [caption, enabled(), act] - «Сохранить в
   файл» where saving, as the window of a common picture has it. */
function pictureChoice(choice, folder, name, changed) {
  // address is the image a variant, or the window's image, draws.
  const address = item => item.image?.content ? `data:${pictureMedia(item.image.format)};base64,${item.image.content}` : item.file ? folder.imageAddress(item.file) : '';
  // The set the window shows: the one given, or the folder's when no image
  // was given; null for a picture of one image or none.
  function currentSet() {
    if (choice.image) return null;
    if (choice.set) return choice.set;
    return folder.set?.length ? folder.set.map(variant => ({...variant})) : null;
  }
  // The image the window shows: of a set, chosen, cleared, or the one in
  // the folder.
  function shownImage() {
    const rows = currentSet();
    if (rows) {
      const row = screenVariant(rows, (window.devicePixelRatio || 1) * 100);
      return row ? {url: address(row), format: 'zip'} : null;
    }
    if (choice.image?.content) return {url: address({image: choice.image}), format: choice.image.format};
    const file = choice.image ? choice.image.file : folder.images[0];
    return file ? {url: folder.imageAddress(file), format: pictureFormat({file}), name: file} : null;
  }
  const shown = shownImage(), rows = currentSet(), image = node('div', undefined, 'picture-window-image');
  if (shown) {
    const drawn = node('img');
    drawn.src = shown.url;
    drawn.alt = name;
    image.append(drawn);
  }
  const kind = node('div', undefined, 'picture-window-type');
  kind.append(node('span', 'Тип картинки:'), node('span', rows ? 'zip' : shown?.format || ''));
  const commands = saving => [
    // «Выбрать из файла»: a file of one of the formats, read whole.
    ['Выбрать из файла', () => true, async () => {
      const chosen = await choosePictureFile(folder.formats);
      if (!chosen) return;
      choice.image = chosen; choice.set = null;
      changed();
    }],
    ['Очистить', () => !!shown || !!rows, () => { choice.image = {}; choice.set = null; changed(); }],
    ['Установить прозрачный фон', () => false, () => {}],
    ['Убрать прозрачность', () => false, () => {}],
    ['Включить набор вариантов', () => !rows, () => {
      const image = choice.image, first = image?.content ? {image} : image?.file ? {file: image.file} : !image && folder.images[0] ? {file: folder.images[0]} : null;
      choice.set = first ? [{density: 100, ...first}] : [];
      choice.image = null;
      changed();
    }],
    ['Выключить набор вариантов', () => !!rows, () => {
      const row = setImage(rows, folder.variantChoices.densities.map(item => item.density));
      choice.image = row?.image ? row.image : row ? {file: row.file} : {};
      choice.set = null;
      changed();
    }],
    ['Редактировать', () => !!rows, async () => {
      const edited = await showPictureSetEditor(name, rows, folder.variantChoices, address, folder.formats);
      if (edited) { choice.set = edited; changed(); }
    }],
    // «Сохранить в файл»: the image as the browser saves a download, named
    // after the picture.
    ...(saving ? [['Сохранить в файл', () => !!shown && !rows, () => {
      const link = node('a');
      link.href = shown.url;
      link.download = `${name}.${shown.format}`;
      link.click();
    }]] : []),
  ];
  return {image, kind, commands};
}

/* pictureChoiceValue is what a window writes of what it gives a picture
   (pictureChoice): the image, or the set - a set of no variant a picture of
   no image - beside what it writes of its own; nothing given, its own. */
function pictureChoiceValue(choice, own = {}) {
  if (choice.image) return {...own, image: structuredClone(choice.image)};
  if (choice.set) return choice.set.length ? {...own, set: structuredClone(choice.set)} : {...own, image: {}};
  return own;
}

/* pictureMedia is the media type of an image of a format. */
function pictureMedia(format) {
  return format === 'svg' ? 'image/svg+xml' : format === 'jpg' ? 'image/jpeg' : format === 'ico' ? 'image/x-icon' : `image/${format}`;
}

/* chooseMainSectionPicture is «Выбор картинки» of the picture of the main
   section, «Открыть» of its row of the palette of the root (konfiguraciya.md,
   «Досъёмка 30.10.2026»; snapshots dosyomka-2651/07, 08): the picture at the
   left, the commands of the picture in a column at the right (pictureChoice,
   without «Сохранить в файл»), a gap, OK and «Отмена» under them, «Тип
   картинки» under the picture. folder is the root's picture as the server
   read it, given what an earlier OK gave and «Сохранить» has not written
   yet. Answers on OK what the picture is given - {image} or {set}, or given
   when nothing changed - and undefined on «Отмена». */
async function chooseMainSectionPicture(folder, given) {
  const choice = {image: given?.image ?? null, set: given?.set ?? null};
  const content = node('div', undefined, 'picture-choice');
  let view = null;
  const draw = () => {
    view = pictureChoice(choice, folder, 'КартинкаОсновногоРаздела', () => { draw(); content.dispatchEvent(new Event('change', {bubbles: true})); });
    content.replaceChildren(view.image, view.kind);
  };
  draw();
  const commands = view.commands(false).map(([caption], at) => [caption, caption, {act: () => view.commands(false)[at][2](), enabled: () => view.commands(false)[at][1]()}]);
  const answer = await showDialog({title: 'Выбор картинки', content, side: true, className: 'picture-choice-dialog',
    answers: [...commands, null, ['ok', 'OK', {primary: () => true}], [null, 'Отмена']]});
  if (answer !== 'ok') return undefined;
  const value = pictureChoiceValue(choice);
  return value.image || value.set ? value : given;
}

/* pictureFormat is the format of an image a variant or a window draws: of
   its new image, or the extension of its file. */
function pictureFormat(item) {
  return item.image?.format || item.file?.slice(item.file.lastIndexOf('.') + 1).toLowerCase() || '';
}

/* choosePictureFile is «Выбрать из файла» of a picture and of a variant: a
   file of one of formats, read whole - {format, content} - or null. */
async function choosePictureFile(formats) {
  const file = await chooseFileContent(formats.map(format => '.' + format).join(','));
  const format = file?.name.slice(file.name.lastIndexOf('.') + 1).toLowerCase();
  return file && formats.includes(format) ? {format, content: file.content} : null;
}

/* Of variants of one density and more, the prototype takes the one of the
   interface «Любой» that is no template (obshchaya-kartinka.md, «Досъёмка
   28.10.2026»); ML has neither, and takes the one of any direction and any
   browser, then the first. */
function plainVariantFirst(rows) {
  return [...rows].sort((one, other) => (!!(one.direction || one.webBrowser)) - (!!(other.direction || other.webBrowser)));
}

/* screenVariant is the variant of a set a window shows on a screen of the
   density dense: the nearest density, the larger of two as near (on a
   Retina screen the prototype shows xdpi of udpi and xdpi, hdpi of hdpi and
   mdpi). */
function screenVariant(rows, dense) {
  const drawn = plainVariantFirst(rows.filter(row => row.file || row.image));
  return drawn.reduce((best, row) => !best || Math.abs(row.density - dense) < Math.abs(best.density - dense) ||
    Math.abs(row.density - dense) === Math.abs(best.density - dense) && row.density > best.density ? row : best, null);
}

/* setImage is the variant whose image a set turned off leaves (obshchaya-
   kartinka.md, «Досъёмка 28.10.2026», eight trials): the density nearest to
   ldpi on the ladder, the smaller of two as near. ladder is the densities in
   order. */
function setImage(rows, ladder) {
  const base = ladder.indexOf(100), away = row => Math.abs(ladder.indexOf(row.density) - base);
  const drawn = plainVariantFirst(rows.filter(row => row.file || row.image));
  return drawn.reduce((best, row) => !best || away(row) < away(best) || away(row) === away(best) && row.density < best.density ? row : best, null);
}

/* showPictureLibrary is «Библиотека картинок проекта» - the prototype's
   «Библиотека картинок конфигурации» (ML-STUDIO.md, «„Проект“ вместо
   „конфигурации“») - the command «Все картинки» of a picture (docs/
   block-00/obshchaya-kartinka.md, «Команды», «Досъёмка 27.10.2026»;
   snapshot series-2.584/099): a window standing over Studio, not shutting
   it, as the prototype's - the pictures of the project
   in the order of the tree, the first current, the image of the current at
   the right with «Очистить», «Сохранить в файл» and «Закрыть», and
   «Добавить», «Изменить», «Удалить» under the list. «Добавить» makes a
   picture as «Добавить» of the tree and opens its window, «Изменить» and a
   double click open the window of the current; «Очистить» and «Удалить» act
   at once, with no question, as the prototype's; after «Удалить» the row
   before is current. The arrows move as «Переместить вверх / вниз» of the
   tree, which Studio does not have yet: they stand unavailable. actions
   are the tree's: add(), open(path), remove(path) and clear(path), each
   answering whether it was done. A picture of a set of variants shows no
   image and is neither cleared nor saved: its set is an item of its own. */
function showPictureLibrary(actions) {
  document.querySelector('dialog.picture-library')?.remove();
  const library = document.createElement('dialog');
  library.className = 'picture-library';
  let items = [], current = 0;
  const list = node('div', undefined, 'picture-library-list'), shown = node('div', undefined, 'picture-library-image');
  const button = (caption, act) => { const element = node('button', caption); element.type = 'button'; element.addEventListener('click', act); return element; };
  const item = () => items[current];
  const clear = button('Очистить', async () => { if (await actions.clear(item().path)) await load(); });
  const save = button('Сохранить в файл', () => {
    const link = node('a');
    link.href = imageAddress(item());
    link.download = `${item().name}${item().image.slice(item().image.lastIndexOf('.'))}`;
    link.click();
  });
  const up = panelButton('move-up', 'Переместить вверх', false, () => {}), down = panelButton('move-down', 'Переместить вниз', false, () => {});
  const change = button('Изменить', () => item() && actions.open(item().path));
  const remove = button('Удалить', async () => {
    if (!item() || !await actions.remove(item().path)) return;
    current = Math.max(0, current - 1);
    await load();
  });
  const add = button('Добавить', async () => { if (await actions.add()) await load(); });
  const imageAddress = picture => `/api/picture/image?path=${encodeURIComponent(picture.path)}&name=${encodeURIComponent(picture.image)}`;
  function render() {
    list.replaceChildren(...items.map((picture, index) => {
      const row = node('div', undefined, 'picture-library-row' + (index === current ? ' current' : ''));
      row.append(iconElement('picture'), node('span', picture.name));
      row.addEventListener('click', () => { current = index; render(); });
      row.addEventListener('dblclick', () => actions.open(picture.path));
      return row;
    }));
    shown.replaceChildren();
    if (item()?.image) {
      const image = node('img');
      image.src = imageAddress(item());
      image.alt = item().name;
      shown.append(image);
    }
    clear.disabled = save.disabled = !item()?.image;
    change.disabled = remove.disabled = !item();
  }
  async function load() {
    const response = await studioRequest('/api/picture/library', {method: 'GET'});
    items = response.ok ? await response.json() : [];
    current = Math.min(current, Math.max(0, items.length - 1));
    render();
  }
  const arrows = node('div', undefined, 'picture-library-arrows'), right = node('div', undefined, 'picture-library-right');
  arrows.append(up, down);
  right.append(shown, clear, save, button('Закрыть', () => library.close()));
  const bottom = node('div', undefined, 'picture-library-bottom');
  bottom.append(add, change, remove);
  const body = node('div', undefined, 'picture-library-body');
  body.append(list, arrows, right, bottom);
  library.append(node('div', 'Библиотека картинок проекта', 'picture-library-title'), body);
  library.addEventListener('close', () => library.remove());
  document.body.append(library);
  library.show();
  return load();
}

/* showPictureSetEditor is the prototype's editor of a set of variants of a
   picture (docs/block-00/obshchaya-kartinka.md, «Окно», «Досъёмка
   28.10.2026»; snapshots obshchaya-kartinka/03-09, configurator-2026-10-28/
   v1-nabor-variantov): titled with the name of the picture, «Добавить»,
   «Скопировать», «Изменить», «Удалить» over the table «Плотность экрана —
   Направление — Веб-браузер (для веб-клиента) — Картинка», «Режим
   коллекции» under it adding «Ширина ячейки» and «Высота ячейки», OK and
   «Отмена». The prototype's columns «Шаблонная» and «Интерфейс» are not
   here: ML has neither (METADATA-OBJECTS, «Варианты»). «Добавить» adds a
   row of no density and no picture; «…» of a picture opens the window of
   the variant, «×» clears it. On OK a row of no picture is dropped, two rows
   of one density, direction and browser are refused with the prototype's
   words, and the size of a cell is kept only in the mode of a collection,
   which opens marked when a variant has one. rows are the variants
   ({density, direction, webBrowser, glyphWidth, glyphHeight} with the file
   of the folder they draw, file, or a new image, image); choices are the
   server's (PictureEditorSource.VariantChoices); address(row) is the image
   a row draws, formats those an image is taken in. Answers the rows, or
   null on «Отмена». */
async function showPictureSetEditor(name, variants, choices, address, formats) {
  const rows = structuredClone(variants);
  let current = rows.length ? 0 : -1;
  let collection = rows.some(row => row.glyphWidth || row.glyphHeight);
  const label = (list, value) => list.find(([key]) => key === value)?.[1] ?? '';
  const densities = choices.densities.map(choice => [choice.density, choice.caption]);
  const content = node('div', undefined, 'picture-set');
  const table = node('table', undefined, 'picture-set-table');
  const pick = async row => {
    const chosen = await showPictureVariantWindow(label(densities, row.density), row, address, formats);
    if (chosen) { delete row.file; delete row.image; Object.assign(row, chosen); }
    draw();
  };
  const bar = windowPanel([
    panelButton('add', 'Добавить', true, () => { rows.splice(current + 1, 0, {density: 0}); current += 1; draw(); }),
    panelButton('clone', 'Скопировать', false, () => { rows.splice(current + 1, 0, structuredClone(rows[current])); current += 1; draw(); }),
    panelButton('change', 'Изменить', false, () => table.querySelector('tbody tr.selected select')?.focus()),
    panelButton('delete', 'Удалить', false, () => { rows.splice(current, 1); current = Math.min(current, rows.length - 1); draw(); }),
  ]);
  const mode = checkboxField('Режим коллекции', collection, value => { collection = value; draw(); });
  function size(row, key) {
    return wholeNumberInput(row[key] || 0, value => { if (value) row[key] = value; else delete row[key]; }, {min: 0, max: 2147483647});
  }
  function draw() {
    const head = node('tr');
    head.append(...['Плотность экрана', 'Направление', 'Веб-браузер (для веб-клиента)', 'Картинка',
      ...collection ? ['Ширина ячейки', 'Высота ячейки'] : []].map(caption => node('th', caption)));
    const body = node('tbody');
    for (const row of rows) {
      const tr = node('tr'), cells = [
        selectInput([['0', ''], ...densities.map(([value, text]) => [String(value), text])], String(row.density || 0),
          value => { row.density = Number(value); }, 'Плотность экрана'),
        selectInput(choices.directions, row.direction || '', value => { if (value) row.direction = value; else delete row.direction; }, 'Направление'),
        selectInput(choices.browsers, row.webBrowser || '', value => { if (value) row.webBrowser = value; else delete row.webBrowser; }, 'Веб-браузер'),
      ];
      const shown = node('input');
      shown.readOnly = true;
      shown.value = row.file || row.image ? `Картинка: ${pictureFormat(row)}` : '';
      cells.push(fieldWithButtons(shown, [fieldChoiceButton('Выбрать картинку', () => pick(row)),
        clearButton('Очистить картинку', () => { delete row.file; delete row.image; draw(); })]));
      if (collection) cells.push(size(row, 'glyphWidth'), size(row, 'glyphHeight'));
      tr.append(...cells.map(cell => { const td = node('td'); td.append(cell); return td; }));
      tr.addEventListener('click', () => { current = rows.indexOf(row); paint(); });
      tr.addEventListener('focusin', () => { current = rows.indexOf(row); paint(); });
      body.append(tr);
    }
    const thead = node('thead');
    thead.append(head);
    table.replaceChildren(thead, body);
    paint();
  }
  function paint() {
    const buttons = bar.querySelectorAll('button');
    for (const button of [...buttons].slice(1)) button.disabled = current < 0;
    [...table.querySelectorAll('tbody tr')].forEach((tr, at) => tr.classList.toggle('selected', at === current));
  }
  draw();
  content.append(bar, table, mode);
  const kept = () => rows.filter(row => row.file || row.image);
  // The prototype names the variants that coincide by the captions of
  // their key; «Любое» it writes as an empty string, which ML does not copy.
  const check = key => {
    if (key !== 'ok') return '';
    if (kept().some(row => !row.density)) return 'Не задана плотность экрана варианта картинки.';
    const seen = new Set();
    for (const row of kept()) {
      const key = [row.density, row.direction || '', row.webBrowser || ''].join('\n');
      if (seen.has(key)) return `Картинка содержит не уникальные варианты "${label(densities, row.density)}", ` +
        `"${label(choices.directions, row.direction || '')}", "${label(choices.browsers, row.webBrowser || '')}".`;
      seen.add(key);
    }
    return '';
  };
  const answer = await showDialog({title: name, content, className: 'picture-set-dialog', check, answers: [['ok', 'OK'], [null, 'Отмена']]});
  if (answer !== 'ok') return null;
  return kept().map(row => {
    if (!collection) { delete row.glyphWidth; delete row.glyphHeight; }
    return row;
  });
}

/* showPictureVariantWindow is the prototype's window of one variant of a
   set, «…» of its picture (obshchaya-kartinka.md, «Досъёмка 28.10.2026»;
   snapshots v1-nabor-variantov/022, 024): titled with the caption of its
   density, the image - a crossed square when there is none - with its size
   under it, «Выбрать из файла», «Очистить», «Сохранить в файл», OK and
   «Отмена» in a column at the right. «Редактировать» is not here, as in the
   window of a picture: Studio has no editor of pictures (the owner,
   15.10.2026). Answers {file} or {image} of the variant, {} for none, or
   null on «Отмена». */
async function showPictureVariantWindow(title, row, address, formats) {
  let chosen = row.image ? {image: row.image} : row.file ? {file: row.file} : {};
  const content = node('div', undefined, 'picture-variant');
  const area = node('div', undefined, 'picture-variant-image'), size = node('div', undefined, 'picture-variant-size');
  const buttons = node('div', undefined, 'picture-window-buttons');
  const button = (caption, act) => { const element = node('button', caption); element.type = 'button'; element.addEventListener('click', act); buttons.append(element); return element; };
  button('Выбрать из файла', async () => {
    const file = await choosePictureFile(formats);
    if (file) { chosen = {image: file}; draw(); }
  });
  button('Очистить', () => { chosen = {}; draw(); });
  const save = button('Сохранить в файл', () => {
    const link = node('a');
    link.href = address(chosen);
    link.download = `${title}.${pictureFormat(chosen)}`;
    link.click();
  });
  function draw() {
    const url = chosen.file || chosen.image ? address(chosen) : '';
    save.disabled = !url;
    area.classList.toggle('empty', !url);
    size.textContent = '';
    if (!url) { area.replaceChildren(); return; }
    const drawn = node('img');
    drawn.addEventListener('load', () => { size.textContent = `${drawn.naturalWidth} x ${drawn.naturalHeight}`; });
    drawn.src = url;
    area.replaceChildren(drawn);
  }
  draw();
  const shown = node('div', undefined, 'picture-variant-shown');
  shown.append(area, size);
  content.append(shown, buttons);
  const answer = await showDialog({title, content, className: 'picture-variant-dialog', side: true, answers: [['ok', 'OK'], [null, 'Отмена']]});
  return answer === 'ok' ? chosen : null;
}
