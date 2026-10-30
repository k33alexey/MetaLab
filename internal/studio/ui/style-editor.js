'use strict';

/* The window of a style, as the prototype's (docs/block-00/stil.md, «Окно»;
   snapshots stil/05, 06, configurator-2026-10-27/series-2.584/004-014): no
   pages - at the left the name, the synonym, the comment, the subsystems
   (windowHeadFields) and «Предварительный просмотр» drawn by the style; at
   the right the style items the window lists (source.items: the platform's
   five, then the project's in the order of the tree), each with its value
   in this style and «…». The palette beside it opens the same values in a
   window of their own with OK and «Отмена» («Стиль: Открыть»). «…» of a
   value opens «Выбор цвета», «Выбор шрифта» or «Выбор рамки». around holds
   the palette and its title. */
function createStyleEditor(host, onChange, around = {}) {
  let source = null, changed = () => {};
  function draw(body, api) {
    source = api.source();
    changed = api.changed;
    const preview = node('fieldset', undefined, 'studio-frame style-window-preview');
    preview.append(node('legend', 'Предварительный просмотр'), ...stylePreview(source));
    const left = node('div', undefined, 'style-window-left');
    left.append(...windowHeadFields(source, source.style, api.typed, api.cleanups(), api.subsystems()), preview);
    body.append(left, styleValueRows(source, 'style-window-items', item => chooseValue(source.style, item, api.changed)));
  }
  // «…» of a value: the window choosing a value of the item's kind, its
  // answer the value this style gives the item. A colour or a font taken
  // from the item itself, unchanged, is what the window shows of an item
  // the style gives nothing - «стиль: <элемент>» - and the style is left
  // giving it nothing; such an item opens on its own row (snapshot
  // series-2.584/024).
  async function chooseValue(style, item, done) {
    const given = styleSetting({...source, style}, item);
    const itself = {source: 'style', from: item.item ? {item: item.item} : {standard: stylePlatformItem(source, item.standard)?.name || item.standard}};
    const give = (kind, chosen) => styleGive(style, item, JSON.stringify(chosen) === JSON.stringify(itself) ? null : {[kind]: chosen});
    if (item.type === 'border') {
      const chosen = await chooseBorder(given?.border, styleBorders(source), source.borderLines);
      if (!chosen) return;
      styleGive(style, item, {border: chosen});
    } else if (item.type === 'color') {
      const chosen = await chooseColor(styleColorValue(source, given?.color) || itself, styleColors({...source, style}), styleWebColors(source));
      if (!chosen) return;
      give('color', chosen);
    } else {
      const chosen = await chooseFont(styleFontValue(source, given?.font) || itself, styleFonts(source), source.systemFonts, await systemFaces());
      if (!chosen) return;
      give('font', chosen);
    }
    done();
  }
  // «Стиль: Открыть»: the values in a window of their own, chosen on a
  // copy - OK gives them to the style, «Отмена» leaves it as it was.
  async function openValues() {
    const copy = structuredClone(source.style), content = node('div');
    const fill = () => content.replaceChildren(styleValueRows({...source, style: copy}, 'style-values-dialog', item => chooseValue(copy, item, fill)));
    fill();
    const answer = await showDialog({title: `${source.kindTitle} ${source.style.name}`, content, answers: [['ok', 'OK'], [null, 'Отмена']], className: 'style-values-window'});
    if (answer !== 'ok' || JSON.stringify(copy.items) === JSON.stringify(source.style.items)) return;
    if (copy.items) source.style.items = copy.items;
    changed();
  }
  return objectWindowEditor(host, onChange, around, {target: opened => opened.style, own: api => ({className: 'style-window',
    draw: body => draw(body, api), context: () => ({windows: ['items'], openWindow: openValues}), value: object => object})});
}

/* styleValueRows are the rows of the values of a style (source, as
   createStyleEditor reads it): the caption of each item listed and a field
   of its value - a sample of a colour and the text of styleValueText - with
   «…». */
