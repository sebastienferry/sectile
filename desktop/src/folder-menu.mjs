// The folders of an execution that its path offers to copy (#762): the list
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
