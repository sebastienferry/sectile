import { currentPullRequest, renderPullRequestIndicator } from './pullRequests.mjs'
import { installTooltips } from './tooltips.js'
import {mcpSettings} from './mcp-settings.mjs'
import { logText } from './log-text.mjs'
import { createGitDiff } from './gitDiff.js'

import { skillResult } from './skill-result.mjs'
import { orderedQueueRuns } from './queue.mjs'
import { orderedTaskGroups } from './task-order.mjs'
import { transitions, announce } from './notifications.mjs'
import { runStateOf, runStateLabel, runStateSvg } from '../../shared/runStates.ts'
import { isMacPlatform, sidebarShortcutAction, sidebarShortcutAria, sidebarShortcutLabel } from '../../shared/sidebarShortcut.mjs'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import './style.css'
import { taskStage, nextTaskStep, skillLabel } from './workflow.mjs'
import { launchModeOverride, modeSelect } from './skill-mode.mjs'
import { orderedTasks, nextSort, DEFAULT_SORT, SORTABLE_FIELDS } from './task-list-order.mjs'
import { consoleNotice, needsConsoleNotice, readOnlyConsole } from './run-console.mjs'
import { previewLines } from './command-preview.mjs'
import { EDITORS, editorChoice, editorLabel } from './editors.mjs'
import { PROVIDERS, DEFAULT_PROVIDER, SETUP_PROVIDERS, projectFields, ownEntries, compact, parseModelList, sourceHint, describe, ipcMessage, agentUnreachable, validSkillCommand, workstationPayload } from './execution-fields.mjs'
import { runEngine } from './run-engine.mjs'
import { nextEngine, taskEngine, engineMark, engineTooltip, moveEngine, removalImpact, removalMessage } from './engines.mjs'
import { pollAction } from './agent-poll.mjs'
import { offerFor, initializedNotice } from './git-init.mjs'
// The repository changelog, inlined by Vite at build time. The app reads it
// with no network at all: the renderer's content security policy forbids any
// outgoing connection, and the release notes have to stay readable with the
// agent stopped.
import changelogSource from '../../CHANGELOG.md?raw'
import { parseChangelog, releaseNotesFor } from './changelog.mjs'
import { APPEARANCE_CHOICES, terminalTheme } from './appearance.mjs'
const api=window.localAgent
// Concurrent execution workers ceiling per project, aligned with agentconfig.MaxParallelism.
// Parallelism is a workstation setting: the server neither stores nor supplies it.
const MAX_PARALLELISM=10
document.querySelector('#app').innerHTML=`
<header><div><button id="toggle-sidebar" aria-expanded="true"></button><strong id="app-title">Sectile Desktop</strong><small>Execution consoles</small></div><button id="command-palette" title="Commands (⌘K / Ctrl+K)">⌘K</button></header>
<section id="setup" hidden><div class="setup-toolbar"><button id="setup-logs" type="button" title="View local-agent diagnostics">Agent logs</button></div><div id="agent-offline" role="status" hidden><strong>Local agent is stopped</strong><p>Start the agent to run tasks and access your local consoles.</p></div><h1>Connect to Sectile</h1><p>In the Sectile web interface, under your profile, choose <strong>Pair a workstation</strong> and paste the code here. A code is single use and expires within ten minutes; this machine keeps the credential it receives, so the code is never needed again.</p>
<form id="start"><label>Sectile server<input name="server" type="url" value="http://localhost:8090" required></label><label>Pairing code<input name="code" type="text" autocomplete="off" spellcheck="false" placeholder="Paste the code from the web interface"></label><button>Connect</button></form></section>
<main id="workspace" hidden><aside><div class="sidebar-scroll"><div class="section">PROJECTS <button id="add-project" title="Add a remote project">+</button></div><div id="runs"></div><button id="clear-history" class="icon-button" type="button" aria-label="Clear finished consoles" title="Clear finished consoles" disabled></button></div><footer class="sidebar-footer"><span id="connection" data-state="off">Connecting…</span><nav aria-label="Local agent controls"><button id="shutdown" class="icon-button" aria-label="Stop agent" title="Stop agent" hidden></button><button id="restart" class="icon-button" aria-label="Restart agent" title="Restart agent" hidden></button></nav><button id="settings" class="icon-button" type="button" aria-label="Settings" title="Settings"></button></footer></aside><div id="sidebar-resizer" role="separator" aria-label="Resize sidebar" aria-orientation="vertical" tabindex="0"></div><article><div id="toolbar"><div class="toolbar-identity"><div class="terminal-title-line"><span id="run-state" class="run-state header-state" hidden></span><strong id="title">Select an execution</strong><span id="skill-result" role="status" hidden></span><span id="native-terminal-badge" class="native-terminal-badge" hidden></span></div><div class="worktree-line"><button id="worktree" class="worktree" type="button" title="Copy this path" hidden><svg class="worktree-icon" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 20a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2z"/></svg><span id="directory"></span></button><button id="open-editor" class="icon-button" type="button" hidden></button><span id="worktree-copied" class="worktree-copied" role="status"></span></div></div><div class="toolbar-actions"><select id="execution-history" aria-label="Execution history" hidden></select><div class="execution-views" role="group" aria-label="Execution view"><button id="view-console" class="icon-button" type="button" aria-label="Console" title="Console" aria-pressed="true" disabled></button><button id="view-changes" class="icon-button" type="button" aria-label="Changes" title="Changes" aria-pressed="false" disabled></button></div><button id="selected-pr" class="icon-button" type="button" hidden></button><button id="detach-terminal" class="icon-button" type="button" aria-label="Detach to native terminal" title="Detach to native terminal" hidden></button><button id="rerun" class="icon-button" type="button" aria-label="Relaunch" title="Relaunch" hidden></button><button id="save-log" class="icon-button" type="button" aria-label="Export log" title="Export log"></button><button id="stop" class="icon-button" type="button" aria-label="Stop execution" title="Stop execution" disabled></button><button id="next-step" class="icon-button" type="button" hidden disabled></button><button id="pickup-chain" class="icon-button" type="button" aria-label="Pickup (full chain)" title="Pickup (full chain)" hidden disabled></button><span id="next-step-label" class="step-badge" aria-hidden="true" hidden></span><button id="mark-reviewed" type="button" class="secondary" hidden>Mark reviewed</button><button id="retry-next-step" type="button" title="Retry reading the task workflow" hidden>Retry</button><button id="force-next-step" type="button" class="secondary" title="Launch although a run is already active on this task" hidden>Launch anyway</button></div></div><section id="changes" aria-label="Worktree changes" hidden></section><div id="terminal"></div><footer id="task-status"><span id="next-step-status" role="status" aria-live="polite">Select a task to see its next step</span></footer></article><section id="tickets-pane" aria-label="Tickets" hidden></section></main>
<dialog id="project-dialog"><button id="close-dialog" class="icon-button" type="button" aria-label="Close" title="Close"><svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18"/></svg></button><div id="dialog-body"></div><div class="dialog-footer" hidden></div></dialog><div id="error" role="alert"></div>`
installTooltips()
// The console shows a prompt the user configured elsewhere - oh-my-posh, starship, powerlevel10k -
// and those draw their separators and icons from the Private Use Area. Menlo is a macOS font, so on
// Windows every one of those glyphs fell back to a replacement box. The Mono variants are the ones
// that keep a glyph to a single cell, which is what the grid needs, and Symbols Nerd Font Mono sits
// near the end as a per-glyph fallback: a host with no patched font still gets the icons.
const TERMINAL_FONT='"FiraCode Nerd Font Mono", "JetBrainsMono Nerd Font Mono", "Hack Nerd Font Mono", "CaskaydiaCove Nerd Font Mono", "MesloLGS NF", Menlo, Consolas, "Symbols Nerd Font Mono", monospace'
// The main process sets the appearance through nativeTheme, which is what this
// query answers: the terminal follows it like the stylesheet does, live.
const darkScheme=window.matchMedia('(prefers-color-scheme: dark)')
const terminal=new Terminal({cursorBlink:true,fontSize:13,fontFamily:TERMINAL_FONT,scrollback:20000,theme:terminalTheme(darkScheme.matches)})
darkScheme.addEventListener('change',event=>{terminal.options.theme=terminalTheme(event.matches)})
const fit=new FitAddon();terminal.loadAddon(fit)
let nextStepData=null,nextStepGeneration=0,nextStepUpdated=0
const submittingSteps=new Map()
const submittedSteps=new Map()
const nextStepErrors=new Map()
// Task keys whose last launch was refused because a run is already active on
// them. Only that refusal is forceable, so the offer is keyed on the marker the
// server puts in its body, never on the 409 status alone: a finished task and a
// disconnected agent answer 409 too, and neither is overridable.
const forceableLaunches=new Map()
// The renderer sees the refusal as a message: the structured body crosses the
// IPC bridge inside the error text. Reading the marker back means finding the
// JSON object in it, and giving up quietly when there is none.
function refusedActiveRun(message){
 const start=String(message||'').indexOf('{')
 if(start<0)return null
 try{
  const body=JSON.parse(String(message).slice(start,String(message).lastIndexOf('}')+1))
  return body&&body.activeRunId?body:null
 }catch{return null}
}
const taskTitles=new Map()
const skillResults=new Map(),loadingSkillResults=new Set()
const pullRequests=new Map()
let localTasks={}
try{localTasks=JSON.parse(localStorage.getItem('localTasks')||'{}')}catch{}
const freeConsole=run=>run?.kind==='console'
// Task runs show the skill and engine; project prompts show their engine name.
const runLabel=run=>freeConsole(run)?(run.engineName||run.provider||'AI')+' · Project prompt':(runEngine(run)?run.skill+' · '+runEngine(run):run.skill)
// A macro skill run has no task: its executions group under the macro.
const macroRun=run=>!!run?.macroKey
const taskKey=run=>JSON.stringify([run.projectId,freeConsole(run)?run.id:macroRun(run)?'macro:'+run.macroKey:run.taskId])
const activeRun=run=>['running','queued','preparing','waiting'].includes(run.status)
const taskState=run=>localTasks[taskKey(run)]||{}
function formatTerminalName(term){
 if(!term)return 'terminal'
 const lower=term.toLowerCase().trim()
 if(lower==='ghostty')return 'Ghostty'
 if(lower==='iterm'||lower==='iterm2')return 'iTerm'
 if(lower==='terminal'||lower==='apple-terminal'||lower==='terminal.app')return 'Terminal'
 if(lower==='wt'||lower==='windows-terminal'||lower==='wt.exe')return 'Windows Terminal'
 if(lower==='cmd'||lower==='cmd.exe')return 'Command Prompt'
 return term
}
let disconnectedProjects=new Set(),projectStateVersion=0,refreshing=false
// A background refresh must not reorder or rebuild the task list under the
// user's pointer: it is deferred until the interaction ends.
let pendingRender=false
const sidebarList=()=>document.querySelector('#runs')
// Only the task rows are held: project headings and the queue view keep
// refreshing normally.
// An open project menu counts too: a render would rebuild the row and drop it.
const sidebarBusy=()=>!!document.querySelector('#runs .local-task:hover')||!!document.activeElement?.closest?.('#runs .local-task')||!!document.querySelector('#runs .project-menu:not([hidden])')
const hiddenProject=id=>disconnectedProjects.has(id)
const hiddenRun=run=>hiddenProject(run.projectId)||(taskState(run).archivedRuns||[]).includes(run.id)&&!activeRun(run)
function saveLocalTasks(){localStorage.setItem('localTasks',JSON.stringify(localTasks))}
// The name a task row shows: the local name, else the tracker title, else the run label.
const displayedName=run=>taskState(run).name||taskTitles.get(run.taskId)||runLabel(run)
// The task row whose title is edited inline (#513): its task key, what was typed so
// far, the field's selection, and whether the edition has just started. It lives
// outside the DOM because most render() calls rebuild the sidebar, not only the
// deferrable refresh, and the edition must survive them.
let renaming=null
// Set while render() empties the sidebar: removing the focused field may fire a
// blur, which must not be taken for the user leaving the field.
let rebuildingSidebar=false
const collapsedProjects=new Set(JSON.parse(localStorage.getItem('collapsedProjects')||'[]'))

const queueProjects=new Set()

