import { markdownModel, renderMarkdown } from './markdownView.mjs'

// A Markdown file is offered a rendered view of its content (#575).
const isMarkdown=file=>file.kind==='text'&&/\.(md|markdown)$/i.test(file.path)

// Results belong only to the selected execution and latest explicit request.
export function createGitDiff({api,container,terminal,panel,divider,consoleButton,changesButton,onConsole}){
 let runID=null,generation=0,active=false,consoleVisible=true,result=null,selection=null
 // A Markdown file opens rendered; the raw diff is chosen per execution and
 // forgotten when another execution is selected.
 let rendered=true
 container.innerHTML='<div class="changes-toolbar"><button type="button" class="diff-refresh">Refresh</button><span class="diff-status" role="status" aria-live="polite"></span></div><p class="diff-error" role="alert" hidden></p><p class="diff-context"></p><p class="diff-summary"></p><div class="diff-body"><nav class="diff-files" aria-label="Changed files"></nav><div class="diff-detail"><button type="button" class="diff-render-toggle" aria-pressed="false" hidden>Rendered</button><p class="diff-file-info"></p><p class="diff-render-note" hidden></p><pre class="diff-patch" tabindex="0" aria-label="Selected file diff"></pre><div class="diff-rendered" tabindex="0" aria-label="Rendered Markdown" hidden></div></div></div>'
 const find=s=>container.querySelector(s)
 function clearDocument(){
  const toggle=find('.diff-render-toggle'),note=find('.diff-render-note'),view=find('.diff-rendered')
  toggle.hidden=true;toggle.disabled=false;toggle.setAttribute('aria-pressed','false')
  note.hidden=true;note.textContent='';view.hidden=true;view.replaceChildren();find('.diff-patch').hidden=false
 }
 function clear(){result=null;find('.diff-context').textContent='';find('.diff-summary').textContent='';find('.diff-files').replaceChildren();find('.diff-patch').replaceChildren();find('.diff-file-info').textContent='';clearDocument()}
 function showDocument(file){
  const toggle=find('.diff-render-toggle'),note=find('.diff-render-note'),view=find('.diff-rendered')
  const reason=!result.markdownDocuments?'Update and restart the local agent to render Markdown.':!file.document?'This file cannot be rendered.':file.document.omittedReason
  const shown=rendered&&!reason
  toggle.hidden=false;toggle.disabled=Boolean(reason);toggle.setAttribute('aria-pressed',String(shown))
  if(reason){note.textContent=reason;note.hidden=false}
  if(!shown)return false
  if(file.document.side==='old'){note.textContent='Old version: this file is deleted.';note.hidden=false}
  view.append(renderMarkdown(markdownModel(file.document.content),{openLink:url=>api.openLink(url)}))
  view.hidden=false;find('.diff-patch').hidden=true
  return true
 }
 function showFile(){
  const file=result?.files.find(f=>f.path===selection),patch=find('.diff-patch');patch.replaceChildren();clearDocument()
  for(const button of find('.diff-files').children)button.setAttribute('aria-pressed',String(button.dataset.path===selection))
  if(!file)return
  find('.diff-file-info').textContent=[file.oldPath?file.oldPath+' → '+file.path:file.path,file.status,file.kind,file.additions==null?'Text counts unavailable':`+${file.additions} −${file.deletions}`,file.omittedReason].filter(Boolean).join(' · ')
  if(isMarkdown(file)&&showDocument(file))return
  for(const line of (file.patch||'').split('\n')){const node=document.createElement('span');node.className=line.startsWith('+')?'diff-add':line.startsWith('-')?'diff-delete':line.startsWith('@@')?'diff-hunk':'diff-line';node.textContent=line+'\n';patch.append(node)}
 }
 function render(){
  find('.diff-context').textContent=`${result.taskId} · ${result.directory} · ${result.branch} · Baseline ${result.baseRef} at ${result.mergeBase} · Read ${result.generatedAt}`
  find('.diff-summary').textContent=result.isClean?'No changes compared with baseline':`${result.filesChanged} changed files · +${result.additions} −${result.deletions}${result.countsPartial||!result.complete?' (partial totals)':''}`
  if(result.warnings?.length)find('.diff-summary').textContent+=' · '+result.warnings.map(w=>w.message).join(' ')
  const files=find('.diff-files');files.replaceChildren()
  if(!result.files.some(f=>f.path===selection))selection=result.files[0]?.path||null
  for(const file of result.files){const button=document.createElement('button');button.type='button';button.dataset.path=file.path;button.textContent=(file.oldPath?file.oldPath+' → ':'')+file.path+' · '+file.status;button.onclick=()=>{selection=file.path;showFile()};files.append(button)}
  showFile()
 }
 async function refresh(){
  if(!active||!runID)return
  const request=++generation,id=runID
  find('.diff-status').textContent='Loading changes…';find('.diff-error').hidden=true
  container.setAttribute('aria-busy','true')
  if(result)find('.diff-status').textContent='Loading changes… Previous result is stale.'
  try{
   if(!api.gitDiff)throw Error('Update and restart Desktop to inspect changes.')
   const data=await api.gitDiff(id)
   if(request!==generation||id!==runID||!active)return
   if(data.runId!==id)throw Error('The agent returned another execution. Refresh to retry.')
   result=data;render();find('.diff-status').textContent='Changes loaded.'
  }catch(err){
   if(request!==generation||id!==runID||!active)return
   clear();find('.diff-error').textContent=err.message||String(err);find('.diff-error').hidden=false;find('.diff-status').textContent='Changes unavailable. Refresh to retry.'
  }finally{if(request===generation)container.setAttribute('aria-busy','false')}
 }
 function renderViews(focusConsole=false){
  terminal.hidden=!consoleVisible;container.hidden=!active;divider.hidden=!(consoleVisible&&active)
  panel.classList.toggle('split',consoleVisible&&active)
  consoleButton.setAttribute('aria-pressed',String(consoleVisible));changesButton.setAttribute('aria-pressed',String(active))
  if(consoleVisible)requestAnimationFrame(()=>onConsole(focusConsole))
 }
 function setSplit(percent){
  const value=Math.max(25,Math.min(75,Math.round(percent)))
  terminal.style.width=value+'%'
  divider.setAttribute('aria-valuenow',String(value))
  requestAnimationFrame(()=>onConsole(false))
 }
 function toggleConsole(){if(consoleVisible&&!active)return;consoleVisible=!consoleVisible;renderViews(consoleVisible)}
 function toggleChanges(){
  if(active&&!consoleVisible)return
  active=!active;generation++;container.setAttribute('aria-busy','false');renderViews()
  if(active)refresh()
 }
 consoleButton.onclick=toggleConsole;changesButton.onclick=toggleChanges;find('.diff-refresh').onclick=refresh
 find('.diff-render-toggle').onclick=()=>{rendered=!rendered;showFile()}
 divider.onpointerdown=event=>{divider.setPointerCapture(event.pointerId);document.body.classList.add('resizing-execution')}
 divider.onpointermove=event=>{if(divider.hasPointerCapture(event.pointerId)){const rect=panel.getBoundingClientRect();setSplit((event.clientX-rect.left)/rect.width*100)}}
 divider.onpointerup=event=>{divider.releasePointerCapture(event.pointerId)}
 divider.onlostpointercapture=()=>document.body.classList.remove('resizing-execution')
 divider.onkeydown=event=>{if(event.key==='ArrowLeft'||event.key==='ArrowRight'){event.preventDefault();setSplit(Number(divider.getAttribute('aria-valuenow'))+(event.key==='ArrowRight'?5:-5))}}
 return {
  get active(){return active},
  get consoleVisible(){return consoleVisible},
  select(id){if(id===runID)return;runID=id;generation++;selection=null;rendered=true;clear();find('.diff-error').hidden=true;find('.diff-status').textContent='';consoleButton.disabled=!id;changesButton.disabled=!id;if(active){if(id)refresh();else{active=false;consoleVisible=true;renderViews()}}},
  disconnect(){generation++;clear();container.setAttribute('aria-busy','false');find('.diff-status').textContent='Changes unavailable. Reconnect and refresh.';if(active){find('.diff-error').textContent='Local agent disconnected. Reconnect and refresh.';find('.diff-error').hidden=false}}
 }
}
