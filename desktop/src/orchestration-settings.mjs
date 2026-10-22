// Project configuration stays in Settings; the sidebar only launches and monitors.
export function orchestrationSettings({container,project,api,onChanged=()=>{}}){
 const node=(tag,text)=>{const element=document.createElement(tag);if(text!==undefined)element.textContent=text;return element}
 const form=node('form');form.className='orchestration-form';form.setAttribute('aria-label','Orchestration settings for '+project.name)
 const intro=node('p','Guidelines come from a local Markdown file in this project. AGENTS.md is used by default. The file is read again whenever you evaluate the backlog.')
 container.append(intro,form)
 const status=node('p');status.setAttribute('role','status');form.append(status);status.textContent='Loading settings…'
 let fields,dirty=false,pending=false
 const enabled=node('input');enabled.type='checkbox';enabled.name='orchestratorEnabled'
 const limit=node('input');limit.type='number';limit.min='1';limit.max='10';limit.required=true;limit.name='orchestratorMaxConcurrent'
 const file=node('input');file.type='text';file.required=true;file.name='orchestrationFile';file.placeholder='AGENTS.md'
 const preview=node('pre');preview.className='orchestration-guidelines';preview.setAttribute('aria-label','Guidelines preview')
 const warning=node('p');warning.className='orchestration-error'
 const error=node('p');error.className='orchestration-error';error.setAttribute('role','alert')
 const save=node('button','Save orchestration settings');save.type='submit'
 const evaluate=node('button','Evaluate backlog');evaluate.type='button'
 const reload=node('button','Reload file');reload.type='button'
 const label=(text,input)=>{const row=node('label',text);row.append(input);form.append(row);return row}
 label('Enable orchestration',enabled).className='orchestration-enabled'
 label('Concurrency limit',limit)
 label('Guidelines file',file)
 form.append(node('p','Path relative to the project folder, for example AGENTS.md or docs/orchestration.md. The file must exist and contain Markdown text before orchestration can be enabled.'),warning,preview)
 const actions=node('div');actions.className='orchestration-actions';actions.append(save,evaluate,reload);form.append(actions,error)
 function update(){
  for(const input of [enabled,limit,file])input.disabled=!fields||pending
  save.disabled=!fields||pending||!dirty||!limit.validity.valid||!file.validity.valid
  evaluate.disabled=!fields||pending||dirty||!enabled.checked||!!fields.guidelinesError
  reload.disabled=!fields||pending||dirty
 }
 function show(result){
  fields=result;enabled.checked=result.orchestratorEnabled===true;limit.value=result.orchestratorMaxConcurrent||1;file.value=result.orchestrationFile||'AGENTS.md'
  preview.textContent=result.guidelines||'';warning.textContent=result.guidelinesError||'';dirty=false
 }
 async function load(){
  pending=true;update();error.textContent=''
  try{show(await api.orchestration(project.id));status.textContent='Runs on demand. Disabling orchestration stops future launches; active agents continue.'}
  catch(err){error.textContent=err.message||String(err);status.textContent=''}
  finally{pending=false;update()}
 }
 for(const input of [enabled,limit,file])input.oninput=()=>{dirty=true;update()}
 form.onsubmit=async event=>{
  event.preventDefault();if(save.disabled)return
  pending=true;update();error.textContent=''
  try{
   const result=await api.saveOrchestration(project.id,{orchestratorEnabled:enabled.checked,orchestratorMaxConcurrent:Number(limit.value),orchestrationFile:file.value.trim()})
   Object.assign(project,{orchestratorEnabled:result.orchestratorEnabled,orchestratorMaxConcurrent:result.orchestratorMaxConcurrent})
   onChanged();await load()
  }catch(err){error.textContent=err.message||String(err)}
  finally{pending=false;update()}
 }
 evaluate.onclick=async()=>{
  if(evaluate.disabled)return
  pending=true;update();error.textContent=''
  try{
   const result=await api.evaluateBacklog(project.id)
   status.textContent=(result.launched||[]).length+' launched · '+(result.skipped||[]).length+' skipped · '+result.activeRunsAfter+' / '+result.maxConcurrent+' slots used'
   for(const item of result.skipped||[])status.append(node('div',(item.key||item.id)+': '+item.reason))
   onChanged()
  }catch(err){error.textContent=err.message||String(err)}
  finally{pending=false;update()}
 }
 reload.onclick=load
 update();load()
 return {reload:load}
}
