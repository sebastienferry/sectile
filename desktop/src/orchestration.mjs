import { runEngine } from './run-engine.mjs'
import { runStateOf, runStateLabel, runStateSvg } from '../../shared/runStates.ts'

const active=run=>['running','queued','preparing','pending','waiting'].includes(run.status)

// Include manual task agents because they consume the same project capacity.
// Free discussions and terminal-only consoles are outside backlog orchestration.
export function orchestrationGroups(projects,runs){
 const candidates=runs.filter(run=>active(run)&&run.taskId&&run.kind!=='console'&&run.skill!=='discuss'&&!run.macroKey)
 return projects.filter(project=>!project.disconnected&&(project.orchestratorEnabled||candidates.some(run=>run.projectId===project.id)))
  .map(project=>({project,agents:candidates.filter(run=>run.projectId===project.id)}))
  .sort((a,b)=>a.project.name.localeCompare(b.project.name))
}

export function createOrchestration({container,onConsole}){
 let previous=''
 const node=(tag,className,text)=>{const element=document.createElement(tag);if(className)element.className=className;if(text!==undefined)element.textContent=text;return element}
 return {render({projects,runs,taskStages=new Map(),taskTitles=new Map(),selected,stale=false,offline=false}){
  const groups=orchestrationGroups(projects,runs)
  const stamp=JSON.stringify([groups,selected,stale,offline,[...taskStages],[...taskTitles]])
  if(stamp===previous)return
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
}
