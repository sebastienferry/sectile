// Engine output is untrusted: prose is rendered through DOM text nodes, never
// HTML. This small Markdown subset covers code, headings and bold emphasis.
export function renderConversationText(container,text){
 const inline=(node,value)=>{
  for(const part of value.split(/(`[^`]+`|\*\*[^*]+\*\*)/g)){
   const tag=part.startsWith('`')&&part.endsWith('`')?'code':part.startsWith('**')&&part.endsWith('**')?'strong':null
   if(tag){const child=document.createElement(tag);child.textContent=part.slice(tag==='code'?1:2,tag==='code'?-1:-2);node.append(child)}
   else node.append(document.createTextNode(part))
  }
 }
 const lines=String(text).split('\n')
 for(let i=0;i<lines.length;i++){
  const line=lines[i]
  if(line.startsWith('```')){
   const pre=document.createElement('pre'),code=document.createElement('code'),body=[]
   while(++i<lines.length&&!lines[i].startsWith('```'))body.push(lines[i])
   code.textContent=body.join('\n');pre.append(code);container.append(pre);continue
  }
  const heading=/^(#{1,6})\s+(.*)$/.exec(line)
  const node=document.createElement(heading?'h'+Math.min(heading[1].length+1,6):'div')
  inline(node,heading?heading[2]:line||'\u00a0');container.append(node)
 }
}

export function createConversationView({api,container,onError}){
 const panel=document.createElement('section');panel.className='conversation';panel.hidden=true
 panel.setAttribute('aria-label','Claude Code conversation')
 panel.innerHTML='<div class="conversation-events" role="log" aria-label="Conversation messages"></div><form class="conversation-composer"><label class="visually-hidden" for="conversation-message">Message Claude Code</label><textarea id="conversation-message" rows="2" maxlength="60000" placeholder="Ask a question or describe a change…" required></textarea><div class="conversation-toolbar"><span class="conversation-chip conversation-model" title="Model inherited from the source execution"></span><label class="conversation-chip conversation-effort" title="Reasoning effort for the next message"><svg viewBox="0 0 20 14" width="18" height="13" aria-hidden="true"><rect x="0" y="10" width="3" height="4" rx="1"/><rect x="4" y="8" width="3" height="6" rx="1"/><rect x="8" y="6" width="3" height="8" rx="1"/><rect x="12" y="3" width="3" height="11" rx="1"/><rect x="16" y="0" width="3" height="14" rx="1"/></svg><select aria-label="Effort"><option value="">Default effort</option><option value="low">Low</option><option value="medium">Medium</option><option value="high">High</option><option value="xhigh">Extra high</option><option value="max">Max</option></select></label><span class="conversation-chip" title="Edits are accepted; tools requiring approval are denied. Stop closes this conversation. History after an agent restart is read-only.">Accept edits</span><span class="conversation-status" role="status"></span><span class="conversation-context" role="img" hidden><svg viewBox="0 0 36 36" aria-hidden="true"><circle cx="18" cy="18" r="15"/><circle class="conversation-context-used" cx="18" cy="18" r="15" pathLength="100" stroke-dasharray="0 100" transform="rotate(-90 18 18)"/></svg><span></span></span><button type="submit" class="conversation-send" aria-label="Send" title="Send (Enter) · New line (Shift+Enter)"><svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M12 19V5M5 12l7-7 7 7" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg></button></div></form>'
 container.append(panel)
 const events=panel.querySelector('.conversation-events'),form=panel.querySelector('form'),input=panel.querySelector('textarea'),send=panel.querySelector('button'),status=panel.querySelector('.conversation-status'),model=panel.querySelector('.conversation-model'),effort=panel.querySelector('.conversation-effort select'),effortBars=panel.querySelectorAll('.conversation-effort rect'),ring=panel.querySelector('.conversation-context')
 const levels=['','low','medium','high','xhigh','max']
 // One lit bar per level; the default effort lights none, since the CLI decides it.
 const showEffort=()=>effortBars.forEach((bar,i)=>bar.classList.toggle('lit',i<levels.indexOf(effort.value)))
 effort.addEventListener('change',showEffort)
 function showContext(context){
  ring.hidden=!context?.window
  if(ring.hidden)return
  const percent=Math.min(100,Math.round(context.used*100/context.window))
  ring.querySelector('.conversation-context-used').setAttribute('stroke-dasharray',percent+' 100')
  ring.querySelector('span').textContent=percent+'%'
  ring.classList.toggle('full',percent>=80)
  const label=context.used.toLocaleString('en-US')+' of '+context.window.toLocaleString('en-US')+' context tokens used ('+percent+'%)'
  ring.title=label;ring.setAttribute('aria-label',label)
 }
 // The textarea grows with its content up to the CSS max-height, then scrolls.
 const grow=()=>{input.style.height='auto';input.style.height=input.scrollHeight+'px'}
 input.addEventListener('input',grow)
 input.addEventListener('keydown',event=>{
  if(event.key==='Enter'&&!event.shiftKey&&!event.isComposing){event.preventDefault();form.requestSubmit()}
 })
 let selected=null,generation=0,timer=null,version=null,pending=false,available=false,effortLoaded=false
 function controls(data){
  available=!data.readOnly&&!data.busy
  input.disabled=effort.disabled=!!data.readOnly;send.disabled=!available||pending
  // The agent's effort is adopted once per selection so polling never undoes a pick.
  if(!effortLoaded){effortLoaded=true;effort.value=levels.includes(data.effort)?data.effort:'';showEffort()}
  showContext(data.context)
  status.textContent=data.readOnly?'Read-only history':data.busy?'Claude Code is working…':'Ready'
 }
 function draw(data){
  if(version===data.version)return
  version=data.version
  const follow=events.scrollHeight-events.scrollTop-events.clientHeight<80
  const expanded=new Set([...events.querySelectorAll('details[open]')].map(node=>node.conversationEvent))
  events.replaceChildren()
  for(const raw of data.events||[]){
   let event;try{event=JSON.parse(raw)}catch{continue}
   const node=document.createElement(event.kind==='tool'?'details':'article');node.className='conversation-event conversation-'+event.kind
   if(event.kind==='tool'){
    node.conversationEvent=raw;node.open=expanded.has(raw)
    const title=document.createElement('summary');title.textContent=event.text;node.append(title)
    const body=document.createElement('pre');body.textContent=event.detail||'';node.append(body)
   }else{
    const label=document.createElement('strong');label.className='conversation-speaker';label.textContent=event.kind==='user'?'You':event.kind==='assistant'?'Claude Code':event.kind==='error'?'Error':event.kind==='thinking'?'Thinking':'Status';node.append(label)
    const body=document.createElement('div');renderConversationText(body,event.text||'');node.append(body)
    if(event.detail){const detail=document.createElement('small');detail.textContent=event.detail;node.append(detail)}
   }
   events.append(node)
  }
  if(follow)events.scrollTop=events.scrollHeight
 }
 async function poll(token,id){
  try{
   const data=await api.conversation(id)
   if(token!==generation||id!==selected)return
   if(data.id!==id)throw Error('The agent returned another conversation.')
   draw(data);controls(data)
  }catch(err){
   if(token!==generation)return
   available=false;send.disabled=true;status.textContent=err.message||String(err)
  }finally{if(token===generation&&selected)timer=setTimeout(()=>poll(token,id),750)}
 }
 form.addEventListener('submit',async event=>{
  event.preventDefault()
  const id=selected,token=generation,message=input.value
  if(!id||!message.trim()||pending||!available)return
  pending=true;send.disabled=true
  try{
   await api.conversationMessage(id,message,effort.value)
   if(token!==generation)return
   input.value='';grow();available=false;status.textContent='Claude Code is working…'
  }catch(err){if(token===generation)onError(err)}
  finally{if(token===generation){pending=false;send.disabled=!available}}
 })
 return {select(run){
  generation++;clearTimeout(timer);selected=run?.conversation?run.id:null;version=null;pending=false;available=false;effortLoaded=false
  events.replaceChildren();effort.value='';effort.disabled=true;showEffort();showContext(null);input.value='';grow();model.textContent=run?.model||'CLI default';input.disabled=true;send.disabled=true
  panel.hidden=!selected;container.classList.toggle('conversation-active',!!selected)
  if(selected){status.textContent='Loading conversation…';poll(generation,selected)}
 },get active(){return !!selected}}
}
