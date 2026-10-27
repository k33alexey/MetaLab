'use strict';

/* The palette of a common attribute, the kind without a window
   (docs/block-00/obshchij-rekvizit.md, «Палитра»): «Основные» with the type
   and its qualifiers (valueSubject, the window of the type), «Использование»
   with «Состав: Открыть» - unavailable until the window «Состав» - the
   auto-use, the separation and the use in the database, «Представление» as
   of a field. around holds the palette and its title. */
function createCommonAttributeEditor(host, onChange, around = {}) {
  return paletteEditor(host, onChange, around, {
    target: source => source.attribute,
    subject: source => typedObjectSubject(source, source.attribute, 'common-attributes'),
    context: source => ({typeOffer: source.typeOffer}),
  });
}
