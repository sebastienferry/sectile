import { ipcMessage } from './execution-fields.mjs'
import { runEngine } from './run-engine.mjs'
import { runStateOf, runStateLabel, runStateSvg } from '../../shared/runStates.ts'

const active=run=>['running','queued','preparing','pending','waiting'].includes(run.status)
const newest=(a,b)=>String(b.createdAt||'').localeCompare(String(a.createdAt||''))||b.id.localeCompare(a.id)

// Discovery and restored desktopRun records currently describe ticket workers,
// discussions, macros and consoles only. Policy evaluation launches workers,
// not dedicated orchestration sessions. Fail closed until an actual dedicated
// producer supplies an authoritative responsibility and stable profile identity.
export function isOrchestrationSession(_run){return false}

export function orchestrationGroups(projects,runs){
 const candidates=runs.filter(run=>active(run)&&isOrchestrationSession(run))
 return projects.filter(project=>!project.disconnected&&(project.orchestratorEnabled||candidates.some(run=>run.projectId===project.id)))
  .map(project=>({project,agents:candidates.filter(run=>run.projectId===project.id)}))
  .sort((a,b)=>a.project.name.localeCompare(b.project.name))
}

export function orchestrationProfileRuns(_projectId,_agentId,runs){
 return runs.filter(isOrchestrationSession).sort(newest)
}

export function orchestrationProfileState(runs,{pending=false,error=''}={}){
 runs=runs.filter(isOrchestrationSession)
 if(pending)return {id:'queued',label:'Evaluating backlog'}
 const live=runs.filter(active)
 const waiting=live.find(run=>runStateOf(run)==='waiting')
 const run=waiting||live[0]||runs[0]
 if(waiting)return {id:'waiting',label:'Waiting for you',run}
 if(run&&live.length)return {id:runStateOf(run),label:run.cancelRequested?'Stopping':runStateLabel(runStateOf(run)),run}
 if(run?.status==='failed')return {id:'failed',label:run.authenticationRequired?'Sign-in needed':'Failed',run}
 if(error)return {id:'failed',label:'Needs attention',run}
 return {id:'completed',label:'Ready',run}
}

// The server stops before reading the backlog when the project has no free
// slot, so an empty report then means "refused", not "nothing eligible".
export function orchestrationReportSummary(report){
 const launched=(report?.launched||[]).length,skipped=(report?.skipped||[]).length
 const counts=launched+' launched · '+skipped+' skipped'
 if(launched||skipped||report?.availableSlots!==0||!report?.maxConcurrent)return {counts}
 const active=Number.isInteger(report.activeRunsBefore)?report.activeRunsBefore:'?'
 return {counts,capacity:'Capacity reached: '+active+' / '+report.maxConcurrent+' runs already active on this project, so no task was evaluated. Finish or stop a run, or raise the limit in project settings.'}
}

const node=(tag,className,text)=>{const element=document.createElement(tag);if(className)element.className=className;if(text!==undefined)element.textContent=text;return element}
const button=(text,className,action)=>{const element=node('button',className,text);element.type='button';element.onclick=action;return element}
const badge=state=>{const element=node('span','orchestration-state',state.label);element.dataset.state=state.id;const icon=node('span','orchestration-state-icon');icon.innerHTML=runStateSvg(state.id,13);icon.setAttribute('aria-hidden','true');element.prepend(icon);return element}

