import {runFolderOutcome} from './run-folders.mjs'
import {ipcMessage} from './execution-fields.mjs'
import {markdownModel,renderMarkdown} from './markdownView.mjs'
import {toolCard,renderToolCard} from './tool-cards.mjs'

// What Claude writes is Markdown: it goes through the same safe renderer as the
// Rendered view of a changed file (#575), so raw HTML stays text, images never
// load and only web and mail links open, outside the window.
const markdownKinds=new Set(['assistant','thinking'])

// canAddFolder tells whether the local agent attaches a folder from a run (#676).
// canControl tells whether the agent interrupts a turn and opens a terminal
// beside the conversation; canQueue whether it takes a message while Claude
// works.
export function createConversationView({api,container,onError,canAddFolder=()=>false,canControl=()=>false,canQueue=()=>false}){
 const panel=document.createElement('section');panel.className='conversation';panel.hidden=true
 panel.setAttribute('aria-label','Claude Code conversation')
 panel.innerHTML='<div class="conversation-events" role="log" aria-label="Conversation messages"></div><form class="conversation-composer"><label class="visually-hidden" for="conversation-message">Message Claude Code</label><textarea id="conversation-message" rows="2" maxlength="60000" placeholder="Ask a question or describe a change…" required></textarea><div class="conversation-toolbar"><span class="conversation-chip conversation-model" title="Model inherited from the source execution"></span><label class="conversation-chip conversation-effort" title="Reasoning effort for the next message"><svg viewBox="0 0 20 14" width="18" height="13" aria-hidden="true"><rect x="0" y="10" width="3" height="4" rx="1"/><rect x="4" y="8" width="3" height="6" rx="1"/><rect x="8" y="6" width="3" height="8" rx="1"/><rect x="12" y="3" width="3" height="11" rx="1"/><rect x="16" y="0" width="3" height="14" rx="1"/></svg><select aria-label="Effort"><option value="">Default effort</option><option value="low">Low</option><option value="medium">Medium</option><option value="high">High</option><option value="xhigh">Extra high</option><option value="max">Max</option></select></label><span class="conversation-chip" title="Edits and Sectile’s tools are accepted; other tools your rules do not allow ask for your approval. Stop closes this conversation. History after an agent restart is read-only.">Accept edits</span><button type="button" class="conversation-chip conversation-add-folder" aria-label="Add folder…" title="Attach a folder of this workstation to the project; Claude sees it from the next message" hidden><svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 20a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2z"/><path d="M12 11v6M9 14h6"/></svg><span>Add folder…</span></button><button type="button" class="conversation-chip conversation-terminal" aria-label="Open a terminal" title="Open a terminal in this conversation’s directory" hidden><svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="m7.5 10 2.5 2.5-2.5 2.5"/><path d="M13 15h4"/></svg><span>Terminal</span></button><span class="conversation-status" role="status"></span><span class="conversation-context" role="img" hidden><svg viewBox="0 0 36 36" aria-hidden="true"><circle cx="18" cy="18" r="15"/><circle class="conversation-context-used" cx="18" cy="18" r="15" pathLength="100" stroke-dasharray="0 100" transform="rotate(-90 18 18)"/></svg><span></span></span><button type="button" class="conversation-interrupt" aria-label="Stop answer" title="Stop this answer (Esc); the conversation stays open" hidden><svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true"><rect x="6" y="6" width="12" height="12" rx="2" fill="currentColor"/></svg></button><button type="submit" class="conversation-send" aria-label="Send" title="Send (Enter) · New line (Shift+Enter)"><svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path d="M12 19V5M5 12l7-7 7 7" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg></button></div></form>'
 container.append(panel)
 const events=panel.querySelector('.conversation-events'),form=panel.querySelector('form'),input=panel.querySelector('textarea'),send=panel.querySelector('.conversation-send'),addFolder=panel.querySelector('.conversation-add-folder'),openTerminal=panel.querySelector('.conversation-terminal'),interrupt=panel.querySelector('.conversation-interrupt'),status=panel.querySelector('.conversation-status'),model=panel.querySelector('.conversation-model'),effort=panel.querySelector('.conversation-effort select'),effortBars=panel.querySelectorAll('.conversation-effort rect'),ring=panel.querySelector('.conversation-context')
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
  if(event.key==='Escape'&&!interrupt.hidden&&!interrupt.disabled){event.preventDefault();interrupt.click()}
 })
 let selected=null,directory='',busy=false,generation=0,timer=null,version=null,pending=false,available=false,effortLoaded=false,readOnly=true,attaching=false
 // The outcome of an added folder stays in the status for a while, over polling.
 let notice='',noticeUntil=0
 const showAddFolder=()=>{addFolder.hidden=!selected||!canAddFolder();addFolder.disabled=readOnly||attaching}
 // While Claude answers, Stop answer appears; Send stays beside it when the
 // agent takes a message mid-answer, as in Claude Code. The terminal is
 // offered as long as the conversation is open.
 const showControls=busy=>{
  const control=!!selected&&canControl()&&!readOnly
  openTerminal.hidden=!control
  interrupt.hidden=!(control&&busy);send.hidden=!interrupt.hidden&&!canQueue()
  if(!busy)interrupt.disabled=false
 }
 function controls(data){
  available=!data.readOnly&&(!data.busy||canQueue())
  readOnly=!!data.readOnly;showAddFolder();showControls(!!data.busy)
  input.disabled=effort.disabled=!!data.readOnly;send.disabled=!available||pending
  // The agent's effort is adopted once per selection so polling never undoes a pick.
  if(!effortLoaded){effortLoaded=true;effort.value=levels.includes(data.effort)?data.effort:'';showEffort()}
  showContext(data.context)
  status.textContent=Date.now()<noticeUntil?notice:data.readOnly?'Read-only history':data.approvals?.length?'Waiting for your approval':data.busy?'Claude Code is working…':'Ready'
 }
 // Claude may be working: the folder is attached at once and given from the
 // next turn on, so the action stays available while busy.
 addFolder.addEventListener('click',async()=>{
  const id=selected,token=generation
  if(!id||readOnly||attaching)return
  notice=''
  try{
   const path=await api.chooseRepository()
   if(!path||token!==generation)return
   attaching=true;showAddFolder()
   const answer=await api.addRunFolder(id,path)
   notice=runFolderOutcome(path,answer)
  }catch(err){notice=ipcMessage(err)}
  finally{
   if(token===generation){
    attaching=false;showAddFolder()
    if(notice){noticeUntil=Date.now()+8000;status.textContent=notice}
   }
  }
 })
 interrupt.addEventListener('click',async()=>{
  const id=selected,token=generation
  if(!id)return
  interrupt.disabled=true;status.textContent='Stopping the answer…'
  try{await api.conversationInterrupt(id)}
  catch(err){if(token===generation){interrupt.disabled=false;onError(err)}}
 })
 openTerminal.addEventListener('click',async()=>{
  const id=selected,token=generation
  if(!id)return
  openTerminal.disabled=true
  try{await api.conversationTerminal(id)}
  catch(err){if(token===generation)onError(err)}
  finally{if(token===generation)openTerminal.disabled=false}
 })
 function draw(data){
  if(version===data.version)return
  version=data.version
  const follow=events.scrollHeight-events.scrollTop-events.clientHeight<80
  // A card the reader already saw keeps the state they left it in; a new one
  // opens as its tool suggests, or because the call failed.
  const seen=new Set([...events.querySelectorAll('details')].map(node=>node.conversationEvent))
  const expanded=new Set([...events.querySelectorAll('details[open]')].map(node=>node.conversationEvent))
  events.replaceChildren()
  // A tool result is drawn inside the card of the call it answers.
  const parsed=[],results=new Map(),decisions=new Map()
  for(const raw of data.events||[]){
   let event;try{event=JSON.parse(raw)}catch{continue}
   if(event.kind==='tool_result'){if(event.toolId)results.set(event.toolId,event);continue}
   if(event.kind==='approval'){if(event.toolId)decisions.set(event.toolId,event.text);continue}
   parsed.push([raw,event])
  }
  for(const [raw,event] of parsed){
   if(event.kind==='tool'){
    const card=toolCard(event,directory,{result:results.get(event.toolId)||null,pending:!!data.busy&&!!event.toolId,decision:decisions.get(event.toolId)||''}),node=renderToolCard(card)
    node.conversationEvent=raw;if(event.toolId)node.dataset.toolId=event.toolId
    if(node.tagName==='DETAILS')node.open=seen.has(raw)?expanded.has(raw):card.open||card.failed
    events.append(node);continue
   }
   const node=document.createElement('article');node.className='conversation-event conversation-'+event.kind
   const label=document.createElement('strong');label.className='conversation-speaker';label.textContent=event.kind==='user'?'You':event.kind==='assistant'?'Claude Code':event.kind==='error'?'Error':event.kind==='thinking'?'Thinking':'Status';node.append(label)
   const body=document.createElement('div')
   if(markdownKinds.has(event.kind)){body.className='conversation-markdown';body.append(renderMarkdown(markdownModel(event.text||''),{openLink:url=>api.openLink(url)}))}
   else{body.className='conversation-plain';body.textContent=event.text||''}
   node.append(body)
   if(event.detail){const detail=document.createElement('small');detail.textContent=event.detail;node.append(detail)}
   events.append(node)
  }
  if(follow)events.scrollTop=events.scrollHeight
 }
 // A tool call Claude may not make on its own waits in its card for Allow,
 // Always allow or Deny. A request whose call has no card yet gets one.
 function drawApprovals(list){
  const pending=new Set(list.map(item=>item.id))
  for(const node of events.querySelectorAll('[data-approval-id]'))if(!pending.has(node.dataset.approvalId)){node.closest('.tool-card')?.classList.remove('tool-card-asking');node.remove()}
  for(const node of events.querySelectorAll('[data-approval-card]'))if(!pending.has(node.dataset.approvalCard))node.remove()
  const follow=events.scrollHeight-events.scrollTop-events.clientHeight<80
  for(const item of list){
   if([...events.querySelectorAll('[data-approval-id]')].some(node=>node.dataset.approvalId===item.id))continue
   const name=String(item.tool||'Tool')
   let card=item.toolUseId&&[...events.querySelectorAll('.tool-card')].find(node=>node.dataset.toolId===item.toolUseId)
   if(!card){
    card=renderToolCard(toolCard({kind:'tool',text:name.startsWith('mcp__')?name.split('__').pop():name,tool:name,input:item.input,toolId:item.toolUseId},directory))
    card.dataset.approvalCard=item.id;events.insertBefore(card,partialNode?.isConnected?partialNode:null)
   }
   if(card.tagName==='DETAILS')card.open=true
   card.classList.add('tool-card-asking')
   const bar=document.createElement('div');bar.className='tool-approval';bar.dataset.approvalId=item.id
   bar.setAttribute('role','group');bar.setAttribute('aria-label','Allow '+name+'?')
   const question=document.createElement('span');question.className='tool-approval-question';question.textContent='Allow '+name+(item.description?': '+item.description:'')+'?'
   bar.append(question)
   const choices=[['allow','Allow'],...(Array.isArray(item.suggestions)&&item.suggestions.length?[['always','Always allow']]:[]),['deny','Deny']]
   for(const [decision,label] of choices){
    const button=document.createElement('button');button.type='button';button.textContent=label;button.className='tool-approval-'+decision
    button.onclick=async()=>{
     const id=selected,token=generation
     for(const other of bar.querySelectorAll('button'))other.disabled=true
     try{await api.conversationApproval(id,item.id,decision)}
     catch(err){if(token===generation){for(const other of bar.querySelectorAll('button'))other.disabled=false;onError(err)}}
    }
    bar.append(button)
   }
   card.append(bar)
  }
  if(list.length&&follow)events.scrollTop=events.scrollHeight
 }
 // The reply Claude is still writing, drawn after the history and redrawn on
 // its own: the history is rebuilt only when a complete event arrives.
 let partialNode=null,partialText=''
 function drawPartial(text){
  if(!text){partialNode?.remove();partialNode=null;partialText='';return}
  if(text===partialText&&partialNode?.isConnected)return
  const follow=events.scrollHeight-events.scrollTop-events.clientHeight<80
  if(!partialNode?.isConnected){
   partialNode=document.createElement('article');partialNode.className='conversation-event conversation-assistant conversation-partial'
   partialNode.setAttribute('aria-busy','true')
   const label=document.createElement('strong');label.className='conversation-speaker';label.textContent='Claude Code'
   const body=document.createElement('div');body.className='conversation-markdown'
   partialNode.append(label,body);events.append(partialNode)
  }
  partialNode.lastChild.replaceChildren(renderMarkdown(markdownModel(text),{openLink:url=>api.openLink(url)}))
  partialText=text
  if(follow)events.scrollTop=events.scrollHeight
 }
 async function poll(token,id){
  try{
   const data=await api.conversation(id)
   if(token!==generation||id!==selected)return
   if(data.id!==id)throw Error('The agent returned another conversation.')
   draw(data);drawPartial(data.busy?data.partial||'':'');drawApprovals(Array.isArray(data.approvals)?data.approvals:[]);controls(data);busy=!!data.busy
  }catch(err){
   if(token!==generation)return
   available=false;send.disabled=true;status.textContent=err.message||String(err)
  }finally{if(token===generation&&selected)timer=setTimeout(()=>poll(token,id),busy?250:750)}
 }
 form.addEventListener('submit',async event=>{
  event.preventDefault()
  const id=selected,token=generation,message=input.value
  if(!id||!message.trim()||pending||!available)return
  pending=true;send.disabled=true
  try{
   await api.conversationMessage(id,message,effort.value)
   if(token!==generation)return
   input.value='';grow();available=canQueue();status.textContent='Claude Code is working…'
  }catch(err){if(token===generation)onError(err)}
  finally{if(token===generation){pending=false;send.disabled=!available}}
 })
 return {select(run){
  generation++;clearTimeout(timer);selected=run?.conversation?run.id:null;directory=run?.directory||'';busy=false;openTerminal.hidden=true;interrupt.hidden=true;send.hidden=false;partialNode=null;partialText='';version=null;pending=false;available=false;effortLoaded=false;readOnly=true;attaching=false;notice='';noticeUntil=0;showAddFolder()
  events.replaceChildren();effort.value='';effort.disabled=true;showEffort();showContext(null);input.value='';grow();model.textContent=run?.model||'CLI default';input.disabled=true;send.disabled=true
  panel.hidden=!selected;container.classList.toggle('conversation-active',!!selected)
  if(selected){status.textContent='Loading conversation…';poll(generation,selected)}
 },get active(){return !!selected}}
}
