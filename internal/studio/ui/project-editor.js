'use strict';

/* Project editor: ML Project manifest (mlproject.yaml) as fields instead of
   hand-edited YAML. Managing the language list itself is a separate,
   dedicated editor (languages-editor.js) reached only through the "Языки"
   branch of the configuration tree — not duplicated into this panel. */
function createProjectModel(source) {
  const manifest = source.manifest;
  manifest.languages ||= [];
  function uniqueCode() {
    const used = new Set(manifest.languages.map(item => item.code.toLocaleLowerCase()));
    let index = 2, code = 'lang';
    while (used.has(code)) code = 'lang' + (index++);
    return code;
  }
  return {
    value() { return structuredClone(manifest); },
    setName(name) { manifest.name = name; },
    setTitle(title) { manifest.title = title; },
    setDefaultLanguage(code) { manifest.defaultLanguage = code; },
    addLanguage() {
      const language = {name: 'Язык', title: 'Новый язык', code: uniqueCode()};
      manifest.languages.push(language);
      return language;
    },
    removeLanguage(code) {
      if (code === 'en') return;
      manifest.languages = manifest.languages.filter(item => item.code !== code);
      if (manifest.defaultLanguage === code) manifest.defaultLanguage = manifest.languages[0]?.code || 'en';
    },
    setLanguageField(code, field, value) {
      if (code === 'en') return;
      const language = manifest.languages.find(item => item.code === code);
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
  function textField(label, value, apply, options = {}) {
    const wrapper = node('label', undefined, 'catalog-field'), input = node('input');
    wrapper.append(node('span', label));
    input.type = 'text';
    input.value = value ?? '';
    input.disabled = !!options.disabled;
    input.maxLength = options.maxLength || 512;
    input.addEventListener('input', () => { apply(input.value); touch(); });
    wrapper.append(input);
    return wrapper;
  }
  function render() {
    host.replaceChildren();
    const panel = node('div', undefined, 'catalog-detail');
    panel.append(node('h3', 'Свойства проекта'));
    panel.append(textField('Имя', source.manifest.name, value => model.setName(value), {maxLength: 128}));
    panel.append(textField('Заголовок', source.manifest.title, value => model.setTitle(value), {maxLength: 512}));
    const defaultLanguageField = node('label', undefined, 'catalog-field');
    defaultLanguageField.append(node('span', 'Язык по умолчанию'));
    const defaultLanguageSelect = node('select');
    for (const language of source.manifest.languages) {
      const option = node('option', language.title || language.code);
      option.value = language.code;
      option.selected = language.code === source.manifest.defaultLanguage;
      defaultLanguageSelect.append(option);
    }
    defaultLanguageSelect.addEventListener('change', () => { model.setDefaultLanguage(defaultLanguageSelect.value); rerender(); });
    defaultLanguageField.append(defaultLanguageSelect);
    panel.append(defaultLanguageField);
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
