// The "Sandbox" category of a project's settings (#700) and of the workstation
// settings (#730): what Claude Code sessions are allowed on this workstation.
// The agent stores the values in the workstation settings and hands every
// built-in Claude line of a project the workstation values it is covered by,
// under its own, through --settings (internal/agentconfig/claude_sandbox.go).

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

// resolvedValues is what a launch of a project applies (#730): each list the
// inherited entries then the project's own, each once; the project's state
// unless it is inherited. inherited is null for a project the workstation
// values do not cover.
export function resolvedValues(own,inherited){
 if(!inherited)return own
 const union=(first,second)=>[...new Set([...first,...second])]
 return {
  state:own.state==='Inherited'?inherited.state:own.state,
  allowedDomains:union(inherited.allowedDomains,own.allowedDomains),allowWrite:union(inherited.allowWrite,own.allowWrite),
  allow:union(inherited.allow,own.allow),deny:union(inherited.deny,own.deny),
 }
}

// whitelistSummary says which projects the workstation values apply to: all
// of them while none is checked, a project added later included.
export function whitelistSummary(selected){
 return selected.length?'Applies only to the checked projects. A project added later is not covered until you check it.'
  :'Applies to every project, including the ones you add later. Check projects to limit it to them.'
}

// whitelistEditor is the list of the workstation's projects the workstation
// values apply to (#730), one checkbox each.
export function whitelistEditor({settingRow,projects,selected}){
 const box=document.createElement('div');box.className='sandbox-whitelist';box.setAttribute('role','group');box.setAttribute('aria-label','Projects the Sandbox values apply to')
 const row=settingRow('Applies to',{stacked:true},box)
 let checked=new Set(selected||[])
 function render(){
  box.replaceChildren()
  if(!projects.length){const empty=document.createElement('p');empty.className='sandbox-empty';empty.textContent='No project is added to this workstation yet.';box.append(empty)}
  for(const project of projects){
   const label=document.createElement('label');label.className='checkbox-label sandbox-whitelist-entry'
   const input=document.createElement('input');input.type='checkbox';input.checked=checked.has(project.id)
   input.onchange=()=>{input.checked?checked.add(project.id):checked.delete(project.id);row.hint.textContent=whitelistSummary(get())}
   label.append(input,document.createTextNode(project.name||project.id));box.append(label)
  }
  row.hint.textContent=whitelistSummary(get())
 }
 // The order of the workstation's projects, and only the ones it still has.
 const get=()=>projects.map(project=>project.id).filter(id=>checked.has(id))
 render()
 return {section:row.section,get,set(next){checked=new Set(next||[]);render()}}
}

