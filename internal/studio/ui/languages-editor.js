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
  let source, model, selected = null;
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
    input.maxLength = options.maxLength || 128;
    input.addEventListener('input', () => { apply(input.value); touch(); });
    wrapper.append(input);
    return wrapper;
  }
  // Passive fallback only: reachable if the editor is reopened (e.g. after
  // an external file change) without a specific language re-selected yet.
  // Plain text, no click targets — navigation happens through the tree.
  function renderOverview() {
    const panel = node('div', undefined, 'catalog-detail');
    panel.append(node('h3', 'Языки'));
    for (const language of source.configuration.languages) panel.append(node('div', language.title || language.code, 'catalog-uuid'));
    return panel;
  }
  function renderLanguage(language) {
    const locked = language.code === 'en';
    const panel = node('div', undefined, 'catalog-detail');
    panel.append(node('h3', language.title || language.code));
    panel.append(textField('Код', language.code, () => {}, {disabled: true, maxLength: 16}));
    panel.append(textField('Имя', language.name, value => model.setLanguageField(language.code, 'name', value), {disabled: locked}));
    panel.append(textField('Заголовок', language.title, value => model.setLanguageField(language.code, 'title', value), {disabled: locked}));
    if (locked) panel.append(node('div', 'Всегда доступен, нельзя изменить или удалить', 'catalog-uuid'));
    return panel;
  }
  function render() {
    host.replaceChildren();
    const language = selected && source.configuration.languages.find(item => item.code === selected);
    host.append(language ? renderLanguage(language) : renderOverview());
  }
  return {
    open(value) {
      source = structuredClone(value);
      model = createProjectModel(source);
      selected = null;
      host.hidden = false;
      render();
    },
    value() { return model?.value(); },
    close() { source = null; model = null; selected = null; host.hidden = true; host.replaceChildren(); },
    setDisabled(disabled) { host.inert = disabled; },
    // Switches straight to the language named by a tree-node fragment
    // (`language:<code>`), so a click on the main configuration tree opens
    // that one language's own fields instead of the overview list.
    selectFragment(fragment) {
      if (!source || !fragment) return;
      const [kind, code] = fragment.split(':');
      if (kind !== 'language') return;
      selected = code;
      render();
    },
  };
}