let linksLoading=false,lastLinksRefresh=0
let selectedProject=null
let ticketsOpen=false,agentConnected=false
let updateSettingsConnection=null
let opened=false,selected=null,runs=[],last='',stopping=false,restarting=false,projects=[],projectsLoaded=false
const changes=createGitDiff({api,container:document.querySelector('#changes'),terminal:document.querySelector('#terminal'),consoleButton:document.querySelector('#view-console'),changesButton:document.querySelector('#view-changes'),onConsole:()=>{resize();if(opened)terminal.focus()}})
api.onOutput(data=>terminal.write(new Uint8Array(data)))
terminal.onData(data=>{if(!changes.active&&!ticketsOpen)api.input(data)})
function resize(){if(opened&&!changes.active&&!ticketsOpen){fit.fit();api.resize(terminal.cols,terminal.rows)}}
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
// The status is a dot and a word beside the controls it describes: a colour
// carries the state, the label names it, and the server address rides in the
// tooltip rather than filling the row.
const connectionDot=document.createElement('span');connectionDot.className='connection-dot';connectionDot.setAttribute('aria-hidden','true')
const connectionLabel=document.createElement('span');connectionLabel.className='connection-label'
// The dot sits inside the body rather than beside it, so a collapsed sidebar can
// drop the words and keep a target that still opens the board.
const connectionLink=document.createElement('a');connectionLink.className='connection-body';connectionLink.href='#'
connectionLink.onclick=event=>{event.preventDefault();api.openBoard().catch(error)}
const connectionPlain=document.createElement('span');connectionPlain.className='connection-body'
// Only what changed is written: a poll that rebuilt the row would take focus
// away from the link between two keystrokes.
function connectionLabelled(container,text,href){
 connectionLabel.textContent=text
 const body=href?connectionLink:connectionPlain
 if(href)connectionLink.href=href
 if(body.firstChild!==connectionDot)body.replaceChildren(connectionDot,connectionLabel)
 if(container.firstChild!==body||container.childNodes.length!==1)container.replaceChildren(body)
}
function connectionStatus(status){
 updateSettingsConnection?.(status)
 const container=document.querySelector('#connection')
 // A contract mismatch is not a dropped link: the server answers, but with a
 // build this agent cannot talk to. Reported as a plain disconnection it reads
 // as a network problem and nobody looks at the build, so it keeps a label of
 // its own and carries the agent's diagnosis in the tooltip.
 if(!status.connected){
  container.dataset.state='off'
  container.title=status.contractError||status.text||'The local agent does not reach the Sectile server.'
  connectionLabelled(container,status.contractError?'Server incompatible':'Not connected',null)
  return
 }
 let board=''
 try{
  const url=new URL(status.server)
  if(!['http:','https:'].includes(url.protocol)||url.username||url.password)throw Error('Invalid server URL')
  url.searchParams.delete('task');url.hash=''
  board=url.href
 }catch{board=''}
 container.dataset.state='on'
 container.title=board?'Open '+status.server+' in the default browser':status.server
 connectionLabelled(container,'Connected',board)
}
function agentUnavailable(){
 agentConnected=false
 changes.disconnect()
 document.querySelector('#start button').disabled=false
 document.querySelector('#start button').textContent='Start local agent'
 document.querySelector('#shutdown').hidden=true
 document.querySelector('#restart').hidden=true
 document.querySelector('#agent-offline').hidden=false
 closeTickets(false)
 document.querySelector('#setup').hidden=false
 document.querySelector('#workspace').hidden=true
 connectionStatus({text:'Local agent stopped'})
 projectsLoaded=false
 openEditorAvailable=false;renderOpenEditor()
 api.detach().catch(()=>{})
}
function ready(){
 agentConnected=true
 document.querySelector('#agent-offline').hidden=true
 if(!projectsLoaded){projectsLoaded=true;loadProjects().catch(()=>{projectsLoaded=false});loadEditorSetting()}
 document.querySelector('#start button').disabled=true;document.querySelector('#restart').hidden=false;document.querySelector('#shutdown').hidden=false
 document.querySelector('#setup').hidden=true;document.querySelector('#workspace').hidden=false
 if(!document.querySelector('#connection a'))connectionStatus({text:'Local agent connected'})
 if(!opened){terminal.open(document.querySelector('#terminal'));opened=true;resize()}
}
function select(run,background=false,options){
 if(hiddenProject(run.projectId))return
 if(!background)closeTickets(false)
 selectedProject=run.projectId
 selected=run.id
 changes.select(selected)

 refreshSkillResult()
 refreshNextStep()
 showDirectory(run.directory)
 document.querySelector('#stop').disabled=!activeRun(run)
 terminal.reset()
 hideRepositoryChoice()
 if(needsConsoleNotice(run)){
  api.detach().catch(error)
  terminal.writeln(consoleNotice(run))
  if(run.status==='waiting')showRepositoryChoice(run)
  render(options);return
 }
 api.attach(run.id).then(()=>{setTimeout(resize,150);if(!changes.active&&!ticketsOpen&&!readOnlyConsole(run))terminal.focus()}).catch(error)
 render(options)
}
// A launch the agent parked until its ticket is pinned to a repository (#456)
// is resumed from here: choosing a repository pins the ticket, and a
// repository not mapped yet is given its folder first. The agent resumes the
// same execution once it reads the pin back.
function showRepositoryChoice(run){
 hideRepositoryChoice()
 const bar=document.createElement('div');bar.id='repository-choice';bar.className='repository-choice'
 bar.setAttribute('role','group');bar.setAttribute('aria-label','Repository of this task')
 const label=document.createElement('span');label.textContent='Repository of '+(run.taskKey||run.taskId)
 const choice=document.createElement('select');choice.setAttribute('aria-label','Repository')
 const start=document.createElement('button');start.type='button';start.textContent='Start in this repository';start.disabled=true
 const status=document.createElement('span');status.setAttribute('role','status')
 bar.append(label,choice,start,status)
 document.querySelector('#terminal').before(bar)
 api.repositories(run.projectId).then(list=>{
  for(const repository of list){
   const option=document.createElement('option');option.value=repository.identity;option.dataset.path=repository.path||''
   option.textContent=repository.identity+(repository.path?'':' · choose its folder');choice.append(option)
  }
  start.disabled=!list.length
 }).catch(err=>{status.textContent=err.message})
 start.onclick=async()=>{
  const option=choice.selectedOptions[0];if(!option)return
  start.disabled=true
  try{
   let path=''
   if(!option.dataset.path){path=await api.chooseRepository();if(!path){start.disabled=false;return}}
   await api.mapRepository({projectId:run.projectId,repository:option.value,taskId:run.taskId,...(path?{path}:{})})
   status.textContent='Repository pinned · the execution starts shortly'
  }catch(err){status.textContent=err.message;start.disabled=false}
 }
}
function hideRepositoryChoice(){document.querySelector('#repository-choice')?.remove()}
// The state the user reads, drawn from the shared definition so the row, the
// execution queue and the banner the desktop raises cannot say three things.
// The glyph is decorative: the label carries the state for anyone who cannot
// resolve an amber hand, and the row tooltip repeats it. "Process:" tells it
// apart from the skill badge beside it, which answers another question.
function renderRunState(element,run){
 const state=runStateOf(run),label=runStateLabel(state)
 if(element.dataset.runState!==state){element.dataset.runState=state;element.innerHTML=runStateSvg(state,14)}
 element.title='Process: '+label;element.setAttribute('aria-label',element.title)
 return label
}
function renderQueue(project,group){
 const runsForProject=runs.filter(run=>run.projectId===project.id)
 const panel=document.createElement('section');panel.className='execution-queue';panel.setAttribute('aria-label','Execution queue for '+project.name)
 const header=document.createElement('div');header.className='queue-heading'
 const label=document.createElement('strong');label.textContent='Execution queue'
 const close=document.createElement('button');close.className='icon-button';close.type='button';close.title='Close queue';close.setAttribute('aria-label','Close queue for '+project.name)
 close.innerHTML='<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18"/></svg>'
 close.onclick=()=>{queueProjects.delete(project.id);render();document.querySelector('.project-more[data-project-id="'+CSS.escape(project.id)+'"]')?.focus()}
 header.append(label,close);panel.append(header)
 const summary=document.createElement('p');summary.className='queue-summary';summary.setAttribute('role','status')
 const list=document.createElement('div');list.className='queue-list';panel.append(summary,list);group.append(panel)
 const active=runsForProject.filter(run=>['running','preparing'].includes(run.status)&&!run.cancelRequested)
 const stopping=runsForProject.filter(run=>activeRun(run)&&run.cancelRequested)
 const waiting=orderedQueueRuns(runsForProject)
 const parked=runsForProject.filter(run=>run.status==='waiting'&&!run.cancelRequested)
 const text=active.length+' active · '+waiting.length+' waiting'+(parked.length?' · '+parked.length+' waiting for a repository':'')+(stopping.length?' · '+stopping.length+' stopping':'')
 if(summary.textContent!==text)summary.textContent=text
 if(!active.length&&!waiting.length&&!stopping.length&&!parked.length){
  const empty=document.createElement('p');empty.textContent='No active or queued executions';list.append(empty);return
 }
 for(const [label,items] of [['Waiting for a repository',parked],['Waiting · submission order',waiting],['Stopping / canceling',stopping],['Running / preparing',active]]){
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
 if(!run||freeConsole(run)||macroRun(run)||run.skill==='discuss'||loadingSkillResults.has(run.id)||loadingSkillResults.size>=4)return
 loadingSkillResults.add(run.id)
 try{
  const result=await api.runResult(run.id)
  // A null result means the agent no longer holds the run. That is expected once
  // the history is cleared or the agent restarted, and the next poll drops the
  // run; a run the latest poll still lists as live is the anomaly worth noting.
  if(result===null&&runs.includes(run)&&!['completed','failed','canceled'].includes(run.status))console.warn('The local agent reports no run '+run.id+' while the desktop still lists it as '+run.status)
  skillResults.set(run.id,result)
 }catch{skillResults.delete(run.id)}
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
  // No skill result is a state of its own: leaving the previous glyph in place
  // would keep showing a verdict that no longer holds, beside a run state that
  // has moved on.
  if(!result){badge.className='status task-skill-status';badge.textContent='';badge.removeAttribute('title');badge.removeAttribute('aria-label');continue}
  badge.className='status task-skill-status '+result.kind
  if(badge.textContent!==result.icon)badge.textContent=result.icon
  badge.title='Skill: '+result.label
  badge.setAttribute('aria-label',badge.title)
 }
}
// The worktree path is the one piece of the header a user needs elsewhere - in a
// shell, in an editor - so it is a control, not a caption: one click puts it on
// the clipboard, and the text stays selectable for anyone who copies by hand.
let copiedNotice
function showDirectory(value){
 const path=value||'',label=document.querySelector('#directory')
 // A confirmation belongs to the path it was given for: showing another one
 // leaves it pointing at a path nobody copied.
 if(label.textContent!==path)clearCopiedNotice()
 label.textContent=path
 const button=document.querySelector('#worktree')
 button.hidden=!path
 button.title=path?'Copy this path':''
 renderOpenEditor()
}
// The editor button (#535) exists once an editor is chosen in Settings and the
// agent can open one; it opens the path shown, which the agent resolves from
// the run, never from what the renderer sends.
let configuredEditor='',openEditorAvailable=false,openingEditor=false
function renderOpenEditor(){
 const button=document.querySelector('#open-editor')
 const label=configuredEditor&&'Open in '+editorLabel(configuredEditor)
 button.hidden=!document.querySelector('#directory').textContent||!configuredEditor||!openEditorAvailable
 button.title=label;button.setAttribute('aria-label',label)
 button.disabled=openingEditor
}
async function loadEditorSetting(){
 try{
  const [status,view]=await Promise.all([api.status(),api.workstationSettings()])
  openEditorAvailable=!!status.capabilities?.includes('open-editor')
  configuredEditor=String(view?.defaults?.editorCommand||'').trim()
 }catch{openEditorAvailable=false;configuredEditor=''}
 renderOpenEditor()
}
document.querySelector('#open-editor').onclick=async()=>{
 if(!selected||openingEditor)return
 openingEditor=true;renderOpenEditor()
 try{await api.openEditor(selected);document.querySelector('#error').textContent=''}
 catch(err){error(Error(ipcMessage(err).trim()))}
 finally{openingEditor=false;renderOpenEditor()}
}
function clearCopiedNotice(){
 clearTimeout(copiedNotice)
 document.querySelector('#worktree-copied').textContent=''
}
document.querySelector('#worktree').onclick=async()=>{
 const path=document.querySelector('#directory').textContent
 if(!path)return
 try{
  await api.copyText(path)
  clearTimeout(copiedNotice)
  document.querySelector('#worktree-copied').textContent='Copied'
  copiedNotice=setTimeout(clearCopiedNotice,2000)
 }catch(err){clearCopiedNotice();error(err)}
}
// The state before the title is the one the sidebar row and the notification
// already show, drawn from the shared definition, with its label spelled out:
// the header has the room the row does not.
function renderHeaderState(run){
 const element=document.querySelector('#run-state')
 element.hidden=!run
 if(!run)return
 const state=runStateOf(run),label=runStateLabel(state)
 if(element.dataset.runState!==state){
  element.dataset.runState=state
  element.innerHTML=runStateSvg(state,14)+'<span class="run-state-label"></span>'
 }
 const text=element.querySelector('.run-state-label')
 if(text.textContent!==label)text.textContent=label
 element.title='Process: '+label
}
function toggleQueue(projectID){
 selectedProject=projectID
 if(queueProjects.has(projectID))queueProjects.delete(projectID);else queueProjects.add(projectID)
 collapsedProjects.delete(projectID);localStorage.setItem('collapsedProjects',JSON.stringify([...collapsedProjects]));render()
}
// Every action of a project, in one context menu: a right click on the row
// opens it at the pointer, the … button under itself. The row itself carries
// nothing else, so the menu is the only place these actions live.
let closeProjectMenu=null
function projectMenu(project,waitingCount=0){
 const more=document.createElement('button');more.type='button';more.className='project-more';more.textContent='…'
 more.setAttribute('aria-haspopup','menu');more.setAttribute('aria-expanded','false')
 more.setAttribute('aria-label','Actions for '+project.name);more.title=more.getAttribute('aria-label');more.dataset.projectId=project.id
 // The queue button used to carry the waiting count; the opener does now.
 if(waitingCount){const badge=document.createElement('span');badge.className='queue-count';badge.textContent=waitingCount;badge.setAttribute('aria-hidden','true');more.append(badge);more.title+=' · '+waitingCount+' waiting'}
 const menu=document.createElement('div');menu.className='project-menu';menu.setAttribute('role','menu');menu.setAttribute('aria-label','Actions for '+project.name);menu.hidden=true
 let dismiss=null
 const close=(focusOpener=false)=>{
  if(menu.hidden)return
  menu.hidden=true;more.setAttribute('aria-expanded','false')
  if(dismiss){document.removeEventListener('pointerdown',dismiss,true);window.removeEventListener('blur',dismiss);dismiss=null}
  if(closeProjectMenu===close)closeProjectMenu=null
  if(focusOpener)more.focus()
  scheduleFlush()
 }
 const openAt=(x,y)=>{
  closeProjectMenu?.()
  menu.hidden=false;more.setAttribute('aria-expanded','true')
  // Fixed to the viewport, and kept inside it, so neither the sidebar's
  // scroll nor its width can clip it.
  const {width,height}=menu.getBoundingClientRect()
  menu.style.left=Math.max(4,Math.min(x,innerWidth-width-4))+'px'
  menu.style.top=Math.max(4,Math.min(y,innerHeight-height-4))+'px'
  dismiss=event=>{if(event.type==='blur'||!menu.contains(event.target)&&event.target!==more)close()}
  document.addEventListener('pointerdown',dismiss,true);window.addEventListener('blur',dismiss)
  closeProjectMenu=close
  menu.querySelector('[role=menuitem]:not(:disabled)')?.focus()
 }
 // Measured once shown: a hidden menu has no width to align on.
 const open=()=>{const box=more.getBoundingClientRect();menu.hidden=false;openAt(box.right-menu.offsetWidth,box.bottom+2)}
 const queued=queueProjects.has(project.id)
 const items=[
  {label:'Open tasks',run:()=>openTickets(project.id)},
  {label:(queued?'Show tasks':'Show execution queue')+(waitingCount?' · '+waitingCount+' waiting':''),run:()=>toggleQueue(project.id)},
  {label:'New task…',run:()=>newProjectTask(project.id)},
  {label:'Project prompt',disabled:!project.path,run:()=>openAgentConsole(project.id)},
  null,
  {label:'Project settings…',run:()=>openProject(project.id)},
  {label:'Remove from desktop',danger:true,run:()=>requestRemoveProject(project.id,project.name)}
 ]
 for(const item of items){
  if(!item){const line=document.createElement('div');line.setAttribute('role','separator');menu.append(line);continue}
  const button=document.createElement('button');button.type='button';button.setAttribute('role','menuitem')
  button.textContent=item.label;button.disabled=!!item.disabled;if(item.danger)button.className='danger'
  button.onclick=()=>{close();item.run()}
  menu.append(button)
 }
 more.onclick=()=>{menu.hidden?open():close()}
 const keys=event=>{
  if(event.key==='Escape'&&!menu.hidden){event.preventDefault();event.stopPropagation();close(true);return}
  if(event.target===more&&event.key==='ArrowDown'&&menu.hidden){event.preventDefault();open();return}
  if(menu.hidden||!['ArrowDown','ArrowUp'].includes(event.key))return
  const enabled=[...menu.querySelectorAll('[role=menuitem]:not(:disabled)')]
  const at=enabled.indexOf(document.activeElement)
  event.preventDefault()
  enabled[(at+(event.key==='ArrowDown'?1:-1)+enabled.length)%enabled.length]?.focus()
 }
 more.onkeydown=menu.onkeydown=keys
 return {more,menu,openAt}
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
 renderHeaderState(run)
 const badge=document.querySelector('#skill-result'),result=skillResult(run,skillResults.get(run?.id))
 badge.hidden=!result
 if(result){badge.className='skill-result '+result.kind;if(badge.textContent!==result.icon+' '+result.label)badge.textContent=result.icon+' '+result.label;badge.title='Skill: '+result.label;badge.setAttribute('aria-label',badge.title)}
}
function render(options){
 if(options?.deferrable&&sidebarBusy()){pendingRender=true;renderHeader();renderTaskRowStates();renderTicketRows();return}
 pendingRender=false
 changes.select(selected)
 renderHeader()
 const list=document.querySelector('#runs'),editing=list.querySelector('.task-rename')
 if(renaming&&editing)renaming.selection=[editing.selectionStart,editing.selectionEnd]
 rebuildingSidebar=true
 try{list.replaceChildren()}finally{rebuildingSidebar=false}
 let renameField=null

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
  const waitingCount=runs.filter(run=>run.projectId===project.id&&['queued','waiting'].includes(run.status)&&!run.cancelRequested).length
  const {more,menu,openAt}=projectMenu(project,waitingCount)
  projectRow.oncontextmenu=event=>{event.preventDefault();openAt(event.clientX,event.clientY)}
  projectRow.append(heading,more,menu);group.append(projectRow)
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
    const key=taskKey(run),title=document.createElement('strong')
    const context=document.createElement('button');context.textContent=run.taskKey||run.taskId;context.className='task-number';context.title='Open task in Sectile';context.setAttribute('aria-label','Open '+(run.taskKey||run.taskId)+' in Sectile');context.disabled=macroRun(run);context.onclick=()=>api.openTask(run.taskId).catch(error)
    const status=document.createElement('span');status.className='status task-skill-status';status.dataset.runId=run.id
    const state=document.createElement('span');state.className='run-state';state.dataset.runId=run.id
    const stateLabel=renderRunState(state,run)
    // data-status stays the status the server reported: the UI tests select on it.
    button.dataset.status=run.status;button.dataset.runId=run.id
    // The glyph leads the row, ahead of the key button it cannot nest inside, and still selects the run.
    button.append(title,status);button.onclick=state.onclick=()=>select(run)
    // The slot keeps the key column's width even for a free console, which has no key.
    const keySlot=document.createElement('span');keySlot.className='task-key-slot'
    if(!freeConsole(run))keySlot.append(context)
    const archive=document.createElement('button');archive.className='task-archive'
    archive.innerHTML='<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true"><path d="M4 8h16v12H4zM3 4h18v4H3zM9 12h6"/></svg>'
    archive.onclick=()=>requestArchive(run)
    const pencil=document.createElement('button');pencil.className='task-rename-button'
    pencil.innerHTML='<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true"><path d="M4 20h4L19 9l-4-4L4 16z"/><path d="m13.5 6.5 4 4"/></svg>'
    // Labels read the current name, so an edition ended in place relabels its row.
    const label=()=>{
     title.textContent=displayedName(run)
     button.title=title.textContent+' · '+runLabel(run)+' · '+stateLabel+' · '+executions.length+' execution(s)'
     const archiveLabel=(executions.some(activeRun)?'Stop and archive ':'Archive ')+(taskState(run).name||run.taskKey||run.taskId||runLabel(run))
     archive.title=archiveLabel;archive.setAttribute('aria-label',archiveLabel)
     pencil.title='Rename '+title.textContent;pencil.setAttribute('aria-label',pencil.title)
    }
    label()
    // Pressing the pencil of another row first blurs the open field, which saves it.
    pencil.onclick=()=>{renaming={key,draft:displayedName(run),fresh:true};render()}
    let field=null
    if(renaming?.key===key){
     field=document.createElement('input');field.className='task-rename';field.value=renaming.draft;field.maxLength=120;field.setAttribute('aria-label','Local task name')
     // The edition ends in place rather than through render(): rebuilding the
     // sidebar under the pointer would swallow the click that took the focus away.
     const finish=(commit,refocus)=>{
      if(renaming?.key!==key)return
      const value=renaming.draft.trim();renaming=null
      if(commit&&value&&value!==displayedName(run)){localTasks[key]={...taskState(run),name:value};saveLocalTasks()}
      label();field.replaceWith(button);renderHeader()
      if(refocus)pencil.focus()
      pendingRender=true;scheduleFlush()
     }
     field.oninput=()=>{if(renaming?.key===key)renaming.draft=field.value}
     field.onkeydown=event=>{
      if(event.key==='Enter'){event.preventDefault();finish(true,true)}
      // Consumed here, so it does not also close the ticket pane.
      else if(event.key==='Escape'){event.preventDefault();event.stopPropagation();finish(false,true)}
     }
     field.onblur=()=>{if(!rebuildingSidebar&&field.isConnected)finish(true,false)}
     field.onclick=field.onpointerdown=event=>event.stopPropagation()
     renameField=field
    }
    row.append(state,keySlot,field||button)
    const link=pullRequests.get(run.taskId)
    if(link){
     const pr=document.createElement('button');pr.className='pr-indicator'
     renderPullRequestIndicator(pr,link,prLabel(link.url)+' for '+(run.taskKey||run.taskId))
     pr.onclick=()=>api.openPR(link.url).catch(error);row.append(pr)
    }
    row.append(archive,pencil);group.append(row)
   }
  }
  list.append(group)
 }
 // An edited row no longer shown (archived, collapsed, project gone) drops its edition unsaved.
 if(renaming&&!renameField)renaming=null
 if(renameField){
  renameField.focus()
  if(renaming.fresh){renameField.select();renaming.fresh=false}
  else if(renaming.selection)renameField.setSelectionRange(...renaming.selection)
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
 if(link){
  const label=prLabel(link.url)
  renderPullRequestIndicator(selectedPR,link,label)
  const text=document.createElement('span');text.className='pr-label';text.textContent=label;selectedPR.append(text)
  selectedPR.onclick=()=>api.openPR(link.url).catch(error)
 }
 // A macro run is relaunched from the macro panel: it has no task to relaunch here.
 document.querySelector('#rerun').hidden=!current||macroRun(current)||!['completed','failed','canceled'].includes(current.status)
 document.querySelector('#stop').disabled=stopping||!current||!activeRun(current)
 const detachBtn=document.querySelector('#detach-terminal')
 if(detachBtn){
  const canDetach=current&&current.status==='running'&&!current.externalTerminal
  detachBtn.hidden=!canDetach
 }
 const terminalBadge=document.querySelector('#native-terminal-badge')
 if(terminalBadge){
  if(current&&current.externalTerminal){
   const termName=formatTerminalName(current.externalTerminal)
   terminalBadge.textContent='Active in '+termName
   terminalBadge.title='Running in host terminal emulator ('+termName+')'
   terminalBadge.hidden=false
  }else{
   terminalBadge.hidden=true
   terminalBadge.textContent=''
  }
 }
 renderNextStep()
 renderTicketRows()
}
async function updateDisconnected(ids,force=false,deferrable=false){
 const changed=ids.length!==disconnectedProjects.size||ids.some(id=>!disconnectedProjects.has(id))
 disconnectedProjects=new Set(ids)
 if(hiddenProject(selectedProject))selectedProject=null
 const current=runs.find(run=>run.id===selected)
 if(current&&hiddenProject(current.projectId)){
  selected=null;hideRepositoryChoice();terminal.reset()
  document.querySelector('#title').textContent='Select an execution'
  showDirectory('')
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
  if(changed||Date.now()-nextStepUpdated>15000)refreshNextStep()
  refreshVisibleSkillResults()
 }catch{agentUnavailable()}
 finally{refreshing=false}
}
async function startLocalAgent(form){
 const button=form.querySelector('button');button.disabled=true
 try{await api.start(Object.fromEntries(new FormData(form)));form.elements.code.value='';document.querySelector('#error').textContent='';ready();await refresh()}
 catch(err){error(err)}finally{button.disabled=!document.querySelector('#shutdown').hidden}
}
document.querySelector('#start').onsubmit=event=>{event.preventDefault();return startLocalAgent(event.target)}
document.querySelector('#stop').onclick=async()=>{
 if(!selected)return
 stopping=true;render()
 try{await api.stop(selected);await refresh()}catch(err){error(err)}finally{stopping=false;render()}
}
let detachingTerminal=false
const detachTerminalBtn=document.querySelector('#detach-terminal')
if(detachTerminalBtn){
 detachTerminalBtn.onclick=async()=>{
  if(!selected||detachingTerminal)return
  const run=runs.find(item=>item.id===selected)
  if(!run||run.status!=='running'||run.externalTerminal)return
  detachingTerminal=true
  detachTerminalBtn.disabled=true
  try{
   const res=await api.detachToNativeTerminal(selected)
   if(res?.terminal){
    run.externalTerminal=res.terminal
   }
   await refresh()
  }catch(err){error(err)}
  finally{detachingTerminal=false;detachTerminalBtn.disabled=false;render()}
 }
}
async function confirmDeclareReviewed(projectId,task){
 const key=task.key||task.id
 showDialog('Declare '+key+' as reviewed?')
 paragraph('This transitions the task to #reviewed and proposes Handoff after human merge.')
 const confirm=document.createElement('button')
 confirm.textContent='Confirm'
 const notice=document.createElement('p')
 notice.setAttribute('role','status')
 dialogBody.append(confirm,notice)
 confirm.focus()
 confirm.onclick=async()=>{
  confirm.disabled=true
  notice.textContent='Declaring code as reviewed…'
  try{
   await api.transitionStage(projectId,task.id,'reviewed','Code declared as reviewed from desktop app')
   dialog.close()
   await refreshNextStep()
   await refresh()
   if(ticketsOpen&&ticketsView?.projectID===projectId&&ticketsView.load)await ticketsView.load()
  }catch(err){
   notice.textContent=err.message
   confirm.disabled=false
  }
 }
}
function flushSidebar(){if(pendingRender&&!sidebarBusy())render()}
// Hover and focus targets settle after the event, so the check runs next tick.
const scheduleFlush=()=>setTimeout(flushSidebar,0)
sidebarList().addEventListener('pointerout',scheduleFlush)
sidebarList().addEventListener('focusout',scheduleFlush)
api.connect().then(connected=>{if(connected){ready();refresh()}else{agentUnavailable()}}).catch(error)
setInterval(async()=>{
 const action=pollAction({restarting,agentConnected,shutdownVisible:!document.querySelector('#shutdown').hidden})
 if(action==='refresh')refresh()
 else if(action==='connect'){
  try{if(await api.connect()){ready();refresh()}}catch{}
 }
},2000)