// listEditor is one list of the panel: its entries, each removable, and a
// field to add one. Enter adds the field's value rather than submitting the
// settings form.
function listEditor(label,placeholder,onChange,action){
 const box=document.createElement('div');box.className='sandbox-list'
 const list=document.createElement('ul');list.className='sandbox-entries';list.setAttribute('aria-label',label)
 const input=document.createElement('input');input.type='text';input.placeholder=placeholder;input.setAttribute('aria-label','New entry for '+label)
 const add=document.createElement('button');add.type='button';add.textContent='Add';add.setAttribute('aria-label','Add to '+label)
 const field=document.createElement('div');field.className='repository-picker';field.append(input,add)
 const status=document.createElement('p');status.className='setting-warning sandbox-status';status.setAttribute('role','status')
 box.append(list,field,status)
 let entries=[],inherited=[],disabled=false
 function render(){
  list.replaceChildren()
  // The workstation entries a project inherits (#730): shown, not removable
  // here, and listed once even when the project repeats one.
  for(const entry of inherited){
   const item=document.createElement('li');item.className='sandbox-entry sandbox-entry-inherited'
   const text=document.createElement('code');text.textContent=entry;text.title=entry
   const tag=document.createElement('span');tag.className='sandbox-origin';tag.textContent='Global';tag.title='From the workstation Sandbox settings'
   item.append(text,tag);list.append(item)
  }
  const own=entries.filter(entry=>!inherited.includes(entry))
  if(!own.length&&!inherited.length){const empty=document.createElement('li');empty.className='sandbox-empty';empty.textContent='None';list.append(empty)}
  for(const entry of entries){
   if(inherited.includes(entry))continue
   const item=document.createElement('li');item.className='sandbox-entry'
   const text=document.createElement('code');text.textContent=entry;text.title=entry
   const remove=document.createElement('button');remove.type='button';remove.textContent='Remove';remove.setAttribute('aria-label','Remove '+entry+' from '+label)
   remove.disabled=disabled
   remove.onclick=()=>{entries=entries.filter(other=>other!==entry);status.textContent='';render();onChange()}
   const buttons=document.createElement('span');buttons.className='sandbox-entry-actions'
   if(action){
    const extra=document.createElement('button');extra.type='button';extra.textContent=action.label;extra.setAttribute('aria-label',action.label+': '+entry)
    extra.disabled=disabled
    extra.onclick=async()=>{extra.disabled=true;try{status.textContent=await action.run(entry)||''}catch(err){status.textContent=err.message}finally{extra.disabled=false}}
    buttons.append(extra)
   }
   buttons.append(remove)
   item.append(text,buttons);list.append(item)
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
  set(values,from=[]){entries=[...values];inherited=[...from];status.textContent='';input.value='';render()},
  disable(value){disabled=value;render()},
 }
}

// sandboxSettings builds the category. settingRow is the dialog's own row
// builder, so the panel reads as the other categories do. project carries
// the engine fields of the project, for the command preview; the workstation
// level (#730) has no project, so no preview. inherited is the workstation
// values a project is covered by, null when it is not; covered says which,
// and onPromote(rule) moves a project allow rule up to the workstation.
export function sandboxSettings({settingRow,stored,platformSandbox,settingsPath,project,workstation=false,inherited=null,covered=true,onPromote}){
 // loaded is what the agent last sent: the save sends it back as the base, so
 // a rule "Always allow" added meanwhile is kept rather than overwritten.
 let values=fromStored(stored),loaded=values,parent=inherited?fromStored(inherited):null,isCovered=covered
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
 // A project allow rule can move up to the workstation (#730).
 const promote=!workstation&&onPromote?{label:'Move to global',run:async rule=>{
  const fresh=await onPromote(rule)
  if(fresh)set(fresh.claudeSandbox,fresh.claudeSandboxGlobal,fresh.claudeSandboxCovered!==false)
  return rule+' moved to the workstation Sandbox settings.'
 }}:null
 const allow=listEditor('Allow rules','Bash(npm test:*)',changed,promote)
 const allowRow=settingRow('Allow rules',{stacked:true},allow.box)
 allowRow.hint.textContent=workstation?'Tools Claude Code runs without asking, in every covered project. A headless run already approves every tool, so these change nothing for it.'
  :'Tools Claude Code runs without asking, written as Claude Code writes them. “Always allow” in a conversation adds its rule here. A headless run already approves every tool, so these change nothing for it.'
 const deny=listEditor('Deny rules','Bash(git push:*)',changed)
 const denyRow=settingRow('Deny rules',{stacked:true},deny.box)
 denyRow.hint.textContent='Tools Claude Code never runs, headless runs included.'
 const overlapWarning=document.createElement('p');overlapWarning.className='setting-warning sandbox-overlap'
 denyRow.hint.after(overlapWarning)

 // What these values do, and what they do not reach.
 const scope=document.createElement('p');scope.className='sandbox-scope'
 const preview=document.createElement('dl');preview.className='command-preview'
 const previewRow=workstation?settingRow('Scope',{stacked:true},scope):settingRow('Command preview',{stacked:true},scope,preview)
 // Whether the workstation values reach this project (#730).
 const coverage=document.createElement('p');coverage.className='sandbox-scope sandbox-coverage'
 const coverageRow=settingRow('Workstation values',{stacked:true},coverage)

 function render(){
  stateButtons.forEach(button=>{button.setAttribute('aria-pressed',String(values.state===button.textContent));button.disabled=!platformSandbox})
  stateRow.hint.textContent=!platformSandbox?'Claude Code’s sandbox does not run on Windows: only the permission rules below apply.'
   :values.state!=='Inherited'?(workstation?'Set for every covered project':'Set for this project')
   :parent&&parent.state!=='Inherited'?'Inherited from the workstation Sandbox settings · '+parent.state
   :'Inherited · Claude Code’s own settings decide'
  const resolved=resolvedValues(values,parent)
  const overlap=bothLists(resolved.allow,resolved.deny)
  overlapWarning.textContent=overlap.length?'In both lists: '+overlap.join(', ')+'. The deny rule wins, as it does in Claude Code.':''
  overlapWarning.hidden=!overlap.length
  scope.textContent=workstation?'These values apply to every Claude Code launch of the covered projects: conversations, terminal sessions and headless runs. A project’s own Sandbox values add to them, and its sandbox state overrides this one. They add to Claude Code’s own settings and cannot remove an entry those already hold. A custom command template and other engines do not receive them.'
   :'These values apply to every Claude Code launch of this project: conversations, terminal sessions and headless runs. They add to Claude Code’s own settings and cannot remove an entry those already hold. A custom command template and other engines do not receive them.'
  coverage.textContent=isCovered?'The workstation Sandbox values apply to this project. Entries marked Global come from them and are changed in the workstation settings.'
   :'The workstation Sandbox values do not apply to this project: it is not checked in the workstation Sandbox settings.'
  if(workstation)return
  preview.replaceChildren()
  const path=launchesGetSettings(resolved,platformSandbox)?settingsPath||'':''
  for(const line of previewLines(project?.aiProvider||'',project?.aiCommandTemplate||'',project?.aiModel||'',project?.aiCommandTemplateAutonomous||'',path)){
   const term=document.createElement('dt');term.textContent=line.label
   const detail=document.createElement('dd');detail.textContent=line.text
   if(!line.ok)detail.className='command-preview-error'
   preview.append(term,detail)
  }
 }
 function set(next,nextInherited=inherited,nextCovered=isCovered){
  values=fromStored(next);loaded=values
  inherited=nextInherited;parent=inherited?fromStored(inherited):null;isCovered=nextCovered
  const from=key=>parent?parent[key]:[]
  domains.set(values.allowedDomains,from('allowedDomains'));writes.set(values.allowWrite,from('allowWrite'))
  allow.set(values.allow,from('allow'));deny.set(values.deny,from('deny'))
  domains.disable(!platformSandbox);writes.disable(!platformSandbox)
  render()
 }
 set(stored)
 return {
  sections:[...(workstation?[]:[coverageRow.section]),stateRow.section,domainsRow.section,writesRow.section,allowRow.section,denyRow.section,previewRow.section],
  payload:()=>sandboxPayload(values),
  base:()=>sandboxPayload(loaded),
  set,
 }
}
