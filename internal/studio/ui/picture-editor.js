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
