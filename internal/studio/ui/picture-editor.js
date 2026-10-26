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
  let source = null, image = null, cleanups = [], frame = null;
  const paletteCleanups = [];
  const picture = () => source.picture;
  // The formats an image of a picture is kept in (metadata.PictureFormats).
  const formats = ['png', 'svg', 'gif', 'ico', 'jpg', 'bmp'];
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
  function title() { return `${source.kindTitle} ${picture().name}`; }
  function changed() { onChange(); render(); }
  // Typing renames the window and the palette, and leaves the field typed
  // into as it is.
  function typed() {
    onChange();
    frame.heading.textContent = title();
    if (!host.contains(document.activeElement)) renderWindow();
    if (!around.palette?.contains(document.activeElement)) renderPalette();
  }
  function setText(key, value) {
    if (typeof value === 'string' ? value : Object.keys(value || {}).length) picture()[key] = value; else delete picture()[key];
  }
  function setFlag(key, value) {
    if (value) picture()[key] = true; else delete picture()[key];
    // Unavailable for appearance is unavailable for choice too: the help
    // makes the one editable only while the other is on.
    if (key === 'availableForChoice' && !value) delete picture().availableForAppearance;
  }
  function renderWindow() {
    cleanups.forEach(destroy => destroy());
    cleanups = [];
    const body = frame.body;
    body.replaceChildren();
    const synonym = createLocalizedField(source, code => picture().title?.[code], (language, value) => setText('title', setTranslation(picture().title, language, value)), typed, titleTextOptions);
    cleanups.push(synonym.destroy);
    const name = nameInput(picture().name, value => { const before = picture().name; picture().name = value; synonym.follow(before, value); typed(); },
      () => ({path: source.path, name: picture().name}));
    const members = (source.subsystems || []).filter(place => place.member).map(place => place.name).join(',');
    const subsystems = node('input');
    subsystems.readOnly = true;
    subsystems.value = members;
    // The window choosing the subsystems is not described yet.
    const choose = fieldChoiceButton('Подсистемы', () => {});
    choose.disabled = true;
    body.append(labeledField('Имя:', name), labeledField('Синоним:', synonym),
      labeledField('Комментарий:', textInput(picture().comment, value => { setText('comment', value); typed(); })),
      labeledField('Подсистемы:', fieldWithButtons(subsystems, [choose])),
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
    button('Выбрать из файла', single, chooseFile);
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
  async function chooseFile() {
    const file = await chooseFileContent(formats.map(format => '.' + format).join(','));
    const format = file?.name.slice(file.name.lastIndexOf('.') + 1).toLowerCase();
    if (!file || !formats.includes(format)) return;
    image = {format, content: file.content};
    changed();
  }
  // «Сохранить в файл»: the image as the browser saves a download, named
  // after the picture.
  function saveToFile(shown) {
    const link = node('a');
    link.href = shown.url;
    link.download = `${picture().name}.${shown.format}`;
    link.click();
  }
  function renderPalette() {
    if (!source) return;
    const subject = {target: picture(), get: key => picture()[key], set: (key, value) => key === 'comment' ? setText(key, value) : setFlag(key, value),
      setName: value => { picture().name = value; }, check: () => ({path: source.path, name: picture().name})};
    showObjectPalette(around, paletteCleanups, picture().name, source.palette, subject,
      {project: source, typed, changed: () => changed(), windows: ['picture'], openWindow: () => host.querySelector('input')?.focus()});
  }
  function render() { renderWindow(); renderPalette(); }
  StudioLanguage.onChange(() => { if (source) render(); });
  return hostedEditor(host, () => { cleanups.forEach(destroy => destroy()); cleanups = []; if (source) clearObjectPalette(around, paletteCleanups); source = null; image = null; }, {
    open(value) {
      if (source) clearObjectPalette(around, paletteCleanups);
      source = structuredClone(value);
      image = null;
      host.hidden = false;
      const element = node('div', undefined, 'object-window picture-window'), heading = node('div', title(), 'object-window-title');
      const body = node('div', undefined, 'picture-window-body');
      element.append(heading, body);
      frame = {heading, body};
      host.replaceChildren(element);
      render();
    },
    value() { return source && {...structuredClone(picture()), ...(image ? {image: structuredClone(image)} : {})}; },
    showPalette() {
      if (!source || !around.palette) return false;
      renderPalette();
      return true;
    },
  });
}
