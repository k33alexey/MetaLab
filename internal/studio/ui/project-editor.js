'use strict';

/* Project editor: the configuration root (configuration.yaml) as fields instead of
   hand-edited YAML. The root is the configuration itself, so this panel holds
   what the configuration says about itself — its synonym, what it is, who made
   it and where to read more. Managing the language list is a separate,
   dedicated editor (languages-editor.js) reached only through the "Языки"
   branch of the configuration tree — not duplicated into this panel.

   Texts a person reads are stored per language, so each of them is edited as
   one field per configured language rather than one field. */
function createProjectModel(source) {
  const configuration = source.configuration;
  configuration.languages ||= [];
  function uniqueCode() {
    return uniqueName('lang', new Set(configuration.languages.map(item => item.code.toLocaleLowerCase())));
  }
  return {
    value() { return structuredClone(configuration); },
    setName(name) { configuration.name = name; },
    setField(field, value) {
      if (value === '') delete configuration[field]; else configuration[field] = value;
    },
    setText(field, code, value) {
      const text = configuration[field] && typeof configuration[field] === 'object' ? configuration[field] : {};
      if (value === '') delete text[code]; else text[code] = value;
      if (Object.keys(text).length === 0) delete configuration[field];
      else configuration[field] = text;
    },
    setDefaultLanguage(code) { configuration.defaultLanguage = code; },
    /* Роли перечислены в том порядке, в каком их перечислила конфигурация, и
       порядок сохраняется: снятая и снова поставленная роль встаёт в конец,
       потому что это и есть «добавили её сейчас». Роль, выданная дважды, не
       даёт больше прав, чем выданная однажды, поэтому повтор невозможен. */
    /* Словарь полнотекстового поиска лежит либо в общем макете, либо в
       константе, поэтому хранится парой «вид и объект»: по одному
       идентификатору читатель файла гадал бы, куда смотреть. */
    setFlag(field, on) { if (on) configuration[field] = true; else delete configuration[field]; },
    /* Назначение, названное дважды, остаётся одним назначением. */
    setPurpose(purpose, used) {
      const kept = (configuration.usePurposes || []).filter(item => item !== purpose);
      if (used) kept.push(purpose);
      if (kept.length === 0) delete configuration.usePurposes; else configuration.usePurposes = kept;
    },
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
    addLanguage() {
      const language = {name: 'Язык', title: 'Новый язык', code: uniqueCode()};
      configuration.languages.push(language);
      return language;
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
  let source, model;
  /* Подписи объектов показываются на текущем языке Studio и меняются вместе
     с ним. */
  StudioLanguage.onChange(() => { if (source) render(); });
  function rerender() { onChange(); render(); }
  function touch() { onChange(); }
  /* Длину ограничивает только то, что ограничено в модели: имя — 255
     знаков, как у всего, что называется идентификатором. У комментария,
     поставщика, версии и подписи предела нет ни в справке, ни в
     конфигураторе (подпись мобильного клиента в sb — 113 КБ). */
  function textField(label, value, apply, options = {}) {
    return labeledField(label, textInput(value, value => { apply(value); touch(); }, options));
  }
  function localizedField(label, field, options = {}) {
    const stored = source.configuration[field] || {};
    return localizedTextField(label, source.configuration.languages, code => stored[code],
      (code, value) => { model.setText(field, code, value); touch(); }, options);
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
    return localizedTitle(choice, source.configuration.defaultLanguage, source.configuration.languages) || choice.id;
  }
  function referenceField(label, field, choices) {
    const stored = source.configuration[field] || '';
    const select = selectInput([['', 'Не задано'], ...choices.map(choice => [choice.id, presentation(choice)])], stored,
      value => { model.setField(field, value); rerender(); });
    if (stored && !choices.some(choice => choice.id === stored)) {
      const missing = node('option', 'Не найдено в конфигурации: ' + stored, 'project-default-missing');
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
      entry => ({kind: entry.kind, id: entry.object, title: 'Не найден в конфигурации: ' + entry.object, chip: chip(entry.kind)}),
      'В конфигурации нет ни макетов, ни констант',
      (row, checked) => model.setDictionary(row.kind, row.id, checked));
  }
  function rolesField(label, field) {
    const known = (source.defaults?.roles || []).map(role => ({key: role.id, title: presentation(role)}));
    return choiceList(label, known, source.configuration[field] || [], id => id,
      id => ({title: 'Не найдена в конфигурации: ' + id}),
      'В конфигурации нет ни одной роли',
      (row, checked) => model.setRoleGranted(field, row.key, checked));
  }
  function checkField(label, field) {
    return checkboxField(label, Boolean(source.configuration[field]), checked => { model.setFlag(field, checked); touch(); }, 'project-role');
  }
  function purposesField(label) {
    const wrapper = node('div', undefined, 'project-localized');
    wrapper.append(node('span', label, 'project-localized-label'));
    const list = node('div', undefined, 'project-roles');
    const chosen = source.configuration.usePurposes || [];
    for (const [value, text] of [['platform-application', 'Приложение платформы'], ['mobile-platform-application', 'Приложение мобильной платформы']]) {
      list.append(checkboxField(text, chosen.includes(value), checked => { model.setPurpose(value, checked); touch(); }, 'project-role'));
    }
    wrapper.append(list);
    return wrapper;
  }
  function note(text) { return node('p', text, 'project-note'); }
  function section(title) {
    const block = node('section', undefined, 'project-section');
    block.append(node('h4', title));
    return block;
  }
  function render() {
    host.replaceChildren();
    const panel = node('div', undefined, 'catalog-detail');
    panel.append(node('h3', 'Корень конфигурации'));

    const identity = section('Как называется');
    identity.append(textField('Имя', source.configuration.name, value => model.setName(value), {maxLength: 255}));
    identity.append(localizedField('Синоним', 'title'));
    identity.append(textField('Комментарий', source.configuration.comment,
      value => model.setField('comment', value)));
    const languages = source.configuration.languages;
    identity.append(labeledField('Язык по умолчанию', selectInput(
      languages.map(language => [language.code, languageTitle(language, source.configuration.defaultLanguage, languages)]),
      source.configuration.defaultLanguage, value => { model.setDefaultLanguage(value); rerender(); })));
    panel.append(identity);

    const about = section('Что это за конфигурация');
    about.append(localizedField('Краткая информация', 'briefInformation'));
    about.append(localizedField('Подробная информация', 'detailedInformation', {kind: 'area', rows: 3}));
    panel.append(about);

    const vendor = section('Кто её выпустил');
    vendor.append(textField('Поставщик', source.configuration.vendor,
      value => model.setField('vendor', value)));
    vendor.append(textField('Версия', source.configuration.version,
      value => model.setField('version', value), {placeholder: '1.0.0.1'}));
    vendor.append(localizedField('Авторские права', 'copyright'));
    panel.append(vendor);

    const addresses = section('Где о ней прочитать');
    addresses.append(localizedField('Адрес поставщика', 'vendorAddress'));
    addresses.append(localizedField('Адрес конфигурации', 'informationAddress'));
    addresses.append(localizedField('Адрес обновлений', 'updateCatalogAddress'));
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
    appearance.append(textField('Основной интерфейс', source.configuration.defaultInterface,
      value => model.setField('defaultInterface', value), {maxLength: 255}));
    appearance.append(note('Основной интерфейс — меню и панели обычного приложения. ML строит только '
      + 'управляемый интерфейс, поэтому значение хранится как написано и ни на что не влияет.'));
    panel.append(appearance);

    const forms = section('Формы по умолчанию');
    forms.append(referenceField('Форма отчёта', 'defaultReportForm', defaults.commonForms || []));
    forms.append(referenceField('Форма настроек отчёта', 'defaultReportSettingsForm', defaults.commonForms || []));
    forms.append(referenceField('Форма варианта отчёта', 'defaultReportVariantForm', defaults.commonForms || []));
    forms.append(referenceField('Форма констант', 'defaultConstantsForm', defaults.commonForms || []));
    forms.append(referenceField('Форма поиска', 'defaultSearchForm', defaults.commonForms || []));
    forms.append(referenceField('Форма настроек динамического списка', 'defaultDynamicListSettingsForm',
      defaults.commonForms || []));
    forms.append(referenceField('Вспомогательная форма констант', 'auxiliaryConstantsForm',
      defaults.commonForms || []));
    forms.append(note('Все они — общие формы: они не принадлежат ни одному объекту, поэтому корень и '
      + 'может выдать их всем объектам сразу. Вспомогательная форма констант открывается тогда, когда '
      + 'основная не задана или не подходит.'));
    forms.append(referenceField('История данных: изменения', 'dataHistoryChangesForm', defaults.commonForms || []));
    forms.append(referenceField('История данных: версия', 'dataHistoryVersionForm', defaults.commonForms || []));
    forms.append(referenceField('История данных: различия версий', 'dataHistoryVersionDifferenceForm',
      defaults.commonForms || []));
    forms.append(referenceField('Выбор пользователей системы взаимодействия', 'collaborationSystemUsersChoiceForm',
      defaults.commonForms || []));
    forms.append(note('Последние четыре не откроются: история данных переносится и не исполняется, а системы '
      + 'взаимодействия в ML нет вовсе. Ссылка всё равно проверяется — указывающая в пустоту, она сообщает '
      + 'не о механизме, а о том, что форму удалили.'));
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
       читается и пишется, что становится с неиспользованным номером, на каком
       варианте языка написана конфигурация, с чего начинаются её имена и о
       каких словарях, кроме собственных, знает полнотекстовый поиск. */
    const settings = section('Как устроена сама конфигурация');
    settings.append(choiceField('Управление блокировкой данных', 'dataLockControl', [
      ['', 'Как в ML'], ['managed', 'Управляемый'], ['automatic', 'Автоматический'],
      ['automatic-and-managed', 'Автоматический и управляемый'],
    ]));
    settings.append(choiceField('Автонумерация объектов', 'objectAutonumeration', [
      ['', 'Как в ML'], ['release', 'Освобождать номер'], ['keep', 'Не освобождать номер'],
    ]));
    settings.append(choiceField('Вариант встроенного языка', 'scriptVariant', [
      ['', 'Как в ML'], ['russian', 'Русский'], ['english', 'Английский'],
    ]));
    settings.append(textField('Префикс имён', source.configuration.namePrefix,
      value => model.setField('namePrefix', value), {maxLength: 255}));
    settings.append(dictionariesField('Дополнительные словари поиска'));
    settings.append(note('Освобождённый номер достаётся следующему объекту, и в нумерации нет дыр; '
      + 'неосвобождённый потрачен, даже если объект так и не записали.'));
    panel.append(settings);

    /* Режимы прототипа. Почти все описывают поведение, которого в ML нет
       вовсе: обычное приложение, модальные окна, синхронные вызовы расширений
       и внешних компонент, табличные пространства и совместимость с прежними
       выпусками. Они хранятся и показываются, но ни на что не влияют, — и
       раздел говорит об этом прямо, чтобы никто не искал, почему выбранное
       ничего не меняет. */
    /* Ответ на «используется ли»: тремя словами, где прототип предупреждает,
       прежде чем отказать, и двумя везде, где нет (project.UseMode). */
    const plainUse = [['', 'Не задано'], ['do-not-use', 'Не использовать'], ['use', 'Использовать']];
    const warnedUse = [...plainUse, ['use-with-warnings', 'Использовать с предупреждениями']];
    const modes = section('Режимы прототипа');
    modes.append(note('ML исполняет только то, что умеет сам: управляемое приложение без модальных окон '
      + 'и без синхронных вызовов. Всё в этом разделе переносится, показывается и ни на что не влияет — '
      + 'но и не теряется: перенесённая конфигурация не должна молча лишаться части своих свойств.'));
    modes.append(choiceField('Основной режим запуска', 'defaultRunMode', [
      ['', 'Не задан'], ['managed-application', 'Управляемое приложение'],
      ['ordinary-application', 'Обычное приложение'], ['auto', 'Автоматически'],
    ]));
    modes.append(purposesField('Назначение использования'));
    modes.append(choiceField('Модальность', 'modalityUse', [['', 'Не задана'], ...warnedUse.slice(1)]));
    modes.append(choiceField('Синхронные вызовы расширений платформы', 'synchronousPlatformExtensionCallUse', warnedUse));
    modes.append(choiceField('Синхронные вызовы расширений (до 8.3.8)', 'synchronousExtensionCallUse', warnedUse));
    modes.append(choiceField('Совместимость интерфейса', 'interfaceCompatibility', [
      ['', 'Не задана'], ['taxi', 'Такси'], ['taxi-allow-version-8-2', 'Такси, разрешить 8.2'],
      ['version-8-2-allow-taxi', '8.2, разрешить Такси'], ['version-8-2', 'Версия 8.2'],
    ]));
    modes.append(choiceField('Основное окно приложения', 'mainWindowMode', [
      ['', 'Не задано'], ['normal', 'Обычное'], ['workplace', 'Рабочее место'],
      ['fullscreen-workplace', 'Полноэкранное рабочее место'],
      ['embedded-workplace', 'Встроенное рабочее место'], ['kiosk', 'Киоск'],
    ]));
    modes.append(choiceField('Табличные пространства базы', 'databaseTablespacesUse', plainUse));
    modes.append(choiceField('Хранилище двоичных данных', 'binaryDataStorage', plainUse));
    modes.append(choiceField('Блочное хранение двоичных данных', 'binaryDataBlockStorageUse', plainUse));
    modes.append(textField('Режим совместимости', source.configuration.compatibilityVersion,
      value => model.setField('compatibilityVersion', value), {maxLength: 16, placeholder: '8.3.21'}));
    modes.append(textField('Совместимость расширений', source.configuration.extensionCompatibilityVersion,
      value => model.setField('extensionCompatibilityVersion', value), {maxLength: 16, placeholder: '8.3.27'}));
    modes.append(checkField('Использовать управляемые формы в обычном приложении', 'useManagedFormsInOrdinaryApplication'));
    modes.append(checkField('Использовать обычные формы в управляемом приложении', 'useOrdinaryFormsInManagedApplication'));
    modes.append(checkField('Включать в содержание справки', 'includeHelpInContents'));
    modes.append(note('Режим совместимости — выпуск прототипа, на который писалась конфигурация: '
      + 'пустое поле означает, что ни на какой прежний выпуск она не оглядывается. Список выпусков '
      + 'растёт без нас, поэтому он и не список, а версия.'));
    panel.append(modes);

    host.append(panel);
  }
  return {
    open(value) {
      source = structuredClone(value);
      model = createProjectModel(source);
      host.hidden = false;
      render();
    },
    value() { return model?.value(); },
    close() { source = null; model = null; host.hidden = true; host.replaceChildren(); },
    setDisabled(disabled) { host.inert = disabled; },
  };
}
