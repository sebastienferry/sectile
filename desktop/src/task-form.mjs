// The rules of the desktop task page (#805), kept free of the DOM so that
// node --test checks them: what a field accepts, what a save sends, and where
// the page starts.

export const TITLE_LIMIT=500
export const DESCRIPTION_LIMIT=60000

export function validateTitle(value){
 const title=String(value??'').trim()
 if(!title)return {ok:false,reason:'A title is required.'}
 if(title.length>TITLE_LIMIT)return {ok:false,reason:'The title is too long ('+title.length.toLocaleString('en-US')+' of '+TITLE_LIMIT+' characters).'}
 return {ok:true,title}
}

export function validateDescription(value){
 const length=String(value??'').length
 if(length>DESCRIPTION_LIMIT)return {ok:false,reason:'The description is too long ('+length.toLocaleString('en-US')+' of '+DESCRIPTION_LIMIT.toLocaleString('en-US')+' characters).'}
 return {ok:true}
}

// A pull request link is an absolute web address without credentials, and
// appears once in the task's set.
export function validatePullRequestUrl(value,links=[]){
 const text=String(value??'').trim()
 if(!text)return {ok:false,reason:'Enter a pull request link.'}
 let url
 try{url=new URL(text)}catch{return {ok:false,reason:'Enter an absolute http or https link.'}}
 if(!['http:','https:'].includes(url.protocol))return {ok:false,reason:'Enter an absolute http or https link.'}
 if(url.username||url.password)return {ok:false,reason:'A link with credentials is refused.'}
 if(links.some(link=>link.url===text))return {ok:false,reason:'This pull request is already listed.'}
 return {ok:true,url:text}
}

// Loaded links are kept as they are, with their state and branch; an added
// one carries its address only.
export const addLink=(links,url)=>[...links,{url}]
export const removeLink=(links,index)=>links.filter((_,i)=>i!==index)
export function moveLink(links,index,delta){
 const target=index+delta
 if(index<0||index>=links.length||target<0||target>=links.length)return links
 const next=[...links];[next[index],next[target]]=[next[target],next[index]];return next
}
// The task's current pull request is the last of its set.
export const currentLink=links=>links.length?links[links.length-1]:null
const sameLinks=(a,b)=>a.length===b.length&&a.every((link,i)=>link.url===b[i].url)

// taskChanges is the update a save sends: only the fields that differ from
// what was loaded. The description is compared by the editor, which knows
// whether its document moved, so its Markdown text is never compared here.
export function taskChanges(loaded,form,{descriptionChanged=false}={}){
 const changes={}
 const title=String(form.title??'').trim()
 if(title!==String(loaded.title??''))changes.title=title
 if(descriptionChanged)changes.description=String(form.description??'')
 if(String(form.assignee??'')!==String(loaded.assignee??'')||String(form.assigneeAccountId??'')!==String(loaded.assigneeAccountId??'')){
  changes.assignee=String(form.assignee??'')
  changes.assigneeAccountId=String(form.assigneeAccountId??'')
  changes.assigneeAvatar=String(form.assigneeAvatar??'')
 }
 const links=form.prLinks||[]
 if(!sameLinks(links,loaded.prLinks||[]))changes.prLinks=links.map(link=>({...link}))
 return changes
}

// Assignees are searched on the trackers that can look a person up; on the
// others the field is a free login, as in the web board.
export const assigneeLookup=provider=>['jira','gitlab'].includes(String(provider||'').toLowerCase())

// The project a creation starts on: the one it was opened from, the selected
// one, the one last used, else the only one there is.
export function initialProject({openedFrom,selected,remembered,known=[]}){
 const found=[openedFrom,selected,remembered].find(id=>id&&known.some(project=>project.id===id))
 return found||(known.length===1?known[0].id:'')
}

export const leaveMessage=key=>'Discard unsaved changes to '+(key||'the new task')+'?'
