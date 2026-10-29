import { ipcMessage } from './execution-fields.mjs'
import { runEngine } from './run-engine.mjs'
import { runStateOf, runStateLabel, runStateSvg } from '../../shared/runStates.ts'

const active=run=>['running','queued','preparing','pending','waiting'].includes(run.status)
const newest=(a,b)=>String(b.createdAt||'').localeCompare(String(a.createdAt||''))||b.id.localeCompare(a.id)

// A project's orchestrator session (ADR 0064) is the only dedicated session:
// the agent marks it with its scope and the profile it runs. Ticket workers,
// the chains it launched included, never count as one.
export function isOrchestrationSession(run){return run?.scope==='orchestrator'}

export function orchestrationGroups(projects,runs){
 const candidates=runs.filter(run=>active(run)&&isOrchestrationSession(run))
 return projects.filter(project=>!project.disconnected&&(project.orchestratorEnabled||candidates.some(run=>run.projectId===project.id)))
  .map(project=>({project,agents:candidates.filter(run=>run.projectId===project.id)}))
  .sort((a,b)=>a.project.name.localeCompare(b.project.name))
}

export function orchestrationProfileRuns(projectId,agentId,runs){
 return runs.filter(run=>isOrchestrationSession(run)&&run.projectId===projectId&&(run.profileId||'')===(agentId||'')).sort(newest)
}

// started says this profile is the one orchestrating its project: between two
// sessions it waits for an orchestrated chain to end, which wakes the next.
export function orchestrationProfileState(runs,{pending=false,error='',started=false}={}){
 runs=runs.filter(isOrchestrationSession)
 if(pending)return {id:'queued',label:'Starting'}
 const live=runs.filter(active)
 const waiting=live.find(run=>runStateOf(run)==='waiting')
 const run=waiting||live[0]||runs[0]
 if(waiting)return {id:'waiting',label:'Waiting for you',run}
 if(run&&live.length)return {id:runStateOf(run),label:run.cancelRequested?'Stopping':'Reviewing backlog',run}
 if(run?.status==='failed')return {id:'failed',label:run.authenticationRequired?'Sign-in needed':'Failed',run}
 if(error)return {id:'failed',label:'Needs attention',run}
 if(started)return {id:'queued',label:'Started',run}
 return {id:'completed',label:'Ready',run}
}

// One line per ticket the orchestrator decided on. A deferral of a ticket edited
// since is reviewed again by the next session, and says so.
export function orchestrationDecisionLine(decision){
 const what=decision?.decision==='launched'?'Launched':'Deferred'
 const ticket=[decision?.taskKey,decision?.title].filter(Boolean).join(' · ')||'Task'
 const changed=decision?.decision!=='launched'&&decision?.unchangedSinceDecision===false
 return {ticket,what,reason:decision?.reason||'',changed}
}

const node=(tag,className,text)=>{const element=document.createElement(tag);if(className)element.className=className;if(text!==undefined)element.textContent=text;return element}
const button=(text,className,action)=>{const element=node('button',className,text);element.type='button';element.onclick=action;return element}
const badge=state=>{const element=node('span','orchestration-state',state.label);element.dataset.state=state.id;const icon=node('span','orchestration-state-icon');icon.innerHTML=runStateSvg(state.id,13);icon.setAttribute('aria-hidden','true');element.prepend(icon);return element}

