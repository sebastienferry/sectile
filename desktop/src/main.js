import { logText } from './log-text.mjs'
import { createGitDiff } from './gitDiff.js'

import { skillResult } from './skill-result.mjs'
import { orderedQueueRuns } from './queue.mjs'
import { orderedTaskGroups } from './task-order.mjs'
import { transitions, announce } from './notifications.mjs'
import { runStateOf, runStateLabel, runStateSvg } from '../../shared/runStates.ts'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import './style.css'
import { taskStage, nextTaskStep, closingStep } from './workflow.mjs'
import { launchModeOverride, modeSelect } from './skill-mode.mjs'
import { consoleNotice, needsConsoleNotice } from './run-console.mjs'
import { previewLines } from './command-preview.mjs'
const api=window.localAgent
// Concurrent execution workers ceiling per project, aligned with agentconfig.MaxParallelism.
// Parallelism is a workstation setting: the server neither stores nor supplies it.
const MAX_PARALLELISM=10
document.querySelector('#app').innerHTML=`
<header><div><button id="toggle-sidebar" aria-expanded="true"></button><strong id="app-title">Sectile Desktop</strong><small>Execution consoles</small></div><span id="connection">Connecting…</span><button id="command-palette" title="Commands (⌘K / Ctrl+K)">⌘K</button><nav aria-label="Local agent controls"><button id="agent-logs" type="button" title="View local-agent diagnostics">Agent logs</button><button id="configure" class="icon-button" aria-label="Local agent" title="Agent connection settings"></button><button id="start-agent" class="icon-button" aria-label="Start agent" title="Start agent"></button><button id="shutdown" class="icon-button" aria-label="Stop agent" title="Stop agent" hidden></button><button id="restart" class="icon-button" aria-label="Restart agent" title="Restart agent" hidden></button><button id="profile" class="icon-button" aria-label="Profile" title="Profile"></button></nav></header>
<section id="setup" hidden><div id="agent-offline" role="status" hidden><strong>Local agent is stopped</strong><p>Start the agent to run tasks and access your local consoles.</p></div><h1>Connect to Sectile</h1><p>In the Sectile web interface, under your profile, choose <strong>Pair a workstation</strong> and paste the code here. A code is single use and expires within ten minutes; this machine keeps the credential it receives, so the code is never needed again.</p>
<form id="start"><label>Sectile server<input name="server" type="url" value="http://localhost:8090" required></label><label>Pairing code<input name="code" type="text" autocomplete="off" spellcheck="false" placeholder="Paste the code from the web interface"></label><details id="advanced-credential"><summary>Advanced: connect with an API key instead</summary><label>API key<input name="token" type="password" autocomplete="off" placeholder="sectile_…"></label></details><button>Connect</button></form></section>
<main id="workspace" hidden><aside><div class="section">PROJECTS <button id="add-project" title="Add a remote project">+</button></div><div id="runs"></div><button id="clear-history" disabled>Clear finished consoles</button><p class="hint">Open an agent console from a project, or launch a task.</p></aside><div id="sidebar-resizer" role="separator" aria-label="Resize sidebar" aria-orientation="vertical" tabindex="0"></div><article><div id="toolbar"><div><div class="terminal-title-line"><strong id="title">Select an execution</strong><span id="skill-result" role="status" hidden></span></div><small id="directory"></small></div><select id="execution-history" aria-label="Execution history" hidden></select><button id="selected-pr" hidden></button><button id="rerun" hidden>Relaunch</button><button id="save-log">Export log</button><button id="stop" class="icon-button" type="button" aria-label="Stop execution" title="Stop execution" disabled></button></div><div class="execution-views" role="group" aria-label="Execution view"><button id="view-console" type="button" aria-pressed="true" disabled>Console</button><button id="view-changes" type="button" aria-pressed="false" disabled>Changes</button></div><section id="changes" aria-label="Worktree changes" hidden></section><div id="terminal"></div><footer id="task-status"><span id="next-step-status" role="status" aria-live="polite">Select a task to see its next step</span><button id="next-step" type="button" hidden disabled></button><button id="retry-next-step" type="button" hidden>Retry</button></footer></article><section id="agent-log-pane" aria-label="Agent logs" hidden></section></main>
<dialog id="project-dialog"><button id="close-dialog" aria-label="Close">×</button><div id="dialog-body"></div><div class="dialog-footer"><button id="dismiss-dialog">Close settings</button></div></dialog><div id="error" role="alert"></div>`
// The console shows a prompt the user configured elsewhere - oh-my-posh, starship, powerlevel10k -
// and those draw their separators and icons from the Private Use Area. Menlo is a macOS font, so on
// Windows every one of those glyphs fell back to a replacement box. The Mono variants are the ones
// that keep a glyph to a single cell, which is what the grid needs, and Symbols Nerd Font Mono sits
// near the end as a per-glyph fallback: a host with no patched font still gets the icons.
const TERMINAL_FONT='"FiraCode Nerd Font Mono", "JetBrainsMono Nerd Font Mono", "Hack Nerd Font Mono", "CaskaydiaCove Nerd Font Mono", "MesloLGS NF", Menlo, Consolas, "Symbols Nerd Font Mono", monospace'
const terminal=new Terminal({cursorBlink:true,fontSize:13,fontFamily:TERMINAL_FONT,scrollback:20000,theme:{background:'#11151c',foreground:'#d8e0ec'}})
const fit=new FitAddon();terminal.loadAddon(fit)
let nextStepData=null,nextStepGeneration=0,nextStepUpdated=0
const submittingSteps=new Set()
const submittedSteps=new Map()
const nextStepErrors=new Map()
const taskTitles=new Map()
const skillResults=new Map(),loadingSkillResults=new Set()
const pullRequests=new Map()
let localTasks={}
try{localTasks=JSON.parse(localStorage.getItem('localTasks')||'{}')}catch{}
const freeConsole=run=>run?.kind==='console'
const runLabel=run=>freeConsole(run)?(run.provider==='claude'?'Claude':'Codex')+' console':run.skill
const taskKey=run=>JSON.stringify([run.projectId,freeConsole(run)?run.id:run.taskId])
const activeRun=run=>['running','queued','preparing'].includes(run.status)
const taskState=run=>localTasks[taskKey(run)]||{}
let disconnectedProjects=new Set(),projectStateVersion=0,refreshing=false
// A background refresh must not reorder or rebuild the task list under the
// user's pointer: it is deferred until the interaction ends.
let pendingRender=false
const sidebarList=()=>document.querySelector('#runs')
// Only the task rows are held: project headings and the queue view keep
// refreshing normally.
const sidebarBusy=()=>!!document.querySelector('#runs .local-task:hover')||!!document.activeElement?.closest?.('#runs .local-task')
const hiddenProject=id=>disconnectedProjects.has(id)
const hiddenRun=run=>hiddenProject(run.projectId)||(taskState(run).archivedRuns||[]).includes(run.id)&&!activeRun(run)
function saveLocalTasks(){localStorage.setItem('localTasks',JSON.stringify(localTasks))}
const collapsedProjects=new Set(JSON.parse(localStorage.getItem('collapsedProjects')||'[]'))

const queueProjects=new Set()

