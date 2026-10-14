import {createMarkdownEditor} from './markdown-editor.mjs'
import {validateTitle,validateDescription,validatePullRequestUrl,addLink,removeLink,moveLink,taskChanges,normalizeIssueTypes,TITLE_LIMIT} from './task-form.mjs'
import {defaultActionShortcut,defaultActionLabel} from './dialog-default.mjs'
import {pullRequestPresentation} from './pullRequests.mjs'
import {ipcMessage} from './execution-fields.mjs'

// The desktop task page (#805): one page creates a task and edits it, in the
// workspace where the console is. It owns its fields, its save sequences and
// what follows a creation; main.js mounts it, guards leaving it and passes the
// few actions that belong to the rest of the window through deps:
//   api           the preload bridge
//   projects      the configured projects, [{id,name}]
//   projectId     the project a creation starts on
//   mac           whether shortcuts read Cmd
//   rememberProject(id)            the project a creation was made in
//   launchClarify(projectId,task)  Clarify now
//   openTickets(projectId,query)   Launch task
//   addAnother(projectId)          Add another
//   done()                         Done and Close
//   saved(projectId,task)          after any write, to refresh other views
export function openTaskPage(container,{mode,projectId,taskId,deps}){
 const {api,mac}=deps
 const page=document.createElement('section');page.className='task-page';page.setAttribute('aria-label','Task')
 container.replaceChildren(page)
 const initialProjectId=projectId||''
 let initialTrackerId=null
 const state={mode,projectId:projectId||'',taskId:taskId||'',task:null,loaded:null,form:{title:'',issueType:'',priority:'medium',prLinks:[]},trackers:[],available:true,saving:false,trackersLoading:false,editor:null,destroyed:false,unsaved:new Set()}
 const element=(tag,className,text)=>{const node=document.createElement(tag);if(className)node.className=className;if(text!==undefined)node.textContent=text;return node}
 const button=(text,className,label)=>{const node=element('button',className,text);node.type='button';if(label){node.setAttribute('aria-label',label);node.title=label}return node}

 const header=element('div','task-page-header')
 const heading=element('h2','task-page-title'),headingText=element('span'),mark=element('span','task-page-unsaved','•')
 mark.title='Unsaved changes';mark.setAttribute('aria-label','Unsaved changes');mark.hidden=true
 heading.append(headingText,mark)
 const trackerLink=button('Open in tracker','secondary task-page-tracker-link');trackerLink.hidden=true
 const close=button('Close','secondary task-page-close');close.onclick=()=>deps.done()
 header.append(heading,trackerLink,close)
 const created=element('div','task-page-created');created.hidden=true
 const body=element('div','task-page-body')
 const status=element('p','task-page-status');status.setAttribute('role','status')
 const footer=element('div','task-page-actions')
 const keys=element('span','task-page-keys')
 const save=element('button','task-page-save');save.type='button'
 footer.append(keys,save)
 page.append(header,created,status,body,footer)

 const field=(text,control,{hint,id}={})=>{
  const wrapper=element('div','task-page-field')
  const label=element('label','task-page-label',text);const controlId=id||'task-page-'+text.toLowerCase().replace(/\W+/g,'-');label.htmlFor=controlId
  if(control.matches?.('input,select,textarea'))control.id=controlId
  wrapper.append(label,control)
  if(hint){const small=element('small','task-page-hint',hint);wrapper.append(small)}
  return wrapper
 }
 const setStatus=(text,kind='')=>{status.textContent=text;status.dataset.kind=kind}
 const key=()=>state.task?.key||state.task?.id||''

 // Project and tracker: a select in creation, read-only text in edition.
 const project=element('select');project.required=true
 const tracker=element('select')
 const projectText=element('p','task-page-readonly'),trackerText=element('p','task-page-readonly')
 const projectField=field('Project',project),trackerField=field('Tracker',tracker)
 const title=element('input');title.placeholder='What needs doing?';title.required=true;title.maxLength=TITLE_LIMIT;title.autocomplete='off'
 const titleField=field('Title',title)

 const issueType=element('select'),priority=element('select')
 const typeNote=element('small','task-page-hint');typeNote.setAttribute('role','status')
 const typeRetry=button('Retry','secondary','Retry issue types');typeRetry.hidden=true
 const typeField=field('Issue type',issueType);typeField.append(typeNote,typeRetry)
 const priorityField=field('Priority',priority)
 for(const [value,label] of [['urgent','Urgent'],['high','High'],['medium','Medium'],['low','Low']]){
  const option=element('option','',label);option.value=value;priority.append(option)
 }
 let typeGeneration=0,typeValues=[]
 function renderTypes(){
  issueType.replaceChildren()
  const defaultOption=element('option','','Default');defaultOption.value='';issueType.append(defaultOption)
  for(const value of normalizeIssueTypes([...typeValues,state.loaded?.issueType,state.form.issueType])){
   const option=element('option','',value);option.value=value;issueType.append(option)
  }
  issueType.value=state.form.issueType
  priority.value=state.form.priority
 }
 async function loadTypes(){
  const generation=++typeGeneration
  const projectID=state.projectId,trackerID=state.mode==='edit'?state.task?.trackerId:tracker.value
  typeValues=[];renderTypes();typeRetry.hidden=true
  if(!projectID||!trackerID){typeNote.textContent='Select a project and tracker to load issue types.';return}
  typeNote.textContent='Loading issue types…'
  try{
   const result=await api.taskIssueTypes(projectID,trackerID)
   if(state.destroyed||generation!==typeGeneration)return
   if(typeof result?.supported!=='boolean'||!Array.isArray(result.issueTypes))throw Error('Invalid issue type response')
   typeValues=normalizeIssueTypes(result.issueTypes)
   if(state.mode==='create'&&!typeValues.includes(state.form.issueType))state.form.issueType=''
   typeNote.textContent=!result.supported?'This tracker does not support issue type discovery.':!typeValues.length?'No issue types are available.':''
   renderTypes();refresh()
  }catch(err){
   if(state.destroyed||generation!==typeGeneration)return
   typeNote.textContent='Could not load issue types: '+ipcMessage(err);typeRetry.hidden=false
  }
 }
 typeRetry.onclick=()=>state.trackers.length?loadTypes():loadTrackers()
 issueType.onchange=()=>{state.form.issueType=issueType.value;refresh()}
 priority.onchange=()=>{state.form.priority=priority.value;refresh()}

 // Pull requests: the ordered set, the last one being the current.
 const links=element('ul','task-page-links')
 const linkInput=element('input');linkInput.placeholder='https://…';linkInput.setAttribute('aria-label','Pull request link');linkInput.autocomplete='off'
 const linkAdd=button('Add','secondary')
 const linkNote=element('small','task-page-hint');linkNote.setAttribute('role','alert')
 const linkRow=element('div','task-page-link-add');linkRow.append(linkInput,linkAdd)
 const linksField=element('div','task-page-field')
 const linksLabel=element('span','task-page-label','Pull requests');linksLabel.id='task-page-links-label';links.setAttribute('aria-labelledby',linksLabel.id)
 linksField.append(linksLabel,links,linkRow,linkNote)

 // Description: the rich editor and its Markdown toggle.
 const descriptionField=element('div','task-page-field task-page-description')
 const descriptionHead=element('div','task-page-description-head')
 const descriptionLabel=element('span','task-page-label','Description')
 const sourceToggle=button('Markdown','secondary task-page-source-toggle');sourceToggle.setAttribute('aria-pressed','false');sourceToggle.title='Show the Markdown source'
 descriptionHead.append(descriptionLabel,sourceToggle)
 const editorRoot=element('div','task-page-editor')
 const descriptionNote=element('small','task-page-hint')
 descriptionField.append(descriptionHead,editorRoot,descriptionNote)

 // What differs from what was loaded, or from the empty form in creation.
 function dirty(){
  if(state.destroyed||!state.available&&state.mode==='edit')return false
  if(state.mode==='edit'&&!state.loaded)return false
  if(state.unsaved.size)return true
  if(state.mode==='create'&&(state.projectId!==initialProjectId||(initialTrackerId!==null&&tracker.value!==initialTrackerId)))return true
  return Object.keys(pendingChanges()).length>0
 }
 function pendingChanges(){
  const base=state.mode==='create'?{title:'',issueType:'',priority:'medium',prLinks:[]}:state.loaded
  return taskChanges(base,{...state.form,title:title.value,description:state.editor?state.editor.markdown():''},{descriptionChanged:!!state.editor?.changed()})
 }
 function refresh(){
  const changed=dirty()
  mark.hidden=!changed
  save.disabled=state.saving||(state.mode==='create'&&state.trackersLoading)||(state.mode==='edit'?!changed||!state.loaded:false)
 }

 function renderHeading(){
  headingText.textContent=state.mode==='create'?'New task':key()
  page.setAttribute('aria-label',state.mode==='create'?'New task':'Task '+key())
  save.textContent=state.mode==='create'?'Create task':'Save'
  keys.textContent=defaultActionLabel(mac)+' to '+(state.mode==='create'?'create':'save')
  const external=state.task?.externalUrl
  trackerLink.hidden=!(state.mode==='edit'&&/^https?:\/\//i.test(external||''))
  trackerLink.onclick=()=>api.openLink(external).catch(err=>setStatus(ipcMessage(err),'error'))
 }

 function renderLinks(){
  links.replaceChildren()
  const list=state.form.prLinks
  if(!list.length){const empty=element('li','task-page-links-empty','No pull request');links.append(empty)}
  for(const [index,link] of list.entries()){
   const item=element('li','task-page-link')
   const open=button(link.url,'task-page-link-url');open.title=(mac?'⌘':'Ctrl+')+'click to open '+link.url
   open.onclick=event=>{if(event.metaKey||event.ctrlKey)api.openPR(link.url).catch(err=>setStatus(ipcMessage(err),'error'))}
   item.append(open)
   if(link.state){const badge=element('span','task-page-link-state',pullRequestPresentation(link).label);badge.dataset.state=link.state;item.append(badge)}
   if(index===list.length-1){const current=element('span','task-page-link-current','Current');item.append(current)}
   const up=button('↑','icon-button','Move '+link.url+' up');up.disabled=index===0;up.onclick=()=>{state.form.prLinks=moveLink(state.form.prLinks,index,-1);renderLinks();refresh();links.querySelectorAll('.task-page-link')[index-1]?.querySelector('button[aria-label^="Move"]')?.focus()}
   const down=button('↓','icon-button','Move '+link.url+' down');down.disabled=index===list.length-1;down.onclick=()=>{state.form.prLinks=moveLink(state.form.prLinks,index,1);renderLinks();refresh()}
   const remove=button('×','icon-button','Remove '+link.url);remove.onclick=()=>{state.form.prLinks=removeLink(state.form.prLinks,index);renderLinks();refresh();linkInput.focus()}
   item.append(up,down,remove);links.append(item)
  }
 }
 const addTypedLink=()=>{
  const result=validatePullRequestUrl(linkInput.value,state.form.prLinks)
  if(!result.ok){linkNote.textContent=result.reason;linkInput.setAttribute('aria-invalid','true');linkInput.focus();return}
  linkNote.textContent='';linkInput.removeAttribute('aria-invalid');linkInput.value=''
  state.form.prLinks=addLink(state.form.prLinks,result.url);renderLinks();refresh()
 }
 linkAdd.onclick=addTypedLink
 linkInput.onkeydown=event=>{if(event.key==='Enter'&&!event.metaKey&&!event.ctrlKey){event.preventDefault();addTypedLink()}}
 linkInput.oninput=()=>{linkNote.textContent='';linkInput.removeAttribute('aria-invalid')}

 title.oninput=()=>{title.removeAttribute('aria-invalid');refresh()}
 project.onchange=()=>{project.removeAttribute('aria-invalid');state.projectId=project.value;loadTrackers()}
 tracker.onchange=()=>{loadTypes();refresh()}
 sourceToggle.onclick=()=>{
  if(!state.editor)return
  const on=!state.editor.sourceMode()
  state.editor.setSourceMode(on);sourceToggle.setAttribute('aria-pressed',String(on));sourceToggle.title=on?'Show the formatted description':'Show the Markdown source'
 }

 async function mountEditor(markdown){
  state.editor?.destroy();state.editor=null;sourceToggle.setAttribute('aria-pressed','false')
  const editor=await createMarkdownEditor(editorRoot,{markdown,openLink:href=>api.openLink(href).catch(err=>setStatus(ipcMessage(err),'error')),onChange:refresh,placeholder:'Write a description, or type / for blocks'})
  if(state.destroyed){editor.destroy();return}
  state.editor=editor;refresh()
 }

 // The trackers of the chosen project, read from its configuration.
 let trackerGeneration=0
 async function loadTrackers(){
  const generation=++trackerGeneration
  state.trackersLoading=true
  state.trackers=[];tracker.replaceChildren();trackerField.hidden=true;typeGeneration++;typeValues=[];typeRetry.hidden=true;renderTypes();typeNote.textContent='Loading trackers…';refresh()
  if(!state.projectId){state.trackersLoading=false;loadTypes();refresh();return}
  let info=null
  try{info=await api.project(state.projectId)}catch(err){
   if(generation!==trackerGeneration||state.destroyed)return
   state.trackersLoading=false;typeNote.textContent='Could not load trackers: '+ipcMessage(err);typeRetry.hidden=false;refresh();return
  }
  if(generation!==trackerGeneration||state.destroyed)return
  state.trackersLoading=false
  state.trackers=Array.isArray(info?.server?.trackers)?info.server.trackers:[]
  for(const item of state.trackers){const option=element('option','',item.name||item.scope||item.provider);option.value=item.id;tracker.append(option)}
  tracker.value=state.trackers.some(item=>item.id===info?.server?.defaultTrackerId)?info.server.defaultTrackerId:state.trackers[0]?.id||''
  if(initialTrackerId===null&&state.projectId===initialProjectId)initialTrackerId=tracker.value
  trackerField.hidden=state.trackers.length<2
  loadTypes();refresh()
 }

 function buildCreate(){
  const known=deps.projects
  project.replaceChildren()
  if(!state.projectId){const none=element('option','','Select a project');none.value='';project.append(none)}
  for(const item of known){const option=element('option','',item.name);option.value=item.id;project.append(option)}
  project.value=state.projectId
  body.replaceChildren(projectField,trackerField,titleField)
  body.append(typeField,priorityField,descriptionField,linksField)
  trackerField.hidden=true
  renderHeading();renderTypes();renderLinks()
  loadTrackers()
  mountEditor('').then(()=>{if(!state.destroyed)(state.projectId?title:project).focus()})
  ;(state.projectId?title:project).focus()
 }

 function fillFromTask(task){
  state.task=task;state.taskId=task.id
  state.loaded={title:task.title||'',issueType:task.issueType||'',priority:task.priority||'medium',prLinks:(task.prLinks||[]).map(link=>({...link}))}
  state.form={title:state.loaded.title,issueType:state.loaded.issueType,priority:state.loaded.priority,prLinks:state.loaded.prLinks.map(link=>({...link}))}
  title.value=state.loaded.title
 }
 function buildEdit(){
  const projectName=deps.projects.find(item=>item.id===state.projectId)?.name||state.projectId
  projectText.textContent=projectName
  const trackerName=state.trackers.find(item=>item.id===state.task.trackerId)
  trackerText.textContent=trackerName?.name||trackerName?.scope||state.task.source||'Tracker'
  body.replaceChildren(field('Project',projectText),field('Tracker',trackerText),titleField,typeField,priorityField,descriptionField,linksField)
  renderHeading();renderTypes();renderLinks();refresh()
 }

 // Edition reads the task fresh from the server, never from a list.
 async function loadTask(){
  state.loaded=null;state.task=null;refresh()
  body.replaceChildren();footer.hidden=true
  headingText.textContent=state.taskId;setStatus('Loading the task…')
  try{
   const [task,info]=await Promise.all([api.task(state.projectId,state.taskId),api.project(state.projectId).catch(()=>null)])
   if(state.destroyed)return
   state.trackers=Array.isArray(info?.server?.trackers)?info.server.trackers:[]
   fillFromTask(task);footer.hidden=false;setStatus('')
   buildEdit();loadTypes();await mountEditor(task.description||'')
   if(!state.destroyed&&document.activeElement===document.body)title.focus()
  }catch(err){
   if(state.destroyed)return
   setStatus('Could not load the task: '+ipcMessage(err),'error')
   const retry=button('Retry');retry.onclick=loadTask;body.replaceChildren(retry);retry.focus()
  }
 }

 function showCreated(task){
  created.replaceChildren();created.hidden=false
  const done=element('p','task-page-created-text','Created '+(task.key||task.id)+' · '+task.title)
  const actions=element('div','task-page-created-actions')
  const clarify=button('Clarify now');clarify.dataset.defaultAction=''
  const launch=button('Launch task','secondary'),another=button('Add another','secondary'),finish=button('Done','secondary')
  clarify.onclick=async()=>{
   clarify.disabled=true;setStatus('Launching clarify…')
   try{await deps.launchClarify(state.projectId,task)}catch(err){setStatus(ipcMessage(err),'error');clarify.disabled=false}
  }
  launch.onclick=()=>deps.openTickets(state.projectId,task.key||task.title)
  another.onclick=()=>deps.addAnother(state.projectId)
  finish.onclick=()=>deps.done()
  actions.append(another,launch,finish,clarify);created.append(done,actions);clarify.focus()
 }

 async function submit(){
  if(state.saving||state.mode==='create'&&state.trackersLoading||state.mode==='edit'&&(!state.loaded||!state.available))return
  const checked=validateTitle(title.value)
  if(state.mode==='create'&&!project.value){project.setAttribute('aria-invalid','true');setStatus('Select a project.','error');project.focus();return}
  if(!checked.ok){title.setAttribute('aria-invalid','true');setStatus(checked.reason,'error');title.focus();return}
  const description=state.editor?state.editor.markdown():''
  const length=validateDescription(description)
  if(!length.ok){setStatus(length.reason,'error');return}
  if(state.mode==='create')return create(checked.title,description)
  const changes=pendingChanges()
  if(!Object.keys(changes).length){refresh();return}
  state.saving=true;refresh();setStatus('Saving…')
  try{
   await api.updateTask(state.projectId,state.taskId,changes)
   const task=await api.task(state.projectId,state.taskId)
   if(state.destroyed)return
   afterSave(task,changes);setStatus('Saved','ok')
   deps.saved?.(state.projectId,task)
  }catch(err){if(!state.destroyed)setStatus('Could not save: '+ipcMessage(err),'error')}
  finally{state.saving=false;if(!state.destroyed)refresh()}
 }
 // After a write, what the server holds is the new baseline. A description
 // the server stored as sent keeps the editor as it is, cursor included.
 function afterSave(task,changes){
  fillFromTask(task);state.unsaved.clear()
  if(state.editor){
   if(!('description' in changes)||(task.description??'')===changes.description)state.editor.rebase()
   else state.editor.reset(task.description||'')
  }
  renderHeading();renderTypes();renderLinks()
 }

 async function create(titleText,description){
  state.saving=true;refresh();setStatus('Creating the task on the server and its tracker…')
  const projectID=project.value
  const extra={}
  if(state.form.prLinks.length)extra.prLinks=state.form.prLinks.map(link=>({...link}))
  let task
  try{
   const input={projectID,title:titleText,description,issueType:state.form.issueType,priority:state.form.priority}
   if(tracker.value)input.trackerID=tracker.value
   task=await api.createTask(input)
  }catch(err){
   state.saving=false
   if(!state.destroyed){setStatus(ipcMessage(err),'error');refresh()}
   return
  }
  if(state.destroyed)return
  deps.rememberProject?.(projectID)
  // The page is now the created task's: what was sent is its baseline, and
  // the fields the creation does not carry follow in one update.
  state.mode='edit';state.projectId=projectID;state.taskId=task.id
  const form={...state.form}
  fillFromTask({...task,title:task.title??titleText})
  state.form={...state.form,prLinks:form.prLinks}
  if((task.description||'')===description)state.editor?.rebase()
  else state.editor?.reset(task.description||'')
  buildEdit();loadTypes()
  let failure=''
  if(Object.keys(extra).length){
   try{
    await api.updateTask(projectID,task.id,extra)
    const fresh=await api.task(projectID,task.id)
    if(state.destroyed)return
    afterSave(fresh,{})
   }catch(err){
    if(state.destroyed)return
    failure=ipcMessage(err)
    for(const name of Object.keys(extra))state.unsaved.add(name)
   }
  }
  state.saving=false;renderHeading();refresh()
  deps.saved?.(projectID,state.task)
  if(failure){
   const what='pull requests'
   setStatus('Created '+(task.key||task.id)+', but '+what+' could not be saved: '+failure,'error')
  }else setStatus('')
  showCreated({...task,title:state.loaded.title})
 }
 save.onclick=submit
 // Cmd+Enter / Ctrl+Enter saves from anywhere on the page, the description
 // included: the capture phase runs before the editor's own keymap.
 page.addEventListener('keydown',event=>{
  if(!defaultActionShortcut({key:event.key,metaKey:event.metaKey,ctrlKey:event.ctrlKey,shiftKey:event.shiftKey,altKey:event.altKey,repeat:event.repeat,isComposing:event.isComposing,mac}))return
  event.preventDefault();event.stopPropagation()
  const defaultAction=!created.hidden&&!dirty()?created.querySelector('[data-default-action]'):null
  if(defaultAction&&!defaultAction.disabled)defaultAction.click();else submit()
 },true)

 async function start(){
  try{state.available=await api.taskPageAvailable()}catch{state.available=false}
  if(state.destroyed)return
  if(state.mode==='create'){buildCreate();refresh();return}
  if(!state.available){
   renderHeading();headingText.textContent=state.taskId;footer.hidden=true
   setStatus('The running local agent is outdated. Stop it, then start the rebuilt agent before editing a task. Closing the desktop alone does not restart the agent.','error')
   return
  }
  await loadTask()
 }
 renderHeading();refresh();start()

 return {
  dirty,
  key:()=>state.mode==='create'?'':key(),
  projectId:()=>state.projectId,
  taskId:()=>state.taskId,
  mode:()=>state.mode,
  destroy(){state.destroyed=true;typeGeneration++;trackerGeneration++;state.editor?.destroy();state.editor=null;page.remove()},
 }
}
