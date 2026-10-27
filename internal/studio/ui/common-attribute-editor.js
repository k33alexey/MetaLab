'use strict';

/* The palette of a common attribute, the kind without a window
   (docs/block-00/obshchij-rekvizit.md, «Палитра»): «Основные» with the type
   and its qualifiers (valueSubject, the window of the type), «Использование»
   with «Состав: Открыть» - the window «Состав» (chooseCommonAttributeContent)
   - the auto-use, the separation and the use in the database,
   «Представление» as of a field. around holds the palette and its title. */
function createCommonAttributeEditor(host, onChange, around = {}) {
  return paletteEditor(host, onChange, around, {
    target: source => source.attribute,
    subject: source => typedObjectSubject(source, source.attribute, 'common-attributes'),
    context: (source, {changed}) => ({typeOffer: source.typeOffer, windows: ['content'], openWindow: async () => {
      const content = await chooseCommonAttributeContent(source);
      if (!content) return;
      if (content.length) source.attribute.content = content; else delete source.attribute.content;
      changed();
    }}),
  });
}

/* commonAttributeUses are the values of the column «Использование» of the
   window «Состав» (obshchij-rekvizit.md, «Окно „Состав“»). */
const commonAttributeUses = [['auto', 'Автоматически'], ['use', 'Использовать'], ['dont-use', 'Не использовать']];

/* chooseCommonAttributeContent is the window «Общий реквизит <Имя>:
   Состав» (obshchij-rekvizit.md, «Окно „Состав“»): the objects the
   attribute may reach by their branches (source.composition), each with its
   use - «Автоматически», «Использовать», «Не использовать» - and below,
   «Используемые объекты»: those the attribute reaches - used, or
   automatic with the auto-use on. A document journal stands with
   «Автоматически» and no other: the prototype offers «Использовать» and
   does not keep it. OK answers the content - an item an object with a use
   said or a condition of separation, in the order it had, new ones after -
   «Отмена» null. */
async function chooseCommonAttributeContent(source) {
  const attribute = source.attribute, items = structuredClone(attribute.content || []);
  const useOf = id => items.find(item => item.metadata === id)?.use || 'auto';
  // A document journal stores no data of its own and gets no field
  // (metadata.commonAttributeTargets): it is never used.
  const journals = new Set(source.composition.flatMap(function all(group) { return [...(group.objects || []), ...(group.children || []).flatMap(all)]; })
    .filter(item => item.fullName.startsWith('ЖурналДокументов.')).map(item => item.id));
  const reaches = id => !journals.has(id) && (useOf(id) === 'use' || useOf(id) === 'auto' && attribute.autoUse === 'use');
  const setUse = (id, use) => {
    const at = items.findIndex(item => item.metadata === id);
    if (at < 0) { if (use !== 'auto') items.push({metadata: id, use}); return; }
    if (use === 'auto' && !items[at].conditionalSeparation) items.splice(at, 1);
    else items[at] = {...items[at], use};
  };
  const content = node('div', undefined, 'common-attribute-content');
  const opened = new Set();
  function draw() {
    const offered = node('div', undefined, 'object-window-tree common-attribute-content-tree');
    const branch = (group, row) => {
      const children = [...(group.children || []).flatMap(inner => branch(inner, row)), ...(group.objects || []).flatMap(row)];
      if (!children.length && row !== offeredRow) return [];
      return [treeNode({icon: group.icon, caption: group.title, children, open: opened.has(group.title) || (group.objects || []).some(item => reaches(item.id)),
        toggled: open => { if (open) opened.add(group.title); else opened.delete(group.title); }}).element];
    };
    const offeredRow = item => {
      const journal = item.fullName.startsWith('ЖурналДокументов.');
      const use = selectInput(journal ? commonAttributeUses.slice(0, 1) : commonAttributeUses, journal ? 'auto' : useOf(item.id),
        value => { setUse(item.id, value); draw(); }, 'Использование');
      use.disabled = journal;
      const caption = node('span', undefined, 'common-attribute-content-row');
      caption.append(node('span', item.name), use);
      return [treeNode({icon: item.icon, caption}).element];
    };
    offered.append(...source.composition.flatMap(group => branch(group, offeredRow)));
    const used = node('div', undefined, 'object-window-tree common-attribute-content-used');
    const usedBranches = groups => groups.map(group => ({...group, objects: (group.objects || []).filter(item => reaches(item.id)), children: usedBranches(group.children || [])}));
    used.append(...usedBranches(source.composition).flatMap(group => branch(group, item => [treeNode({icon: item.icon, caption: item.name}).element])));
    const head = node('div', undefined, 'common-attribute-content-head');
    head.append(node('span', 'Объект'), node('span', 'Использование'));
    content.replaceChildren(head, offered, node('div', 'Используемые объекты', 'object-window-caption'), used);
  }
  draw();
  const answer = await showDialog({title: `Общий реквизит ${attribute.name}: Состав`, content, className: 'common-attribute-content-dialog',
    answers: [['ok', 'OK'], [null, 'Отмена']]});
  return answer === 'ok' ? items : null;
}
