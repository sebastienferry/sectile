// Helpers shared by the "Execution defaults" panel and the project dialog.
// The agent owns every execution setting (#305): these only shape what it
// answers and what the forms send back to it.

// The providers the desktop offers. A stored provider outside this list (a
// custom command seeded from the server, say) is added to the select rather
// than silently replaced.
export const PROVIDERS=[
 {id:'agy',label:'AGY CLI (Google Antigravity)'},
 {id:'claude',label:'Claude Code CLI'},
 {id:'codex',label:'Codex CLI'},
]
export const DEFAULT_PROVIDER='agy'
export const SETUP_PROVIDERS=['claude','codex','agy']

// A skill command is one word, optionally led by a slash (mirrors the agent's
// skillCommandName rule).
const SKILL_COMMAND=/^\/?[A-Za-z0-9][A-Za-z0-9_-]*$/
export function validSkillCommand(value){
 const trimmed=String(value||'').trim()
 return trimmed===''||SKILL_COMMAND.test(trimmed)
}

// The flat keys and override flags an agent older than #305 answers with, used
// when its answer carries no `fields`. The engine fields left the project
// settings with the engine catalogue (#510): a project picks a default engine.
const LEGACY={
 terminal:['terminal','terminalOverride'],
 useWorktrees:['useWorktrees','worktreeOverride'],
 parallelism:['parallelism',null],
}
const EMPTY={defaultEngine:'',setupProviders:[],skillCommands:{}}

// projectFields returns, per execution field, {value, inherited, source}.
// source is "project", "workstation" or "default".
export function projectFields(info){
 const out={}
 const given=info?.fields||{}
 for(const name of [...Object.keys(LEGACY),...Object.keys(EMPTY)]){
  const field=given[name]
  if(field&&typeof field==='object'&&'source' in field){out[name]={value:field.value,inherited:field.inherited,source:field.source};continue}
  const legacy=LEGACY[name]
  if(legacy){
   const value=info?.[legacy[0]]
   out[name]={value,inherited:undefined,source:legacy[1]&&info?.[legacy[1]]?'project':'workstation'}
  }else out[name]={value:EMPTY[name],inherited:EMPTY[name],source:'default'}
 }
 return out
}

// ownEntries is what a project states itself in a map field: the effective
// entries that differ from the inherited ones.
export function ownEntries(field){
 if(!field||field.source!=='project')return {}
 const value=field.value||{},inherited=field.inherited||{}
 const out={}
 for(const [key,entry] of Object.entries(value))if(entry&&entry!==inherited[key])out[key]=entry
 return out
}

// compact drops blank keys and values; an emptied map is sent empty, so the
// agent removes it from the file.
export function compact(map){
 const out={}
 for(const [key,value] of Object.entries(map||{})){
  const k=String(key).trim(),v=String(value??'').trim()
  if(k&&v)out[k]=v
 }
 return out
}

// parseModelList reads a comma or newline separated list, dropping blanks and
// repeats.
export function parseModelList(text){
 const out=[]
 for(const item of String(text||'').split(/[,\n]/)){
  const model=item.trim()
  if(model&&!out.includes(model))out.push(model)
 }
 return out
}

// describe renders a field value for a hint.
export function describe(value,empty='(none)'){
 if(value===undefined||value===null||value==='')return empty
 if(typeof value==='boolean')return value?'Yes':'No'
 if(Array.isArray(value))return value.length?value.join(', '):'None'
 if(typeof value==='object'){
  const entries=Object.entries(value)
  return entries.length?entries.map(([key,entry])=>key+': '+entry).join(', '):empty
 }
 return String(value)
}

// sourceHint says where a project field comes from and what it would be
// without the project's statement.
export function sourceHint(source,inherited,empty){
 const shown=describe(inherited,empty)
 if(source==='project')return inherited===undefined?'Set for this project':'Set for this project · Inherited: '+shown
 if(source==='default')return 'Inherited default'+(inherited===undefined?'':': '+shown)
 return 'Inherited from workstation'+(inherited===undefined?'':': '+shown)
}

// ipcMessage strips the wrapper Electron puts around an error thrown in the
// main process, leaving the agent's own reason.
export function ipcMessage(err){
 return String(err?.message||err).replace(/^Error invoking remote method '[^']*': (Error: )?/,'')
}

// agentUnreachable tells a stopped agent from a refusal.
export function agentUnreachable(err){
 const text=ipcMessage(err)
 return /Connect to the local agent first|fetch failed|ECONNREFUSED|Agent disconnected/i.test(text)
}

// workstationPayload builds the full `defaults` object PUT to the agent from
// the panel's state. A field left unset is absent, so it inherits.
export function workstationPayload(state){
 const out={}
 // The engine lives in the engine catalogue (#510): the defaults carry none.
 for(const key of ['terminal','editorCommand']){
  const value=String(state[key]??'').trim()
  if(value)out[key]=value
 }
 if(typeof state.useWorktrees==='boolean')out.useWorktrees=state.useWorktrees
 if(Number.isInteger(state.parallelism)&&state.parallelism!==0)out.parallelism=state.parallelism
 out.setupProviders=Array.isArray(state.setupProviders)?[...state.setupProviders]:null
 const lists={}
 for(const [provider,models] of Object.entries(state.aiProviderModels||{}))if(Array.isArray(models))lists[provider]=[...models]
 if(Object.keys(lists).length)out.aiProviderModels=lists
 return out
}
