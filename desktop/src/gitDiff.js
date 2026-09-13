// Results belong only to the selected execution and latest explicit request.
export function createGitDiff({api,container,terminal,consoleButton,changesButton,onConsole}){
 let runID=null,generation=0,active=false,result=null,selection=null
 container.innerHTML='<div class="changes-toolbar"><button type="button" class="diff-refresh">Refresh</button><span class="diff-status" role="status" aria-live="polite"></span></div><p class="diff-error" role="alert" hidden></p><p class="diff-context"></p><p class="diff-summary"></p><div class="diff-body"><nav class="diff-files" aria-label="Changed files"></nav><div class="diff-detail"><p class="diff-file-info"></p><pre class="diff-patch" tabindex="0" aria-label="Selected file diff"></pre></div></div>'
 const find=s=>container.querySelector(s)
 function clear(){result=null;find('.diff-context').textContent='';find('.diff-summary').textContent='';find('.diff-files').replaceChildren();find('.diff-patch').replaceChildren();find('.diff-file-info').textContent=''}
 function showFile(){
  const file=result?.files.find(f=>f.path===selection),patch=find('.diff-patch');patch.replaceChildren()
  for(const button of find('.diff-files').children)button.setAttribute('aria-pressed',String(button.dataset.path===selection))
  if(!file)return
  find('.diff-file-info').textContent=[file.oldPath?file.oldPath+' → '+file.path:file.path,file.status,file.kind,file.additions==null?'Text counts unavailable':`+${file.additions} −${file.deletions}`,file.omittedReason].filter(Boolean).join(' · ')
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
 function view(changes){
  active=changes;generation++;container.hidden=!active;terminal.hidden=active
  consoleButton.setAttribute('aria-pressed',String(!active));changesButton.setAttribute('aria-pressed',String(active))
  if(active)refresh();else onConsole()
 }
 consoleButton.onclick=()=>view(false);changesButton.onclick=()=>view(true);find('.diff-refresh').onclick=refresh
 return {
  get active(){return active},
  select(id){if(id===runID)return;runID=id;generation++;selection=null;clear();find('.diff-error').hidden=true;find('.diff-status').textContent='';consoleButton.disabled=!id;changesButton.disabled=!id;if(active){if(id)refresh();else view(false)}},
  disconnect(){generation++;clear();if(active){find('.diff-error').textContent='Local agent disconnected. Reconnect and refresh.';find('.diff-error').hidden=false}}
 }
}
