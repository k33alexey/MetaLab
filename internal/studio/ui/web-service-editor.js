'use strict';

/* The window of a web service, as the prototype's (docs/block-00/
   web-servis.md, «Окно»; snapshots web-servis/01-04): the pages of the kind
   (source.pages) - «Основные», «Подсистемы», «Операции», «Управление
   сеансами», «Прочее» - built of the palette's rows (source.windowRows),
   «Подсистемы» the tree of the subsystems with boxes; the palette of the
   service beside it, each showing what the other typed. «Операции» stands
   empty until its item; «…» of «Пакеты XDTO» is unavailable until the window
   choosing packages. around holds the palette, its title, the opening of a
   module, the actions and the closing of the window. */
function createWebServiceEditor(host, onChange, around = {}) {
  let source = null, frame = null, marks = new Set(), cleanups = [];
  const paletteCleanups = [];
  const service = () => source.service;
  const title = () => `${source.kindTitle} ${service().name}`;
  function subject() { return ownSubject(source, service()); }
  function context(collect) {
    return {project: source, typed, changed, packages: source.packages, windows: [], cleanups: collect,
      openModule: role => around.openModule?.(role)};
  }
  // Typing renames the window and the palette and leaves the field typed
  // into as it is; anything else draws both anew.
  function typed() {
    onChange();
    frame.setTitle(title());
    if (!host.contains(document.activeElement)) frame.refresh();
    if (!around.palette?.contains(document.activeElement)) renderPalette();
  }
  function changed() { onChange(); frame.refresh(); renderPalette(); }
  function renderPage(key) {
    cleanups.forEach(destroy => destroy());
    cleanups = [];
    if (key === 'subsystems') {
      // The caption above the tree is not shot for any kind but a catalog
      // (series 2.584, В2): the tree stands alone.
      const page = node('div', undefined, 'object-window-fields');
      page.append(subsystemsTree(source.subsystems, marks, onChange));
      return page;
    }
    if (!source.windowRows?.[key]) return null;
    return rowsPage(source.windowRows[key], source.palette, paletteFields(context(() => cleanups)), subject(), role => around.openModule?.(role));
  }
  function renderPalette() {
    if (!source) return;
    showObjectPalette(around, paletteCleanups, service().name, source.palette, subject(), context());
  }
  StudioLanguage.onChange(() => { if (source) { frame.refresh(); renderPalette(); } });
  return hostedEditor(host, () => { cleanups.forEach(destroy => destroy()); cleanups = []; if (source) clearObjectPalette(around, paletteCleanups); source = null; }, {
    open(value) {
      if (source) clearObjectPalette(around, paletteCleanups);
      source = structuredClone(value);
      marks = new Set((source.subsystems || []).filter(place => place.member).map(place => place.id));
      frame = showObjectWindow(host, around, {title: title(), pages: source.pages, chosen: 'main', page: renderPage});
      renderPalette();
    },
    // The service and the subsystems marked, in one order so a mark taken
    // back leaves the window unchanged.
    value() { return source && {...structuredClone(service()), subsystems: [...marks].sort()}; },
    showPalette() {
      if (!source || !around.palette) return false;
      renderPalette();
      return true;
    },
  });
}