export function createOrchestration({container,details,onConsole,onOpen=()=>true,onClose=()=>{},onSettings=()=>{},api,onChanged=()=>{}}){
 let previous='',lastInput,revision=0,selection=null
 const states=new Map()
 const stateFor=key=>{if(!states.has(key))states.set(key,{});return states.get(key)}
 // The server's orchestration state per project: which agent is started, its
 // decisions. Read every 15 seconds while shown, and after a start or a stop.
 // An agent or server that predates ADR 0064 has none: nothing is started.
 const servers=new Map()
 const serverOf=projectId=>{if(!servers.has(projectId))servers.set(projectId,{readAt:0});return servers.get(projectId)}
 const redraw=()=>{revision++;if(lastInput)view.render(lastInput)}
 async function loadServer(projectId,force=false){
  const server=serverOf(projectId)
  if(server.loading||(!force&&Date.now()-server.readAt<15000))return
  server.loading=true;server.readAt=Date.now()
  try{const result=await api.orchestration(projectId);server.state=result?.state||null;server.error=''}
  catch(error){server.error=ipcMessage(error)}
  finally{server.loading=false;redraw()}
 }
 const armedOf=projectId=>serverOf(projectId).state?.armed||null
 const startedHere=(projectId,agentId)=>{const armed=armedOf(projectId);return !!armed&&(armed.profileId||'')===(agentId||'')}
 const titleOf=run=>isOrchestrationSession(run)?'Backlog review':lastInput.taskTitles?.get(run.taskId)||run.skill||'Task agent'
 const keyOf=run=>isOrchestrationSession(run)?'':lastInput.taskKeys?.get(run.taskId)||(/^[0-9a-f]{8}-[0-9a-f-]{27}$/i.test(run.taskKey||'')?'':run.taskKey)||'Task'
 async function choose(next){
  if(await onOpen()===false)return
  selection=next;redraw()
  details?.querySelector('h1')?.focus({preventScroll:true})
 }
 async function startAgent(project,agent,state){
  if(state.pending)return
  state.pending=true;state.error='';redraw()
  try{await api.runOrchestrationAgent(project.id,agent.id);onChanged()}
  catch(error){state.error=ipcMessage(error)}
  finally{state.pending=false;await loadServer(project.id,true)}
 }
 // Stopping disarms the project and stops its session on this workstation;
 // the chains already launched finish on their own.
 async function stopAgent(project,history,state){
  state.stopping=true;state.error='';redraw()
  try{await api.stopOrchestration(project.id);await Promise.all(history.filter(active).map(run=>api.stop(run.id)));onChanged()}
  catch(error){state.error=ipcMessage(error)}
  finally{state.stopping=false;await loadServer(project.id,true)}
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
  const facts=isOrchestrationSession(run)?[['Engine',run.engineName||runEngine(run)||'Not reported']]:[['Stage',lastInput.taskStages?.get(run.taskId)||'Stage unavailable'],['Task',keyOf(run)+' · '+titleOf(run)],['Engine',run.engineName||runEngine(run)||'Not reported']]
  for(const [label,value] of facts)list.append(node('dt','',label),node('dd','',value))
  host.append(list)
 }
 function decisions(host,projectId){
  const section=node('section','orchestration-decisions');section.append(node('h2','','Decisions'))
  const items=serverOf(projectId).state?.decisions||[]
  if(!items.length)section.append(node('p','hint','No decision yet. Each backlog review explains why it launches or defers every ticket it considers.'))
  for(const decision of items.slice(0,20)){
   const line=orchestrationDecisionLine(decision),item=node('div','orchestration-decision');item.dataset.decision=decision.decision
   const decided=new Date(decision.decidedAt),when=Number.isNaN(decided.getTime())?'':decided.toLocaleString(undefined,{month:'short',day:'numeric',hour:'2-digit',minute:'2-digit'})
   item.append(node('strong','',line.ticket),node('span','orchestration-session-meta',[line.what,when].filter(Boolean).join(' · ')))
   if(line.reason)item.append(node('p','',line.reason))
   if(line.changed)item.append(node('p','hint','Changed since: the next review considers it again.'))
   section.append(item)
  }
  host.append(section)
 }
 function sessions(host,runs){
  const section=node('section','orchestration-activity');section.append(node('h2','','Activity'))
  // Keep every live session; bound only the history of finished sessions.
  const shown=[...runs.filter(active),...runs.filter(run=>!active(run)).slice(0,8)]
  if(!shown.length)section.append(node('p','hint','No backlog review yet.'))
  for(const run of shown){
   const item=node('div','orchestration-session');const copy=node('div','orchestration-row-copy')
   const created=new Date(run.createdAt)
   const submitted=Number.isNaN(created.getTime())?'':created.toLocaleString(undefined,{month:'short',day:'numeric',hour:'2-digit',minute:'2-digit'})
   const stage=isOrchestrationSession(run)?'':lastInput.taskStages?.get(run.taskId)||run.skill||'Stage unavailable'
   copy.append(node('strong','',titleOf(run)),node('span','orchestration-session-meta',[keyOf(run),stage,submitted].filter(Boolean).join(' · ')))
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
  const started=!!agent&&startedHere(project.id,agent.id),armed=armedOf(project.id),elsewhere=!!agent&&!!armed&&!started
  const status=agent?orchestrationProfileState(history,{...state,started}):{id:runStateOf(manual),label:runStateLabel(runStateOf(manual)),run:manual}
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
  else if(state.pending)notice.append(node('h2','','Starting…'))
  else if(history.some(active))notice.append(node('h2','','Reviewing the backlog'),node('p','','The session reads the whole backlog with this agent’s instructions and launches the tickets it chooses, within the concurrency limit.'))
  else if(latest?.status==='failed'&&!started){notice.classList.add('orchestration-notice-error');notice.append(node('h2','','The last review failed'),node('p','','Open its console for details, or start this agent again.'))}
  else if(started)notice.append(node('h2','','Started'),node('p','','Waiting for a slot: when a chain this agent launched ends, a new session reviews the backlog again. Stop to let the launched chains finish without taking more.'))
  else if(elsewhere)notice.append(node('h2','','Another agent is started'),node('p','',(armed.profileName||'Another agent')+' orchestrates this project. Stop it before starting this one.'))
  else notice.append(node('h2','','Ready to start'),node('p','','Start: an agent session reviews the whole backlog with this agent’s instructions, launches the tickets it chooses, and reviews it again whenever one of their chains ends.'))
  const actions=node('div','orchestration-actions')
  if(agent&&started){
   const stop=button('Stop','primary',()=>stopAgent(project,history,state));stop.classList.add('orchestration-run');stop.dataset.focus='run';stop.disabled=lastInput.offline||!!state.stopping;actions.append(stop)
  }else if(agent){const runButton=button('Start','primary',()=>startAgent(project,agent,state));runButton.classList.add('orchestration-run');runButton.dataset.focus='run';runButton.disabled=lastInput.offline||lastInput.stale||!!state.pending||elsewhere;actions.append(runButton)}
  if(latest){const consoleButton=button('Open console','secondary',()=>onConsole(latest));consoleButton.dataset.focus='console';consoleButton.disabled=lastInput.offline;actions.append(consoleButton)}
  notice.append(actions);details.append(notice)
  if(lastInput.offline||lastInput.stale)details.append(node('p','hint',lastInput.offline?'Reconnect the local agent to start agents or open consoles.':'Capacity could not be refreshed. Reconnect before starting an agent.'))
  if(state.error){const error=node('p','orchestration-error',state.error);error.setAttribute('role','alert');details.append(error)}
  if(latest)taskDetails(details,latest)
  if(agent)decisions(details,project.id)
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
 const view={close(){if(!selection)return;selection=null;onClose();redraw()},
 // Another surface started or stopped an agent: read the servers again.
 invalidate(){for(const server of servers.values())server.readAt=0;redraw()},
 render(input){
  lastInput=input
  const {projects,runs,taskStages=new Map(),taskTitles=new Map(),taskKeys=new Map(),stale=false,offline=false}=input
  const groups=orchestrationGroups(projects,runs)
  if(!offline)for(const {project} of groups)if(project.orchestratorEnabled)loadServer(project.id)
  const stamp=JSON.stringify([groups,runs,stale,offline,revision,[...taskStages],[...taskTitles],[...taskKeys],[...servers].map(([id,server])=>[id,server.state||null])])
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
    const status=orchestrationProfileState(history,{...state,started:startedHere(project.id,agent.id)})
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
