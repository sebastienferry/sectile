export function renderDesktopUpdates(container,state,api){
 container.replaceChildren()
 const message=document.createElement('p');message.className='desktop-update-status';message.setAttribute('role','status')
 const messages={idle:'Updates have not been checked yet.',checking:'Checking for updates…','up-to-date':'Sectile Desktop is up to date.',available:'A Desktop update is available.',unavailable:'A newer release has no download for this platform.',uncomparable:'The installed Desktop version cannot be compared.',error:'The update check failed. Last known information is shown.'}
 message.textContent=messages[state.status]||messages.idle
 if(state.error)message.textContent+=' '+({network:'Network unavailable.',timeout:'The check timed out.',throttled:'GitHub rate limit reached. Try again later.','invalid-response':'Release information is incomplete or invalid.',storage:'Update information could not be saved.'}[state.error]||'')
 container.append(message)
 if(state.checkedAt){const time=document.createElement('p');time.textContent='Last successful check: '+new Date(state.checkedAt).toLocaleString();container.append(time)}
 if(state.unavailableVersion){const note=document.createElement('p');note.textContent=`Version ${state.unavailableVersion} has no download for this platform.`;container.append(note)}
 if(state.available){
  const label=document.createElement('p');label.textContent='Available version: '+state.available.version;container.append(label)
  for(const [text,url] of [['Release page',state.available.releaseUrl],['Download Desktop',state.available.downloadUrl]]){
   const button=document.createElement('button');button.type='button';button.textContent=text
   button.onclick=()=>api.openDesktopUpdate(url).catch(()=>{message.textContent='The update link could not be opened.'});container.append(button)
  }
 }
 const check=document.createElement('button');check.type='button';check.textContent='Check for updates';check.disabled=state.status==='checking'
 check.onclick=()=>api.checkDesktopUpdates().catch(()=>{message.textContent='The update check could not be started.'})
 container.append(check)
}
