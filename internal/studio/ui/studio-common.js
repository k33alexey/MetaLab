'use strict';

/* Studio-wide current display language: every localized title field shows
   this one language collapsed to a single line; a magnifying-glass button
   opens all configured translations at once (в духе 1С). Starts from the
   ML Project's own default language (set in Manager, carried into Studio)
   and can be switched within Studio to any configured project language. */
const StudioLanguage = (() => {
  let current = null;
  const listeners = new Set();
  return {
    get() { return current; },
    set(code) {
      if (!code || code === current) return;
      current = code;
      for (const listener of listeners) listener(code);
    },
    init(defaultCode) { if (current === null) current = defaultCode; },
    onChange(listener) { listeners.add(listener); return () => listeners.delete(listener); },
  };
})();

/* createLocalizedTitleField renders one collapsed-to-one-line title input
   bound to StudioLanguage.get(), plus a 🔍 button (hidden when the project
   has only one language — nothing to expand) that opens every configured
   language as its own input in a small dialog.
   languages: [{code,title}], getValue(code)=>string, setValue(code,value),
   onChange() called after every edit (collapsed or expanded). */
function createLocalizedTitleField(languages, getValue, setValue, onChange) {
  const wrapper = document.createElement('div');
  wrapper.className = 'localized-title-field';
  const input = document.createElement('input');
  input.type = 'text';
  input.maxLength = 512;
  function currentLanguage() { return StudioLanguage.get() || languages[0]?.code || ''; }
  function languageTitle(code) { return languages.find(item => item.code === code)?.title || code; }
  function refresh() { input.value = getValue(currentLanguage()) || ''; input.title = languageTitle(currentLanguage()); }
  input.addEventListener('input', () => { setValue(currentLanguage(), input.value); onChange(); });
  wrapper.append(input);
  if (languages.length > 1) {
    const expand = document.createElement('button');
    expand.type = 'button';
    expand.className = 'localized-title-expand';
    expand.title = 'Заголовки на всех языках проекта';
    expand.textContent = '🔍';
    expand.addEventListener('click', () => openLocalizedTitleDialog(languages, getValue, setValue, () => { refresh(); onChange(); }));
    wrapper.append(expand);
  }
  const unsubscribe = StudioLanguage.onChange(refresh);
  wrapper.dataset.localizedTitleField = 'true';
  wrapper.destroy = unsubscribe;
  refresh();
  return wrapper;
}

function openLocalizedTitleDialog(languages, getValue, setValue, onChange) {
  const dialog = document.createElement('dialog');
  dialog.className = 'localized-title-dialog';
  const form = document.createElement('form');
  form.method = 'dialog';
  const heading = document.createElement('h3');
  heading.textContent = 'Заголовок на языках проекта';
  form.append(heading);
  for (const language of languages) {
    const label = document.createElement('label');
    label.append(document.createTextNode(language.title || language.code));
    const fieldInput = document.createElement('input');
    fieldInput.type = 'text';
    fieldInput.maxLength = 512;
    fieldInput.value = getValue(language.code) || '';
    fieldInput.addEventListener('input', () => { setValue(language.code, fieldInput.value); onChange(); });
    label.append(fieldInput);
    form.append(label);
  }
  const actions = document.createElement('div');
  actions.className = 'localized-title-actions';
  const close = document.createElement('button');
  close.type = 'submit';
  close.className = 'primary';
  close.textContent = 'Готово';
  actions.append(close);
  form.append(actions);
  dialog.append(form);
  document.body.append(dialog);
  dialog.addEventListener('close', () => dialog.remove());
  dialog.showModal();
}
