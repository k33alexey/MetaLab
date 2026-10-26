'use strict';

/* The palette of a common template, which has no window of its own: a
   double click and «Открыть макет» open the editor of its content, «Свойства»
   the palette (docs/block-00/obshchij-maket.md). The rows are the kind's
   (source.palette): «Тип макета» shown and unavailable - it is chosen once,
   in the constructor - and «Макет: Открыть», unavailable until the editors
   of content are built (the item «редакторы макетов — как у прототипа»).
   around holds the palette and its title. */
function createTemplateEditor(host, onChange, around = {}) {
  return paletteEditor(host, onChange, around, {target: source => source.template});
}

/* showTemplateConstructor is the prototype's «Конструктор макета», which
   «Добавить» of a template opens (obshchij-maket.md, «Команды»; snapshots
   obshchij-maket/03-06): the name «Макет», the synonym following it, the
   comment, «Выберите тип макета:» - the ten kinds of the palette's row
   «Тип макета» (kinds), «Табличный документ» marked, «Active document»
   unavailable - and «Загрузить из файла:», available for binary data and an
   external component; «Готово», «Отмена», «Справка». project holds the
   languages; offer is the server's {name, kinds}: the name of a new
   template and the [key, caption] pairs of the kinds. draft, when given, is
   the constructor as it was left; check(draft) is what the name is checked
   by when its field is left (watchName). Answers the draft {name, title,
   comment, kind, content} or null when cancelled. */
async function showTemplateConstructor(project, offer, draft, check) {
  draft = structuredClone(draft || {name: offer.name, title: {}, comment: '', kind: offer.kinds[0][0]});
  const kinds = offer.kinds;
  const content = node('div', undefined, 'template-constructor');
  const synonym = createLocalizedField(project, code => draft.title?.[code], (language, value) => { draft.title = setTranslation(draft.title, language, value); }, () => {}, titleTextOptions);
  // A new name is put in its synonym, as everywhere a name is typed.
  if (!Object.keys(draft.title || {}).length) synonym.follow('', draft.name);
  const name = nameInput(draft.name, value => { const before = draft.name; draft.name = value; synonym.follow(before, value); },
    () => check(draft));
  const frame = radioFrame('Выберите тип макета:', kinds, draft.kind, kind => { draft.kind = kind; if (!loads()) { draft.content = undefined; draft.file = ''; } arrange(); });
  frame.querySelector('input[value="active-document"]').disabled = true;
  const loads = () => draft.kind === 'binary' || draft.kind === 'add-in';
  const file = node('input');
  file.readOnly = true;
  file.value = draft.file || '';
  const pick = fieldChoiceButton('Загрузить из файла', async () => {
    const chosen = await chooseFileContent();
    if (!chosen) return;
    draft.content = chosen.content;
    draft.file = file.value = chosen.name;
  });
  const loading = labeledField('Загрузить из файла:', fieldWithButtons(file, [pick]));
  function arrange() {
    pick.disabled = !loads();
    loading.classList.toggle('disabled', !loads());
    if (!loads()) file.value = '';
  }
  arrange();
  content.append(labeledField('Имя:', name), labeledField('Синоним:', synonym),
    labeledField('Комментарий:', textInput(draft.comment, value => { draft.comment = value; })), frame, loading);
  const answer = await showDialog({title: 'Конструктор макета', content, className: 'template-constructor-dialog', help: 'template-constructor',
    answers: [['done', 'Готово'], [null, 'Отмена']]});
  synonym.destroy?.();
  if (answer !== 'done') return null;
  return draft;
}

/* templateOffer is what the constructor offers, the server's: the name of a
   new template and the kinds. */
async function templateOffer() {
  const response = await fetch('/api/template/new');
  if (!response.ok) throw new Error(await response.text());
  return response.json();
}

/* constructTemplate is «Добавить» of a template, common or of an object:
   the constructor, then make(draft) - which answers what was made or throws
   the refusal; a refusal is shown and the constructor opens again as it was
   left. check(draft) checks the name, as in the constructor. Answers what
   make answered, or null when the constructor is cancelled. */
async function constructTemplate(project, check, make) {
  const offer = await templateOffer();
  let draft;
  for (;;) {
    draft = await showTemplateConstructor(project, offer, draft, check);
    if (!draft) return null;
    try {
      return await make(draft);
    } catch (error) {
      await askChoice(String(error.message || error).trim(), '', [['ok', 'ОК']]);
    }
  }
}
