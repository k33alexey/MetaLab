(() => {
  'use strict';

  window.createBSLEditor = textarea => {
    const host=document.querySelector('#editor-host'),stack=document.querySelector('#editor-stack'),gutter=document.querySelector('#line-gutter'),layer=document.querySelector('#syntax-layer'),code=document.querySelector('#syntax-code'),panel=document.querySelector('#bsl-panel'),properties=document.querySelector('#properties'),symbols=document.querySelector('#bsl-symbols'),problems=document.querySelector('#bsl-diagnostics'),summary=document.querySelector('#bsl-summary'),inspectorTitle=document.querySelector('#inspector-title');
    const completion=document.createElement('div'),signature=document.createElement('div');completion.id='bsl-completion';completion.role='listbox';completion.hidden=true;signature.id='bsl-signature';signature.hidden=true;stack.append(completion,signature);
    let file=null,sourceText='',analysis=null,active=false,timer=0,generation=0,controller=null,selectedPair=null,selectedPairKey='',folded=null,completionTimer=0,completionGeneration=0,completionController=null,completionResult=null,completionSelection=0;

    function open(nextFile){
      generation++;cancelCompletion();if(controller)controller.abort();clearTimeout(timer);file=nextFile;sourceText=nextFile.content;analysis=null;selectedPair=null;selectedPairKey='';folded=null;active=nextFile.language==='bsl';textarea.readOnly=false;host.classList.toggle('bsl-active',active);host.classList.remove('editor-folded');panel.hidden=!active;properties.hidden=active;inspectorTitle.textContent=active?'BSL':'Свойства';summary.textContent='';summary.className='muted';code.textContent=active?sourceText+'\n':'';renderGutter();renderInspector();if(active)schedule(0)
    }

    function close(){generation++;cancelCompletion();if(controller)controller.abort();clearTimeout(timer);file=null;sourceText='';analysis=null;active=false;folded=null;selectedPair=null;selectedPairKey='';textarea.readOnly=false;host.classList.remove('bsl-active','editor-folded');panel.hidden=true;properties.hidden=false;inspectorTitle.textContent='Свойства';summary.textContent='';summary.className='muted';code.textContent='';gutter.replaceChildren()}

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

    function cancelCompletion(){completionGeneration++;clearTimeout(completionTimer);if(completionController)completionController.abort();completionResult=null;completion.hidden=true;signature.hidden=true}
    function scheduleCompletion(force=false,caretMove=false){
      clearTimeout(completionTimer);if(!active||!file||folded||textarea.selectionStart!==textarea.selectionEnd){hideCompletion();return}const cursor=textarea.selectionStart,last=sourceText.slice(Math.max(0,cursor-2),cursor);if(!force&&!caretMove&&!/[\p{L}\p{N}_.(),&]$/u.test(last)&&signature.hidden){hideCompletion();return}const current=++completionGeneration;completionTimer=setTimeout(()=>requestCompletion(current,force),force?0:90)
    }
    async function requestCompletion(current,force){
      if(!active||!file||current!==completionGeneration)return;if(completionController)completionController.abort();completionController=new AbortController();const requestedSource=sourceText,requestedPath=file.path,position=cursorPosition(textarea.selectionStart);
      try{const response=await fetch('/api/bsl/complete',{method:'POST',headers:{'Content-Type':'application/json','X-ML-CSRF':'1'},body:JSON.stringify({path:requestedPath,content:requestedSource,position}),signal:completionController.signal});if(!response.ok)throw new Error(await response.text());const result=await response.json();if(current!==completionGeneration||sourceText!==requestedSource||file?.path!==requestedPath)return;completionResult=result;completionSelection=0;renderCompletion(force,position)}catch(error){if(error.name!=='AbortError'&&current===completionGeneration)hideCompletion()}
    }
    function renderCompletion(force,position){
      renderSignature(completionResult?.signature,position);const items=completionResult?.items||[];completion.replaceChildren();if(!items.length){completion.hidden=true;return}const prefix=sourceSlice(completionResult.replace);if(!force&&!prefix&&!sourceText.slice(0,textarea.selectionStart).endsWith('.')){completion.hidden=true;return}items.forEach((item,index)=>{const button=document.createElement('button');button.type='button';button.role='option';button.className='completion-item';button.setAttribute('aria-selected',String(index===completionSelection));const icon=document.createElement('span'),text=document.createElement('span'),detail=document.createElement('span');icon.className=`completion-kind kind-${item.kind}`;icon.textContent=completionIcon(item.kind);text.className='completion-label';text.textContent=item.label;detail.className='completion-detail';detail.textContent=item.detail||'';button.append(icon,text,detail);button.addEventListener('mousedown',event=>{event.preventDefault();applyCompletion(index)});completion.append(button)});completion.hidden=false;positionPopup(completion,position)
    }
    function renderSignature(value,position){signature.replaceChildren();if(!value){signature.hidden=true;return}const label=document.createElement('strong');label.textContent=value.label;signature.append(label);const parameter=value.parameters?.[value.activeParameter]?.label;if(parameter){const activeParameter=document.createElement('span');activeParameter.textContent=`Параметр: ${parameter}`;signature.append(activeParameter)}signature.hidden=false;positionPopup(signature,position);signature.style.transform='translateY(calc(-100% - 6px))'}
    function applyCompletion(index){const item=completionResult?.items?.[index];if(!item)return;const resolve=positionResolver(),start=resolve(completionResult.replace.start),end=resolve(completionResult.replace.end);textarea.focus();textarea.setRangeText(item.insertText,start,end,'end');hideCompletion();textarea.dispatchEvent(new Event('input',{bubbles:true}));scheduleCompletion(false)}
    function moveCompletion(delta){const count=completionResult?.items?.length||0;if(!count)return;completionSelection=(completionSelection+delta+count)%count;const rows=completion.querySelectorAll('.completion-item');rows.forEach((row,index)=>row.setAttribute('aria-selected',String(index===completionSelection)));rows[completionSelection]?.scrollIntoView({block:'nearest'})}
    function hideCompletion(){completionResult=null;completion.hidden=true;signature.hidden=true}
    function positionPopup(node,position){const charWidth=8.45,lineHeight=21.7,width=node===completion?468:Math.min(620,node.scrollWidth||620),left=20+(position.column-1)*charWidth-textarea.scrollLeft;node.style.left=`${Math.max(4,Math.min(left,stack.clientWidth-width-8))}px`;node.style.top=`${Math.max(4,16+position.line*lineHeight-textarea.scrollTop)}px`}
    function cursorPosition(offset){const before=sourceText.slice(0,offset),lineBreak=Math.max(before.lastIndexOf('\n'),before.lastIndexOf('\r'));return{line:(before.match(/\r\n|\r|\n/g)||[]).length+1,column:Array.from(before.slice(lineBreak+1)).length+1}}
    function sourceSlice(range){const resolve=positionResolver();return sourceText.slice(resolve(range.start),resolve(range.end))}
    function completionIcon(kind){if(kind==='function')return'ƒ';if(kind==='procedure'||kind==='method')return'P';if(kind==='variable'||kind==='parameter')return'V';if(kind==='module')return'M';if(kind==='keyword'||kind==='directive')return'K';return'◆'}

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

    textarea.addEventListener('scroll',()=>{syncScroll();if(completionResult){const position=completionResult.replace.end;if(!completion.hidden)positionPopup(completion,position);if(!signature.hidden)positionPopup(signature,position)}});textarea.addEventListener('input',()=>{if(active){sourceText=textarea.value;scheduleCompletion(false)}});textarea.addEventListener('click',()=>{updatePair();scheduleCompletion(false,true)});textarea.addEventListener('keyup',event=>{updatePair();if(!['ArrowDown','ArrowUp','Enter','Tab','Escape'].includes(event.key))scheduleCompletion(false,true)});textarea.addEventListener('keydown',event=>{if((event.ctrlKey||event.metaKey)&&event.code==='Space'){event.preventDefault();event.stopImmediatePropagation();scheduleCompletion(true);return}if(completion.hidden&&signature.hidden)return;if(event.key==='Escape'){event.preventDefault();event.stopImmediatePropagation();hideCompletion()}else if(!completion.hidden&&(event.key==='ArrowDown'||event.key==='ArrowUp')){event.preventDefault();event.stopImmediatePropagation();moveCompletion(event.key==='ArrowDown'?1:-1)}else if(!completion.hidden&&(event.key==='Enter'||event.key==='Tab')){event.preventDefault();event.stopImmediatePropagation();applyCompletion(completionSelection)}});textarea.addEventListener('blur',()=>setTimeout(()=>{if(document.activeElement!==textarea)hideCompletion()},120));layer.addEventListener('scroll',()=>{if(folded)gutter.scrollTop=layer.scrollTop});
    return{open,close,change,source:()=>active?sourceText:textarea.value};
  };
})();