let linksLoading=false,lastLinksRefresh=0
let selectedProject=null
let logsOpen=false,agentConnected=false
let opened=false,selected=null,runs=[],last='',stopping=false,restarting=false,projects=[],projectsLoaded=false
const changes=createGitDiff({api,container:document.querySelector('#changes'),terminal:document.querySelector('#terminal'),consoleButton:document.querySelector('#view-console'),changesButton:document.querySelector('#view-changes'),onConsole:()=>{resize();if(opened)terminal.focus()}})
api.onOutput(data=>terminal.write(new Uint8Array(data)))
terminal.onData(data=>{if(!changes.active&&!logsOpen)api.input(data)})
function resize(){if(opened&&!changes.active&&!logsOpen){fit.fit();api.resize(terminal.cols,terminal.rows)}}
window.addEventListener('resize',resize)
// The system buttons are painted over the header, so the header has to keep their strip clear.
// Their geometry comes from the overlay itself rather than from a guess: it differs per platform,
// moves when the window resizes, and is absent entirely when the overlay is hidden - full screen,
// or a window that was never shown - where the header takes the full width again. Deriving it in
// CSS from env(titlebar-area-*) looks tidier but reads 0 in exactly that case, which turns the
// padding into the whole window width.
function fitTitlebar(){
 const overlay=navigator.windowControlsOverlay
 const header=document.querySelector('header')
 if(!overlay||!overlay.visible){header.style.removeProperty('padding-left');header.style.removeProperty('padding-right');return}
 const area=overlay.getTitlebarAreaRect()
 header.style.paddingLeft=(area.x+16)+'px'
 header.style.paddingRight=Math.max(16,window.innerWidth-area.x-area.width+16)+'px'
}
navigator.windowControlsOverlay?.addEventListener('geometrychange',fitTitlebar)
window.addEventListener('resize',fitTitlebar)
fitTitlebar()
function error(err){document.querySelector('#error').textContent=err?.message||String(err)}
function connectionStatus(status){
 const container=document.querySelector('#connection')
 // A contract mismatch is not a dropped link: the server answers, but with a
 // build this agent cannot talk to. Reported as a disconnection it reads as a
 // network problem and nobody looks at the build, so name it and carry the
 // agent's own diagnosis in the tooltip.
 if(!status.connected){
  container.title=status.contractError||''
  container.textContent=status.contractError?'Local agent ready · Server incompatible':status.text||'Local agent ready · Server disconnected'
  return
 }
 container.title=''
 let link=container.querySelector('a')
 if(!link){
  link=document.createElement('a')
  link.onclick=event=>{event.preventDefault();api.openBoard().catch(error)}
  container.replaceChildren(document.createTextNode('Connected to '),link)
 }
 link.textContent=status.server
 link.title='Open board in default browser'
 try{
  const url=new URL(status.server)
  if(!['http:','https:'].includes(url.protocol)||url.username||url.password)throw Error('Invalid server URL')
  url.searchParams.delete('task');url.hash=''
  link.href=url.href
 }catch{container.textContent='Connected to '+status.server}
}
function agentUnavailable(){
 agentConnected=false
 changes.disconnect()
 document.querySelector('#start-agent').disabled=false
 document.querySelector('#start button').disabled=false
 document.querySelector('#start button').textContent='Start local agent'
 document.querySelector('#shutdown').hidden=true
 document.querySelector('#restart').hidden=true
 document.querySelector('#agent-offline').hidden=false
 document.querySelector('#setup').hidden=logsOpen
 document.querySelector('#workspace').hidden=!logsOpen
 connectionStatus({text:'Local agent stopped'})
 projectsLoaded=false
 api.detach().catch(()=>{})
}
function ready(){
 agentConnected=true
 document.querySelector('#agent-offline').hidden=true
 if(!projectsLoaded){projectsLoaded=true;loadProjects().catch(()=>{projectsLoaded=false})}
 document.querySelector('#start-agent').disabled=true;document.querySelector('#start button').disabled=true;document.querySelector('#restart').hidden=false;document.querySelector('#shutdown').hidden=false
 document.querySelector('#setup').hidden=true;document.querySelector('#workspace').hidden=false
 if(!document.querySelector('#connection a'))connectionStatus({text:'Local agent connected'})
 if(!opened){terminal.open(document.querySelector('#terminal'));opened=true;resize()}
}
function select(run,background=false,options){
 if(hiddenProject(run.projectId))return
 if(!background)closeLogs(false)
 selectedProject=run.projectId
 selected=run.id
 changes.select(selected)

 refreshSkillResult()
 refreshNextStep()
 document.querySelector('#directory').textContent=run.directory
 document.querySelector('#stop').disabled=!['running','queued','preparing'].includes(run.status)
 terminal.reset()
 if(needsConsoleNotice(run)){
  api.detach().catch(error)
  terminal.writeln(consoleNotice(run))
  render(options);return
 }
 api.attach(run.id).then(()=>{setTimeout(resize,150);if(!changes.active&&!logsOpen)terminal.focus()}).catch(error)
 render(options)
}
// The state the user reads, drawn from the shared definition so the row, the
// execution queue and the banner the desktop raises cannot say three things.
// The glyph is decorative: the label carries the state for anyone who cannot
// resolve an amber hand, and the row tooltip repeats it.
function renderRunState(element,run){
 const state=runStateOf(run),label=runStateLabel(state)
 if(element.dataset.runState!==state){element.dataset.runState=state;element.innerHTML=runStateSvg(state,14)}
 element.title=label;element.setAttribute('aria-label',label)
 return label
}
function renderQueue(project,group){
 const runsForProject=runs.filter(run=>run.projectId===project.id)
 const panel=document.createElement('section');panel.className='execution-queue';panel.setAttribute('aria-label','Execution queue for '+project.name)
 const header=document.createElement('div');header.className='queue-heading'
 const label=document.createElement('strong');label.textContent='Execution queue'
 const close=document.createElement('button');close.className='icon-button';close.type='button';close.title='Close queue';close.setAttribute('aria-label','Close queue for '+project.name)
 close.innerHTML='<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18"/></svg>'
 close.onclick=()=>{queueProjects.delete(project.id);render();document.querySelectorAll('.project-queue-toggle').forEach(button=>{if(button.dataset.projectId===project.id)button.focus()})}
 header.append(label,close);panel.append(header)
 const summary=document.createElement('p');summary.className='queue-summary';summary.setAttribute('role','status')
 const list=document.createElement('div');list.className='queue-list';panel.append(summary,list);group.append(panel)
 const active=runsForProject.filter(run=>['running','preparing'].includes(run.status)&&!run.cancelRequested)
 const stopping=runsForProject.filter(run=>activeRun(run)&&run.cancelRequested)
 const waiting=orderedQueueRuns(runsForProject)
 const text=active.length+' active · '+waiting.length+' waiting'+(stopping.length?' · '+stopping.length+' stopping':'')
 if(summary.textContent!==text)summary.textContent=text
 if(!active.length&&!waiting.length&&!stopping.length){
  const empty=document.createElement('p');empty.textContent='No active or queued executions';list.append(empty);return
 }
 for(const [label,items] of [['Waiting · submission order',waiting],['Stopping / canceling',stopping],['Running / preparing',active]]){
  if(!items.length)continue
  const heading=document.createElement('h3');heading.textContent=label;list.append(heading)
  const entries=document.createElement('ul');list.append(entries)
  for(const run of items){
   const item=document.createElement('li'),button=document.createElement('button')
   button.className='queue-execution';button.dataset.runId=run.id
   button.setAttribute('aria-pressed',String(run.id===selected))
   const title=document.createElement('strong');title.textContent=[run.taskKey||run.taskId,taskState(run).name||taskTitles.get(run.taskId)||runLabel(run)].filter(Boolean).join(' · ')
   const context=document.createElement('small');context.textContent=(projects.find(project=>project.id===run.projectId)?.name||run.projectId)+' · '+runLabel(run)+' · '+(run.cancelRequested?(run.status==='queued'?'Canceling':'Stopping; waiting for exit'):runStateLabel(runStateOf(run)))
   button.append(title,context);button.onclick=()=>select(run);item.append(button);entries.append(item)
  }
 }
 if(waiting.length){const note=document.createElement('p');note.className='queue-note';note.textContent='Starts when project capacity and checkout availability permit. Independent projects may start separately.';list.append(note)}
}
async function refreshSkillResult(id=selected){
 const run=runs.find(item=>item.id===id)
 if(!run||freeConsole(run)||loadingSkillResults.has(run.id)||loadingSkillResults.size>=4)return
 loadingSkillResults.add(run.id)
 try{skillResults.set(run.id,await api.runResult(run.id))}catch{skillResults.delete(run.id)}
 finally{loadingSkillResults.delete(run.id);renderHeader();renderTaskSkillStatuses()}
}
let refreshingVisibleSkillResults=false
async function refreshVisibleSkillResults(){
 if(refreshingVisibleSkillResults)return
 refreshingVisibleSkillResults=true
 const ids=[...new Set([selected,...[...document.querySelectorAll('.task-skill-status')].map(item=>item.dataset.runId)].filter(Boolean))]
 try{
  await Promise.all(Array.from({length:Math.min(4,ids.length)},async()=>{
   while(ids.length)await refreshSkillResult(ids.shift())
  }))
 }finally{refreshingVisibleSkillResults=false}
}
// Keeps the visible indicators live while the row sequence is held.
function renderTaskRowStates(){
 for(const button of document.querySelectorAll('.local-task .run')){
  const run=runs.find(item=>item.id===button.dataset.runId)
  if(run)button.dataset.status=run.status
 }
 for(const element of document.querySelectorAll('.local-task .run-state')){
  const run=runs.find(item=>item.id===element.dataset.runId)
  if(run)renderRunState(element,run)
 }
 renderTaskSkillStatuses()
}
function renderTaskSkillStatuses(){
 for(const badge of document.querySelectorAll('.task-skill-status')){
  const run=runs.find(item=>item.id===badge.dataset.runId)
  const result=skillResult(run,skillResults.get(run?.id))
  if(!result)continue
  badge.className='status task-skill-status '+result.kind
  if(badge.textContent!==result.icon)badge.textContent=result.icon
  badge.title=runLabel(run)+' · '+result.label
  badge.setAttribute('aria-label',badge.title)
 }
}
function renderHeader(){
 const run=runs.find(item=>item.id===selected)
 let text='Select an execution'
 if(run){
  const identity=run.taskKey||run.taskId
  const name=taskState(run).name?.trim()||taskTitles.get(run.taskId)?.trim()
  text=freeConsole(run)?(taskState(run).name||runLabel(run)):[identity,...(name&&name!==identity?[name]:[]),run.skill].join(' · ')
 }
 const title=document.querySelector('#title')
 title.textContent=text;title.title=text
 const badge=document.querySelector('#skill-result'),result=skillResult(run,skillResults.get(run?.id))
 badge.hidden=!result
 if(result){badge.className='skill-result '+result.kind;if(badge.textContent!==result.icon+' '+result.label)badge.textContent=result.icon+' '+result.label;badge.title=result.label;badge.setAttribute('aria-label',result.label)}
}
function render(options){
 if(options?.deferrable&&sidebarBusy()){pendingRender=true;renderHeader();renderTaskRowStates();return}
 pendingRender=false
 changes.select(selected)
 renderHeader()
 const list=document.querySelector('#runs');list.replaceChildren()

 const groups=new Map(projects.filter(project=>project.path&&!hiddenProject(project.id)).map(project=>[project.id,project]))
 for(const run of runs)if(!hiddenProject(run.projectId)&&!groups.has(run.projectId))groups.set(run.projectId,{id:run.projectId,name:run.projectId})
 for(const project of [...groups.values()].sort((a,b)=>a.name.localeCompare(b.name))){
  const group=document.createElement('section');group.className='project-group'

  const projectRow=document.createElement('div');projectRow.className='project-row'
  const heading=document.createElement('button');heading.className='project-heading';heading.textContent=(collapsedProjects.has(project.id)?'▸ ':'▾ ')+project.name;heading.setAttribute('aria-expanded',String(!collapsedProjects.has(project.id)))
  heading.setAttribute('aria-label',heading.textContent)
  const name=document.createElement('span');name.className='project-name';name.textContent=heading.textContent
  const capacity=document.createElement('span');capacity.className='project-capacity'
  const running=runs.filter(run=>run.projectId===project.id&&['running','preparing'].includes(run.status)).length
  const maximum=Number.isInteger(project.executionLimit)&&project.executionLimit>0?project.executionLimit:'?'
  capacity.textContent='('+running+'/'+maximum+')'
  capacity.title=running+' running / '+(maximum==='?'?'maximum unavailable':maximum+' maximum')+' · Includes preparing and stopping executions'
  capacity.setAttribute('aria-label',capacity.title)
  heading.replaceChildren(name,capacity)
  heading.onclick=()=>{selectedProject=project.id;if(collapsedProjects.has(project.id))collapsedProjects.delete(project.id);else collapsedProjects.add(project.id);localStorage.setItem('collapsedProjects',JSON.stringify([...collapsedProjects]));render()}
  const configure=document.createElement('button');configure.textContent='⚙';configure.setAttribute('aria-label','Configure '+project.name);configure.onclick=()=>openProject(project.id)
  const browse=document.createElement('button');browse.textContent='+';browse.title='New task';browse.setAttribute('aria-label','New task in '+project.name);browse.onclick=()=>newProjectTask(project.id)
  const openTasks=document.createElement('button');openTasks.className='project-open-tasks';openTasks.title='Open tasks in '+project.name;openTasks.setAttribute('aria-label',openTasks.title)
  openTasks.innerHTML='<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true"><path d="M9 5h12M9 12h12M9 19h12M3 5h1M3 12h1M3 19h1"/></svg>'
  openTasks.onclick=()=>browseTasks(project.id)
  const queue=document.createElement('button');queue.className='project-queue-toggle icon-button';queue.dataset.projectId=project.id
  const waitingCount=runs.filter(run=>run.projectId===project.id&&run.status==='queued'&&!run.cancelRequested).length
  queue.setAttribute('aria-label','Queue view for '+project.name);queue.setAttribute('aria-pressed',String(queueProjects.has(project.id)))
  queue.title=(queueProjects.has(project.id)?'Show tasks':'Show execution queue')+' · '+waitingCount+' waiting'
  queue.innerHTML='<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true"><path d="M3 4h18l-7 8v7l-4 2v-9Z"/></svg>'
  if(waitingCount){const badge=document.createElement('span');badge.className='queue-count';badge.textContent=waitingCount;badge.setAttribute('aria-hidden','true');queue.append(badge)}
  queue.onclick=()=>{selectedProject=project.id;if(queueProjects.has(project.id))queueProjects.delete(project.id);else queueProjects.add(project.id);collapsedProjects.delete(project.id);localStorage.setItem('collapsedProjects',JSON.stringify([...collapsedProjects]));render()}
  const consoleButton=document.createElement('button');consoleButton.textContent='>_';consoleButton.title='Open agent console';consoleButton.setAttribute('aria-label','Open agent console in '+project.name);consoleButton.disabled=!project.path;consoleButton.onclick=()=>openAgentConsole(project.id)
  projectRow.append(heading,openTasks,queue,browse,consoleButton,configure);group.append(projectRow)
  const children=runs.filter(run=>run.projectId===project.id&&!hiddenRun(run))
  const taskGroups=new Map()
  for(const run of children){const key=taskKey(run);if(!taskGroups.has(key))taskGroups.set(key,[]);taskGroups.get(key).push(run)}
  if(!collapsedProjects.has(project.id)&&queueProjects.has(project.id)){
   renderQueue(project,group)
  }else if(!collapsedProjects.has(project.id)){
   if(!children.length){const empty=document.createElement('p');empty.className='hint';empty.textContent='No local tasks';group.append(empty)}
   for(const {executions,run} of orderedTaskGroups(taskGroups.values())){
    const isSelected=executions.some(item=>item.id===selected)
    const row=document.createElement('div');row.className='local-task '+(isSelected?'selected':'')
    const button=document.createElement('button');button.className='run '+(isSelected?'selected':'')
    const title=document.createElement('strong');title.textContent=taskState(run).name||taskTitles.get(run.taskId)||runLabel(run)
    const context=document.createElement('button');context.textContent=run.taskKey||run.taskId;context.className='task-number';context.title='Open task in Sectile';context.setAttribute('aria-label','Open '+(run.taskKey||run.taskId)+' in Sectile');context.onclick=()=>api.openTask(run.taskId).catch(error)
    const status=document.createElement('span');status.className='status task-skill-status';status.dataset.runId=run.id
    const state=document.createElement('span');state.className='run-state';state.dataset.runId=run.id
    const stateLabel=renderRunState(state,run)
    // data-status stays the status the server reported: the UI tests select on it.
    button.title=title.textContent+' · '+runLabel(run)+' · '+stateLabel+' · '+executions.length+' execution(s)';button.dataset.status=run.status;button.dataset.runId=run.id
    button.append(title,state,status);button.onclick=()=>select(run)
    const menu=document.createElement('button');menu.textContent='…';menu.className='task-menu';menu.setAttribute('aria-label','Actions for '+(taskState(run).name||run.taskKey||run.taskId||runLabel(run)));menu.onclick=()=>taskMenu(run)
    const archive=document.createElement('button');archive.className='task-archive'
    const archiveLabel=(executions.some(activeRun)?'Stop and archive ':'Archive ')+(taskState(run).name||run.taskKey||run.taskId||runLabel(run))
    archive.title=archiveLabel;archive.setAttribute('aria-label',archiveLabel)
    archive.innerHTML='<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true"><path d="M4 8h16v12H4zM3 4h18v4H3zM9 12h6"/></svg>'
    archive.onclick=()=>requestArchive(run)
    if(!freeConsole(run))row.append(context)
    row.append(button)
    const link=pullRequests.get(run.taskId)
    if(link){
     const pr=document.createElement('button');pr.className='pr-indicator';pr.title=link;pr.setAttribute('aria-label','Open '+prLabel(link)+' for '+(run.taskKey||run.taskId))
     pr.innerHTML='<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="6" cy="5" r="3"/><circle cx="6" cy="19" r="3"/><circle cx="18" cy="19" r="3"/><path d="M6 8v8M18 16V9a4 4 0 0 0-4-4h-2m3-3-3 3 3 3"/></svg>'
     pr.onclick=()=>api.openPR(link).catch(error);row.append(pr)
    }
    row.append(archive,menu);group.append(row)
   }
  }
  list.append(group)
 }
 renderTaskSkillStatuses()
 document.querySelector('#clear-history').disabled=!runs.some(run=>['completed','failed','canceled'].includes(run.status))
 const current=runs.find(run=>run.id===selected)
 const history=document.querySelector('#execution-history')
 const executions=current?runs.filter(run=>taskKey(run)===taskKey(current)).sort((a,b)=>(a.createdAt||'').localeCompare(b.createdAt||'')||a.id.localeCompare(b.id)):[]
 history.replaceChildren();history.hidden=executions.length<2
 for(const [i,run] of executions.entries()){const option=document.createElement('option');option.value=run.id;option.textContent=(i+1)+' · '+run.skill+' · '+run.status;history.append(option)}
 history.value=selected||'';history.onchange=()=>{const run=runs.find(run=>run.id===history.value);if(run)select(run)}
 const selectedPR=document.querySelector('#selected-pr'),link=current&&pullRequests.get(current.taskId)
 selectedPR.hidden=!link
 if(link){selectedPR.textContent=prLabel(link);selectedPR.title=link;selectedPR.onclick=()=>api.openPR(link).catch(error)}
 document.querySelector('#rerun').hidden=!current||!['completed','failed','canceled'].includes(current.status)
 document.querySelector('#stop').disabled=stopping||!current||!['running','queued','preparing'].includes(current.status)
 renderNextStep()
}
async function updateDisconnected(ids,force=false,deferrable=false){
 const changed=ids.length!==disconnectedProjects.size||ids.some(id=>!disconnectedProjects.has(id))
 disconnectedProjects=new Set(ids)
 if(hiddenProject(selectedProject))selectedProject=null
 const current=runs.find(run=>run.id===selected)
 if(current&&hiddenProject(current.projectId)){
  selected=null;terminal.reset()
  document.querySelector('#title').textContent='Select an execution'
  document.querySelector('#directory').textContent=''
  await api.detach().catch(error)
 }
 if(changed||force)render(deferrable?{deferrable:true}:undefined)
}
async function refresh(){
 if(refreshing||restarting||!document.querySelector('#setup').hidden)return
 refreshing=true
 const version=projectStateVersion
 try{
  const next=await api.runs(),status=await api.status()
  if(version!==projectStateVersion)return
  ready();refreshPRs(next)
  connectionStatus(status)
  const previous=runs.find(run=>run.id===selected)
  // Compare the two polls before the new list replaces the old: a session that
  // has just started waiting, or has just finished, is what earns a banner.
  announce(transitions(runs,next))
  const serialized=JSON.stringify(next),changed=serialized!==last
  runs=next;last=serialized
  await updateDisconnected(status.disconnectedProjects||[],changed,true)
  const current=runs.find(run=>run.id===selected)
  if(current&&((current.status!==previous?.status&&(current.status==='running'||!current.sessionId))||current.sessionId!==previous?.sessionId))select(current,true,{deferrable:true})
  if(!selected){const visible=runs.find(run=>!hiddenRun(run));if(visible)select(visible,true,{deferrable:true})}
  // Sessions Sectile did not launch have no run to compare; they report
  // themselves and are announced as they are drained.
  try{const alerts=await api.sessionAlerts();if(alerts?.length)announce(alerts.map(alert=>({id:'session:'+alert.session,state:alert.state,name:alert.session})))}catch{}
  if(changed||Date.now()-nextStepUpdated>15000)refreshNextStep()
  refreshVisibleSkillResults()
 }catch{agentUnavailable()}
 finally{refreshing=false}
}
document.querySelector('#start').onsubmit=async event=>{
 event.preventDefault();const button=event.target.querySelector('button');button.disabled=true
 try{await api.start(Object.fromEntries(new FormData(event.target)));event.target.elements.code.value='';document.querySelector('#error').textContent='';ready();await refresh()}
 catch(err){error(err)}finally{button.disabled=!document.querySelector('#shutdown').hidden}
}
document.querySelector('#stop').onclick=async()=>{
 if(!selected)return
 const run=runs.find(item=>item.id===selected)
 stopping=true;render()
 let stopped=false
 try{await api.stop(selected);await refresh();stopped=true}catch(err){error(err)}finally{stopping=false;render()}
 if(stopped&&run)await offerClosure(run)
}
// Stopping an execution is where the user stands when a task has reached
// reviewed, and nothing else in the desktop proposes its closing step. The
// offer comes after the stop so that stopping never depends on it: an
// unreadable task, a project without the skill or a failed stop simply means
// no dialog.
async function offerClosure(run){
 if(freeConsole(run))return
 let task=null,step=null
 try{
  const [tasks,project]=await Promise.all([api.serverTasks(run.projectId,run.taskKey||run.taskId),api.project(run.projectId)])
  task=tasks.find(item=>item.id===run.taskId)
  step=task?closingStep(task,project):null
 }catch{return}
 if(!step)return
 showDialog('Close '+(task.key||run.taskKey||run.taskId)+'?')
 paragraph('This task is reviewed: its pull request is in human hands. Closing it runs the handoff skill, which writes the handover report and takes the task to finished.')
 const confirm=document.createElement('button');confirm.textContent=step.label
 const notice=document.createElement('p');notice.setAttribute('role','status')
 dialogBody.append(confirm,notice);confirm.focus()
 confirm.onclick=async()=>{
  confirm.disabled=true;notice.textContent='Launching the closing skill…'
  try{await api.launchServerTask(run.projectId,run.taskId,step.skillId,'');dialog.close();await refresh()}
  catch(err){notice.textContent=err.message;confirm.disabled=false}
 }
}
function flushSidebar(){if(pendingRender&&!sidebarBusy())render()}
// Hover and focus targets settle after the event, so the check runs next tick.
const scheduleFlush=()=>setTimeout(flushSidebar,0)
sidebarList().addEventListener('pointerout',scheduleFlush)
sidebarList().addEventListener('focusout',scheduleFlush)
api.connect().then(connected=>{if(connected){ready();refresh()}else{agentUnavailable()}}).catch(error)
setInterval(async()=>{
 if(restarting)return
 if(document.querySelector('#setup').hidden)refresh()
 else if(document.querySelector('#shutdown').hidden){
  try{if(await api.connect()){ready();refresh()}}catch{}
 }
},2000)

