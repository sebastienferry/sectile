// Repository instructions are independent of the workstation engine catalogue.
export function agentProfileFields({api,projectID,active,onCreating}){
 const box=document.createElement('div'),label=document.createElement('label'),select=document.createElement('select')
 label.textContent='Agent profile';select.setAttribute('aria-label','Agent profile')
 const status=document.createElement('p');status.setAttribute('role','status');status.textContent='Loading agent profiles…'
 const create=document.createElement('button');create.type='button';create.textContent='Create profile';create.disabled=true
 label.append(select);box.append(label,create,status)
 let profiles=[],saving=false,available=false,launching=false
 function render(id=''){
  select.replaceChildren(new Option('None',''))
  for(const profile of profiles)select.append(new Option(profile.name,profile.id))
  select.value=profiles.some(profile=>profile.id===id)?id:''
 }
 render();select.disabled=true
 async function load(id=''){
  try{
   if(!api.agentProfiles)throw Error('Update the local agent to use repository profiles.')
   const result=await api.agentProfiles(projectID)
   if(!active())return
   profiles=result.profiles||[];render(id);available=true;select.disabled=launching;create.disabled=launching
   status.textContent=result.warnings?.length?result.warnings.map(item=>item.id+': '+item.message).join('\n')
    :profiles.length?'Instructions apply only to this new session.':'No repository profiles yet. Choose None or create one.'
  }catch(err){if(active()){status.textContent=available?'Profile refresh failed: '+err.message:'Profiles unavailable: '+err.message+' You can still launch with None.';select.disabled=!available||launching;create.disabled=!available||launching}}
 }
 create.onclick=()=>{
  if(saving||launching||!active())return
  const editor=document.createElement('fieldset'),legend=document.createElement('legend')
  legend.textContent='Create repository agent profile'
  const nameLabel=document.createElement('label'),name=document.createElement('input')
  nameLabel.textContent='Profile name';name.setAttribute('aria-label','Profile name');name.placeholder='code-reviewer';name.maxLength=64
  const promptLabel=document.createElement('label'),prompt=document.createElement('textarea')
  promptLabel.textContent='Reusable agent instructions';prompt.setAttribute('aria-label','Reusable agent instructions');prompt.rows=8;prompt.className='cli-command'
  const explanation=document.createElement('p');explanation.textContent='Saved as .sectile/agents/<name>.md in this project. Enter the session’s request after launch.'
  const save=document.createElement('button'),cancel=document.createElement('button'),message=document.createElement('p')
  save.type=cancel.type='button';save.textContent='Save profile';cancel.textContent='Cancel profile';message.setAttribute('role','status')
  const actions=document.createElement('div');actions.className='deployment-actions';actions.append(save,cancel)
  nameLabel.append(name);promptLabel.append(prompt);editor.append(legend,explanation,nameLabel,promptLabel,actions,message)
  label.hidden=create.hidden=true;box.append(editor);onCreating(true);name.focus()
  function close(){editor.remove();label.hidden=create.hidden=false;onCreating(false);create.focus()}
  cancel.onclick=()=>{if(!saving)close()}
  editor.onkeydown=event=>{if(event.key==='Escape'){event.preventDefault();event.stopPropagation();if(!saving)close()}}
  save.onclick=async()=>{
   if(saving)return
   if(!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(name.value)||name.value.length>64||/^(con|prn|aux|nul|com[1-9]|lpt[1-9])$/.test(name.value)){message.textContent='Use 1–64 lowercase letters or digits, separated by single hyphens; avoid Windows device names such as con or nul.';return}
   if(!prompt.value.trim()||new TextEncoder().encode(prompt.value).length>65536||prompt.value.includes('\0')){message.textContent='Enter nonempty instructions of at most 64 KiB, without NUL.';return}
   saving=true;save.disabled=cancel.disabled=true;message.textContent='Saving profile…'
   try{
    const result=await api.createAgentProfile(projectID,name.value,prompt.value)
    if(!active())return
    // Persistence succeeded: preserve the selection even if refresh later fails.
    profiles=[...profiles.filter(profile=>profile.id!==result.profile.id),result.profile].sort((a,b)=>a.id.localeCompare(b.id))
    render(result.profile.id);status.textContent='Profile saved. Open the console when ready.';close();await load(result.profile.id)
   }catch(err){if(active())message.textContent='Not saved: '+err.message}
   finally{saving=false;save.disabled=cancel.disabled=false}
  }
 }
 return {element:box,ready:load(),selected:()=>select.value,disable:disabled=>{launching=disabled;select.disabled=disabled||!available;create.disabled=disabled||!available}}
}
