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
 panel.innerHTML='<div class="conversation-events" role="log" aria-label="Conversation messages"></div><form class="conversation-composer"><label for="conversation-message">Message Claude Code</label><textarea id="conversation-message" rows="3" maxlength="60000" placeholder="Ask a question or describe a change…" required></textarea><div><span class="conversation-status" role="status"></span><button type="submit">Send</button></div><small>Experimental · edits accepted; tools requiring approval are denied. Stop closes this conversation. History after an agent restart is read-only.</small></form>'
 container.append(panel)
 const events=panel.querySelector('.conversation-events'),form=panel.querySelector('form'),input=panel.querySelector('textarea'),send=panel.querySelector('button'),status=panel.querySelector('.conversation-status')
 let selected=null,generation=0,timer=null,version=null,pending=false,available=false
 function controls(data){
  available=!data.readOnly&&!data.busy
  input.disabled=!!data.readOnly;send.disabled=!available||pending
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
   await api.conversationMessage(id,message)
   if(token!==generation)return
   input.value='';available=false;status.textContent='Claude Code is working…'
  }catch(err){if(token===generation)onError(err)}
  finally{if(token===generation){pending=false;send.disabled=!available}}
 })
 return {select(run){
  generation++;clearTimeout(timer);selected=run?.conversation?run.id:null;version=null;pending=false;available=false
  events.replaceChildren();input.value='';input.disabled=true;send.disabled=true
  panel.hidden=!selected;container.classList.toggle('conversation-active',!!selected)
  if(selected){status.textContent='Loading conversation…';poll(generation,selected)}
 },get active(){return !!selected}}
}
