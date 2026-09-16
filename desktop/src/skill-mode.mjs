// The execution mode of one launch, as the desktop dialogs offer it.
//
// The empty value is not "interactive": it means the launch sends no override,
// so the server's precedence (skill setting, then project default, then
// interactive) decides. Both dialogs default to it, which is why an untouched
// control never changes what the project is configured to do.
export const SKILL_MODE_OPTIONS=[
 {value:'',label:'Configured mode'},
 {value:'interactive',label:'Interactive'},
 {value:'autonomous',label:'Autonomous (headless)'}
]

// launchModeOverride normalizes what a dialog produces into what the launch
// payload should carry. Anything unrecognized is dropped rather than sent: a
// stale control must not pin a mode nobody chose.
export function launchModeOverride(value){
 const mode=String(value||'').trim().toLowerCase()
 return mode==='interactive'||mode==='autonomous'?mode:''
}

// modeSelect builds the control both dialogs share, so they cannot drift apart.
export function modeSelect(document,ariaLabel){
 const select=document.createElement('select')
 select.setAttribute('aria-label',ariaLabel)
 for(const option of SKILL_MODE_OPTIONS){
  const element=document.createElement('option')
  element.value=option.value;element.textContent=option.label
  select.append(element)
 }
 select.value=''
 return select
}