document.querySelector('#save-log').onclick=()=>{
 const buffer=terminal.buffer.active,lines=[]
 for(let i=0;i<buffer.length;i++)lines.push(buffer.getLine(i)?.translateToString(true)||'')
 api.saveLog(lines.join('\n')).catch(error)
}


document.querySelector('#restart').onclick=async()=>{
 const button=document.querySelector('#restart');button.disabled=true;restarting=true
 try{
  if(await api.restart()){
   selected=null;runs=[];last='';terminal.reset();render()
   renderHeader()
   document.querySelector('#directory').textContent=''
   document.querySelector('#error').textContent=''
  }
 }catch(err){error(err)}finally{restarting=false;button.disabled=false;await refresh()}
}

async function loadSettings(){
 const settings=await api.settings()
 for(const name of ['server','token']){
  if(settings[name])document.querySelector('#start').elements[name].value=settings[name]
 }
 // A stored credential is shown where it lives, so a paired machine sees why the
 // code field can stay empty.
 if(settings.token)document.querySelector('#advanced-credential').open=true
}
const settingsReady=loadSettings().catch(error)
document.querySelector('#configure').onclick=()=>{
 if(logsOpen){closeLogs(false);document.querySelector('#setup').hidden=false;document.querySelector('#workspace').hidden=true;return}
 const setup=document.querySelector('#setup');setup.hidden=!setup.hidden
 document.querySelector('#workspace').hidden=!setup.hidden
}
document.querySelector('#shutdown').onclick=async()=>{
 const button=document.querySelector('#shutdown');button.disabled=true;restarting=true
 try{
  if(await api.shutdown()){
   selected=null;runs=[];last='';terminal.reset();render()
   document.querySelector('#setup').hidden=false;document.querySelector('#workspace').hidden=true
   document.querySelector('#restart').hidden=true;button.hidden=true
   document.querySelector('#start-agent').disabled=false;document.querySelector('#start button').disabled=false
   agentUnavailable()
   document.querySelector('#error').textContent=''
  }
 }catch(err){error(err)}finally{restarting=false;button.disabled=false}
}

