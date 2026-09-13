import { orderedQueueRuns } from './queue.mjs'
import { orderedTaskGroups } from './task-order.mjs'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import './style.css'
import { taskStage, nextTaskStep } from './workflow.mjs'
const api=window.localAgent
document.querySelector('#app').innerHTML=`
<header><div><button id="toggle-sidebar" aria-label="Toggle projects" aria-expanded="true">☰</button><span class="brand">S</span><strong>Sectile Local</strong><small>Execution consoles</small></div><span id="connection">Connecting…</span><button id="command-palette" title="Commands (⌘K / Ctrl+K)">⌘K</button><nav aria-label="Local agent controls"><button id="configure" class="icon-button" aria-label="Local agent" title="Agent connection settings"></button><button id="start-agent" class="icon-button" aria-label="Start agent" title="Start agent"></button><button id="shutdown" class="icon-button" aria-label="Stop agent" title="Stop agent" hidden></button><button id="restart" class="icon-button" aria-label="Restart agent" title="Restart agent" hidden></button><button id="profile" class="icon-button" aria-label="Profile" title="Profile"></button></nav></header>
<section id="setup" hidden><div id="agent-offline" role="status" hidden><strong>Local agent is stopped</strong><p>Start the agent to run tasks and access your local consoles.</p></div><h1>Connect to TaskFlow</h1><p>Enter your server address and authentication token. Account sign-in is not available yet.</p>
<form id="start"><label>TaskFlow server<input name="server" type="url" value="http://localhost:8090" required></label><label>Server token<input name="token" type="password" required autocomplete="off"></label><button>Connect</button></form></section>
<main id="workspace" hidden><aside><section id="execution-queue" aria-label="Execution queue"><h2>Execution queue</h2><p id="queue-summary" role="status" aria-live="polite"></p><details id="queue-details" open><summary>View executions</summary><div id="queue-list"></div></details></section><div class="section">PROJECTS <button id="add-project" title="Add a remote project">+</button></div><div id="runs"></div><button id="clear-history" disabled>Clear finished consoles</button><p class="hint">Launch a task from TaskFlow web. Its console appears here.</p></aside><div id="sidebar-resizer" role="separator" aria-label="Resize sidebar" aria-orientation="vertical" tabindex="0"></div><article><div id="toolbar"><div><strong id="title">Select an execution</strong><small id="directory"></small></div><select id="execution-history" aria-label="Execution history" hidden></select><button id="selected-pr" hidden></button><button id="rerun" hidden>Relaunch</button><button id="save-log">Export log</button><button id="stop" class="icon-button" type="button" aria-label="Stop execution" title="Stop execution" disabled></button></div><div id="terminal"></div><footer id="task-status"><span id="next-step-status" role="status" aria-live="polite">Select a task to see its next step</span><button id="next-step" type="button" hidden disabled></button><button id="retry-next-step" type="button" hidden>Retry</button></footer></article></main>
<dialog id="project-dialog"><button id="close-dialog" aria-label="Close">×</button><div id="dialog-body"></div><div class="dialog-footer"><button id="dismiss-dialog">Close settings</button></div></dialog><div id="error" role="alert"></div>`
const terminal=new Terminal({cursorBlink:true,fontSize:13,fontFamily:'Menlo, monospace',scrollback:20000,theme:{background:'#11151c',foreground:'#d8e0ec'}})
const fit=new FitAddon();terminal.loadAddon(fit)
let nextStepData=null,nextStepGeneration=0,nextStepUpdated=0
const submittingSteps=new Set()
const submittedSteps=new Map()
const nextStepErrors=new Map()
const taskTitles=new Map()
const pullRequests=new Map()
let localTasks={}
try{localTasks=JSON.parse(localStorage.getItem('localTasks')||'{}')}catch{}
const taskKey=run=>JSON.stringify([run.projectId,run.taskId])
const activeRun=run=>['running','queued','preparing'].includes(run.status)
const taskState=run=>localTasks[taskKey(run)]||{}
let disconnectedProjects=new Set(),projectStateVersion=0,refreshing=false
const hiddenProject=id=>disconnectedProjects.has(id)
const hiddenRun=run=>hiddenProject(run.projectId)||(taskState(run).archivedRuns||[]).includes(run.id)&&!activeRun(run)
function saveLocalTasks(){localStorage.setItem('localTasks',JSON.stringify(localTasks))}
const collapsedProjects=new Set(JSON.parse(localStorage.getItem('collapsedProjects')||'[]'))