function styleValueRows(source, className, choose) {
  const list = node('div', undefined, 'style-values ' + className);
  for (const item of source.items) {
    const caption = node('span', undefined, 'style-value-caption');
    caption.append(iconElement('style-item-' + item.type), node('span', styleItemTitle(source, item)));
    const shown = styleValueText(source, item), field = node('div', undefined, 'style-value-field');
    field.title = shown.text;
    if (item.type === 'color') {
      const sample = node('span', undefined, 'style-value-sample');
      if (shown.color) sample.style.background = shown.color;
      field.append(sample);
    }
    field.append(node('span', shown.text));
    const pick = fieldChoiceButton(styleItemTitle(source, item), () => choose(item));
    list.append(caption, fieldWithButtons(field, [pick]));
  }
  return list;
}

/* styleItemTitle is how the windows of a style name a style item: one of
   the platform's by its caption, one of the project's by its synonym in
   the language of Studio, its name where there is none. */
function styleItemTitle(source, item) {
  if (item.standard) return stylePlatformItem(source, item.standard)?.title || item.standard;
  return textPresentation(item.title, StudioLanguage.get(), item.name);
}

/* stylePlatformItem is the platform's own style item named in either
   language (source.platform), or undefined. */
function stylePlatformItem(source, name) {
  const folded = name.toLowerCase();
  return source.platform.find(item => item.name.toLowerCase() === folded || item.russian.toLowerCase() === folded);
}

/* styleSetting is the value this style gives an item - by the identifier
   of one of the project's, by the name in either language of one of the
   platform's - or undefined where it gives none. */
function styleSetting(source, item) {
  const platform = item.standard && stylePlatformItem(source, item.standard);
  return (source.style.items || []).find(setting => item.item ? setting.item === item.item
    : !!setting.standard && stylePlatformItem(source, setting.standard) === platform)?.value;
}

/* styleReferenced is the item a reference to a style item names (metadata.
   StyleItemReference): one of the platform's by name in either language,
   one of the project's by identifier - as source.items holds them, the
   platform's not listed there as {standard, type} - or null. */
function styleReferenced(source, reference) {
  if (reference?.standard) {
    const own = stylePlatformItem(source, reference.standard);
    return own ? (source.items.find(item => item.standard === own.name) || {standard: own.name, type: own.type}) : null;
  }
  return reference?.item ? source.items.find(item => item.item === reference.item) || null : null;
}

/* styleValueText is what the field of an item's value shows in a style
   (stil.md, «Окно», «Досъёмка 27.10.2026»; snapshots series-2.584/004,
   008, 014, 049, 055): {text, color} - the text and the colour of the
   sample of a colour. A value the style does not give shows «стиль:
   <элемент>» with the sample of the item's own value; one of a border,
   nothing. A colour of its own shows its numbers «250, 195, 113»; a colour
   of the web palette its name «Мятный крем (MintCream)»; a colour or a
   border taken from a style item «стиль: <элемент>»; a font as the
   platform writes one (fontPresentation): «стиль: Шрифт текста,
   зачеркнутый», «Шрифт диалогов и меню, 10, полужирный», «Arial, 12,
   наклонный». What the inventory does not show - a colour of the system,
   an automatic one - shows nothing; a border of its own too, until its
   own item. */
function styleValueText(source, item) {
  const given = styleSetting(source, item);
  if (!given) {
    if (item.type === 'border') return {text: ''};
    return {text: `стиль: ${styleItemTitle(source, item)}`, color: styleColorOf(source, item, 0)};
  }
  const value = given[item.type];
  const from = value?.source === 'style' && styleReferenced(source, value.from);
  if (item.type === 'color') {
    if (value.source === 'absolute') return {text: colorNumbers(value.rgb).join(', '), color: value.rgb};
    if (value.source === 'web') {
      const web = styleWebColor(source, value.name);
      return {text: web ? styleWebTitle(web) : value.name, color: web?.rgb};
    }
    if (from) return {text: `стиль: ${styleItemTitle(source, from)}`, color: styleColorOf(source, from, 0)};
    return {text: ''};
  }
  if (item.type === 'border') return {text: from ? `стиль: ${styleItemTitle(source, from)}` : ''};
  return {text: fontPresentation(value, font => styleFontBase(source, font))};
}

/* styleFontBase is the caption of what a font is based on (metadata.
   FontValue): a style item by styleItemTitle, a font of the system by its
   caption in «Выбор шрифта» - or '' where it names nothing known. */
