// The engine catalogue of the workstation, as the desktop shows it (#510).
// Pure helpers: the agent owns the catalogue and the task choices, these only
// read what it answers.

// nextEngine is the engine a click moves a task to: the one after its current
// engine in catalogue order, the last one wrapping to the first. A current
// engine that is not in the catalogue reads as the project default one.
export function nextEngine(catalogue,currentId,projectDefaultId){
 const list=Array.isArray(catalogue)?catalogue:[]
 if(!list.length)return null
 let index=list.findIndex(engine=>engine.id===currentId)
 if(index<0)index=list.findIndex(engine=>engine.id===projectDefaultId)
 if(index<0)return list[0]
 return list[(index+1)%list.length]
}

// taskEngine is the engine a task runs: its stored choice when it names a
// catalogue engine, else the project default engine.
export function taskEngine(view,taskId){
 const list=view?.catalogue||[]
 const byId=id=>list.find(engine=>engine.id===id)
 return byId(view?.tasks?.[taskId])||byId(view?.projectDefault)||list[0]||null
}

// The letters drawn in a provider's icon. No third-party logo is used.
const MARKS={claude:'Cl',codex:'Cx',agy:'Ag',gemini:'Ge',cursor:'Cu',vibe:'Vi',custom:'{}'}
export function engineMark(provider){
 const id=String(provider||'').trim().toLowerCase()
 return MARKS[id]||(id?id.slice(0,2).replace(/^./,c=>c.toUpperCase()):'?')
}

// engineTooltip names the engine, its provider and its model.
export function engineTooltip(engine,isProjectDefault){
 if(!engine)return 'No engine'
 const model=String(engine.model||'').trim()||'provider default'
 return engine.name+' - '+engine.provider+' · '+model+(isProjectDefault?' (project default)':'')
}

// moveEngine returns the catalogue with one engine moved by delta places.
export function moveEngine(catalogue,id,delta){
 const list=[...catalogue]
 const from=list.findIndex(engine=>engine.id===id)
 const to=from+delta
 if(from<0||to<0||to>=list.length)return list
 const [engine]=list.splice(from,1)
 list.splice(to,0,engine)
 return list
}

// removalImpact says what removing an engine affects, for its confirmation.
export function removalImpact(view,id,projectNames={}){
 const projects=Object.entries(view?.projects||{}).filter(([,engine])=>engine===id).map(([project])=>projectNames[project]||project).sort()
 return {projects,tasks:Number(view?.taskCounts?.[id])||0}
}

// removalMessage is the sentence the confirmation shows.
export function removalMessage(name,impact){
 const parts=[]
 if(impact.projects.length)parts.push((impact.projects.length===1?'project ':'projects ')+impact.projects.join(', ')+' will use the workstation default engine')
 if(impact.tasks)parts.push(impact.tasks+(impact.tasks===1?' task':' tasks')+' will use their project default engine')
 return 'Remove '+name+'?'+(parts.length?' '+parts.join('; ')+'.':' No project or task uses it.')
}
