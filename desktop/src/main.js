import { skillCommandMapping } from './skill-command-mapping.mjs'
import { installSettingsSearch } from './settings-search.mjs'
import { pullRequestPresentation, renderPullRequestIndicator, repositoryPullRequests } from './pullRequests.mjs'
import { installTooltips } from './tooltips.js'
import {mcpSettings} from './mcp-settings.mjs'
import { mcpProviders } from '../../shared/mcpConfig.mjs'
import { skillEnded, skillResultDue, skillResultStamp } from './skill-result-refresh.mjs'
import { newTaskShortcutAction, newTaskShortcutLabel } from './new-task-shortcut.mjs'
import { paletteMatches } from './command-palette.mjs'
import { defaultActionShortcut, defaultActionTarget, defaultActionLabel } from './dialog-default.mjs'
import { logText } from './log-text.mjs'
import { createGitDiff } from './gitDiff.js'
import { createConversationView } from './conversation.js'

import { skillResult } from './skill-result.mjs'
import { orderedQueueRuns } from './queue.mjs'
import { followedExecution, orderedTaskGroups } from './task-order.mjs'
import { transitions, announce } from './notifications.mjs'
import { runStateOf, runStateLabel, runStateSvg } from '../../shared/runStates.ts'
import { isMacPlatform, sidebarShortcutAction, sidebarShortcutAria, sidebarShortcutLabel } from '../../shared/sidebarShortcut.mjs'
import { configShortcutAction, configShortcutAria, configShortcutLabel } from './config-shortcut.mjs'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import './style.css'
import { STAGES, taskStage, nextTaskStep, skillLabel } from './workflow.mjs'
import { launchModeOverride, modeSelect } from './skill-mode.mjs'
import { orderedTasks, nextSort, DEFAULT_SORT, SORTABLE_FIELDS } from './task-list-order.mjs'
import { consoleNotice, needsConsoleNotice, readOnlyConsole } from './run-console.mjs'
import { previewLines } from './command-preview.mjs'
import { sandboxSettings, whitelistEditor } from './sandbox-settings.mjs'
import { EDITORS, editorChoice, editorLabel } from './editors.mjs'
import { chosenFolder, folderRoleLabel, menuFolders } from './folder-menu.mjs'
import { PROVIDERS, DEFAULT_PROVIDER, projectFields, ownEntries, compact, parseModelList, sourceHint, describe, ipcMessage, agentUnreachable, validSkillCommand, workstationPayload } from './execution-fields.mjs'
import { runEngine } from './run-engine.mjs'
import { nextEngine, taskEngine, launchEngineChange, engineMark, engineTooltip, moveEngine, removalImpact, removalMessage } from './engines.mjs'
import { pollAction, startEnabled } from './agent-poll.mjs'
import { runFolderOutcome, offersRunFolder } from './run-folders.mjs'
import { offerFor, initializedNotice } from './git-init.mjs'
import { archiveLabel, archiveRefusal, archiveFailure } from './archive-workspace.mjs'
// The repository changelog, inlined by Vite at build time. The app reads it
// with no network at all: the renderer's content security policy forbids any
// outgoing connection, and the release notes have to stay readable with the
// agent stopped.
import changelogSource from '../../CHANGELOG.md?raw'
import { parseChangelog, releaseNotesFor } from './changelog.mjs'
import { APPEARANCE_CHOICES, CONSOLE_VIEW_CHOICES, CONVERSATION_MODE_CHOICES, terminalOptions } from './appearance.mjs'
import { claudeMark, settingsCategoryIcon } from './claude-mark.mjs'
import { OPENAI_MARK_PATH } from './openai-mark.mjs'
const api=window.localAgent
// Concurrent execution workers ceiling per project, aligned with agentconfig.MaxParallelism.
// Parallelism is a workstation setting: the server neither stores nor supplies it.
const MAX_PARALLELISM=10
// Parallel executions of a project when no setting states one, aligned with
// agentconfig.DefaultParallelism.
const DEFAULT_PARALLELISM=5
document.querySelector('#app').innerHTML=`
<header><div><button id="toggle-sidebar" aria-expanded="true"></button><strong id="app-title">Sectile Desktop</strong><small>Execution consoles</small></div><button id="command-palette" title="Commands (⌘K / Ctrl+K)">⌘K</button></header>
<section id="setup" hidden><div class="setup-toolbar"><button id="setup-logs" type="button" title="View local-agent diagnostics">Agent logs</button></div><div id="agent-offline" role="status" hidden><strong>Local agent is stopped</strong><p>Start the agent to run tasks and access your local consoles.</p></div><h1>Connect to Sectile</h1><p id="setup-intro">In the Sectile web interface, under your profile, choose <strong>Pair a workstation</strong> and paste the code here. A code is single use and expires within ten minutes; this machine keeps the credential it receives, so the code is never needed again.</p>
<form id="start"><label>Sectile server<input name="server" type="url" value="http://localhost:8090" required></label>
<div id="pair-again"><button id="browser-sign-in" type="button">Sign in with your browser</button><label>Pairing code<input name="code" type="text" autocomplete="off" spellcheck="false" placeholder="Paste the code from the web interface"></label></div>
<p class="start-reason" role="alert" hidden></p><button type="submit">Connect</button></form></section>
<main id="workspace" hidden><aside><div class="sidebar-scroll"><div class="section">PROJECTS <button id="add-project" title="Add a remote project">+</button></div><div id="runs"></div><button id="clear-history" class="icon-button" type="button" aria-label="Clear finished consoles" title="Clear finished consoles" disabled></button></div><footer class="sidebar-footer"><span id="connection" data-state="off">Connecting…</span><nav aria-label="Local agent controls"><button id="shutdown" class="icon-button" aria-label="Stop agent" title="Stop agent" hidden></button><button id="restart" class="icon-button" aria-label="Restart agent" title="Restart agent" hidden></button></nav><button id="settings" class="icon-button" type="button" aria-label="Settings" title="Settings"></button></footer></aside><div id="sidebar-resizer" role="separator" aria-label="Resize sidebar" aria-orientation="vertical" tabindex="0"></div><article><div id="toolbar"><div class="toolbar-row toolbar-primary"><div class="terminal-title-line"><strong id="title">Select an execution</strong><span id="native-terminal-badge" class="native-terminal-badge" hidden></span></div><div class="toolbar-meta"><select id="execution-history" aria-label="Execution history" hidden></select><span id="next-step-label" class="step-badge" aria-hidden="true" hidden></span></div></div><div class="toolbar-row toolbar-secondary"><div class="worktree-line"><button id="worktree" class="worktree" type="button" title="Copy this path" hidden><svg class="worktree-icon" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 20a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2z"/></svg><span id="directory"></span></button><button id="worktree-folders" class="icon-button worktree-folders" type="button" aria-haspopup="menu" aria-expanded="false" aria-controls="worktree-folders-menu" aria-label="Folders of this execution" title="Folders of this execution" hidden><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m6 9 6 6 6-6"/></svg></button><button id="open-editor" class="icon-button" type="button" hidden></button><span id="worktree-copied" class="worktree-copied" role="status"></span><div id="worktree-folders-menu" class="folder-menu" role="menu" aria-label="Folders of this execution" hidden></div></div><div class="toolbar-actions"><div class="execution-views" role="group" aria-label="Execution view"><button id="view-console" class="icon-button" type="button" aria-label="Console" title="Console" aria-pressed="true" disabled></button><button id="view-changes" class="icon-button" type="button" aria-label="Changes" title="Changes" aria-pressed="false" disabled></button></div><button id="selected-pr" class="icon-button" type="button" hidden></button><span id="selected-pr-others" hidden></span><button id="detach-terminal" class="icon-button" type="button" aria-label="Detach to native terminal" title="Detach to native terminal" hidden></button><button id="rerun" class="icon-button" type="button" aria-label="Launch" title="Launch" hidden></button><button id="save-log" class="icon-button" type="button" aria-label="Export log" title="Export log"></button><button id="stop" class="icon-button" type="button" aria-label="Stop execution" title="Stop execution" disabled></button><button id="next-step" class="icon-button" type="button" hidden disabled></button><button id="pickup-chain" class="icon-button" type="button" aria-label="Pickup (full chain)" title="Pickup (full chain)" hidden disabled></button><button id="mark-reviewed" type="button" class="secondary" title="The pull request needs no more changes: skip Adjust, move the task to reviewed, and hand off once it is merged" hidden>Skip to Handoff</button><button id="retry-next-step" type="button" title="Retry reading the task workflow" hidden>Retry</button><button id="force-next-step" type="button" class="secondary" title="Launch although a run is already active on this task" hidden>Launch anyway</button></div></div></div><div id="execution-content"><div id="terminal"></div><div id="execution-divider" role="separator" aria-label="Resize execution views" aria-orientation="vertical" aria-valuemin="25" aria-valuemax="75" aria-valuenow="50" tabindex="0" hidden></div><section id="changes" aria-label="Worktree changes" hidden></section></div><footer id="task-status"><div class="execution-status"><span id="run-state" class="run-state selected-run-state" hidden></span><span id="skill-result" role="status" hidden></span></div><span id="next-step-status" role="status" aria-live="polite">Select a task to see its next step</span></footer></article><section id="tickets-pane" aria-label="Tickets" hidden></section></main>
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
const terminal=new Terminal({cursorBlink:true,fontSize:13,fontFamily:TERMINAL_FONT,scrollback:20000,...terminalOptions(darkScheme.matches)})
darkScheme.addEventListener('change',event=>{terminal.options=terminalOptions(event.matches)})
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
// The workflow stage of each listed task, read with its title; absent when unknown.
const taskStages=new Map()
const skillResults=new Map(),loadingSkillResults=new Set()
// When each run's skill result was last read, and in what state the run was.
const skillResultReads=new Map()
const pullRequests=new Map()
let localTasks={}
try{localTasks=JSON.parse(localStorage.getItem('localTasks')||'{}')}catch{}
const freeConsole=run=>run?.kind==='console'
// Task runs show the skill and engine; project prompts show their engine name.
const runLabel=run=>run.conversation&&run.taskId?run.skill+' · Conversation (test)':run.conversation?(run.provider==='codex'?'Codex':'Claude Code')+' · Conversation (test)':freeConsole(run)?(run.engineName||run.provider||'AI')+' · Project prompt':(runEngine(run)?run.skill+' · '+runEngine(run):run.skill)
// A macro skill run has no task: its executions group under the macro.
const macroRun=run=>!!run?.macroKey
// Archiving a ticket task removes its worktrees first (#755); a free console or a macro run has none.
const archivesWorktree=run=>!freeConsole(run)&&!macroRun(run)&&!!run?.taskId
// The tasks whose archive is under way, so their button cannot start a second one.
const archiving=new Set()
const taskKey=run=>JSON.stringify([run.projectId,freeConsole(run)?run.id:macroRun(run)?'macro:'+run.macroKey:run.taskId])
const activeRun=run=>['running','queued','preparing'].includes(run.status)
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
// Projects whose tasks are listed by workflow stage, chosen from their menu.
const stageGroupedProjects=new Set()
try{for(const id of JSON.parse(localStorage.getItem('stageGroupedProjects')||'[]'))stageGroupedProjects.add(id)}catch{}
function saveStageGrouping(){localStorage.setItem('stageGroupedProjects',JSON.stringify([...stageGroupedProjects]))}
// Projects the user hid from the sidebar. Hiding keeps the local configuration
// and the executions: the project stays in Settings, where it can be shown again.
const sidebarHiddenProjects=new Set()
try{for(const id of JSON.parse(localStorage.getItem('sidebarHiddenProjects')||'[]'))sidebarHiddenProjects.add(id)}catch{}
function setSidebarHidden(id,hidden){
 if(hidden)sidebarHiddenProjects.add(id);else sidebarHiddenProjects.delete(id)
 try{localStorage.setItem('sidebarHiddenProjects',JSON.stringify([...sidebarHiddenProjects]))}catch{}
 if(hidden&&runs.find(run=>run.id===selected)?.projectId===id){selected=null;conversation.select(null);api.detach().catch(()=>{});terminal.reset()}
 render()
}
// A project added to this workstation: configured, mapped, or with executions.
const addedProject=project=>!hiddenProject(project.id)&&(project.configured||!!project.path||runs.some(run=>run.projectId===project.id))

const queueProjects=new Set()

let linksLoading=false,lastLinksRefresh=0
let selectedProject=null
let ticketsOpen=false,agentConnected=false
let updateSettingsConnection=null
let opened=false,selected=null,runs=[],last='',stopping=false,restarting=false,projects=[],projectsLoaded=false
// Whether the local agent attaches a folder from a run (#676), and serves
// another folder of a run than its directory (#784), read with the editor
// setting from its status.
let folderSelectionAvailable=false,runFoldersAvailable=false,runFoldersTerminalsAvailable=false,conversationControlsAvailable=false,conversationQueueAvailable=false,providerModels={}
const conversation=createConversationView({api,container:document.querySelector('#terminal'),onError:error,canAddFolder:()=>runFoldersAvailable,canControl:()=>conversationControlsAvailable,canQueue:()=>conversationQueueAvailable,models:provider=>providerModels[provider]||[]})
// The conversation view is opt-in from Appearance; the terminal stays the default.
let consoleView='terminal'
api.consoleView().then(value=>{consoleView=value;render()}).catch(()=>{})
// "Add folder…" on a running ticket discussion or free console, in Sectile or
// detached to the native terminal (#676, #689): the folder joins the project,
// and the agent types /add-dir into a Claude Code session.
const addFolderButton=document.createElement('button')
addFolderButton.type='button';addFolderButton.id='add-run-folder';addFolderButton.textContent='Add folder…';addFolderButton.hidden=true
addFolderButton.title='Attach a folder of this workstation to the project and give it to this session'
const addFolderStatus=document.createElement('span')
addFolderStatus.id='add-run-folder-status';addFolderStatus.className='run-folder-status';addFolderStatus.setAttribute('role','status')
document.querySelector('#save-log').before(addFolderButton,addFolderStatus)
let addFolderRun=null
addFolderButton.onclick=async()=>{
 const id=selected,run=runs.find(item=>item.id===id)
 const where={console:freeConsole(run),terminal:run?.externalTerminal?formatTerminalName(run.externalTerminal):''}
 addFolderStatus.textContent='';addFolderRun=id
 try{
  const path=await api.chooseRepository()
  if(!path)return
  addFolderButton.disabled=true
  const answer=await api.addRunFolder(id,path)
  if(addFolderRun===id)addFolderStatus.textContent=runFolderOutcome(path,answer,where)
 }catch(err){if(addFolderRun===id)addFolderStatus.textContent=ipcMessage(err)}
 finally{addFolderButton.disabled=false}
}
const changes=createGitDiff({api,container:document.querySelector('#changes'),terminal:document.querySelector('#terminal'),panel:document.querySelector('#execution-content'),divider:document.querySelector('#execution-divider'),consoleButton:document.querySelector('#view-console'),changesButton:document.querySelector('#view-changes'),onConsole:focus=>{resize();if(focus&&opened&&!conversation.active)terminal.focus()}})
api.onOutput(data=>terminal.write(new Uint8Array(data)))
terminal.onData(data=>{if(changes.consoleVisible&&!ticketsOpen)api.input(data)})
function resize(){if(opened&&changes.consoleVisible&&!ticketsOpen&&!conversation.active){fit.fit();api.resize(terminal.cols,terminal.rows)}}
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
function error(err){document.querySelector('#error').textContent=startFailure(err)}
// Electron prefixes an error thrown across invoke with the channel it crossed; the user needs the reason only.
function startFailure(err){return (err?.message||String(err)).replace(/^Error invoking remote method '[^']*': (?:Error: )?/,'')}
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
const connectForm=document.querySelector('#start')
let startPending=false,agentActionPending=false,autoStartTried=false,credential={state:'missing',server:''},pairingReason='',startReason='',renderSettingsAgent=()=>{},claudeChecked=false
// Every start control obeys one rule: clickable whenever no agent runs and nothing is in progress (#716).
function updateStartControl(){
 const button=connectForm.querySelector('button[type=submit]'),stored=credential.state==='present'&&!pairingReason
 button.disabled=!startEnabled({agentConnected,restarting,pending:agentActionPending,starting:startPending})
 // Signing in through the browser starts the agent too, so it follows the same rule as the start button (#717).
 connectForm.querySelector('#browser-sign-in').disabled=button.disabled
 button.textContent=startPending?'Starting…':stored?'Start local agent':'Connect'
 renderStartForm(stored);renderSettingsAgent()
}
function renderStartForm(stored){
 const pair=connectForm.querySelector('#pair-again');pair.hidden=stored&&!connectForm.closest('#settings-panel-Connection')
 document.querySelector('#setup-intro').textContent=stored?'This workstation is paired. Start the local agent to run tasks and reconnect your AI engines.':'Sign in with your browser, or, in the Sectile web interface, under your profile, choose Pair a workstation and paste the code here. A code is single use and expires within ten minutes.'
 // A refusal stays said while a sign-in after it waits or fails: the two are shown together, the refusal first (#717).
 setStartReason([pairingReason,startReason].filter(Boolean).join(' '))
}
function setStartReason(text){const p=connectForm.querySelector('.start-reason');p.textContent=text;p.hidden=!text}
function agentUnavailable(){
 agentConnected=false
 changes.disconnect()
 updateStartControl()
 document.querySelector('#shutdown').hidden=true
 document.querySelector('#restart').hidden=true
 document.querySelector('#agent-offline').hidden=false
 closeTickets(false)
 document.querySelector('#setup').hidden=configurationActive()
 document.querySelector('#workspace').hidden=!configurationActive()
 connectionStatus({text:'Local agent stopped'})
 projectsLoaded=false
 openEditorAvailable=false;projectTerminalAvailable=false;renderOpenEditor()
 api.detach().catch(()=>{})
}
function ready(){
 agentConnected=true
 document.querySelector('#agent-offline').hidden=true
 if(!projectsLoaded){projectsLoaded=true;loadProjects().catch(()=>{projectsLoaded=false});loadEditorSetting();api.syncConsoleView().catch(()=>{});api.syncConversationMode().catch(()=>{})}
 updateStartControl();document.querySelector('#restart').hidden=false;document.querySelector('#shutdown').hidden=false
 document.querySelector('#setup').hidden=true;document.querySelector('#workspace').hidden=false
 if(!document.querySelector('#connection a'))connectionStatus({text:'Local agent connected'})
 if(!opened){terminal.open(document.querySelector('#terminal'));opened=true;resize()}
 // A Claude Code entry with a key this workstation does not use is reported once per launch (#716).
 if(!claudeChecked){claudeChecked=true;api.mcpConfig('claude').then(info=>{if(info?.needsRepair)error("Claude Code's sectile MCP entry uses a key this workstation does not use. Open Settings → Deployment → MCP configuration to repair it.")}).catch(()=>{claudeChecked=false})}
}
function select(run,background=false,options){
 if(hiddenProject(run.projectId))return
 if(!background)closeTickets(false)
 selectedProject=run.projectId
 selected=run.id
 changes.select(selected,currentFolder(run)?.path)
 conversation.select(run)

 refreshSkillResult()
 refreshNextStep()
 showDirectory(currentFolder(run)?.path||run.directory)
 document.querySelector('#stop').disabled=!activeRun(run)
 terminal.reset()
 if(run.conversation){
  api.detach().catch(error)
  render(options);return
 }
 if(needsConsoleNotice(run)){
  api.detach().catch(error)
  terminal.writeln(consoleNotice(run))
  render(options);return
 }
 api.attach(run.id).then(()=>{setTimeout(resize,150);if(changes.consoleVisible&&!ticketsOpen&&!readOnlyConsole(run))terminal.focus()}).catch(error)
 render(options)
}
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
 if(!run||freeConsole(run)||macroRun(run)||run.skill==='discuss'||loadingSkillResults.has(run.id)||loadingSkillResults.size>=4)return
 loadingSkillResults.add(run.id)
 try{
  const result=await api.runResult(run.id)
  // A null result means the agent no longer holds the run. That is expected once
  // the history is cleared or the agent restarted, and the next poll drops the
  // run; a run the latest poll still lists as live is the anomaly worth noting.
  if(result===null&&runs.includes(run)&&!['completed','failed','canceled'].includes(run.status))console.warn('The local agent reports no run '+run.id+' while the desktop still lists it as '+run.status)
  skillResults.set(run.id,result)
  skillResultReads.set(run.id,skillResultStamp(run,Date.now(),skillResultReads.get(run.id),skillEnded(result)))
 }catch{skillResults.delete(run.id)}
 finally{loadingSkillResults.delete(run.id);renderHeader();renderTaskSkillStatuses()}
}
let refreshingVisibleSkillResults=false
async function refreshVisibleSkillResults(){
 if(refreshingVisibleSkillResults)return
 refreshingVisibleSkillResults=true
 // Only the rows whose result can have changed are read again.
 const now=Date.now()
 const ids=[...new Set([selected,...[...document.querySelectorAll('.task-skill-status')].map(item=>item.dataset.runId)].filter(Boolean))]
  .filter(id=>{const run=runs.find(item=>item.id===id);return run&&skillResultDue(run,skillResultReads.get(id),now)})
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
 renderFolders()
}
// The editor button (#535) exists once an editor is chosen in Settings and the
// agent can open one; it opens the path shown, which the agent resolves from
// the run, never from what the renderer sends.
let configuredEditor='',openEditorAvailable=false,openingEditor=false,projectTerminalAvailable=false
function renderOpenEditor(){
 const button=document.querySelector('#open-editor')
 const label=configuredEditor&&'Open in '+editorLabel(configuredEditor)
 button.hidden=!document.querySelector('#directory').textContent||!configuredEditor||!openEditorAvailable
 button.title=label;button.setAttribute('aria-label',label)
 button.disabled=openingEditor
}
async function loadEditorSetting(){
 try{
  // What the agent offers does not depend on reading the workstation settings.
  const [status,view]=await Promise.all([api.status(),api.workstationSettings().catch(()=>null)])
  openEditorAvailable=!!status.capabilities?.includes('open-editor')
  projectTerminalAvailable=!!status.capabilities?.includes('project-terminal')
  runFoldersAvailable=!!status.capabilities?.includes('run-folders')
  folderSelectionAvailable=!!status.capabilities?.includes('folder-selection')
  runFoldersTerminalsAvailable=!!status.capabilities?.includes('run-folders-terminals')
  conversationControlsAvailable=!!status.capabilities?.includes('conversation-controls')
  conversationQueueAvailable=!!status.capabilities?.includes('conversation-queue')
  configuredEditor=String(view?.defaults?.editorCommand||'').trim()
  if(view)providerModels=view.defaults?.aiProviderModels||{}
  if(view)renderCustomSkillSignal(view.customSkillsUsed)
 }catch{openEditorAvailable=false;projectTerminalAvailable=false;runFoldersAvailable=false;folderSelectionAvailable=false;runFoldersTerminalsAvailable=false;conversationControlsAvailable=false;conversationQueueAvailable=false;configuredEditor=''}
 renderOpenEditor();render({deferrable:true})
}
// A project's custom skill ran instead of the installed one (#267): a passive
// signal on the settings button, never an interruption. The list itself is
// in Execution defaults.
function renderCustomSkillSignal(list){
 const button=document.querySelector('#settings'),used=Array.isArray(list)&&list.length>0
 button.classList.toggle('custom-skills-used',used)
 button.setAttribute('aria-label',used?'Settings, custom skills used':'Settings')
}
async function loadCustomSkillSignal(){
 try{renderCustomSkillSignal((await api.workstationSettings())?.customSkillsUsed)}catch{}
}
document.querySelector('#open-editor').onclick=async()=>{
 if(!selected||openingEditor)return
 openingEditor=true;renderOpenEditor()
 try{await api.openEditor(selected,currentFolder()?.path);document.querySelector('#error').textContent=''}
 catch(err){error(Error(ipcMessage(err).trim()))}
 finally{openingEditor=false;renderOpenEditor()}
}
function clearCopiedNotice(){
 clearTimeout(copiedNotice)
 document.querySelector('#worktree-copied').textContent=''
}
async function copyPath(path){
 if(!path)return
 try{
  await api.copyText(path)
  clearTimeout(copiedNotice)
  document.querySelector('#worktree-copied').textContent='Copied'
  copiedNotice=setTimeout(clearCopiedNotice,2000)
 }catch(err){clearCopiedNotice();error(err)}
}
document.querySelector('#worktree').onclick=()=>copyPath(document.querySelector('#directory').textContent)
// An execution may work in several folders: other repositories' worktrees,
// read-only context checkouts, attached folders, its specifications worktree
// (#762). A chevron after the path lists them. Choosing one selects it (#784):
// the path, its copy, the editor and Changes then speak of that folder. An
// agent that cannot serve another folder keeps the items copying their path.
// The menu is built when it opens, so a folder added during the run shows at
// the next opening and never moves under the pointer.
const foldersButton=document.querySelector('#worktree-folders'),foldersMenu=document.querySelector('#worktree-folders-menu')
let foldersRun=null,foldersDismiss=null
// The folder chosen per execution, forgotten when Desktop restarts.
const folderSelections=new Map()
// The folder the toolbar speaks of: the one chosen from the menu, or null for
// the run's own directory.
function currentFolder(run=runs.find(item=>item.id===selected)){
 return folderSelectionAvailable&&run?chosenFolder(run,folderSelections.get(run.id)):null
}
function selectFolder(run,folder){
 if(chosenFolder(run,folder.path))folderSelections.set(run.id,folder.path)
 else folderSelections.delete(run.id)
 if(run.id===selected)showDirectory(currentFolder(run)?.path||run.directory)
}
function closeFoldersMenu(focusOpener=false){
 if(foldersMenu.hidden)return
 foldersMenu.hidden=true;foldersButton.setAttribute('aria-expanded','false')
 if(foldersDismiss){document.removeEventListener('pointerdown',foldersDismiss,true);window.removeEventListener('blur',foldersDismiss);foldersDismiss=null}
 if(focusOpener)foldersButton.focus()
}
function renderFolders(){
 const label=document.querySelector('#directory')
 const run=label.textContent?runs.find(item=>item.id===selected):null
 const folders=menuFolders(run)
 // A menu left open belongs to the execution it was opened for.
 if(!folders.length||run.id!==foldersRun)closeFoldersMenu()
 foldersButton.hidden=!folders.length
 if(!run)return
 // A chosen folder that left the list gives the toolbar back to the run's
 // directory, Changes included.
 const folder=currentFolder(run)
 if(!folder)folderSelections.delete(run.id)
 const path=folder?.path||run.directory
 if(path&&label.textContent!==path){clearCopiedNotice();label.textContent=path;renderOpenEditor()}
 changes.select(selected,folder?.path)
}
function openFoldersMenu(){
 const run=runs.find(item=>item.id===selected),folders=menuFolders(run)
 if(!folders.length)return
 foldersRun=run.id
 const selecting=folderSelectionAvailable,current=currentFolder(run)?.path||folders[0].path
 foldersMenu.replaceChildren(...folders.map(folder=>{
  const item=document.createElement('button');item.type='button';item.setAttribute('role',selecting?'menuitemradio':'menuitem')
  const name=document.createElement('span');name.className='folder-name';name.textContent=folder.name||folder.path
  const role=document.createElement('span');role.className='folder-role';role.textContent=folderRoleLabel(folder)
  const path=document.createElement('span');path.className='folder-path';path.textContent=folder.path
  item.append(name,role,path)
  item.setAttribute('aria-label',(folder.name||folder.path)+', '+folderRoleLabel(folder)+', '+folder.path)
  if(selecting){
   item.title='Show '+folder.path;item.setAttribute('aria-checked',String(folder.path===current))
   item.onclick=()=>{closeFoldersMenu(true);selectFolder(run,folder)}
  }else{
   item.title='Copy '+folder.path
   item.onclick=()=>{closeFoldersMenu(true);copyPath(folder.path)}
  }
  return item
 }))
 foldersMenu.hidden=false;foldersButton.setAttribute('aria-expanded','true')
 // Fixed to the viewport and kept inside it, below the chevron.
 const box=foldersButton.getBoundingClientRect(),{width,height}=foldersMenu.getBoundingClientRect()
 foldersMenu.style.left=Math.max(4,Math.min(box.left,innerWidth-width-4))+'px'
 foldersMenu.style.top=Math.max(4,Math.min(box.bottom+2,innerHeight-height-4))+'px'
 foldersDismiss=event=>{if(event.type==='blur'||!foldersMenu.contains(event.target)&&!foldersButton.contains(event.target))closeFoldersMenu()}
 document.addEventListener('pointerdown',foldersDismiss,true);window.addEventListener('blur',foldersDismiss)
 const first=foldersMenu.querySelector('[aria-checked=true]')||foldersMenu.querySelector('[role^=menuitem]')
 first?.focus()
}
foldersButton.onclick=()=>{foldersMenu.hidden?openFoldersMenu():closeFoldersMenu()}
foldersButton.onkeydown=foldersMenu.onkeydown=event=>{
 if(event.key==='Escape'&&!foldersMenu.hidden){event.preventDefault();event.stopPropagation();closeFoldersMenu(true);return}
 if(event.target===foldersButton&&event.key==='ArrowDown'&&foldersMenu.hidden){event.preventDefault();openFoldersMenu();return}
 if(foldersMenu.hidden)return
 if(event.key==='Tab'){closeFoldersMenu();return}
 const items=[...foldersMenu.querySelectorAll('[role^=menuitem]')],at=items.indexOf(document.activeElement)
 const next={ArrowDown:at+1,ArrowUp:at-1,Home:0,End:items.length-1}[event.key]
 if(next===undefined)return
 event.preventDefault()
 items[(next+items.length)%items.length]?.focus()
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
function toggleStageGrouping(projectID){
 selectedProject=projectID
 if(stageGroupedProjects.has(projectID))stageGroupedProjects.delete(projectID);else stageGroupedProjects.add(projectID)
 saveStageGrouping();render()
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
  menu.querySelector('[role^=menuitem]:not(:disabled)')?.focus()
 }
 // Measured once shown: a hidden menu has no width to align on.
 const open=()=>{const box=more.getBoundingClientRect();menu.hidden=false;openAt(box.right-menu.offsetWidth,box.bottom+2)}
 const queued=queueProjects.has(project.id)
 const items=[
  {label:'Open tasks',run:()=>openTickets(project.id)},
  {label:(queued?'Show tasks':'Show execution queue')+(waitingCount?' · '+waitingCount+' waiting':''),run:()=>toggleQueue(project.id)},
  {label:'Group by stage',checked:stageGroupedProjects.has(project.id),run:()=>toggleStageGrouping(project.id)},
  {label:'New task…',run:()=>newProjectTask(project.id)},
  {label:'Project prompt',disabled:!project.path,run:()=>openAgentConsole(project.id)},
  // An older agent cannot open it, so the item is left out rather than refused.
  ...(projectTerminalAvailable?[{label:'Open terminal',disabled:!project.path,run:()=>openProjectTerminal(project.id)}]:[]),
  null,
  {label:'Hide from sidebar',run:()=>setSidebarHidden(project.id,true)},
  {label:'Project settings…',run:()=>openProject(project.id)},
  {label:'Remove from desktop',danger:true,run:()=>requestRemoveProject(project.id,project.name)}
 ]
 for(const item of items){
  if(!item){const line=document.createElement('div');line.setAttribute('role','separator');menu.append(line);continue}
  const button=document.createElement('button');button.type='button';button.setAttribute('role',item.checked===undefined?'menuitem':'menuitemcheckbox')
  if(item.checked!==undefined)button.setAttribute('aria-checked',String(item.checked))
  button.textContent=item.label;button.disabled=!!item.disabled;if(item.danger)button.className='danger'
  button.onclick=()=>{close();item.run()}
  menu.append(button)
 }
 more.onclick=()=>{menu.hidden?open():close()}
 const keys=event=>{
  if(event.key==='Escape'&&!menu.hidden){event.preventDefault();event.stopPropagation();close(true);return}
  if(event.target===more&&event.key==='ArrowDown'&&menu.hidden){event.preventDefault();open();return}
  if(menu.hidden||!['ArrowDown','ArrowUp'].includes(event.key))return
  const enabled=[...menu.querySelectorAll('[role^=menuitem]:not(:disabled)')]
  const at=enabled.indexOf(document.activeElement)
  event.preventDefault()
  enabled[(at+(event.key==='ArrowDown'?1:-1)+enabled.length)%enabled.length]?.focus()
 }
 more.onkeydown=menu.onkeydown=keys
 return {more,menu,openAt}
}
function renderHeader(){
 const run=runs.find(item=>item.id===selected)
 renderFolders()
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
 changes.select(selected,currentFolder()?.path)
 renderHeader()
 const list=document.querySelector('#runs'),editing=list.querySelector('.task-rename')
 if(renaming&&editing)renaming.selection=[editing.selectionStart,editing.selectionEnd]
 rebuildingSidebar=true
 try{list.replaceChildren()}finally{rebuildingSidebar=false}
 let renameField=null

 const listed=id=>!hiddenProject(id)&&!sidebarHiddenProjects.has(id)
 const groups=new Map(projects.filter(project=>project.path&&listed(project.id)).map(project=>[project.id,project]))
 for(const run of runs)if(listed(run.projectId)&&!groups.has(run.projectId))groups.set(run.projectId,{id:run.projectId,name:run.projectId})
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
  const waitingCount=runs.filter(run=>run.projectId===project.id&&run.status==='queued'&&!run.cancelRequested).length
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
   const stageOf=stageGroupedProjects.has(project.id)?run=>taskStages.get(run.taskId):undefined
   for(const {executions,run,skillRun} of orderedTaskGroups(taskGroups.values(),{stageOf})){
    const isSelected=executions.some(item=>item.id===selected)
    const row=document.createElement('div');row.className='local-task '+(isSelected?'selected':'')
    const button=document.createElement('button');button.className='run '+(isSelected?'selected':'')
    const key=taskKey(run),title=document.createElement('strong')
    const context=document.createElement('button');context.textContent=run.taskKey||run.taskId;context.className='task-number';context.title='Open task in Sectile';context.setAttribute('aria-label','Open '+(run.taskKey||run.taskId)+' in Sectile');context.disabled=macroRun(run);context.onclick=()=>api.openTask(run.taskId).catch(error)
    const stage=taskStages.get(run.taskId)
    if(stage){context.dataset.stage=stage;context.title+=' · Stage: '+stage}
    // The skill badge speaks for the task's newest skill (#586); the rest of the row for its leading run.
    const status=document.createElement('span');status.className='status task-skill-status';status.dataset.runId=skillRun.id
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
    archive.disabled=archiving.has(key)
    archive.onclick=()=>{archive.disabled=true;requestArchive(run)}
    const pencil=document.createElement('button');pencil.className='task-rename-button'
    pencil.innerHTML='<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true"><path d="M4 20h4L19 9l-4-4L4 16z"/><path d="m13.5 6.5 4 4"/></svg>'
    // Labels read the current name, so an edition ended in place relabels its row.
    const label=()=>{
     title.textContent=displayedName(run)
     button.title=title.textContent+' · '+runLabel(run)+' · '+stateLabel+' · '+executions.length+' execution(s)'
     const archiveTitle=archiveLabel(taskState(run).name||run.taskKey||run.taskId||runLabel(run),{active:executions.some(activeRun),ticket:archivesWorktree(run)})
     archive.title=archiveTitle;archive.setAttribute('aria-label',archiveTitle)
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
    const [link,...others]=pullRequests.get(run.taskId)||[]
    if(link){
     const pr=document.createElement('button');pr.className='pr-indicator'
     renderPullRequestIndicator(pr,link,prLabel(link.url)+' for '+(run.taskKey||run.taskId))
     pr.onclick=()=>api.openPR(link.url).catch(error);row.append(pr)
     if(others.length)row.append(otherPullRequestsBadge(others))
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
 addFolderButton.hidden=!agentConnected||!!current?.conversation||!offersRunFolder(current,runFoldersAvailable,runFoldersTerminalsAvailable)
 // The outcome belongs to the run it was given for.
 if(addFolderRun!==selected){addFolderRun=null;addFolderStatus.textContent=''}
 addFolderStatus.hidden=addFolderButton.hidden||!addFolderStatus.textContent
 document.querySelector('#save-log').disabled=!!current?.conversation
 const history=document.querySelector('#execution-history')
 const executions=current?runs.filter(run=>taskKey(run)===taskKey(current)).sort((a,b)=>(a.createdAt||'').localeCompare(b.createdAt||'')||a.id.localeCompare(b.id)):[]
 history.replaceChildren();history.hidden=executions.length<2
 for(const [i,run] of executions.entries()){const option=document.createElement('option');option.value=run.id;option.textContent=(i+1)+' · '+run.skill+' · '+run.status;history.append(option)}
 history.value=selected||'';history.onchange=()=>{const run=runs.find(run=>run.id===history.value);if(run)select(run)}
 const selectedPR=document.querySelector('#selected-pr'),selectedOthers=document.querySelector('#selected-pr-others')
 const [link,...others]=(current&&pullRequests.get(current.taskId))||[]
 selectedPR.hidden=!link
 if(link){
  const label=prLabel(link.url)
  renderPullRequestIndicator(selectedPR,link,label)
  const text=document.createElement('span');text.className='pr-label';text.textContent=label;selectedPR.append(text)
  selectedPR.onclick=()=>api.openPR(link.url).catch(error)
 }
 // The other repositories the task changed, each with its own pull request,
 // after the primary repository's.
 selectedOthers.replaceChildren(...others.map(other=>{
  const button=document.createElement('button');button.type='button';button.className='icon-button selected-pr-other'
  const label=prLabel(other.url)
  renderPullRequestIndicator(button,other,label+' in '+other.repository)
  const text=document.createElement('span');text.className='pr-label';text.textContent=repositoryName(other.repository)+' '+label;button.append(text)
  button.onclick=()=>api.openPR(other.url).catch(error)
  return button
 }))
 selectedOthers.hidden=!others.length
 // A macro run is relaunched from the macro panel: it has no task to relaunch here.
 document.querySelector('#rerun').hidden=!current||macroRun(current)
 document.querySelector('#stop').disabled=stopping||!current||!activeRun(current)
 const detachBtn=document.querySelector('#detach-terminal')
 if(detachBtn){
  const canDetach=current&&current.status==='running'&&!current.externalTerminal&&!current.conversation
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
  conversation.select(null)
  selected=null;terminal.reset()
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
  // A new execution of the displayed ticket takes the console over (#639).
  const followed=followedExecution(runs,next,selected,taskKey,run=>!freeConsole(run)&&!macroRun(run)&&!hiddenRun(run))
  const serialized=JSON.stringify(next),changed=serialized!==last
  runs=next;last=serialized
  await updateDisconnected(status.disconnectedProjects||[],changed,true)
  const current=runs.find(run=>run.id===selected)
  if(followed&&selected)select(followed,true,{deferrable:true})
  else if(current&&((current.status!==previous?.status&&(current.status==='running'||!current.sessionId))||current.sessionId!==previous?.sessionId))select(current,true,{deferrable:true})
  if(!selected){const visible=runs.find(run=>!hiddenRun(run));if(visible)select(visible,true,{deferrable:true})}
  if(changed||Date.now()-nextStepUpdated>15000)refreshNextStep()
  if(changed)loadCustomSkillSignal()
  refreshVisibleSkillResults()
 }catch{agentUnavailable()}
 finally{refreshing=false}
}
async function startLocalAgent(form){
 if(startPending)return
 startPending=true;startReason=credential.state==='present'&&form.querySelector('#pair-again').hidden?'Starting the local agent with the saved key…':'';updateStartControl()
 try{
  // A closed disclosure still contributes its inputs to FormData: its code is dropped explicitly.
  const values=Object.fromEntries(new FormData(form));if(form.querySelector('#pair-again').hidden)values.code=''
  const result=await api.start(values)
  if(result?.needsPairing){pairingReason=result.needsPairing;startReason='';return}
  form.elements.code.value='';pairingReason='';startReason='';credential=await api.credentialState();document.querySelector('#error').textContent='';ready();await refresh()
 }catch(err){pairingReason='';startReason=startFailure(err)}
 finally{startPending=false;updateStartControl()}
}
document.querySelector('#start').onsubmit=event=>{event.preventDefault();return startLocalAgent(event.target)}
// A browser sign-in holds the start controls like a start does: it ends by starting the agent on the key it pairs (#717).
document.querySelector('#browser-sign-in').onclick=async()=>{
 if(startPending)return
 // A refusal is kept until the sign-in succeeds: the pairing form stays open, never offering the refused key again.
 startPending=true;startReason='Waiting for the sign-in in your browser…';updateStartControl()
 const before=credential.state
 try{
  await api.signIn(connectForm.elements.server.value)
  pairingReason='';startReason='';credential=await api.credentialState();document.querySelector('#error').textContent='';ready();await refresh()
 }catch(err){
  startReason=startFailure(err)
  try{credential=await api.credentialState()}catch{}
  // A key the sign-in saved before failing replaces a missing or unreadable one: that reason no longer holds.
  if(before!=='present'&&credential.state==='present')pairingReason=''
 }
 finally{startPending=false;updateStartControl()}
}
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
 showDialog('Skip to Handoff for '+key+'?')
 paragraph('Use this when the pull request needs no more changes: Adjust is skipped, '+key+' moves to #reviewed, and Handoff is proposed once the pull request is merged.')
 const confirm=document.createElement('button')
 confirm.textContent='Skip to Handoff'
 const notice=document.createElement('p')
 notice.setAttribute('role','status')
 dialogBody.append(confirm,notice)
 confirm.focus()
 confirm.onclick=async()=>{
  confirm.disabled=true
  notice.textContent='Moving '+key+' to reviewed…'
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
// After a reboot the agent is not running: with a usable saved key the desktop starts it once, unasked (#716).
async function launch(){
 if(await api.connect()){ready();refresh();api.credentialState().then(state=>{credential=state;updateStartControl()}).catch(()=>{});return}
 // The saved key is read before the setup screen is drawn, so a start on it never shows the pairing code on the way (#717).
 credential=await api.credentialState().catch(()=>({state:'unreadable',server:''}))
 agentUnavailable()
 if(credential.server)connectForm.elements.server.value=credential.server
 if(credential.state!=='present'){pairingReason=credentialReason(credential.state);updateStartControl();return}
 if(autoStartTried)return
 autoStartTried=true;updateStartControl();await startLocalAgent(connectForm)
}
const credentialReason=state=>state==='unreadable'?'The key saved on this workstation can no longer be read. Pair again to replace it.':'This workstation has no saved key yet. Paste a pairing code from your profile in the web interface.'
launch().catch(error)
setInterval(async()=>{
 const action=pollAction({restarting,starting:startPending,agentConnected,shutdownVisible:!document.querySelector('#shutdown').hidden})
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
   conversation.select(null);selected=null;runs=[];last='';terminal.reset();render()
   renderHeader()
   showDirectory('')
   document.querySelector('#error').textContent=''
  }
 }catch(err){error(err)}finally{restarting=false;button.disabled=false;updateStartControl();await refresh()}
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
   conversation.select(null);selected=null;runs=[];last='';terminal.reset();render()
   document.querySelector('#setup').hidden=false;document.querySelector('#workspace').hidden=true
   document.querySelector('#restart').hidden=true;button.hidden=true
   document.querySelector('#start button[type=submit]').disabled=false
   agentUnavailable()
   document.querySelector('#error').textContent=''
  }
 }catch(err){error(err)}finally{restarting=false;button.disabled=false;updateStartControl()}
}

document.querySelector('#shutdown').onclick=stopLocalAgent

document.querySelector('#clear-history').onclick=async()=>{
 const button=document.querySelector('#clear-history');button.disabled=true
 try{
  const {removed}=await api.clearHistory()
  // Drop the removed runs locally right away: until the next poll the desktop
  // would otherwise keep asking for results the agent no longer holds.
  runs=runs.filter(run=>!removed.includes(run.id))
  for(const id of removed){skillResults.delete(id);skillResultReads.delete(id)}
  if(removed.includes(selected)){
   conversation.select(null)
   selected=null;terminal.reset()
   renderHeader()
   showDirectory('')
  }
  await refresh()
 }catch(err){error(err)}finally{render()}
}

const dialog=document.querySelector('#project-dialog'),dialogBody=document.querySelector('#dialog-body')
const dialogFooter=document.querySelector('.dialog-footer')
let configurationPage=null,configurationGeneration=0,configurationHidden=null,expandedConfigurationProject=null
function returnConnectForm(){if(connectForm.parentElement!==document.querySelector('#setup'))document.querySelector('#setup').append(connectForm);renderStartForm(credential.state==='present'&&!pairingReason)}
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
 if(dialog.open||configurationActive())return
 updateSettingsConnection=null
 returnConnectForm()
 dialogBody.replaceChildren()
 clearDialogFooter()
})
function showDialog(title){
 closeConfiguration()
 updateSettingsConnection=null
 dialog.classList.remove('workstation-settings','palette-dialog','quick-add-dialog')
 returnConnectForm()
 // The footer is shared by every dialog, so a control one of them added there
 // must go before the next one opens.
 clearDialogFooter()
 dialogBody.replaceChildren()
 const heading=document.createElement('h2');heading.textContent=title;dialogBody.append(heading)
 if(!dialog.open)dialog.showModal()
}
// Configuration is a page, not a dialog. Reusing the existing body and footer
// keeps every established setting control and its local save behaviour intact;
// the page shell alone owns entering, leaving and restoring the workspace.
function showConfiguration(title){
 if(configurationPage){
  configurationPage.searchCleanup?.()
  configurationGeneration++
  updateSettingsConnection=null
  returnConnectForm()
  dialogBody.replaceChildren()
  clearDialogFooter()
  return
 }
 configurationGeneration++
 updateSettingsConnection=null
 if(dialog.open)dialog.close()
 const page=document.createElement('section');page.className='configuration-page';page.setAttribute('aria-label','Configuration')
 page.returnFocus=document.activeElement
 const back=document.createElement('button');back.type='button';back.className='configuration-back';back.textContent='←';back.setAttribute('aria-label','Back');back.title='Back';back.onclick=closeConfiguration
 const content=document.createElement('div');content.className='configuration-body'
 const workspace=document.querySelector('#workspace')
 workspace.hidden=false;document.querySelector('#setup').hidden=true
 configurationHidden=new Map([...workspace.children].map(child=>[child,child.hidden]))
 for(const child of configurationHidden.keys())child.hidden=true
 workspace.append(page);page.append(content)
 page.backButton=back
 content.append(dialogBody,dialogFooter)
 dialog.classList.remove('workstation-settings')
 dialogBody.replaceChildren()
 clearDialogFooter()
 const dialogHeading=document.createElement('h2');dialogHeading.textContent=title;dialogHeading.className='visually-hidden';dialogBody.append(dialogHeading)
 configurationPage=page
}
function closeConfiguration(){
 if(!configurationPage)return
 configurationGeneration++
 const page=configurationPage;page.searchCleanup?.();configurationPage=null
 updateSettingsConnection=null
 returnConnectForm()
 dialog.append(dialogBody,dialogFooter)
 dialogBody.replaceChildren()
 clearDialogFooter()
 page.remove()
 for(const [child,hidden] of configurationHidden||[])child.hidden=hidden
 configurationHidden=null
 document.querySelector('#workspace').hidden=!agentConnected
 document.querySelector('#setup').hidden=agentConnected
 resize()
 if(page.returnFocus?.isConnected)page.returnFocus.focus()
}
const configurationActive=()=>!!configurationPage
function paragraph(text){const p=document.createElement('p');p.textContent=text;dialogBody.append(p);return p}
window.addEventListener('keydown',event=>{
 if(event.key==='Escape'&&!dialog.open&&configurationActive()){event.preventDefault();closeConfiguration();return}
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
const KNOWN_COMMANDS=['',"/path/to/custom-cli {mode:-p|-i} '{prompt}'","claude --model {model} '{prompt}'",'agy --dangerously-skip-permissions --model {model} "{prompt}"',"codex --model {model} '{prompt}'"]
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
function executionDefaultsPanel(panel,modelPanels){
 const unavailable=document.createElement('p');unavailable.className='execution-unavailable';unavailable.setAttribute('role','status');unavailable.hidden=true
 const body=document.createElement('div');body.className='execution-defaults';body.hidden=true
 let view=null
 // A field is "stated" when the workstation sets it; unset, it runs the default.
 const stated={}
 const changed=()=>{notice.textContent='';notice.dataset.tone=''}

 // Deployment configures MCP per provider: this selects its target CLI.
 const providerSelect=document.createElement('select');providerSelect.className='provider-select';providerSelect.setAttribute('aria-label','MCP provider')
 providerOptions(providerSelect)

 // The models a launch may pick, per provider. A provider without a list of
 // its own offers the one Sectile ships; editing it creates the list.
 const listInputs={}
 const modelHosts={}
 for(const [id,target] of Object.entries(modelPanels)){
  const host=document.createElement('div');host.className='provider-model-lists';host.dataset.provider=id
  const title=document.createElement('h3');title.textContent=({agy:'Antigravity',claude:'Claude',codex:'Codex'})[id]
  target.append(title,host);modelHosts[id]=host
 }

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
 const globalCommands=skillCommandMapping({validate:validSkillCommand,onChange:changed})
 const commandsRow=settingRow('Skill command names',{stacked:true,resetLabel:'Reset skill command names to the standard ones',onReset:()=>{globalCommands.set({});changed()}},globalCommands.box)
 commandsRow.hint.textContent='Commands used for all projects on this workstation. Empty entries run the standard command.'
 // Which skill a dispatch runs (#267): a project's edited skill, or the one
 // installed on this workstation, from the direct copy or the Claude plugin.
 let customSkillsWin=null
 const customGroup=document.createElement('div');customGroup.className='segmented'
 customGroup.setAttribute('role','group');customGroup.setAttribute('aria-label','Custom project skills win')
 const customButtons=['Yes','No'].map(value=>{
  const button=document.createElement('button');button.type='button';button.textContent=value
  button.onclick=()=>{customSkillsWin=value==='Yes';changed();render()}
  customGroup.append(button);return button
 })
 const customRow=settingRow('Custom project skills win',{resetLabel:'Reset custom project skills win to default',onReset:()=>{customSkillsWin=null;changed();render()}},customGroup)
 const customHelp=document.createElement('p');customHelp.className='setting-help'
 customHelp.textContent='A skill a project edited in Sectile runs instead of the one installed on this workstation.'
 const customUsedList=document.createElement('ul')
 const customUsedTitle=document.createElement('strong');customUsedTitle.textContent='Custom skills used'
 const customUsed=document.createElement('div');customUsed.className='custom-skills-notice';customUsed.setAttribute('role','note');customUsed.hidden=true
 customUsed.append(customUsedTitle,customUsedList)
 customRow.section.querySelector('.setting-text').append(customHelp,customUsed)
 let installedSkillSource=''
 const sourceSelect=document.createElement('select');sourceSelect.setAttribute('aria-label','Installed skills source')
 for(const [value,label] of [['direct','Direct copy'],['plugin','Claude plugin']]){
  const option=document.createElement('option');option.value=value;option.textContent=label;sourceSelect.append(option)
 }
 sourceSelect.onchange=()=>{installedSkillSource=sourceSelect.value;changed();render()}
 const sourceRow=settingRow('Installed skills source',{resetLabel:'Reset installed skills source to default',onReset:()=>{installedSkillSource='';changed();render()}},sourceSelect)
 const sourceHelp=document.createElement('p');sourceHelp.className='setting-help'
 sourceHelp.textContent='Tried first when a skill is not custom; the other one is the fallback. The Claude plugin serves Claude only.'
 sourceRow.section.querySelector('.setting-text').append(sourceHelp)
 function renderCustomSkillsUsed(list){
  customUsedList.replaceChildren()
  for(const use of list){
   const item=document.createElement('li')
   const when=use.lastRun?new Date(use.lastRun).toLocaleString():''
   item.textContent=(use.projectName||use.projectId)+' · '+use.directory+(when?' · '+when:'')
   customUsedList.append(item)
  }
  customUsed.hidden=!list.length
 }
 const notice=document.createElement('p');notice.setAttribute('role','status');notice.className='workstation-notice'
 const save=document.createElement('button');save.type='button';save.className='dialog-action primary';save.textContent='Save execution defaults'
 const actions=document.createElement('div');actions.className='deployment-actions';actions.style.marginTop='16px'
 actions.append(save,notice)
 body.append(terminalRow.section,editorRow.section,worktreeRow.section,parallelRow.section,commandsRow.section,customRow.section,sourceRow.section,actions)

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
  const limit=parallelism||DEFAULT_PARALLELISM
  parallelInput.value=String(limit)
  parallelReadout.textContent=limit+(limit===1?' execution':' executions')
  hint(parallelRow,parallelism!==0,DEFAULT_PARALLELISM+' executions')
  const customWins=customSkillsWin??true
  customButtons.forEach((button,i)=>button.setAttribute('aria-pressed',String(customWins===(i===0))))
  hint(customRow,customSkillsWin!==null,'Yes')
  sourceSelect.value=installedSkillSource||'direct'
  hint(sourceRow,!!installedSkillSource,'Direct copy')
 }

 function fill(){
  const defaults=view.defaults||{}
  globalCommands.set(defaults.skillCommands||{},view.skillCommands||[])
  const globalAvailable=view.globalConfiguration===true
  for(const input of commandsRow.section.querySelectorAll('input,button,select'))input.disabled=!globalAvailable
  commandsRow.hint.textContent=globalAvailable?'Commands used for all projects on this workstation. Empty entries run the standard command.':'Update and restart the local agent to edit global skill command names.'
  // An agent that predates #510 names its provider directly.
  const provider=view.effective?.defaultEngine?.provider||view.effective?.aiProvider||DEFAULT_PROVIDER
  providerOptions(providerSelect,[provider])
  providerSelect.value=provider
  terminal.set(defaults.terminal||'')
  editor.set(defaults.editorCommand||'')
  useWorktrees=typeof defaults.useWorktrees==='boolean'?defaults.useWorktrees:null
  parallelism=Number(defaults.parallelism)||0
  setupProviders=Array.isArray(defaults.setupProviders)?[...defaults.setupProviders]:null
  customSkillsWin=typeof defaults.customSkillsWin==='boolean'?defaults.customSkillsWin:null
  installedSkillSource=defaults.installedSkillSource||''
  renderCustomSkillsUsed(Array.isArray(view.customSkillsUsed)?view.customSkillsUsed:[])
  renderCustomSkillSignal(view.customSkillsUsed)
  for(const host of Object.values(modelHosts))host.replaceChildren()
  for(const key of Object.keys(listInputs))delete listInputs[key]
  const providers=[...new Set([...Object.keys(modelPanels),...Object.keys(view.providerModels||{}),...Object.keys(defaults.aiProviderModels||{})])].sort()
  for(const id of providers){
   if(!modelHosts[id])continue
   const input=document.createElement('input');input.type='text';input.className='model-input';input.setAttribute('aria-label','Models offered for '+id)
   const own=defaults.aiProviderModels&&Object.prototype.hasOwnProperty.call(defaults.aiProviderModels,id)
   stated['models:'+id]=!!own
   input.value=own?(defaults.aiProviderModels[id]||[]).join(', '):''
   input.oninput=()=>{stated['models:'+id]=true;changed();render()}
   const row=settingRow('Models offered',{resetLabel:'Reset '+id+' models to the shipped list',onReset:()=>{stated['models:'+id]=false;render()}},input)
   listInputs[id]={input,row}
   const modelNotice=document.createElement('p');modelNotice.setAttribute('role','status')
   const modelSave=document.createElement('button');modelSave.type='button';modelSave.className='dialog-action primary';modelSave.textContent='Save models'
   modelSave.onclick=async()=>{
    const models=parseModelList(input.value),custom=stated['models:'+id]
    if(custom&&models.some(model=>!validateModel(model))){modelNotice.textContent='Invalid model in the list of '+id;return}
    modelSave.disabled=true;modelNotice.textContent='Saving…'
    try{
     const latest=await api.workstationSettings()
     const defaults={...latest.defaults,aiProviderModels:{...latest.defaults?.aiProviderModels}}
     if(custom)defaults.aiProviderModels[id]=models
     else delete defaults.aiProviderModels[id]
     await api.saveWorkstationSettings(defaults)
     view.defaults.aiProviderModels=defaults.aiProviderModels
     modelNotice.textContent='Models saved'
    }catch(err){modelNotice.textContent='Not saved: '+ipcMessage(err)}
    finally{modelSave.disabled=false}
   }
   modelHosts[id].append(row.section,modelSave,modelNotice)
  }
  render()
 }
 function state(){
  return {
   terminal:terminal.get(),editorCommand:editor.get(),useWorktrees,parallelism,setupProviders,customSkillsWin,installedSkillSource,...(view.globalConfiguration?{skillCommands:compact(globalCommands.get())}:{})
  }
 }
 save.onclick=async()=>{
  if(globalCommands.invalid()){notice.textContent='A skill command name is a single word, optionally led by / and by a plugin name such as sectile:.';notice.dataset.tone='error';return}
  save.disabled=true;notice.textContent='Saving…';notice.dataset.tone=''
  try{
   const current=state(),saved=JSON.stringify(current)
   await api.saveWorkstationSettings(workstationPayload(current,view.defaults))
   loadEditorSetting()
   // "Saved" is announced once the panel shows what the agent now serves, and
   // that refill leaves alone a setting edited while it was on its way.
   try{view=await api.workstationSettings();if(body.isConnected&&JSON.stringify(state())===saved)fill()}catch{}
   notice.textContent='Execution defaults saved'
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
 }
 panel.append(unavailable,body)
 return {providerSelect,load}
}

// The workstation's own settings, gathered where the project panel already puts
// a project's: one dialog, a category per surface. The header carried three
// unrelated controls for these; the sidebar now carries one.
const SETTINGS_CATEGORIES=[
 {id:'Profile',label:'General',icon:'<circle cx="12" cy="8" r="4"/><path d="M4 21v-2a8 8 0 0 1 16 0v2"/>'},
 {id:'Connection',label:'Agent connection',icon:'<path d="M4 7h16M4 17h16"/><circle cx="9" cy="7" r="3"/><circle cx="15" cy="17" r="3"/>'},
 {id:'AgentCli',label:'Execution defaults',icon:'<rect x="3" y="4" width="18" height="16" rx="2"/><path d="m7 9 3 3-3 3M12 15h5"/>'},
 {id:'Engines',label:'AI engines',icon:'<rect x="4" y="8" width="16" height="11" rx="3"/><path d="M12 4v4M9 13h.01M15 13h.01"/>'},
 {id:'Sandbox',label:'Claude settings',mark:'claude'},
 {id:'Codex',label:'Codex settings',icon:'<path fill="currentColor" stroke="none" fill-rule="evenodd" d="'+OPENAI_MARK_PATH+'"/>'},
 {id:'Deployment',label:'Deployment',icon:'<path d="M12 20V7m0 0 4 4m-4-4-4 4"/><path d="M5 4h14"/>'},
 {id:'Logs',label:'Agent logs',icon:'<path d="M14 3H7a1 1 0 0 0-1 1v16a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1V7Z"/><path d="M14 3v4h4"/><path d="M9 13h6M9 17h6"/>'},
 {id:'Changelog',label:'Changelog',icon:'<circle cx="12" cy="12" r="9"/><path d="M12 11v5"/><path d="M12 8h.01"/>'}
]
const PROJECT_SETTINGS_CATEGORIES=[
 {id:'Remove',label:'General',saves:true,icon:'<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7h.01"/>'},
 {id:'General',label:'Folders',saves:true,icon:'<path d="M4 7a2 2 0 0 1 2-2h3l2 2.5h7a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2Z"/>'},
 {id:'Execution',label:'Execution',saves:true,icon:'<path d="M4 7h16M4 17h16"/><circle cx="9" cy="7" r="2.4"/><circle cx="15" cy="17" r="2.4"/>'},
 {id:'Sandbox',label:'Claude settings',saves:true,mark:'claude'}
]
function configurationNavigation(tabs,projectId){
 if(projectId)expandedConfigurationProject=projectId
 const current=new Map([...tabs.querySelectorAll('button[data-category]')].map(tab=>[tab.dataset.category,tab]))
 tabs.replaceChildren(configurationPage.backButton)
 const projectGroups=[]
 function updateProjectGroups(){
  for(const group of projectGroups){
   const expanded=group.id===expandedConfigurationProject
   group.toggle.setAttribute('aria-expanded',String(expanded))
   for(const tab of group.tabs)tab.hidden=!expanded
  }
 }
 function group(label,categories,active,navigate,id){
  const heading=document.createElement('h2');heading.className='settings-group-label';heading.textContent=label;tabs.append(heading)
  const projectGroup=id?{id,tabs:[]}:null
  if(projectGroup){
   const toggle=document.createElement('button');toggle.type='button';toggle.className='project-settings-toggle';toggle.textContent=label
   toggle.onclick=()=>{expandedConfigurationProject=expandedConfigurationProject===id?null:id;updateProjectGroups()}
   heading.replaceChildren(toggle);projectGroup.toggle=toggle;projectGroups.push(projectGroup)
  }
  for(const category of categories){
   let tab=active?current.get(category.id):null
   if(!tab){
    tab=document.createElement('button');tab.type='button';tab.setAttribute('role','tab');tab.setAttribute('aria-selected','false');tab.title=category.label
    tab.append(settingsCategoryIcon(document,category))
    const text=document.createElement('span');text.className='settings-nav-label';text.textContent=category.label;tab.append(text)
    tab.onclick=()=>navigate(category.id)
   }
   tabs.append(tab)
   if(projectGroup)projectGroup.tabs.push(tab)
  }
 }
 group('General',SETTINGS_CATEGORIES,!projectId,name=>openSettings(name))
 // Only the projects added to this workstation have settings here; the others
 // are reached from Add project.
 for(const project of projects)if(addedProject(project)||project.id===projectId)group(project.name+(sidebarHiddenProjects.has(project.id)?' (hidden)':''),PROJECT_SETTINGS_CATEGORIES,project.id===projectId,name=>openProject(project.id,name),project.id)
 updateProjectGroups()
}
// The workstation Sandbox category (#730): the Sandbox values every covered
// project applies under its own, and the projects they cover. It has its own
// save: the execution defaults form replaces the defaults whole and does not
// carry them.
function workstationSandboxPanel(panel){
 const unavailable=document.createElement('p');unavailable.className='execution-unavailable';unavailable.setAttribute('role','status');unavailable.hidden=true
 const body=document.createElement('div');body.className='workstation-sandbox';body.hidden=true
 const notice=document.createElement('p');notice.setAttribute('role','status');notice.className='workstation-notice'
 const save=document.createElement('button');save.type='button';save.className='dialog-action primary';save.textContent='Save Claude settings'
 const actions=document.createElement('div');actions.className='deployment-actions';actions.style.marginTop='16px'
 actions.append(save,notice)
 panel.append(unavailable,body)
 let sandbox=null,whitelist=null
 const STOPPED_NOTICE='Claude settings are unavailable while the local agent is stopped. Start the agent to edit them.'
 function showUnavailable(text){unavailable.textContent=text;unavailable.hidden=false;body.hidden=true}
 function fill(view){
  // A disconnected project that keeps its local settings is still covered, so
  // it is listed too, marked hidden: left out, its checkbox would be dropped by
  // the next save. A project hidden from the sidebar only is an added project.
  const added=projects.filter(project=>addedProject(project)||(hiddenProject(project.id)&&(project.configured||!!project.path)))
   .map(project=>({id:project.id,name:project.name+(hiddenProject(project.id)?' (hidden)':'')}))
  sandbox=sandboxSettings({settingRow,stored:view.claudeSandbox,platformSandbox:view.platformSandbox!==false,workstation:true})
  whitelist=whitelistEditor({settingRow,projects:added,selected:view.projects||[]})
  body.replaceChildren(whitelist.section,...sandbox.sections,actions)
  body.hidden=false
 }
 save.onclick=async()=>{
  if(!sandbox)return
  save.disabled=true;notice.textContent='Saving…';notice.dataset.tone=''
  try{
   const fresh=await api.saveWorkstationSandbox({claudeSandbox:sandbox.payload(),claudeSandboxBase:sandbox.base(),projects:whitelist.get()})
   sandbox.set(fresh.claudeSandbox);whitelist.set(fresh.projects||[])
   notice.textContent='Claude settings saved'
  }catch(err){
   if(agentUnreachable(err))showUnavailable(STOPPED_NOTICE)
   else{notice.textContent='Not saved: '+ipcMessage(err);notice.dataset.tone='error'}
  }finally{save.disabled=false}
 }
 async function load(){
  if(!agentConnected){showUnavailable(STOPPED_NOTICE);return}
  unavailable.hidden=true
  let view
  try{view=await api.workstationSandbox()}
  catch(err){
   if(!body.isConnected)return
   const text=ipcMessage(err)
   showUnavailable(agentUnreachable(err)?STOPPED_NOTICE:/404|not found/i.test(text)?'Update and restart the local agent to edit the workstation Claude settings here.':'Unable to read the Claude settings: '+text)
   return
  }
  if(body.isConnected)fill(view)
 }
 return {load}
}
function deploymentPanel(panel,mcpSection){
 const globalTitle=document.createElement('h3');globalTitle.textContent='Global AI engine setup'
 const globalHint=document.createElement('p');globalHint.textContent='Install Sectile’s skills and register its MCP server in your user configuration for the selected engine’s provider, each on its own. Engines sharing a provider share this installation. This setup is optional: for Claude, installing the sectile plugin does both, and a dispatch never installs anything.'
 const engine=document.createElement('select');engine.setAttribute('aria-label','Setup AI engine')
 panel.append(globalTitle,globalHint,settingRow('AI engine',{},engine).section)
 const source=projects.find(project=>project.id===selectedProject)||projects[0]
 let engines=[]
 const selectedEngine=()=>engines.find(item=>item.id===engine.value)
 // Skills: the server's skills for the provider, nothing else.
 const installSkills=document.createElement('button');installSkills.type='button';installSkills.textContent='Install skills';installSkills.disabled=true
 const skillsRow=settingRow('Skills',null,installSkills)
 skillsRow.hint.textContent=source?'Supplied by '+source.name+'. Installation is user-wide, not project-local.':'Connect a project to obtain the server skills.'
 const skillsResult=document.createElement('p');skillsResult.className='setup-result';skillsResult.setAttribute('role','status')
 // MCP: registered on its own, over HTTP, to the server or through the agent.
 const mcpModes=document.createElement('div');mcpModes.className='segmented';mcpModes.setAttribute('role','group');mcpModes.setAttribute('aria-label','MCP target')
 let mcpTarget='remote'
 const MCP_TARGETS=[['remote','Remote server','The engine calls the Sectile server over HTTP with your API key, written in its configuration.'],['local','Local agent','The engine calls this workstation’s agent over HTTP, which forwards with its own identity: no key is written, and the agent must keep running.']]
 const registerMCP=document.createElement('button');registerMCP.type='button';registerMCP.textContent='Register MCP';registerMCP.disabled=true
 const mcpRow=settingRow('MCP server',null,mcpModes,registerMCP)
 const markTarget=()=>{for(const button of mcpModes.children)button.setAttribute('aria-pressed',String(button.dataset.value===mcpTarget));mcpRow.hint.textContent=MCP_TARGETS.find(([value])=>value===mcpTarget)[2]}
 for(const [value,label] of MCP_TARGETS){const button=document.createElement('button');button.type='button';button.dataset.value=value;button.textContent=label;button.onclick=()=>{mcpTarget=value;markTarget()};mcpModes.append(button)}
 const mcpResult=document.createElement('p');mcpResult.className='setup-result';mcpResult.setAttribute('role','status')
 panel.append(skillsRow.section,skillsResult,mcpRow.section,mcpResult)
 markTarget()
 // The mode the provider is registered in today, when the agent knows it.
 const loadTarget=()=>{
  const provider=selectedEngine()?.provider
  if(!provider||!mcpProviders[provider])return
  api.mcpConfig(provider).then(info=>{if(panel.isConnected&&info?.choice?.target){mcpTarget=info.choice.target==='local'?'local':'remote';markTarget()}}).catch(()=>{})
 }
 const enable=()=>{const ready=!!selectedEngine();installSkills.disabled=!ready||!source;registerMCP.disabled=!ready}
 api.engines().then(view=>{
  if(!panel.isConnected)return
  engines=view.catalogue||[]
  for(const item of engines){const option=document.createElement('option');option.value=item.id;option.textContent=item.name+' ('+item.provider+')';engine.append(option)}
  engine.value=view.default||engines[0]?.id||''
  enable();loadTarget()
 }).catch(()=>{globalHint.textContent='Update the local agent to configure setup by AI engine.'})
 engine.addEventListener('change',()=>{skillsResult.textContent='';mcpResult.textContent='';enable();loadTarget()})
 installSkills.onclick=async()=>{
  const chosen=selectedEngine();if(!chosen||!source)return
  installSkills.disabled=true;skillsResult.textContent='Installing skills…'
  try{
   const info=await api.project(source.id)
   if(!info.configured)throw Error('Configure '+source.name+'’s local folder first: the skills come from its server configuration.')
   const result=await api.deployProject(source.id,'provider-skills',chosen.provider)
   if(!panel.isConnected)return
   const step=result.skills||{}
   skillsResult.textContent=({success:'Installed',failed:'Failed',skipped:'Skipped'}[step.status]||'Done')+' · '+(step.message||result.message||'')
  }catch(err){if(panel.isConnected)skillsResult.textContent=/unknown/i.test(ipcMessage(err))?'Update and restart the local agent to install skills on their own.':ipcMessage(err)}
  finally{enable()}
 }
 registerMCP.onclick=async()=>{
  const chosen=selectedEngine();if(!chosen)return
  if(!mcpProviders[chosen.provider]){mcpResult.textContent='Register MCP manually for this provider: see Settings → MCP connection.';return}
  registerMCP.disabled=true;mcpResult.textContent='Registering the MCP server…'
  try{
   const result=await api.configureMCP(chosen.provider,{target:mcpTarget,transport:'http'})
   if(panel.isConnected)mcpResult.textContent='Registered in '+result.path+' ('+(mcpTarget==='local'?'local agent':'remote server')+', HTTP). Restart the AI engine to reconnect.'
  }catch(err){if(panel.isConnected)mcpResult.textContent='Registration failed: '+ipcMessage(err)}
  finally{enable()}
 }
 if(mcpSection)panel.append(mcpSection)
 const localTitle=document.createElement('h3');localTitle.textContent='Local project SDD setup'
 const localHint=document.createElement('p');localHint.textContent='Install the project’s SDD framework in its local repository. This does not install global engine skills or MCP.'
 panel.append(localTitle,localHint)
 const target=document.createElement('select');target.setAttribute('aria-label','Deployment project')
 for(const project of projects){const option=document.createElement('option');option.value=project.id;option.textContent=project.name;target.append(option)}
 if(projects.some(project=>project.id===selectedProject))target.value=selectedProject
 panel.append(settingRow('Project',{},target).section)
 const notice=document.createElement('p');notice.setAttribute('role','status')
 const results=document.createElement('div');results.className='initialization-result';results.setAttribute('role','status')
 const actions=document.createElement('div');actions.className='deployment-actions'
 let pending=false
 const framework=document.createElement('button');framework.type='button';framework.textContent='Install SDD in project';framework.disabled=!projects.length
 framework.onclick=async()=>{
  const projectId=target.value
  if(pending||!projectId)return
  pending=true;target.disabled=true;framework.disabled=true
  results.replaceChildren();notice.textContent='Deployment in progress…'
  try{
   const info=await api.project(projectId)
   if(!info.configured)throw Error('Configure this project’s local folder before deployment.')
   const result=await api.deployProject(projectId,'framework')
   if(panel.isConnected)notice.textContent=result.message||'Deployment complete'
  }catch(err){if(panel.isConnected)notice.textContent=ipcMessage(err)}finally{pending=false;target.disabled=false;framework.disabled=false}
 }
 actions.append(framework)
 panel.append(actions,notice,results)
}
function openSettings(initial='Profile',project){
 if(initial==='Appearance')initial='Profile'
 showConfiguration('Configuration')
 const generation=configurationGeneration
 const layout=document.createElement('div');layout.className='settings-layout workstation-settings'
 const tabs=document.createElement('div');tabs.className='settings-nav';tabs.setAttribute('role','tablist')
  tabs.setAttribute('aria-orientation','vertical');tabs.setAttribute('aria-label','Settings categories')
 const generalLabel=document.createElement('h2');generalLabel.className='settings-group-label';generalLabel.textContent='General'
 tabs.append(generalLabel)
 const content=document.createElement('div');content.className='settings-content stretch'
 const pageTitle=document.createElement('h2');pageTitle.className='configuration-panel-title';content.append(pageTitle)
 layout.append(tabs,content);dialogBody.append(layout)
 const panels={}
 for(const category of SETTINGS_CATEGORIES){
  const tab=document.createElement('button');tab.type='button';tab.setAttribute('role','tab');tab.dataset.category=category.id;tab.title=category.label
  tab.append(settingsCategoryIcon(document,category))
  const text=document.createElement('span');text.className='settings-nav-label';text.textContent=category.label;tab.append(text)
  tab.id='settings-tab-'+category.id;tab.setAttribute('aria-controls','settings-panel-'+category.id)
  const panel=document.createElement('section');panel.id='settings-panel-'+category.id
  panel.setAttribute('role','tabpanel');panel.setAttribute('aria-labelledby',tab.id);panels[category.id]=panel
  tab.onclick=()=>selectCategory(category.id)
  tabs.append(tab);content.append(panel)
 }
 if(project){
  const projectLabel=document.createElement('h2');projectLabel.className='settings-group-label';projectLabel.textContent=project.name
  tabs.append(projectLabel)
  for(const category of PROJECT_SETTINGS_CATEGORIES){
   const tab=document.createElement('button');tab.type='button';tab.setAttribute('role','tab');tab.title=category.label
   tab.append(settingsCategoryIcon(document,category))
   const text=document.createElement('span');text.className='settings-nav-label';text.textContent=category.label;tab.append(text)
   tab.onclick=()=>openProject(project.id,category.id)
   tabs.append(tab)
  }
 }
 function selectCategory(name){
  pageTitle.textContent=SETTINGS_CATEGORIES.find(category=>category.id===name).label
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
 panels.Profile.append(appearance.section)
 const viewGroup=document.createElement('div');viewGroup.className='segmented'
 viewGroup.setAttribute('role','group');viewGroup.setAttribute('aria-label','AI consoles')
 const markView=value=>{for(const button of viewGroup.children)button.setAttribute('aria-pressed',String(button.dataset.value===value))}
 for(const choice of CONSOLE_VIEW_CHOICES){
  const button=document.createElement('button');button.type='button';button.textContent=choice.label;button.dataset.value=choice.value
  button.onclick=()=>api.setConsoleView(choice.value).then(value=>{consoleView=value;markView(value);showConversationMode();render()}).catch(error)
  viewGroup.append(button)
 }
 const view=settingRow('AI consoles',null,viewGroup)
 view.hint.textContent='Conversation opens Claude and Codex interactive launches in a structured view (experimental). Other engines keep the terminal.'
 panels.Profile.append(view.section)
 // Claude settings holds the initial conversation permission mode. It applies
 // only in the conversation view, and only to
 // an agent that applies it, so it is disabled otherwise and says why.
 const modeSelect=document.createElement('select');modeSelect.setAttribute('aria-label','Conversation permission mode')
 for(const choice of CONVERSATION_MODE_CHOICES){const option=document.createElement('option');option.value=choice.value;option.textContent=choice.label;modeSelect.append(option)}
 modeSelect.value='acceptEdits'
 const conversationMode=settingRow('Conversation permission mode',null,modeSelect)
 conversationMode.section.querySelector('.setting-name strong').prepend(claudeMark(document))
 let conversationModeSupported=false
 const showConversationMode=()=>{
  modeSelect.disabled=!conversationModeSupported||consoleView!=='conversation'
  conversationMode.hint.textContent=!conversationModeSupported?'Update and restart the local agent to choose the mode new conversations start in.'
   :consoleView!=='conversation'?'Applies to Claude conversations: choose Conversation in General to use it.'
   :'What Claude may do without asking in the first message of every new conversation, including the skills and prompts you launch in it. The composer changes it from the next message.'
 }
 modeSelect.onchange=()=>api.setConversationMode(modeSelect.value).then(value=>{modeSelect.value=value}).catch(error)
 panels.Sandbox.prepend(conversationMode.section)
 showConversationMode()
 api.conversationMode().then(value=>{if(configurationActive()&&generation===configurationGeneration)modeSelect.value=value}).catch(()=>{})
 api.status().then(status=>{
  if(!configurationActive()||generation!==configurationGeneration)return
  conversationModeSupported=!!status.capabilities?.includes('conversation-mode-default');showConversationMode()
 }).catch(()=>{})
 markView(consoleView)
 markAppearance('system')
 api.appearance().then(value=>{if(configurationActive()&&generation===configurationGeneration)markAppearance(value)}).catch(()=>{})

 // Execution defaults: the workstation level of every execution setting,
 // owned by the local agent. Deployment contains the MCP configuration.
 const engines=enginesSection()
 engines.section.querySelector('h3').remove()
 panels.Engines.append(engines.section)
 const offeredModels=document.createElement('section')
 const modelsTitle=document.createElement('h2');modelsTitle.textContent='Models offered'
 offeredModels.append(modelsTitle);panels.Engines.append(offeredModels)
 const execution=executionDefaultsPanel(panels.AgentCli,{agy:offeredModels,claude:offeredModels,codex:offeredModels})
 engines.load().catch(()=>{})
 const codexReviewer=document.createElement('select');codexReviewer.setAttribute('aria-label','Approval reviewer')
 for(const [value,label] of [['user','Ask me'],['auto_review','Approve on my behalf']]){const option=document.createElement('option');option.value=value;option.textContent=label;codexReviewer.append(option)}
 const codexRow=settingRow('Approval reviewer',null,codexReviewer)
 codexRow.hint.textContent='Applies to Codex conversations from the next message. Automatic review may approve or deny eligible requests; the sandbox stays active.'
 let savedCodexReviewer='user'
 const codexNotice=document.createElement('p');codexNotice.setAttribute('role','status');codexReviewer.disabled=true
 panels.Codex.append(codexRow.section,codexNotice)
 api.codexSettings().then(value=>{if(configurationActive()&&generation===configurationGeneration){savedCodexReviewer=value.approvalsReviewer;codexReviewer.value=savedCodexReviewer;codexReviewer.disabled=false}}).catch(()=>{if(panels.Codex.isConnected)codexNotice.textContent='Start or update the local agent to edit Codex settings.'})
 codexReviewer.onchange=async()=>{
  codexReviewer.disabled=true;codexNotice.textContent='Saving…';codexNotice.dataset.tone=''
  try{const value=await api.saveCodexSettings({approvalsReviewer:codexReviewer.value});savedCodexReviewer=value.approvalsReviewer;codexReviewer.value=savedCodexReviewer;codexNotice.textContent='Codex settings saved'}
  catch(err){codexReviewer.value=savedCodexReviewer;codexNotice.textContent='Not saved: '+ipcMessage(err);codexNotice.dataset.tone='error'}
  finally{codexReviewer.disabled=false}
 }
 const workstationSandbox=workstationSandboxPanel(panels.Sandbox)
 workstationSandbox.load().catch(()=>{})
 const mcpPanel=mcpSettings(api,execution.providerSelect)
 mcpPanel.section.insertBefore(settingRow('Provider',null,execution.providerSelect).section,mcpPanel.section.children[1])
 deploymentPanel(panels.Deployment,mcpPanel.section)

 const agentState=readOnlyRow('Local agent','The agent process this desktop talks to.')
 const agentActions=document.createElement('span');agentActions.className='settings-agent-actions'
 const agentButtons=[]
 const renderAgentActions=()=>{
  agentState.value.textContent=agentConnected?'Running':'Stopped'
  for(const [button,needsRunning] of agentButtons)button.disabled=needsRunning?agentActionPending||restarting||!agentConnected:!startEnabled({agentConnected,restarting,pending:agentActionPending,starting:startPending})
 }
 for(const [label,needsRunning,action] of [
  ['Start agent',false,()=>startLocalAgent(connectForm)],
  ['Stop agent',true,stopLocalAgent],
  ['Restart agent',true,restartLocalAgent]
 ]){
  const button=document.createElement('button');button.type='button';button.textContent=label
  button.onclick=async()=>{
   if(agentActionPending||restarting)return
   agentActionPending=true;updateStartControl()
   try{await action()}finally{agentActionPending=false;if(agentActions.isConnected)await fill();updateStartControl()}
  }
  agentButtons.push([button,needsRunning]);agentActions.append(button)
 }
 agentState.control.append(agentActions)
 renderAgentActions()
 renderSettingsAgent=()=>{if(agentActions.isConnected)renderAgentActions()}
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
 const reload=document.createElement('button');reload.type='button';reload.textContent='Refresh'
 const logToolbar=document.createElement('div');logToolbar.className='agent-log-toolbar';logToolbar.append(reload)
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
 configurationNavigation(tabs)
 configurationPage.searchCleanup=installSettingsSearch({tabs,content,panels,categories:SETTINGS_CATEGORIES,selectCategory})
 // The connection facts come from two sources the agent answers separately, and
 // a stopped agent still has a paired server to report: the stored settings fill
 // the panel first, the live status refines it when the agent answers.
 const fill=async()=>{
  let stored={}
  try{stored=await api.settings()}catch{}
  if(!configurationActive()||generation!==configurationGeneration||!account.value.isConnected)return
  account.value.textContent=stored.server||'Not paired'
  device.value.textContent=stored.deviceId||'Not paired'
  renderAgentActions()
  link.control.dataset.state='off'
  link.value.textContent=agentConnected?'Connecting…':'Unreachable'
  pairing.hint.textContent=stored.token
   ?'This workstation is paired. Pasting a new code re-pairs it.'
   :'Sign in with your browser, or, in the web interface, under your profile, choose Pair a workstation and paste the code here.'
  renderStartForm(credential.state==='present'&&!pairingReason)
  pairingNote.textContent=agentConnected?'Stop the local agent before connecting it to another server.':''
  // The agent answers for the execution defaults; a start or a stop from the
  // connection panel reloads them, so the panel follows the agent's state.
  if(execution.providerSelect.isConnected)execution.load().catch(()=>{}).finally(()=>{if(execution.providerSelect.isConnected)mcpPanel.load()})
  if(panels.Sandbox.isConnected)workstationSandbox.load().catch(()=>{})
  if(!agentConnected)return
  try{
   const status=await api.status()
   if(!configurationActive()||generation!==configurationGeneration||!link.value.isConnected)return
   link.control.dataset.state=status.connected?'on':'off'
   link.value.textContent=status.connected?'Connected':status.contractError?'Server incompatible':'Server disconnected'
   if(status.contractError)link.hint.textContent=status.contractError
   if(status.server)account.value.textContent=status.server
  }catch{if(configurationActive()&&generation===configurationGeneration&&link.value.isConnected)link.value.textContent='Unreachable'}
 }
 updateSettingsConnection=status=>{
  renderAgentActions()
  link.control.dataset.state=agentConnected&&status.connected?'on':'off'
  link.value.textContent=!agentConnected?'Unreachable':status.connected?'Connected':status.contractError?'Server incompatible':'Server disconnected'
  renderStartForm(credential.state==='present'&&!pairingReason)
  pairingNote.textContent=agentConnected?'Stop the local agent before connecting it to another server.':''
 }
 fill()
}
const settingsButton=document.querySelector('#settings')
settingsButton.onclick=()=>openSettings('Profile')
api.onOpenSettings(()=>{if(!dialog.open&&!configurationActive())openSettings('Profile')})
const settingsMac=isMacPlatform(navigator)
settingsButton.title='Settings ('+configShortcutLabel(settingsMac)+')'
settingsButton.setAttribute('aria-keyshortcuts',configShortcutAria(settingsMac))

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
// The sidebar's + button and the command palette open the same dialog, so the
// two entry points cannot drift apart.
async function openAddProject(){
 showDialog('Add project')
 paragraph('Discover projects from your Sectile server and configure their local directory.')
 try{
  await loadProjects()
  if(!projects.length)paragraph('No projects available on the server.')
  for(const project of projects){
   const button=document.createElement('button');button.className='discovered-project'
   const added=addedProject(project),hidden=added&&sidebarHiddenProjects.has(project.id)
   button.textContent=project.name+(hidden?' · Hidden, show in sidebar':added?' · Already added':'')
   button.disabled=added&&!hidden
   if(added&&!hidden)button.title='Use the project settings button in the sidebar to edit this project.'
   button.onclick=hidden?()=>{setSidebarHidden(project.id,false);dialog.close()}:()=>openProject(project.id);dialogBody.append(button)
  }
 }catch(err){error(err)}
}
document.querySelector('#add-project').onclick=openAddProject
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
  if(stageGroupedProjects.delete(id))saveStageGrouping()
  if(sidebarHiddenProjects.has(id))setSidebarHidden(id,false)
  await updateDisconnected([...disconnectedProjects,id])
  dialog.close()
  try{await loadProjects()}catch(err){error('Project disconnected, but refreshing projects failed: '+err.message)}
 }
 dialogBody.append(confirm,cancel,notice)
}
async function openProject(id,initial='Remove'){
 selectedProject=id
 if(!configurationActive())showConfiguration('Configuration')
 let generation=++configurationGeneration
 try{
  const info=await api.project(id)
  if(!configurationActive()||generation!==configurationGeneration)return
  showConfiguration('Configuration')
  generation=configurationGeneration
  let config=info.server

  // A single Local panel had grown into one long scroll mixing the repository
  // path, execution limits and the agent command lines. Categories in a side
  // navigation name each group and keep the panel they open short, the way the
  // project modal of the web interface does.
  const layout=document.createElement('div');layout.className='settings-layout'
  const tabs=document.createElement('div');tabs.className='settings-nav';tabs.setAttribute('role','tablist')
  tabs.setAttribute('aria-orientation','vertical');tabs.setAttribute('aria-label','Project settings categories')
  const generalLabel=document.createElement('h2');generalLabel.className='settings-group-label';generalLabel.textContent='General'
  const projectLabel=document.createElement('h2');projectLabel.className='settings-group-label';projectLabel.textContent=config.projectName
  tabs.append(generalLabel)
  for(const category of SETTINGS_CATEGORIES){
   const tab=document.createElement('button');tab.type='button';tab.setAttribute('role','tab');tab.title=category.label
   tab.append(settingsCategoryIcon(document,category))
   const text=document.createElement('span');text.className='settings-nav-label';text.textContent=category.label;tab.append(text)
   tab.onclick=()=>openSettings(category.id,{id,name:config.projectName})
   tabs.append(tab)
  }
  tabs.append(projectLabel)
  const content=document.createElement('div');content.className='settings-content'
  const pageTitle=document.createElement('h2');pageTitle.className='configuration-panel-title';content.append(pageTitle)
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
   pageTitle.textContent=PROJECT_SETTINGS_CATEGORIES.find(category=>category.id===name).label
   const stores=PROJECT_SETTINGS_CATEGORIES.find(category=>category.id===name).saves
   for(const [key,value] of Object.entries(panels))value.hidden=key!==name
   form.hidden=!stores;save.hidden=!stores;syncDialogFooter()
   for(const item of tabs.children)item.setAttribute('aria-selected',String(item.dataset.category===name))
  }
  for(const category of PROJECT_SETTINGS_CATEGORIES){
   const tab=document.createElement('button');tab.type='button';tab.setAttribute('role','tab');tab.dataset.category=category.id;tab.title=category.label
   tab.append(settingsCategoryIcon(document,category))
   const text=document.createElement('span');text.className='settings-nav-label';text.textContent=category.label;tab.append(text)
   tab.id='project-tab-'+category.id;tab.setAttribute('aria-controls','project-panel-'+category.id)
   const panel=document.createElement('section');panel.id='project-panel-'+category.id
   panel.setAttribute('role','tabpanel');panel.setAttribute('aria-labelledby',tab.id);panels[category.id]=panel
   tab.onclick=()=>selectCategory(category.id)
   tabs.append(tab)
  }
  dialogBody.append(layout)
  content.append(form)
  for(const category of PROJECT_SETTINGS_CATEGORIES){
   if(category.saves)form.append(panels[category.id]);else content.append(panels[category.id])
  }
  selectCategory(PROJECT_SETTINGS_CATEGORIES.some(category=>category.id===initial)?initial:'Remove')
  configurationNavigation(tabs,id)
  configurationPage.searchCleanup=installSettingsSearch({tabs,content,panels,categories:PROJECT_SETTINGS_CATEGORIES,selectCategory})
  const path=document.createElement('input');path.value=info.path||'';path.required=true;path.placeholder='/path/to/repository';path.setAttribute('aria-label','Local repository')
  const browse=document.createElement('button');browse.type='button';browse.textContent='Choose folder…'
  browse.onclick=async()=>{try{const selected=await api.chooseRepository();if(selected){path.value=selected;pathOffer.examine()}}catch(err){error(err)}}
  const picker=document.createElement('div');picker.className='repository-picker';picker.append(path,browse)
  const repository=settingRow('Local repository',{stacked:true},picker)
  const pathOffer=attachGitOffer(path,repository,'Local repository')
  // Two specifications folders (#736): the macro skills read and write the
  // Macro one, the issue skills write the tasks' clarifications and
  // specifications in the Issue one. Only an override is stored: each follows
  // the local repository otherwise (#484).
  function specFolderRow(kind,field,skills){
   const name=kind+' specifications folder'
   const input=document.createElement('input');input.value=info[field]||'';input.setAttribute('aria-label',name)
   const browse=document.createElement('button');browse.type='button';browse.textContent='Choose folder…';browse.setAttribute('aria-label','Choose '+name+'…')
   browse.onclick=async()=>{try{const selected=await api.chooseRepository();if(selected){input.value=selected;render({...data,[field]:selected});offer.examine()}}catch(err){error(err)}}
   const picker=document.createElement('div');picker.className='repository-picker';picker.append(input,browse)
   // The specifications either share the code checkout or live in a folder of
   // their own. Unticking reveals the folder; ticking drops the override so the
   // folder follows the local repository again.
   const sameRepo=document.createElement('input');sameRepo.type='checkbox';sameRepo.id='spec-same-repository-'+kind.toLowerCase()
   const sameRepoLabel=document.createElement('label');sameRepoLabel.className='spec-same-repository';sameRepoLabel.htmlFor=sameRepo.id
   sameRepoLabel.append(sameRepo,document.createTextNode(' '+kind+' specifications live in the code repository'))
   sameRepo.onchange=()=>{
    if(sameRepo.checked)input.value=''
    render({...data,[field]:input.value},{separate:!sameRepo.checked})
    if(!sameRepo.checked)input.focus()
   }
   input.oninput=()=>render({...data,[field]:input.value},{separate:true})
   const kindStatus=document.createElement('span');kindStatus.className='spec-kind';kindStatus.setAttribute('role','status');kindStatus.setAttribute('aria-label',name+' kind')
   const row=settingRow(name,{stacked:true},sameRepoLabel,picker,kindStatus)
   const offer=attachGitOffer(input,row,name,folder=>render({...data,[field]:folder},{separate:true}))
   const defaultKey=field.replace(/Path$/,'Default'),kindKey=field.replace(/Path$/,'Kind')
   let data=info
   function render(next,options){
    data=next
    const override=!!(next[field]||'').trim()
    const separate=override||!!options?.separate
    sameRepo.checked=!separate
    // Following the local repository, the folder has no offer of its own.
    if(picker.hidden!==!separate)offer.show(separate)
    picker.hidden=!separate
    input.placeholder='Folder holding the specifications'
    row.hint.textContent=!separate?skills+' in the local repository.'
     :'A folder of its own · Tick the box to use the local repository again.'
    // The kind is detected on the folder the agent resolves, which is not
    // always the one typed: name it, so the verdict says what it is about.
    // A value typed but not saved yet has not been examined.
    const stored=(next[field]||'').trim()===(info[field]||'').trim()
    const examined=(stored?(next[field]||'').trim():'')||(!override?next[defaultKey]:'')||''
    const verdict={git:'Git repository',folder:'Folder, not a Git repository',missing:'Folder not found'}[stored?next[kindKey]:'']||''
    kindStatus.textContent=verdict&&examined?verdict+' · '+examined:''
    kindStatus.title=kindStatus.textContent
    kindStatus.dataset.kind=stored?next[kindKey]||'':''
   }
   render(info)
   return {input,row,render,field,showOffer:()=>offer.show(!picker.hidden)}
  }
  const macroSpec=specFolderRow('Macro','specPath','Macro skills read and write specifications')
  const issueSpec=specFolderRow('Issue','issueSpecPath','Issue skills write clarifications and specifications')
  const specRows=[macroSpec,issueSpec]
  // Each row re-renders from fresh agent data with its own typed value.
  const renderSpecs=(fresh,typed)=>{for(const spec of specRows)spec.render(typed?{...fresh,[spec.field]:spec.input.value}:fresh)}
  pathOffer.examine();for(const spec of specRows)spec.showOffer()
  // The other repositories the project declares, each in a folder of this
  // workstation (#456), on every project (#484). The project's own repository
  // is the local repository above; the list itself is a project setting of
  // the web interface.
  const repositoryList=document.createElement('div');repositoryList.className='repository-list'
  const repositoriesRow=settingRow('Other repositories',{stacked:true},repositoryList)
  repositoriesRow.hint.textContent='Tasks pinned to one of these repositories run in a worktree of its folder; the others are given to the agent as read-only context.'
  repositoriesRow.section.hidden=true
  const repositoryInputs=[]
  function loadRepositories(){
   return api.repositories(id).then(list=>{
    repositoryList.replaceChildren();repositoryInputs.length=0
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
  loadRepositories()
  // Folders attached to the project on this workstation only (#484): every
  // execution of the project is given them as context. Adding and removing
  // apply at once, like a repository folder chosen from the agent; a checkout
  // of one of the project's repositories becomes that repository's folder.
  const folderList=document.createElement('ul');folderList.className='attached-folders';folderList.setAttribute('aria-label','Attached folders')
  const addFolder=document.createElement('button');addFolder.type='button';addFolder.textContent='Add folder…'
  const folderStatus=document.createElement('p');folderStatus.className='attached-folder-status';folderStatus.setAttribute('role','status')
  const foldersRow=settingRow('Attached folders',{stacked:true},folderList,addFolder,folderStatus)
  foldersRow.hint.textContent='Other folders of this workstation handed to every execution of this project. A Git repository with a remote is changed through a worktree and its own pull request; a folder without a remote is changed in place.'
  const folderKind=folder=>folder.duplicate?'Duplicate of the folder of '+folder.duplicate
   :folder.kind==='git'?(folder.remote?'Git repository · '+folder.remote:'Git repository, no remote')
   :folder.kind==='folder'?'Folder, not a Git repository':'Folder not found'
  function renderFolders(list){
   folderList.replaceChildren()
   if(!list.length){const empty=document.createElement('li');empty.className='attached-folder-empty';empty.textContent='No attached folder';folderList.append(empty)}
   for(const folder of list){
    const item=document.createElement('li');item.className='attached-folder';item.dataset.kind=folder.duplicate?'duplicate':folder.kind
    const where=document.createElement('span');where.className='attached-folder-path';where.textContent=folder.path;where.title=folder.path
    const kind=document.createElement('small');kind.className='attached-folder-kind';kind.textContent=folderKind(folder)
    const remove=document.createElement('button');remove.type='button';remove.textContent='Remove';remove.setAttribute('aria-label','Remove '+folder.path)
    remove.onclick=async()=>{
     remove.disabled=true
     try{await api.detachFolder(id,folder.path);folderStatus.textContent='Removed '+folder.path;await loadFolders()}
     catch(err){folderStatus.textContent=ipcMessage(err);remove.disabled=false}
    }
    const text=document.createElement('div');text.append(where,kind)
    item.append(text,remove);folderList.append(item)
   }
  }
  function loadFolders(){return api.folders(id).then(renderFolders).catch(err=>{folderStatus.textContent=ipcMessage(err)})}
  addFolder.onclick=async()=>{
   try{
    const selected=await api.chooseRepository();if(!selected)return
    addFolder.disabled=true
    const answer=await api.attachFolder(id,selected)
    if(answer?.mappedAs){folderStatus.textContent=selected+' is a checkout of '+answer.mappedAs+': it is now that repository\'s folder.';await loadRepositories()}
    else folderStatus.textContent='Attached '+selected
    await loadFolders()
   }catch(err){folderStatus.textContent=ipcMessage(err)}finally{addFolder.disabled=false}
  }
  loadFolders()
  // Any repository (#737): tickets may change a repository the project does
  // not list, at a checkout the session names or in a clone the agent makes in
  // the clones folder. Off by default.
  const anyRepository=document.createElement('input');anyRepository.type='checkbox';anyRepository.id='any-repository';anyRepository.checked=!!info.anyRepository
  const anyRepositoryLabel=document.createElement('label');anyRepositoryLabel.htmlFor=anyRepository.id
  anyRepositoryLabel.append(anyRepository,document.createTextNode(' Let tasks change any repository'))
  const clonesPath=document.createElement('input');clonesPath.value=info.clonesPath||'';clonesPath.setAttribute('aria-label','Clones folder')
  clonesPath.placeholder=info.clonesDefault||'Folder the agent clones repositories into'
  const chooseClones=document.createElement('button');chooseClones.type='button';chooseClones.textContent='Choose folder…';chooseClones.setAttribute('aria-label','Choose the clones folder…')
  chooseClones.onclick=async()=>{try{const selected=await api.chooseRepository();if(selected)clonesPath.value=selected}catch(err){error(err)}}
  const clonesPicker=document.createElement('div');clonesPicker.className='repository-picker';clonesPicker.append(clonesPath,chooseClones)
  const anyRepositoryRow=settingRow('Any repository',{stacked:true},anyRepositoryLabel,clonesPicker)
  const renderAnyRepository=()=>{
   clonesPicker.hidden=!anyRepository.checked
   anyRepositoryRow.hint.textContent=anyRepository.checked
    ?'A repository no folder here holds is used at the checkout the session finds, or cloned into '+(clonesPath.value.trim()||info.clonesDefault||'the clones folder')+'. Either is remembered on this workstation.'
    :'Tasks change only the repositories listed above and the attached folders.'
  }
  anyRepository.onchange=renderAnyRepository;clonesPath.oninput=renderAnyRepository;renderAnyRepository()
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
  let parallelism=Number(fields.parallelism.value)||DEFAULT_PARALLELISM,inheritParallelism=inherits('parallelism')
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
  const resetSetup=()=>{setupProviders=[...(fields.setupProviders.inherited||[])];inheritSetupProviders=true}

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
    inheritTerminal=inherits('terminal')
   }
   if(inheritWorktrees)resetWorktrees()
   if(inheritParallelism)resetParallelism()
   if(inheritSetupProviders)resetSetup()
   if(inheritTerminal)resetTerminal()
   fillEngines()
   update();updateEngine();updateTerminal()
  }

  const notice=document.createElement('p');notice.setAttribute('role','status')
  panels.General.append(repository.section,macroSpec.row.section,issueSpec.row.section,repositoriesRow.section,foldersRow.section,anyRepositoryRow.section)
  panels.Execution.append(controls.worktrees.section,controls.specArtifacts.section,controls.parallel.section,terminalRow.section)
  // What the project's Claude Code sessions are allowed (#700). An agent that
  // predates it sends no values, and the save sends none back.
  // The workstation values it inherits come with it (#730); an agent that
  // predates them sends none, and the project then shows only its own.
  const sandbox=sandboxSettings({settingRow,stored:info.claudeSandbox,platformSandbox:info.platformSandbox!==false,settingsPath:info.claudeSettingsPath,project:info,
   inherited:info.claudeSandboxGlobal||null,covered:info.claudeSandboxCovered!==false,
   onPromote:'claudeSandboxCovered' in info?rule=>api.promoteSandboxRule(id,rule):null})
  panels.Sandbox.append(...sandbox.sections)
  panels.Remove.append(engineRow.section)
  const remove=document.createElement('button');remove.type='button';remove.textContent='Remove from desktop';remove.className='remove-project'
  remove.onclick=()=>requestRemoveProject(id,config.projectName)
  const removalNote=document.createElement('p');removalNote.textContent='Remove this project from this workstation. The project remains on the Sectile server.'
  panels.Remove.append(removalNote)
  if(info.configured||runs.some(run=>run.projectId===id))panels.Remove.append(remove)
  if(sidebarHiddenProjects.has(id)){
   const show=document.createElement('button');show.type='button';show.textContent='Show in sidebar'
   show.onclick=()=>{setSidebarHidden(id,false);show.remove();hiddenRow.section.remove()}
   const hiddenRow=settingRow('Sidebar',null,show)
   hiddenRow.hint.textContent='This project is hidden from the sidebar. Its configuration and executions are kept.'
   panels.Remove.prepend(hiddenRow.section)
  }
  content.append(notice)
  form.onsubmit=async event=>{
   event.preventDefault()
   save.disabled=true
   try{
    await api.mapProject({projectId:id,path:path.value,specPath:macroSpec.input.value.trim(),issueSpecPath:issueSpec.input.value.trim(),
     anyRepository:anyRepository.checked,clonesPath:clonesPath.value.trim(),
     useWorktrees,inheritWorktrees,specArtifacts,inheritSpecArtifacts,parallelism,inheritParallelism,
     ...(engineView?{defaultEngine,inheritDefaultEngine}:{}),
     terminal:terminal.get(),inheritTerminal,
     setupProviders:[...setupProviders],inheritSetupProviders,
     ...(info.claudeSandbox?{claudeSandbox:sandbox.payload(),claudeSandboxBase:sandbox.base()}:{})})
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
    try{const fresh=await api.project(id);for(const spec of specRows){info[spec.field]=fresh[spec.field]||'';spec.input.value=info[spec.field]};renderSpecs(fresh);applyFields(fresh);if(fresh.claudeSandbox)sandbox.set(fresh.claudeSandbox,fresh.claudeSandboxGlobal||null,fresh.claudeSandboxCovered!==false)}catch(err){error(err)}
    await loadProjects()
   }catch(err){notice.textContent=err.message}finally{save.disabled=false}
  }
  const metadata=document.createElement('div');panels.Remove.prepend(metadata)
  function renderServer(){
   metadata.replaceChildren()
   for(const [label,value] of [['Git remote',config.gitRemoteUrl||'Not configured'],['SDD framework',config.specFramework||'Not configured']]){
    const row=readOnlyRow(label,'Managed on the Sectile server')
    row.value.textContent=value;metadata.append(row.section)
   }
  }
  renderServer()
  const reload=document.createElement('button');reload.type='button';reload.textContent='Refresh from server';reload.className='dialog-action refresh-project'
  dialogFooter.append(reload);syncDialogFooter()
  reload.onclick=async()=>{
   reload.disabled=true;notice.textContent='Refreshing server settings…'
   try{
    const fresh=await api.project(id)
    if(!reload.isConnected)return
    config=fresh.server
    if(!configurationActive()||generation!==configurationGeneration)return
    if(inheritSpecArtifacts)specArtifacts=config.specArtifacts==='drop'?'drop':'keep'
    applyFields(fresh,true);renderServer();renderSpecs(fresh,true)
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
  const [link,...others]=repositoryPullRequests(task);renderPullRequestIndicator(pr,link,prLabel(link.url)+' for '+key)
  pr.onclick=()=>api.openPR(link.url).catch(error);prCell.append(pr)
  if(others.length)prCell.append(otherPullRequestsBadge(others))
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
  items.push({label:'Skip to Handoff…',transition:'reviewed'})
 }
 items.push({label:'Discussion (no skill)',skillId:'discuss'},{label:'Discussion in native terminal',nativeTerminal:true},{label:'Custom instructions…',compose:true},{label:'Launch…',dialog:true})
 for(const item of items){
  const button=document.createElement('button');button.type='button';button.setAttribute('role','menuitem');button.textContent=item.label;button.disabled=!view.info.configured
  if(item.skillId)button.dataset.skillId=item.skillId
  if(item.transition==='reviewed'){
   entry.declareReviewed=button
   button.onclick=()=>{closeMenu();confirmDeclareReviewed(view.projectID,task)}
  }else{
   button.onclick=()=>{closeMenu();if(item.dialog)openTicketLaunchDialog(view,task);else if(item.compose)openCompose(view,entry);else if(item.nativeTerminal)submitNativeDiscussion(view,entry).catch(()=>{});else submitTicketLaunch(view,entry,item.skillId,'',item.skillId==='pickup'?'autonomous':'').catch(()=>{})}
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
// A row's "Launch…" opens the Launch dialog on the task's next workflow step,
// or on a discussion when the workflow offers none.
function openTicketLaunchDialog(view,task){
 const step=nextTaskStep(task,view.info)
 openLaunchDialog({projectID:view.projectID,taskId:task.id,taskKey:task.key,skill:step.skillId||'discuss',prompt:''})
}
async function submitTicketLaunch(view,entry,skillId,prompt,mode){
 const key=entry.task.key||entry.task.id
 view.submitting.add(entry.task.id);updateTicketRow(view,entry)
 view.status.textContent='Submitting execution for '+key+'…'
 try{
  // An interactive launch follows the AI consoles preference; the agent
  // falls back to the terminal for an engine that cannot hold a conversation.
  await api.launchServerTask(view.projectID,entry.task.id,skillId,prompt,mode,false,consoleView)
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

document.querySelector('#rerun').onclick=()=>{
 const run=runs.find(item=>item.id===selected)
 if(!run||macroRun(run))return
 if(freeConsole(run)){openAgentConsole(run.projectId,run.engineId||run.provider);return}
 openLaunchDialog({projectID:run.projectId,taskId:run.taskId,taskKey:run.taskKey,skill:run.skill,prompt:run.prompt})
}
// The Launch dialog (#786) starts a new execution of a task with a chosen
// skill, instructions, mode and engine. The toolbar opens it on the selected
// execution, a ticket row on its next workflow step. The engine sticks to the
// task, as the Engine column's choice does: it is stored before the launch,
// and a refusal launches nothing.
async function openLaunchDialog({projectID,taskId,taskKey,skill:initialSkill,prompt:initialPrompt}){
 showDialog('Launch '+(taskKey||taskId))
 try{
  const info=await api.project(projectID)
  let engines=null
  if(await taskEnginesAvailable()){try{engines=await api.taskEngines(projectID)}catch{}}
  const form=document.createElement('form')
  const skillLabel=document.createElement('label');skillLabel.textContent='Skill'
  const skill=document.createElement('select');skill.setAttribute('aria-label','Launch skill')
  for(const item of info.server.skills||[]){
   const option=document.createElement('option');option.value=item.id;option.textContent=item.command||item.id;skill.append(option)
  }
  const discuss=document.createElement('option');discuss.value='discuss';discuss.textContent='Discussion (no skill)';skill.append(discuss)
  const custom=document.createElement('option');custom.value='custom';custom.textContent='Custom instructions';skill.append(custom)
  skill.value=initialSkill
  if(!skill.value){
   const missing=document.createElement('option');missing.value='';missing.textContent='Select a skill (previous skill unavailable)';missing.disabled=true;skill.prepend(missing);skill.value=''
  }
  skill.required=true;skillLabel.append(skill)
  const promptLabel=document.createElement('label');promptLabel.textContent='Instructions'
  const prompt=document.createElement('textarea');prompt.className='cli-command';prompt.setAttribute('aria-label','Launch instructions');prompt.value=initialPrompt||''
  promptLabel.append(prompt)
  const modeLabel=document.createElement('label');modeLabel.textContent='Execution mode'
  const mode=modeSelect(document,'Launch execution mode');modeLabel.append(mode)
  const fields=[skillLabel,promptLabel,modeLabel]
  let engine=null
  if(engines?.catalogue?.length){
   const engineLabel=document.createElement('label');engineLabel.textContent='AI engine'
   engine=document.createElement('select');engine.setAttribute('aria-label','Launch AI engine')
   for(const item of engines.catalogue){
    const option=document.createElement('option');option.value=item.id
    option.textContent=engineTooltip(item,item.id===engines.projectDefault);engine.append(option)
   }
   engine.value=taskEngine(engines,taskId)?.id||''
   engineLabel.append(engine);fields.push(engineLabel)
  }
  const submit=document.createElement('button');submit.textContent='Launch';submit.disabled=!info.configured
  const notice=document.createElement('p');notice.setAttribute('role','status')
  if(!info.configured)notice.textContent='Configure a local repository before launching.'
  form.append(...fields,submit,notice);dialogBody.append(form)
  form.onsubmit=async event=>{
   event.preventDefault()
   if(skill.value==='custom'&&!prompt.value.trim()){notice.textContent='Enter custom instructions.';prompt.focus();return}
   submit.disabled=true
   const change=engine?launchEngineChange(engines,taskId,engine.value):null
   if(change){
    try{
     const stored=await api.setTaskEngine(projectID,taskId,change)
     // A launch refused next is retried against what is now stored.
     if(stored)engines=stored
     if(ticketsView?.projectID===projectID&&ticketsView.engines&&stored){
      ticketsView.engines.tasks=stored.tasks||{}
      for(const row of ticketsView.rows.values())renderEngineToggle(ticketsView,row)
     }
    }catch(err){notice.textContent='Could not switch the engine: '+ipcMessage(err);submit.disabled=false;return}
   }
   try{
    await api.launchServerTask(projectID,taskId,skill.value,prompt.value,launchModeOverride(mode.value),false,consoleView)
    dialog.close();await refresh()
   }catch(err){notice.textContent=err.message;submit.disabled=false}
  }
 }catch(err){paragraph(err.message)}
}

// The last segment of a repository identity, enough to tell two of a task's
// repositories apart in the toolbar.
function repositoryName(repository){return String(repository||'').split('/').pop()||'PR / MR'}
// A task's other repositories are counted where a single indicator fits: the
// toolbar of the selected task lists them.
function otherPullRequestsBadge(others){
 const badge=document.createElement('span');badge.className='pr-others';badge.textContent='+'+others.length
 badge.title=others.map(other=>other.repository+': '+prLabel(other.url)+' ('+pullRequestPresentation(other).label+')').join('\n')
 return badge
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
     const stage=task&&taskStage(task)
     if(STAGES.includes(stage))taskStages.set(run.taskId,stage)
     else taskStages.delete(run.taskId)
     if(task?.prUrl&&/^https?:\/\//i.test(task.prUrl))pullRequests.set(run.taskId,repositoryPullRequests(task))
     else pullRequests.delete(run.taskId)
    }
   }catch{}
  }))
  render()
 }finally{linksLoading=false}
}

// archiveDialog archives the task, after stopping its active executions when
// stop says the user asked for it. A refusal keeps the dialog open with its
// reason, one line per paragraph, and the button tries again: executions
// already stopped stay stopped.
function archiveDialog(run,title,intro,label,stop){
 showDialog(title)
 if(intro)paragraph(intro)
 const message=document.createElement('div');message.className='archive-refusal';message.setAttribute('role','alert')
 const confirm=document.createElement('button');confirm.textContent=label;dialogBody.append(message,confirm)
 const refuse=err=>message.replaceChildren(...startFailure(err).split('\n').map(line=>{const p=document.createElement('p');p.textContent=line;return p}))
 confirm.onclick=async()=>{
  confirm.disabled=true;message.replaceChildren()
  try{
   if(stop)await Promise.all(runs.filter(item=>taskKey(item)===taskKey(run)&&activeRun(item)).map(item=>api.stop(item.id)))
   await archiveTask(run)
  }catch(err){refuse(err);confirm.disabled=false}
 }
 return refuse
}
function requestArchive(run){
 const key=taskKey(run)
 if(runs.some(item=>taskKey(item)===key&&activeRun(item))){
  archiveDialog(run,'Stop and archive task?','This task has active executions. They must stop before it can be archived'+(archivesWorktree(run)?' and its worktree removed.':' locally.'),'Stop and archive',true)
  render()
  return
 }
 if(archiving.has(key))return
 archiving.add(key)
 archiveTask(run).catch(err=>archiveDialog(run,'Task not archived','','Archive again',false)(err)).finally(()=>{archiving.delete(key);render()})
}
async function archiveTask(run){
 const latest=await api.runs()
 const related=latest.filter(item=>taskKey(item)===taskKey(run))
 if(related.some(activeRun))throw Error('An execution is still active. Stop it before archiving.')
 // The worktrees go before the runs are hidden: a task whose worktree stays is not archived (#755).
 if(archivesWorktree(run)){
  let result
  try{result=await api.archiveWorkspace(run.projectId,run.taskId)}catch(err){throw Error(archiveFailure(err))}
  const refusal=archiveRefusal(result)
  if(refusal)throw Error(refusal)
 }
 localTasks[taskKey(run)]={...taskState(run),archivedRuns:related.map(item=>item.id)}
 saveLocalTasks()
 const current=runs.find(item=>item.id===selected)
 if(current&&taskKey(current)===taskKey(run)){
  conversation.select(null)
  selected=null;terminal.reset();await api.detach()
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

// The palette searches, in one list, the desktop's actions, the projects and
// the executions: a few keys reach anything the sidebar shows. Actions are a
// list rather than hardcoded rows, so adding one is a line.
function paletteEntries(){
 const mac=isMacPlatform(navigator)
 const entries=[
  {group:'action',label:'New task',detail:'Create a task in a project',hint:newTaskShortcutLabel(mac),run:()=>quickAdd()},
  {group:'action',label:'Tasks list',detail:'Browse the open tasks of a project',run:()=>openTicketsFromPalette()},
  {group:'action',label:'Add project',detail:'Connect a project of the server to this workstation',run:()=>openAddProject()},
  {group:'action',label:(document.querySelector('#workspace').classList.contains('sidebar-hidden')?'Show':'Hide')+' the sidebar',hint:sidebarShortcutLabel(mac),run:()=>{dialog.close();toggleSidebar()}},
  {group:'action',label:'Settings',hint:configShortcutLabel(mac),run:()=>{dialog.close();openSettings('Profile')}},
  {group:'action',label:'Open the web interface',run:()=>{dialog.close();api.openBoard().catch(error)}},
 ]
 for(const project of projects.filter(addedProject))entries.push({group:'project',label:project.name,detail:'Open its tasks',run:()=>{dialog.close();openTickets(project.id)}})
 // The executions the sidebar shows, the latest first.
 const shown=runs.filter(run=>!hiddenRun(run)&&!sidebarHiddenProjects.has(run.projectId))
 for(const run of shown.slice().reverse()){
  const key=run.taskKey||'',name=taskTitles.get(run.taskId)||taskState(run).name||''
  const label=[key,name||runLabel(run)].filter(Boolean).join(' · ')
  const project=projects.find(item=>item.id===run.projectId)?.name||run.projectId
  entries.push({group:'execution',label,detail:[project,run.skill,runStateLabel(runStateOf(run))].filter(Boolean).join(' · '),run:()=>{dialog.close();select(run)}})
 }
 return entries
}
// The tickets list needs a project. The selected one answers that, and a single
// configured project answers it too; otherwise the palette asks rather than
// guessing which project the user meant.
async function openTicketsFromPalette(){
 if(!projects.length){
  try{await loadProjects()}catch(err){showDialog('Tasks list');paragraph(err.message);return}
 }
 const known=projects.filter(project=>!hiddenProject(project.id))
 const chosen=known.find(project=>project.id===selectedProject)||(known.length===1?known[0]:null)
 if(chosen){dialog.close();openTickets(chosen.id);return}
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
const PALETTE_GROUPS={action:'Actions',project:'Projects',execution:'Executions'}
function openCommandPalette(){
 showDialog('Commands');dialog.classList.add('palette-dialog')
 dialogBody.querySelector('h2').className='visually-hidden'
 const entries=paletteEntries()
 const filter=document.createElement('input');filter.className='palette-filter';filter.placeholder='Search actions, projects and tasks…'
 filter.setAttribute('role','combobox');filter.setAttribute('aria-label','Search commands');filter.setAttribute('aria-controls','palette-list');filter.setAttribute('aria-expanded','true');filter.setAttribute('aria-autocomplete','list')
 const list=document.createElement('ul');list.id='palette-list';list.className='palette-list';list.setAttribute('role','listbox');list.setAttribute('aria-label','Commands')
 const empty=document.createElement('p');empty.className='palette-empty';empty.textContent='Nothing matches'
 let shown=[],active=0
 const mark=()=>{
  for(const [index,item] of [...list.querySelectorAll('[role=option]')].entries())item.setAttribute('aria-selected',String(index===active))
  const current=list.querySelectorAll('[role=option]')[active]
  if(current){filter.setAttribute('aria-activedescendant',current.id);current.scrollIntoView({block:'nearest'})}else filter.removeAttribute('aria-activedescendant')
 }
 const draw=()=>{
  shown=paletteMatches(entries,filter.value);active=Math.min(active,Math.max(0,shown.length-1))
  list.replaceChildren();empty.hidden=shown.length>0
  let group=null
  for(const [index,entry] of shown.entries()){
   if(entry.group!==group){group=entry.group;const heading=document.createElement('li');heading.className='palette-group';heading.setAttribute('role','presentation');heading.textContent=PALETTE_GROUPS[group];list.append(heading)}
   const item=document.createElement('li');item.id='palette-option-'+index;item.className='palette-option';item.setAttribute('role','option')
   const label=document.createElement('span');label.className='palette-label';label.textContent=entry.label;item.append(label)
   if(entry.detail){const detail=document.createElement('span');detail.className='palette-detail';detail.textContent=entry.detail;item.append(detail)}
   if(entry.hint){const hint=document.createElement('kbd');hint.className='palette-hint';hint.textContent=entry.hint;item.append(hint)}
   item.addEventListener('mousemove',()=>{if(active!==index){active=index;mark()}})
   item.addEventListener('click',()=>entry.run())
   list.append(item)
  }
  mark()
 }
 filter.oninput=()=>{active=0;draw()}
 filter.onkeydown=event=>{
  if(event.key==='ArrowDown'||event.key==='ArrowUp'){event.preventDefault();if(shown.length){active=(active+(event.key==='ArrowDown'?1:shown.length-1))%shown.length;mark()}}
  else if(event.key==='Enter'){const entry=shown[active];if(entry){event.preventDefault();entry.run()}}
 }
 dialogBody.append(filter,list,empty);draw();filter.focus()
}
document.querySelector('#command-palette').onclick=openCommandPalette
window.addEventListener('keydown',event=>{
 const action=configShortcutAction({key:event.key,metaKey:event.metaKey,ctrlKey:event.ctrlKey,shiftKey:event.shiftKey,altKey:event.altKey,repeat:event.repeat,defaultPrevented:event.defaultPrevented,mac:isMacPlatform(navigator),modalOpen:dialog.open,configurationOpen:configurationActive()})
 if(action==='ignore')return
 event.preventDefault();event.stopPropagation()
 if(action==='open')openSettings('Profile')
},true)
window.addEventListener('keydown',event=>{
 if((event.metaKey||event.ctrlKey)&&event.key.toLowerCase()==='k'){event.preventDefault();event.stopPropagation();openCommandPalette()}
},true)
// Cmd+Enter / Ctrl+Enter activates the default button of the open dialog or
// settings page: the submit of the form being typed in, else a marked button.
window.addEventListener('keydown',event=>{
 if(!defaultActionShortcut({key:event.key,metaKey:event.metaKey,ctrlKey:event.ctrlKey,shiftKey:event.shiftKey,altKey:event.altKey,repeat:event.repeat,isComposing:event.isComposing,mac:isMacPlatform(navigator)}))return
 const root=dialog.open?dialog:configurationActive()?dialogBody.parentElement:null
 const target=root&&defaultActionTarget(root,document.activeElement)
 if(!target)return
 event.preventDefault();event.stopPropagation()
 if(target.form)target.form.requestSubmit(target.button);else target.button.click()
},true)
// Cmd+N / Ctrl+N opens the new task dialog; a focused terminal keeps Ctrl+N.
window.addEventListener('keydown',event=>{
 const action=newTaskShortcutAction({key:event.key,metaKey:event.metaKey,ctrlKey:event.ctrlKey,shiftKey:event.shiftKey,altKey:event.altKey,repeat:event.repeat,defaultPrevented:event.defaultPrevented,mac:isMacPlatform(navigator),inTerminal:!!document.activeElement?.closest?.('.xterm'),modalOpen:dialog.open})
 if(action==='open'&&agentConnected){event.preventDefault();event.stopPropagation();quickAdd()}
},true)
// Cmd+B / Ctrl+B toggles the projects sidebar (#474). In the capture phase, so
// that on macOS Cmd+B never reaches the terminal; an ignored key is left
// untouched, which is how Ctrl+B still reaches a focused terminal elsewhere.
window.addEventListener('keydown',event=>{
 const action=sidebarShortcutAction({key:event.key,metaKey:event.metaKey,ctrlKey:event.ctrlKey,shiftKey:event.shiftKey,altKey:event.altKey,repeat:event.repeat,defaultPrevented:event.defaultPrevented,mac:isMacPlatform(navigator),inTerminal:!!document.activeElement?.closest?.('.xterm'),modalOpen:dialog.open})
 if(action==='toggle'){event.preventDefault();event.stopPropagation();toggleSidebar()}
},true)
// The project quick add opens on: the selected one, else the one last used.
const QUICK_ADD_PROJECT='quickAddProject'
async function quickAdd(projectID){
 showDialog('New task');dialog.classList.add('quick-add-dialog')
 try{await loadProjects()}catch(err){paragraph(err.message);return}
 let remembered='';try{remembered=localStorage.getItem(QUICK_ADD_PROJECT)||''}catch{}
 const known=projects.filter(addedProject)
 const choice=[projectID,selectedProject,remembered].find(id=>id&&known.some(project=>project.id===id))||(known.length===1?known[0].id:'')
 const mac=isMacPlatform(navigator)
 const form=document.createElement('form');form.className='quick-add'
 const field=(text,control,hint)=>{
  const label=document.createElement('label');label.className='quick-add-field'
  const name=document.createElement('span');name.className='quick-add-name';name.textContent=text;label.append(name)
  if(hint){const small=document.createElement('small');small.textContent=hint;name.append(small)}
  label.append(control);return label
 }
 const project=document.createElement('select')
 if(!choice){const none=document.createElement('option');none.value='';none.textContent='Select a project';project.append(none)}
 for(const item of known){const option=document.createElement('option');option.value=item.id;option.textContent=item.name;project.append(option)}
 project.value=choice;project.required=true
 const title=document.createElement('input');title.placeholder='What needs doing?';title.required=true;title.maxLength=500
 const description=document.createElement('textarea');description.rows=4;description.placeholder='Context, acceptance criteria, links…'
 const grow=()=>{description.style.height='auto';description.style.height=Math.min(description.scrollHeight,320)+'px'}
 description.addEventListener('input',grow)
 const notice=document.createElement('p');notice.className='quick-add-notice';notice.setAttribute('role','status')
 const actions=document.createElement('div');actions.className='quick-add-actions'
 const keys=document.createElement('span');keys.className='quick-add-keys';keys.textContent=defaultActionLabel(mac)+' to create'
 const cancel=document.createElement('button');cancel.type='button';cancel.className='secondary';cancel.textContent='Cancel';cancel.onclick=()=>dialog.close()
 const submit=document.createElement('button');submit.type='submit';submit.textContent='Create task'
 actions.append(keys,cancel,submit)
 form.append(field('Project',project),field('Title',title),field('Description',description,'Optional, Markdown'),notice,actions)
 dialogBody.append(form);(choice?title:project).focus()
 form.onsubmit=async event=>{
  event.preventDefault();if(!title.value.trim()||!project.value)return
  submit.disabled=cancel.disabled=true
  notice.textContent='Creating the task on the server and its tracker…'
  try{
   const task=await api.createTask({projectID:project.value,title:title.value.trim(),description:description.value})
   selectedProject=project.value
   try{localStorage.setItem(QUICK_ADD_PROJECT,project.value)}catch{}
   created(task,project.value)
  }catch(err){notice.textContent=err.message;submit.disabled=cancel.disabled=false}
 }
 // What follows a creation: clarify it at once, pick another launch, or add
 // the next one in the same project.
 function created(task,projectId){
  form.replaceChildren()
  const done=document.createElement('p');done.className='quick-add-created'
  done.textContent='Created '+(task.key||task.id)+' · '+task.title
  const next=document.createElement('div');next.className='quick-add-actions'
  const status=document.createElement('p');status.className='quick-add-notice';status.setAttribute('role','status')
  const clarify=document.createElement('button');clarify.type='button';clarify.textContent='Clarify now';clarify.dataset.defaultAction=''
  clarify.onclick=async()=>{
   clarify.disabled=true;status.textContent='Launching clarify…'
   try{await api.launchServerTask(projectId,task.id,'clarify','','',false,consoleView);dialog.close();await refresh()}
   catch(err){status.textContent=err.message;clarify.disabled=false}
  }
  const launch=document.createElement('button');launch.type='button';launch.className='secondary';launch.textContent='Launch task'
  launch.onclick=()=>{dialog.close();openTickets(projectId,task.key||task.title)}
  const another=document.createElement('button');another.type='button';another.className='secondary';another.textContent='Add another'
  another.onclick=()=>quickAdd(projectId)
  const close=document.createElement('button');close.type='button';close.className='secondary';close.textContent='Done'
  close.onclick=()=>dialog.close()
  next.append(another,launch,close,clarify)
  form.append(done,status,next);clarify.focus()
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
 if(busy||pending){button.hidden=false;button.disabled=true;setStepLabel(('Current: '+(skillLabel(current)||step.label||'')).trim());label.hidden=true}
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
  await api.launchServerTask(run.projectId,run.taskId,launchSkill,'',kind==='pickup'?'autonomous':undefined,force,consoleView)
  submittedSteps.set(key,{skillId:launchSkill,kind,runIds:latestRuns.filter(item=>taskKey(item)===key).map(item=>item.id)})
  await refresh()
  if(taskKey(currentTaskRun()||{})===key){
   const launched=runs.find(item=>taskKey(item)===key&&!latestRuns.some(previous=>previous.id===item.id))
   if(launched&&launched.id!==selected)select(launched)
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

// Open terminal (#761): a native window on the project's local repository, in
// the terminal the project is set to use. The agent resolves the folder.
async function openProjectTerminal(projectID){
 try{await api.projectTerminal(projectID);document.querySelector('#error').textContent=''}
 catch(err){error(Error(ipcMessage(err).trim()))}
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
   const run=await api.launchConsole(projectID,catalogue?undefined:provider.value,catalogue?provider.value:undefined,consoleView)
   collapsedProjects.delete(projectID);queueProjects.delete(projectID)
   if(!runs.some(item=>item.id===run.id))runs.push(run)
   dialog.close();select(run);await refresh()
  }catch(err){notice.textContent=err.message;launch.disabled=false}
 }
}
