const {app,BrowserWindow,Menu,ipcMain,dialog,safeStorage,shell,clipboard}=require('electron')
const path=require('node:path'),fs=require('node:fs'),crypto=require('node:crypto')
const {spawn}=require('node:child_process')
const WebSocket=require('ws')
const {checkServer}=require('./server-check.cjs')
const {exchangePairingCode,resolveConnectCredential}=require('./pairing.cjs')
const credentials=require('./credential-store.cjs')
const storeKey=(saved,token)=>credentials.storeKey(saved,token,safeStorage)
const storedKey=saved=>credentials.storedKey(saved,safeStorage)
const {carryOverDataDirectory}=require('./datadir.cjs')
const {readAgentLog}=require('./agent-log.cjs')
const {fileSha256,agentOutdated}=require('./agent-identity.cjs')
const {connectionUpdates,connectionView}=require('./connection-settings.cjs')
if(process.env.SECTILE_DESKTOP_DATA_DIR)app.setPath('userData',process.env.SECTILE_DESKTOP_DATA_DIR)
// The app kept its data under the previous package name; carry it over once.
if(!process.env.SECTILE_DESKTOP_DATA_DIR){
 carryOverDataDirectory(path.join(app.getPath('appData'),'taskflow-desktop'),app.getPath('userData'))
}
let window,connection,socket,starting=false
// outdated says the running agent is not the binary this app would start;
// promptedFor is the running agent the restart was already offered for, so a
// refusal is not asked again for that agent during this session.
let outdated=false,promptedFor=null
const defaultInfoPath=()=>process.env.SECTILE_DESKTOP_DATA_DIR
 ? path.join(app.getPath('userData'),'agent-connection.json')
 : path.join(app.getPath('home'),'.taskflow','agent-connection.json')
