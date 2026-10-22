import { runEngine } from './run-engine.mjs'
import { runStateOf, runStateLabel, runStateSvg } from '../../shared/runStates.ts'

const active=run=>['running','queued','preparing','pending','waiting'].includes(run.status)

// Include manual task agents because they consume the same project capacity.
// Free discussions and terminal-only consoles are outside backlog orchestration.
export function orchestrationGroups(projects,runs){
 const candidates=runs.filter(run=>active(run)&&run.taskId&&run.kind!=='console'&&run.skill!=='discuss'&&!run.macroKey)
 return projects.filter(project=>!project.disconnected&&(project.path||project.configured||project.orchestratorEnabled||candidates.some(run=>run.projectId===project.id)))
  .map(project=>({project,agents:candidates.filter(run=>run.projectId===project.id)}))
  .sort((a,b)=>a.project.name.localeCompare(b.project.name))
}

export function createOrchestration({container,onConsole,api,onChanged=()=>{}}){
 let previous='',lastInput,revision=0
 const forms=new Map()
 const redraw=()=>{revision++;if(lastInput)view.render(lastInput)}
 container.addEventListener('focusout',()=>queueMicrotask(()=>{if(lastInput)view.render(lastInput)}))
 const settingsOf=result=>({orchestratorEnabled:result.orchestratorEnabled===true,orchestratorMaxConcurrent:result.orchestratorMaxConcurrent||1,orchestratorPrompt:result.orchestratorPrompt||''})
 async function setup(project){
  let state=forms.get(project.id)
  if(state?.open){state.open=false;redraw();return}
  if(state?.dirty){state.open=true;redraw();return}
  state={open:true,loading:true,dirty:false,error:'',fields:null,report:state?.report};forms.set(project.id,state);redraw()
  try{state.fields=settingsOf(await api.orchestration(project.id))}catch(error){state.error=error.message||String(error)}
  finally{state.loading=false;redraw()}
 }
 async function save(project,state){
  if(state.pending)return
  state.pending=true;state.error='';redraw()
  try{state.fields=settingsOf(await api.saveOrchestration(project.id,state.fields));state.dirty=false;Object.assign(project,state.fields);onChanged()}
  catch(error){state.error=error.message||String(error)}
  finally{state.pending=false;redraw()}
 }
 async function evaluate(project,state){
  if(state.pending)return
  state.pending=true;state.error='';state.report=null;redraw()
  try{state.report=await api.evaluateBacklog(project.id);onChanged()}
  catch(error){state.error=error.message||String(error)}
  finally{state.pending=false;redraw()}
 }
 const node=(tag,className,text)=>{const element=document.createElement(tag);if(className)element.className=className;if(text!==undefined)element.textContent=text;return element}
 const view={render(input){
  lastInput=input
  const {projects,runs,taskStages=new Map(),taskTitles=new Map(),selected,stale=false,offline=false}=input
  const groups=orchestrationGroups(projects,runs)
  const stamp=JSON.stringify([groups,selected,stale,offline,revision,[...taskStages],[...taskTitles]])
  if(stamp===previous)return
  if(!offline&&container.contains(document.activeElement)&&document.activeElement.matches('input,textarea')&&!forms.get(document.activeElement.closest('form')?.dataset.projectId)?.pending)return
  previous=stamp
  const scrollTop=container.scrollTop
  const focused=container.contains(document.activeElement)?document.activeElement.dataset.runId:null
  container.replaceChildren(node('h2','section','ORCHESTRATION'))
  const scope=node('p','orchestration-scope',offline?'Local agent offline':stale?'Project capacity unavailable · showing last known usage':'Agents on this workstation · project-wide capacity')
  scope.setAttribute('role','status');container.append(scope)
  if(!groups.length){container.append(node('p','orchestration-empty','No enabled orchestrators or active task agents'));return}
  for(const {project,agents} of groups){
   const group=node('section','orchestration-project');group.setAttribute('aria-label','Orchestration for '+project.name)
   const heading=node('div','orchestration-heading');heading.append(node('strong','',project.name))
   const capacity=node('span','orchestration-capacity')
   const known=Number.isInteger(project.orchestratorActiveRuns)&&project.orchestratorActiveRuns>=0
   capacity.textContent=project.orchestratorEnabled?(known?project.orchestratorActiveRuns:'?')+' / '+(project.orchestratorMaxConcurrent||'?')+' slots':project.orchestratorMaxConcurrent?'Not enabled':'Capacity unavailable'
   capacity.title='Project-wide orchestration capacity'+(stale?' (last known)':'')
   heading.append(capacity);group.append(heading)
   const state=forms.get(project.id)
   const setupButton=node('button','orchestration-setup',state?.open?'Close setup':'Set up')
   setupButton.type='button';setupButton.setAttribute('aria-label','Set up orchestration for '+project.name)
   setupButton.setAttribute('aria-expanded',String(!!state?.open));setupButton.disabled=offline||!!state?.pending
   setupButton.onclick=()=>setup(project);group.append(setupButton)
   if(state?.open){
    const form=node('form','orchestration-form');form.dataset.projectId=project.id;form.setAttribute('aria-label','Orchestration settings for '+project.name)
    if(state.loading)form.append(node('p','orchestration-empty','Loading settings…'))
    if(state.fields){
     const fields=state.fields
     const enabledLabel=node('label','orchestration-enabled','Enable orchestration')
     const enabled=node('input');enabled.type='checkbox';enabled.checked=fields.orchestratorEnabled;enabled.name='orchestratorEnabled';enabledLabel.prepend(enabled)
     const limitLabel=node('label','','Concurrency limit')
     const limit=node('input');limit.type='number';limit.min='1';limit.max='10';limit.required=true;limit.value=fields.orchestratorMaxConcurrent;limit.name='orchestratorMaxConcurrent';limitLabel.append(limit)
     const promptLabel=node('label','','Eligibility guidelines')
     const prompt=node('textarea');prompt.rows=3;prompt.maxLength=8000;prompt.value=fields.orchestratorPrompt;prompt.name='orchestratorPrompt';prompt.placeholder='Which tickets may run autonomously?';promptLabel.append(prompt)
     const actions=node('div','orchestration-actions')
     const saveButton=node('button','','Save settings');saveButton.type='submit'
     const evaluateButton=node('button','','Evaluate backlog');evaluateButton.type='button'
     const update=()=>{
      saveButton.disabled=offline||state.pending||!state.dirty||!limit.validity.valid
      evaluateButton.disabled=offline||state.pending||state.dirty||!fields.orchestratorEnabled
     }
     for(const field of [enabled,limit,prompt]){
      field.disabled=offline||state.pending
      field.oninput=()=>{fields[field.name]=field===enabled?enabled.checked:field===limit?Number(limit.value):prompt.value;state.dirty=true;state.report=null;update()}
     }
     update();actions.append(saveButton,evaluateButton)
     form.append(enabledLabel,limitLabel,promptLabel,actions,node('p','orchestration-empty',state.pending?'Working…':'Runs on demand. Save changes before evaluating; disabling stops future launches, not current agents.'))
     form.onsubmit=event=>{event.preventDefault();if(!saveButton.disabled)save(project,state)}
     evaluateButton.onclick=()=>{if(!evaluateButton.disabled)evaluate(project,state)}
    }
    if(state.error){const error=node('p','orchestration-error',state.error);error.setAttribute('role','alert');form.append(error)}
    group.append(form)
   }
   if(state?.report){
    const report=node('div','orchestration-report');report.setAttribute('role','status')
    const result=state.report,launched=result.launched||[],skipped=result.skipped||[]
    report.append(node('p','',launched.length+' launched · '+skipped.length+' skipped · '+result.activeRunsAfter+' / '+result.maxConcurrent+' slots used'))
    for(const item of skipped)report.append(node('p','',(item.key||item.id)+': '+item.reason))
    group.append(report)
   }
   if(project.orchestratorEnabled){
    const meter=node('progress','orchestration-meter');meter.max=project.orchestratorMaxConcurrent||1
    if(known&&!stale&&!offline)meter.value=project.orchestratorActiveRuns
    meter.setAttribute('aria-label','Project capacity for '+project.name);group.append(meter)
   }
   if(!agents.length)group.append(node('p','orchestration-empty',offline?'Reconnect to see local agents':'No active agents on this workstation'))
   for(const run of agents){
    const engine=run.engineName||runEngine(run)||'Agent'
    const button=node('button','orchestration-agent');button.type='button';button.dataset.runId=run.id
    button.setAttribute('aria-label','Open console for '+(run.taskKey||run.taskId)+' · '+engine)
    button.setAttribute('aria-pressed',String(run.id===selected));button.disabled=offline
    const title=node('span','orchestration-agent-title');title.append(node('strong','',run.taskKey||run.taskId),node('span','',engine))
    title.title=runEngine(run)
    const description=node('span','orchestration-task',taskTitles.get(run.taskId)||run.skill||'Task agent')
    const stage=taskStages.get(run.taskId)
    const meta=node('span','orchestration-agent-meta');meta.append(node('span','orchestration-stage',stage?'Stage: '+stage:'Stage unavailable'))
    const state=runStateOf(run),stateText=run.cancelRequested?(run.status==='queued'?'Canceling':'Stopping'):runStateLabel(state)
    const badge=node('span','orchestration-state',stateText);badge.dataset.state=state
    const icon=node('span','orchestration-state-icon');icon.innerHTML=runStateSvg(state,12);icon.setAttribute('aria-hidden','true');badge.prepend(icon);meta.append(badge)
    button.append(title,description,meta,node('span','orchestration-console','Open console →'))
    button.onclick=()=>onConsole(run);group.append(button)
   }
   container.append(group)
  }
  container.scrollTop=scrollTop
  if(focused)for(const button of container.querySelectorAll('button'))if(button.dataset.runId===focused)button.focus({preventScroll:true})
 }}
 return view
}
