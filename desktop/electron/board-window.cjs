const crypto=require('node:crypto')

// The server's own board, shown in a window of the desktop instead of the
// browser (spike #396, ADR 0025). The page is the server's web interface and
// nothing else: it gets no preload, so it reaches none of the desktop's IPC,
// and it runs in a session of its own, held in memory, so that whatever it
// stores dies with the window and never meets the desktop's own renderer.
//
// It is signed in by the workstation API key, which the main process adds to
// the requests that go to the server, and only to those. The key never enters
// the page: EventSource cannot carry a header, and a key handed to page script
// could be read back by it. For the same reason the page cannot sign in or out
// by itself: the key is the only identity this window may have.

// boardOrigin is the origin the key may be sent to, or an error when the
// address is not one the desktop would open in the browser either.
function boardOrigin(server){
 const url=new URL(server)
 if(!['http:','https:'].includes(url.protocol)||url.username||url.password)throw Error('Invalid server URL')
 return url.origin
}

// boardURL keeps the server's base path, as the browser link does, and puts
// the task the board should open in the query the web interface reads.
function boardURL(server,taskId){
 const url=new URL(server)
 boardOrigin(url.href)
 url.searchParams.delete('task')
 if(taskId)url.searchParams.set('task',taskId)
 url.hash=''
 return url.href
}

function originOf(value){
 try{return new URL(value).origin}catch{return ''}
}

// navigation answers what a link followed in the board does: the server's
// pages stay in the window, any other web address goes to the browser, and
// every other scheme goes nowhere.
function navigation(target,origin){
 let url
 try{url=new URL(target)}catch{return 'deny'}
 if(url.origin===origin)return 'stay'
 if(['http:','https:'].includes(url.protocol)&&!url.username&&!url.password)return 'external'
 return 'deny'
}

// signedHeaders is the request's headers with the key added, or null when the
// request must go out as it is: another origin, another window, or a frame
// that is not the server's own.
function signedHeaders(details,{origin,key,webContentsId}){
 if(!key||originOf(details.url)!==origin)return null
 if(details.webContentsId!==undefined&&details.webContentsId!==webContentsId)return null
 // A frame that went away while its request was in flight cannot vouch for it.
 let frame=''
 try{frame=details.frame?.url||''}catch{return null}
 if(frame&&originOf(frame)!==origin)return null
 return {...details.requestHeaders,Authorization:'Bearer '+key}
}

// The sign-in and sign-out routes of the web interface. The window refuses
// them, so a key that stops working leaves the board signed out instead of
// signed in as somebody else, and "sign out" does not pretend to work.
function authRoute(target,origin){
 let url
 try{url=new URL(target)}catch{return false}
 return url.origin===origin&&url.pathname.startsWith('/auth/')
}

// The one permission the board keeps is writing to the clipboard, which its
// copy buttons need; it reads nothing, records nothing and notifies nothing.
const allowedPermissions=new Set(['clipboard-sanitized-write'])

function createBoardWindows({BrowserWindow,session,shell,show=true,icon}){
 let current=null,opened=0
 function close(){
  if(!current)return
  const {window,ses}=current
  current=null
  if(!window.isDestroyed())window.destroy()
  ses.clearStorageData().catch(()=>{})
 }
 function open({server,key,taskId}){
  const origin=boardOrigin(server)
  const url=boardURL(server,taskId)
  if(current&&(current.origin!==origin||current.key!==key||current.window.isDestroyed()))close()
  if(current){
   // Opening the board again brings it forward; opening a task loads it.
   if(taskId||current.window.webContents.getURL()==='')current.window.loadURL(url)
   current.window.show();current.window.focus()
   return current.window
  }
  // Not "persist:": the partition lives in memory. Each window gets a fresh
  // one, so clearing the partition of the window it replaces, which finishes
  // later, cannot empty this one while it loads.
  const ses=session.fromPartition('board-'+crypto.createHash('sha256').update(origin).digest('hex').slice(0,16)+'-'+(++opened))
  const window=new BrowserWindow({show,width:1320,height:860,minWidth:800,minHeight:500,title:'Sectile',icon,webPreferences:{session:ses,nodeIntegration:false,contextIsolation:true,sandbox:true,webSecurity:true}})
  const state={origin,key,webContentsId:window.webContents.id}
  current={window,ses,origin,key}
  ses.setPermissionRequestHandler((_,permission,callback)=>callback(allowedPermissions.has(permission)))
  ses.setPermissionCheckHandler((_,permission)=>allowedPermissions.has(permission))
  ses.webRequest.onBeforeRequest((details,callback)=>callback({cancel:authRoute(details.url,origin)}))
  ses.webRequest.onBeforeSendHeaders((details,callback)=>{
   const requestHeaders=signedHeaders(details,state)
   callback(requestHeaders?{requestHeaders}:{})
  })
  const follow=(event,target)=>{
   const verdict=navigation(target,origin)
   if(verdict==='stay')return
   event.preventDefault()
   if(verdict==='external')shell.openExternal(target).catch(()=>{})
  }
  // Electron passes the target on the event and, in older releases, second.
  window.webContents.on('will-navigate',(event,target)=>follow(event,event.url||target))
  window.webContents.on('will-redirect',(event,target)=>follow(event,event.url||target))
  window.webContents.setWindowOpenHandler(({url:target})=>{
   const verdict=navigation(target,origin)
   if(verdict==='stay')window.loadURL(target)
   else if(verdict==='external')shell.openExternal(target).catch(()=>{})
   return {action:'deny'}
  })
  window.on('closed',()=>{if(current?.window===window)close()})
  window.loadURL(url)
  return window
 }
 return {open,close,current:()=>current?.window||null}
}

module.exports={boardOrigin,boardURL,navigation,signedHeaders,authRoute,createBoardWindows}
