// The Tickets pane shows one project, or a saved board view spanning several
// (#429). In a view, every row belongs to its own project: its skills, next step
// and launch come from that project, never from the view.

export const TICKET_COLUMNS=[['state',''],['key','Key'],['title','Title'],['stage','Stage'],['priority','Priority'],['pr','PR'],['actions','Actions']]

// A view's rows name their project, right after the key.
export function ticketColumns(boardView){
 if(!boardView)return TICKET_COLUMNS
 const columns=[...TICKET_COLUMNS]
 columns.splice(2,0,['project','Project'])
 return columns
}

export function rowProjectID(view,task){
 return view.boardView?task.projectId:view.projectID
}

// The project info of a row. A view with a folder on this workstation makes
// every row launchable there, mapped project or not: the agent runs launches
// from the view in that folder. An unreadable project is not configured.
export function rowInfo(view,task){
 if(!view.boardView)return view.info
 const info=view.infos.get(task.projectId)||{configured:false,server:{skills:[]}}
 return view.boardView.directory?{...info,configured:true}:info
}

// The distinct projects of a view's tasks, in first-seen order: one project
// read serves every row of that project.
export function viewProjectIDs(tasks){
 return [...new Set(tasks.map(task=>task.projectId).filter(Boolean))]
}