document.querySelector('#save-log').onclick=()=>{
 const buffer=terminal.buffer.active,lines=[]
 for(let i=0;i<buffer.length;i++)lines.push(buffer.getLine(i)?.translateToString(true)||'')
 api.saveLog(lines.join('\n')).catch(error)
}


async function restartLocalAgent(){
 const button=document.querySelector('#restart');button.disabled=true;restarting=true
 try{
  if(await api.restart()){
   selected=null;runs=[];last='';hideRepositoryChoice();terminal.reset();render()
   renderHeader()
   showDirectory('')
   document.querySelector('#error').textContent=''
  }
 }catch(err){error(err)}finally{restarting=false;button.disabled=false;await refresh()}
}

document.querySelector('#restart').onclick=restartLocalAgent

async function loadSettings(){
 const settings=await api.settings()
 if(settings.server)document.querySelector('#start').elements.server.value=settings.server
}
const settingsReady=loadSettings().catch(error)
document.querySelector('#setup-logs').onclick=()=>openSettings('Logs')
async function stopLocalAgent(){
 const button=document.querySelector('#shutdown');button.disabled=true;restarting=true
 try{
  if(await api.shutdown()){
   selected=null;runs=[];last='';hideRepositoryChoice();terminal.reset();render()
   document.querySelector('#setup').hidden=false;document.querySelector('#workspace').hidden=true
   document.querySelector('#restart').hidden=true;button.hidden=true
   document.querySelector('#start button').disabled=false
   agentUnavailable()
   document.querySelector('#error').textContent=''
  }
 }catch(err){error(err)}finally{restarting=false;button.disabled=false}
}

document.querySelector('#shutdown').onclick=stopLocalAgent

document.querySelector('#clear-history').onclick=async()=>{
 const button=document.querySelector('#clear-history');button.disabled=true
 try{
  const {removed}=await api.clearHistory()
  // Drop the removed runs locally right away: until the next poll the desktop
  // would otherwise keep asking for results the agent no longer holds.
  runs=runs.filter(run=>!removed.includes(run.id))
  for(const id of removed)skillResults.delete(id)
  if(removed.includes(selected)){
   selected=null;hideRepositoryChoice();terminal.reset()
   renderHeader()
   showDirectory('')
  }
  await refresh()
 }catch(err){error(err)}finally{render()}
}

const dialog=document.querySelector('#project-dialog'),dialogBody=document.querySelector('#dialog-body')
const dialogFooter=document.querySelector('.dialog-footer')
const connectForm=document.querySelector('#start')
function returnConnectForm(){if(connectForm.parentElement!==document.querySelector('#setup'))document.querySelector('#setup').append(connectForm)}
document.querySelector('#close-dialog').onclick=()=>dialog.close()
// The footer carries only the actions a dialog puts there, so it stays out of
// the way until one does: a bar whose single button repeated the cross is one
// more thing to read and nothing to do.
function syncDialogFooter(){dialogFooter.hidden=![...dialogFooter.children].some(child=>!child.hidden)}
function clearDialogFooter(){
 for(const extra of dialogFooter.querySelectorAll('.dialog-action'))extra.remove()
 syncDialogFooter()
}
// A closed dialog keeps nothing on screen, so a read still in flight when it
// closes writes into a detached node and is dropped. The close event is queued,
// so a flow that reopens the dialog in the same task keeps its fresh content.
dialog.addEventListener('close',()=>{
 if(dialog.open)return
 updateSettingsConnection=null
 returnConnectForm()
 dialogBody.replaceChildren()
 clearDialogFooter()
})
function showDialog(title){
 updateSettingsConnection=null
 dialog.classList.remove('workstation-settings')
 returnConnectForm()
 // The footer is shared by every dialog, so a control one of them added there
 // must go before the next one opens.
 clearDialogFooter()
 dialogBody.replaceChildren()
 const heading=document.createElement('h2');heading.textContent=title;dialogBody.append(heading)
 if(!dialog.open)dialog.showModal()
}
function paragraph(text){const p=document.createElement('p');p.textContent=text;dialogBody.append(p);return p}
window.addEventListener('keydown',event=>{
 if(event.key==='Escape'&&!dialog.open&&ticketsOpen){event.preventDefault();closeTickets()}
})

// Every setting reads as one row: its name on the left with the inherited value
// in small type beneath, the control that changes it on the right. A control
// that needs the whole width takes the stacked variant instead. Shared by the
// project panel and the global one so both spell a setting the same way.
const RESET_ICON='<svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.7"><path d="M4 4v6h6M4 10a8 8 0 1 1 1 8"/></svg>'
function settingRow(name,options,...widgets){
 const {resetLabel,onReset,stacked}=options||{}
 const section=document.createElement('section');section.className='setting-row'+(stacked?' stacked':'')
 const text=document.createElement('div');text.className='setting-text'
 const line=document.createElement('div');line.className='setting-name'
 const title=document.createElement('strong');title.textContent=name
 const hint=document.createElement('p')
 const control=document.createElement('div');control.className='setting-control'
 let reset=null
 if(resetLabel){
  reset=document.createElement('button');reset.type='button';reset.className='reset-setting'
  reset.setAttribute('aria-label',resetLabel);reset.title=resetLabel;reset.innerHTML=RESET_ICON
  if(onReset)reset.onclick=onReset
 }
 // A stacked control owns the whole width, so its reset belongs on the name
 // line rather than beside the control.
 line.append(title);if(stacked&&reset)line.append(reset)
 text.append(line,hint)
 control.append(...widgets);if(!stacked&&reset)control.append(reset)
 section.append(text,control)
 return {section,control,hint,reset}
}
// A read-only row: the value the workstation holds, stated where the control
// would sit, so the panel stays one grammar whether a setting is editable here.
function readOnlyRow(name,hint){
 const value=document.createElement('span');value.className='setting-value'
 const row=settingRow(name,null,value)
 row.hint.textContent=hint||''
 row.value=value
 return row
}

// The offer to make the folder of a field a Git repository (#481), shown under
// it while the folder is a plain one or a repository with no commit. It is
// examined when the settings open, when Browse returns and when a typed value
// is committed. Initializing never saves: the field takes the repository's
// top level and the user saves it as any other change.
function attachGitOffer(input,row,label,onReady){
 const box=document.createElement('div');box.className='git-offer';box.hidden=true
 box.setAttribute('role','group');box.setAttribute('aria-label',label+' Git initialization')
 const title=document.createElement('strong'),detail=document.createElement('p')
 const initialize=document.createElement('button');initialize.type='button';initialize.className='git-offer-action'
 const dismiss=document.createElement('button');dismiss.type='button'
 const actions=document.createElement('div');actions.className='git-offer-actions';actions.append(initialize,dismiss)
 box.append(title,detail,actions)
 const status=document.createElement('p');status.className='git-offer-status';status.setAttribute('role','status')
 status.setAttribute('aria-label',label+' Git initialization status')
 row.control.append(box,status)
 // "Not now" holds for the value it was said to; another value is examined.
 const dismissed=new Set()
 let shown=true,offered=''
 async function examine(){
  const value=input.value.trim()
  if(!shown||!value||dismissed.has(value)){box.hidden=true;return}
  let answer
  try{answer=await api.gitState(value)}catch{box.hidden=true;return}
  // The field changed while the agent answered: that answer is stale.
  if(!shown||input.value.trim()!==value)return
  const offer=offerFor(answer?.state)
  box.hidden=!offer;offered=offer?value:''
  if(offer){title.textContent=offer.title;detail.textContent=offer.detail;initialize.textContent=offer.action;dismiss.textContent=offer.dismiss}
 }
 initialize.onclick=async()=>{
  // The explanation was given for one folder: another value typed since is
  // examined first, never initialized unseen.
  const value=input.value.trim()
  if(value!==offered){await examine();return}
  initialize.disabled=dismiss.disabled=true;status.textContent='';status.dataset.tone=''
  try{
   const result=await api.gitInit(value)
   input.value=result?.path||value;box.hidden=true
   status.textContent=initializedNotice(input.value)
   onReady?.(input.value)
  }catch(err){
   // Git's own words, then what the folder is now: a repository whose commit
   // failed is unborn, and its offer makes the commit alone.
   status.textContent=String(err?.message||err).replace(/^Error invoking remote method '[^']*': (Error: )?/,'');status.dataset.tone='error'
   await examine()
  }finally{initialize.disabled=dismiss.disabled=false}
 }
 dismiss.onclick=()=>{dismissed.add(input.value.trim());box.hidden=true}
 input.addEventListener('change',()=>{status.textContent='';examine()})
 return {
  examine(){status.textContent='';return examine()},
  // A field that is not shown offers nothing.
  show(value){shown=value;if(value)examine();else{box.hidden=true;status.textContent=''}}
 }
}

const MODEL_REGEX=/^[A-Za-z0-9][A-Za-z0-9._:@/-]*$/
function validateModel(val){
 const trimmed=String(val||'').trim()
 if(trimmed==='')return true
 return MODEL_REGEX.test(trimmed)
}

// The terminals a launch may open, shared by the workstation panel and the
// project dialog.
const TERMINALS=[
 {id:'',label:'Auto-detect (Ghostty, iTerm, Terminal)'},
 {id:'ghostty',label:'Ghostty'},
 {id:'terminal',label:'Terminal.app'},
 {id:'iterm',label:'iTerm'},
 {id:'custom',label:'Custom command…'}
]
const STANDARD_TERMINALS=['','ghostty','terminal','iterm']
// terminalPicker is a select of the known terminals plus a free command.
function terminalPicker(onChange){
 const select=document.createElement('select');select.className='terminal-select';select.setAttribute('aria-label','Terminal emulator')
 for(const t of TERMINALS){const opt=document.createElement('option');opt.value=t.id;opt.textContent=t.label;select.append(opt)}
 const custom=document.createElement('input');custom.type='text';custom.className='custom-terminal-input'
 custom.setAttribute('aria-label','Custom terminal command');custom.placeholder='e.g. alacritty -e {command}'
 const picker={select,custom,
  set(value){
   value=String(value||'')
   if(value&&!STANDARD_TERMINALS.includes(value.toLowerCase())){select.value='custom';custom.value=value}
   else{select.value=value.toLowerCase();custom.value=''}
   custom.hidden=select.value!=='custom'
  },
  get(){return select.value==='custom'?custom.value.trim():select.value}
 }
 select.onchange=()=>{custom.hidden=select.value!=='custom';onChange?.()}
 custom.oninput=()=>onChange?.()
 return picker
}
// editorPicker is a select of the known editors plus a free command (#535).
function editorPicker(onChange){
 const select=document.createElement('select');select.className='terminal-select';select.setAttribute('aria-label','Editor')
 for(const e of EDITORS){const opt=document.createElement('option');opt.value=e.id;opt.textContent=e.label;select.append(opt)}
 const custom=document.createElement('input');custom.type='text';custom.className='custom-terminal-input'
 custom.setAttribute('aria-label','Custom editor command');custom.placeholder='e.g. cursor -n'
 const picker={select,custom,
  set(value){
   const choice=editorChoice(value)
   select.value=choice.select;custom.value=choice.custom
   custom.hidden=select.value!=='custom'
  },
  get(){return select.value==='custom'?custom.value.trim():select.value}
 }
 select.onchange=()=>{custom.hidden=select.value!=='custom';onChange?.()}
 custom.oninput=()=>onChange?.()
 return picker
}
function providerOptions(select,extra){
 select.replaceChildren()
 const known=PROVIDERS.map(p=>p.id)
 for(const p of PROVIDERS){const opt=document.createElement('option');opt.value=p.id;opt.textContent=p.label;select.append(opt)}
 // A provider stored outside the list stays selectable instead of being lost.
 for(const id of extra||[])if(id&&!known.includes(id)){const opt=document.createElement('option');opt.value=id;opt.textContent=id;select.append(opt);known.push(id)}
}
const CLI_PRESETS=[
 {label:'AGY',provider:'agy',cmd:'agy --dangerously-skip-permissions --model {model} "{prompt}"',auto:'agy --dangerously-skip-permissions --model {model} -p "{prompt}"'},
 {label:'Claude',provider:'claude',cmd:"claude --model {model} '{prompt}'",auto:"claude -p --permission-mode bypassPermissions --model {model} '{prompt}'"},
 {label:'Codex',provider:'codex',cmd:"codex --model {model} '{prompt}'",auto:"codex exec --model {model} '{prompt}'"},
 {label:'Clear to defaults',provider:'agy',cmd:'',auto:''}
]
const KNOWN_COMMANDS=['',"/path/to/custom-cli {mode:-p|-i} '{prompt}'","claude --model {model} '{prompt}'",'agy --dangerously-skip-permissions --model {model} "{prompt}"',"codex --model {model} '{prompt}'","gemini --model {model} '{prompt}'","vibe '{prompt}'"]
const PLACEHOLDER_HELP='Required in a command: {prompt} (instructions). Also: {issueKey}, {issueTitle}, {issueDesc}, {branchName}, {repoPath} (local directory), {tracker}, {repo}, {model}, {mode:AUTONOMOUS|INTERACTIVE}, {addDirs} (the other folders of the task, as --add-dir options for Claude).'
const INVALID_MODEL='Invalid model: must only contain letters, digits, and allowed punctuation (. _ - : @ /)'

// A list of key/value rows (skill → model, skill → command) with its own add
// and remove controls. `fixed` lists keys shown whether set or not.
function entryList({keyLabel,valueLabel,addLabel,fixed,placeholder,onChange,validate}){
 const box=document.createElement('div');box.className='entry-list'
 const rows=document.createElement('div');rows.className='entry-rows'
 const add=document.createElement('button');add.type='button';add.textContent=addLabel;add.hidden=!!fixed
 box.append(rows,add)
 const entries=[]
 function addRow(key,value,locked){
  const row=document.createElement('div');row.className='entry-row'
  const keyInput=document.createElement('input');keyInput.type='text';keyInput.value=key||'';keyInput.setAttribute('aria-label',keyLabel);keyInput.readOnly=!!locked
  const valueInput=document.createElement('input');valueInput.type='text';valueInput.value=value||''
  valueInput.setAttribute('aria-label',valueLabel+(key?' '+key:''))
  valueInput.placeholder=placeholder?.(key)||''
  row.append(keyInput,valueInput)
  if(!locked){
   const remove=document.createElement('button');remove.type='button';remove.textContent='Remove';remove.setAttribute('aria-label','Remove '+valueLabel.toLowerCase()+(key?' '+key:''))
   remove.onclick=()=>{row.remove();entries.splice(entries.indexOf(entry),1);onChange?.()}
   row.append(remove)
  }
  const entry={keyInput,valueInput}
  const changed=()=>{
   const bad=validate&&!validate(valueInput.value)
   if(bad)valueInput.setAttribute('aria-invalid','true');else valueInput.removeAttribute('aria-invalid')
   onChange?.()
  }
  valueInput.oninput=changed;keyInput.oninput=changed
  entries.push(entry);rows.append(row)
  return entry
 }
 add.onclick=()=>{addRow('','',false).keyInput.focus();onChange?.()}
 return {box,
  set(map){
   rows.replaceChildren();entries.length=0
   const values=map||{}
   for(const key of fixed||[])addRow(key,values[key]||'',true)
   for(const [key,value] of Object.entries(values))if(!(fixed||[]).includes(key))addRow(key,value,false)
  },
  get(){const out={};for(const {keyInput,valueInput} of entries){const k=keyInput.value.trim(),v=valueInput.value.trim();if(k&&v)out[k]=v}return out},
  invalid(){return entries.some(({valueInput})=>validate&&!validate(valueInput.value))},
  refreshPlaceholders(){for(const {keyInput,valueInput} of entries)valueInput.placeholder=placeholder?.(keyInput.value.trim())||''}
 }
}

// Whether the running agent keeps an engine catalogue and a per-task engine
// (#510). An older agent answers neither, so the desktop hides what needs them.
async function taskEnginesAvailable(){
 try{return !!(await api.status()).capabilities?.includes('task-engines')}catch{return false}
}