document.querySelector('#clear-history').onclick=async()=>{
 const button=document.querySelector('#clear-history');button.disabled=true
 try{
  const {removed}=await api.clearHistory()
  if(removed.includes(selected)){
   selected=null;terminal.reset()
   renderHeader()
   document.querySelector('#directory').textContent=''
  }
  await refresh()
 }catch(err){error(err)}finally{render()}
}

const dialog=document.querySelector('#project-dialog'),dialogBody=document.querySelector('#dialog-body')
document.querySelector('#close-dialog').onclick=()=>dialog.close()
document.querySelector('#dismiss-dialog').onclick=()=>dialog.close()
function showDialog(title){
 closeLogs(false)
 document.querySelector('#dismiss-dialog').textContent='Close settings'
 dialogBody.replaceChildren()
 const heading=document.createElement('h2');heading.textContent=title;dialogBody.append(heading)
 if(!dialog.open)dialog.showModal()
}
function paragraph(text){const p=document.createElement('p');p.textContent=text;dialogBody.append(p);return p}
const logPane=document.querySelector('#agent-log-pane')
function closeLogs(restoreFocus=true){
 if(!logsOpen)return
 logsOpen=false;logPane.hidden=true;logPane.replaceChildren()
 document.querySelector('#workspace article').hidden=false
 document.querySelector('#workspace').hidden=!agentConnected
 document.querySelector('#setup').hidden=agentConnected
 document.querySelector('#agent-logs').setAttribute('aria-pressed','false')
 resize()
 if(restoreFocus)document.querySelector('#agent-logs').focus()
}
window.addEventListener('keydown',event=>{
 if(event.key==='Escape'&&logsOpen&&!dialog.open){event.preventDefault();closeLogs()}
})
document.querySelector('#agent-logs').setAttribute('aria-pressed','false')
document.querySelector('#agent-logs').onclick=()=>{
 if(dialog.open)dialog.close()
 logsOpen=true;logPane.hidden=false;logPane.replaceChildren()
 document.querySelector('#workspace article').hidden=true
 document.querySelector('#workspace').hidden=false
 document.querySelector('#setup').hidden=true
 document.querySelector('#agent-logs').setAttribute('aria-pressed','true')
 const heading=document.createElement('h2');heading.textContent='Agent logs'
 const close=document.createElement('button');close.type='button';close.textContent='Close logs';close.onclick=()=>closeLogs()
 const toolbar=document.createElement('div');toolbar.className='agent-log-toolbar';toolbar.append(heading,close)
 const description=document.createElement('p');description.textContent='Diagnostics captured by this desktop app. Agents started elsewhere may write to their original terminal instead.'
 const source=document.createElement('p');source.className='agent-log-source'
 const refresh=document.createElement('button');refresh.type='button';refresh.textContent='Refresh'
 const status=document.createElement('p');status.setAttribute('role','status')
 const output=document.createElement('pre');output.className='agent-log-output';output.tabIndex=0;output.setAttribute('aria-label','Agent log contents')
 logPane.append(toolbar,description,source,refresh,status,output)
 const load=async()=>{
  refresh.disabled=true;output.textContent='';status.textContent='Loading agent log…'
  try{
   const snapshot=await api.agentLogs()
   if(!output.isConnected)return
   source.textContent=snapshot.path
   status.textContent=snapshot.missing?'No desktop agent log exists yet.':!snapshot.text?'The agent log is empty.':snapshot.truncated?'Showing the latest 256 KiB; earlier output omitted.':'Showing the current log snapshot.'
   output.textContent=logText(snapshot.text)
   output.scrollTop=output.scrollHeight
  }catch(err){if(output.isConnected)status.textContent='Unable to read agent log: '+(err.message||String(err))}
  finally{refresh.disabled=false}
 }
 refresh.onclick=load
 close.focus()
 load()
}