function styleFontBase(source, font) {
  if (font.source === 'system') {
    const folded = font.system.toLowerCase();
    return source.systemFonts.find(system => system.name.toLowerCase() === folded || system.russian.toLowerCase() === folded)?.title || '';
  }
  const from = styleReferenced(source, font.from);
  return from ? styleItemTitle(source, from) : '';
}

/* styleFonts are the rows of «Шрифт стиля» of «Выбор шрифта»
   (chooseFont): the platform's five fonts, then the project's in the order
   of the tree, each with what it is known to be drawn by - a font of the
   project its face, size and switches followed through the items it is
   taken from; the platform's are not known (Studio does not draw them).
   styleFontValue is a font as those rows name it - a platform's font by
   its English name, a font of the system by its English name. */
function styleFonts(source) {
  const drawn = (item, depth) => {
    const own = item?.value?.font;
    if (!own || depth > 40) return {};
    const base = own.source === 'style' ? drawn(styleReferenced(source, own.from), depth + 1) : {};
    const known = {...base};
    for (const field of ['face', 'size', ...fontFlags.map(([flag]) => flag)]) if (own[field] !== undefined && own[field] !== '') known[field] = own[field];
    return known;
  };
  return [...source.platform.filter(item => item.type === 'font').map(item => ({caption: item.title, value: {source: 'style', from: {standard: item.name}}, drawn: {}})),
    ...source.items.filter(item => item.item && item.type === 'font').map(item => ({caption: styleItemTitle(source, item), value: {source: 'style', from: {item: item.item}}, drawn: drawn(item, 0)}))];
}

function styleFontValue(source, value) {
  if (value?.source === 'style' && value.from?.standard) return {...value, from: {standard: stylePlatformItem(source, value.from.standard)?.name || value.from.standard}};
  if (value?.source === 'system') {
    const folded = value.system.toLowerCase();
    return {...value, system: source.systemFonts.find(system => system.name.toLowerCase() === folded || system.russian.toLowerCase() === folded)?.name || value.system};
  }
  return value;
}

/* styleColorOf is the colour an item of a colour has in a style, for its
   sample - what the style gives it, else its own value, followed through
   the items it is taken from; one of the platform's the style gives
   nothing, the platform's own (metadata.PlatformStyleItem.Color) - or
   undefined where it is not known: a colour of the system, an automatic
   one. */
function styleColorOf(source, item, depth) {
  if (!item || item.type !== 'color' || depth > 40) return undefined;
  const value = styleSetting(source, item)?.color || item.value?.color;
  if (!value) return item.standard ? stylePlatformItem(source, item.standard)?.color : undefined;
  if (value.source === 'absolute') return value.rgb;
  if (value.source === 'web') return styleWebColor(source, value.name)?.rgb;
  if (value.source === 'style') return styleColorOf(source, styleReferenced(source, value.from), depth + 1);
  return undefined;
}

/* styleWebColor is a colour of the web palette by its English name
   (metadata.WebColorName: its caption and the numbers the prototype draws
   it by), or undefined; styleWebTitle is how the windows name it,
   «Мятный крем (MintCream)». */
function styleWebColor(source, name) {
  const folded = name.toLowerCase();
  return source.webColors.find(color => color.name.toLowerCase() === folded);
}

function styleWebTitle(web) {
  return `${web.title} (${web.name})`;
}

/* styleColors are the rows of «Из стиля» of «Выбор цвета» (chooseColor):
   the platform's 28 colours, then the project's in the order of the tree,
   each drawn by the colour it has in this style; styleWebColors the rows
   of «Web». styleColorValue is a colour as those rows hold it - a
   platform's colour by its English name, a web colour by the palette's -
   so that the window finds the row of the value it opens on. */
function styleColors(source) {
  return [...source.platform.filter(item => item.type === 'color').map(item => ({standard: item.name, type: 'color'})),
    ...source.items.filter(item => item.item && item.type === 'color')].map(item => {
    const reference = item.item ? {item: item.item} : {standard: item.standard};
    return {caption: styleItemTitle(source, item), rgb: styleColorOf(source, styleReferenced(source, reference), 0), value: {source: 'style', from: reference}};
  });
}

