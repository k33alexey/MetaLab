'use strict';

/* The palette of a constant, the kind without a window: its double click,
   «Изменить» and «Свойства» open its palette, and the work area stays as it
   was (docs/block-00/konstanta.md, «Окно редактирования»). The rows are the
   kind's (source.palette); the fields are the palette's of every object
   (paletteFields), the type and its qualifiers the window of the type's
   (valueSubject). around holds the palette, its title, and the opening of a
   module and of a file. */
function createConstantEditor(host, onChange, around = {}) {
  return paletteEditor(host, onChange, around, {
    target: source => source.constant,
    subject: source => valueSubject(source.constant, source.typeOffer, types => { source.constant.types = types; }, {owner: 'constants',
      setName: value => { source.constant.name = value; }, check: () => ({path: source.path, name: source.constant.name})}),
    context: source => ({typeOffer: source.typeOffer, choices: source.choices,
      openModule: role => around.openModule?.(role), openFile: path => around.openFile?.(path),
      // The main form of a constant is a common form, named by its
      // identifier.
      form: id => {
        const form = objectChoiceItem(source.choices?.defaultForm, id);
        return {name: form?.name || id, path: form?.path};
      },
      choose: (row, items) => chooseObjects('Выбор объекта', source.choices?.[row.key] || [], items)}),
  });
}