let connectedInfoPath
const infoPath=()=>connectedInfoPath||defaultInfoPath()
function validConnection(value){
 const url=new URL(value.url)
 if(url.protocol!=='http:'||url.hostname!=='127.0.0.1'||!value.token)throw Error('Invalid local connection')
 return value
}
async function api(route,method='GET',body){
 if(!connection)throw Error('Connect to the local agent first')
 const response=await fetch(connection.url+route,{method,headers:{Authorization:'Bearer '+connection.token,'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined,signal:AbortSignal.timeout(route==="/desktop/create-task"?120000:route.startsWith("/desktop/tasks")&&method==="POST"?60000:route.startsWith("/desktop/project?")&&method==="POST"?420000:15000),redirect:'error'})
 if(!response.ok){
  const detail=await response.text().catch(()=>'')
  // Keep the raw body as the message so callers can parse structured errors; name the call when it is empty.
  const failure=Error(detail||method+' '+route+' failed with HTTP '+response.status)
  Object.assign(failure,{status:response.status,route,method,body:detail})
  throw failure
 }
 return response.status===204?null:response.json()
}
async function connectAgent(){
 for(const file of new Set([defaultInfoPath(),path.join(app.getPath('userData'),'agent-connection.json')])){
  try{connection=validConnection(JSON.parse(fs.readFileSync(file,'utf8')));await api('/desktop/runs');connectedInfoPath=file;return true}
  catch{connection=null}
 }
 return false
}
ipcMain.handle('connect',async()=>{
 const connected=await connectAgent()
 if(connected)scheduleIdentityCheck()
 return connected
})
function bundledAgentBinary(){
 // The UI tests have no bundled agent: they name a file standing for it.
 if(process.env.SECTILE_DESKTOP_TEST==='1'&&process.env.SECTILE_DESKTOP_TEST_AGENT_BINARY)return process.env.SECTILE_DESKTOP_TEST_AGENT_BINARY
 return require('./runtime.cjs').resolveAgentBinary({packaged:app.isPackaged,resourcesPath:process.resourcesPath,directory:__dirname})
}
// The bundled binary is hashed once per file state, not on every connection.
let bundledIdentity={key:null,sha:null}
async function bundledAgentSha256(){
 let binary,stat
 try{binary=bundledAgentBinary();stat=fs.statSync(binary)}catch{return null}
 const key=binary+':'+stat.size+':'+stat.mtimeMs
 if(bundledIdentity.key!==key)bundledIdentity={key,sha:await fileSha256(binary)}
 return bundledIdentity.sha
}
function setOutdated(value){
 outdated=value
 if(window&&!window.isDestroyed())window.webContents.send('agent-outdated',value)
}
// Runs after the IPC answers, so a connection is never held up by hashing and
// the restart it may offer never overlaps the start that connected.
function scheduleIdentityCheck(){setImmediate(()=>checkAgentIdentity().catch(()=>{}))}
async function checkAgentIdentity(){
 if(starting)return
 let running
 try{running=await api('/desktop/version')}catch{return}
 const bundled=await bundledAgentSha256()
 setOutdated(agentOutdated({running,bundled}))
 const identity=running.binarySha256||'legacy'
 if(!outdated||promptedFor===identity)return
 promptedFor=identity
 await lifecycle('restart',{reason:'outdated'})
}
ipcMain.handle('pair',async(_,{server,code,label})=>{
 const credential=await exchangePairingCode(server,code,label)
 let previous={}
 try{previous=readSettings()}catch{}
 const saved={...previous,server,deviceId:credential.deviceId}
 delete saved.binary
 storeKey(saved,credential.token)
 fs.mkdirSync(path.dirname(settingsPath()),{recursive:true,mode:0o700})
 fs.writeFileSync(settingsPath()+'.tmp',JSON.stringify(saved),{mode:0o600})
 fs.renameSync(settingsPath()+'.tmp',settingsPath())
 return {deviceId:credential.deviceId,token:credential.token}
})
const settingsPath=()=>process.env.SECTILE_DESKTOP_DATA_DIR
 ? path.join(app.getPath('userData'),'settings.json')
 : path.join(app.getPath('home'),'.config','sectile','settings.json')
function readSettings(){
 try{return JSON.parse(fs.readFileSync(settingsPath(),'utf8'))}
 catch(error){
  if(error.code!=='ENOENT')throw error
  try{return JSON.parse(fs.readFileSync(path.join(app.getPath('userData'),'agent-settings.json'),'utf8'))}catch{return {}}
 }
}
// The renderer reads the connection facts only: the execution sections of the
// same file belong to the agent and are read through /desktop/workstation.
ipcMain.handle('settings',()=>{
 try{
  const saved=readSettings()
  return connectionView(saved,storedKey(saved))
 }catch{return {}}
})
// Only connection keys are written; any execution key is dropped, since the
// agent is the only writer of the execution sections (#305). What the agent
// wrote in the file is kept as it was.
ipcMain.handle('save-settings',async(_,updates)=>{
 let previous={}
 try{previous=readSettings()}catch{}
 const saved={...previous,...connectionUpdates(updates)}
 fs.mkdirSync(path.dirname(settingsPath()),{recursive:true,mode:0o700})
 fs.writeFileSync(settingsPath()+'.tmp',JSON.stringify(saved,null,2),{mode:0o600})
 fs.renameSync(settingsPath()+'.tmp',settingsPath())
 return connectionView(saved,storedKey(saved))
})
// What this installation is running. The desktop's own version comes from the
// package the app was built from; the agent's comes from the agent itself,
// because the two are distributed separately and a workstation that upgraded
// one and not the other is exactly the case this panel has to make visible.
// An agent that is not running leaves its version null rather than failing the
// call: the desktop version is the answer somebody stopped to look for.
ipcMain.handle('version',async()=>{
 let agent=null
 try{agent=(await api('/desktop/version')).version||null}catch{}
 return {desktop:app.getVersion(),agent,outdated:Boolean(agent)&&outdated}
})
ipcMain.handle('start',async(_,settings)=>{
 if(starting)throw Error('Agent is starting')
 starting=true
 let started=false
 try{
  started=await startAgent(settings)
  return started
 }finally{
  starting=false
  if(started)scheduleIdentityCheck()
 }
})
// startAgent connects to a running agent or spawns the bundled one. The caller
// holds the lifecycle guard.
async function startAgent(settings){
 if(await connectAgent())return true
 const url=new URL(settings.server)
 if(!['http:','https:'].includes(url.protocol)||url.username||url.password)throw Error('Use an HTTP or HTTPS server URL')
 // A pairing code is spent here, once the server address is known to be usable:
 // burning a single-use code on a malformed URL would cost the user a new one.
 // With no code, the credential an earlier pairing left behind restarts the
 // agent: the form asks for a code, never for a key to paste back in.
 let kept=''
 try{kept=storedKey(readSettings())}catch{}
 const credential=await resolveConnectCredential({...settings,token:kept})
 const token=credential.token
 await checkServer(url,token)
 // Preserve existing mappings when upgrading; new installations use private app data.
 let previous={}
 try{previous=readSettings()}catch{}
 const repo=previous.repo||path.dirname(settingsPath())
 fs.mkdirSync(repo,{recursive:true,mode:0o700})
 const binary=bundledAgentBinary()
 if(connection){try{await api('/desktop/runs');return true}catch{connection=null}}
 fs.mkdirSync(app.getPath('userData'),{recursive:true})
 let legacy={}
 try{legacy=JSON.parse(fs.readFileSync(path.join(repo,'.taskflow','agent.json'),'utf8'))}catch{}
 const saved={...legacy,...previous,server:settings.server,repo}
 if(credential.deviceId)saved.deviceId=credential.deviceId
 delete saved.binary
 fs.mkdirSync(path.dirname(settingsPath()),{recursive:true,mode:0o700})
 storeKey(saved,token)
 fs.writeFileSync(settingsPath()+'.tmp',JSON.stringify(saved),{mode:0o600})
 fs.renameSync(settingsPath()+'.tmp',settingsPath())
 const output=fs.openSync(path.join(app.getPath('userData'),'agent.log'),'a',0o600)
 const info=infoPath()
 if(fs.existsSync(info))fs.unlinkSync(info)
 const child=spawn(binary,['--desktop-info',info,'--url',settings.server,'--repo',repo],{
  detached:true,stdio:['ignore',output,output],
  env:{...process.env,TOKEN:token,SECTILE_DESKTOP_TOKEN:crypto.randomBytes(32).toString('hex')}
 })
 let spawnError
 child.on('error',error=>{spawnError=error})
 child.unref();fs.closeSync(output)
 for(let attempt=0;attempt<50;attempt++){
  if(spawnError)throw spawnError
  await new Promise(resolve=>setTimeout(resolve,200))
  if(fs.existsSync(info)){
   connection=validConnection(JSON.parse(fs.readFileSync(info,'utf8')))
   try{await api('/desktop/runs');return true}catch{connection=null}
  }
 }
 throw Error('Agent did not become ready. Check agent.log in the application data directory.')
}
async function lifecycle(action,{reason}={}){
 if(starting)throw Error('Agent lifecycle operation already in progress')
 starting=true
 let restarted=false
 try{
  const runs=await api('/desktop/runs')
  const active=runs.filter(run=>['running','queued','preparing'].includes(run.status))
  const detail=active.length
   ? active.length+' active execution(s) will be stopped. Console history will be cleared.'
   : 'Console history will be cleared. Server settings and local project directories are preserved.'
  const result=await dialog.showMessageBox(window,{type:'warning',buttons:['Cancel',action==='restart'?'Restart agent':'Stop agent'],defaultId:0,cancelId:0,message:action==='restart'?'Restart the local agent?':'Stop the local agent?',detail:reason==='outdated'
   ? 'The running agent is not the one bundled with this app.\n'+detail
   : detail})
  if(result.response!==1)return false
  await Promise.all(active.map(run=>api('/desktop/stop?id='+encodeURIComponent(run.id),'POST')))
  if(socket){socket.removeAllListeners();socket.close();socket=null}
  const previous=fs.readFileSync(infoPath(),'utf8')
  const modified=fs.statSync(infoPath()).mtimeMs
  // An outdated agent may have been started from another path, which
  // /desktop/restart would run again: it is stopped, and the bundled binary
  // started in its place. Without a saved server there is nothing to start it
  // with, and the agent's own restart is the only one available.
  let server
  try{server=readSettings().server}catch{}
  const replace=action==='restart'&&reason==='outdated'&&Boolean(server)
  await api('/desktop/'+(action==='restart'&&!replace?'restart':'shutdown'),'POST')
  if(action==='stop'||replace){
   for(let attempt=0;attempt<100;attempt++){
    await new Promise(resolve=>setTimeout(resolve,200))
    try{await api('/desktop/runs')}catch{
     connection=null
     if(!replace)return true
     restarted=await startAgent({server})
     return restarted
    }
   }
   throw Error('Agent has not stopped yet.')
  }
  for(let attempt=0;attempt<100;attempt++){
   await new Promise(resolve=>setTimeout(resolve,200))
   try{
    const raw=fs.readFileSync(infoPath(),'utf8')
    if(raw===previous&&fs.statSync(infoPath()).mtimeMs===modified)continue
    connection=validConnection(JSON.parse(raw))
    await api('/desktop/runs')
    restarted=true
    return true
   }catch{}
  }
  throw Error('Agent did not restart. Check agent.log in the application data directory.')
 }finally{
  starting=false
  // A restarted agent is checked again, which clears the mark once it is the
  // bundled one.
  if(restarted)scheduleIdentityCheck()
 }
}
// Restarting an outdated agent from the settings replaces it with the bundled one too.
ipcMain.handle('restart',()=>lifecycle('restart',outdated?{reason:'outdated'}:{}))
ipcMain.handle('shutdown',()=>lifecycle('stop'))
ipcMain.handle('agent-logs',()=>readAgentLog(path.join(app.getPath('userData'),'agent.log')))
ipcMain.handle('save-log',async(_,text)=>{
 if(typeof text!=='string'||text.length>10_000_000)throw Error('Invalid log')
 const result=await dialog.showSaveDialog(window,{defaultPath:'sectile-execution.log'})
 if(!result.canceled&&result.filePath)fs.writeFileSync(result.filePath,text,{mode:0o600})
})
// The renderer has no clipboard permission of its own; copying goes through the
// main process, which writes exactly the string it was handed and nothing else.
ipcMain.handle('copy-text',(_,text)=>{
 if(typeof text!=='string'||!text)throw Error('Nothing to copy')
 clipboard.writeText(text)
})
ipcMain.handle('mcp-config',(_,provider)=>api('/desktop/mcp?provider='+encodeURIComponent(provider)))
ipcMain.handle('configure-mcp',(_,provider,choice)=>api('/desktop/mcp?provider='+encodeURIComponent(provider),'POST',choice))
ipcMain.handle('status',()=>api('/desktop/status'))
// The workstation execution defaults, read and written by the agent. Without a
// running agent the call fails: the desktop never writes them itself.
ipcMain.handle('workstation-settings',()=>api('/desktop/workstation'))
ipcMain.handle('save-workstation-settings',(_,defaults)=>{
 if(!defaults||typeof defaults!=='object'||Array.isArray(defaults))throw Error('Invalid workstation settings')
 return api('/desktop/workstation','PUT',defaults)
})
ipcMain.handle('choose-repository',async()=>{
 const result=await dialog.showOpenDialog(window,{title:'Select local repository',properties:['openDirectory']})
 return result.canceled?null:result.filePaths[0]
})
ipcMain.handle('server-tasks',(_,id,q,launchable)=>api('/desktop/tasks?projectId='+encodeURIComponent(id)+'&q='+encodeURIComponent(q||'')+'&launchable='+Boolean(launchable)))
ipcMain.handle('launch-console',(_,projectId,provider)=>api('/desktop/consoles','POST',{projectId,provider}))
// An absent mode means "no override": nothing is sent, so a launch with no
// explicit choice puts exactly the payload on the wire that it always did.
ipcMain.handle('launch-server-task',(_,id,taskID,skillID,prompt,mode,force)=>api('/desktop/tasks?projectId='+encodeURIComponent(id),'POST',Object.assign({taskID,skillID,prompt},mode?{mode}:null,force?{force:true}:null)))
ipcMain.handle('launch-native-discussion',async(_,{projectId,taskId,terminal}={})=>api('/desktop/tasks/terminal-external','POST',{projectId,taskId,skillId:'discuss',terminal}))
ipcMain.handle('detach-to-native-terminal',async(_,{runId,terminal}={})=>api('/desktop/terminal/detach','POST',{runId,terminal}))
ipcMain.handle('open-board',async()=>{
 const status=await api('/desktop/status')
 if(!status.connected)throw Error('Server disconnected')
 const url=new URL(status.server)
 if(!['http:','https:'].includes(url.protocol)||url.username||url.password)throw Error('Invalid server URL')
 url.searchParams.delete('task')
 url.hash=''
 await shell.openExternal(url.href)
})
ipcMain.handle('open-task',async(_,id)=>{
 if(typeof id!=='string'||!id.trim())throw Error('Invalid task ID')
 const status=await api('/desktop/status')
 const url=new URL(status.server||readSettings().server)
 if(!['http:','https:'].includes(url.protocol)||url.username||url.password)throw Error('Invalid server URL')
 url.searchParams.set('task',id)
 url.hash=''
 await shell.openExternal(url.href)
})
ipcMain.handle('open-pr',async(_,value)=>{
 const url=new URL(value)
 if(!['http:','https:'].includes(url.protocol)||url.username||url.password)throw Error('Invalid pull request URL')
 await shell.openExternal(url.href)
})
ipcMain.handle('create-task',async(_,input)=>{
 const status=await api('/desktop/status')
 if(!status.capabilities?.includes('create-task'))throw Error('The running local agent is outdated. Stop it, then start the rebuilt agent before creating a task. Closing the desktop alone does not restart the agent.')
 return api('/desktop/create-task','POST',input)
})
ipcMain.handle('transition-stage',async(_,{projectId,taskId,stage,note})=>{
 if(!projectId||!taskId||!stage)throw Error('Project, task, and stage required')
 const status=await api('/desktop/status')
 if(!status.capabilities?.includes('transition-stage')){
  throw Error('The running local agent does not support stage transitions. Update and restart the agent.')
 }
 return api('/desktop/tasks/transition?projectId='+encodeURIComponent(projectId),'POST',{taskId,stage,note})
})
ipcMain.handle('project',(_,id)=>api('/desktop/project?id='+encodeURIComponent(id)))
ipcMain.handle('deploy-project',(_,id,action,provider)=>api('/desktop/project?id='+encodeURIComponent(id)+'&action='+encodeURIComponent(action)+(provider?'&provider='+encodeURIComponent(provider):''),'POST'))
ipcMain.handle('projects',()=>api('/desktop/projects'))
ipcMain.handle('remove-project',async(_,id)=>{
 const status=await api('/desktop/status')
 if(!status.capabilities?.includes('remove-project'))throw Error('The running local agent does not support project removal. Update it, then stop and restart the agent. Closing the desktop alone does not restart it.')
 return api('/desktop/projects?id='+encodeURIComponent(id),'DELETE')
})
ipcMain.handle('map-project',(_,mapping)=>api('/desktop/projects','POST',mapping))
// The folders of a multi-repo project's repositories on this workstation
// (#456). An agent that predates them answers nothing useful, so it is named.
async function requireRepositories(){
 const status=await api('/desktop/status')
 if(!status.capabilities?.includes('repositories'))throw Error('Update and restart the local agent to map the repositories of a multi-repo project.')
}
ipcMain.handle('repositories',async(_,projectId)=>{await requireRepositories();return api('/desktop/repositories?projectId='+encodeURIComponent(projectId))})
ipcMain.handle('map-repository',async(_,mapping)=>{await requireRepositories();return api('/desktop/repositories','POST',mapping)})
// The Git initialization of a project folder (#481). An agent that predates
// it reports no state, so the settings offer nothing and behave as before.
async function hasGitInit(){
 const status=await api('/desktop/status')
 return !!status.capabilities?.includes('git-init')
}
ipcMain.handle('git-state',async(_,folder)=>{
 if(!(await hasGitInit()))return {state:'unknown'}
 return api('/desktop/git-init?path='+encodeURIComponent(folder))
})
ipcMain.handle('git-init',async(_,folder)=>{
 if(!(await hasGitInit()))throw Error('Update and restart the local agent to initialize a Git repository.')
 return api('/desktop/git-init','POST',{path:folder})
})
ipcMain.handle('clear-history',()=>api('/desktop/history','DELETE'))
ipcMain.handle('git-diff',async(_,id)=>{
 if(typeof id!=='string'||!id||id.length>512)throw Error('Select an execution to inspect changes.')
 const status=await api('/desktop/status')
 if(!status.capabilities?.includes('git-diff'))throw Error('Update and restart the local agent to inspect changes.')
 try{return await api('/desktop/git-diff?id='+encodeURIComponent(id))}
 catch(err){
  let detail
  try{detail=JSON.parse(err.message)}catch{throw err}
  throw Error(detail.error?.message||'Inspection failed. Refresh to retry.')
 }
})
ipcMain.handle('runs',()=>api('/desktop/runs'))
// The agent forgets a run once its history is cleared or it restarts, and
// answers 404 by contract. Report "no result" instead of rejecting the IPC
// promise: Electron logs every rejected handler with a stack, and this outcome
// is expected. The renderer decides whether the missing run is an anomaly.
ipcMain.handle('run-result',async(_,id)=>{
 try{return await api('/desktop/run-result?id='+encodeURIComponent(id))}
 catch(failure){if(failure.status===404)return null;throw failure}
})
ipcMain.handle('stop',(_,id)=>api('/desktop/stop?id='+encodeURIComponent(id),'POST'))
ipcMain.handle('detach',()=>{if(socket){socket.removeAllListeners();socket.close();socket=null}})
ipcMain.handle('attach',(_,id)=>{
 if(socket){socket.removeAllListeners();socket.close()}
 if(!connection)throw Error('Agent disconnected')
 socket=new WebSocket(connection.url.replace('http:','ws:')+'/desktop/terminal?id='+encodeURIComponent(id),{headers:{Authorization:'Bearer '+connection.token}})
 const current=socket
 current.on('message',data=>{if(window&&!window.isDestroyed())window.webContents.send('terminal-output',Array.from(data))})
 current.on('error',error=>{if(window&&!window.isDestroyed())window.webContents.send('terminal-output',Array.from(Buffer.from('\r\nConnection error: '+error.message+'\r\n')))})
})
ipcMain.on('terminal-input',(_,data)=>{if(socket?.readyState===WebSocket.OPEN&&typeof data==='string')socket.send(JSON.stringify({type:'input',data}))})
ipcMain.on('terminal-resize',(_,size)=>{if(socket?.readyState===WebSocket.OPEN&&size.cols>0&&size.rows>0)socket.send(JSON.stringify({type:'resize',...size}))})
Menu.setApplicationMenu(process.platform==='darwin'?Menu.buildFromTemplate([{role:'appMenu'},{role:'editMenu'},{role:'windowMenu'}]):null)
function openWindow(){
 if(window&&!window.isDestroyed()){window.show();return}
 // The window draws its own title bar: the app header is the title bar, and the system buttons are
 // painted over it in the app's colours. macOS keeps its traffic lights, positioned to sit centred
 // in that 68px header. The renderer asks the overlay itself where the buttons ended up, so the
 // header can keep their strip clear whatever the platform draws.
 window=new BrowserWindow({show:process.env.SECTILE_DESKTOP_TEST!=='1',width:1240,height:820,minWidth:800,minHeight:500,backgroundColor:'#11151c',title:'Sectile Desktop',titleBarStyle:'hidden',titleBarOverlay:{color:'#11151c',symbolColor:'#d8e0ec',height:68},trafficLightPosition:{x:18,y:25},icon:path.join(__dirname,'../assets/icon.png'),webPreferences:{preload:path.join(__dirname,'preload.cjs'),nodeIntegration:false,contextIsolation:true,sandbox:true}})
 window.webContents.setWindowOpenHandler(()=>({action:'deny'}))
 window.webContents.on('will-navigate',event=>event.preventDefault())
 window.loadFile(path.join(__dirname,'../dist/index.html'))
 window.on('closed',()=>{window=null;if(socket){socket.close();socket=null}})
}
if(!app.requestSingleInstanceLock())app.quit()
else{
 if(process.platform==='darwin'&&app.dock)app.dock.setIcon(path.join(__dirname,'../assets/icon.png'))
 app.on('second-instance',openWindow)
 app.whenReady().then(()=>{
  if(process.platform==='darwin'&&app.dock)app.dock.setIcon(path.join(__dirname,'../assets/icon.png'))
  openWindow()
 })
 app.on('activate',openWindow)
 // The detached agent and its PTYs survive closing or quitting this UI.
 app.on('window-all-closed',()=>app.quit())
}
