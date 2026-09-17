// The command line a launch would run, in each execution mode.
//
// The agent builds the real thing (internal/agent/agent_config.go); this mirrors
// it so the local CLI override can show both modes before anything runs. The web
// settings carry the same mirror in web/src/lib/commandTemplate.ts: the three
// must say the same thing.

// How a template says which words depend on the mode: {mode:AUTONOMOUS|INTERACTIVE}.
export const TEMPLATE_MODE_PLACEHOLDER='{mode:'

// Stand-in for the instructions, kept literal so the preview stays readable.
const PROMPT="'{prompt}'"

// The optional slot a command template may carry for the resolved model.
const MODEL_PLACEHOLDER='{model}'

// Providers whose model is passed as --model; the others take none.
const MODEL_FLAG_PROVIDERS=new Set(['claude','codex','gemini','cursor'])

// A template owns the mode only by declaring the placeholder. Without it the
// template can only run what its author wrote, which is why the agent refuses an
// autonomous launch rather than reinterpreting the command silently.
export function templateCarriesMode(template){return String(template||'').includes(TEMPLATE_MODE_PLACEHOLDER)}

// resolveTemplateMode picks one side of every mode placeholder, autonomous left.
export function resolveTemplateMode(template,autonomous){
 let rest=String(template||''),out=''
 for(;;){
  const start=rest.indexOf(TEMPLATE_MODE_PLACEHOLDER)
  if(start<0)return out+rest
  const end=rest.indexOf('}',start)
  if(end<0)return out+rest
  const body=rest.slice(start+TEMPLATE_MODE_PLACEHOLDER.length,end)
  const bar=body.indexOf('|')
  const chosen=autonomous?(bar<0?body:body.slice(0,bar)):(bar<0?'':body.slice(bar+1))
  out+=rest.slice(0,start)+chosen
  rest=rest.slice(end+1)
 }
}

// modelArgs is the --model pair to splice in, empty when the provider takes no
// model or none is configured.
export function modelArgs(provider,model){
 const resolved=String(model||'').trim()
 if(!resolved||!MODEL_FLAG_PROVIDERS.has(String(provider||'').trim().toLowerCase()))return []
 return ['--model',resolved]
}

