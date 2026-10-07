// The folders of an execution listed after its path (#762): the list
// the agent keeps for the run, the run's own directory first. A run with one
// folder, or from an agent that sends none, offers nothing beyond its path.
export function menuFolders(run){
 const folders=Array.isArray(run?.folders)?run.folders.filter(folder=>typeof folder?.path==='string'&&folder.path):[]
 return folders.length>1?folders:[]
}

const ROLE_LABELS={primary:'primary',changed:'changed',context:'context',spec:'specifications',local:'attached'}

// What an item says the folder is to the run. An attached folder reads as
// attached whatever its repository role: that is how the person added it.
export function folderRoleLabel(folder){
 if(folder?.attached)return 'attached'
 return ROLE_LABELS[folder?.role]||String(folder?.role||'')
}

// The folder of a run that path selects (#784), as the run lists it, or null
// for the run's own directory: the primary folder, a path the list no longer
// holds, or a run that offers no list.
export function chosenFolder(run,path){
 if(!path)return null
 const folder=menuFolders(run).find(item=>item.path===path)
 return folder&&folder.path!==run.directory&&folder.role!=='primary'?folder:null
}
