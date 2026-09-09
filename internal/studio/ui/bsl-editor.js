(() => {
  'use strict';

  window.createBSLEditor = textarea => {
    const host=document.querySelector('#editor-host'),gutter=document.querySelector('#line-gutter'),layer=document.querySelector('#syntax-layer'),code=document.querySelector('#syntax-code'),panel=document.querySelector('#bsl-panel'),properties=document.querySelector('#properties'),symbols=document.querySelector('#bsl-symbols'),problems=document.querySelector('#bsl-diagnostics'),summary=document.querySelector('#bsl-summary'),inspectorTitle=document.querySelector('#inspector-title');
    let file=null,sourceText='',analysis=null,active=false,timer=0,generation=0,controller=null,selectedPair=null,selectedPairKey='',folded=null;

    function open(nextFile){
      generation++;if(controller)controller.abort();clearTimeout(timer);file=nextFile;sourceText=nextFile.content;analysis=null;selectedPair=null;selectedPairKey='';folded=null;active=nextFile.language==='bsl';textarea.readOnly=false;host.classList.toggle('bsl-active',active);host.classList.remove('editor-folded');panel.hidden=!active;properties.hidden=active;inspectorTitle.textContent=active?'BSL':'Свойства';summary.textContent='';summary.className='muted';code.textContent=active?sourceText+'\n':'';renderGutter();renderInspector();if(active)schedule(0)
    }

    function close(){generation++;if(controller)controller.abort();clearTimeout(timer);file=null;sourceText='';analysis=null;active=false;folded=null;selectedPair=null;selectedPairKey='';textarea.readOnly=false;host.classList.remove('bsl-active','editor-folded');panel.hidden=true;properties.hidden=false;inspectorTitle.textContent='Свойства';summary.textContent='';summary.className='muted';code.textContent='';gutter.replaceChildren()}

    function change(value){
      if(!active)return;sourceText=value;folded=null;host.classList.remove('editor-folded');textarea.readOnly=false;renderGutter();schedule(220)
    }

    function schedule(delay){
      clearTimeout(timer);const current=++generation;summary.textContent='Анализ…';summary.className='muted';timer=setTimeout(()=>analyze(current),delay)
    }

    async function analyze(current){
      if(!active||!file||current!==generation)return;if(controller)controller.abort();controller=new AbortController();const requestedSource=sourceText,requestedPath=file.path;
      try{
        const response=await fetch('/api/bsl/analyze',{method:'POST',headers:{'Content-Type':'application/json','X-ML-CSRF':'1'},body:JSON.stringify({path:requestedPath,content:requestedSource}),signal:controller.signal});
        if(!response.ok)throw new Error(await response.text());const result=await response.json();if(current!==generation||sourceText!==requestedSource||file?.path!==requestedPath)return;analysis=result;selectedPair=null;selectedPairKey='';renderAll()
      }catch(error){if(error.name==='AbortError')return;if(current!==generation)return;summary.textContent='Ошибка анализа';summary.className='error';problems.replaceChildren(emptyItem(String(error.message||error)))}
    }

    function renderAll(){renderHighlight();renderGutter();renderInspector();const count=analysis?.diagnostics?.length||0;summary.textContent=count?`Ошибок: ${count}`:'Ошибок нет';summary.className=count?'error':'muted';if(analysis?.truncated){summary.textContent+=' · результат ограничен';summary.className='warning'}}

    function renderHighlight(){
      if(!active)return;if(folded){renderFolded();return}const resolve=positionResolver(),ranges=(analysis?.highlights||[]).map(item=>({...item,offsets:rangeOffsets(item.range,resolve)})).sort((a,b)=>a.offsets[0]-b.offsets[0]),diagnosticRanges=(analysis?.diagnostics||[]).map(item=>{const range=rangeOffsets(item.range,resolve);range[1]=Math.max(range[1],range[0]+1);return range}).sort((a,b)=>a[0]-b[0]);let cursor=0,diagnosticIndex=0,html='';
      for(const item of ranges){const [start,end]=item.offsets;if(start<cursor||end<=start||start>sourceText.length)continue;while(diagnosticIndex<diagnosticRanges.length&&diagnosticRanges[diagnosticIndex][1]<=start)diagnosticIndex++;html+=escapeHTML(sourceText.slice(cursor,start));const classes=[`tok-${item.kind}`];if(diagnosticIndex<diagnosticRanges.length&&overlaps(start,end,diagnosticRanges[diagnosticIndex][0],diagnosticRanges[diagnosticIndex][1]))classes.push('syntax-error');if(selectedPair&&(sameRange(item.range,selectedPair.open)||sameRange(item.range,selectedPair.close)))classes.push('pair-match');html+=`<span class="${classes.join(' ')}">${escapeHTML(sourceText.slice(start,end))}</span>`;cursor=end}
      code.innerHTML=html+escapeHTML(sourceText.slice(cursor))+'\n';syncScroll()
    }

    function renderInspector(){
      symbols.replaceChildren();problems.replaceChildren();if(!active)return;
      const symbolItems=analysis?.symbols||[];if(!symbolItems.length)symbols.append(emptyItem(analysis?'Нет процедур, функций и переменных':'Анализ…'));
      for(const item of symbolItems){const icon=item.kind==='function'?'ƒ':item.kind==='procedure'?'P':'V';symbols.append(navigationItem(icon,item.name,item.detail,item.range,false))}
      const diagnosticItems=analysis?.diagnostics||[];if(!diagnosticItems.length)problems.append(emptyItem(analysis?'Ошибок нет':'Анализ…'));
      for(const item of diagnosticItems)problems.append(navigationItem('!',`${item.code} · ${item.message}`,`Строка ${item.range.start.line}, столбец ${item.range.start.column}`,item.range,true))
    }

    function navigationItem(icon,label,detail,range,problem){const button=document.createElement('button');button.type='button';button.className=`bsl-item${problem?' bsl-problem':''}`;const iconNode=document.createElement('span');iconNode.className='bsl-item-icon';iconNode.textContent=icon;const text=document.createElement('span');text.className='bsl-item-text';text.textContent=label;if(detail){const small=document.createElement('span');small.className='bsl-item-detail';small.textContent=detail;text.append(small)}button.append(iconNode,text);button.addEventListener('click',()=>goTo(range.start));return button}
    function emptyItem(text){const item=document.createElement('div');item.className='bsl-empty';item.textContent=text;return item}

    function renderGutter(){
      const lineCount=Math.min(sourceText.split('\n').length,50000),foldByLine=new Map(),fragment=document.createDocumentFragment();for(const item of analysis?.folds||[])if(!foldByLine.has(item.startLine))foldByLine.set(item.startLine,item);
      const appendLine=line=>{const row=document.createElement('div');row.className='gutter-line';const fold=typeof line==='number'?foldByLine.get(line):null;if(fold){const button=document.createElement('button');button.type='button';button.textContent=folded===fold?'+':'−';button.title=folded===fold?'Развернуть блок':`Свернуть: ${fold.label}`;button.addEventListener('click',()=>toggleFold(fold));row.append(button)}else row.append(document.createElement('span'));const number=document.createElement('span');number.textContent=typeof line==='number'?String(line):'⋯';row.append(number);fragment.append(row)};
      if(folded){for(let line=1;line<=Math.min(folded.startLine,lineCount);line++)appendLine(line);appendLine('fold');for(let line=Math.max(folded.endLine,folded.startLine+1);line<=lineCount;line++)appendLine(line)}else for(let line=1;line<=lineCount;line++)appendLine(line);gutter.replaceChildren(fragment);syncScroll()
    }

    function toggleFold(item){if(folded){unfold();return}folded=item;textarea.readOnly=true;host.classList.add('editor-folded');renderFolded();renderGutter();layer.scrollTop=Math.max(0,(item.startLine-3)*21.7);gutter.scrollTop=layer.scrollTop}
    function unfold(){const line=folded?.startLine||1;folded=null;textarea.readOnly=false;host.classList.remove('editor-folded');renderHighlight();renderGutter();const offset=positionResolver()({line,column:1});textarea.focus();textarea.setSelectionRange(offset,offset);textarea.scrollTop=Math.max(0,(line-3)*21.7);syncScroll()}
    function renderFolded(){if(!folded)return;const lines=sourceText.split('\n'),hidden=Math.max(0,folded.endLine-folded.startLine-1),before=lines.slice(0,folded.startLine).join('\n'),after=lines.slice(folded.endLine-1).join('\n');code.innerHTML=escapeHTML(before)+'\n'+`<span class="fold-placeholder" title="Нажмите, чтобы развернуть">⋯ ${hidden} строк ⋯</span>`+'\n'+escapeHTML(after);layer.querySelector('.fold-placeholder')?.addEventListener('click',unfold)}

    function updatePair(){if(!active||folded||!analysis)return;const cursor=textarea.selectionStart,resolve=positionResolver();let next=null;for(const pair of analysis.pairs||[]){const open=rangeOffsets(pair.open,resolve),close=rangeOffsets(pair.close,resolve);if(inRange(cursor,open)||inRange(cursor,close)){next=pair;break}}const key=next?`${next.kind}:${next.open.start.line}:${next.open.start.column}:${next.close.start.line}:${next.close.start.column}`:'';if(key===selectedPairKey)return;selectedPair=next;selectedPairKey=key;renderHighlight()}
    function goTo(position){if(folded)unfold();const offset=positionResolver()(position);textarea.focus();textarea.setSelectionRange(offset,offset);textarea.scrollTop=Math.max(0,(position.line-3)*21.7);syncScroll();updatePair()}
    function rangeOffsets(range,resolve){return[resolve(range.start),resolve(range.end)]}
    function positionResolver(){const starts=lineStarts(sourceText),columns=new Map();return position=>{const line=Math.max(0,Math.min(starts.length-1,position.line-1));let offsets=columns.get(line);if(!offsets){offsets=[starts[line]];let current=starts[line];while(current<sourceText.length){const point=sourceText.codePointAt(current);if(point===10||point===13)break;current+=point>0xffff?2:1;offsets.push(current)}columns.set(line,offsets)}return offsets[Math.max(0,Math.min(offsets.length-1,position.column-1))]}}
    function lineStarts(value){const starts=[0];for(let index=0;index<value.length;index++){const current=value.charCodeAt(index);if(current===13){if(value.charCodeAt(index+1)===10)index++;starts.push(index+1)}else if(current===10)starts.push(index+1)}return starts}
    function sameRange(left,right){return left.start.line===right.start.line&&left.start.column===right.start.column&&left.end.line===right.end.line&&left.end.column===right.end.column}
    function overlaps(aStart,aEnd,bStart,bEnd){return aStart<Math.max(bEnd,bStart+1)&&bStart<aEnd}
    function inRange(offset,range){return offset>=range[0]&&offset<=range[1]}
    function escapeHTML(value){return value.replace(/[&<>]/g,char=>char==='&'?'&amp;':char==='<'?'&lt;':'&gt;')}
    function syncScroll(){if(folded)return;layer.scrollTop=textarea.scrollTop;layer.scrollLeft=textarea.scrollLeft;gutter.scrollTop=textarea.scrollTop}

    textarea.addEventListener('scroll',syncScroll);textarea.addEventListener('click',updatePair);textarea.addEventListener('keyup',updatePair);layer.addEventListener('scroll',()=>{if(folded)gutter.scrollTop=layer.scrollTop});
    return{open,close,change,source:()=>active?sourceText:textarea.value};
  };
})();
