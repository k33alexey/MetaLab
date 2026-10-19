'use strict';

/* Dedicated editor for the project's configured languages — reached only
   through the "Языки" branch of the configuration tree, 1C-style, never
   mixed into the project properties panel. Languages still live inline in
   the same configuration.yaml, so this reuses createProjectModel from
   project-editor.js (loaded on the same page) rather than a model of its
   own. Same unified style as every other tree-driven panel (catalogs,
   attributes, ...): the group node itself stays inert — clicking "Языки"
   shows nothing here, exactly like clicking "Справочники" shows nothing —
   only picking one specific language in the tree opens that one language's
   own properties. There is no in-panel back/delete navigation: add/delete
   for tree objects belongs to the configuration tree's own toolbar, not to
   ad-hoc buttons duplicated inside each panel. */
function createLanguagesEditor(host, onChange) {
  let source, model, selected = null, synonymCleanup = null;
  function touch() { onChange(); }
  /* Языки названы на языке Studio и меняются вместе с ним. */
  StudioLanguage.onChange(() => { if (source) render(); });
  /* Предел — только тот, что есть в модели: имя языка — 255 знаков, у
     комментария предела нет. */
  function textField(label, value, apply, options = {}) {
    return labeledField(label, textInput(value, value => { apply(value); touch(); },
      {maxLength: options.unbounded ? undefined : options.maxLength || 255, disabled: options.disabled}));
  }
  // Passive fallback only: reachable if the editor is reopened (e.g. after
  // an external file change) without a specific language re-selected yet.
  // Plain text, no click targets — navigation happens through the tree.
  function renderOverview() {
    const panel = node('div', undefined, 'catalog-detail');
    panel.append(node('h3', 'Языки'));
    for (const language of source.configuration.languages) panel.append(node('div', title(language), 'catalog-uuid'));
    return panel;
  }
  /* Синоним языка локализован, как у всякого объекта, и показывается на
     языке Studio — одним правилом languageTitle. */
  function title(language) {
    return languageTitle(language, source.configuration.defaultLanguage);
  }
  function renderLanguage(language) {
    const locked = language.code === 'en';
    const panel = node('div', undefined, 'catalog-detail');
    panel.append(node('h3', title(language)));
    panel.append(textField('Код', language.code, () => {}, {disabled: true, maxLength: 16}));
    panel.append(textField('Имя', language.name, value => model.setLanguageField(language.code, 'name', value), {disabled: locked}));
    /* Синоним — многоязычное поле, как всякий читаемый человеком текст
       конфигурации: язык называется на всех языках проекта. */
    const synonym = createLocalizedField(source.configuration, code => language.title?.[code],
      (code, value) => model.setLanguageTitle(language.code, code, value), touch, {disabled: locked});
    synonymCleanup = synonym.destroy;
    panel.append(labeledField('Синоним', synonym));
    panel.append(textField('Комментарий', language.comment,
      value => model.setLanguageField(language.code, 'comment', value), {disabled: locked, unbounded: true}));
    if (locked) panel.append(node('div', 'Всегда доступен, нельзя изменить или удалить', 'catalog-uuid'));
    return panel;
  }
  function render() {
    synonymCleanup?.();
    synonymCleanup = null;
    host.replaceChildren();
    const language = selected && source.configuration.languages.find(item => item.code === selected);
    host.append(language ? renderLanguage(language) : renderOverview());
  }
  return hostedEditor(host, () => { synonymCleanup?.(); synonymCleanup = null; source = null; model = null; selected = null; }, {
    open(value) {
      source = structuredClone(value);
      model = createProjectModel(source);
      selected = null;
      host.hidden = false;
      render();
    },
    value() { return model?.value(); },
    // Switches straight to the language a node of the tree names
    // (`language:<code>`), so a click on the main configuration tree opens
    // that one language's own fields instead of the overview list.
    select(kind, code) {
      if (!source || kind !== 'language') return;
      selected = code;
      render();
    },
  });
}