export function createOrchestration({container,details,onConsole,onOpen=()=>true,onClose=()=>{},onSettings=()=>{},api,onChanged=()=>{}}){
 let previous='',lastInput,revision=0,selection=null
 const states=new Map()
 const stateFor=key=>{if(!states.has(key))states.set(key,{});return states.get(key)}
 const redraw=()=>{revision++;if(lastInput)view.render(lastInput)}
 const titleOf=run=>lastInput.taskTitles?.get(run.taskId)||run.skill||'Task agent'
 const keyOf=run=>lastInput.taskKeys?.get(run.taskId)||(/^[0-9a-f]{8}-[0-9a-f-]{27}$/i.test(run.taskKey||'')?'':run.taskKey)||'Task'
 async function choose(next){
  if(await onOpen()===false)return
  selection=next;redraw()
  details?.querySelector('h1')?.focus({preventScroll:true})
 }
 async function runAgent(project,agent,state){
  if(state.pending)return
  state.pending=true;state.error='';state.report=null;redraw()
  try{state.report=await api.runOrchestrationAgent(project.id,agent.id);onChanged()}
  catch(error){state.error=ipcMessage(error)}
  finally{state.pending=false;redraw()}
 }
 function row(label,meta,state,key,chooseRow){
  const element=button('','orchestration-agent',chooseRow);element.dataset.selection=key
  element.title=label+' · '+meta+' · '+state.label
  element.setAttribute('aria-pressed',String(key===selection?.key));element.setAttribute('aria-label',label+' · '+state.label)
  const copy=node('span','orchestration-row-copy');copy.append(node('strong','',label),node('span','orchestration-policy-file',meta))
  element.append(copy,badge(state));return element
 }
 function taskDetails(host,run){
  const list=node('dl','orchestration-facts')
  for(const [label,value] of [['Stage',lastInput.taskStages?.get(run.taskId)||'Stage unavailable'],['Task',keyOf(run)+' · '+titleOf(run)],['Engine',run.engineName||runEngine(run)||'Not reported']])list.append(node('dt','',label),node('dd','',value))
  host.append(list)
 }
 function sessions(host,runs){
  const section=node('section','orchestration-activity');section.append(node('h2','','Activity'))
  // Keep every live session; bound only the history of finished sessions.
  const shown=[...runs.filter(active),...runs.filter(run=>!active(run)).slice(0,8)]
  if(!shown.length)section.append(node('p','hint','No dedicated orchestration sessions yet.'))
  for(const run of shown){
   const item=node('div','orchestration-session');const copy=node('div','orchestration-row-copy')
   const created=new Date(run.createdAt)
   const submitted=Number.isNaN(created.getTime())?'':created.toLocaleString(undefined,{month:'short',day:'numeric',hour:'2-digit',minute:'2-digit'})
   copy.append(node('strong','',titleOf(run)),node('span','orchestration-session-meta',[keyOf(run),lastInput.taskStages?.get(run.taskId)||run.skill||'Stage unavailable',submitted].filter(Boolean).join(' · ')))
   item.append(copy,badge({id:runStateOf(run),label:runStateLabel(runStateOf(run))}))
   const consoleButton=button('Open console','secondary',()=>onConsole(run));consoleButton.disabled=lastInput.offline;item.append(consoleButton);section.append(item)
  }
  host.append(section)
 }
 async function loadPreview(project,state){
  const generation=(state.previewGeneration||0)+1;state.previewGeneration=generation;state.previewLoading=true;redraw()
  try{
   const result=await api.orchestration(project.id)
   if(generation!==state.previewGeneration)return
   state.preview=result.agents||[];state.previewError=''
  }catch(error){if(generation===state.previewGeneration)state.previewError=ipcMessage(error)}
  finally{if(generation===state.previewGeneration){state.previewLoading=false;redraw()}}
 }
 function overview(groups,runs){
  if(!details)return
  const focused=details.contains(document.activeElement)?document.activeElement?.dataset.focus:null
  const instructionsOpen=details.querySelector('.orchestration-instructions')?.open
  details.hidden=!selection
  if(!selection){details.replaceChildren();return}
  const group=groups.find(group=>group.project.id===selection.projectId)
  const agent=group?.project.orchestrationAgents?.find(agent=>agent.id===selection.agentId)
  const manual=selection.runId&&group?.agents.find(run=>run.id===selection.runId)
  if(!group||(!agent&&!manual)){view.close();return}
  const project=group.project,state=stateFor(selection.key)
  if(instructionsOpen!==undefined)state.instructionsOpen=instructionsOpen
  const history=agent?orchestrationProfileRuns(project.id,agent.id,runs):[manual]
  const status=agent?orchestrationProfileState(history,state):{id:runStateOf(manual),label:runStateLabel(runStateOf(manual)),run:manual}
  const latest=status.run
  details.replaceChildren()
  const top=node('div','orchestration-overview-top')
  top.append(node('span','hint',project.name+' / Orchestration'),button('Project settings','secondary',()=>onSettings(project)),button('Close overview','secondary',()=>view.close()))
  const heading=node('div','orchestration-overview-heading'),title=node('h1','',agent?.name||manual.engineName||runEngine(manual)||'Task agent');title.tabIndex=-1
  heading.append(title,badge(status));details.append(top,heading)
  if(agent)details.append(node('p','orchestration-instructions-file',agent.file))
  const notice=node('section','orchestration-notice');notice.setAttribute('aria-label','Agent status')
  const authenticatedFailure=latest?.status==='failed'&&latest.authenticationRequired
  if(authenticatedFailure){
   notice.classList.add('orchestration-notice-error')
   const provider=latest.provider==='claude'?'Claude':latest.provider==='codex'?'Codex':'AI provider'
   notice.append(node('h2','',provider+' sign-in required'),node('p','','The last run could not authenticate. Sign in again, then retry this agent. Your task and instructions are saved.'))
   const command=latest.provider==='claude'?'claude auth login':latest.provider==='codex'?'codex login':''
   if(command){const instruction=node('p','');instruction.append('In your terminal, run ',node('code','',command),'.');notice.append(instruction)}
   const signIn=button('Open sign-in terminal','secondary',async()=>{
    signIn.disabled=true
    try{await api.projectTerminal(project.id);state.signInOpened=true;state.error=''}catch(error){state.error=ipcMessage(error)}finally{redraw()}
   });signIn.dataset.focus='signin';signIn.disabled=lastInput.offline;notice.append(signIn)
   if(state.signInOpened)notice.append(node('p','hint','Complete sign-in in the terminal, then choose Retry agent.'))
  }else if(status.id==='waiting')notice.append(node('h2','','Waiting for you'),node('p','','Open the console to see and answer the agent’s request.'))
  else if(state.pending)notice.append(node('h2','','Evaluating backlog…'))
  else if(history.some(active))notice.append(node('h2','',status.label),node('p','','Working on '+titleOf(latest)))
  else if(latest?.status==='failed'){notice.classList.add('orchestration-notice-error');notice.append(node('h2','','The last run failed'),node('p','','Open its console for details, or retry this agent.'))}
  else notice.append(node('h2','','Ready to run'),node('p','','Evaluate the backlog using this agent’s saved instructions and start eligible work.'))
  const actions=node('div','orchestration-actions')
  if(agent){const runButton=button(latest?.status==='failed'?'Retry agent':'Run agent','primary',()=>runAgent(project,agent,state));runButton.classList.add('orchestration-run');runButton.dataset.focus='run';runButton.disabled=lastInput.offline||lastInput.stale||!!state.pending;actions.append(runButton)}
  if(latest){const consoleButton=button('Open console','secondary',()=>onConsole(latest));consoleButton.dataset.focus='console';consoleButton.disabled=lastInput.offline;actions.append(consoleButton)}
  if(history.some(active)){
   const stop=button('Stop agent','secondary',async()=>{
    state.stopping=true;state.error='';redraw()
    try{await Promise.all(history.filter(active).map(run=>api.stop(run.id)));onChanged()}catch(error){state.error=ipcMessage(error)}finally{state.stopping=false;redraw()}
   });stop.dataset.focus='stop';stop.disabled=lastInput.offline||!!state.stopping;actions.append(stop)
  }
  notice.append(actions);details.append(notice)
  if(lastInput.offline||lastInput.stale)details.append(node('p','hint',lastInput.offline?'Reconnect the local agent to run agents or open consoles.':'Capacity could not be refreshed. Reconnect before running an agent.'))
  if(state.error){const error=node('p','orchestration-error',state.error);error.setAttribute('role','alert');details.append(error)}
  if(state.report){const report=node('section','orchestration-report'),summary=orchestrationReportSummary(state.report);report.setAttribute('role','status');report.append(node('h2','','Last backlog evaluation'),node('p','',summary.counts))
   if(summary.capacity)report.append(node('p','orchestration-capacity-reached',summary.capacity))
   for(const item of state.report.skipped||[])report.append(node('p','',(item.key||'Task')+': '+item.reason));details.append(report)}
  if(latest)taskDetails(details,latest)
  sessions(details,history)
  if(agent){
   const preview=node('details','orchestration-instructions'),summary=node('summary','','Instructions · '+agent.file);preview.append(summary);preview.open=!!state.instructionsOpen
   const saved=state.preview?.find(item=>item.id===agent.id)
   if(saved?.guidelinesError)preview.append(node('p','orchestration-error',saved.guidelinesError))
   else if(saved)preview.append(node('pre','orchestration-guidelines',saved.guidelines))
   else preview.append(button(state.previewLoading?'Loading…':'Read local file','secondary',()=>loadPreview(project,state)))
   if(state.previewError)preview.append(node('p','orchestration-error',state.previewError))
   details.append(preview)
  }
  if(focused)details.querySelector('[data-focus="'+focused+'"]')?.focus({preventScroll:true})
 }
 const view={close(){if(!selection)return;selection=null;onClose();redraw()},render(input){
  lastInput=input
  const {projects,runs,taskStages=new Map(),taskTitles=new Map(),taskKeys=new Map(),stale=false,offline=false}=input
  const groups=orchestrationGroups(projects,runs)
  const stamp=JSON.stringify([groups,runs,stale,offline,revision,[...taskStages],[...taskTitles],[...taskKeys]])
  if(stamp===previous)return
  // The operational pane stays live while navigation above a hovered/focused
  // task is held. Do not consume the stamp until navigation can catch up.
  if(input.holdNavigation){overview(groups,runs);return}
  previous=stamp
  const scrollTop=container.scrollTop,focused=container.contains(document.activeElement)?document.activeElement?.dataset.selection:null
  container.hidden=!groups.length;container.replaceChildren()
  if(!groups.length){overview(groups,runs);return}
  container.append(node('h2','section','ORCHESTRATION'))
  if(offline||stale){const scope=node('p','orchestration-scope',offline?'Local agent offline':'Capacity unavailable · showing last known usage');scope.setAttribute('role','status');container.append(scope)}
  for(const {project,agents} of groups){
   const group=node('section','orchestration-project');group.setAttribute('aria-label','Orchestration for '+project.name)
   const heading=node('div','orchestration-heading');heading.append(node('strong','',project.name))
   const capacity=node('span','orchestration-capacity'),known=Number.isInteger(project.orchestratorActiveRuns)&&project.orchestratorActiveRuns>=0
   capacity.textContent=project.orchestratorEnabled?(known?project.orchestratorActiveRuns:'?')+' / '+(project.orchestratorMaxConcurrent||'?')+' in use':project.orchestratorMaxConcurrent?'Not enabled':'Capacity unavailable'
   capacity.title='Runs the orchestrator started on this project, queued ones and those on other workstations included; sessions started by hand take no slot'+(stale?' (last known)':'');heading.append(capacity);group.append(heading)
   const definitions=project.orchestratorEnabled?project.orchestrationAgents||[]:[]
   if(project.orchestratorEnabled&&!definitions.length)group.append(node('p','orchestration-empty','Add agents in project settings'))
   for(const agent of definitions){
    const key=project.id+':'+agent.id,state=stateFor(key),history=orchestrationProfileRuns(project.id,agent.id,runs)
    const status=orchestrationProfileState(history,state)
    const element=row(agent.name,agent.file,status,key,()=>choose({key,projectId:project.id,agentId:agent.id}))
    element.classList.add('orchestration-policy');element.dataset.agentId=agent.id;group.append(element)
   }
   // Only eligible dedicated sessions can outlive their policy definition.
   for(const run of agents.filter(run=>!definitions.some(agent=>orchestrationProfileRuns(project.id,agent.id,[run]).length))){
    const key='run:'+run.id,state={id:runStateOf(run),label:run.cancelRequested?'Stopping':runStateLabel(runStateOf(run))}
    const element=row(run.engineName||runEngine(run)||'Task agent',keyOf(run)+' · '+titleOf(run),state,key,()=>choose({key,projectId:project.id,runId:run.id}));element.dataset.runId=run.id;element.disabled=offline;group.append(element)
   }
   container.append(group)
  }
  overview(groups,runs);container.scrollTop=scrollTop
  if(focused)for(const element of container.querySelectorAll('[data-selection]'))if(element.dataset.selection===focused)element.focus({preventScroll:true})
 }}
 return view
}