// The engine catalogue (#510): each AI CLI profile of the workstation, named,
// one of them the workstation default engine. The agent owns it; every change
// is saved through it at once, so the list always shows what is stored.
function enginesSection(){
 const section=document.createElement('section');section.className='engines-section'
 const heading=document.createElement('h3');heading.textContent='Engines'
 const intro=document.createElement('p');intro.className='hint'
 intro.textContent='A project runs the workstation default engine unless it picks another one; a task can be switched to any engine from the ticket list.'
 const unavailable=document.createElement('p');unavailable.className='hint';unavailable.hidden=true
 const list=document.createElement('ul');list.className='engine-list';list.setAttribute('aria-label','Engines')
 const add=document.createElement('button');add.type='button';add.textContent='Add an engine'
 const notice=document.createElement('p');notice.setAttribute('role','status');notice.className='engines-notice'
 const editorBox=document.createElement('div');editorBox.className='engine-editor';editorBox.setAttribute('role','group');editorBox.setAttribute('aria-label','Engine editor');editorBox.hidden=true
 section.append(heading,intro,unavailable,list,add,editorBox,notice)
 let view=null,confirming=null
 const say=(text,tone='')=>{notice.textContent=text;notice.dataset.tone=tone}

 // The editor: name, provider, model, per-skill models and both templates.
 const nameInput=document.createElement('input');nameInput.type='text';nameInput.className='model-input';nameInput.setAttribute('aria-label','Engine name');nameInput.maxLength=64
 const nameRow=settingRow('Name',null,nameInput)
 const providerSelect=document.createElement('select');providerSelect.className='provider-select';providerSelect.setAttribute('aria-label','AI Provider')
 providerOptions(providerSelect)
 const providerRow=settingRow('AI Provider',null,providerSelect)
 const modelInput=document.createElement('input');modelInput.type='text';modelInput.className='model-input';modelInput.setAttribute('aria-label','AI Model')
 modelInput.placeholder='Empty: use provider default model'
 const modelRow=settingRow('AI Model',null,modelInput)
 const skillModels=entryList({keyLabel:'Skill',valueLabel:'Model for skill',addLabel:'Add a skill model',validate:validateModel,onChange:()=>renderEditor()})
 const skillModelsRow=settingRow('Per-skill models',{stacked:true},skillModels.box)
 const command=document.createElement('textarea');command.className='cli-command';command.setAttribute('aria-label','Interactive CLI command')
 command.placeholder='Provider default command'
 const autonomousCommand=document.createElement('textarea');autonomousCommand.className='cli-command';autonomousCommand.setAttribute('aria-label','Autonomous CLI command')
 autonomousCommand.placeholder='Empty: the interactive command serves headless launches too'
 const previewBox=document.createElement('dl');previewBox.className='command-preview'
 const help=document.createElement('details');help.className='placeholder-help'
 const helpSummary=document.createElement('summary');helpSummary.textContent='Placeholders'
 const helpText=document.createElement('p');helpText.textContent=PLACEHOLDER_HELP
 help.append(helpSummary,helpText)
 const presetsBar=document.createElement('div');presetsBar.className='cli-presets-bar'
 for(const preset of CLI_PRESETS){
  const button=document.createElement('button');button.type='button';button.textContent=preset.label
  button.onclick=()=>{providerSelect.value=preset.provider;command.value=preset.cmd;autonomousCommand.value=preset.auto;renderEditor()}
  presetsBar.append(button)
 }
 const commandRow=settingRow('Interactive CLI command',{stacked:true},command,help,presetsBar)
 const autonomousRow=settingRow('Autonomous CLI command (headless)',{stacked:true},autonomousCommand,previewBox)
 const saveEngine=document.createElement('button');saveEngine.type='button';saveEngine.className='dialog-action primary';saveEngine.textContent='Save engine'
 const cancelEngine=document.createElement('button');cancelEngine.type='button';cancelEngine.textContent='Cancel'
 const editorActions=document.createElement('div');editorActions.className='deployment-actions';editorActions.append(saveEngine,cancelEngine)
 editorBox.append(nameRow.section,providerRow.section,modelRow.section,skillModelsRow.section,commandRow.section,autonomousRow.section,editorActions)
 let editing=null
 function renderEditor(){
  if(!validateModel(modelInput.value)){modelRow.hint.textContent=INVALID_MODEL;modelInput.setAttribute('aria-invalid','true')}
  else{modelInput.removeAttribute('aria-invalid');modelRow.hint.textContent=modelInput.value.trim()?'':'Provider default model'}
  skillModelsRow.hint.textContent=skillModels.invalid()?INVALID_MODEL:'A model per skill beats the AI model above.'
  commandRow.hint.textContent='Both empty runs the provider default for each mode.'
  autonomousRow.hint.textContent='Command template used for autonomous runs. Empty falls back to interactive command.'
  previewBox.replaceChildren()
  for(const line of previewLines(providerSelect.value,command.value,modelInput.value,autonomousCommand.value)){
   const term=document.createElement('dt');term.textContent=line.label
   const detail=document.createElement('dd');detail.textContent=line.text
   if(!line.ok)detail.className='command-preview-error'
   previewBox.append(term,detail)
  }
 }
 providerSelect.onchange=()=>{
  // A command written for another provider does not follow the switch; a
  // preset or an empty one does.
  if(KNOWN_COMMANDS.includes(command.value.trim())){
   command.value=providerSelect.value==='custom'?"/path/to/custom-cli {mode:-p|-i} '{prompt}'":''
   autonomousCommand.value=''
  }
  renderEditor()
 }
 for(const input of [modelInput,command,autonomousCommand])input.addEventListener('input',renderEditor)
 function openEditor(engine){
  editing=engine||{id:'',name:'',provider:DEFAULT_PROVIDER}
  confirming=null
  providerOptions(providerSelect,[editing.provider])
  nameInput.value=editing.name||'';providerSelect.value=editing.provider||DEFAULT_PROVIDER
  modelInput.value=editing.model||'';skillModels.set(editing.skillModels||{})
  command.value=editing.command||'';autonomousCommand.value=editing.commandAutonomous||''
  editorBox.hidden=false;add.hidden=true;say('');renderEditor();renderList();nameInput.focus()
 }
 function closeEditor(){editing=null;editorBox.hidden=true;add.hidden=false;renderList()}
 cancelEngine.onclick=closeEditor

 // Each save sends the whole catalogue built from the stored one, so the
 // actions wait for the previous answer: a second click would undo the first.
 let pending=false
 async function save(catalogue,defaultId,done){
  if(pending)return
  pending=true;saveEngine.disabled=true;say('Saving…');renderList()
  try{
   view=await api.saveEngines({catalogue,default:defaultId})
   done?.();say(done?'Engine saved':'Engines saved')
  }catch(err){
   if(agentUnreachable(err))say('The local agent is stopped. Start it to edit engines.','error')
   else say('Not saved: '+ipcMessage(err),'error')
  }finally{pending=false;saveEngine.disabled=false;renderList()}
 }
 saveEngine.onclick=()=>{
  const name=nameInput.value.trim()
  if(!name){say('An engine needs a name','error');return}
  if(!validateModel(modelInput.value)||skillModels.invalid()){say(INVALID_MODEL,'error');return}
  if(providerSelect.value==='custom'&&!command.value.includes('{prompt}')){say('Custom provider requires a command template containing {prompt}','error');return}
  const edited={...editing,name,provider:providerSelect.value,model:modelInput.value.trim(),skillModels:compact(skillModels.get()),command:command.value.trim(),commandAutonomous:autonomousCommand.value.trim()}
  const catalogue=editing.id?view.catalogue.map(engine=>engine.id===editing.id?edited:engine):[...view.catalogue,edited]
  save(catalogue,view.default,closeEditor)
 }
 add.onclick=()=>openEditor(null)

 function button(text,label,onclick,disabled){
  const b=document.createElement('button');b.type='button';b.textContent=text;b.setAttribute('aria-label',label);b.title=label
  b.disabled=!!disabled;b.onclick=onclick;return b
 }
 function renderList(){
  list.replaceChildren()
  if(!view)return
  const count=view.catalogue.length
  view.catalogue.forEach((engine,index)=>{
   const isDefault=engine.id===view.default
   const item=document.createElement('li');item.className='engine-item';item.dataset.engineId=engine.id
   const mark=document.createElement('span');mark.className='engine-mark';mark.setAttribute('aria-hidden','true');mark.textContent=engineMark(engine.provider)
   const text=document.createElement('span');text.className='engine-text'
   const title=document.createElement('strong');title.textContent=engine.name
   const detail=document.createElement('small');detail.textContent=engine.provider+' · '+(engine.model||'provider default')
   text.append(title,detail)
   item.append(mark,text)
   if(isDefault){const badge=document.createElement('span');badge.className='engine-default';badge.textContent='Default';item.append(badge)}
   const actions=document.createElement('span');actions.className='engine-actions'
   const busy=!!editing||pending
   actions.append(
    button('Edit','Edit '+engine.name,()=>openEditor(engine),busy),
    button('↑','Move '+engine.name+' up',()=>save(moveEngine(view.catalogue,engine.id,-1),view.default),busy||index===0),
    button('↓','Move '+engine.name+' down',()=>save(moveEngine(view.catalogue,engine.id,1),view.default),busy||index===count-1),
   )
   if(!isDefault)actions.append(button('Make default','Make '+engine.name+' the default engine',()=>save(view.catalogue,engine.id),busy))
   // The default engine and the last one cannot go: mark another one first.
   actions.append(button('Remove','Remove '+engine.name,()=>{confirming=engine.id;renderList()},busy||isDefault||count===1))
   item.append(actions)
   list.append(item)
   if(confirming===engine.id){
    const confirm=document.createElement('li');confirm.className='engine-confirm'
    const names=Object.fromEntries((projects||[]).map(project=>[project.id,project.name]))
    const message=document.createElement('span');message.textContent=removalMessage(engine.name,removalImpact(view,engine.id,names))
    confirm.append(message,
     button('Confirm removal','Confirm the removal of '+engine.name,()=>{confirming=null;save(view.catalogue.filter(e=>e.id!==engine.id),view.default)},pending),
     button('Cancel','Keep '+engine.name,()=>{confirming=null;renderList()}))
    list.append(confirm)
   }
  })
 }
 async function load(){
  editing=null;confirming=null;editorBox.hidden=true
  if(!await taskEnginesAvailable()){
   view=null;list.replaceChildren();add.hidden=true
   unavailable.textContent='Update and restart the local agent to manage engines.';unavailable.hidden=false
   return
  }
  unavailable.hidden=true;add.hidden=false
  try{view=await api.engines()}
  catch(err){view=null;say('Unable to read engines: '+ipcMessage(err),'error')}
  renderList()
 }
 return {section,load,defaultProvider:()=>view?.catalogue.find(engine=>engine.id===view.default)?.provider||''}
}

// The "Execution defaults" panel: the workstation level of every execution
// setting, read from and written through the local agent (#305). The agent is
// the only writer of these sections, so without it the panel says the settings
// are unavailable rather than writing the file itself.
function executionDefaultsPanel(panel){
 const unavailable=document.createElement('p');unavailable.className='execution-unavailable';unavailable.setAttribute('role','status');unavailable.hidden=true
 const body=document.createElement('div');body.className='execution-defaults';body.hidden=true
 let view=null
 // A field is "stated" when the workstation sets it; unset, it runs the default.
 const stated={}
 const changed=()=>{notice.textContent='';notice.dataset.tone=''}

 // The engines (#510) come first: what a project runs unless it picks another.
 const engines=enginesSection()
 // The MCP configuration below is per provider: this picks which one.
 const providerSelect=document.createElement('select');providerSelect.className='provider-select';providerSelect.setAttribute('aria-label','MCP provider')
 providerOptions(providerSelect)

 // The models a launch may pick, per provider. A provider without a list of
 // its own offers the one Sectile ships; editing it creates the list.
 const listsBox=document.createElement('div');listsBox.className='provider-model-lists'
 const listInputs={}
 const listsRow=settingRow('Models offered per provider',{stacked:true},listsBox)

 const terminal=terminalPicker(()=>{changed();render()})
 const terminalRow=settingRow('Terminal emulator',{resetLabel:'Reset terminal emulator to default',onReset:()=>{terminal.set('');render()}},terminal.select,terminal.custom)

 const editor=editorPicker(()=>{changed();render()})
 const editorRow=settingRow('Editor',{resetLabel:'Reset editor to default',onReset:()=>{editor.set('');render()}},editor.select,editor.custom)

 let useWorktrees=null
 const worktreeGroup=document.createElement('div');worktreeGroup.className='segmented'
 worktreeGroup.setAttribute('role','group');worktreeGroup.setAttribute('aria-label','Worktrees')
 const worktreeButtons=['Yes','No'].map(value=>{
  const button=document.createElement('button');button.type='button';button.textContent=value
  button.onclick=()=>{useWorktrees=value==='Yes';changed();render()}
  worktreeGroup.append(button);return button
 })
 const worktreeRow=settingRow('Worktrees',{resetLabel:'Reset worktrees to default',onReset:()=>{useWorktrees=null;render()}},worktreeGroup)

 let parallelism=0
 const parallelInput=document.createElement('input');parallelInput.type='range'
 parallelInput.min='1';parallelInput.max=String(MAX_PARALLELISM);parallelInput.step='1'
 parallelInput.className='slider-input';parallelInput.setAttribute('aria-label','Parallel executions')
 parallelInput.oninput=()=>{parallelism=Number(parallelInput.value);changed();render()}
 const parallelReadout=document.createElement('span');parallelReadout.className='slider-value'
 const parallelRow=settingRow('Parallel executions',{resetLabel:'Reset parallel executions to default',onReset:()=>{parallelism=0;render()}},parallelInput,parallelReadout)

 let setupProviders=null
 const setupBox=document.createElement('div');setupBox.className='setup-providers'
 const setupChecks={}
 const setupRow=settingRow('Extra setup providers',{resetLabel:'Reset setup providers to default',onReset:()=>{setupProviders=null;render()}},setupBox)
 function renderSetupChoices(choices){
  setupBox.replaceChildren()
  for(const id of choices){
   const label=document.createElement('label');label.className='checkbox-label'
   const box=document.createElement('input');box.type='checkbox';box.setAttribute('aria-label','Set up '+id)
   box.onchange=()=>{setupProviders=Object.entries(setupChecks).filter(([,input])=>input.checked).map(([key])=>key);changed();render()}
   setupChecks[id]=box;label.append(box,document.createTextNode(' '+id));setupBox.append(label)
  }
 }

 const notice=document.createElement('p');notice.setAttribute('role','status');notice.className='workstation-notice'
 const save=document.createElement('button');save.type='button';save.className='dialog-action primary';save.textContent='Save execution defaults'
 const actions=document.createElement('div');actions.className='deployment-actions';actions.style.marginTop='16px'
 actions.append(save,notice)
 body.append(engines.section,listsRow.section,terminalRow.section,editorRow.section,worktreeRow.section,parallelRow.section,setupRow.section,actions)

 function hint(row,set,defaultText,setText){row.hint.textContent=set?(setText||'Workstation default'):'Default · '+defaultText}
 function render(){
  for(const [id,entry] of Object.entries(listInputs)){
   entry.row.hint.textContent=stated['models:'+id]?'Custom list · Empty offers no model at launch.':'Shipped list'
   if(!stated['models:'+id])entry.input.value=(view?.providerModels?.[id]||[]).join(', ')
  }
  hint(terminalRow,!!terminal.get(),'Auto-detect')
  hint(editorRow,!!editor.get(),'None')
  const worktrees=useWorktrees??true
  worktreeButtons.forEach((button,i)=>button.setAttribute('aria-pressed',String(worktrees===(i===0))))
  hint(worktreeRow,useWorktrees!==null,'Yes')
  const limit=parallelism||1
  parallelInput.value=String(limit)
  parallelReadout.textContent=limit+(limit===1?' execution':' executions')
  hint(parallelRow,parallelism!==0,'1 execution')
  for(const [id,box] of Object.entries(setupChecks))box.checked=!!setupProviders?.includes(id)
  setupRow.hint.textContent=setupProviders===null?'Default · None beyond the provider':setupProviders.length?'Workstation default':'Workstation default · None'
 }

 function fill(){
  const defaults=view.defaults||{}
  // An agent that predates #510 names its provider directly.
  const provider=view.effective?.defaultEngine?.provider||view.effective?.aiProvider||DEFAULT_PROVIDER
  providerOptions(providerSelect,[provider])
  providerSelect.value=provider
  terminal.set(defaults.terminal||'')
  editor.set(defaults.editorCommand||'')
  useWorktrees=typeof defaults.useWorktrees==='boolean'?defaults.useWorktrees:null
  parallelism=Number(defaults.parallelism)||0
  setupProviders=Array.isArray(defaults.setupProviders)?[...defaults.setupProviders]:null
  renderSetupChoices(view.setupProviders?.length?view.setupProviders:SETUP_PROVIDERS)
  listsBox.replaceChildren()
  for(const key of Object.keys(listInputs))delete listInputs[key]
  const providers=[...new Set([...Object.keys(view.providerModels||{}),...Object.keys(defaults.aiProviderModels||{})])].sort()
  for(const id of providers){
   const input=document.createElement('input');input.type='text';input.className='model-input';input.setAttribute('aria-label','Models offered for '+id)
   const own=defaults.aiProviderModels&&Object.prototype.hasOwnProperty.call(defaults.aiProviderModels,id)
   stated['models:'+id]=!!own
   input.value=own?(defaults.aiProviderModels[id]||[]).join(', '):''
   input.oninput=()=>{stated['models:'+id]=true;changed();render()}
   const row=settingRow(id,{resetLabel:'Reset '+id+' models to the shipped list',onReset:()=>{stated['models:'+id]=false;render()}},input)
   listInputs[id]={input,row};listsBox.append(row.section)
  }
  render()
 }
 function state(){
  const lists={}
  for(const [id,entry] of Object.entries(listInputs))if(stated['models:'+id])lists[id]=parseModelList(entry.input.value)
  return {
   terminal:terminal.get(),editorCommand:editor.get(),useWorktrees,parallelism,setupProviders,aiProviderModels:lists
  }
 }
 save.onclick=async()=>{
  const invalidList=Object.entries(listInputs).find(([id,entry])=>stated['models:'+id]&&parseModelList(entry.input.value).some(model=>!validateModel(model)))
  if(invalidList){notice.textContent='Invalid model in the list of '+invalidList[0];notice.dataset.tone='error';return}
  save.disabled=true;notice.textContent='Saving…';notice.dataset.tone=''
  try{
   await api.saveWorkstationSettings(workstationPayload(state(),view.defaults))
   notice.textContent='Execution defaults saved'
   loadEditorSetting()
   try{view=await api.workstationSettings();if(body.isConnected){fill();notice.textContent='Execution defaults saved'}}catch{}
  }catch(err){
   // The agent refused: its reason is shown as it gave it, and nothing changed.
   if(agentUnreachable(err))showUnavailable(STOPPED_NOTICE)
   else{notice.textContent='Not saved: '+ipcMessage(err);notice.dataset.tone='error'}
  }finally{save.disabled=false}
 }
 const STOPPED_NOTICE='Execution settings are unavailable while the local agent is stopped. Start the agent to edit them.'
 function showUnavailable(text){unavailable.textContent=text;unavailable.hidden=false;body.hidden=true}
 async function load(){
  if(!agentConnected){showUnavailable(STOPPED_NOTICE);return}
  unavailable.hidden=true
  try{view=await api.workstationSettings()}
  catch(err){
   if(!body.isConnected)return
   const text=ipcMessage(err)
   showUnavailable(agentUnreachable(err)?STOPPED_NOTICE:/404|not found/i.test(text)?'Update and restart the local agent to edit execution settings here.':'Unable to read execution settings: '+text)
   return
  }
  if(!body.isConnected)return
  body.hidden=false;fill()
  await engines.load()
 }
 panel.append(unavailable,body)
 return {providerSelect,load}
}

