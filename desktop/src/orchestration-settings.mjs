import { ipcMessage } from './execution-fields.mjs'
// Named agent policies stay in project settings; the sidebar launches saved agents.
export function orchestrationSettings({container,project,api,onChanged=()=>{}}){
 const node=(tag,text)=>{const element=document.createElement(tag);if(text!==undefined)element.textContent=text;return element}
 const form=node('form');form.className='orchestration-form';form.setAttribute('aria-label','Orchestration settings for '+project.name)
 container.append(node('p','Add agents with different local Markdown files. Each Run agent action reads its file again. All agents share the project concurrency limit.'),form)
 const status=node('p','Loading settings…');status.setAttribute('role','status');form.append(status)
 let fields,agents=[],dirty=false,pending=false,views=[]
 const outcomes=new Map()
 const enabled=node('input');enabled.type='checkbox'
 const limit=node('input');limit.type='number';limit.min='1';limit.max='10';limit.required=true
 const label=(parent,text,input)=>{const row=node('label',text);row.append(input);parent.append(row);return row}
 label(form,'Enable orchestration',enabled).className='orchestration-enabled'
 label(form,'Concurrency limit',limit)
 form.append(node('p','Guidelines paths are relative to the project folder, for example AGENTS.md or docs/documentation-agent.md. Enabled agents need existing Markdown files.'))
 const cards=node('div');cards.className='orchestration-agent-settings-list';form.append(cards)
 const add=node('button','Add agent');add.type='button'
 const save=node('button','Save orchestration settings');save.type='submit'
 const actions=node('div');actions.className='orchestration-actions';actions.append(add,save);form.append(actions)
 const error=node('p');error.className='orchestration-error';error.setAttribute('role','alert');form.append(error)
 function update(){
  enabled.disabled=limit.disabled=!fields||pending
  const names=agents.map(agent=>agent.name.trim().toLowerCase())
  const valid=form.checkValidity()&&new Set(names).size===names.length&&(!enabled.checked||agents.length>0)
  save.disabled=!fields||pending||!dirty||!valid
  add.disabled=!fields||pending||agents.length>=(fields.maxAgents||20)
  for(const view of views){
   view.name.disabled=view.file.disabled=view.remove.disabled=!fields||pending
   view.run.disabled=!fields||pending||dirty||!enabled.checked||!!view.agent.guidelinesError
   view.reload.disabled=!fields||pending||dirty
  }
 }
 function showOutcome(agent,report){
  report.replaceChildren();const result=outcomes.get(agent.id)
  report.className='orchestration-report'
  if(!result)return
  report.setAttribute('role','status')
  report.append(node('p',(result.launched||[]).length+' launched · '+(result.skipped||[]).length+' skipped · '+result.activeRunsAfter+' / '+result.maxConcurrent+' slots used'))
  for(const item of result.skipped||[])report.append(node('p',(item.key||item.id)+': '+item.reason))
 }
 function renderAgents(){
  cards.replaceChildren();views=[]
  if(!agents.length)cards.append(node('p','No agents configured. Add an agent before enabling orchestration.'))
  for(const agent of agents){
   const card=node('fieldset');card.className='orchestration-agent-settings';card.dataset.agentId=agent.id
   const legend=node('legend',agent.name);card.append(legend)
   const name=node('input');name.type='text';name.required=true;name.maxLength=80;name.value=agent.name
   const file=node('input');file.type='text';file.required=true;file.value=agent.file;file.placeholder='AGENTS.md'
   label(card,'Agent name',name);label(card,'Guidelines file',file)
   const warning=node('p',agent.guidelinesError||'');warning.className='orchestration-error'
   const preview=node('pre',agent.guidelines||'');preview.className='orchestration-guidelines';preview.setAttribute('aria-label','Guidelines preview')
   const run=node('button','Run agent');run.type='button'
   const reload=node('button','Reload file');reload.type='button'
   const remove=node('button','Remove agent');remove.type='button'
   const buttons=node('div');buttons.className='orchestration-actions';buttons.append(run,reload,remove)
   const report=node('div');showOutcome(agent,report)
   card.append(warning,preview,buttons,report);cards.append(card)
   views.push({agent,name,file,run,reload,remove})
   name.oninput=()=>{agent.name=name.value;legend.textContent=name.value||'Unnamed agent';dirty=true;outcomes.delete(agent.id);update()}
   file.oninput=()=>{agent.file=file.value;preview.textContent='';warning.textContent='Save changes to preview this file.';dirty=true;outcomes.delete(agent.id);update()}
   remove.onclick=()=>{agents=agents.filter(item=>item.id!==agent.id);outcomes.delete(agent.id);dirty=true;renderAgents();update()}
   reload.onclick=load
   run.onclick=async()=>{
    if(run.disabled)return
    pending=true;error.textContent='';update()
    try{outcomes.set(agent.id,await api.runOrchestrationAgent(project.id,agent.id));showOutcome(agent,report);onChanged()}
    catch(err){error.textContent=agent.name+': '+ipcMessage(err)}
    finally{pending=false;update()}
   }
  }
 }
 async function load(){
  pending=true;update();error.textContent=''
  try{
   fields=await api.orchestration(project.id)
   if(!Array.isArray(fields.agents))throw Error('Update and restart the local agent to configure multiple agents.')
   agents=fields.agents.map(agent=>({...agent}));enabled.checked=fields.orchestratorEnabled===true;limit.value=fields.orchestratorMaxConcurrent||1;dirty=false
   renderAgents();status.textContent='Agents run on demand. Disabling orchestration stops future launches; active tasks continue.'
  }catch(err){error.textContent=ipcMessage(err);status.textContent=''}
  finally{pending=false;update()}
 }
 add.onclick=()=>{
  let count=agents.length+1
  while(agents.some(agent=>agent.name.toLowerCase()==='agent '+count))count++
  agents.push({id:crypto.randomUUID(),name:'Agent '+count,file:'AGENTS.md',guidelines:''})
  dirty=true;renderAgents();update();views.at(-1).name.focus()
 }
 for(const input of [enabled,limit])input.oninput=()=>{dirty=true;update()}
 form.onsubmit=async event=>{
  event.preventDefault();if(save.disabled)return
  pending=true;update();error.textContent=''
  try{
   await api.saveOrchestration(project.id,{orchestratorEnabled:enabled.checked,orchestratorMaxConcurrent:Number(limit.value),agents:agents.map(({id,name,file})=>({id,name:name.trim(),file:file.trim()}))})
   onChanged();await load()
  }catch(err){error.textContent=ipcMessage(err)}
  finally{pending=false;update()}
 }
 update();load()
 return {reload:load}
}
