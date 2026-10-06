import { ipcMessage } from './execution-fields.mjs'

// Archiving a ticket task removes its worktrees first (#755). These are the
// texts the archive control and its refusal show, kept apart from main.js so
// they are tested without Electron.

// archiveLabel names the sidebar's archive button. Only a ticket task has
// worktrees to remove: a free console and a macro run keep the plain label.
export function archiveLabel(name,{active=false,ticket=true}={}){
 if(!ticket)return (active?'Stop and archive ':'Archive ')+name
 return (active?'Stop, archive ':'Archive ')+name+' and remove its worktree'
}

const ROLES={code:'',specifications:' (specifications)'}

// archiveRefusal is the message of an archive the agent refused, or '' when
// the answer lets the task be archived. Each worktree left behind is named
// with Git's reason, then what to do about it.
export function archiveRefusal(result){
 const entries=Array.isArray(result?.repositories)?result.repositories:[]
 if(result?.archivable&&entries.length)return ''
 const failed=entries.filter(entry=>entry.outcome==='failed')
 if(!failed.length)return 'Not archived: the local agent did not report the task\'s worktrees. Update and restart the agent, then archive again.'
 const lines=failed.map(entry=>(entry.repository||'Project checkout')+(ROLES[entry.role]??'')+': '+(entry.error||'not removed'))
 return ['Not archived: a worktree of this task could not be removed.',...lines,'Commit or discard the changes there, then archive again.'].join('\n')
}

// archiveFailure is the message of an archive that could not reach the
// agent or the server at all: nothing on the workstation was checked.
export function archiveFailure(error){
 const message=error?ipcMessage(error):''
 return 'Not archived: '+(message||'the local agent did not answer')+'\nStart or update the local agent, then archive again.'
}