// The workstation's own settings, gathered where the project panel already puts
// a project's: one dialog, a category per surface. The header carried three
// unrelated controls for these; the sidebar now carries one.
const SETTINGS_CATEGORIES=[
 {id:'Profile',label:'User profile',icon:'<circle cx="12" cy="8" r="4"/><path d="M4 21v-2a8 8 0 0 1 16 0v2"/>'},
 {id:'Appearance',label:'Appearance',icon:'<circle cx="12" cy="12" r="9"/><path d="M12 3a9 9 0 0 0 0 18Z" fill="currentColor"/>'},
 {id:'Connection',label:'Agent connection',icon:'<path d="M4 7h16M4 17h16"/><circle cx="9" cy="7" r="3"/><circle cx="15" cy="17" r="3"/>'},
 {id:'AgentCli',label:'Execution defaults',icon:'<rect x="3" y="4" width="18" height="16" rx="2"/><path d="m7 9 3 3-3 3M12 15h5"/>'},
 {id:'Logs',label:'Agent logs',icon:'<path d="M14 3H7a1 1 0 0 0-1 1v16a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1V7Z"/><path d="M14 3v4h4"/><path d="M9 13h6M9 17h6"/>'},
 {id:'Changelog',label:'Changelog',icon:'<circle cx="12" cy="12" r="9"/><path d="M12 11v5"/><path d="M12 8h.01"/>'}
]
function openSettings(initial='Profile'){
 showDialog('Settings')
 dialog.classList.add('workstation-settings')
 const layout=document.createElement('div');layout.className='settings-layout'
 const tabs=document.createElement('div');tabs.className='settings-nav';tabs.setAttribute('role','tablist')
 tabs.setAttribute('aria-orientation','vertical');tabs.setAttribute('aria-label','Settings categories')
 const content=document.createElement('div');content.className='settings-content stretch'
 layout.append(tabs,content);dialogBody.append(layout)
 const panels={}
 for(const category of SETTINGS_CATEGORIES){
  const tab=document.createElement('button');tab.type='button';tab.setAttribute('role','tab');tab.dataset.category=category.id;tab.title=category.label
  tab.innerHTML='<svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">'+category.icon+'</svg>'
  const text=document.createElement('span');text.className='settings-nav-label';text.textContent=category.label;tab.append(text)
  tab.id='settings-tab-'+category.id;tab.setAttribute('aria-controls','settings-panel-'+category.id)
  const panel=document.createElement('section');panel.id='settings-panel-'+category.id
  panel.setAttribute('role','tabpanel');panel.setAttribute('aria-labelledby',tab.id);panels[category.id]=panel
  tab.onclick=()=>selectCategory(category.id)
  tabs.append(tab);content.append(panel)
 }
 function selectCategory(name){
  for(const [key,value] of Object.entries(panels))value.hidden=key!==name
  for(const item of tabs.children)item.setAttribute('aria-selected',String(item.dataset.category===name))
  if(name==='Logs')loadLog()
 }

 // Nothing here is stored locally beyond the pairing the connection screen
 // writes, so the profile states what this workstation knows about its account
 // and sends the rest to the web interface, which owns the profile itself.
 const account=readOnlyRow('Sectile server','The server this workstation is paired with.')
 const device=readOnlyRow('Workstation','The identifier this machine was paired under.')
 const openWeb=document.createElement('button');openWeb.type='button';openWeb.textContent='Open the web interface'
 openWeb.onclick=()=>api.openBoard().catch(error)
 const web=settingRow('Profile and API keys',null,openWeb)
 web.hint.textContent='Display name, password and API keys live in the web interface.'
 panels.Profile.append(account.section,device.section,web.section)

 // Appearance: a workstation preference, applied as soon as it is pressed.
 // The main process stores it and switches the whole window; the buttons only
 // mirror what it answers.
 const appearanceGroup=document.createElement('div');appearanceGroup.className='segmented'
 appearanceGroup.setAttribute('role','group');appearanceGroup.setAttribute('aria-label','Appearance')
 const markAppearance=value=>{for(const button of appearanceGroup.children)button.setAttribute('aria-pressed',String(button.dataset.value===value))}
 for(const choice of APPEARANCE_CHOICES){
  const button=document.createElement('button');button.type='button';button.textContent=choice.label;button.dataset.value=choice.value
  button.onclick=()=>api.setAppearance(choice.value).then(markAppearance).catch(error)
  appearanceGroup.append(button)
 }
 const appearance=settingRow('Theme',null,appearanceGroup)
 appearance.hint.textContent='System follows the appearance of this computer. The web interface keeps its own theme.'
 panels.Appearance.append(appearance.section)
 markAppearance('system')
 api.appearance().then(markAppearance).catch(()=>{})

 // Execution defaults: the workstation level of every execution setting,
 // owned by the local agent. The MCP connection choice follows its provider.
 const execution=executionDefaultsPanel(panels.AgentCli)
 const mcpPanel=mcpSettings(api,execution.providerSelect)
 mcpPanel.section.insertBefore(settingRow('Provider',null,execution.providerSelect).section,mcpPanel.section.children[1])
 panels.AgentCli.append(mcpPanel.section)

 const agentState=readOnlyRow('Local agent','The agent process this desktop talks to.')
 const agentActions=document.createElement('span');agentActions.className='settings-agent-actions'
 const agentButtons=[]
 let agentActionPending=false
 const renderAgentActions=()=>{
  agentState.value.textContent=agentConnected?'Running':'Stopped'
  for(const [button,needsRunning] of agentButtons)button.disabled=agentActionPending||restarting||agentConnected!==needsRunning
 }
 for(const [label,needsRunning,action] of [
  ['Start agent',false,()=>startLocalAgent(connectForm)],
  ['Stop agent',true,stopLocalAgent],
  ['Restart agent',true,restartLocalAgent]
 ]){
  const button=document.createElement('button');button.type='button';button.textContent=label
  button.onclick=async()=>{
   if(agentActionPending||restarting)return
   agentActionPending=true;renderAgentActions()
   try{await action()}finally{agentActionPending=false;if(agentActions.isConnected){await fill();renderAgentActions()}}
  }
  agentButtons.push([button,needsRunning]);agentActions.append(button)
 }
 agentState.control.append(agentActions)
 renderAgentActions()
 const link=readOnlyRow('Server link','Whether the local agent reaches the Sectile server.')
 const linkDot=document.createElement('span');linkDot.className='connection-dot';linkDot.setAttribute('aria-hidden','true')
 link.control.classList.add('settings-connection-status');link.control.dataset.state='off'
 link.control.prepend(linkDot)
 const pairing=settingRow('Pairing',{stacked:true},connectForm)
 const pairingNote=document.createElement('p');pairingNote.setAttribute('role','status')
 pairing.control.append(pairingNote)
 panels.Connection.append(agentState.section,link.section,pairing.section)

 fillChangelogPanel(panels.Changelog)

 const logs=document.createElement('div');logs.className='settings-logs'
 const logHeading=document.createElement('h3');logHeading.textContent='Agent logs'
 const reload=document.createElement('button');reload.type='button';reload.textContent='Refresh'
 const logToolbar=document.createElement('div');logToolbar.className='agent-log-toolbar';logToolbar.append(logHeading,reload)
 const description=document.createElement('p');description.textContent='Diagnostics captured by this desktop app. Agents started elsewhere may write to their original terminal instead.'
 const source=document.createElement('p');source.className='agent-log-source'
 const logStatus=document.createElement('p');logStatus.setAttribute('role','status')
 const output=document.createElement('pre');output.className='agent-log-output';output.tabIndex=0;output.setAttribute('aria-label','Agent log contents')
 logs.append(logToolbar,description,source,logStatus,output)
 panels.Logs.append(logs)
 // A read that lands after the dialog is gone, or after another panel replaced
 // this one, writes into a detached node: isConnected is what tells them apart.
 const loadLog=async()=>{
  reload.disabled=true;output.textContent='';logStatus.textContent='Loading agent log…'
  try{
   const snapshot=await api.agentLogs()
   if(!output.isConnected)return
   source.textContent=snapshot.path
   logStatus.textContent=snapshot.missing?'No desktop agent log exists yet.':!snapshot.text?'The agent log is empty.':snapshot.truncated?'Showing the latest 256 KiB; earlier output omitted.':'Showing the current log snapshot.'
   output.textContent=logText(snapshot.text)
   output.scrollTop=output.scrollHeight
  }catch(err){if(output.isConnected)logStatus.textContent='Unable to read agent log: '+(err.message||String(err))}
  finally{if(reload.isConnected)reload.disabled=false}
 }
 reload.onclick=loadLog

 selectCategory(SETTINGS_CATEGORIES.some(category=>category.id===initial)?initial:'Profile')
 // The connection facts come from two sources the agent answers separately, and
 // a stopped agent still has a paired server to report: the stored settings fill
 // the panel first, the live status refines it when the agent answers.
 const fill=async()=>{
  let stored={}
  try{stored=await api.settings()}catch{}
  if(!account.value.isConnected)return
  account.value.textContent=stored.server||'Not paired'
  device.value.textContent=stored.deviceId||'Not paired'
  renderAgentActions()
  link.control.dataset.state='off'
  link.value.textContent=agentConnected?'Connecting…':'Unreachable'
  pairing.hint.textContent=stored.token
   ?'This workstation is paired. Pasting a new code re-pairs it.'
   :'In the web interface, under your profile, choose Pair a workstation and paste the code here.'
  pairingNote.textContent=agentConnected?'Stop the local agent before connecting it to another server.':''
  // The agent answers for the execution defaults; a start or a stop from the
  // connection panel reloads them, so the panel follows the agent's state.
  if(execution.providerSelect.isConnected)execution.load().catch(()=>{}).finally(()=>{if(execution.providerSelect.isConnected)mcpPanel.load()})
  if(!agentConnected)return
  try{
   const status=await api.status()
   if(!link.value.isConnected)return
   link.control.dataset.state=status.connected?'on':'off'
   link.value.textContent=status.connected?'Connected':status.contractError?'Server incompatible':'Server disconnected'
   if(status.contractError)link.hint.textContent=status.contractError
   if(status.server)account.value.textContent=status.server
  }catch{if(link.value.isConnected)link.value.textContent='Unreachable'}
 }
 updateSettingsConnection=status=>{
  renderAgentActions()
  link.control.dataset.state=agentConnected&&status.connected?'on':'off'
  link.value.textContent=!agentConnected?'Unreachable':status.connected?'Connected':status.contractError?'Server incompatible':'Server disconnected'
  pairingNote.textContent=agentConnected?'Stop the local agent before connecting it to another server.':''
 }
 fill()
}
document.querySelector('#settings').onclick=()=>openSettings('Profile')

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
 const button=document.querySelector('#toggle-sidebar'),mac=isMacPlatform(navigator)
 const chevron=hidden?'m13 9 3 3-3 3':'m16 9-3 3 3 3'
 button.setAttribute('aria-expanded',String(!hidden))
 button.setAttribute('aria-label',hidden?'Show projects':'Hide projects')
 button.setAttribute('aria-keyshortcuts',sidebarShortcutAria(mac))
 button.title=(hidden?'Show projects':'Hide projects')+' ('+sidebarShortcutLabel(mac)+')'
 button.innerHTML='<svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16"/><path d="'+chevron+'"/></svg>'
}
function toggleSidebar(){
 const hidden=document.querySelector('#workspace').classList.toggle('sidebar-hidden')
 renderSidebarToggle(hidden);localStorage.setItem('sidebarCollapsed',String(hidden));resize()
}
document.querySelector('#toggle-sidebar').onclick=toggleSidebar
// The Changelog pane: what is installed, and what changed. Both versions are
// shown because the app and the agent are distributed separately, and a
// workstation that upgraded one and not the other is exactly the case this
// pane exists to make visible.
function versionRow(label, value, help){
 const row=document.createElement('div');row.className='setting-row'
 const text=document.createElement('div');text.className='setting-text'
 const name=document.createElement('div');name.className='setting-name'
 const strong=document.createElement('strong');strong.textContent=label;name.append(strong)
 text.append(name)
 if(help){const p=document.createElement('p');p.textContent=help;text.append(p)}
 const control=document.createElement('div');control.className='setting-control'
 const version=document.createElement('code');version.className='version-value';version.textContent=value
 control.append(version);row.append(text,control)
 return row
}
function renderChangelog(container,releases){
 for(const release of releases){
  if(!release.sections.some(section=>section.items.length))continue
  const entry=document.createElement('section');entry.className='changelog-release'
  const heading=document.createElement('h3')
  heading.textContent=release.date?release.version+' · '+release.date:release.version
  entry.append(heading)
  for(const section of release.sections){
   if(!section.items.length)continue
   const title=document.createElement('h4');title.textContent=section.title
   const list=document.createElement('ul')
   for(const item of section.items){const li=document.createElement('li');li.textContent=item;list.append(li)}
   entry.append(title,list)
  }
  container.append(entry)
 }
 if(!container.childElementCount){
  const empty=document.createElement('p');empty.textContent='No release notes in this build.';container.append(empty)
 }
}
// The pane main built as the whole Settings dialog becomes one category of it:
// what is installed sits beside the account, the connection and the logs
// instead of replacing them.
// The mark stays until the running agent is the one this app bundles; the main
// process says when that changes, which is what the listener is for.
function showAgentOutdated(row,outdated){
 let mark=row.querySelector('.version-outdated')
 if(!outdated){mark?.remove();return}
 if(!mark){
  mark=document.createElement('span');mark.className='version-outdated'
  mark.textContent='outdated: restart the local agent'
  row.querySelector('.setting-control').append(mark)
 }
}
async function fillChangelogPanel(panel){
 const versions=document.createElement('div');versions.className='settings-versions'
 const desktopRow=versionRow('Sectile Desktop','…','The application window and its consoles.')
 const agentRow=versionRow('Local agent','…','The workstation daemon that runs the tasks.')
 versions.append(desktopRow,agentRow)
 const notesHeading=document.createElement('h3');notesHeading.className='changelog-heading';notesHeading.textContent='Release notes'
 const notes=document.createElement('div');notes.className='changelog'
 panel.append(versions,notesHeading,notes)

 const releases=parseChangelog(changelogSource)
 renderChangelog(notes,releases)

 let installed=null
 try{
  const reported=await api.version()
  // The panel may be gone by the time the agent answers: a detached node is
  // what says so, the same way the log reader tells a late read apart.
  if(!panel.isConnected)return
  installed=reported.desktop
  desktopRow.querySelector('.version-value').textContent=reported.desktop||'unknown'
  // A stopped agent has no version to give. Saying so beats leaving an
  // ellipsis that reads as a load which never finishes.
  agentRow.querySelector('.version-value').textContent=reported.agent||'not running'
  showAgentOutdated(agentRow,reported.outdated)
  const unsubscribe=api.onAgentOutdated?.(value=>{
   if(!agentRow.isConnected){unsubscribe?.();return}
   showAgentOutdated(agentRow,value)
  })
 }catch{
  if(!panel.isConnected)return
  desktopRow.querySelector('.version-value').textContent='unknown'
  agentRow.querySelector('.version-value').textContent='not running'
 }
 const current=releaseNotesFor(releases,installed)
 if(current)notesHeading.textContent='Release notes · '+current.version
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

  // A single Local panel had grown into one long scroll mixing the repository
  // path, execution limits and the agent command lines. Categories in a side
  // navigation name each group and keep the panel they open short, the way the
  // project modal of the web interface does.
  const CATEGORIES=[
   {id:'General',label:'General',saves:true,icon:'<path d="M4 7a2 2 0 0 1 2-2h3l2 2.5h7a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2Z"/>'},
   {id:'Execution',label:'Execution',saves:true,icon:'<path d="M4 7h16M4 17h16"/><circle cx="9" cy="7" r="2.4"/><circle cx="15" cy="17" r="2.4"/>'},
   {id:'Agent',label:'AI agent',saves:true,icon:'<rect x="4" y="8" width="16" height="11" rx="3"/><path d="M12 4v4M9 13h.01M15 13h.01"/>'},
   {id:'Deployment',label:'Deployment',saves:false,icon:'<path d="M12 20V7m0 0 4 4m-4-4-4 4"/><path d="M5 4h14"/>'},
   {id:'Server',label:'Server',saves:false,icon:'<rect x="4" y="5" width="16" height="6" rx="2"/><rect x="4" y="14" width="16" height="6" rx="2"/><path d="M8 8h.01M8 17h.01"/>'}
  ]
  const layout=document.createElement('div');layout.className='settings-layout'
  const tabs=document.createElement('div');tabs.className='settings-nav';tabs.setAttribute('role','tablist')
  tabs.setAttribute('aria-orientation','vertical');tabs.setAttribute('aria-label','Project settings categories')
  const content=document.createElement('div');content.className='settings-content'
  layout.append(tabs,content)
  const panels={}
  // The categories that store something share one form, so a single save keeps
  // the whole local configuration consistent whichever one is open.
  const form=document.createElement('form');form.id='project-local-form'
  // The save control lives in the dialog footer, so it stays in view whichever
  // storing category is open and however far its panel scrolls.
  const save=document.createElement('button');save.textContent='Save local configuration';save.className='dialog-action primary'
  save.setAttribute('form',form.id)
  dialogFooter.prepend(save);syncDialogFooter()
  function selectCategory(name){
   const stores=CATEGORIES.find(category=>category.id===name).saves
   for(const [key,value] of Object.entries(panels))value.hidden=key!==name
   form.hidden=!stores;save.hidden=!stores;syncDialogFooter()
   for(const item of tabs.children)item.setAttribute('aria-selected',String(item.dataset.category===name))
  }
  for(const category of CATEGORIES){
   const tab=document.createElement('button');tab.type='button';tab.setAttribute('role','tab');tab.dataset.category=category.id;tab.title=category.label
   tab.innerHTML='<svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">'+category.icon+'</svg>'
   const text=document.createElement('span');text.className='settings-nav-label';text.textContent=category.label;tab.append(text)
   tab.id='project-tab-'+category.id;tab.setAttribute('aria-controls','project-panel-'+category.id)
   const panel=document.createElement('section');panel.id='project-panel-'+category.id
   panel.setAttribute('role','tabpanel');panel.setAttribute('aria-labelledby',tab.id);panels[category.id]=panel
   tab.onclick=()=>selectCategory(category.id)
   tabs.append(tab)
  }
  dialogBody.append(layout)
  content.append(form)
  for(const category of CATEGORIES){
   if(category.saves)form.append(panels[category.id]);else content.append(panels[category.id])
  }
  selectCategory('General')
  const path=document.createElement('input');path.value=info.path||'';path.required=true;path.placeholder='/path/to/repository';path.setAttribute('aria-label','Local repository')
  const browse=document.createElement('button');browse.type='button';browse.textContent='Choose folder…'
  browse.onclick=async()=>{try{const selected=await api.chooseRepository();if(selected){path.value=selected;pathOffer.examine()}}catch(err){error(err)}}
  const picker=document.createElement('div');picker.className='repository-picker';picker.append(path,browse)
  const repository=settingRow('Local repository',{stacked:true},picker)
  const pathOffer=attachGitOffer(path,repository,'Local repository')
  // Macro operations read and write specifications here. Only an override is
  // stored: a mono-repo project inherits its local repository, a multi-repo
  // project needs one. The layout itself is a project setting held by the
  // server, so it is stated here and changed in the web interface.
  const layoutRow=readOnlyRow('Repository layout','Project setting · Change it in the project settings of the web interface.')
  const specPath=document.createElement('input');specPath.value=info.specPath||'';specPath.setAttribute('aria-label','Specifications folder')
  const specBrowse=document.createElement('button');specBrowse.type='button';specBrowse.textContent='Choose folder…';specBrowse.setAttribute('aria-label','Choose specifications folder…')
  specBrowse.onclick=async()=>{try{const selected=await api.chooseRepository();if(selected){specPath.value=selected;renderSpec({...specData,specPath:selected});specOffer.examine()}}catch(err){error(err)}}
  const specPicker=document.createElement('div');specPicker.className='repository-picker';specPicker.append(specPath,specBrowse)
  // Mono-repo only: the specifications either share the code checkout or live
  // in a folder of their own. Unticking reveals the folder; ticking drops the
  // override so the folder follows the local repository again.
  const sameRepo=document.createElement('input');sameRepo.type='checkbox';sameRepo.id='spec-same-repository'
  const sameRepoLabel=document.createElement('label');sameRepoLabel.className='spec-same-repository';sameRepoLabel.htmlFor=sameRepo.id
  sameRepoLabel.append(sameRepo,document.createTextNode(' Specifications live in the code repository'))
  sameRepo.onchange=()=>{
   if(sameRepo.checked)specPath.value=''
   renderSpec({...specData,specPath:specPath.value},{separate:!sameRepo.checked})
   if(!sameRepo.checked)specPath.focus()
  }
  specPath.oninput=()=>renderSpec({...specData,specPath:specPath.value},{separate:true})
  const specKind=document.createElement('span');specKind.className='spec-kind';specKind.setAttribute('role','status');specKind.setAttribute('aria-label','Specifications folder kind')
  const specRepository=settingRow('Specifications folder',{stacked:true},sameRepoLabel,specPicker,specKind)
  const specOffer=attachGitOffer(specPath,specRepository,'Specifications folder',folder=>renderSpec({...specData,specPath:folder},{separate:true}))
  let specData=info
  function renderSpec(data,options){
   specData=data
   const mono=data.monoRepo!==false,override=!!(data.specPath||'').trim()
   const separate=!mono||override||!!options?.separate
   const required=!mono&&!override
   layoutRow.value.textContent=mono?'Mono-repo':'Multi-repo'
   sameRepoLabel.hidden=!mono;sameRepo.checked=mono&&!separate
   // Following the local repository, the folder has no offer of its own.
   if(specPicker.hidden!==!separate)specOffer.show(separate)
   specPicker.hidden=!separate
   specPath.placeholder=mono?'Folder holding the specifications':'Required for a multi-repo project'
   specRepository.section.classList.toggle('required',required)
   specPath.setAttribute('aria-invalid',required?'true':'false')
   specRepository.hint.textContent=required?'Required · Macro operations of a multi-repo project need this folder. Settings can still be saved without it.'
    :!separate?'Macro skills read and write specifications in the local repository.'
    :mono?'A folder of its own · Tick the box to use the local repository again.'
    :'Where macro skills read and write specifications.'
   // The kind is detected on the folder the agent resolves, which is not
   // always the one typed: name it, so the verdict says what it is about.
   // A value typed but not saved yet has not been examined.
   const stored=(data.specPath||'').trim()===(info.specPath||'').trim()
   const examined=(stored?(data.specPath||'').trim():'')||(!override?data.specDefault:'')||''
   const verdict={git:'Git repository',folder:'Folder, not a Git repository',missing:'Folder not found'}[stored?data.specKind:'']||''
   specKind.textContent=verdict&&examined?verdict+' · '+examined:''
   specKind.title=specKind.textContent
   specKind.dataset.kind=stored?data.specKind||'':''
  }
  renderSpec(info)
  pathOffer.examine();specOffer.show(!specPicker.hidden)
  // The other repositories of a multi-repo project, each in a folder of this
  // workstation (#456). The project's own repository is the local repository
  // above; the list itself is a project setting of the web interface.
  const repositoryList=document.createElement('div');repositoryList.className='repository-list'
  const repositoriesRow=settingRow('Other repositories',{stacked:true},repositoryList)
  repositoriesRow.hint.textContent='Tasks pinned to one of these repositories run in a worktree of its folder; the others are given to the agent as read-only context.'
  repositoriesRow.section.hidden=true
  const repositoryInputs=[]
  if(info.monoRepo===false){
   api.repositories(id).then(list=>{
    for(const repository of list.filter(item=>!item.code)){
     const input=document.createElement('input');input.value=repository.path||'';input.placeholder='Folder holding a checkout of '+repository.identity
     input.setAttribute('aria-label','Folder of '+repository.identity)
     const choose=document.createElement('button');choose.type='button';choose.textContent='Choose folder…';choose.setAttribute('aria-label','Choose the folder of '+repository.identity+'…')
     choose.onclick=async()=>{try{const selected=await api.chooseRepository();if(selected)input.value=selected}catch(err){error(err)}}
     const name=document.createElement('span');name.className='repository-name';name.textContent=repository.identity
     const row=document.createElement('div');row.className='repository-picker';row.append(input,choose)
     repositoryList.append(name,row)
     repositoryInputs.push({repository,input})
    }
    repositoriesRow.section.hidden=!repositoryInputs.length
   }).catch(error)
  }
  // Every execution field reads {value, inherited, source} from the agent: set
  // for the project, or inherited from the workstation defaults, else the
  // provider default. The server never supplies one (#305).
  let fields=projectFields(info)
  const skills=(info.skills||[]).map(skill=>skill.id).filter(Boolean)
  // The workstation defaults tell, once a project value is reset, whether the
  // inherited value comes from the workstation or from the provider default.
  let wsView=null
  try{wsView=await api.workstationSettings()}catch{}
  const statedValue=value=>value!==undefined&&value!==null&&value!==''&&value!==0&&!(typeof value==='object'&&!Array.isArray(value)&&!Object.keys(value).length)
  function inheritedSource(name){
   const field=fields[name]
   if(field.source!=='project')return field.source
   const defaults=wsView?.defaults
   if(!defaults)return 'workstation'
   if(name==='skillCommands')return 'default'
   return statedValue(defaults[name])?'workstation':'default'
  }
  const hintFor=(name,inherit,empty)=>sourceHint(inherit?inheritedSource(name):'project',fields[name].inherited,empty)
  const inherits=name=>fields[name].source!=='project'

  let useWorktrees=fields.useWorktrees.value??true,inheritWorktrees=inherits('useWorktrees')
  let parallelism=Number(fields.parallelism.value)||1,inheritParallelism=inherits('parallelism')
  const controls={}
  const worktreeGroup=document.createElement('div');worktreeGroup.className='segmented'
  worktreeGroup.setAttribute('role','group');worktreeGroup.setAttribute('aria-label','Worktrees')
  const worktreeButtons=['Yes','No'].map(value=>{
   const button=document.createElement('button');button.type='button';button.textContent=value
   button.onclick=()=>{useWorktrees=value==='Yes';inheritWorktrees=false;update()}
   worktreeGroup.append(button);return button
  })
  const resetWorktrees=()=>{useWorktrees=fields.useWorktrees.inherited??useWorktrees;inheritWorktrees=true;update()}
  controls.worktrees=settingRow('Worktrees',{resetLabel:'Reset worktrees to workstation default',onReset:resetWorktrees},worktreeGroup)
  controls.worktrees.buttons=worktreeButtons
  // Whether the tasks' clarifications and specifications are committed (#487).
  // No override follows the server; the reset removes the override.
  let specArtifacts=info.specArtifacts==='drop'?'drop':'keep',inheritSpecArtifacts=!info.specArtifactsOverride
  const specArtifactsGroup=document.createElement('div');specArtifactsGroup.className='segmented'
  specArtifactsGroup.setAttribute('role','group');specArtifactsGroup.setAttribute('aria-label','Specifications')
  const specArtifactsButtons=['Keep','Drop'].map(value=>{
   const button=document.createElement('button');button.type='button';button.textContent=value
   button.onclick=()=>{specArtifacts=value.toLowerCase();inheritSpecArtifacts=false;update()}
   specArtifactsGroup.append(button);return button
  })
  controls.specArtifacts=settingRow('Specifications',{resetLabel:'Reset specifications to server default',onReset:()=>{specArtifacts=config.specArtifacts==='drop'?'drop':'keep';inheritSpecArtifacts=true;update()}},specArtifactsGroup)
  controls.specArtifacts.buttons=specArtifactsButtons
  const specArtifactsWarning=document.createElement('p');specArtifactsWarning.className='setting-warning'
  controls.specArtifacts.hint.after(specArtifactsWarning)
  // A magnitude between 1 and a ceiling, which a segmented control cannot show
  // without overflowing the row once the ceiling grows. The readout states the
  // value, so the slider needs no printed scale beneath it.
  const parallelInput=document.createElement('input');parallelInput.type='range'
  parallelInput.min='1';parallelInput.max=String(MAX_PARALLELISM);parallelInput.step='1'
  parallelInput.className='slider-input';parallelInput.setAttribute('aria-label','Parallel executions')
  parallelInput.oninput=()=>{parallelism=Number(parallelInput.value);inheritParallelism=false;update()}
  const parallelReadout=document.createElement('span');parallelReadout.className='slider-value'
  const resetParallelism=()=>{parallelism=Number(fields.parallelism.inherited)||parallelism;inheritParallelism=true;update()}
  controls.parallel=settingRow('Parallel executions',{resetLabel:'Reset parallel executions to workstation default',onReset:resetParallelism},parallelInput,parallelReadout)
  controls.parallel.input=parallelInput;controls.parallel.readout=parallelReadout
  function update(){
   controls.worktrees.buttons.forEach((button,i)=>button.setAttribute('aria-pressed',String(useWorktrees===(i===0))))
   controls.worktrees.hint.textContent=hintFor('useWorktrees',inheritWorktrees)
   controls.specArtifacts.buttons.forEach(button=>button.setAttribute('aria-pressed',String(specArtifacts===button.textContent.toLowerCase())))
   controls.specArtifacts.hint.textContent=(inheritSpecArtifacts?'Inherited':'Local override')+' · Server default: '+(config.specArtifacts==='drop'?'Drop':'Keep')
   const tracked=info.specArtifactsTracked||0
   specArtifactsWarning.textContent=specArtifacts==='drop'&&tracked>0?'This repository already tracks '+tracked+' specification '+(tracked===1?'file':'files')+'. They stay in its history; only the next tasks\' specifications are dropped.':''
   specArtifactsWarning.hidden=!specArtifactsWarning.textContent
   const effective=useWorktrees?parallelism:1
   controls.parallel.input.disabled=!useWorktrees
   controls.parallel.input.value=String(effective)
   controls.parallel.readout.textContent=effective+(effective===1?' execution':' executions')
   controls.parallel.hint.textContent=useWorktrees?hintFor('parallelism',inheritParallelism)+' · Extra executions queue locally.':'Without worktrees, executions are limited to one.'
  }
  update()

  // The providers set up beside the one that runs, when a project is
  // initialized. An empty list is the statement "none".
  let setupProviders=Array.isArray(fields.setupProviders.value)?[...fields.setupProviders.value]:[],inheritSetupProviders=inherits('setupProviders')
  const setupBox=document.createElement('div');setupBox.className='setup-providers'
  const setupChecks={}
  for(const provider of wsView?.setupProviders?.length?wsView.setupProviders:SETUP_PROVIDERS){
   const label=document.createElement('label');label.className='checkbox-label'
   const box=document.createElement('input');box.type='checkbox';box.setAttribute('aria-label','Set up '+provider)
   box.onchange=()=>{setupProviders=Object.entries(setupChecks).filter(([,input])=>input.checked).map(([key])=>key);inheritSetupProviders=false;updateSetup()}
   setupChecks[provider]=box;label.append(box,document.createTextNode(' '+provider));setupBox.append(label)
  }
  const resetSetup=()=>{setupProviders=[...(fields.setupProviders.inherited||[])];inheritSetupProviders=true;updateSetup()}
  const setupRow=settingRow('Extra setup providers',{resetLabel:'Reset setup providers to workstation default',onReset:resetSetup},setupBox)
  function updateSetup(){
   for(const [provider,box] of Object.entries(setupChecks))box.checked=setupProviders.includes(provider)
   setupRow.hint.textContent=hintFor('setupProviders',inheritSetupProviders,'None')
  }
  updateSetup()

  // The project default engine (#510): an engine of the workstation catalogue,
  // or the workstation default engine. Engines themselves are edited in
  // Settings; an agent without a catalogue shows no selector.
  const selectedProvider=info.aiProvider||DEFAULT_PROVIDER
  let engineView=null
  if(await taskEnginesAvailable()){try{engineView=await api.engines()}catch{}}
  let defaultEngine=fields.defaultEngine.source==='project'?fields.defaultEngine.value||'':'',inheritDefaultEngine=inherits('defaultEngine')
  const engineSelect=document.createElement('select');engineSelect.className='provider-select';engineSelect.setAttribute('aria-label','Default engine')
  const engineRow=settingRow('Default engine',{resetLabel:'Reset default engine to the workstation default'},engineSelect)
  engineRow.section.hidden=!engineView
  const engineById=engineId=>engineView?.catalogue.find(engine=>engine.id===engineId)
  function fillEngines(){
   engineSelect.replaceChildren()
   const workstation=engineById(engineView?.default)
   const inherit=document.createElement('option');inherit.value='';inherit.textContent='Inherit the workstation default ('+(workstation?.name||'none')+')'
   engineSelect.append(inherit)
   for(const engine of engineView?.catalogue||[]){const option=document.createElement('option');option.value=engine.id;option.textContent=engine.name;engineSelect.append(option)}
   engineSelect.value=inheritDefaultEngine||!engineById(defaultEngine)?'':defaultEngine
  }
  function updateEngine(){
   const engine=engineById(inheritDefaultEngine?engineView?.default:defaultEngine)
   const detail=engine?' · '+engine.provider+' · '+(engine.model||'provider default'):''
   engineRow.hint.textContent=(inheritDefaultEngine?'Inherited from workstation':'Set for this project')+detail
  }
  const resetEngine=()=>{defaultEngine='';inheritDefaultEngine=true;engineSelect.value='';updateEngine()}
  engineRow.reset.onclick=resetEngine
  engineSelect.onchange=()=>{defaultEngine=engineSelect.value;inheritDefaultEngine=!defaultEngine;updateEngine()}
  fillEngines();updateEngine()

  // The slash command each stage runs, when the local CLI installs it under
  // another name. One word, an optional leading slash.
  let inheritSkillCommands=inherits('skillCommands')
  const skillCommands=entryList({keyLabel:'Skill',valueLabel:'Command for skill',addLabel:'Add a skill command',fixed:skills,validate:validSkillCommand,
   placeholder:skill=>fields.skillCommands.inherited?.[skill]||'Standard command',onChange:()=>{inheritSkillCommands=false;updateSkillCommands()}})
  skillCommands.set(ownEntries(fields.skillCommands))
  const resetSkillCommands=()=>{skillCommands.set({});inheritSkillCommands=true;updateSkillCommands()}
  const skillCommandsRow=settingRow('Skill command names',{stacked:true,resetLabel:'Reset skill command names to the standard ones',onReset:resetSkillCommands},skillCommands.box)
  skillCommandsRow.section.hidden=!skills.length
  function updateSkillCommands(){
   skillCommands.refreshPlaceholders()
   skillCommandsRow.hint.textContent=skillCommands.invalid()?'A skill command name is a single word, optionally led by /.':inheritSkillCommands?'Standard commands':'Set for this project · Empty entries run the standard command.'
  }
  updateSkillCommands()

  let inheritTerminal=inherits('terminal')
  const terminal=terminalPicker(()=>{inheritTerminal=false;updateTerminal()})
  terminal.set(fields.terminal.value||'')
  const terminalRow=settingRow('Terminal emulator',{resetLabel:'Reset terminal emulator to workstation default'},terminal.select,terminal.custom)
  const terminalReset=terminalRow.reset,terminalHint=terminalRow.hint
  function updateTerminal(){
    terminalHint.textContent=hintFor('terminal',inheritTerminal,'Auto-detect')
  }
  const resetTerminal=()=>{terminal.set(fields.terminal.inherited||'');inheritTerminal=true;updateTerminal()}
  terminalReset.onclick=resetTerminal
  updateTerminal()
  // After a save, the agent's answer is the reference: sources and inherited
  // values are read again, and inherited fields take the value they inherit
  // now. A refresh keeps the dialog's own edits: only the fields still
  // inheriting in the dialog take the refreshed inherited value.
  function applyFields(fresh,keepEdits=false){
   fields=projectFields(fresh)
   if(!keepEdits){
    inheritSpecArtifacts=!fresh.specArtifactsOverride
    specArtifacts=fresh.specArtifacts==='drop'?'drop':'keep'
    inheritWorktrees=inherits('useWorktrees');inheritParallelism=inherits('parallelism');inheritSetupProviders=inherits('setupProviders')
    inheritDefaultEngine=inherits('defaultEngine');defaultEngine=inheritDefaultEngine?'':fields.defaultEngine.value||''
    inheritSkillCommands=inherits('skillCommands');inheritTerminal=inherits('terminal')
   }
   if(inheritWorktrees)resetWorktrees()
   if(inheritParallelism)resetParallelism()
   if(inheritSetupProviders)resetSetup()
   if(inheritTerminal)resetTerminal()
   fillEngines()
   update();updateSetup();updateEngine();updateSkillCommands();updateTerminal()
  }

  const notice=document.createElement('p');notice.setAttribute('role','status')
  panels.General.append(repository.section,layoutRow.section,specRepository.section,repositoriesRow.section)
  panels.Execution.append(controls.worktrees.section,controls.specArtifacts.section,controls.parallel.section,terminalRow.section,setupRow.section)
  panels.Agent.append(engineRow.section,skillCommandsRow.section)
  const remove=document.createElement('button');remove.type='button';remove.textContent='Remove from desktop';remove.className='remove-project'
  remove.onclick=()=>requestRemoveProject(id,config.projectName)
  if(info.configured||runs.some(run=>run.projectId===id))panels.General.append(remove)
  content.append(notice)
  const tools=document.createElement('div');tools.className='deployment-actions'
  form.onsubmit=async event=>{
   event.preventDefault()
   if(skillCommands.invalid()){
    notice.textContent='A skill command name is a single word, optionally led by /.'
    return
   }
   save.disabled=true
   try{
    await api.mapProject({projectId:id,path:path.value,specPath:specPath.value.trim(),
     useWorktrees,inheritWorktrees,specArtifacts,inheritSpecArtifacts,parallelism,inheritParallelism,
     ...(engineView?{defaultEngine,inheritDefaultEngine}:{}),
     terminal:terminal.get(),inheritTerminal,
     setupProviders:[...setupProviders],inheritSetupProviders,
     skillCommands:compact(skillCommands.get()),inheritSkillCommands})
    // Each repository folder is checked against its origin by the agent, so
    // a wrong folder is refused by name rather than saved. The settings above
    // are saved by then, which the notice says rather than hiding it.
    const refused=[]
    for(const entry of repositoryInputs){
     const value=entry.input.value.trim()
     if(value===(entry.repository.path||''))continue
     try{await api.mapRepository({projectId:id,repository:entry.repository.identity,path:value});entry.repository.path=value}
     catch(err){refused.push(entry.repository.identity+': '+err.message)}
    }
    projectStateVersion++;disconnectedProjects.delete(id)
    notice.textContent=refused.length?'Local configuration saved, except the folder of '+refused.join('; '):'Local configuration saved'
    // The agent normalised the folder and detected its kind: show what it
    // stored, not what was typed.
    try{const fresh=await api.project(id);info.specPath=fresh.specPath||'';specPath.value=info.specPath;renderSpec(fresh);applyFields(fresh)}catch(err){error(err)}
    await loadProjects()
    for(const button of tools.querySelectorAll('button'))button.disabled=false
   }catch(err){notice.textContent=err.message}finally{save.disabled=false}
  }
  const initProvider=document.createElement('select');initProvider.setAttribute('aria-label','Initialization provider')
  for(const [value,label] of [['agy','Antigravity'],['claude','Claude'],['codex','Codex'],['cursor','Cursor'],['gemini','Gemini'],['vibe','Vibe']]){
   const option=document.createElement('option');option.value=value;option.textContent=label;initProvider.append(option)
  }
  initProvider.value=[...initProvider.options].some(option=>option.value===selectedProvider)?selectedProvider:'agy'
  const initRow=settingRow('Initialization provider',{},initProvider)
  initRow.hint.textContent='Initialize this provider’s server skills and MCP connection. You can run this again at any time.'
  const initResult=document.createElement('div');initResult.setAttribute('role','status');initResult.className='initialization-result'
  panels.Deployment.append(initRow.section,initResult)
  for(const [action,title] of [['initialize','Initialize'],['framework','Deploy SDD framework']]){
   const button=document.createElement('button');button.textContent=title;button.disabled=!info.configured
   button.onclick=async()=>{
    for(const item of tools.querySelectorAll('button'))item.disabled=true
    initProvider.disabled=true
    notice.textContent=action==='initialize'?'Initialization in progress…':'Deployment in progress…'
    if(action==='initialize')initResult.replaceChildren()
    try{
     const result=await api.deployProject(id,action,action==='initialize'?initProvider.value:undefined)
     notice.textContent=result.message||'Deployment complete'
     if(action==='initialize'){
      for(const [label,step] of [['MCP',result.mcp],['Skills',result.skills]]){
       const line=document.createElement('p');line.textContent=label+': '+({success:'Success',failed:'Failed',skipped:'Skipped',not_run:'Not run'}[step.status]||step.status)+' - '+step.message;initResult.append(line)
      }
     }
    }catch(err){notice.textContent=err.message}finally{initProvider.disabled=false;for(const item of tools.querySelectorAll('button'))item.disabled=false}
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
  layout.before(reload)
  reload.onclick=async()=>{
   reload.disabled=true;notice.textContent='Refreshing server settings…'
   try{
    const fresh=await api.project(id)
    if(!reload.isConnected)return
    config=fresh.server
    dialogBody.querySelector('h2').textContent=config.projectName
    if(inheritSpecArtifacts)specArtifacts=config.specArtifacts==='drop'?'drop':'keep'
    applyFields(fresh,true);renderServer(fresh.monoRepo);renderSpec({...fresh,specPath:specPath.value})
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
 'clear-history':'<path d="m15 3-6 9M7 11l7 5M8 12c-3 1-5 4-5 8h12c0-3-1-5-3-6M6 20l2-5M10 20l1-4"/>',
 'detach-terminal':'<path d="M17 13.5V18a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V9a2 2 0 0 1 2-2h4.5"/><path d="m7.5 11.5 2 2-2 2"/><path d="M15 4h5v5"/><path d="m13.5 10.5 6.5-6.5"/>',
 'view-console':'<rect x="3" y="4" width="18" height="16" rx="2"/><path d="m7.5 10 2.5 2.5-2.5 2.5"/><path d="M13 15h4"/>',
 'view-changes':'<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5"/><path d="M12 11v5"/><path d="M9.5 13.5h5"/><path d="M9.5 18.5h5"/>',
 rerun:'<path d="M3.5 12a8.5 8.5 0 0 1 14.4-6.1L20.5 8"/><path d="M20.5 3.5V8h-4.5"/><path d="m10 9.4 5 2.9-5 2.9Z"/>',
 'save-log':'<path d="M12 3v11"/><path d="m7.5 10 4.5 4 4.5-4"/><path d="M5 20h14"/>',
 stop:'<circle cx="12" cy="12" r="9"/><path d="m8.2 12.4 2.6 2.6 5-5.4"/>',
 'next-step':'<path d="m9 18 6-6-6-6"/>',
 'open-editor':'<path d="m8 7-5 5 5 5"/><path d="m16 7 5 5-5 5"/><path d="m13.5 4-3 16"/>',
 'pickup-chain':'<path d="m6 17 5-5-5-5"/><path d="m13 17 5-5-5-5"/>',
 shutdown:'<path d="M12 4v8"/><path d="M7.4 7.4a6.5 6.5 0 1 0 9.2 0"/>',
 restart:'<path d="M20 7v5h-5M20 12a8 8 0 1 0-2 5M20 7v5"/>',
 settings:'<circle cx="12" cy="12" r="3"/><path d="M19.4 14.5a1.6 1.6 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.6 1.6 0 0 0-1.8-.3 1.6 1.6 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.2a1.6 1.6 0 0 0-1-1.4 1.6 1.6 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.6 1.6 0 0 0 .3-1.8 1.6 1.6 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.2a1.6 1.6 0 0 0 1.4-1 1.6 1.6 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.6 1.6 0 0 0 1.8.3H9a1.6 1.6 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.2a1.6 1.6 0 0 0 1 1.5 1.6 1.6 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.6 1.6 0 0 0-.3 1.8V9a1.6 1.6 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.2a1.6 1.6 0 0 0-1.4 1Z"/>'
}
for(const [id,paths] of Object.entries(iconPaths)){
 document.getElementById(id).innerHTML='<svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">'+paths+'</svg>'
}
if(localStorage.getItem('sidebarCollapsed')==='true')document.querySelector('#workspace').classList.add('sidebar-hidden')
renderSidebarToggle(document.querySelector('#workspace').classList.contains('sidebar-hidden'))

function newProjectTask(projectID){
 selectedProject=projectID
 showDialog('New task')
 const existing=document.createElement('button');existing.className='discovered-project';existing.textContent='Run an existing ticket'
 existing.onclick=()=>openTickets(projectID)
 const create=document.createElement('button');create.className='discovered-project';create.textContent='Quick add task'
 create.onclick=()=>quickAdd(projectID)
 dialogBody.append(existing,create);existing.focus()
}

// The tickets pane replaces the console the way the agent-log pane does: one
// project at a time, its open tasks as a table, and the console back on close.
const ticketsPane=document.querySelector('#tickets-pane')
let ticketsView=null
function closeTickets(restoreFocus=true){
 if(!ticketsOpen)return
 const projectID=ticketsView?.projectID,opener=ticketsView?.opener
 ticketsView?.closeOpenMenu?.()
 ticketsOpen=false;ticketsView=null;ticketsPane.hidden=true;ticketsPane.replaceChildren()
 document.querySelector('#workspace article').hidden=false
 resize()
 // The opener is where focus belongs, but only while it can still take it: a
 // dialog button stays in the DOM after its dialog closes, and a project
 // without a local path has no sidebar icon. The fallbacks keep focus inside
 // the app instead of dropping it on the body.
 if(!restoreFocus)return
 const reachable=element=>element?.isConnected&&element.offsetParent!==null
 const target=reachable(opener)?opener:document.querySelector('.project-more[data-project-id="'+CSS.escape(projectID||'')+'"]')||document.querySelector('#command-palette')
 if(reachable(target))target.focus()
}
async function openTickets(projectID,initialQuery=''){
 selectedProject=projectID
 if(!agentConnected){showDialog('Tickets');paragraph('Connect to the local agent to browse this project\u2019s tickets.');return}
 const opener=document.activeElement
 if(dialog.open)dialog.close()
 const project=projects.find(item=>item.id===projectID)
 const view={projectID,projectName:project?.name||projectID,sort:{...DEFAULT_SORT},tasks:[],info:null,query:'',rows:new Map(),submitting:new Set(),compose:null,generation:0,opener:opener&&opener!==document.body?opener:null}
 ticketsOpen=true;ticketsView=view;ticketsPane.hidden=false;ticketsPane.replaceChildren()
 document.querySelector('#workspace article').hidden=true
 document.querySelector('#workspace').hidden=false;document.querySelector('#setup').hidden=true
 const heading=document.createElement('h2');heading.textContent='Tickets · '+view.projectName
 const close=document.createElement('button');close.type='button';close.textContent='Close tickets';close.onclick=()=>closeTickets()
 const toolbar=document.createElement('div');toolbar.className='tickets-toolbar';toolbar.append(heading,close)
 const search=document.createElement('form'),query=document.createElement('input'),submit=document.createElement('button')
 query.placeholder='Search by title or task key';query.setAttribute('aria-label','Search server tasks');submit.textContent='Search'
 query.value=initialQuery
 search.append(query,submit)
 const status=document.createElement('p');status.className='tickets-status';status.setAttribute('role','status')
 const list=document.createElement('div');list.className='tickets-list'
 view.status=status;view.list=list
 ticketsPane.append(toolbar,search,status,list)
 query.focus()
 async function load(){
  const current=++view.generation,searchText=query.value.trim()
  const isCurrent=()=>current===view.generation&&ticketsView===view&&ticketsOpen
  list.textContent='Loading open tasks…';list.setAttribute('aria-busy','true');view.rows.clear();view.compose=null;status.textContent=''
  try{
   // The engine of each task (#510), when the agent keeps one: an older agent
   // answers nothing, and the table shows no engine column.
   const engines=taskEnginesAvailable().then(ok=>ok?api.taskEngines(projectID):null).catch(()=>null)
   const [tasks,info,taskEngines]=await Promise.all([api.serverTasks(projectID,searchText,true),api.project(projectID),engines])
   if(!isCurrent())return
   view.tasks=tasks.filter(task=>!isFinishedTask(task));view.info=info;view.query=searchText;view.engines=taskEngines
   renderTicketsTable(view)
  }catch(err){if(isCurrent()){const message=document.createElement('p');message.setAttribute('role','alert');message.textContent='Could not load open tasks: '+err.message+'. Use Search to retry.';list.replaceChildren(message)}}
  finally{if(isCurrent())list.setAttribute('aria-busy','false')}
 }
 view.load=load
 search.onsubmit=event=>{event.preventDefault();load()}
 await load()
}
const TICKET_COLUMNS=[['state',''],['key','Key'],['title','Title'],['stage','Stage'],['priority','Priority'],['pr','PR'],['actions','Actions']]
// The engine column sits before the PR one, and only with an agent that keeps
// a per-task engine (#510).
function ticketColumns(view){
 if(!view.engines)return TICKET_COLUMNS
 const columns=[...TICKET_COLUMNS]
 columns.splice(columns.findIndex(([field])=>field==='pr'),0,['engine','Engine'])
 return columns
}
function renderTicketsTable(view,focusField=null){
 const {list,tasks,info,sort}=view
 view.closeOpenMenu?.()
 // Sorting or searching rebuilds the rows. Instructions being typed are the
 // user's work, not render state, so they survive the rebuild.
 const pending=view.compose?{...view.compose}:null
 list.replaceChildren();view.rows.clear();view.compose=null
 if(!info.configured){const notice=document.createElement('p');notice.textContent='Configure a local repository before launching tasks.';list.append(notice)}
 if(!tasks.length){const empty=document.createElement('p');empty.textContent=view.query?'No matching open tasks':'No open tasks in this project';list.append(empty);return}
 const table=document.createElement('table');table.className='tickets-table'
 const caption=document.createElement('caption');caption.className='visually-hidden';caption.textContent='Open tasks in '+view.projectName
 const head=document.createElement('thead'),headRow=document.createElement('tr')
 for(const [field,label] of ticketColumns(view)){
  const cell=document.createElement('th');cell.scope='col';cell.dataset.column=field
  if(SORTABLE_FIELDS.includes(field)){
   const active=sort.field===field
   cell.setAttribute('aria-sort',active?(sort.ascending?'ascending':'descending'):'none')
   const button=document.createElement('button');button.type='button';button.className='sort-header';button.dataset.field=field
   button.append(document.createTextNode(label))
   // The arrow is decoration: aria-sort on the header already says which way the column reads.
   const arrow=document.createElement('span');arrow.className='sort-arrow';arrow.setAttribute('aria-hidden','true');arrow.textContent=active?(sort.ascending?'▲':'▼'):''
   button.append(arrow)
   button.onclick=()=>{view.sort=nextSort(view.sort,field);renderTicketsTable(view,field)}
   cell.append(button)
  }else{
   cell.textContent=label
   if(field==='state')cell.setAttribute('aria-label','Execution state')
  }
  headRow.append(cell)
 }
 head.append(headRow)
 const body=document.createElement('tbody')
 for(const task of orderedTasks(tasks,sort))body.append(ticketRow(view,task))
 table.append(caption,head,body);list.append(table)
 if(pending&&view.rows.has(pending.taskId))openCompose(view,view.rows.get(pending.taskId),pending,false)
 if(focusField)list.querySelector('.sort-header[data-field="'+focusField+'"]')?.focus()
}
function ticketRow(view,task){
 const key=task.key||task.id
 const row=document.createElement('tr');row.className='ticket-row';row.dataset.taskId=task.id
 const cell=(className,text)=>{const td=document.createElement('td');td.className=className;if(text!==undefined)td.textContent=text;return td}
 const stateCell=cell('ticket-state'),state=document.createElement('span');state.className='run-state';stateCell.append(state)
 const titleCell=cell('ticket-title',task.title||'');titleCell.title=task.title||''
 const priorityCell=cell('ticket-priority')
 const dot=document.createElement('span');dot.className='priority-dot';dot.dataset.priority=String(task.priority||'').toLowerCase();dot.setAttribute('aria-hidden','true')
 priorityCell.append(dot,document.createTextNode(task.priority||'-'))
 const prCell=cell('ticket-pr')
 if(task.prUrl&&/^https?:\/\//i.test(task.prUrl)){
  const pr=document.createElement('button');pr.type='button';pr.className='pr-indicator'
  const link=currentPullRequest(task);renderPullRequestIndicator(pr,link,prLabel(link.url)+' for '+key)
  pr.onclick=()=>api.openPR(link.url).catch(error);prCell.append(pr)
 }
 const actions=cell('ticket-actions')
 const run=document.createElement('button');run.type='button';run.className='ticket-run'
 const more=document.createElement('button');more.type='button';more.className='ticket-more';more.textContent='…'
 more.setAttribute('aria-haspopup','menu');more.setAttribute('aria-expanded','false');more.setAttribute('aria-label','More actions for '+key)
 const menu=document.createElement('div');menu.className='ticket-menu';menu.setAttribute('role','menu');menu.setAttribute('aria-label','Actions for '+key);menu.hidden=true
 const entry={task,row,state,run,more,menu,actions}
 // The menu is a popup, so it must swallow the gestures that dismiss a popup.
 // Focus can sit on the opener rather than inside the menu - a mouse click on a
 // project with no launchable skill leaves it there - and the menu's own
 // handlers never see the event then, so the dismissal is bound to both and an
 // outside pointer press is watched while the menu is open.
 let dismiss=null
 const closeMenu=(focusOpener=false)=>{
  if(menu.hidden)return
  menu.hidden=true;more.setAttribute('aria-expanded','false')
  if(dismiss){document.removeEventListener('pointerdown',dismiss,true);dismiss=null}
  if(view.closeOpenMenu===closeMenu)view.closeOpenMenu=null
  if(focusOpener)more.focus()
 }
 const openMenu=()=>{
  view.closeOpenMenu?.()
  const activeRun=runs.find(r=>r.taskId===task.id&&r.status==='running'&&!r.externalTerminal)
  let detachBtn=menu.querySelector('.ticket-detach')
  if(activeRun){
   if(!detachBtn){
    detachBtn=document.createElement('button');detachBtn.type='button';detachBtn.className='ticket-detach';detachBtn.setAttribute('role','menuitem');detachBtn.textContent='Detach to native terminal'
    const discussItem=menu.querySelector('[data-skill-id="discuss"]')
    if(discussItem)menu.insertBefore(detachBtn,discussItem)
    else menu.append(detachBtn)
   }
   detachBtn.onclick=async()=>{
    closeMenu()
    try{
     const res=await api.detachToNativeTerminal(activeRun.id)
     if(res?.terminal)activeRun.externalTerminal=res.terminal
     render();await refresh()
    }catch(err){view.status.textContent='Could not detach: '+err.message}
   }
  }else if(detachBtn){
   detachBtn.remove()
  }
  menu.hidden=false;more.setAttribute('aria-expanded','true')
  dismiss=event=>{if(!menu.contains(event.target)&&event.target!==more)closeMenu()}
  document.addEventListener('pointerdown',dismiss,true)
  view.closeOpenMenu=closeMenu
  menu.querySelector('[role=menuitem]:not(:disabled)')?.focus()
 }
 more.onclick=()=>{menu.hidden?openMenu():closeMenu()}
 more.onkeydown=event=>{
  if(event.key==='Escape'&&!menu.hidden){event.preventDefault();event.stopPropagation();closeMenu(true)}
  else if(event.key==='ArrowDown'&&menu.hidden){event.preventDefault();openMenu()}
 }
 const skills=view.info.server?.skills||[]
 const items=[]
 if(skills.some(item=>item.id==='pickup'))items.push({label:'Pickup (full chain)',skillId:'pickup'})
 for(const item of skills)if(item.id!=='pickup')items.push({label:item.command||item.id,skillId:item.id})
 if(taskStage(task)==='implemented'){
  items.push({label:'Declare code as reviewed…',transition:'reviewed'})
 }
 items.push({label:'Discussion (no skill)',skillId:'discuss'},{label:'Discussion in native terminal',nativeTerminal:true},{label:'Custom instructions…',compose:true})
 for(const item of items){
  const button=document.createElement('button');button.type='button';button.setAttribute('role','menuitem');button.textContent=item.label;button.disabled=!view.info.configured
  if(item.skillId)button.dataset.skillId=item.skillId
  if(item.transition==='reviewed'){
   entry.declareReviewed=button
   button.onclick=()=>{closeMenu();confirmDeclareReviewed(view.projectID,task)}
  }else{
   button.onclick=()=>{closeMenu();if(item.compose)openCompose(view,entry);else if(item.nativeTerminal)submitNativeDiscussion(view,entry).catch(()=>{});else submitTicketLaunch(view,entry,item.skillId,'','').catch(()=>{})}
  }
  menu.append(button)
 }
 menu.onkeydown=event=>{
  const options=[...menu.querySelectorAll('[role=menuitem]:not(:disabled)')],index=options.indexOf(document.activeElement)
  if(event.key==='Escape'){event.preventDefault();event.stopPropagation();closeMenu(true)}
  else if(event.key==='ArrowDown'){event.preventDefault();options[(index+1)%options.length]?.focus()}
  else if(event.key==='ArrowUp'){event.preventDefault();options[(index-1+options.length)%options.length]?.focus()}
 }
 menu.addEventListener('focusout',event=>{if(!menu.contains(event.relatedTarget)&&event.relatedTarget!==more)closeMenu()})
 run.onclick=()=>{if(run.dataset.skillId)submitTicketLaunch(view,entry,run.dataset.skillId,'','').catch(()=>{})}
 actions.append(run,more,menu)
 const stageCell=cell('ticket-stage',taskStage(task));if(task.trackerStatus)stageCell.title='Tracker status: '+task.trackerStatus
 // The key opens the task in Sectile, the same gesture the sidebar task number
 // offers, so the identity means the same thing on both surfaces.
 const keyCell=cell('ticket-key')
 const keyLink=document.createElement('button');keyLink.type='button';keyLink.className='task-number';keyLink.textContent=key
 keyLink.title='Open task in Sectile';keyLink.setAttribute('aria-label','Open '+key+' in Sectile')
 keyLink.onclick=()=>api.openTask(task.id).catch(error)
 keyCell.append(keyLink)
 row.append(stateCell,keyCell,titleCell,stageCell,priorityCell)
 if(view.engines){
  const engineCell=cell('ticket-engine')
  const toggle=document.createElement('button');toggle.type='button';toggle.className='engine-toggle'
  toggle.onclick=()=>switchTaskEngine(view,entry)
  entry.engine=toggle;engineCell.append(toggle);row.append(engineCell)
  renderEngineToggle(view,entry)
 }
 row.append(prCell,actions)
 view.rows.set(task.id,entry)
 updateTicketRow(view,entry)
 return row
}
// The engine icon of a row: the monogram of its task engine, a tooltip naming
// it, and a highlight when the task left its project default engine (#510).
function renderEngineToggle(view,entry){
 const {task,engine:toggle}=entry
 if(!toggle)return
 const key=task.key||task.id
 const current=taskEngine(view.engines,task.id)
 const onDefault=!current||current.id===view.engines.projectDefault
 const tooltip=engineTooltip(current,onDefault)
 toggle.textContent=engineMark(current?.provider)
 toggle.title=tooltip;toggle.setAttribute('aria-label','Engine for '+key+': '+tooltip)
 toggle.dataset.engineId=current?.id||''
 if(onDefault)delete toggle.dataset.offDefault;else toggle.dataset.offDefault='true'
}
// A click moves the task to the next engine of the catalogue. The icon moves at
// once; a refusal puts back what the agent stores and says why. A run already
// going keeps its engine: the choice applies from the next launch.
async function switchTaskEngine(view,entry){
 const engines=view.engines
 if(!engines||engines.catalogue.length<2)return
 const taskId=entry.task.id
 const previous=engines.tasks?.[taskId]
 const next=nextEngine(engines.catalogue,taskEngine(engines,taskId)?.id,engines.projectDefault)
 if(!next)return
 engines.tasks={...engines.tasks,[taskId]:next.id}
 if(next.id===engines.projectDefault)delete engines.tasks[taskId]
 renderEngineToggle(view,entry)
 try{
  const stored=await api.setTaskEngine(view.projectID,taskId,next.id)
  if(ticketsView===view&&stored)view.engines.tasks=stored.tasks||{}
 }catch(err){
  if(previous)engines.tasks[taskId]=previous;else delete engines.tasks[taskId]
  view.status.textContent='Could not switch the engine of '+(entry.task.key||taskId)+': '+ipcMessage(err)
  try{const fresh=await api.taskEngines(view.projectID);if(ticketsView===view)view.engines=fresh}catch{}
 }
 if(ticketsView===view)for(const row of view.rows.values())renderEngineToggle(view,row)
}
// Only the parts that depend on the polled runs are refreshed here: the row
// itself stays, so a poll cannot move focus or close an open menu.
function updateTicketRow(view,entry){
 const {task,run,state}=entry,key=task.key||task.id
 const executions=runs.filter(item=>item.taskId===task.id&&item.projectId===view.projectID&&!freeConsole(item)&&!hiddenRun(item))
 const representative=executions.find(activeRun)||[...executions].sort((a,b)=>(b.createdAt||'').localeCompare(a.createdAt||''))[0]
 if(representative)renderRunState(state,representative)
 else{state.innerHTML='';state.title='';state.removeAttribute('aria-label');delete state.dataset.runState}
 const step=nextTaskStep(task,view.info)
 const active=executions.some(activeRun),pending=view.submitting.has(task.id)
 if(entry.declareReviewed)entry.declareReviewed.disabled=!view.info.configured||active||pending
 if(step.skillId){run.textContent='Run: '+step.label;run.dataset.skillId=step.skillId}
 else{run.textContent='Run';delete run.dataset.skillId}
 const hadFocus=document.activeElement===run
 run.disabled=!step.skillId||active||pending
 // A disabled control loses focus to the body, which sends a keyboard user back
 // to the top of the page. The row's own menu is always available, so focus
 // stays where the user was working.
 if(run.disabled&&hadFocus)entry.more.focus()
 run.title=!step.skillId?step.message:active?'An execution is active on this task':pending?'Submitting execution…':'Launch '+step.label+' on '+key
 // Chromium delivers no pointer events to a disabled control, so its own title
 // would never appear: the cell carries the explanation while it is unusable.
 if(run.disabled)entry.actions.title=run.title
 else entry.actions.removeAttribute('title')
}
function renderTicketRows(){
 const view=ticketsView
 if(!view||!ticketsOpen||!view.info)return
 for(const entry of view.rows.values())updateTicketRow(view,entry)
}
function closeCompose(view){
 if(!view.compose)return
 view.compose.row.remove();view.compose=null
}
function openCompose(view,entry,initial=null,focusPrompt=true){
 closeCompose(view)
 const key=entry.task.key||entry.task.id
 const row=document.createElement('tr');row.className='ticket-compose'
 const cell=document.createElement('td');cell.colSpan=ticketColumns(view).length
 const prompt=document.createElement('textarea');prompt.placeholder='What should the agent do?';prompt.setAttribute('aria-label','Custom instructions')
 const mode=modeSelect(document,'Execution mode for '+key)
 const launch=document.createElement('button');launch.type='button';launch.textContent='Launch';launch.disabled=!view.info.configured
 const cancel=document.createElement('button');cancel.type='button';cancel.textContent='Cancel'
 const notice=document.createElement('p');notice.setAttribute('role','status')
 const controls=document.createElement('div');controls.className='ticket-compose-controls';controls.append(mode,launch,cancel)
 cell.append(prompt,controls,notice);row.append(cell)
 prompt.value=initial?.text||''
 mode.value=initial?.mode||''
 entry.row.after(row);view.compose={row,taskId:entry.task.id,text:prompt.value,mode:mode.value}
 // What the user types is held on the view, so a sort or a search rebuilds the
 // rows around it instead of throwing it away.
 prompt.oninput=()=>{if(view.compose?.row===row)view.compose.text=prompt.value}
 mode.onchange=()=>{if(view.compose?.row===row)view.compose.mode=mode.value}
 cancel.onclick=()=>{closeCompose(view);entry.more.focus()}
 row.onkeydown=event=>{if(event.key==='Escape'){event.preventDefault();event.stopPropagation();closeCompose(view);entry.more.focus()}}
 launch.onclick=async()=>{
  if(!prompt.value.trim()){notice.textContent='Enter custom instructions.';prompt.focus();return}
  launch.disabled=true;notice.textContent='Submitting execution…'
  try{await submitTicketLaunch(view,entry,'custom',prompt.value,launchModeOverride(mode.value));closeCompose(view);entry.more.focus()}
  catch(err){notice.textContent=err.message;launch.disabled=false}
 }
 if(focusPrompt)prompt.focus()
}
async function submitTicketLaunch(view,entry,skillId,prompt,mode){
 const key=entry.task.key||entry.task.id
 view.submitting.add(entry.task.id);updateTicketRow(view,entry)
 view.status.textContent='Submitting execution for '+key+'…'
 try{
  await api.launchServerTask(view.projectID,entry.task.id,skillId,prompt,mode)
  view.status.textContent='Execution submitted for '+key
  await refresh()
 }catch(err){view.status.textContent='Could not launch '+key+': '+err.message;throw err}
 finally{view.submitting.delete(entry.task.id);if(view.rows.get(entry.task.id)===entry)updateTicketRow(view,entry)}
}
async function submitNativeDiscussion(view,entry){
 const key=entry.task.key||entry.task.id
 view.submitting.add(entry.task.id);updateTicketRow(view,entry)
 view.status.textContent='Launching native terminal for '+key+'…'
 try{
  await api.launchNativeDiscussion(view.projectID,entry.task.id)
  view.status.textContent='Native terminal launched for '+key
  await refresh()
 }catch(err){view.status.textContent='Could not launch native terminal for '+key+': '+err.message;throw err}
 finally{view.submitting.delete(entry.task.id);if(view.rows.get(entry.task.id)===entry)updateTicketRow(view,entry)}
}

document.querySelector('#rerun').onclick=async()=>{
 const run=runs.find(item=>item.id===selected)
 if(!run||!['completed','failed','canceled'].includes(run.status))return
 if(freeConsole(run)){openAgentConsole(run.projectId,run.engineId||run.provider);return}
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
     if(task?.prUrl&&/^https?:\/\//i.test(task.prUrl))pullRequests.set(run.taskId,currentPullRequest(task))
     else pullRequests.delete(run.taskId)
    }
   }catch{}
  }))
  render()
 }finally{linksLoading=false}
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
  selected=null;hideRepositoryChoice();terminal.reset();await api.detach()
  renderHeader();showDirectory('')
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

