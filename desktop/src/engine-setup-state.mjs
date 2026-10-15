export const SETUP_PROVIDERS=['claude','codex','agy']
export const SETUP_STEPS=['skills','mcp','permissions','sdd']
export function supportedEngines(view){return (view?.catalogue||[]).filter(engine=>SETUP_PROVIDERS.includes(engine.provider))}
export function readableSetup(view,provider){
 return view?.schemaVersion===1&&view.provider===provider&&SETUP_STEPS.every(step=>['pending','success','skipped','unknown','failed'].includes(view.steps?.[step]?.status))
}
export function setupIncomplete(view){
 // An unavailable read is never evidence that configuration is incomplete.
 return readableSetup(view,view?.provider)&&!!view.server&&SETUP_STEPS.some(step=>['pending','failed'].includes(view.steps[step].status))
}
export function setupReducer(state,event){
 if(event.type==='select')return {provider:event.provider,generation:state.generation+1,view:null,busy:false,error:''}
 if(event.generation!==state.generation||event.provider!==state.provider)return state
 if(event.type==='read')return {...state,view:readableSetup(event.view,state.provider)?event.view:null,error:readableSetup(event.view,state.provider)?'':'Mettez à jour l’agent local pour utiliser l’assistant.'}
 if(event.type==='start')return {...state,busy:true,error:''}
 if(event.type==='end')return {...state,busy:false,error:event.error||''}
 return state
}
export function deploymentSucceeded(result,step){
 if(step==='skills')return result?.skills?.status==='success'&&result.success===true
 return !!result&&!result.error&&!result.Error&&(result.installed===true||result.alreadyInit===true)
}
export function startupSetupPrompt(){
 let shown=false,reading=false
 return async(api,notify)=>{
  if(shown||reading)return
  reading=true
  try{
   const providers=[...new Set(supportedEngines(await api.engines()).map(engine=>engine.provider))]
   const reads=await Promise.allSettled(providers.map(provider=>api.setupState(provider)))
   const incomplete=providers.filter((provider,index)=>reads[index].status==='fulfilled'&&readableSetup(reads[index].value,provider)&&setupIncomplete(reads[index].value))
   if(incomplete.length){shown=true;await notify(incomplete)}
   else if(reads.every((read,index)=>read.status==='fulfilled'&&readableSetup(read.value,providers[index])&&SETUP_STEPS.every(step=>['success','skipped'].includes(read.value.steps[step].status))))shown=true
  }catch{}finally{reading=false}
 }
}