function styleWebColors(source) {
  return source.webColors.map(web => ({caption: styleWebTitle(web), rgb: web.rgb, value: {source: 'web', name: web.name}}));
}

function styleColorValue(source, value) {
  if (value?.source === 'web') return {source: 'web', name: styleWebColor(source, value.name)?.name || value.name};
  if (value?.source === 'style' && value.from?.standard) return {source: 'style', from: {standard: stylePlatformItem(source, value.from.standard)?.name || value.from.standard}};
  return value;
}

/* styleGive makes the value a style gives an item value (metadata.
   StyleItemValue): the setting of the item is changed where it stands, or
   added last; value null takes it away. */
function styleGive(style, item, value) {
  const items = style.items || (style.items = []);
  const at = items.findIndex(setting => item.item ? setting.item === item.item : !!setting.standard && setting.standard.toLowerCase() === item.standard.toLowerCase());
  const setting = item.item ? {item: item.item, value} : {standard: item.standard, value};
  if (!value) { if (at >= 0) items.splice(at, 1); return; }
  if (at < 0) items.push(setting); else items[at] = setting;
}

/* styleBorders are the borders a style may take a border from, as
   «Выбор рамки» lists them: the platform's, then the project's in the
   order of the tree, each with the type and thickness it draws - the
   platform's own «Нет рамки», 1 (snapshot series-2.584/048), one of the
   project's its own value followed through the items it is taken from. */
function styleBorders(source) {
  const drawn = (item, depth) => {
    const own = item?.value?.border;
    if (!own || depth > 40) return {line: 'none', width: 1};
    if (own.source === 'style') return drawn(styleReferenced(source, own.from), depth + 1);
    return {line: own.line, width: own.line === 'none' ? 1 : own.width || 1};
  };
  return [...source.platform.filter(item => item.type === 'border').map(item => ({reference: {standard: item.name}, caption: item.title, drawn: {line: 'none', width: 1}})),
    ...source.items.filter(item => item.item && item.type === 'border').map(item => ({reference: {item: item.item}, caption: styleItemTitle(source, item), drawn: drawn(item, 0)}))];
}

/* stylePreview is «Предварительный просмотр» of the window of a style
   (snapshots stil/05, series-2.584/008): the panel of sections in the
   navigation colour, the panel of functions in the auxiliary navigation
   colour, the heading of a group and the mark of the flag in the accent
   colour, the default button in the colour of the important, the field in
   the colour of activity - as the two styles shot draw them - with a
   table of two columns between. A colour not known (styleColorOf) leaves
   the element as Studio draws it. */
function stylePreview(source) {
  const color = name => styleColorOf(source, styleReferenced(source, {standard: name}), 0);
  const paint = (element, property, name) => { const value = color(name); if (value) element.style[property] = value; return element; };
  const sections = paint(node('div', 'Панель разделов', 'style-preview-sections'), 'background', 'NavigationColor');
  const functions = paint(node('div', 'Панель функций текущего раздела', 'style-preview-functions'), 'background', 'AuxiliaryNavigationColor');
  const heading = paint(node('div', 'Заголовок группы', 'style-preview-heading'), 'color', 'AccentColor');
  const button = paint(node('button', 'Кнопка по умолчанию', 'style-preview-button'), 'background', 'ImportantColor');
  button.type = 'button';
  button.tabIndex = -1;
  const flag = node('label', undefined, 'catalog-check style-preview-flag'), box = node('input');
  box.type = 'checkbox';
  box.checked = true;
  box.tabIndex = -1;
  flag.append(paint(box, 'accentColor', 'AccentColor'), node('span', 'Флажок'));
  const table = node('table', undefined, 'style-preview-table'), head = node('tr');
  head.append(node('th', 'Колонка1'), node('th', 'Колонка2'));
  table.append(head);
  for (let row = 0; row < 4; row++) {
    const line = node('tr');
    line.append(node('td'), node('td'));
    table.append(line);
  }
  const input = node('input');
  input.readOnly = true;
  input.tabIndex = -1;
  const field = paint(node('div', undefined, 'style-preview-field'), 'borderColor', 'ActivityColor');
  field.append(input, node('span', '…'), node('span', '×'));
  return [sections, functions, heading, button, flag, table, field];
}