// The palette is a list of actions rather than a hardcoded one, so adding a
// command is a row and not another hidden-state to maintain by hand.
const COMMANDS=[
 {label:'Quick add task',run:()=>quickAdd()},
 {label:'Tasks list',run:()=>openTicketsFromPalette()}
]
// The tickets list needs a project. The selected one answers that, and a single
// configured project answers it too; otherwise the palette asks rather than
// guessing which project the user meant.
async function openTicketsFromPalette(){
 if(!projects.length){
  try{await loadProjects()}catch(err){showDialog('Tasks list');paragraph(err.message);return}
 }
 const known=projects.filter(project=>!hiddenProject(project.id))
 const chosen=known.find(project=>project.id===selectedProject)||(known.length===1?known[0]:null)
 if(chosen){openTickets(chosen.id);return}
 showDialog('Tasks list')
 if(!known.length){paragraph('Add a project before browsing its tasks.');return}
 paragraph('Choose the project whose tasks you want to browse.')
 for(const project of known){
  const button=document.createElement('button');button.className='discovered-project';button.textContent=project.name
  button.onclick=()=>openTickets(project.id)
  dialogBody.append(button)
 }
 dialogBody.querySelector('.discovered-project')?.focus()
}
function openCommandPalette(){
 showDialog('Commands')
 const filter=document.createElement('input');filter.placeholder='Search actions…';filter.setAttribute('aria-label','Search commands')
 const buttons=COMMANDS.map(command=>{
  const button=document.createElement('button');button.className='discovered-project';button.textContent=command.label
  button.onclick=()=>command.run()
  return button
 })
 filter.oninput=()=>{
  const text=filter.value.trim().toLowerCase()
  for(const [index,button] of buttons.entries())button.hidden=!COMMANDS[index].label.toLowerCase().includes(text)
 }
 filter.onkeydown=event=>{
  if(event.key!=='Enter')return
  const first=buttons.find(button=>!button.hidden)
  if(first){event.preventDefault();first.click()}
 }
 dialogBody.append(filter,...buttons);filter.focus()
}
document.querySelector('#command-palette').onclick=openCommandPalette
window.addEventListener('keydown',event=>{
 if((event.metaKey||event.ctrlKey)&&event.key.toLowerCase()==='k'){event.preventDefault();event.stopPropagation();openCommandPalette()}
},true)
// Cmd+B / Ctrl+B toggles the projects sidebar (#474). In the capture phase, so
// that on macOS Cmd+B never reaches the terminal; an ignored key is left
// untouched, which is how Ctrl+B still reaches a focused terminal elsewhere.
window.addEventListener('keydown',event=>{
 const action=sidebarShortcutAction({key:event.key,metaKey:event.metaKey,ctrlKey:event.ctrlKey,shiftKey:event.shiftKey,altKey:event.altKey,repeat:event.repeat,defaultPrevented:event.defaultPrevented,mac:isMacPlatform(navigator),inTerminal:!!document.activeElement?.closest?.('.xterm'),modalOpen:dialog.open})
 if(action==='toggle'){event.preventDefault();event.stopPropagation();toggleSidebar()}
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
   launch.onclick=()=>openTickets(task.projectId,task.key||task.title)
   form.append(notice,launch)
  }catch(err){notice.textContent=err.message;submit.disabled=false}
 }
}

