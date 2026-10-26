'use strict';

/* The palette of a WS reference, which has no window to edit it in
   (docs/block-00/ws-ssylka.md): the name, the synonym, the comment, «URL
   источника» - the address the import wrote, a link nobody types into - and
   «WS-ссылка: Открыть», unavailable until the window viewing the
   description is built. around holds the palette and its title. */
function createWSReferenceEditor(host, onChange, around = {}) {
  return paletteEditor(host, onChange, around, {target: source => source.reference});
}

/* askWSDLAddress is the prototype's window «Введите URL импортируемого
   WSDL» (snapshot ws-ssylka/01): one line - an address of the network or a
   path to a file - with OK and «Отмена»; «Импорт WSDL…» opens it with the
   address before. Answers the address, or null when cancelled. */
async function askWSDLAddress(address = '') {
  let typed = address;
  const field = textInput(address, value => { typed = value; }, {placeholder: ''});
  field.classList.add('wsdl-address');
  const answer = await showDialog({title: 'Введите URL импортируемого WSDL', content: field, className: 'wsdl-address-dialog',
    answers: [['ok', 'ОК'], [null, 'Отмена']]});
  return answer === 'ok' ? typed : null;
}

/* importWSDL imports the description at the address the window gives: a new
   reference, or - with source - in place of that reference's description.
   A refusal is shown in the prototype's words and changes nothing. Answers
   the reference as its palette reads it, or null. */
async function importWSDL(source) {
  const address = await askWSDLAddress(source?.reference.locationUrl || '');
  if (address === null) return null;
  try {
    return await postJSON('/api/ws-reference/import', source ? {path: source.path, revision: source.revision, address} : {address});
  } catch (error) {
    await askChoice(String(error.message || error).trim(), '', [['ok', 'ОК']]);
    return null;
  }
}