async function loadProjects(){
 const version=projectStateVersion
 const status=await api.status(),next=await api.projects()
 if(version!==projectStateVersion)return
 const loaded=[...new Map(next.map(project=>[project.id,project])).values()]
 projects=loaded
 await updateDisconnected(status.disconnectedProjects||[],true)
 const limits=await Promise.allSettled(loaded.map(project=>api.project(project.id)))
 if(version!==projectStateVersion||projects!==loaded)return
 for(const [index,result] of limits.entries()){
  if(result.status==='fulfilled')loaded[index].executionLimit=result.value.parallelism
 }
 render()
}
// A control that looks the same either way says nothing: the chevron points where the next click
// sends the panel, and the label names that click rather than the state it leaves behind.
function renderSidebarToggle(hidden){
 const button=document.querySelector('#toggle-sidebar')
 const chevron=hidden?'m13 9 3 3-3 3':'m16 9-3 3 3 3'
 button.setAttribute('aria-expanded',String(!hidden))
 button.setAttribute('aria-label',hidden?'Show projects':'Hide projects')
 button.title=hidden?'Show projects':'Hide projects'
 button.innerHTML='<svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16"/><path d="'+chevron+'"/></svg>'
}
document.querySelector('#toggle-sidebar').onclick=()=>{
 const hidden=document.querySelector('#workspace').classList.toggle('sidebar-hidden')
 renderSidebarToggle(hidden);localStorage.setItem('sidebarCollapsed',String(hidden));resize()
}
document.querySelector('#profile').onclick=()=>{
 showDialog('Profile')

}
document.querySelector('#add-project').onclick=async()=>{
 showDialog('Add project')
 paragraph('Discover projects from your Sectile server and configure their local directory.')
 try{
  await loadProjects()
  if(!projects.length)paragraph('No projects available on the server.')
  for(const project of projects){
   const button=document.createElement('button');button.className='discovered-project'
   const added=!hiddenProject(project.id)&&(project.configured||!!project.path||runs.some(run=>run.projectId===project.id))
   button.textContent=project.name+(added?' · Already added':'')
   button.disabled=added
   if(added)button.title='Use the project settings button in the sidebar to edit this project.'
   button.onclick=()=>openProject(project.id);dialogBody.append(button)
  }
 }catch(err){error(err)}
}
function requestRemoveProject(id,name){
 showDialog('Remove '+name+' from desktop?')
 paragraph('Disconnect this project from this workstation and remove its local configuration. Repository files and server data are preserved. Add the project again before launching new executions.')
 const confirm=document.createElement('button');confirm.textContent='Disconnect project'
 const cancel=document.createElement('button');cancel.textContent='Cancel';cancel.onclick=()=>dialog.close()
 const notice=document.createElement('p');notice.setAttribute('role','status')
 confirm.onclick=async()=>{
  confirm.disabled=true;cancel.disabled=true
  try{await api.removeProject(id)}
  catch(err){notice.textContent=err.message;confirm.disabled=false;cancel.disabled=false;return}
  projectStateVersion++
  await updateDisconnected([...disconnectedProjects,id])
  dialog.close()
  try{await loadProjects()}catch(err){error('Project disconnected, but refreshing projects failed: '+err.message)}
 }
 dialogBody.append(confirm,cancel,notice)
}
async function openProject(id){
 selectedProject=id
 showDialog('Project configuration')
 try{
  const info=await api.project(id)
  let config=info.server
  dialogBody.querySelector('h2').textContent=config.projectName

  const tabs=document.createElement('div');tabs.className='project-tabs';tabs.setAttribute('role','tablist')
  const panels={}
  for(const name of ['Local','Deployment','Server']){
   const tab=document.createElement('button');tab.type='button';tab.textContent=name;tab.setAttribute('role','tab')
   tab.id='project-tab-'+name;tab.setAttribute('aria-controls','project-panel-'+name)
   const panel=document.createElement('section');panel.id='project-panel-'+name;panel.setAttribute('role','tabpanel');panel.setAttribute('aria-labelledby',tab.id);panels[name]=panel
   tab.onclick=()=>{for(const [key,value] of Object.entries(panels))value.hidden=key!==name;for(const item of tabs.children)item.setAttribute('aria-selected',String(item===tab))}
   tab.setAttribute('aria-selected',String(name==='Local'));panel.hidden=name!=='Local';tabs.append(tab)
  }
  dialogBody.append(tabs,...Object.values(panels))
  const form=document.createElement('form')
  const label=document.createElement('label');label.textContent='Local repository'
  const row=document.createElement('div');row.className='repository-picker'
  const path=document.createElement('input');path.value=info.path||'';path.required=true;path.placeholder='/path/to/repository';path.setAttribute('aria-label','Local repository')
  const browse=document.createElement('button');browse.type='button';browse.textContent='Choose folder…'
  browse.onclick=async()=>{try{const selected=await api.chooseRepository();if(selected)path.value=selected}catch(err){error(err)}}
  row.append(path,browse);label.append(row)
  let useWorktrees=info.useWorktrees,inheritWorktrees=!info.worktreeOverride
  let parallelism=info.parallelism||1
  const controls={}
  // A setting without a server default (resetLabel omitted) carries no reset control.
  function setting(name,values,resetLabel,onSelect,onReset){
   const section=document.createElement('section');section.className='execution-setting'
   const heading=document.createElement('div');heading.className='setting-heading'
   const title=document.createElement('strong');title.textContent=name
   let reset=null
   if(resetLabel){
    reset=document.createElement('button');reset.type='button';reset.className='reset-setting';reset.setAttribute('aria-label',resetLabel);reset.title=resetLabel
    reset.innerHTML='<svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.7"><path d="M4 4v6h6M4 10a8 8 0 1 1 1 8"/></svg>'
    reset.onclick=onReset
   }
   heading.append(title);if(reset)heading.append(reset)
   const group=document.createElement('div');group.className='segmented';group.setAttribute('role','group');group.setAttribute('aria-label',name)
   const buttons=values.map(value=>{const button=document.createElement('button');button.type='button';button.textContent=String(value);button.onclick=()=>onSelect(value);group.append(button);return button})
   const hint=document.createElement('p');section.append(heading,group,hint)
   return {section,buttons,hint,reset}
  }
  // A magnitude between 1 and a ceiling, which a segmented control cannot show
  // without overflowing the dialog once the ceiling grows.
  function slider(name,max,onSelect){
   const section=document.createElement('section');section.className='execution-setting'
   const heading=document.createElement('div');heading.className='setting-heading'
   const title=document.createElement('strong');title.textContent=name
   const readout=document.createElement('span');readout.className='slider-value'
   heading.append(title,readout)
   const input=document.createElement('input');input.type='range';input.min='1';input.max=String(max);input.step='1'
   input.className='slider-input';input.setAttribute('aria-label',name)
   input.oninput=()=>onSelect(Number(input.value))
   const scale=document.createElement('div');scale.className='slider-scale'
   for(const mark of [1,Math.round(max/2),max]){const item=document.createElement('span');item.textContent=String(mark);scale.append(item)}
   const hint=document.createElement('p');section.append(heading,input,scale,hint)
   return {section,input,readout,hint}
  }
  controls.worktrees=setting('Worktrees',['Yes','No'],'Reset worktrees to server default',value=>{useWorktrees=value==='Yes';inheritWorktrees=false;update()},()=>{useWorktrees=!!config.useWorktrees;inheritWorktrees=true;update()})
  controls.parallel=slider('Parallel executions',MAX_PARALLELISM,value=>{parallelism=value;update()})
  function update(){
   controls.worktrees.buttons.forEach((button,i)=>button.setAttribute('aria-pressed',String(useWorktrees===(i===0))))
   controls.worktrees.hint.textContent=(inheritWorktrees?'Inherited':'Local override')+' · Server default: '+(config.useWorktrees?'Yes':'No')
   const effective=useWorktrees?parallelism:1
   controls.parallel.input.disabled=!useWorktrees
   controls.parallel.input.value=String(effective)
   controls.parallel.readout.textContent=effective+(effective===1?' execution':' executions')
   controls.parallel.hint.textContent=useWorktrees?'Workstation setting · Additional executions wait in the local queue.':'Without worktrees, executions are limited to one.'
  }
  update()
  let inheritCommand=!info.commandOverride
  // The two execution modes run different command lines, so they get one field
  // each. Overriding only the interactive one would leave the server's headless
  // command running beside it, which is not what an override means.
  const commandLabel=document.createElement('label');commandLabel.textContent='Interactive CLI command'
  const command=document.createElement('textarea');command.className='cli-command';command.setAttribute('aria-label','Interactive CLI command')
  command.value=info.aiCommandTemplate??config.aiCommandTemplate??''
  command.placeholder='Server provider default command'
  const autonomousLabel=document.createElement('label');autonomousLabel.textContent='Autonomous CLI command (headless)'
  const autonomousCommand=document.createElement('textarea');autonomousCommand.className='cli-command';autonomousCommand.setAttribute('aria-label','Autonomous CLI command')
  autonomousCommand.value=info.aiCommandTemplateAutonomous??config.aiCommandTemplateAutonomous??''
  autonomousCommand.placeholder='Empty: the interactive command serves headless launches too'
  const commandHint=document.createElement('p')
  const commandPreviewBox=document.createElement('dl');commandPreviewBox.className='command-preview'
  function renderCommandPreview(){
   commandPreviewBox.replaceChildren()
   for(const line of previewLines(config.aiProvider,command.value,config.aiModel,autonomousCommand.value)){
    const term=document.createElement('dt');term.textContent=line.label
    const detail=document.createElement('dd');detail.textContent=line.text
    if(!line.ok)detail.className='command-preview-error'
    commandPreviewBox.append(term,detail)
   }
  }
  function commandState(){commandHint.textContent=(inheritCommand?'Inherited from server':'Local override')+' · Both empty runs the provider default for each mode. Required in a command: {prompt} (instructions). Also: {issueKey}, {issueTitle}, {issueDesc}, {branchName}, {repoPath} (local directory), {tracker}, {repo}, {model}, {mode:AUTONOMOUS|INTERACTIVE}.';renderCommandPreview()}
  command.oninput=()=>{inheritCommand=false;commandState()}
  autonomousCommand.oninput=()=>{inheritCommand=false;commandState()}
  const commandReset=document.createElement('button');commandReset.type='button';commandReset.className='reset-setting'
  commandReset.setAttribute('aria-label','Reset CLI commands to server defaults');commandReset.title='Reset CLI commands to server defaults';commandReset.innerHTML=controls.worktrees.reset.innerHTML
  commandReset.onclick=()=>{command.value=config.aiCommandTemplate||'';autonomousCommand.value=config.aiCommandTemplateAutonomous||'';inheritCommand=true;commandState()}
  commandState();commandLabel.append(commandReset,command,commandHint)
  autonomousLabel.append(autonomousCommand,commandPreviewBox)
  const save=document.createElement('button');save.textContent='Save local configuration'
  const notice=document.createElement('p');notice.setAttribute('role','status')
  form.append(label,controls.worktrees.section,controls.parallel.section,commandLabel,autonomousLabel,save)
  panels.Local.append(form)
  const remove=document.createElement('button');remove.type='button';remove.textContent='Remove from desktop'
  remove.onclick=()=>requestRemoveProject(id,config.projectName)
  if(info.configured||runs.some(run=>run.projectId===id))panels.Local.append(remove)
  dialogBody.append(notice)
  const tools=document.createElement('div');tools.className='deployment-actions'
  form.onsubmit=async event=>{
   event.preventDefault();save.disabled=true
   try{
    await api.mapProject({projectId:id,path:path.value,useWorktrees,inheritWorktrees,parallelism,aiCommandTemplate:command.value,aiCommandTemplateAutonomous:autonomousCommand.value,inheritCommand})
    projectStateVersion++;disconnectedProjects.delete(id)
    notice.textContent='Local configuration saved';await loadProjects()
    for(const button of tools.querySelectorAll('button'))button.disabled=false
   }catch(err){notice.textContent=err.message}finally{save.disabled=false}
  }
  for(const [action,title] of [['skills','Deploy server skills'],['framework','Deploy SDD framework']]){
   const button=document.createElement('button');button.textContent=title;button.disabled=!info.configured
   button.onclick=async()=>{
    for(const item of tools.querySelectorAll('button'))item.disabled=true
    notice.textContent='Deployment in progress…'
    try{const result=await api.deployProject(id,action);notice.textContent=result.message||'Deployment complete'}
    catch(err){notice.textContent=err.message}finally{for(const item of tools.querySelectorAll('button'))item.disabled=false}
   };tools.append(button)
  }
  panels.Deployment.append(tools)
  function renderServer(monoRepo){
   panels.Server.replaceChildren()
  const readOnly=document.createElement('p');readOnly.textContent='Server configuration · Read only';panels.Server.append(readOnly)
  const metadata=document.createElement('dl')
  for(const [label,value] of [['Repository',config.gitRemoteUrl||'Not configured'],['Repository layout',monoRepo?'Mono-repo':'Multi-repo'],['SDD framework',config.specFramework||'Not configured']]){
   const term=document.createElement('dt'),description=document.createElement('dd');term.textContent=label;description.textContent=value;metadata.append(term,description)
  }
  panels.Server.append(metadata)
  for(const skill of config.skills||[]){
   const details=document.createElement('details'),summary=document.createElement('summary'),content=document.createElement('pre')
   summary.textContent=skill.command||skill.id;content.textContent=skill.content;details.append(summary,content);panels.Server.append(details)
  }

  }
  renderServer(info.monoRepo)
  const reload=document.createElement('button');reload.type='button';reload.textContent='Refresh from server';reload.className='refresh-project'
  tabs.before(reload)
  reload.onclick=async()=>{
   reload.disabled=true;notice.textContent='Refreshing server settings…'
   try{
    const fresh=await api.project(id)
    if(!reload.isConnected)return
    config=fresh.server
    dialogBody.querySelector('h2').textContent=config.projectName
    if(inheritWorktrees)useWorktrees=!!config.useWorktrees
    if(inheritCommand){command.value=config.aiCommandTemplate||'';autonomousCommand.value=config.aiCommandTemplateAutonomous||''}
    update();commandState();renderServer(fresh.monoRepo)
    notice.textContent='Server settings refreshed. Local overrides preserved.'
   }catch(err){notice.textContent=err.message}finally{reload.disabled=false}
  }

 }catch(err){
  paragraph(err.message)
  if(!hiddenProject(id)&&(projects.some(project=>project.id===id&&project.path)||runs.some(run=>run.projectId===id))){
   const remove=document.createElement('button');remove.textContent='Remove from desktop'
   remove.onclick=()=>requestRemoveProject(id,projects.find(project=>project.id===id)?.name||id)
   dialogBody.append(remove)
  }
 }
}

