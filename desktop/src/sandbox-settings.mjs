// The "Sandbox" category of a project's settings (#700) and of the workstation
// settings (#730): what Claude Code sessions are allowed on this workstation.
// The agent stores the values in the workstation settings and hands every
// built-in Claude line of a project the workstation values it is covered by,
// under its own, through --settings (internal/agentconfig/claude_sandbox.go).

import { previewLines } from './command-preview.mjs'
import { PRESETS, RECOMMENDED, applyPreset, listsEmpty, presetApplied, presetHasEntries, removePreset } from './claude-presets.mjs'

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
  ...(values.autoAllowBashIfSandboxed!==undefined?{autoAllowBashIfSandboxed:values.autoAllowBashIfSandboxed}:{}),
  ...(values.allowUnsandboxedCommands!==undefined?{allowUnsandboxedCommands:values.allowUnsandboxedCommands}:{}),
  ...(values.additionalDirectories!==undefined?{additionalDirectories:[...values.additionalDirectories]}:{}),
  allowedDomains:[...values.allowedDomains],allowWrite:[...values.allowWrite],
  allow:[...values.allow],deny:[...values.deny],
 }
}

// launchesGetSettings says whether a launch of the project receives a settings
// file: the agent writes none for values that state nothing this platform
// applies, and Windows applies only the rules.
export function launchesGetSettings(values,platformSandbox){
 if(values.allow.length||values.deny.length||values.additionalDirectories?.length)return true
 return !!platformSandbox&&(values.autoAllowBashIfSandboxed!=null||values.allowUnsandboxedCommands!=null||values.state!=='Inherited'||values.allowedDomains.length>0||values.allowWrite.length>0)
}

// fromStored turns the agent's payload into the panel's values.
export function fromStored(stored){
 const list=value=>Array.isArray(value)?[...value]:[]
 return {...(stored?.autoAllowBashIfSandboxed!=null?{autoAllowBashIfSandboxed:stored.autoAllowBashIfSandboxed}:{}),...(stored?.allowUnsandboxedCommands!=null?{allowUnsandboxedCommands:stored.allowUnsandboxedCommands}:{}),...(stored?.additionalDirectories?.length?{additionalDirectories:list(stored.additionalDirectories)}:{}),state:sandboxState(stored?.enabled),allowedDomains:list(stored?.allowedDomains),allowWrite:list(stored?.allowWrite),allow:list(stored?.allow),deny:list(stored?.deny)}
}

// resolvedValues is what a launch of a project applies (#730): each list the
// inherited entries then the project's own, each once; the project's state
// unless it is inherited. inherited is null for a project the workstation
// values do not cover.
export function resolvedValues(own,inherited){
 if(!inherited)return own
 const union=(first,second)=>[...new Set([...first,...second])]
 return {
  ...Object.fromEntries(['autoAllowBashIfSandboxed','allowUnsandboxedCommands'].filter(key=>own[key]!=null||inherited[key]!=null).map(key=>[key,own[key]??inherited[key]])),
  ...((own.additionalDirectories?.length||inherited.additionalDirectories?.length)?{additionalDirectories:union(inherited.additionalDirectories||[],own.additionalDirectories||[])}:{}),
  state:own.state==='Inherited'?inherited.state:own.state,
  allowedDomains:union(inherited.allowedDomains,own.allowedDomains),allowWrite:union(inherited.allowWrite,own.allowWrite),
  allow:union(inherited.allow,own.allow),deny:union(inherited.deny,own.deny),
 }
}

// whitelistSummary says which projects the workstation values apply to. An
// empty list is stored for "every project", a project added later included,
// so "only these" with nothing checked still covers every project.
export function whitelistSummary(selected,only=selected.length>0){
 if(!only)return 'Every project, including the ones you add later.'
 return selected.length?'Applies only to the checked projects. A project added later is not covered until you check it.'
  :'Check at least one project: with none checked, the settings still apply to every project.'
}

