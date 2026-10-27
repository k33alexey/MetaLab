'use strict';

/* The window of a subsystem, as the prototype's (docs/block-00/
   podsistema.md, «Окно»; snapshots podsistema/04-07): the window of an
   object of rows (objectWindowEditor) - «Основные» with the flags, the
   button «Командный интерфейс», the explanation and the picture,
   «Функциональные опции» - the options of the project marked where the
   subsystem is in their content - and «Прочее» with the help - and its
   palette of three tabs. «Состав», the window «Командный интерфейс» and
   «Выбор картинки» are items of their own: their buttons and «…» stand
   unavailable. */
function createSubsystemEditor(host, onChange, around = {}) {
  return objectWindowEditor(host, onChange, around, {target: source => source.subsystem});
}