const iconPaths={
 stop:'<rect x="6" y="6" width="12" height="12" rx="1" fill="currentColor" stroke="none"/>',
 configure:'<path d="M4 7h16M4 17h16"/><circle cx="9" cy="7" r="3"/><circle cx="15" cy="17" r="3"/>',
 'start-agent':'<path d="m8 5 11 7-11 7Z"/>',
 shutdown:'<rect x="6" y="6" width="12" height="12" rx="1"/>',
 restart:'<path d="M20 7v5h-5M20 12a8 8 0 1 0-2 5M20 7v5"/>',
 profile:'<circle cx="12" cy="8" r="4"/><path d="M4 21v-2a8 8 0 0 1 16 0v2"/>'
}
for(const [id,paths] of Object.entries(iconPaths)){
 document.getElementById(id).innerHTML='<svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">'+paths+'</svg>'
}
if(localStorage.getItem('sidebarCollapsed')==='true')document.querySelector('#workspace').classList.add('sidebar-hidden')
renderSidebarToggle(document.querySelector('#workspace').classList.contains('sidebar-hidden'))
document.querySelector('#start-agent').onclick=async()=>{
 closeLogs(false)
 await settingsReady
 document.querySelector('#setup').hidden=false
 document.querySelector('#workspace').hidden=true
 const form=document.querySelector('#start')
 if(form.reportValidity())form.requestSubmit()
}

function newProjectTask(projectID){
 selectedProject=projectID
 showDialog('New task')
 const existing=document.createElement('button');existing.className='discovered-project';existing.textContent='Run an existing ticket'
 existing.onclick=()=>browseTasks(projectID)
 const create=document.createElement('button');create.className='discovered-project';create.textContent='Quick add task'
 create.onclick=()=>quickAdd(projectID)
 dialogBody.append(existing,create);existing.focus()
}

