// Render tooltips ourselves: native title bubbles are delayed, unavailable to
// keyboard users, and unreliable on disabled Electron controls.
export function installTooltips(){
 const tooltip=document.createElement('div')
 tooltip.id='action-tooltip';tooltip.className='action-tooltip'
 tooltip.setAttribute('role','tooltip');tooltip.setAttribute('popover','manual')
 let target=null,timer=null,nativeTitle=null
 function hide(){
  clearTimeout(timer)
  if(target){
   const descriptions=(target.getAttribute('aria-describedby')||'').split(/\s+/).filter(id=>id&&id!==tooltip.id)
   if(descriptions.length)target.setAttribute('aria-describedby',descriptions.join(' '))
   else target.removeAttribute('aria-describedby')
   if(nativeTitle!==null&&!target.hasAttribute('title'))target.setAttribute('title',nativeTitle)
  }
  if(tooltip.matches(':popover-open'))tooltip.hidePopover()
  tooltip.remove();target=null;nativeTitle=null
 }
 function show(button,immediate=false){
  if(button===target)return
  hide()
  const label=button?.getAttribute('title')||button?.getAttribute('aria-label')
  if(!label)return
  target=button
  const display=()=>{
   if(!button.isConnected||!button.getClientRects().length){hide();return}
   nativeTitle=button.getAttribute('title');button.removeAttribute('title')
   tooltip.textContent=label
   // A dialog is modal; keeping its tooltip inside it preserves accessibility.
   ;(button.closest('dialog')||document.body).append(tooltip)
   tooltip.showPopover()
   const box=button.getBoundingClientRect(),size=tooltip.getBoundingClientRect()
   const left=Math.max(8,Math.min(box.left+(box.width-size.width)/2,innerWidth-size.width-8))
   const top=box.bottom+size.height+16<=innerHeight?box.bottom+8:Math.max(8,box.top-size.height-8)
   tooltip.style.left=left+'px';tooltip.style.top=top+'px'
   button.setAttribute('aria-describedby',[(button.getAttribute('aria-describedby')||''),tooltip.id].filter(Boolean).join(' '))
  }
  if(immediate)display();else timer=setTimeout(display,180)
 }
 const buttonAt=event=>event.target instanceof Element?event.target.closest('button[title],button[aria-label]'):null
 document.addEventListener('pointerover',event=>{const button=buttonAt(event);if(button)show(button)},true)
 document.addEventListener('pointerout',event=>{if(target&&!target.contains(event.relatedTarget))hide()},true)
 document.addEventListener('focusin',event=>{const button=buttonAt(event);if(button)show(button,true)})
 document.addEventListener('focusout',hide)
 document.addEventListener('pointerdown',hide,true)
 document.addEventListener('keydown',event=>{if(event.key==='Escape')hide()},true)
 document.addEventListener('scroll',hide,true)
 document.addEventListener('close',hide,true)
 window.addEventListener('resize',hide)
 new MutationObserver(()=>{if(target&&!target.isConnected)hide()}).observe(document.body,{childList:true,subtree:true})
}
