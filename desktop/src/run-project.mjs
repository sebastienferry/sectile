// The project a run works for (#741), on the renderer side: reading the
// server's refusal of a launch on a ticket of several projects, and the mode a
// pickup is launched with.

// A pickup runs unattended: it names the autonomous mode explicitly, since the
// server treats a launch as unattended only on an explicit mode=autonomous or a
// batch, whatever the project's default mode.
export const PICKUP_MODE='autonomous'

// The mode a launch sends: a pickup's is always autonomous, any other launch
// keeps the one chosen, empty for none.
export function launchModeFor(kind,mode){
 return kind==='pickup'?PICKUP_MODE:mode
}

// The refusal of a launch whose project could not be chosen, or null when the
// failure is something else. The agent passes the server's JSON answer on, and
// the error that crosses the IPC carries it in its message.
export function runProjectRefusal(message){
 const text=String(message||'')
 const start=text.indexOf('{')
 if(start<0)return null
 try{
  const body=JSON.parse(text.slice(start,text.lastIndexOf('}')+1))
  const candidates=Array.isArray(body?.candidates)?body.candidates.filter(candidate=>candidate&&candidate.id):[]
  if(candidates.length===0)return null
  return {error:typeof body.error==='string'?body.error:'',candidates,unattended:Boolean(body.unattended)}
 }catch{return null}
}

// What an unattended launch's refusal reads as: the server's reason and the
// projects the ticket belongs to, so the person knows where to launch it from.
export function runProjectRefusalText(refusal){
 const names=(refusal?.candidates||[]).map(candidate=>candidate.name||candidate.id)
 const reason=refusal?.error||'This ticket belongs to several projects.'
 return reason+(names.length?' Projects: '+names.join(', ')+'.':'')
}