let linksLoading=false,lastLinksRefresh=0
let selectedProject=null
let opened=false,selected=null,runs=[],last='',stopping=false,restarting=false,projects=[],projectsLoaded=false
api.onOutput(data=>terminal.write(new Uint8Array(data)))
terminal.onData(data=>api.input(data))
function resize(){if(opened){fit.fit();api.resize(terminal.cols,terminal.rows)}}
window.addEventListener('resize',resize)
function error(err){document.querySelector('#error').textContent=err?.message||String(err)}
function agentUnavailable(){
 document.querySelector('#start-agent').disabled=false
 document.querySelector('#start button').disabled=false
 document.querySelector('#start button').textContent='Start local agent'
 document.querySelector('#shutdown').hidden=true
 document.querySelector('#restart').hidden=true
 document.querySelector('#agent-offline').hidden=false
 document.querySelector('#setup').hidden=false
 document.querySelector('#workspace').hidden=true
 document.querySelector('#connection').textContent='Local agent stopped'
 projectsLoaded=false
 api.detach().catch(()=>{})
}
function ready(){
 document.querySelector('#agent-offline').hidden=true
 if(!projectsLoaded){projectsLoaded=true;loadProjects().catch(()=>{projectsLoaded=false})}
 document.querySelector('#start-agent').disabled=true;document.querySelector('#start button').disabled=true;document.querySelector('#restart').hidden=false;document.querySelector('#shutdown').hidden=false
 document.querySelector('#setup').hidden=true;document.querySelector('#workspace').hidden=false
 document.querySelector('#connection').textContent='Local agent connected'
 if(!opened){terminal.open(document.querySelector('#terminal'));opened=true;resize()}
}
function select(run){
 if(hiddenProject(run.projectId))return
 selectedProject=run.projectId
 selected=run.id
 refreshNextStep()
 document.querySelector('#title').textContent=(taskState(run).name||run.taskKey||run.taskId)+' · '+run.skill
 document.querySelector('#directory').textContent=run.directory
 document.querySelector('#stop').disabled=!['running','queued','preparing'].includes(run.status)
 terminal.reset()
 if(run.status==='queued'||run.status==='preparing'||!run.sessionId){
  api.detach().catch(error)
  const message=run.status==='queued'?'Execution queued. Waiting for a console.':run.status==='preparing'?'Preparing execution. Waiting for a console.':run.status==='canceled'?'Execution canceled before a console was created.':'No console is available for this execution. Check the task activity and local agent.log for launch errors.'
  terminal.writeln(message)
  render();return
 }
 api.attach(run.id).then(()=>{setTimeout(resize,150);terminal.focus()}).catch(error)
 render()
}
function renderQueue(){
 const active=runs.filter(run=>['running','preparing'].includes(run.status)&&!run.cancelRequested)
 const stopping=runs.filter(run=>activeRun(run)&&run.cancelRequested)
 const waiting=orderedQueueRuns(runs)
 const summary=document.querySelector('#queue-summary')
 const text=active.length+' active · '+waiting.length+' waiting'+(stopping.length?' · '+stopping.length+' stopping':'')
 if(summary.textContent!==text)summary.textContent=text
 const list=document.querySelector('#queue-list');list.replaceChildren()
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
   const title=document.createElement('strong');title.textContent=(run.taskKey||run.taskId)+' · '+(taskState(run).name||taskTitles.get(run.taskId)||run.skill)
   const context=document.createElement('small');context.textContent=(projects.find(project=>project.id===run.projectId)?.name||run.projectId)+' · '+run.skill+' · '+(run.cancelRequested?(run.status==='queued'?'Canceling':'Stopping; waiting for exit'):run.status)
   button.append(title,context);button.onclick=()=>select(run);item.append(button);entries.append(item)
  }
 }
 if(waiting.length){const note=document.createElement('p');note.className='queue-note';note.textContent='Starts when project capacity and checkout availability permit. Independent projects may start separately.';list.append(note)}
}
function render(){
 renderQueue()
 const list=document.querySelector('#runs');list.replaceChildren()

 const groups=new Map(projects.filter(project=>project.path&&!hiddenProject(project.id)).map(project=>[project.id,project]))
 for(const run of runs)if(!hiddenProject(run.projectId)&&!groups.has(run.projectId))groups.set(run.projectId,{id:run.projectId,name:run.projectId})
 for(const project of [...groups.values()].sort((a,b)=>a.name.localeCompare(b.name))){
  const group=document.createElement('section');group.className='project-group'

  const projectRow=document.createElement('div');projectRow.className='project-row'
  const heading=document.createElement('button');heading.className='project-heading';heading.textContent=(collapsedProjects.has(project.id)?'▸ ':'▾ ')+project.name;heading.setAttribute('aria-expanded',String(!collapsedProjects.has(project.id)))
  heading.onclick=()=>{selectedProject=project.id;if(collapsedProjects.has(project.id))collapsedProjects.delete(project.id);else collapsedProjects.add(project.id);localStorage.setItem('collapsedProjects',JSON.stringify([...collapsedProjects]));render()}
  const configure=document.createElement('button');configure.textContent='⚙';configure.setAttribute('aria-label','Configure '+project.name);configure.onclick=()=>openProject(project.id)
  const browse=document.createElement('button');browse.textContent='+';browse.title='New task';browse.setAttribute('aria-label','New task in '+project.name);browse.onclick=()=>newProjectTask(project.id)
  projectRow.append(heading,browse,configure);group.append(projectRow)
  const children=runs.filter(run=>run.projectId===project.id&&!hiddenRun(run))
  const taskGroups=new Map()
  for(const run of children){const key=taskKey(run);if(!taskGroups.has(key))taskGroups.set(key,[]);taskGroups.get(key).push(run)}
  if(!collapsedProjects.has(project.id)){
   if(!children.length){const empty=document.createElement('p');empty.className='hint';empty.textContent='No local tasks';group.append(empty)}
   for(const {executions,run} of orderedTaskGroups(taskGroups.values())){
    const row=document.createElement('div');row.className='local-task'
    const button=document.createElement('button');button.className='run '+(executions.some(item=>item.id===selected)?'selected':'')
    const title=document.createElement('strong');title.textContent=taskState(run).name||taskTitles.get(run.taskId)||run.skill
    const context=document.createElement('button');context.textContent=run.taskKey||run.taskId;context.className='task-number';context.title='Open task in TaskFlow';context.setAttribute('aria-label','Open '+(run.taskKey||run.taskId)+' in TaskFlow');context.onclick=()=>api.openTask(run.taskId).catch(error)
    const status=document.createElement('span');status.className='status '+run.status;status.textContent=({running:'◉',queued:'◷',preparing:'◌',completed:'✓',failed:'!',canceled:'⊘'})[run.status]||'○';status.setAttribute('aria-label',run.status);status.title=run.status
    button.title=title.textContent+' · '+run.skill+' · '+executions.length+' execution(s)';button.dataset.status=run.status
    button.append(title,status);button.onclick=()=>select(run)
    const menu=document.createElement('button');menu.textContent='…';menu.className='task-menu';menu.setAttribute('aria-label','Actions for '+(taskState(run).name||run.taskKey||run.taskId));menu.onclick=()=>taskMenu(run)
    const archive=document.createElement('button');archive.className='task-archive'
    const archiveLabel=(executions.some(activeRun)?'Stop and archive ':'Archive ')+(taskState(run).name||run.taskKey||run.taskId)
    archive.title=archiveLabel;archive.setAttribute('aria-label',archiveLabel)
    archive.innerHTML='<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true"><path d="M4 8h16v12H4zM3 4h18v4H3zM9 12h6"/></svg>'
    archive.onclick=()=>requestArchive(run)
    row.append(context,button)
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
async function updateDisconnected(ids,force=false){
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
 if(changed||force)render()
}
async function refresh(){
 if(refreshing||restarting||!document.querySelector('#setup').hidden)return
 refreshing=true
 const version=projectStateVersion
 try{
  const next=await api.runs(),status=await api.status()
  if(version!==projectStateVersion)return
  ready();refreshPRs(next)
  document.querySelector('#connection').textContent=status.connected?'Connected to '+status.server:'Local agent ready · Server disconnected'
  const previous=runs.find(run=>run.id===selected)
  const serialized=JSON.stringify(next),changed=serialized!==last
  runs=next;last=serialized
  await updateDisconnected(status.disconnectedProjects||[],changed)
  const current=runs.find(run=>run.id===selected)
  if(current&&((current.status!==previous?.status&&(current.status==='running'||!current.sessionId))||current.sessionId!==previous?.sessionId))select(current)
  if(!selected){const visible=runs.find(run=>!hiddenRun(run));if(visible)select(visible)}
  if(changed||Date.now()-nextStepUpdated>15000)refreshNextStep()
 }catch{agentUnavailable()}
 finally{refreshing=false}
}
document.querySelector('#start').onsubmit=async event=>{
 event.preventDefault();const button=event.target.querySelector('button');button.disabled=true
 try{await api.start(Object.fromEntries(new FormData(event.target)));document.querySelector('#error').textContent='';ready();await refresh()}
 catch(err){error(err)}finally{button.disabled=!document.querySelector('#shutdown').hidden}
}
document.querySelector('#stop').onclick=async()=>{
 if(!selected)return;stopping=true;render()
 try{await api.stop(selected);await refresh()}catch(err){error(err)}finally{stopping=false;render()}
}
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
   document.querySelector('#title').textContent='Select an execution'
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
}
const settingsReady=loadSettings().catch(error)
document.querySelector('#configure').onclick=()=>{
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
   document.querySelector('#title').textContent='Select an execution'
   document.querySelector('#directory').textContent=''
  }
  await refresh()
 }catch(err){error(err)}finally{render()}
}

const dialog=document.querySelector('#project-dialog'),dialogBody=document.querySelector('#dialog-body')
document.querySelector('#close-dialog').onclick=()=>dialog.close()
document.querySelector('#dismiss-dialog').onclick=()=>dialog.close()
function showDialog(title){
 dialogBody.replaceChildren()
 const heading=document.createElement('h2');heading.textContent=title;dialogBody.append(heading)
 if(!dialog.open)dialog.showModal()
}
function paragraph(text){const p=document.createElement('p');p.textContent=text;dialogBody.append(p);return p}
async function loadProjects(){
 const version=projectStateVersion
 const status=await api.status(),next=await api.projects()
 if(version!==projectStateVersion)return
 projects=[...new Map(next.map(project=>[project.id,project])).values()]
 await updateDisconnected(status.disconnectedProjects||[],true)
}
document.querySelector('#toggle-sidebar').onclick=event=>{
 const hidden=document.querySelector('#workspace').classList.toggle('sidebar-hidden')
 event.currentTarget.setAttribute('aria-expanded',String(!hidden));localStorage.setItem('sidebarCollapsed',String(hidden));resize()
}
document.querySelector('#profile').onclick=()=>{
 showDialog('Profile')

}
document.querySelector('#add-project').onclick=async()=>{
 showDialog('Add project')
 paragraph('Discover projects from your TaskFlow server and configure their local directory.')
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
  let parallelism=info.parallelism||1,inheritParallelism=!info.parallelismOverride
  const controls={}
  function setting(name,values,resetLabel,onSelect,onReset){
   const section=document.createElement('section');section.className='execution-setting'
   const heading=document.createElement('div');heading.className='setting-heading'
   const title=document.createElement('strong');title.textContent=name
   const reset=document.createElement('button');reset.type='button';reset.className='reset-setting';reset.setAttribute('aria-label',resetLabel);reset.title=resetLabel
   reset.innerHTML='<svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.7"><path d="M4 4v6h6M4 10a8 8 0 1 1 1 8"/></svg>'
   reset.onclick=onReset;heading.append(title,reset)
   const group=document.createElement('div');group.className='segmented';group.setAttribute('role','group');group.setAttribute('aria-label',name)
   const buttons=values.map(value=>{const button=document.createElement('button');button.type='button';button.textContent=String(value);button.onclick=()=>onSelect(value);group.append(button);return button})
   const hint=document.createElement('p');section.append(heading,group,hint)
   return {section,buttons,hint,reset}
  }
  controls.worktrees=setting('Worktrees',['Yes','No'],'Reset worktrees to server default',value=>{useWorktrees=value==='Yes';inheritWorktrees=false;update()},()=>{useWorktrees=!!config.useWorktrees;inheritWorktrees=true;update()})
  controls.parallel=setting('Parallel executions',[1,2,3],'Reset parallelism to server default',value=>{parallelism=value;inheritParallelism=false;update()},()=>{parallelism=config.parallelism||1;inheritParallelism=true;update()})
  function update(){
   controls.worktrees.buttons.forEach((button,i)=>button.setAttribute('aria-pressed',String(useWorktrees===(i===0))))
   controls.worktrees.hint.textContent=(inheritWorktrees?'Inherited':'Local override')+' · Server default: '+(config.useWorktrees?'Yes':'No')
   controls.parallel.buttons.forEach((button,i)=>{button.disabled=!useWorktrees;button.setAttribute('aria-pressed',String(i+1===(useWorktrees?parallelism:1)))})
   controls.parallel.hint.textContent=useWorktrees?(inheritParallelism?'Inherited':'Local override')+' · Server default: '+(config.parallelism||1):'Without worktrees, executions are limited to one.'
  }
  update()
  let inheritCommand=!info.commandOverride
  const commandLabel=document.createElement('label');commandLabel.textContent='CLI command'
  const command=document.createElement('textarea');command.className='cli-command';command.setAttribute('aria-label','CLI command')
  command.value=info.aiCommandTemplate??config.aiCommandTemplate??''
  command.placeholder='Server provider default command'
  const commandHint=document.createElement('p')
  function commandState(){commandHint.textContent=(inheritCommand?'Inherited from server':'Local override')+' · Required: {prompt} (instructions). Also: {issueKey}, {issueTitle}, {issueDesc}, {branchName}, {repoPath} (local directory), {tracker}, {repo}.'}
  command.oninput=()=>{inheritCommand=false;commandState()}
  const commandReset=document.createElement('button');commandReset.type='button';commandReset.className='reset-setting'
  commandReset.setAttribute('aria-label','Reset CLI command to server default');commandReset.title='Reset CLI command to server default';commandReset.innerHTML=controls.worktrees.reset.innerHTML
  commandReset.onclick=()=>{command.value=config.aiCommandTemplate||'';inheritCommand=true;commandState()}
  commandState();commandLabel.append(commandReset,command,commandHint)
  const save=document.createElement('button');save.textContent='Save local configuration'
  const notice=document.createElement('p');notice.setAttribute('role','status')
  form.append(label,controls.worktrees.section,controls.parallel.section,commandLabel,save)
  panels.Local.append(form)
  const remove=document.createElement('button');remove.type='button';remove.textContent='Remove from desktop'
  remove.onclick=()=>requestRemoveProject(id,config.projectName)
  if(info.configured||runs.some(run=>run.projectId===id))panels.Local.append(remove)
  dialogBody.append(notice)
  const tools=document.createElement('div');tools.className='deployment-actions'
  form.onsubmit=async event=>{
   event.preventDefault();save.disabled=true
   try{
    await api.mapProject({projectId:id,path:path.value,useWorktrees,inheritWorktrees,parallelism,inheritParallelism,aiCommandTemplate:command.value,inheritCommand})
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
    if(inheritParallelism)parallelism=config.parallelism||1
    if(inheritCommand)command.value=config.aiCommandTemplate||''
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
if(localStorage.getItem('sidebarCollapsed')==='true'){
 document.querySelector('#workspace').classList.add('sidebar-hidden')
 document.querySelector('#toggle-sidebar').setAttribute('aria-expanded','false')
}
document.querySelector('#start-agent').onclick=async()=>{
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

async function browseTasks(projectID){
 selectedProject=projectID
 showDialog('Launch task')
 const search=document.createElement('form'),query=document.createElement('input'),submit=document.createElement('button'),list=document.createElement('div')
 query.placeholder='Search by title or task key';query.setAttribute('aria-label','Search server tasks');submit.textContent='Search'
 search.append(query,submit);dialogBody.append(search,list)
 let generation=0
 async function load(){
  if(!query.value.trim()){list.textContent='Search for a task to launch.';return}
  const current=++generation;submit.disabled=true
  try{
   const [tasks,info]=await Promise.all([api.serverTasks(projectID,query.value,true),api.project(projectID)])
   if(current!==generation)return
   list.replaceChildren()
   const launchableTasks=tasks.filter(task=>!isFinishedTask(task))
   if(!launchableTasks.length){const empty=document.createElement('p');empty.textContent='No matching server tasks';list.append(empty)}
   for(const task of launchableTasks){
    const card=document.createElement('section');card.className='server-task'
    const title=document.createElement('strong');title.textContent=(task.key||task.id)+' · '+task.title
    const status=document.createElement('p');status.className='task-server-status';status.textContent='Current state: '+taskStage(task)+(task.trackerStatus?' · '+task.trackerStatus:'')
    const skill=document.createElement('select');skill.setAttribute('aria-label','Skill for '+(task.key||task.id))
    for(const item of info.server.skills||[]){const option=document.createElement('option');option.value=item.id;option.textContent=item.command||item.id;skill.append(option)}
    const custom=document.createElement('option');custom.value='custom';custom.textContent='Custom instructions';skill.append(custom)
    const prompt=document.createElement('textarea');prompt.placeholder='What should the agent do?';prompt.setAttribute('aria-label','Custom instructions');prompt.hidden=true
    skill.onchange=()=>{prompt.hidden=skill.value!=='custom'}
    const launch=document.createElement('button');launch.textContent='Launch';launch.disabled=!info.configured||!skill.options.length
    const notice=document.createElement('p');notice.setAttribute('role','status')
    launch.onclick=async()=>{
     if(skill.value==='custom'&&!prompt.value.trim()){notice.textContent='Enter custom instructions.';prompt.focus();return}
     launch.disabled=true;notice.textContent='Submitting execution…'
     try{await api.launchServerTask(projectID,task.id,skill.value,prompt.value);notice.textContent='Execution submitted';dialog.close();await refresh()}
     catch(err){notice.textContent=err.message;launch.disabled=false}
    }
    card.append(title,status,skill,prompt,launch,notice);list.append(card)
   }
  }catch(err){const message=document.createElement('p');message.textContent=err.message;list.replaceChildren(message)}
  finally{if(current===generation)submit.disabled=false}
 }
 search.onsubmit=event=>{event.preventDefault();load()}
 await load()
}

document.querySelector('#rerun').onclick=async()=>{
 const run=runs.find(item=>item.id===selected)
 if(!run||!['completed','failed','canceled'].includes(run.status))return
 showDialog('Relaunch '+(run.taskKey||run.taskId))
 try{
  const info=await api.project(run.projectId)
  const form=document.createElement('form')
  const skillLabel=document.createElement('label');skillLabel.textContent='Skill'
  const skill=document.createElement('select');skill.setAttribute('aria-label','Relaunch skill')
  for(const item of info.server.skills||[]){
   const option=document.createElement('option');option.value=item.id;option.textContent=item.command||item.id;skill.append(option)
  }
  const custom=document.createElement('option');custom.value='custom';custom.textContent='Custom instructions';skill.append(custom)
  skill.value=run.skill
  if(!skill.value){
   const missing=document.createElement('option');missing.value='';missing.textContent='Select a skill (previous skill unavailable)';missing.disabled=true;skill.prepend(missing);skill.value=''
  }
  skill.required=true;skillLabel.append(skill)
  const promptLabel=document.createElement('label');promptLabel.textContent='Instructions'
  const prompt=document.createElement('textarea');prompt.className='cli-command';prompt.setAttribute('aria-label','Relaunch instructions');prompt.value=run.prompt||''
  promptLabel.append(prompt)
  const submit=document.createElement('button');submit.textContent='Launch new execution';submit.disabled=!info.configured
  const notice=document.createElement('p');notice.setAttribute('role','status')
  if(!info.configured)notice.textContent='Configure a local repository before relaunching.'
  form.append(skillLabel,promptLabel,submit,notice);dialogBody.append(form)
  form.onsubmit=async event=>{
   event.preventDefault()
   if(skill.value==='custom'&&!prompt.value.trim()){notice.textContent='Enter custom instructions.';prompt.focus();return}
   submit.disabled=true
   try{
    await api.launchServerTask(run.projectId,run.taskId,skill.value,prompt.value)
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
 if(linksLoading||Date.now()-lastLinksRefresh<15000)return
 linksLoading=true;lastLinksRefresh=Date.now()
 try{
  await Promise.all([...new Set(executions.map(run=>run.projectId))].map(async projectID=>{
   try{
    const tasks=await api.serverTasks(projectID,'')
    for(const run of executions.filter(run=>run.projectId===projectID)){
     const task=tasks.find(task=>task.id===run.taskId)
     if(task?.title)taskTitles.set(run.taskId,task.title)
     if(task?.prUrl&&/^https?:\/\//i.test(task.prUrl))pullRequests.set(run.taskId,task.prUrl)
     else pullRequests.delete(run.taskId)
    }
   }catch{}
  }))
  render()
 }finally{linksLoading=false}
}

function taskMenu(run){
 showDialog(taskState(run).name||run.taskKey||run.taskId)
 const related=()=>runs.filter(item=>taskKey(item)===taskKey(run))
 const relaunch=document.createElement('button');relaunch.textContent='Relaunch';relaunch.disabled=related().some(activeRun)
 relaunch.onclick=()=>{select(run);document.querySelector('#rerun').click()}
 const rename=document.createElement('form'),name=document.createElement('input'),save=document.createElement('button')
 name.setAttribute('aria-label','Local task name');name.value=taskState(run).name||run.taskKey||run.taskId;name.maxLength=120;name.required=true
 save.textContent='Rename locally';rename.append(name,save)
 rename.onsubmit=event=>{event.preventDefault();if(!name.value.trim())return;localTasks[taskKey(run)]={...taskState(run),name:name.value.trim()};saveLocalTasks();dialog.close();render();const current=runs.find(item=>item.id===selected);if(current&&taskKey(current)===taskKey(run))document.querySelector('#title').textContent=name.value.trim()+' · '+current.skill}
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
  document.querySelector('#title').textContent='Select an execution';document.querySelector('#directory').textContent=''
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
   launch.onclick=async()=>{await browseTasks(task.projectId);const query=dialogBody.querySelector('[aria-label="Search server tasks"]');query.value=task.key||task.title;query.form.requestSubmit()}
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
 if(!run){nextStepData=null;renderNextStep();return}
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