// whitelistEditor picks the projects the workstation values apply to (#730):
// every project, or only the checked ones, whose checkboxes show in that mode.
export function whitelistEditor({settingRow,projects,selected}){
 const box=document.createElement('div');box.className='sandbox-whitelist'
 const modes=document.createElement('div');modes.className='sandbox-whitelist-modes';modes.setAttribute('role','radiogroup');modes.setAttribute('aria-label','Projects the Claude settings cover')
 const radio=text=>{
  const label=document.createElement('label');label.className='checkbox-label sandbox-whitelist-entry'
  const input=document.createElement('input');input.type='radio';input.name='claude-settings-coverage'
  input.onchange=()=>{only=input===onlyInput;render()}
  label.append(input,document.createTextNode(text));modes.append(label);return input
 }
 const allInput=radio('All projects'),onlyInput=radio('Only these projects')
 const list=document.createElement('div');list.className='sandbox-whitelist-projects';list.setAttribute('role','group');list.setAttribute('aria-label','Projects the Claude settings apply to')
 box.append(modes,list)
 const row=settingRow('Applies to',{stacked:true},box)
 let checked=new Set(selected||[]),only=checked.size>0
 function render(){
  allInput.checked=!only;onlyInput.checked=only;list.hidden=!only
  list.replaceChildren()
  if(!projects.length){const empty=document.createElement('p');empty.className='sandbox-empty';empty.textContent='No project is added to this workstation yet.';list.append(empty)}
  for(const project of projects){
   const label=document.createElement('label');label.className='checkbox-label sandbox-whitelist-entry'
   const input=document.createElement('input');input.type='checkbox';input.checked=checked.has(project.id)
   input.onchange=()=>{input.checked?checked.add(project.id):checked.delete(project.id);row.hint.textContent=whitelistSummary(get(),only)}
   label.append(input,document.createTextNode(project.name||project.id));list.append(label)
  }
  row.hint.textContent=whitelistSummary(get(),only)
 }
 // The order of the workstation's projects, and only the ones it still has;
 // nothing in "All projects" mode, the checks kept for a switch back.
 const get=()=>only?projects.map(project=>project.id).filter(id=>checked.has(id)):[]
 render()
 return {section:row.section,get,set(next){checked=new Set(next||[]);only=checked.size>0;render()}}
}