function isFinishedTask(task){
 return ['finished','done'].includes(task.status)||(task.labels||[]).some(label=>label.trim().replace(/^#/,'').toLowerCase()==='finished')
}

function pickupAvailable(project){
 return Boolean(project?.configured&&project.server?.skills?.some(skill=>skill.id==='pickup'))
}

function currentTaskRun(){return runs.find(run=>run.id===selected)}
function renderNextStep(){
 const run=currentTaskRun(),status=document.querySelector('#next-step-status'),button=document.querySelector('#next-step'),chain=document.querySelector('#pickup-chain'),label=document.querySelector('#next-step-label'),markReviewed=document.querySelector('#mark-reviewed'),retry=document.querySelector('#retry-next-step'),force=document.querySelector('#force-next-step')
 button.hidden=true;button.disabled=true;chain.hidden=true;chain.disabled=true;label.hidden=true;retry.hidden=true
 function setStepLabel(text){
  button.setAttribute('aria-label',text);button.title=text;label.textContent=text;label.hidden=button.hidden
 }
 if(force){force.hidden=true;force.disabled=true}
 if(markReviewed){markReviewed.hidden=true;markReviewed.disabled=true}
 if(!run){status.textContent='Select a task to see its next step';return}
 if(freeConsole(run)){status.textContent='Project prompt · '+(run.cancelRequested?'Stopping':run.status);return}
 const key=taskKey(run)
 if(!nextStepData||nextStepData.key!==key){status.textContent='Loading task workflow…';return}
 if(nextStepData.error){status.textContent=nextStepData.error;retry.hidden=false;return}
 const step=nextStepData.step
 const busy=runs.some(item=>taskKey(item)===key&&activeRun(item))
 const submitted=submittedSteps.get(key)
 if(submitted&&((submitted.kind!=='pickup'&&submitted.skillId!==step.skillId)||runs.some(item=>taskKey(item)===key&&!submitted.runIds.includes(item.id))))submittedSteps.delete(key)
 const pending=submittingSteps.has(key)||submittedSteps.has(key)
 const message=submittingSteps.has(key)?'Submitting execution…':pending?'Execution submitted; waiting for its console':busy?'Execution in progress':nextStepErrors.get(key)||step.message
 status.textContent=(nextStepData.task.key||run.taskKey||run.taskId)+' · '+step.stage+' · '+message
 if(pickupAvailable(nextStepData.project)&&taskStage(nextStepData.task)!=='finished'){chain.hidden=false;chain.disabled=busy||pending}
 // The most recent active execution names the button, then the launch in
 // flight; the stage step only reads once nothing runs on the task. An active
 // run without a skill still counts, so it never leaves `Next:` enabled.
 const active=runs.filter(item=>taskKey(item)===key&&activeRun(item)).sort((a,b)=>(b.createdAt||'').localeCompare(a.createdAt||''))[0]
 const current=active?.skill||submittingSteps.get(key)||submittedSteps.get(key)?.skillId||''
 if(busy||pending){button.hidden=false;button.disabled=true;setStepLabel(('Current: '+(skillLabel(current)||step.label||'')).trim())}
 else if(step.skillId){button.hidden=false;button.disabled=false;setStepLabel('Next: '+step.label)}
 const forcedKind=forceableLaunches.get(key)
 if(force&&forceableLaunches.has(key)&&(forcedKind==='pickup'?!chain.hidden:Boolean(step.skillId))){force.hidden=false;force.disabled=busy||pending}
 if(markReviewed&&nextStepData?.task&&taskStage(nextStepData.task)==='implemented'){
  markReviewed.hidden=false
  markReviewed.disabled=busy||pending
 }
}
new ResizeObserver(resize).observe(document.querySelector('#task-status'))
new ResizeObserver(resize).observe(document.querySelector('#toolbar'))
async function readNextStep(run){
 const [tasks,project]=await Promise.all([api.serverTasks(run.projectId,run.taskKey||run.taskId),api.project(run.projectId)])
 const task=tasks.find(task=>task.id===run.taskId)
 if(!task)throw Error('Task workflow unavailable. Refresh to try again.')
 return {key:taskKey(run),task,project,step:nextTaskStep(task,project)}
}
async function refreshNextStep(){
 const run=currentTaskRun()
 if(run&&submittingSteps.has(taskKey(run)))return
 const generation=++nextStepGeneration
 nextStepUpdated=Date.now()
 if(!run||freeConsole(run)||macroRun(run)){nextStepData=null;renderNextStep();return}
 if(nextStepData?.key!==taskKey(run))nextStepData=null
 renderNextStep()
 try{
  const data=await readNextStep(run)
  if(generation===nextStepGeneration)nextStepData=data
 }catch(err){if(generation===nextStepGeneration)nextStepData={key:taskKey(run),error:'Cannot load next step: '+err.message}}
 if(generation===nextStepGeneration)renderNextStep()
}
document.querySelector('#retry-next-step').onclick=refreshNextStep
// force re-sends the very same launch with the duplicate check waived. It is
// the only difference between the two buttons: everything else, from the
// freshness recheck to the local busy guard, applies identically.
async function launchTaskWork(kind,force){
 const run=currentTaskRun(),displayed=nextStepData
 if(!run||displayed?.key!==taskKey(run))return
 if(kind==='next'&&!displayed.step?.skillId)return
 if(kind==='pickup'&&(!pickupAvailable(displayed.project)||taskStage(displayed.task)==='finished'))return
 const key=taskKey(run)
 if(submittingSteps.has(key)||submittedSteps.has(key)||runs.some(item=>taskKey(item)===key&&activeRun(item)))return
 const skill=kind==='pickup'?'pickup':displayed.step.skillId
 nextStepGeneration++;nextStepErrors.delete(key);forceableLaunches.delete(key);submittingSteps.set(key,skill);renderNextStep()
 try{
  const [fresh,latestRuns]=await Promise.all([readNextStep(run),api.runs()])
  if(taskKey(currentTaskRun()||{})!==key)return
  nextStepData=fresh
  const abandoned=latestRuns.some(item=>taskKey(item)===key&&activeRun(item))||(kind==='next'?fresh.step.skillId!==displayed.step.skillId:(!pickupAvailable(fresh.project)||taskStage(fresh.task)==='finished'))
  if(abandoned){await refresh();return}
  const launchSkill=kind==='pickup'?'pickup':fresh.step.skillId
  submittingSteps.set(key,launchSkill)
  await api.launchServerTask(run.projectId,run.taskId,launchSkill,'',kind==='pickup'?'autonomous':undefined,force)
  submittedSteps.set(key,{skillId:launchSkill,kind,runIds:latestRuns.filter(item=>taskKey(item)===key).map(item=>item.id)})
  await refresh()
  if(taskKey(currentTaskRun()||{})===key){
   const launched=runs.find(item=>taskKey(item)===key&&!latestRuns.some(previous=>previous.id===item.id))
   if(launched)select(launched)
  }
 }catch(err){
  const refusal=refusedActiveRun(err.message)
  if(refusal){nextStepErrors.set(key,refusal.error||'A run is already active on this task.');forceableLaunches.set(key,kind)}
  else nextStepErrors.set(key,(kind==='pickup'?'Could not launch full chain: ':'Could not launch next step: ')+err.message)
 }finally{submittingSteps.delete(key);renderNextStep()}
}
document.querySelector('#next-step').onclick=()=>launchTaskWork('next',false)
document.querySelector('#pickup-chain').onclick=()=>launchTaskWork('pickup',false)
document.querySelector('#force-next-step').onclick=()=>{const run=currentTaskRun();launchTaskWork(run?forceableLaunches.get(taskKey(run))||'next':'next',true)}
document.querySelector('#mark-reviewed').onclick=()=>{
 const run=currentTaskRun()
 if(run&&nextStepData?.task&&taskStage(nextStepData.task)==='implemented'){
  confirmDeclareReviewed(run.projectId,nextStepData.task)
 }
}

async function openAgentConsole(projectID,previousProvider){
 showDialog('Project prompt')
 paragraph('Start an interactive agent in this project’s local repository. Enter your instructions directly in its console.')
 const form=document.createElement('form'),label=document.createElement('label'),provider=document.createElement('select')
 label.textContent='AI engine';provider.setAttribute('aria-label','Console agent')
 for(const [value,text] of [['codex','Codex'],['claude','Claude']]){const option=document.createElement('option');option.value=value;option.textContent=text;provider.append(option)}
 label.append(provider)
 const location=document.createElement('p');location.textContent=projects.find(project=>project.id===projectID)?.path||''
 const launch=document.createElement('button');launch.textContent='Open console'
 const notice=document.createElement('p');notice.setAttribute('role','status')
 form.append(label,location,launch,notice);dialogBody.append(form)
 let catalogue=false
 launch.disabled=true
 try{
  if(await taskEnginesAvailable()){
   const view=await api.taskEngines(projectID)
   if(!form.isConnected)return
   provider.replaceChildren()
   for(const engine of view.catalogue){
    const option=document.createElement('option');option.value=engine.id
    option.textContent=engineTooltip(engine,engine.id===view.projectDefault);provider.append(option)
   }
   catalogue=true
   provider.value=view.catalogue.some(engine=>engine.id===previousProvider)?previousProvider:view.projectDefault
   if(!provider.value&&provider.options.length)provider.selectedIndex=0
  }else{
   const info=await api.project(projectID)
   const initial=previousProvider||info.aiProvider||info.server?.aiProvider
   if(['codex','claude'].includes(initial))provider.value=initial
  }
  launch.disabled=!provider.value
 }catch(err){notice.textContent=err.message}
 form.onsubmit=async event=>{
  event.preventDefault();if(launch.disabled)return
  launch.disabled=true;notice.textContent='Opening console…'
  try{
   const run=await api.launchConsole(projectID,catalogue?undefined:provider.value,catalogue?provider.value:undefined)
   collapsedProjects.delete(projectID);queueProjects.delete(projectID)
   if(!runs.some(item=>item.id===run.id))runs.push(run)
   dialog.close();select(run);await refresh()
  }catch(err){notice.textContent=err.message;launch.disabled=false}
 }
}