async function browseTasks(projectID,initialQuery=''){
 selectedProject=projectID
 showDialog('Launch task')
 const search=document.createElement('form'),query=document.createElement('input'),submit=document.createElement('button'),list=document.createElement('div')
 query.placeholder='Search by title or task key';query.setAttribute('aria-label','Search server tasks');submit.textContent='Search'
 query.value=initialQuery
 search.append(query,submit);dialogBody.append(search,list)
 query.focus()
 let generation=0
 async function load(){
  const current=++generation,searchText=query.value.trim()
  const isCurrent=()=>current===generation&&list.isConnected&&dialog.open
  list.textContent='Loading open tasks…';list.setAttribute('aria-busy','true')
  try{
   const [tasks,info]=await Promise.all([api.serverTasks(projectID,searchText,true),api.project(projectID)])
   if(!isCurrent())return
   list.replaceChildren()
   const launchableTasks=tasks.filter(task=>!isFinishedTask(task))
   if(!launchableTasks.length){const empty=document.createElement('p');empty.textContent=searchText?'No matching open tasks':'No open tasks in this project';list.append(empty)}
   if(!info.configured){const notice=document.createElement('p');notice.textContent='Configure a local repository before launching tasks.';list.append(notice)}
   for(const task of launchableTasks){
    const card=document.createElement('section');card.className='server-task'
    const title=document.createElement('strong');title.textContent=(task.key||task.id)+' · '+task.title
    const status=document.createElement('p');status.className='task-server-status';status.textContent='Current state: '+taskStage(task)+(task.trackerStatus?' · '+task.trackerStatus:'')
    const skill=document.createElement('select');skill.setAttribute('aria-label','Skill for '+(task.key||task.id))
    for(const item of info.server.skills||[]){const option=document.createElement('option');option.value=item.id;option.textContent=item.command||item.id;skill.append(option)}
    const discuss=document.createElement('option');discuss.value='discuss';discuss.textContent='Discussion (no skill)';skill.append(discuss)
    const custom=document.createElement('option');custom.value='custom';custom.textContent='Custom instructions';skill.append(custom)
    if((info.server.skills||[]).some(item=>item.id==='pickup'))skill.value='pickup'
    const prompt=document.createElement('textarea');prompt.placeholder='What should the agent do?';prompt.setAttribute('aria-label','Custom instructions');prompt.hidden=true
    skill.onchange=()=>{prompt.hidden=skill.value!=='custom'}
    const mode=modeSelect(document,'Execution mode for '+(task.key||task.id))
    const launch=document.createElement('button');launch.textContent='Launch';launch.disabled=!info.configured||!skill.options.length
    const notice=document.createElement('p');notice.setAttribute('role','status')
    launch.onclick=async()=>{
     if(skill.value==='custom'&&!prompt.value.trim()){notice.textContent='Enter custom instructions.';prompt.focus();return}
     launch.disabled=true;notice.textContent='Submitting execution…'
     try{await api.launchServerTask(projectID,task.id,skill.value,prompt.value,launchModeOverride(mode.value));notice.textContent='Execution submitted';dialog.close();await refresh()}
     catch(err){notice.textContent=err.message;launch.disabled=false}
    }
    card.append(title,status,skill,prompt,mode,launch,notice);list.append(card)
   }
  }catch(err){if(isCurrent()){const message=document.createElement('p');message.setAttribute('role','alert');message.textContent='Could not load open tasks: '+err.message+'. Use Search to retry.';list.replaceChildren(message)}}
  finally{if(isCurrent())list.setAttribute('aria-busy','false')}
 }
 search.onsubmit=event=>{event.preventDefault();load()}
 await load()
}

document.querySelector('#rerun').onclick=async()=>{
 const run=runs.find(item=>item.id===selected)
 if(!run||!['completed','failed','canceled'].includes(run.status))return
 if(freeConsole(run)){openAgentConsole(run.projectId,run.provider);return}
 showDialog('Relaunch '+(run.taskKey||run.taskId))
 try{
  const info=await api.project(run.projectId)
  const form=document.createElement('form')
  const skillLabel=document.createElement('label');skillLabel.textContent='Skill'
  const skill=document.createElement('select');skill.setAttribute('aria-label','Relaunch skill')
  for(const item of info.server.skills||[]){
   const option=document.createElement('option');option.value=item.id;option.textContent=item.command||item.id;skill.append(option)
  }
  const discuss=document.createElement('option');discuss.value='discuss';discuss.textContent='Discussion (no skill)';skill.append(discuss)
  const custom=document.createElement('option');custom.value='custom';custom.textContent='Custom instructions';skill.append(custom)
  skill.value=run.skill
  if(!skill.value){
   const missing=document.createElement('option');missing.value='';missing.textContent='Select a skill (previous skill unavailable)';missing.disabled=true;skill.prepend(missing);skill.value=''
  }
  skill.required=true;skillLabel.append(skill)
  const promptLabel=document.createElement('label');promptLabel.textContent='Instructions'
  const prompt=document.createElement('textarea');prompt.className='cli-command';prompt.setAttribute('aria-label','Relaunch instructions');prompt.value=run.prompt||''
  promptLabel.append(prompt)
  const modeLabel=document.createElement('label');modeLabel.textContent='Execution mode'
  const mode=modeSelect(document,'Relaunch execution mode');modeLabel.append(mode)
  const submit=document.createElement('button');submit.textContent='Launch new execution';submit.disabled=!info.configured
  const notice=document.createElement('p');notice.setAttribute('role','status')
  if(!info.configured)notice.textContent='Configure a local repository before relaunching.'
  form.append(skillLabel,promptLabel,modeLabel,submit,notice);dialogBody.append(form)
  form.onsubmit=async event=>{
   event.preventDefault()
   if(skill.value==='custom'&&!prompt.value.trim()){notice.textContent='Enter custom instructions.';prompt.focus();return}
   submit.disabled=true
   try{
    await api.launchServerTask(run.projectId,run.taskId,skill.value,prompt.value,launchModeOverride(mode.value))
    dialog.close();await refresh()
   }catch(err){notice.textContent=err.message;submit.disabled=false}
  }
 }catch(err){paragraph(err.message)}
}

