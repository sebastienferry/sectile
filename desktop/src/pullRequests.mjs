export function currentPullRequest(task) {
 const links=task?.prLinks
 if(links?.length)return links[links.length-1]
 return task?.prUrl?{url:task.prUrl}:undefined
}
// The repository a pull request URL names, as host/path in lower case, the
// identity the server derives: a GitHub pull request on a host naming GitHub,
// or a GitLab merge request on any host. Empty for anything else.
export function pullRequestRepository(value) {
 let url
 try{url=new URL(String(value||'').trim())}catch{return ''}
 if(!['https:','http:'].includes(url.protocol)||url.username||url.password||url.search||url.hash)return ''
 const host=url.hostname.toLowerCase(),parts=url.pathname.replace(/^\/+|\/+$/g,'').split('/'),n=parts.length
 if(parts.some(part=>part===''||part==='.'||part==='..'))return ''
 let repository='',number=''
 if(n>=5&&parts[n-3]==='-'&&parts[n-2]==='merge_requests'){repository=parts.slice(0,n-3).join('/');number=parts[n-1]}
 else if(n===4&&parts[2]==='pull'&&host.includes('github')){repository=parts.slice(0,2).join('/');number=parts[3]}
 if(!repository||!/^[1-9]\d*$/.test(number))return ''
 return (host+'/'+repository).toLowerCase()
}
// The current pull request of each repository the task changed, the primary
// repository's first. The server keeps the primary one last in prLinks; a link
// naming no repository belongs to the primary one, as every link once did.
export function repositoryPullRequests(task) {
 const links=task?.prLinks?.length?task.prLinks:task?.prUrl?[{url:task.prUrl}]:[]
 if(!links.length)return []
 const repositoryOf=link=>link.repository||pullRequestRepository(link.url)
 const primary=repositoryOf(links[links.length-1]),current=new Map([[primary,null]])
 for(const link of links)current.set(repositoryOf(link)||primary,link)
 return [...current].map(([repository,link])=>({...link,repository}))
}
// The last segment of a repository identity, enough to tell two of a task's
// repositories apart in the toolbar.
export function repositoryName(repository) {return String(repository||'').split('/').pop()||'PR / MR'}
export function prLabel(value) {
 try{
  const url=new URL(value)
  const match=url.pathname.match(/\/(pull|merge_requests)\/(\d+)/)
  return match?(match[1]==='merge_requests'?'MR !':'PR #')+match[2]:'PR / MR'
 }catch{return 'PR / MR'}
}
// The entries of the toolbar's pull request menu: every repository's current
// pull request, primary first. Empty below two, where the button suffices.
export function pullRequestMenuEntries(links) {
 if(!links||links.length<2)return []
 return links.map(link=>({url:link.url,repository:link.repository,name:repositoryName(link.repository),label:prLabel(link.url),link}))
}
const paths={
 open:'<circle cx="6" cy="5" r="3"/><circle cx="6" cy="19" r="3"/><circle cx="18" cy="19" r="3"/><path d="M6 8v8M18 16V9a4 4 0 0 0-4-4h-2m3-3-3 3 3 3"/>',
 merged:'<circle cx="6" cy="5" r="3"/><circle cx="18" cy="19" r="3"/><path d="M6 8v2a9 9 0 0 0 9 9M18 16V3m-3 3 3-3 3 3"/>',
 closed:'<circle cx="6" cy="5" r="3"/><circle cx="6" cy="19" r="3"/><path d="M6 8v8m9-12 6 6m0-6-6 6M18 16v5"/>',
 conflicting:'<path d="m12 3 10 18H2L12 3Z M12 9v5m0 3v1"/>'
}
export function pullRequestPresentation(link) {
 const state=Object.hasOwn(paths,link?.state)?link.state:'unknown'
 let label={open:'Open',merged:'Merged',closed:'Closed without merge',conflicting:'Conflicting',unknown:'State unknown'}[state]
 // A state left unknown for want of a token says which token is missing.
 if(state==='unknown'&&link?.missingToken)label+=': no '+(link.missingToken==='gitlab'?'GitLab':'GitHub')+' token'
 // The tones live in the stylesheet's colour tokens, so they follow the appearance.
 const color='var(--pr-'+state+')'
 return {state,label,color,icon:'<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">'+(paths[state]||paths.open)+'</svg>'}
}
export function renderPullRequestIndicator(element,link,label) {
 const presentation=pullRequestPresentation(link)
 element.innerHTML=presentation.icon
 element.style.color=presentation.color
 element.title='Open '+label+' — '+presentation.label+' — '+link.url
 element.setAttribute('aria-label','Open '+label+' — '+presentation.label)
}
