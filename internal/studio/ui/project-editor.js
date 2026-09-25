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
    const used = new Set(configuration.languages.map(item => item.code.toLocaleLowerCase()));
    let index = 2, code = 'lang';
    while (used.has(code)) code = 'lang' + (index++);
    return code;
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
    setRoleGranted(id, granted) {
      const kept = (configuration.defaultRoles || []).filter(item => item !== id);
      if (granted) kept.push(id);
      if (kept.length === 0) delete configuration.defaultRoles; else configuration.defaultRoles = kept;
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
      if (language) language[field] = value;
    },
  };
}

function createProjectEditor(host, onChange) {
  let source, model;
  function node(tag, text, className) {
    const element = document.createElement(tag);
    if (text !== undefined) element.textContent = text;
    if (className) element.className = className;
    return element;
  }
  function rerender() { onChange(); render(); }
  function touch() { onChange(); }
  function control(kind, value, apply, options = {}) {
    const input = node(kind === 'area' ? 'textarea' : 'input');
    if (kind !== 'area') input.type = 'text';
    input.value = value ?? '';
    if (kind === 'area') input.rows = options.rows || 3;
    input.maxLength = options.maxLength || 512;
    input.placeholder = options.placeholder || '';
    input.addEventListener('input', () => { apply(input.value); touch(); });
    return input;
  }
  function textField(label, value, apply, options = {}) {
    const wrapper = node('label', undefined, 'catalog-field');
    wrapper.append(node('span', label));
    wrapper.append(control(options.kind || 'input', value, apply, options));
    return wrapper;
  }
  /* One text in every language the project has: the language is shown beside
     its own field, so which translation is being written is never a guess. */
  function localizedField(label, field, options = {}) {
    const wrapper = node('div', undefined, 'project-localized');
    wrapper.append(node('span', label, 'project-localized-label'));
    const rows = node('div', undefined, 'project-localized-rows');
    const stored = source.configuration[field] || {};
    for (const language of source.configuration.languages) {
      const row = node('div', undefined, 'project-localized-row');
      row.append(node('span', language.code, 'project-language-chip'));
      row.append(control(options.kind || 'input', stored[language.code],
        value => model.setText(field, language.code, value), options));
      rows.append(row);
    }
    wrapper.append(rows);
    return wrapper;
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
    return choice.title?.[source.configuration.defaultLanguage] || choice.name || choice.id;
  }
  function referenceField(label, field, choices) {
    const wrapper = node('label', undefined, 'catalog-field');
    wrapper.append(node('span', label));
    const select = node('select');
    const stored = source.configuration[field] || '';
    const nothing = node('option', 'Не задано');
    nothing.value = '';
    select.append(nothing);
    for (const choice of choices) {
      const option = node('option', presentation(choice));
      option.value = choice.id;
      select.append(option);
    }
    if (stored && !choices.some(choice => choice.id === stored)) {
      const missing = node('option', 'Не найдено в конфигурации: ' + stored, 'project-default-missing');
      missing.value = stored;
      select.append(missing);
      select.classList.add('project-default-broken');
    }
    select.value = stored;
    select.addEventListener('change', () => { model.setField(field, select.value); rerender(); });
    wrapper.append(select);
    return wrapper;
  }
  /* Ролей выдаётся сколько угодно, поэтому это не выбор одной строки, а список
     с отметками: видно сразу и что выдано, и что можно выдать. */
  function rolesField(label) {
    const wrapper = node('div', undefined, 'project-localized');
    wrapper.append(node('span', label, 'project-localized-label'));
    const list = node('div', undefined, 'project-roles');
    const granted = source.configuration.defaultRoles || [];
    const known = source.defaults?.roles || [];
    const rows = [...known.map(role => ({id: role.id, title: presentation(role), missing: false}))];
    for (const id of granted) {
      if (!known.some(role => role.id === id)) rows.push({id, title: 'Не найдена в конфигурации: ' + id, missing: true});
    }
    if (rows.length === 0) list.append(node('span', 'В конфигурации нет ни одной роли', 'project-empty'));
    for (const role of rows) {
      const row = node('label', undefined, role.missing ? 'project-role project-default-missing' : 'project-role');
      const check = node('input');
      check.type = 'checkbox';
      check.checked = granted.includes(role.id);
      check.addEventListener('change', () => { model.setRoleGranted(role.id, check.checked); rerender(); });
      row.append(check);
      row.append(node('span', role.title));
      list.append(row);
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
    identity.append(textField('Имя', source.configuration.name, value => model.setName(value), {maxLength: 128}));
    identity.append(localizedField('Синоним', 'title'));
    identity.append(textField('Комментарий', source.configuration.comment,
      value => model.setField('comment', value), {maxLength: 1024}));
    const defaultLanguageField = node('label', undefined, 'catalog-field');
    defaultLanguageField.append(node('span', 'Язык по умолчанию'));
    const defaultLanguageSelect = node('select');
    for (const language of source.configuration.languages) {
      const option = node('option', language.title || language.code);
      option.value = language.code;
      option.selected = language.code === source.configuration.defaultLanguage;
      defaultLanguageSelect.append(option);
    }
    defaultLanguageSelect.addEventListener('change', () => { model.setDefaultLanguage(defaultLanguageSelect.value); rerender(); });
    defaultLanguageField.append(defaultLanguageSelect);
    identity.append(defaultLanguageField);
    panel.append(identity);

    const about = section('Что это за конфигурация');
    about.append(localizedField('Краткая информация', 'briefInformation'));
    about.append(localizedField('Подробная информация', 'detailedInformation', {kind: 'area', rows: 3}));
    panel.append(about);

    const vendor = section('Кто её выпустил');
    vendor.append(textField('Поставщик', source.configuration.vendor,
      value => model.setField('vendor', value)));
    vendor.append(textField('Версия', source.configuration.version,
      value => model.setField('version', value), {maxLength: 128, placeholder: '1.0.0.1'}));
    vendor.append(localizedField('Авторские права', 'copyright'));
    panel.append(vendor);

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
    appearance.append(rolesField('Основные роли'));
    appearance.append(textField('Основной интерфейс', source.configuration.defaultInterface,
      value => model.setField('defaultInterface', value), {maxLength: 128}));
    appearance.append(note('Основной интерфейс — меню и панели обычного приложения. ML строит только '
      + 'управляемый интерфейс, поэтому значение хранится как написано и ни на что не влияет.'));
    panel.append(appearance);

    const forms = section('Формы по умолчанию');
    forms.append(referenceField('Форма отчёта', 'defaultReportForm', defaults.commonForms || []));
    forms.append(referenceField('Форма настроек отчёта', 'defaultReportSettingsForm', defaults.commonForms || []));
    forms.append(referenceField('Форма варианта отчёта', 'defaultReportVariantForm', defaults.commonForms || []));
    forms.append(referenceField('Форма констант', 'defaultConstantsForm', defaults.commonForms || []));
    forms.append(referenceField('Форма поиска', 'defaultSearchForm', defaults.commonForms || []));
    forms.append(note('Все пять — общие формы: они не принадлежат ни одному объекту, поэтому корень и '
      + 'может выдать их всем объектам сразу.'));
    panel.append(forms);

    const storages = section('Где хранится то, что сохранил пользователь');
    const settingsStorages = defaults.settingsStorages || [];
    storages.append(referenceField('Общие настройки', 'commonSettingsStorage', settingsStorages));
    storages.append(referenceField('Настройки отчётов', 'reportsUserSettingsStorage', settingsStorages));
    storages.append(referenceField('Варианты отчётов', 'reportsVariantsStorage', settingsStorages));
    storages.append(referenceField('Настройки динамических списков', 'dynamicListsUserSettingsStorage', settingsStorages));
    storages.append(referenceField('Данные форм', 'formDataSettingsStorage', settingsStorages));
    storages.append(note('Незаполненное хранилище означает хранилище платформы, а не потерянные настройки.'));
    panel.append(storages);

    const addresses = section('Где о ней прочитать');
    addresses.append(localizedField('Адрес поставщика', 'vendorAddress'));
    addresses.append(localizedField('Адрес конфигурации', 'informationAddress'));
    addresses.append(localizedField('Адрес обновлений', 'updateCatalogAddress'));
    panel.append(addresses);

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
