// The "Sandbox" category of a project's settings (#700): what the project's
// Claude Code sessions are allowed on this workstation. The agent stores the
// values in the workstation settings and hands them to every built-in Claude
// line of the project through --settings (internal/agentconfig/claude_sandbox.go).

import { previewLines } from './command-preview.mjs'

// The sandbox states, in the order the segmented control shows them.
export const SANDBOX_STATES=['Inherited','On','Off']

// sandboxState reads the stored value: true, false, or unset (inherited).
export function sandboxState(enabled){return enabled===true?'On':enabled===false?'Off':'Inherited'}

// addEntry appends one entry to a list the way the agent normalises it:
// trimmed, never twice, never empty. It returns the list and, when nothing
// was added, why.
export function addEntry(list,value){
 const entry=String(value??'').trim()
 if(!entry)return {list,error:'Enter a value before adding it.'}
 if(/[\r\n]/.test(entry))return {list,error:'An entry is a single line.'}
 if(list.includes(entry))return {list,error:entry+' is already in the list.'}
 return {list:[...list,entry]}
}

// bothLists names the rules that are allowed and denied at once: Claude Code
// denies them, the deny rule winning.
export function bothLists(allow,deny){
 const denied=new Set(deny)
 return allow.filter(rule=>denied.has(rule))
}

// sandboxPayload is what the save sends the agent. Every key is present, so
// emptying a list clears it.
export function sandboxPayload(values){
 return {
  enabled:values.state==='On'?true:values.state==='Off'?false:null,
  allowedDomains:[...values.allowedDomains],allowWrite:[...values.allowWrite],
  allow:[...values.allow],deny:[...values.deny],
 }
}

// launchesGetSettings says whether a launch of the project receives a settings
// file: the agent writes none for values that state nothing this platform
// applies, and Windows applies only the rules.
export function launchesGetSettings(values,platformSandbox){
 if(values.allow.length||values.deny.length)return true
 return !!platformSandbox&&(values.state!=='Inherited'||values.allowedDomains.length>0||values.allowWrite.length>0)
}

// fromStored turns the agent's payload into the panel's values.
export function fromStored(stored){
 const list=value=>Array.isArray(value)?[...value]:[]
 return {state:sandboxState(stored?.enabled),allowedDomains:list(stored?.allowedDomains),allowWrite:list(stored?.allowWrite),allow:list(stored?.allow),deny:list(stored?.deny)}
}

// listEditor is one list of the panel: its entries, each removable, and a
// field to add one. Enter adds the field's value rather than submitting the
// settings form.
function listEditor(label,placeholder,onChange){
 const box=document.createElement('div');box.className='sandbox-list'
 const list=document.createElement('ul');list.className='sandbox-entries';list.setAttribute('aria-label',label)
 const input=document.createElement('input');input.type='text';input.placeholder=placeholder;input.setAttribute('aria-label','New entry for '+label)
 const add=document.createElement('button');add.type='button';add.textContent='Add';add.setAttribute('aria-label','Add to '+label)
 const field=document.createElement('div');field.className='repository-picker';field.append(input,add)
 const status=document.createElement('p');status.className='setting-warning sandbox-status';status.setAttribute('role','status')
 box.append(list,field,status)
 let entries=[],disabled=false
 function render(){
  list.replaceChildren()
  if(!entries.length){const empty=document.createElement('li');empty.className='sandbox-empty';empty.textContent='None';list.append(empty)}
  for(const entry of entries){
   const item=document.createElement('li');item.className='sandbox-entry'
   const text=document.createElement('code');text.textContent=entry;text.title=entry
   const remove=document.createElement('button');remove.type='button';remove.textContent='Remove';remove.setAttribute('aria-label','Remove '+entry+' from '+label)
   remove.disabled=disabled
   remove.onclick=()=>{entries=entries.filter(other=>other!==entry);status.textContent='';render();onChange()}
   item.append(text,remove);list.append(item)
  }
  input.disabled=disabled;add.disabled=disabled
 }
 function commit(){
  const result=addEntry(entries,input.value)
  status.textContent=result.error||''
  if(result.error)return
  entries=result.list;input.value='';render();onChange()
 }
 add.onclick=commit
 input.addEventListener('keydown',event=>{if(event.key==='Enter'){event.preventDefault();commit()}})
 return {box,
  get:()=>[...entries],
  set(values){entries=[...values];status.textContent='';input.value='';render()},
  disable(value){disabled=value;render()},
 }
}

