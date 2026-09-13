const {app,BrowserWindow,ipcMain,dialog,safeStorage,shell}=require('electron')
const path=require('node:path'),fs=require('node:fs'),crypto=require('node:crypto')
const {spawn}=require('node:child_process')
const WebSocket=require('ws')
const {checkServer}=require('./server-check.cjs')
if(process.env.TASKFLOW_DESKTOP_DATA_DIR)app.setPath('userData',process.env.TASKFLOW_DESKTOP_DATA_DIR)
let window,connection,socket,starting=false
const defaultInfoPath=()=>process.env.TASKFLOW_DESKTOP_DATA_DIR
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
 if(!response.ok)throw Error(await response.text())
 return response.status===204?null:response.json()
}
async function connectAgent(){
 for(const file of new Set([defaultInfoPath(),path.join(app.getPath('userData'),'agent-connection.json')])){
  try{connection=validConnection(JSON.parse(fs.readFileSync(file,'utf8')));await api('/desktop/runs');connectedInfoPath=file;return true}
  catch{connection=null}
 }
 return false
}
ipcMain.handle('connect',connectAgent)
const settingsPath=()=>process.env.TASKFLOW_DESKTOP_DATA_DIR
 ? path.join(app.getPath('userData'),'settings.json')
 : path.join(app.getPath('home'),'.config','taskflow','settings.json')
