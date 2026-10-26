'use strict';

/* The palette of a constant, the kind without a window: its double click,
   «Изменить» and «Свойства» open its palette, and the work area stays as it
   was (docs/block-00/konstanta.md, «Окно редактирования»). The rows are the
   kind's (source.palette); the fields are the palette's of every object
   (paletteFields), the type and its qualifiers the window of the type's
   (valueSubject). around holds the palette, its title, and the opening of a
   module and of a file. */
function createConstantEditor(host, onChange, around = {}) {
  let source = null, cleanups = [];
  const constant = () => source.constant;
  function clear() {
    cleanups.forEach(destroy => destroy());
    cleanups = [];
    around.palette?.replaceChildren();
    around.paletteTitle?.('Свойства');
  }
  // Typing renames the palette and leaves its fields as they are; anything
  // else draws the palette anew - the rows of a new type among them.
  function typed() { onChange(); around.paletteTitle?.(`Свойства: ${constant().name}`); }
  function changed() { onChange(); render(); }
  function subject() {
    return valueSubject(constant(), source.typeOffer, types => { constant().types = types; }, {owner: 'constants',
      setName: value => { constant().name = value; }, check: () => ({path: source.path, name: constant().name})});
  }
  function render() {
    if (!source || !around.palette) return;
    clear();
    around.paletteTitle?.(`Свойства: ${constant().name}`);
    const fields = paletteFields({project: source, typeOffer: source.typeOffer, choices: source.choices, cleanups: () => cleanups,
      typed, changed, openModule: role => around.openModule?.(role), openFile: path => around.openFile?.(path),
      // The main form of a constant is a common form, named by its
      // identifier.
      form: id => {
        const name = objectChoiceItem(source.choices?.defaultForm, id)?.name || id;
        return {name, path: `metadata/common-forms/${name}/form.yaml`};
      },
      choose: (row, items) => chooseObjects('Выбор объекта', source.choices?.[row.key] || [], items)});
    const shown = subject();
    around.palette.append(objectPalette(source.palette || [], row => fields(shown, row), paletteShown(shown)));
  }
  StudioLanguage.onChange(() => render());
  return hostedEditor(host, () => { if (source) clear(); source = null; }, {
    open(value) {
      if (source) clear();
      source = structuredClone(value);
      render();
    },
    value() { return source && structuredClone(constant()); },
    // The palette is all the constant has: a click on its node shows it.
    showPalette() {
      if (!source || !around.palette) return false;
      render();
      return true;
    },
  });
}
