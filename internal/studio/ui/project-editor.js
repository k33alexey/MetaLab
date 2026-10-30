'use strict';

/* The root of the project (configuration.yaml). The root has no window: its
   double click and «Свойства» open its palette, the five tabs of the
   prototype's (docs/block-00/konfiguraciya.md, «Палитра»; kinds.RootPalette),
   built by the palette of every object (paletteEditor). Managing the list of
   languages is a separate editor (languages-editor.js) reached through the
   branch «Языки» of the tree; it shares the model below. */
function createProjectModel(source) {
  const configuration = source.configuration;
  configuration.languages ||= [];
  return {
    value() { return structuredClone(configuration); },
    setDefaultLanguage(code) { configuration.defaultLanguage = code; },
    /* The root's half of the command interface - the order of the sections
       and their visibility - as its window «Командный интерфейс» writes it
       (showProjectCommandInterface): nothing said, nothing kept. */
    setSections(order, visibility) {
      if (order.length) configuration.subsystemsOrder = order; else delete configuration.subsystemsOrder;
      if (visibility.length) configuration.subsystemsVisibility = visibility; else delete configuration.subsystemsVisibility;
    },
    removeLanguage(code) {
      if (code === 'en') return;
      configuration.languages = configuration.languages.filter(item => item.code !== code);
      if (configuration.defaultLanguage === code) configuration.defaultLanguage = configuration.languages[0]?.code || 'en';
    },
    setLanguageField(code, field, value) {
      if (code === 'en') return;
      const language = configuration.languages.find(item => item.code === code);
      if (!language) return;
      if (value === '' && field === 'comment') delete language[field]; else language[field] = value;
    },
    /* Синоним языка локализован, как синоним всякого объекта: язык, названный
       на двух языках, иначе терял бы одно из названий — а список языков ровно
       то место, где два языка читатель видит разом. */
    setLanguageTitle(code, translation, value) {
      if (code === 'en') return;
      const language = configuration.languages.find(item => item.code === code);
      if (!language) return;
      const text = language.title && typeof language.title === 'object' ? language.title : {};
      if (value === '') delete text[translation]; else text[translation] = value;
      language.title = text;
    },
  };
}

/* rootSubject is what the palette of the root edits: its properties by the
   keys of the rows. An empty value, an unmarked flag, an empty list or
   translation is not written - «не задано» is the platform's own, not a
   reference to nothing. Two rows name an object otherwise than by its
   identifier: the main language by its code, and a dictionary of the search
   by its kind and object - the palette sees their identifiers, the
   description keeps what it keeps; a dictionary chosen anew takes its kind
   from the tree of its window (choices, objectChoiceTree). */
function rootSubject(configuration, choices) {
  const languages = () => configuration.languages || [];
  const dictionaries = () => configuration.additionalFullTextSearchDictionaries || [];
  return {target: configuration,
    setName: value => { configuration.name = value; },
    check: () => ({path: 'configuration.yaml', name: configuration.name}),
    get(key) {
      if (key === 'defaultLanguage') return languages().find(item => item.code === configuration.defaultLanguage)?.id || configuration.defaultLanguage;
      if (key === 'additionalFullTextSearchDictionaries') return dictionaries().map(item => item.object);
      return configuration[key];
    },
    set(key, value) {
      if (key === 'defaultLanguage') value = languages().find(item => item.id === value)?.code || configuration.defaultLanguage;
      if (key === 'additionalFullTextSearchDictionaries') value = (value || []).map(object => dictionaries().find(item => item.object === object)
        || {kind: objectChoiceItem(choices?.[key], object)?.kind, object}).filter(item => item.kind);
      if (value === false || value === null || value === undefined) delete configuration[key];
      else setPathValue(configuration, key, value);
    }};
}

/* createProjectEditor is the palette of the root. around holds the
   palette and its title, and opens what its rows open: a module of the root
   by its file (openModule), the windows of its interface (openWindow:
   commandInterface, homePage, mainSectionCommands, clientInterface); the
   picture of the main section in its own window (chooseMainSectionPicture).
   The help has no window in Studio yet, as the root's menu shows
   «Справочная информация» unavailable. */
function createProjectEditor(host, onChange, around = {}) {
  // What the palette edits, for the window of the command interface; what
  // «Выбор картинки» gave the picture of the main section, written by
  // «Сохранить» with the description, as the prototype keeps a change of
  // the configuration until it is saved.
  let opened = null, picture = null, mainSection = null;
  // «Командный интерфейс основного раздела»: what its OK gives is the
  // root's until «Сохранить», as the picture's.
  async function chooseMainSection() {
    const chosen = await showMainSectionCommandInterface(mainSection);
    if (chosen instanceof Error) { around.problem?.(chosen); return; }
    if (!chosen) return;
    mainSection = chosen;
    onChange();
  }
  async function choosePicture() {
    const chosen = await chooseMainSectionPicture({...opened.picture, imageAddress: name => `/api/project/picture/image?name=${encodeURIComponent(name)}`}, picture);
    if (chosen === undefined || chosen === picture) return;
    picture = chosen;
    onChange();
  }
  const editor = paletteEditor(host, onChange, around, {
    target: source => (opened = source).configuration,
    subject: source => rootSubject(source.configuration, source.choices),
    // The languages of the texts are the root's own.
    context: source => ({project: source.configuration, choices: source.choices, windows: ['commandInterface', 'homePage', 'mainSectionCommands', 'mainSectionPicture', 'clientInterface'],
      openModule: path => around.openModule?.({path}),
      openWindow: key => key === 'mainSectionPicture' ? choosePicture() : key === 'mainSectionCommands' ? chooseMainSection() : around.openWindow?.(key),
      openFile: path => around.openFile?.(path),
      // A main form of the root is a common form, named by its identifier;
      // every row of a form offers the same ones.
      form: id => chosenForm(source.choices?.defaultReportForm, id),
      // The main roles in «Выбор объекта: Роль»; the dictionaries of the
      // search in «Выбор объекта» of the common templates and the
      // constants, both branches open (snapshots dosyomka-2651/02 to 04).
      choose: (row, items) => row.key === 'defaultRoles' ? chooseObjects('Выбор объекта: Роль', source.choices?.[row.key] || [], items)
        : chooseObjects('Выбор объекта', source.choices?.[row.key] || [], items, {open: true})}),
  });
  return {...editor,
    open(value) { picture = null; mainSection = null; editor.open(value); },
    // What «Сохранить» writes with the description of the root
    // (RootWindowsValue): of the picture of the main section and of its
    // command interface, what their windows gave - null where nothing.
    windows() { return picture || mainSection ? {picture, mainSection} : null; },
    // The window of the command interface of the main section, from the
    // menu of the root as from its row.
    openMainSection: () => chooseMainSection(),
    // «Открыть командный интерфейс проекта» of the menu of the root: what
    // its window writes goes into the description the palette edits.
    async openCommandInterface() {
      if (!opened) return;
      const configuration = opened.configuration, model = createProjectModel({configuration});
      if (await showProjectCommandInterface(configuration, configuration.defaultLanguage, (order, visibility) => model.setSections(order, visibility))) onChange();
    },
  };
}