function readSettings(){
 try{return JSON.parse(fs.readFileSync(settingsPath(),'utf8'))}
 catch(error){
  if(error.code!=='ENOENT')throw error
  try{return JSON.parse(fs.readFileSync(path.join(app.getPath('userData'),'agent-settings.json'),'utf8'))}catch{return {}}
 }
}
ipcMain.handle('settings',()=>{
 try{
  const saved=readSettings()
  return {...saved,token:saved.secret&&safeStorage.isEncryptionAvailable()?safeStorage.decryptString(Buffer.from(saved.secret,'base64')):'',secret:undefined}
 }catch{return {}}
})
ipcMain.handle('start',async(_,settings)=>{
 if(starting)throw Error('Agent is starting')
 starting=true
 try{
  if(await connectAgent())return true
  const url=new URL(settings.server)
  if(!['http:','https:'].includes(url.protocol)||url.username||url.password)throw Error('Use an HTTP or HTTPS server URL')
  if(!settings.token)throw Error('Authentication token is required')
  await checkServer(url,settings.token)
  // Preserve existing mappings when upgrading; new installations use private app data.
  let previous={}
  try{previous=readSettings()}catch{}
  const repo=previous.repo||path.dirname(settingsPath())
  fs.mkdirSync(repo,{recursive:true,mode:0o700})
  const bundled=app.isPackaged?path.join(process.resourcesPath,'taskflow'):path.resolve(__dirname,'../bin/taskflow')
  const binary=(fs.existsSync(bundled)?bundled:path.resolve(__dirname,'../../bin/taskflow'))
  if(!fs.existsSync(binary))throw Error('The bundled TaskFlow agent is missing. Rebuild or reinstall the app.')
  if(connection){try{await api('/desktop/runs');return true}catch{connection=null}}
  fs.mkdirSync(app.getPath('userData'),{recursive:true})
  let legacy={}
  try{legacy=JSON.parse(fs.readFileSync(path.join(repo,'.taskflow','agent.json'),'utf8'))}catch{}
  const saved={...legacy,...previous,server:settings.server,repo}
  delete saved.binary
  fs.mkdirSync(path.dirname(settingsPath()),{recursive:true,mode:0o700})
  if(safeStorage.isEncryptionAvailable())saved.secret=safeStorage.encryptString(settings.token).toString('base64')
  fs.writeFileSync(settingsPath()+'.tmp',JSON.stringify(saved),{mode:0o600})
  fs.renameSync(settingsPath()+'.tmp',settingsPath())
  const output=fs.openSync(path.join(app.getPath('userData'),'agent.log'),'a',0o600)
  const info=infoPath()
  if(fs.existsSync(info))fs.unlinkSync(info)
  const child=spawn(binary,['agent','--desktop-info',info,'--url',settings.server,'--repo',repo],{
   detached:true,stdio:['ignore',output,output],
   env:{...process.env,TASKFLOW_AGENT_TOKEN:settings.token,TASKFLOW_DESKTOP_TOKEN:crypto.randomBytes(32).toString('hex')}
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
 }finally{starting=false}
})
async function lifecycle(action){
 if(starting)throw Error('Agent lifecycle operation already in progress')
 starting=true
 try{
  const runs=await api('/desktop/runs')
  const active=runs.filter(run=>['running','queued','preparing'].includes(run.status))
  const result=await dialog.showMessageBox(window,{type:'warning',buttons:['Cancel',action==='restart'?'Restart agent':'Stop agent'],defaultId:0,cancelId:0,message:action==='restart'?'Restart the local agent?':'Stop the local agent?',detail:active.length
   ? active.length+' active execution(s) will be stopped. Console history will be cleared.'
   : 'Console history will be cleared. Server settings and local project directories are preserved.'})
  if(result.response!==1)return false
  await Promise.all(active.map(run=>api('/desktop/stop?id='+encodeURIComponent(run.id),'POST')))
  if(socket){socket.removeAllListeners();socket.close();socket=null}
  const previous=fs.readFileSync(infoPath(),'utf8')
  const modified=fs.statSync(infoPath()).mtimeMs
  await api('/desktop/'+(action==='restart'?'restart':'shutdown'),'POST')
  if(action==='stop'){
   for(let attempt=0;attempt<100;attempt++){
    await new Promise(resolve=>setTimeout(resolve,200))
    try{await api('/desktop/runs')}catch{connection=null;return true}
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
    return true
   }catch{}
  }
  throw Error('Agent did not restart. Check agent.log in the application data directory.')
 }finally{starting=false}
}
ipcMain.handle('restart',()=>lifecycle('restart'))
ipcMain.handle('shutdown',()=>lifecycle('stop'))
ipcMain.handle('save-log',async(_,text)=>{
 if(typeof text!=='string'||text.length>10_000_000)throw Error('Invalid log')
 const result=await dialog.showSaveDialog(window,{defaultPath:'taskflow-execution.log'})
 if(!result.canceled&&result.filePath)fs.writeFileSync(result.filePath,text,{mode:0o600})
})
ipcMain.handle('status',()=>api('/desktop/status'))
ipcMain.handle('choose-repository',async()=>{
 const result=await dialog.showOpenDialog(window,{title:'Select local repository',properties:['openDirectory']})
 return result.canceled?null:result.filePaths[0]
})
ipcMain.handle('server-tasks',(_,id,q,launchable)=>api('/desktop/tasks?projectId='+encodeURIComponent(id)+'&q='+encodeURIComponent(q||'')+'&launchable='+Boolean(launchable)))
ipcMain.handle('launch-server-task',(_,id,taskID,skillID,prompt)=>api('/desktop/tasks?projectId='+encodeURIComponent(id),'POST',{taskID,skillID,prompt}))
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
ipcMain.handle('project',(_,id)=>api('/desktop/project?id='+encodeURIComponent(id)))
ipcMain.handle('deploy-project',(_,id,action)=>api('/desktop/project?id='+encodeURIComponent(id)+'&action='+encodeURIComponent(action),'POST'))
ipcMain.handle('projects',()=>api('/desktop/projects'))
ipcMain.handle('map-project',(_,mapping)=>api('/desktop/projects','POST',mapping))
ipcMain.handle('clear-history',()=>api('/desktop/history','DELETE'))
ipcMain.handle('runs',()=>api('/desktop/runs'))
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
function openWindow(){
 if(window&&!window.isDestroyed()){window.show();return}
 window=new BrowserWindow({show:process.env.TASKFLOW_DESKTOP_TEST!=='1',width:1240,height:820,minWidth:800,minHeight:500,backgroundColor:'#11151c',title:'Sectile Desktop',webPreferences:{preload:path.join(__dirname,'preload.cjs'),nodeIntegration:false,contextIsolation:true,sandbox:true}})
 window.webContents.setWindowOpenHandler(()=>({action:'deny'}))
 window.webContents.on('will-navigate',event=>event.preventDefault())
 window.loadFile(path.join(__dirname,'../dist/index.html'))
 window.on('closed',()=>{window=null;if(socket){socket.close();socket=null}})
}
if(!app.requestSingleInstanceLock())app.quit()
else{
 app.on('second-instance',openWindow)
 app.whenReady().then(openWindow)
 app.on('activate',openWindow)
 // The detached agent and its PTYs survive closing or quitting this UI.
 app.on('window-all-closed',()=>app.quit())
}
