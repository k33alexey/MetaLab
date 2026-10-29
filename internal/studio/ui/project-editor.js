'use strict';

/* Project editor: the configuration root (configuration.yaml) as fields instead of
   hand-edited YAML. The root is the configuration itself, so this panel holds
   what the configuration says about itself — its synonym, what it is, who made
   it and where to read more. Managing the language list is a separate,
   dedicated editor (languages-editor.js) reached only through the "Языки"
   branch of the configuration tree — not duplicated into this panel.

   Texts a person reads are stored per language, and each of them is one
   localized field: the translation in the language Studio edits in, and the
   magnifier that opens every language (createLocalizedField). */
function createProjectModel(source) {
  const configuration = source.configuration;
  configuration.languages ||= [];
  return {
    value() { return structuredClone(configuration); },
    setName(name) { configuration.name = name; },
    setField(field, value) {
      if (value === '') delete configuration[field]; else configuration[field] = value;
    },
    setText(field, code, value) {
      const text = setTranslation(configuration[field] && typeof configuration[field] === 'object' ? configuration[field] : undefined, code, value);
      if (Object.keys(text).length === 0) delete configuration[field];
      else configuration[field] = text;
    },
    setDefaultLanguage(code) { configuration.defaultLanguage = code; },
    /* The root's half of the command interface - the order of the sections
       and their visibility - as its window «Командный интерфейс» writes it
       (showProjectCommandInterface): nothing said, nothing kept. */
    setSections(order, visibility) {
      if (order.length) configuration.subsystemsOrder = order; else delete configuration.subsystemsOrder;
      if (visibility.length) configuration.subsystemsVisibility = visibility; else delete configuration.subsystemsVisibility;
    },
    /* Роли перечислены в том порядке, в каком их перечислила конфигурация, и
       порядок сохраняется: снятая и снова поставленная роль встаёт в конец,
       потому что это и есть «добавили её сейчас». Роль, выданная дважды, не
       даёт больше прав, чем выданная однажды, поэтому повтор невозможен. */
    /* Словарь полнотекстового поиска лежит либо в общем макете, либо в
       константе, поэтому хранится парой «вид и объект»: по одному
       идентификатору читатель файла гадал бы, куда смотреть. */
    setFlag(field, on) { if (on) configuration[field] = true; else delete configuration[field]; },
    setDictionary(kind, id, used) {
      const kept = (configuration.additionalFullTextSearchDictionaries || [])
        .filter(item => !(item.kind === kind && item.object === id));
      if (used) kept.push({kind, object: id});
      if (kept.length === 0) delete configuration.additionalFullTextSearchDictionaries;
      else configuration.additionalFullTextSearchDictionaries = kept;
    },
    setRoleGranted(field, id, granted) {
      const kept = (configuration[field] || []).filter(item => item !== id);
      if (granted) kept.push(id);
      if (kept.length === 0) delete configuration[field]; else configuration[field] = kept;
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

function createProjectEditor(host, onChange) {
  let source, model, fieldCleanups = [];
  /* Подписи объектов показываются на текущем языке Studio и меняются вместе
     с ним. */
  StudioLanguage.onChange(() => { if (source) render(); });
  function rerender() { onChange(); render(); }
  function touch() { onChange(); }
  /* Длину ограничивает только то, что ограничено в модели (limited): имя —
     пределом имени, как всё, что называется идентификатором. У комментария,
     поставщика, версии и подписи предела нет ни в справке, ни в
     конфигураторе (подпись мобильного клиента в sb — 113 КБ). */
  function textField(label, value, apply, options = {}) {
    return labeledField(label, textInput(value, value => { apply(value); touch(); }, options));
  }
  function localizedField(label, field, options = {}) {
    const text = createLocalizedField(source.configuration, code => source.configuration[field]?.[code],
      (code, value) => model.setText(field, code, value), touch, options);
    fieldCleanups.push(text.destroy);
    const labeled = labeledField(label, text);
    labeled.text = text;
    return labeled;
  }
  /* Умолчание называет другой объект конфигурации, поэтому оно выбирается из
     того, что в конфигурации есть, а не набирается руками: набранный
     идентификатор рано или поздно оказывается набран неверно.

     То, что корень называет, а конфигурация уже не содержит, остаётся в списке
     отдельной строкой и помечено. Умолчание, указывающее в пустоту, — это
     форма, которая не откроется; молча выбросить его значило бы потерять
     единственный след того, что имелось в виду, а показать его — единственный
     способ дать его починить. */
  function presentation(choice) {
    return localizedTitle(choice, source.configuration.defaultLanguage) || choice.id;
  }
  function referenceField(label, field, choices) {
    const stored = source.configuration[field] || '';
    const select = selectInput([['', 'Не задано'], ...choices.map(choice => [choice.id, presentation(choice)])], stored,
      value => { model.setField(field, value); rerender(); });
    if (stored && !choices.some(choice => choice.id === stored)) {
      const missing = node('option', 'Не найдено в проекте: ' + stored, 'project-default-missing');
      missing.value = stored;
      select.append(missing);
      select.classList.add('project-default-broken');
      select.value = stored;
    }
    return labeledField(label, select);
  }
  /* Ролей выдаётся сколько угодно, поэтому это не выбор одной строки, а список
     с отметками: видно сразу и что выдано, и что можно выдать. */
  /* Режим — слово из известного набора, поэтому он выбирается, а не пишется.
     Пустое значение названо своим смыслом: платформа делает по-своему, а не
     вопрос остался без ответа. */
  function choiceField(label, field, options) {
    return labeledField(label, selectInput(options, source.configuration[field] || '',
      value => { model.setField(field, value); rerender(); }));
  }
  /* Выбор из объектов конфигурации галочками: известные объекты, а за ними —
     выбранные, которых в конфигурации уже нет, чтобы ссылку в пустоту было
     видно и можно было снять. Строка — {key, title, chip?}; keyOf называет
     ключ выбранного, как его хранит корень. */
  function choiceList(label, known, chosen, keyOf, missingTitle, emptyText, apply) {
    const wrapper = node('div', undefined, 'project-localized');
    wrapper.append(node('span', label, 'project-localized-label'));
    const list = node('div', undefined, 'project-roles');
    const rows = known.map(item => ({...item, missing: false}));
    for (const entry of chosen) {
      const key = keyOf(entry);
      if (!rows.some(row => row.key === key)) rows.push({...missingTitle(entry), key, missing: true});
    }
    if (rows.length === 0) list.append(node('span', emptyText, 'project-empty'));
    for (const row of rows) {
      const box = checkboxField(row.title, chosen.some(entry => keyOf(entry) === row.key),
        checked => { apply(row, checked); rerender(); },
        row.missing ? 'project-role project-default-missing' : 'project-role');
      if (row.chip) box.firstChild.after(node('span', row.chip, 'project-language-chip'));
      list.append(box);
    }
    wrapper.append(list);
    return wrapper;
  }
  /* Словарей может быть сколько угодно, и каждый из них — либо общий макет,
     либо константа: вид написан рядом с именем, чтобы не гадать, что выбрано. */
  function dictionariesField(label) {
    const chip = kind => kind === 'constants' ? 'константа' : 'макет';
    const known = (source.defaults?.dictionaries || []).map(item =>
      ({key: item.kind + ':' + item.id, kind: item.kind, id: item.id, title: presentation(item), chip: chip(item.kind)}));
    return choiceList(label, known, source.configuration.additionalFullTextSearchDictionaries || [],
      entry => entry.kind + ':' + entry.object,
      entry => ({kind: entry.kind, id: entry.object, title: 'Не найден в проекте: ' + entry.object, chip: chip(entry.kind)}),
      'В проекте нет ни макетов, ни констант',
      (row, checked) => model.setDictionary(row.kind, row.id, checked));
  }
  function rolesField(label, field) {
    const known = (source.defaults?.roles || []).map(role => ({key: role.id, title: presentation(role)}));
    return choiceList(label, known, source.configuration[field] || [], id => id,
      id => ({title: 'Не найдена в проекте: ' + id}),
      'В проекте нет ни одной роли',
      (row, checked) => model.setRoleGranted(field, row.key, checked));
  }
  function checkField(label, field) {
    return checkboxField(label, Boolean(source.configuration[field]), checked => { model.setFlag(field, checked); touch(); }, 'project-role');
  }
  function note(text) { return node('p', text, 'project-note'); }
  function section(title) {
    const block = node('section', undefined, 'project-section');
    block.append(node('h4', title));
    return block;
  }
  function clearFields() {
    for (const cleanup of fieldCleanups) cleanup();
    fieldCleanups = [];
  }
  function render() {
    clearFields();
    host.replaceChildren();
    const panel = node('div', undefined, 'catalog-detail');
    panel.append(node('h3', 'Корень проекта'));

    const identity = section('Как называется');
    const synonym = localizedField('Синоним', 'title');
    const name = nameInput(source.configuration.name, value => {
      const before = source.configuration.name;
      model.setName(value);
      synonym.text.follow(before, value);
      touch();
    }, () => ({path: 'configuration.yaml', name: name.value}));
    identity.append(labeledField('Имя', name), synonym);
    identity.append(textField('Комментарий', source.configuration.comment,
      value => model.setField('comment', value)));
    const languages = source.configuration.languages;
    identity.append(labeledField('Язык по умолчанию', selectInput(
      languages.map(language => [language.code, languageTitle(language, source.configuration.defaultLanguage)]),
      source.configuration.defaultLanguage, value => { model.setDefaultLanguage(value); rerender(); })));
    panel.append(identity);

    const about = section('Что это за проект');
    about.append(localizedField('Краткая информация', 'briefInformation'));
    about.append(localizedField('Подробная информация', 'detailedInformation', {kind: 'area', rows: 3}));
    panel.append(about);

    const vendor = section('Кто его выпустил');
    vendor.append(textField('Поставщик', source.configuration.vendor,
      value => model.setField('vendor', value)));
    vendor.append(textField('Версия', source.configuration.version,
      value => model.setField('version', value), {placeholder: '1.0.0.1'}));
    vendor.append(localizedField('Авторские права', 'copyright'));
    panel.append(vendor);

    const addresses = section('Где о нём прочитать');
    addresses.append(localizedField('Адрес поставщика', 'vendorAddress'));
    addresses.append(localizedField('Адрес проекта', 'informationAddress'));
    panel.append(addresses);

    /* Пятнадцать умолчаний корня: чем приложение рисуется, под какими правами
       работает, какие формы открывает за объект, который своей формы не
       называет, и куда складывает то, что сохранил пользователь. Основной язык
       стоит выше, среди языков: он проверяется вместе с ними и читается раньше
       всего остального. */
    const defaults = source.defaults || {};
    const appearance = section('Чем рисуется и под какими правами');
    appearance.append(referenceField('Основной стиль', 'defaultStyle', defaults.styles || []));
    appearance.append(referenceField('Макет оформления отчётов', 'defaultReportAppearanceTemplate',
      defaults.appearanceTemplates || []));
    appearance.append(rolesField('Основные роли', 'defaultRoles'));
    panel.append(appearance);

    const forms = section('Формы по умолчанию');
    /* Форма по умолчанию корня — всегда общая форма: своей формы у корня нет. */
    const formField = (label, key) => referenceField(label, key, defaults.commonForms || []);
    forms.append(formField('Форма отчёта', 'defaultReportForm'));
    forms.append(formField('Форма настроек отчёта', 'defaultReportSettingsForm'));
    forms.append(formField('Форма варианта отчёта', 'defaultReportVariantForm'));
    forms.append(formField('Форма констант', 'defaultConstantsForm'));
    forms.append(formField('Форма поиска', 'defaultSearchForm'));
    forms.append(formField('Форма настроек динамического списка', 'defaultDynamicListSettingsForm'));
    forms.append(note('Все они — общие формы: они не принадлежат ни одному объекту, поэтому корень и '
      + 'может выдать их всем объектам сразу.'));
    panel.append(forms);

    const storages = section('Где хранится то, что сохранил пользователь');
    const settingsStorages = defaults.settingsStorages || [];
    storages.append(referenceField('Общие настройки', 'commonSettingsStorage', settingsStorages));
    storages.append(referenceField('Настройки отчётов', 'reportsUserSettingsStorage', settingsStorages));
    storages.append(referenceField('Варианты отчётов', 'reportsVariantsStorage', settingsStorages));
    storages.append(referenceField('Настройки динамических списков', 'dynamicListsUserSettingsStorage', settingsStorages));
    storages.append(referenceField('Данные форм', 'formDataSettingsStorage', settingsStorages));
    storages.append(referenceField('Внешние данные навигационных ссылок', 'urlExternalDataStorage', settingsStorages));
    storages.append(note('Незаполненное хранилище означает хранилище платформы, а не потерянные настройки.'));
    panel.append(storages);

    /* Настройки, которые не называют другой объект: чем запирается то, что
       читается и пишется, на каком варианте языка написана конфигурация, с
       чего начинаются её имена, о каких словарях, кроме собственных, знает
       полнотекстовый поиск и входит ли её справка в содержание. Режимы
       прототипа, которых ML не исполняет, — основное окно, хранение двоичных
       данных, автонумерация и прочие — не переносятся вовсе
       (METADATA-OBJECTS, «Корень конфигурации»). */
    const settings = section('Как устроен сам проект');
    settings.append(choiceField('Управление блокировкой данных', 'dataLockControl', [
      ['', 'Как в ML'], ['managed', 'Управляемый'], ['automatic', 'Автоматический'],
      ['automatic-and-managed', 'Автоматический и управляемый'],
    ]));
    settings.append(choiceField('Вариант встроенного языка', 'scriptVariant', [
      ['', 'Как в ML'], ['russian', 'Русский'], ['english', 'Английский'],
    ]));
    settings.append(textField('Префикс имён', source.configuration.namePrefix,
      value => model.setField('namePrefix', value), limited('name')));
    settings.append(dictionariesField('Дополнительные словари поиска'));
    settings.append(checkField('Включать в содержание справки', 'includeHelpInContents'));
    panel.append(settings);

    host.append(panel);
  }
  return hostedEditor(host, () => { clearFields(); source = null; model = null; }, {
    open(value) {
      source = structuredClone(value);
      model = createProjectModel(source);
      host.hidden = false;
      render();
    },
    value() { return model?.value(); },
    // «Открыть командный интерфейс проекта» of the menu of the root.
    async openCommandInterface() {
      const configuration = model.value();
      if (await showProjectCommandInterface(configuration, configuration.defaultLanguage, (order, visibility) => model.setSections(order, visibility))) touch();
    },
  });
}