// listEditor is one list of the panel: its entries, each removable, and a
// field to add one. Enter adds the field's value rather than submitting the
// settings form.
function listEditor(label,placeholder,onChange,action,kind=''){
 const box=document.createElement('div');box.className='sandbox-list'+(kind?' sandbox-list-'+kind:'')
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
   const tag=document.createElement('span');tag.className='sandbox-origin';tag.textContent='Global';tag.title='From the workstation Claude settings';item.title=entry+' · from the workstation Claude settings'
   item.append(text,tag);list.append(item)
  }
  const own=entries.filter(entry=>!inherited.includes(entry))
  if(!own.length&&!inherited.length){const empty=document.createElement('li');empty.className='sandbox-empty';empty.textContent='None';list.append(empty)}
  for(const entry of entries){
   if(inherited.includes(entry))continue
   const item=document.createElement('li');item.className='sandbox-entry'
   const text=document.createElement('code');text.textContent=entry;text.title=entry
   // Each entry is a chip: the list stays a few lines high however long it is.
   const remove=document.createElement('button');remove.type='button';remove.className='sandbox-chip-button';remove.textContent='×';remove.title='Remove';remove.setAttribute('aria-label','Remove '+entry+' from '+label)
   remove.disabled=disabled
   remove.onclick=()=>{entries=entries.filter(other=>other!==entry);status.textContent='';render();onChange()}
   const buttons=document.createElement('span');buttons.className='sandbox-entry-actions'
   if(action){
    const extra=document.createElement('button');extra.type='button';extra.className='sandbox-chip-button';extra.textContent=action.icon;extra.title=action.label;extra.setAttribute('aria-label',action.label+': '+entry)
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
 // Policy fields stay inherited until the owner explicitly changes them.
 const policyBox=document.createElement('div')
 const autonomy=document.createElement('button');autonomy.type='button';autonomy.textContent='Autonomy in sandbox'
 autonomy.onclick=()=>{values={...values,state:'On',autoAllowBashIfSandboxed:true,allowUnsandboxedCommands:false};render()}
 const policyControls=['autoAllowBashIfSandboxed','allowUnsandboxedCommands'].map((key,index)=>{
  const label=document.createElement('label');label.textContent=index===0?'Sandboxed commands':'Unsandboxed retries'
  const select=document.createElement('select');select.setAttribute('aria-label',label.textContent)
  for(const [value,text] of [['','Inherited'],['true',index===0?'Run automatically':'Allow'],['false',index===0?'Use permission rules':'Block']]){
   const option=document.createElement('option');option.value=value;option.textContent=text;select.append(option)
  }
  select.onchange=()=>{values={...values,[key]:select.value===''?null:select.value==='true'};render()}
  label.append(select);policyBox.append(label);return {key,select}
 })
 policyBox.prepend(autonomy)
 const policyRow=settingRow('Execution policy',{stacked:true},policyBox)
 policyRow.hint.textContent='Autonomy runs shell commands inside the sandbox and blocks unsandboxed retries. Configure toolchain caches and network domains below. File and MCP tools follow their own permissions. Save and start the next message to apply.'
 const folders=listEditor('Approved folders','/path/to/shared-project',()=>{values={...values,additionalDirectories:folders.get()};render()})
 const foldersRow=settingRow('Approved folders',{stacked:true},folders.box)
 foldersRow.hint.textContent='Additional folders Claude may access. “Always allow” directory approvals are kept here for this project.'
 const domains=listEditor('Allowed network domains','registry.npmjs.org',changed)
 const domainsRow=settingRow('Allowed network domains',{stacked:true},domains.box)
 domainsRow.hint.textContent='Hosts a sandboxed command may reach.'
 const writes=listEditor('Extra writable paths','~/.cache/go-build',changed)
 const writesRow=settingRow('Extra writable paths',{stacked:true},writes.box)
 writesRow.hint.textContent='Folders a sandboxed command may write besides the task’s own. A path with ~ is kept as typed; Claude Code expands it.'
 // A project allow rule can move up to the workstation (#730).
 const promote=!workstation&&onPromote?{label:'Move to global',icon:'↑',run:async rule=>{
  const fresh=await onPromote(rule)
  if(fresh)set(fresh.claudeSandbox,fresh.claudeSandboxGlobal,fresh.claudeSandboxCovered!==false)
  return rule+' moved to the workstation Claude settings.'
 }}:null
 const allow=listEditor('Allow rules','Bash(npm test:*)',changed,promote,'allow')
 const allowRow=settingRow('Allow rules',{stacked:true},allow.box)
 allowRow.hint.textContent=workstation?'Tools Claude Code runs without asking, in every covered project. A headless run already approves every tool, so these change nothing for it.'
  :'Tools Claude Code runs without asking, written as Claude Code writes them. “Always allow” in a conversation adds its rule here. A headless run already approves every tool, so these change nothing for it.'
 const deny=listEditor('Deny rules','Bash(git push:*)',changed,null,'deny')
 const denyRow=settingRow('Deny rules',{stacked:true},deny.box)
 denyRow.hint.textContent='Tools Claude Code never runs, headless runs included.'
 const overlapWarning=document.createElement('p');overlapWarning.className='setting-warning sandbox-overlap'
 denyRow.hint.after(overlapWarning)

 // The presets of the workstation (#745): applying one copies its entries
 // into the lists, which the save then keeps like any other edit.
 const presetsBox=document.createElement('div');presetsBox.className='claude-presets'
 const recommended=document.createElement('div');recommended.className='claude-presets-recommended'
 const recommendedText=document.createElement('span');recommendedText.textContent='Nothing is set yet. Start with Common and Dangerous actions.'
 const recommendedButton=document.createElement('button');recommendedButton.type='button';recommendedButton.textContent='Apply recommended'
 recommendedButton.onclick=()=>showValues(RECOMMENDED.reduce((next,id)=>applyPreset(next,PRESETS.find(preset=>preset.id===id),platformSandbox),values))
 recommended.append(recommendedText,recommendedButton)
 const presetItems=workstation?PRESETS.map(presetItem):[]
 presetsBox.append(recommended,...presetItems.map(item=>item.element))
 const presetsRow=workstation?settingRow('Presets',{stacked:true},presetsBox):null
 if(presetsRow)presetsRow.hint.textContent='Ready-made entries for the lists below, per toolchain. Applying one adds its missing entries, each removable like any other; save to keep them.'
 // presetItem is one preset on a single line: its name, what it allows and
 // what it denies at a glance, and its button. The description and the
 // entries open on demand.
 function presetItem(preset){
  const element=document.createElement('div');element.className='claude-preset'
  const head=document.createElement('div');head.className='claude-preset-head'
  const name=document.createElement('strong');name.textContent=preset.name;name.title=preset.description
  const kinds=document.createElement('span');kinds.className='claude-preset-kinds'
  for(const [list,kind,verb] of [['allow','allow','Allows '],['deny','deny','Denies ']]){
   if(!preset[list].length)continue
   const tag=document.createElement('span');tag.className='claude-preset-kind claude-preset-kind-'+kind;tag.textContent=verb+preset[list].length
   kinds.append(tag)
  }
  const reach=platformSandbox?[[preset.allowedDomains.length,'domain'],[preset.allowWrite.length,'path']].filter(([count])=>count).map(([count,noun])=>count+' '+noun+(count>1?'s':'')):[]
  if(reach.length){const tag=document.createElement('span');tag.className='claude-preset-kind';tag.textContent=reach.join(' · ');kinds.append(tag)}
  const badge=document.createElement('span');badge.className='sandbox-origin';badge.textContent='Applied'
  const toggle=document.createElement('button');toggle.type='button'
  toggle.onclick=()=>showValues(presetApplied(values,preset,platformSandbox)?removePreset(values,preset,platformSandbox):applyPreset(values,preset,platformSandbox))
  // The disclosure sits on the preset's own line rather than below it.
  const entries=document.createElement('div');entries.className='claude-preset-entries';entries.hidden=true
  const expand=document.createElement('button');expand.type='button';expand.className='claude-preset-expand';expand.textContent='▸'
  expand.setAttribute('aria-label','Entries of '+preset.name);expand.setAttribute('aria-expanded','false')
  expand.onclick=()=>{entries.hidden=!entries.hidden;expand.setAttribute('aria-expanded',String(!entries.hidden));expand.textContent=entries.hidden?'▸':'▾';element.classList.toggle('claude-preset-open',!entries.hidden)}
  head.append(expand,name,kinds,badge,toggle)
  const description=document.createElement('p');description.className='claude-preset-description';description.textContent=preset.description
  entries.append(description)
  for(const [list,label,kind] of [['allow','Allow rules','allow'],['deny','Deny rules','deny'],['allowedDomains','Allowed network domains','net'],['allowWrite','Extra writable paths','write']]){
   if(!preset[list].length)continue
   const group=document.createElement('div');group.className='claude-preset-group'
   const title=document.createElement('span');title.className='claude-preset-group-label';title.textContent=label
   const chips=document.createElement('span');chips.className='claude-preset-chips claude-preset-chips-'+kind
   for(const entry of preset[list]){const code=document.createElement('code');code.textContent=entry;chips.append(code)}
   group.append(title,chips);entries.append(group)
  }
  if(!platformSandbox&&(preset.allowedDomains.length||preset.allowWrite.length)){
   const note=document.createElement('p');note.textContent='Its domains and writable paths do not apply on Windows: only its rules are added.'
   entries.append(note)
  }
  element.append(head,entries)
  return {preset,element,badge,toggle}
 }
 // showValues puts values computed outside the editors back into them; the
 // workstation inherits nothing, so no entry is marked Global.
 function showValues(next){
  values=next
  domains.set(values.allowedDomains);writes.set(values.allowWrite);allow.set(values.allow);deny.set(values.deny)
  render()
 }

 // What these values do, and what they do not reach.
 const scope=document.createElement('p');scope.className='sandbox-scope'
 const preview=document.createElement('dl');preview.className='command-preview'
 const previewRow=workstation?settingRow('Scope',{stacked:true},scope):settingRow('Command preview',{stacked:true},scope,preview)
 // Whether the workstation values reach this project (#730).
 const coverage=document.createElement('p');coverage.className='sandbox-scope sandbox-coverage'
 const coverageRow=settingRow('Workstation values',{stacked:true},coverage)

 function render(){
  autonomy.disabled=!platformSandbox
  policyControls.forEach(({key,select})=>{select.value=values[key]==null?'':String(values[key]);select.disabled=!platformSandbox;select.title=parent?.[key]==null?'Claude Code settings decide':'Inherited value: '+parent[key]})
  stateButtons.forEach(button=>{button.setAttribute('aria-pressed',String(values.state===button.textContent));button.disabled=!platformSandbox})
  stateRow.hint.textContent=!platformSandbox?'Claude Code’s sandbox does not run on Windows: only the permission rules below apply.'
   :values.state!=='Inherited'?(workstation?'Set for every covered project':'Set for this project')
   :parent&&parent.state!=='Inherited'?'Inherited from the workstation Claude settings · '+parent.state
   :'Inherited · Claude Code’s own settings decide'
  const resolved=resolvedValues(values,parent)
  const overlap=bothLists(resolved.allow,resolved.deny)
  overlapWarning.textContent=overlap.length?'In both lists: '+overlap.join(', ')+'. The deny rule wins, as it does in Claude Code.':''
  overlapWarning.hidden=!overlap.length
  scope.textContent=workstation?'These values apply to every Claude Code launch of the covered projects: conversations, terminal sessions and headless runs. A project’s own Claude settings add to them, and its sandbox state overrides this one. They add to Claude Code’s own settings and cannot remove an entry those already hold. A custom command template and other engines do not receive them.'
   :'These values apply to every Claude Code launch of this project: conversations, terminal sessions and headless runs. They add to Claude Code’s own settings and cannot remove an entry those already hold. A custom command template and other engines do not receive them.'
  coverage.textContent=isCovered?'The workstation Claude settings apply to this project. Entries marked Global come from them and are changed in the workstation settings.'
   :'The workstation Claude settings do not apply to this project: it is not checked in the workstation Claude settings.'
  for(const item of presetItems){
   const applied=presetApplied(values,item.preset,platformSandbox)
   item.badge.hidden=!applied
   item.toggle.textContent=applied?'Remove':'Apply'
   item.element.classList.toggle('claude-preset-applied',applied)
   item.toggle.setAttribute('aria-label',(applied?'Remove preset ':'Apply preset ')+item.preset.name)
   item.toggle.hidden=!presetHasEntries(item.preset,platformSandbox)
  }
  recommended.hidden=!listsEmpty(values)
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
  folders.set(values.additionalDirectories||[],parent?.additionalDirectories||[])
  domains.set(values.allowedDomains,from('allowedDomains'));writes.set(values.allowWrite,from('allowWrite'))
  allow.set(values.allow,from('allow'));deny.set(values.deny,from('deny'))
  domains.disable(!platformSandbox);writes.disable(!platformSandbox)
  render()
 }
 set(stored)
 return {
  sections:[...(workstation?[presetsRow.section]:[coverageRow.section]),stateRow.section,policyRow.section,foldersRow.section,domainsRow.section,writesRow.section,allowRow.section,denyRow.section,previewRow.section],
  payload:()=>sandboxPayload(values),
  base:()=>sandboxPayload(loaded),
  set,
 }
}