// sandboxSettings builds the category. settingRow is the dialog's own row
// builder, so the panel reads as the other categories do. project carries
// the engine fields of the project, for the command preview.
export function sandboxSettings({settingRow,stored,platformSandbox,settingsPath,project}){
 // loaded is what the agent last sent: the save sends it back as the base, so
 // a rule "Always allow" added meanwhile is kept rather than overwritten.
 let values=fromStored(stored),loaded=values
 const changed=()=>{values={...values,allowedDomains:domains.get(),allowWrite:writes.get(),allow:allow.get(),deny:deny.get()};render()}

 const stateGroup=document.createElement('div');stateGroup.className='segmented'
 stateGroup.setAttribute('role','group');stateGroup.setAttribute('aria-label','Claude Code sandbox')
 const stateButtons=SANDBOX_STATES.map(state=>{
  const button=document.createElement('button');button.type='button';button.textContent=state
  button.onclick=()=>{values={...values,state};render()}
  stateGroup.append(button);return button
 })
 const stateRow=settingRow('Claude Code sandbox',null,stateGroup)
 const domains=listEditor('Allowed network domains','registry.npmjs.org',changed)
 const domainsRow=settingRow('Allowed network domains',{stacked:true},domains.box)
 domainsRow.hint.textContent='Hosts a sandboxed command may reach.'
 const writes=listEditor('Extra writable paths','~/.cache/go-build',changed)
 const writesRow=settingRow('Extra writable paths',{stacked:true},writes.box)
 writesRow.hint.textContent='Folders a sandboxed command may write besides the task’s own. A path with ~ is kept as typed; Claude Code expands it.'
 const allow=listEditor('Allow rules','Bash(npm test:*)',changed)
 const allowRow=settingRow('Allow rules',{stacked:true},allow.box)
 allowRow.hint.textContent='Tools Claude Code runs without asking, written as Claude Code writes them. “Always allow” in a conversation adds its rule here. A headless run already approves every tool, so these change nothing for it.'
 const deny=listEditor('Deny rules','Bash(git push:*)',changed)
 const denyRow=settingRow('Deny rules',{stacked:true},deny.box)
 denyRow.hint.textContent='Tools Claude Code never runs, headless runs included.'
 const overlapWarning=document.createElement('p');overlapWarning.className='setting-warning sandbox-overlap'
 denyRow.hint.after(overlapWarning)

 // What these values do, and what they do not reach.
 const scope=document.createElement('p');scope.className='sandbox-scope'
 scope.textContent='These values apply to every Claude Code launch of this project: conversations, terminal sessions and headless runs. They add to Claude Code’s own settings and cannot remove an entry those already hold. A custom command template and other engines do not receive them.'
 const preview=document.createElement('dl');preview.className='command-preview'
 const previewRow=settingRow('Command preview',{stacked:true},scope,preview)

 function render(){
  stateButtons.forEach(button=>{button.setAttribute('aria-pressed',String(values.state===button.textContent));button.disabled=!platformSandbox})
  stateRow.hint.textContent=!platformSandbox?'Claude Code’s sandbox does not run on Windows: only the permission rules below apply.'
   :values.state==='Inherited'?'Inherited · Claude Code’s own settings decide':'Set for this project'
  const overlap=bothLists(values.allow,values.deny)
  overlapWarning.textContent=overlap.length?'In both lists: '+overlap.join(', ')+'. The deny rule wins, as it does in Claude Code.':''
  overlapWarning.hidden=!overlap.length
  preview.replaceChildren()
  const path=launchesGetSettings(values,platformSandbox)?settingsPath||'':''
  for(const line of previewLines(project?.aiProvider||'',project?.aiCommandTemplate||'',project?.aiModel||'',project?.aiCommandTemplateAutonomous||'',path)){
   const term=document.createElement('dt');term.textContent=line.label
   const detail=document.createElement('dd');detail.textContent=line.text
   if(!line.ok)detail.className='command-preview-error'
   preview.append(term,detail)
  }
 }
 function set(next){
  values=fromStored(next);loaded=values
  domains.set(values.allowedDomains);writes.set(values.allowWrite);allow.set(values.allow);deny.set(values.deny)
  domains.disable(!platformSandbox);writes.disable(!platformSandbox)
  render()
 }
 set(stored)
 return {
  sections:[stateRow.section,domainsRow.section,writesRow.section,allowRow.section,denyRow.section,previewRow.section],
  payload:()=>sandboxPayload(values),
  base:()=>sandboxPayload(loaded),
  set,
 }
}
