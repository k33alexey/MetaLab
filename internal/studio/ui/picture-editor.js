'use strict';

/* The window of a common picture, as the prototype's (docs/block-00/
   obshchaya-kartinka.md, «Окно»; snapshot obshchaya-kartinka/02-okno): no
   pages - the name, the synonym, the comment, the subsystems and the two
   flags of availability at the top; below, the picture at the left and its
   buttons at the right; «Тип картинки» under it. The palette beside it shows
   the same name, synonym, comment and flags (source.palette). An image
   chosen or cleared is the window's until «Сохранить», which writes it with
   the description. The buttons of the set of variants and of the
   transparency stand unavailable: they are items of their own. The
   prototype's «Редактировать» is not here: Studio has no editor of
   pictures, a picture is loaded from a file and saved to one (the owner's
   decision of 15.10.2026, ML-STUDIO.md, «Окно общей картинки»).
   around holds the palette and its title. */
function createPictureEditor(host, onChange, around = {}) {
  let image = null, source = null;
  // The formats an image of a picture is kept in (metadata.PictureFormats).
  const formats = ['png', 'svg', 'gif', 'ico', 'jpg', 'bmp'];
  const picture = () => source.picture;
  // A picture of more than one image, or of variants, keeps a set.
  const set = () => (picture().variants || []).length > 0 || source.images.length > 1;
  // The image the window shows: chosen, cleared, or the one in the folder.
  function shownImage() {
    if (image) return image.content ? {url: `data:${mediaOf(image.format)};base64,${image.content}`, format: image.format} : null;
    const name = source.images[0];
    return name ? {url: `/api/picture/image?path=${encodeURIComponent(source.path)}&name=${encodeURIComponent(name)}`,
      format: name.slice(name.lastIndexOf('.') + 1).toLowerCase(), name} : null;
  }
  function mediaOf(format) { return format === 'svg' ? 'image/svg+xml' : format === 'jpg' ? 'image/jpeg' : format === 'ico' ? 'image/x-icon' : `image/${format}`; }
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
    body.append(...windowHeadFields(source, picture(), typed, api.cleanups()),
      checkboxField('Доступность для выбора', !!picture().availableForChoice, value => { setFlag('availableForChoice', value); changed(); }),
      checkboxField('Доступность для оформления', !!picture().availableForAppearance, value => { setFlag('availableForAppearance', value); changed(); },
        'catalog-check', !picture().availableForChoice));
    const shown = shownImage(), area = node('div', undefined, 'picture-window-image');
    if (shown) {
      const drawn = node('img');
      drawn.src = shown.url;
      drawn.alt = picture().name;
      area.append(drawn);
    }
    const buttons = node('div', undefined, 'picture-window-buttons');
    const button = (caption, enabled, act) => {
      const element = node('button', caption);
      element.type = 'button';
      element.disabled = !enabled;
      element.addEventListener('click', act);
      buttons.append(element);
      return element;
    };
    const single = !set();
    button('Выбрать из файла', single, () => chooseFile(api));
    button('Очистить', single && !!shown, () => { image = {}; changed(); });
    button('Установить прозрачный фон', false, () => {});
    button('Убрать прозрачность', false, () => {});
    button('Включить набор вариантов', false, () => {});
    button('Выключить набор вариантов', false, () => {});
    button('Сохранить в файл', !!shown && single, () => saveToFile(shown));
    const middle = node('div', undefined, 'picture-window-middle');
    middle.append(area, buttons);
    const kind = node('div', undefined, 'picture-window-type');
    kind.append(node('span', 'Тип картинки:'), node('span', set() ? 'zip' : shown?.format || ''));
    body.append(middle, kind);
  }
  // «Выбрать из файла»: a file of one of the formats, read whole; it becomes
  // the picture when the window is saved.
  async function chooseFile(api) {
    const file = await chooseFileContent(formats.map(format => '.' + format).join(','));
    const format = file?.name.slice(file.name.lastIndexOf('.') + 1).toLowerCase();
    if (!file || !formats.includes(format)) return;
    image = {format, content: file.content};
    api.changed();
  }
  // «Сохранить в файл»: the image as the browser saves a download, named
  // after the picture.
  function saveToFile(shown) {
    const link = node('a');
    link.href = shown.url;
    link.download = `${picture().name}.${shown.format}`;
    link.click();
  }
  return objectWindowEditor(host, onChange, around, {target: opened => opened.picture, own: api => ({className: 'picture-window',
    draw: body => { source = api.source(); draw(body, api); },
    subject: opened => ({target: opened.picture, get: key => opened.picture[key], set: (key, value) => key === 'comment' ? setText(key, value) : setFlag(key, value),
      setName: value => { opened.picture.name = value; }, check: () => ({path: opened.path, name: opened.picture.name})}),
    context: () => ({windows: ['picture'], openWindow: () => host.querySelector('input')?.focus()}),
    reset: () => { image = null; },
    value: object => image ? {...object, image: structuredClone(image)} : object})});
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