// shellQuote wraps a value so a shell reads it literally, as the agent does.
function shellQuote(value){return "'"+String(value).replace(/'/g,"'\\''")+"'"}

const isBlank=ch=>ch===' '||ch==='\t'

// tokenAround returns the bounds of the blank-delimited token covering `at`,
// quotes included.
function tokenAround(s,at){
 let start=at,end=at
 while(start>0&&!isBlank(s[start-1]))start--
 while(end<s.length&&!isBlank(s[end]))end++
 return [start,end]
}

// precedingToken returns the bounds of the token before `start`, or null.
function precedingToken(s,start){
 let end=start
 while(end>0&&isBlank(s[end-1]))end--
 if(end===0)return null
 let begin=end
 while(begin>0&&!isBlank(s[begin-1]))begin--
 return [begin,end]
}

// cutRun removes a run and the blanks it would leave doubled around it.
function cutRun(s,start,end){
 while(start>0&&isBlank(s[start-1]))start--
 if(start===0){while(end<s.length&&isBlank(s[end]))end++}
 return s.slice(0,start)+s.slice(end)
}

// dropModelSlot removes every {model} slot a template has nothing to put in,
// along with the option the slot is the value of: a flag left with nothing
// behind it consumes the next word instead of disappearing. Mirrors
// agentconfig's dropModelSlots, down to the token that glues the two markers
// together: the prompt the command exists to carry is never taken with it.
export function dropModelSlot(template){
 for(;;){
  const at=template.indexOf(MODEL_PLACEHOLDER)
  if(at<0)return template.trim()
  let [start,end]=tokenAround(template,at)
  if(template.slice(start,end).includes('{prompt}')){
   template=cutRun(template,at,at+MODEL_PLACEHOLDER.length)
   continue
  }
  if(!template.slice(start,end).startsWith('-')){
   const previous=precedingToken(template,start)
   if(previous&&template[previous[0]]==='-'&&!template.slice(previous[0],previous[1]).includes('{prompt}')){
    start=previous[0]
   }
  }
  template=cutRun(template,start,end)
 }
}

// expandModel fills {model} the way the agent does: quoting belongs to the
// template, so the slot is quoted only where it sits outside quotes.
function expandModel(template,model){
 const value=String(model||'').trim()
 if(value==='')return dropModelSlot(template)
 let out='',quote=''
 for(let i=0;i<template.length;){
  if(template.startsWith(MODEL_PLACEHOLDER,i)){
   out+=quote==="'"?value.replace(/'/g,"'\\''"):quote==='"'?'"'+shellQuote(value)+'"':shellQuote(value)
   i+=MODEL_PLACEHOLDER.length
   continue
  }
  const ch=template[i]
  if(ch==="'"||ch==='"'){if(quote==='')quote=ch;else if(quote===ch)quote=''}
  out+=ch
  i++
 }
 return out
}

// A named provider given a template without {prompt} falls back to its own
// command: legacy rows hold a bare CLI name there. A custom provider cannot.
function usesTemplate(provider,template){
 return template!==''&&(provider==='custom'||template.includes('{prompt}'))
}

function words(...parts){return parts.filter(part=>part!=='').join(' ')}

// commandPreview returns {command,error}: an empty command carries the reason
// that mode cannot run. An autonomous run also carries the provider's
// non-interactive approval flag, because nobody is there to answer a prompt.
//
// autonomousTemplate is the command written for headless launches. It answers
// for itself and needs no {mode:...} marker: its author wrote it for that mode.
export function commandPreview(provider,template,model,autonomous,autonomousTemplate=''){
 const cli=String(provider||'').trim().toLowerCase()
 const dedicated=String(autonomousTemplate||'').trim()
 if(autonomous&&usesTemplate(cli,dedicated)){
  return {command:expandModel(resolveTemplateMode(dedicated,true),model).replace(/ {2,}/g,' ').trim()}
 }
 const trimmed=String(template||'').trim()
 if(usesTemplate(cli,trimmed)){
  if(autonomous&&!templateCarriesMode(trimmed)){
   return {command:'',error:'This command decides the mode itself. Set an autonomous command, or add a {mode:AUTONOMOUS|INTERACTIVE} placeholder to this one.'}
  }
  return {command:expandModel(resolveTemplateMode(trimmed,autonomous),model).replace(/ {2,}/g,' ').trim()}
 }
 const flag=modelArgs(cli,model).join(' ')
 if(autonomous){
  switch(cli){
   case 'claude':return {command:words('claude','-p','--permission-mode','bypassPermissions',flag,PROMPT)}
   case 'codex':return {command:words('codex','exec',flag,PROMPT)}
   case 'vibe':return {command:'vibe -p --auto-approve '+PROMPT}
   default:return {command:'',error:(cli||'This provider')+' has no attested headless mode. Run interactively, or write a template carrying {mode:AUTONOMOUS|INTERACTIVE}.'}
  }
 }
 switch(cli){
  case 'agy':return {command:'agy -i '+PROMPT}
  case 'claude':case 'codex':case 'gemini':return {command:words(cli,flag,PROMPT)}
  case 'vibe':return {command:'vibe -p '+PROMPT}
  case 'cursor':return {command:words('cursor','agent',flag,PROMPT)}
  default:return {command:'',error:'Unsupported provider '+(cli||'(none)')+': configure an AI command template.'}
 }
}

// previewLines renders both modes as the label/text pairs the dialog shows.
export function previewLines(provider,template,model,autonomousTemplate=''){
 return [['Interactive',false],['Autonomous',true]].map(([label,autonomous])=>{
  const {command,error}=commandPreview(provider,template,model,autonomous,autonomousTemplate)
  return {label,text:command||error,ok:Boolean(command)}
 })
}
