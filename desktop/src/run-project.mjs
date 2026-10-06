// The project a run works for (#741), on the renderer side: the mode a pickup
// is launched with, the project a run is launched again for, and the text a
// refused launch shows. Every desktop launch names its project; only a run
// recorded before #741 has none, and is launched again for the board's.
import { ipcMessage } from './execution-fields.mjs'

// A pickup runs unattended: it names the autonomous mode explicitly, since the
// server treats a launch as unattended only on an explicit mode=autonomous or a
// batch, whatever the project's default mode.
export const PICKUP_MODE='autonomous'

// The mode a launch sends: a pickup's is always autonomous, any other launch
// keeps the one chosen, empty for none.
export function launchModeFor(kind,mode){
 return kind==='pickup'?PICKUP_MODE:mode
}

// What a run recorded without a project, with no board open to stand for one,
// shows instead of the server's refusal.
export const NO_RUN_PROJECT='This execution was recorded without a project. Open the tasks of its project to launch it again.'

// The project a run is launched again for: its own, or, for a run recorded
// before #741 without one, the project of the board the user is on. Empty
// when there is neither.
export function relaunchProject(run,boardProjectId){
 return run?.projectId||boardProjectId||''
}

// The text a refused launch shows. A structured refusal crosses the IPC bridge
// as raw JSON inside the error text: its `error` field is the reason; anything
// else is shown as the agent gave it.
export function launchErrorText(err){
 const text=ipcMessage(err)
 const start=text.indexOf('{')
 if(start<0)return text
 try{
  const body=JSON.parse(text.slice(start,text.lastIndexOf('}')+1))
  return body&&typeof body.error==='string'&&body.error?body.error:text
 }catch{return text}
}