function prLabel(value){
 try{
  const url=new URL(value)
  const match=url.pathname.match(/\/(pull|merge_requests)\/(\d+)/)
  return match?(match[1]==='merge_requests'?'MR !':'PR #')+match[2]:'PR / MR'
 }catch{return 'PR / MR'}
}
async function refreshPRs(executions){
 executions=executions.filter(run=>!freeConsole(run))
 if(linksLoading||Date.now()-lastLinksRefresh<15000)return
 linksLoading=true;lastLinksRefresh=Date.now()
 try{
  await Promise.all([...new Set(executions.map(run=>run.projectId))].map(async projectID=>{
   try{
    const tasks=await api.serverTasks(projectID,'')
    for(const run of executions.filter(run=>run.projectId===projectID)){
     const task=tasks.find(task=>task.id===run.taskId)
     if(task?.title?.trim())taskTitles.set(run.taskId,task.title.trim())
     else taskTitles.delete(run.taskId)
     if(task?.prUrl&&/^https?:\/\//i.test(task.prUrl))pullRequests.set(run.taskId,task.prUrl)
     else pullRequests.delete(run.taskId)
    }
   }catch{}
  }))
  render()
 }finally{linksLoading=false}
}

function taskMenu(run){
 showDialog(taskState(run).name||run.taskKey||run.taskId||runLabel(run))
 const related=()=>runs.filter(item=>taskKey(item)===taskKey(run))
 const relaunch=document.createElement('button');relaunch.textContent='Relaunch';relaunch.disabled=related().some(activeRun)
 relaunch.onclick=()=>{select(run);document.querySelector('#rerun').click()}
 const rename=document.createElement('form'),name=document.createElement('input'),save=document.createElement('button')
 name.setAttribute('aria-label','Local task name');name.value=taskState(run).name||run.taskKey||run.taskId||runLabel(run);name.maxLength=120;name.required=true
 save.textContent='Rename locally';rename.append(name,save)
 rename.onsubmit=event=>{event.preventDefault();if(!name.value.trim())return;localTasks[taskKey(run)]={...taskState(run),name:name.value.trim()};saveLocalTasks();dialog.close();render()}
 const archive=document.createElement('button');archive.textContent='Archive'
 archive.onclick=()=>requestArchive(run)
 dialogBody.append(relaunch,rename,archive)
}
function requestArchive(run){
 const active=runs.filter(item=>taskKey(item)===taskKey(run)&&activeRun(item))
 if(active.length){
  showDialog('Stop and archive task?')
  paragraph('This task has active executions. They must stop before it can be archived locally.')
  const confirm=document.createElement('button');confirm.textContent='Stop and archive';dialogBody.append(confirm)
  confirm.onclick=async()=>{confirm.disabled=true;try{await Promise.all(active.map(item=>api.stop(item.id)));await archiveTask(run)}catch(err){paragraph(err.message);confirm.disabled=false}}
 }else archiveTask(run).catch(error)
}
async function archiveTask(run){
 const latest=await api.runs()
 const related=latest.filter(item=>taskKey(item)===taskKey(run))
 if(related.some(activeRun))throw Error('An execution is still active. Stop it before archiving.')
 localTasks[taskKey(run)]={...taskState(run),archivedRuns:related.map(item=>item.id)}
 saveLocalTasks()
 const current=runs.find(item=>item.id===selected)
 if(current&&taskKey(current)===taskKey(run)){
  selected=null;terminal.reset();await api.detach()
  renderHeader();document.querySelector('#directory').textContent=''
 }
 runs=latest;last=JSON.stringify(latest);dialog.close();render()
}

const sidebarHandle=document.querySelector('#sidebar-resizer')
function sidebarWidth(value){
 const width=Math.max(210,Math.min(value,Math.min(520,window.innerWidth-400)))
 document.querySelector('aside').style.width=width+'px'
 sidebarHandle.setAttribute('aria-valuenow',String(width))
 localStorage.setItem('sidebarWidth',String(width));resize()
}
sidebarWidth(Number(localStorage.getItem('sidebarWidth'))||280)
sidebarHandle.onpointerdown=event=>{sidebarHandle.setPointerCapture(event.pointerId);document.body.classList.add('resizing-sidebar')}
sidebarHandle.onpointermove=event=>{if(sidebarHandle.hasPointerCapture(event.pointerId))sidebarWidth(event.clientX)}
sidebarHandle.onpointerup=event=>{sidebarHandle.releasePointerCapture(event.pointerId);document.body.classList.remove('resizing-sidebar')}
sidebarHandle.onlostpointercapture=()=>document.body.classList.remove('resizing-sidebar')
sidebarHandle.onkeydown=event=>{if(event.key==='ArrowLeft'||event.key==='ArrowRight'){event.preventDefault();sidebarWidth(document.querySelector('aside').getBoundingClientRect().width+(event.key==='ArrowRight'?20:-20))}}
document.querySelector('#toggle-sidebar').innerHTML='<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16m7-12-4 4 4 4"/></svg>'

function openCommandPalette(){
 showDialog('Commands')
 const filter=document.createElement('input');filter.placeholder='Search actions…';filter.setAttribute('aria-label','Search commands')
 const quick=document.createElement('button');quick.className='discovered-project';quick.textContent='Quick add task'
 quick.onclick=()=>quickAdd()
 filter.oninput=()=>{quick.hidden=!('quick add task'.includes(filter.value.toLowerCase()))}
 filter.onkeydown=event=>{if(event.key==='Enter'&&!quick.hidden){event.preventDefault();quickAdd()}}
 dialogBody.append(filter,quick);filter.focus()
}
document.querySelector('#command-palette').onclick=openCommandPalette
window.addEventListener('keydown',event=>{
 if((event.metaKey||event.ctrlKey)&&event.key.toLowerCase()==='k'){event.preventDefault();event.stopPropagation();openCommandPalette()}
},true)
async function quickAdd(projectID=selectedProject){
 showDialog('Quick add task')
 const form=document.createElement('form'),projectLabel=document.createElement('label'),project=document.createElement('select')
 projectLabel.textContent='Project';project.setAttribute('aria-label','Quick add project')
 try{await loadProjects()}catch(err){paragraph(err.message);return}
 for(const item of projects){const option=document.createElement('option');option.value=item.id;option.textContent=item.name;project.append(option)}
 project.value=projectID||''
 if(!project.value){const empty=document.createElement('option');empty.value='';empty.textContent='Select a project';project.prepend(empty);project.value=''}
 project.required=true;projectLabel.append(project)
 const title=document.createElement('input');title.placeholder='Task title';title.setAttribute('aria-label','Task title');title.required=true;title.maxLength=500
 const description=document.createElement('textarea');description.className='cli-command';description.placeholder='Description (optional)';description.setAttribute('aria-label','Task description')
 const submit=document.createElement('button');submit.textContent='Create task'
 const notice=document.createElement('p');notice.setAttribute('role','status')
 form.append(projectLabel,title,description,submit,notice);dialogBody.append(form);title.focus()
 form.onsubmit=async event=>{
  event.preventDefault();if(!title.value.trim())return;submit.disabled=true
  notice.textContent='Creating task on the server and configured tracker…'
  try{
   const task=await api.createTask({projectID:project.value,title:title.value.trim(),description:description.value})
   selectedProject=project.value
   form.replaceChildren()
   notice.textContent='Created '+(task.key||task.id)+' · '+task.title
   const launch=document.createElement('button');launch.type='button';launch.textContent='Launch task'
   launch.onclick=()=>browseTasks(task.projectId,task.key||task.title)
   form.append(notice,launch)
  }catch(err){notice.textContent=err.message;submit.disabled=false}
 }
}

function isFinishedTask(task){
 return ['finished','done'].includes(task.status)||(task.labels||[]).some(label=>label.trim().replace(/^#/,'').toLowerCase()==='finished')
}

function currentTaskRun(){return runs.find(run=>run.id===selected)}
function renderNextStep(){
 const run=currentTaskRun(),status=document.querySelector('#next-step-status'),button=document.querySelector('#next-step'),retry=document.querySelector('#retry-next-step')
 button.hidden=true;button.disabled=true;retry.hidden=true
 if(!run){status.textContent='Select a task to see its next step';return}
 if(freeConsole(run)){status.textContent='Free agent console · '+(run.cancelRequested?'Stopping':run.status);return}
 const key=taskKey(run)
 if(!nextStepData||nextStepData.key!==key){status.textContent='Loading task workflow…';return}
 if(nextStepData.error){status.textContent=nextStepData.error;retry.hidden=false;return}
 const step=nextStepData.step
 const busy=runs.some(item=>taskKey(item)===key&&activeRun(item))
 const submitted=submittedSteps.get(key)
 if(submitted&&(submitted.skillId!==step.skillId||runs.some(item=>taskKey(item)===key&&!submitted.runIds.includes(item.id))))submittedSteps.delete(key)
 const pending=submittingSteps.has(key)||submittedSteps.has(key)
 const message=submittingSteps.has(key)?'Submitting execution…':pending?'Execution submitted; waiting for its console':busy?'Execution in progress':nextStepErrors.get(key)||step.message
 status.textContent=(nextStepData.task.key||run.taskKey||run.taskId)+' · '+step.stage+' · '+message
 if(step.skillId){button.hidden=false;button.textContent='Next: '+step.label;button.disabled=busy||pending}
}
new ResizeObserver(resize).observe(document.querySelector('#task-status'))
new ResizeObserver(resize).observe(document.querySelector('#toolbar'))
async function readNextStep(run){
 const [tasks,project]=await Promise.all([api.serverTasks(run.projectId,run.taskKey||run.taskId),api.project(run.projectId)])
 const task=tasks.find(task=>task.id===run.taskId)
 if(!task)throw Error('Task workflow unavailable. Refresh to try again.')
 return {key:taskKey(run),task,step:nextTaskStep(task,project)}
}
async function refreshNextStep(){
 const run=currentTaskRun()
 if(run&&submittingSteps.has(taskKey(run)))return
 const generation=++nextStepGeneration
 nextStepUpdated=Date.now()
 if(!run||freeConsole(run)){nextStepData=null;renderNextStep();return}
 if(nextStepData?.key!==taskKey(run))nextStepData=null
 renderNextStep()
 try{
  const data=await readNextStep(run)
  if(generation===nextStepGeneration)nextStepData=data
 }catch(err){if(generation===nextStepGeneration)nextStepData={key:taskKey(run),error:'Cannot load next step: '+err.message}}
 if(generation===nextStepGeneration)renderNextStep()
}
document.querySelector('#retry-next-step').onclick=refreshNextStep
document.querySelector('#next-step').onclick=async()=>{
 const run=currentTaskRun(),displayed=nextStepData
 if(!run||displayed?.key!==taskKey(run)||!displayed.step?.skillId)return
 const key=taskKey(run)
 if(submittingSteps.has(key)||submittedSteps.has(key)||runs.some(item=>taskKey(item)===key&&activeRun(item)))return
 nextStepGeneration++;nextStepErrors.delete(key);submittingSteps.add(key);renderNextStep()
 try{
  const [fresh,latestRuns]=await Promise.all([readNextStep(run),api.runs()])
  if(taskKey(currentTaskRun()||{})!==key)return
  nextStepData=fresh
  if(fresh.step.skillId!==displayed.step.skillId||latestRuns.some(item=>taskKey(item)===key&&activeRun(item))){await refresh();return}
  await api.launchServerTask(run.projectId,run.taskId,fresh.step.skillId,'')
  submittedSteps.set(key,{skillId:fresh.step.skillId,runIds:latestRuns.filter(item=>taskKey(item)===key).map(item=>item.id)})
  await refresh()
  if(taskKey(currentTaskRun()||{})===key){
   const launched=runs.find(item=>taskKey(item)===key&&!latestRuns.some(previous=>previous.id===item.id))
   if(launched)select(launched)
  }
 }catch(err){nextStepErrors.set(key,'Could not launch next step: '+err.message)}finally{submittingSteps.delete(key);renderNextStep()}
}

async function openAgentConsole(projectID,previousProvider){
 showDialog('Open agent console')
 paragraph('Start an interactive agent in this project’s local repository. Enter your instructions directly in its console.')
 const form=document.createElement('form'),label=document.createElement('label'),provider=document.createElement('select')
 label.textContent='Agent';provider.setAttribute('aria-label','Console agent')
 for(const [value,text] of [['codex','Codex'],['claude','Claude']]){const option=document.createElement('option');option.value=value;option.textContent=text;provider.append(option)}
 label.append(provider)
 const location=document.createElement('p');location.textContent=projects.find(project=>project.id===projectID)?.path||''
 const launch=document.createElement('button');launch.textContent='Open console'
 const notice=document.createElement('p');notice.setAttribute('role','status')
 form.append(label,location,launch,notice);dialogBody.append(form)
 const pendingInfo=api.project(projectID).then(info=>{if(form.isConnected){const initial=previousProvider||info.server?.aiProvider;if(['codex','claude'].includes(initial))provider.value=initial}}).catch(()=>{})
 launch.disabled=true;await pendingInfo;launch.disabled=false
 form.onsubmit=async event=>{
  event.preventDefault();if(launch.disabled)return
  launch.disabled=true;notice.textContent='Opening console…'
  try{
   const run=await api.launchConsole(projectID,provider.value)
   collapsedProjects.delete(projectID);queueProjects.delete(projectID)
   if(!runs.some(item=>item.id===run.id))runs.push(run)
   dialog.close();select(run);await refresh()
  }catch(err){notice.textContent=err.message;launch.disabled=false}
 }
}
