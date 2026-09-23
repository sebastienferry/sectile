export function currentPullRequest(task) {
 const links=task?.prLinks
 if(links?.length)return links[links.length-1]
 return task?.prUrl?{url:task.prUrl}:undefined
}
const paths={
 open:'<circle cx="6" cy="5" r="3"/><circle cx="6" cy="19" r="3"/><circle cx="18" cy="19" r="3"/><path d="M6 8v8M18 16V9a4 4 0 0 0-4-4h-2m3-3-3 3 3 3"/>',
 merged:'<circle cx="6" cy="5" r="3"/><circle cx="18" cy="19" r="3"/><path d="M6 8v2a9 9 0 0 0 9 9M18 16V3m-3 3 3-3 3 3"/>',
 closed:'<circle cx="6" cy="5" r="3"/><circle cx="6" cy="19" r="3"/><path d="M6 8v8m9-12 6 6m0-6-6 6M18 16v5"/>',
 conflicting:'<path d="m12 3 10 18H2L12 3Z M12 9v5m0 3v1"/>'
}
export function pullRequestPresentation(link) {
 const state=Object.hasOwn(paths,link?.state)?link.state:'unknown'
 const label={open:'Open',merged:'Merged',closed:'Closed without merge',conflicting:'Conflicting',unknown:'State unknown'}[state]
 const color={open:'#4ade80',merged:'#c084fc',closed:'#f87171',conflicting:'#fbbf24',unknown:'#94a3b8'}[state]
 return {state,label,color,icon:'<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">'+(paths[state]||paths.open)+'</svg>'}
}
export function renderPullRequestIndicator(element,link,label) {
 const presentation=pullRequestPresentation(link)
 element.innerHTML=presentation.icon
 element.style.color=presentation.color
 element.title='Open '+label+' — '+presentation.label+' — '+link.url
 element.setAttribute('aria-label','Open '+label+' — '+presentation.label)
}
