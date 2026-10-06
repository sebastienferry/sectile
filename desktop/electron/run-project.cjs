// A run works for one project (#741). A ticket may belong to several; a launch
// from the desktop names the project whose board it is made from, and should
// the server still answer with candidates, the person picks one in a native
// dialog. This module holds what the main process builds for both, so a test
// can read it without Electron.

// The request a launch from a project's board sends to the local agent: the
// project travels in the address, and the agent forwards it to the run.
// An absent mode means "no override": nothing is sent for it.
function launchRequest(projectId,taskID,skillID,prompt,mode,force,view){
 return {
  route:'/desktop/tasks?projectId='+encodeURIComponent(projectId),
  body:Object.assign({taskID,skillID,prompt},mode?{mode}:null,force?{force:true}:null,view==='conversation'?{view}:null),
 }
}

function candidatesOf(candidates){
 return (Array.isArray(candidates)?candidates:[]).filter(candidate=>candidate&&candidate.id)
}

// The native dialog asking which project a run works for: one button per
// candidate, then Cancel, which is also what closing the dialog answers.
function runProjectDialog(candidates,taskLabel){
 const list=candidatesOf(candidates)
 return {
  type:'question',
  buttons:[...list.map(candidate=>String(candidate.name||candidate.id)),'Cancel'],
  defaultId:0,
  cancelId:list.length,
  noLink:true,
  message:'Which project is this run for?',
  detail:(taskLabel?String(taskLabel):'This ticket')+' belongs to several projects. The run works in the repositories and with the rules of the project you choose.',
 }
}

// The project the dialog's answer names, or null for Cancel.
function chosenRunProject(candidates,response){
 const list=candidatesOf(candidates)
 return Number.isInteger(response)&&response>=0&&response<list.length?String(list[response].id):null
}

module.exports={launchRequest,runProjectDialog,chosenRunProject}
