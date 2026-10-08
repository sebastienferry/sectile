// A run works for one project (#741). A ticket may belong to several; a launch
// from the desktop names the project whose board it is made from, which the
// agent forwards to the run, so the server never has to ask which one. This
// module holds the request the main process builds, so a test can read it
// without Electron.

// The request a launch from a project's board sends to the local agent: the
// project travels in the address, and the agent forwards it to the run.
// An absent mode means "no override": nothing is sent for it.
function launchRequest(projectId,taskID,skillID,prompt,mode,force,view){
 return {
  route:'/desktop/tasks?projectId='+encodeURIComponent(projectId),
  body:Object.assign({taskID,skillID,prompt},mode?{mode}:null,force?{force:true}:null,view==='conversation'?{view}:null),
 }
}

module.exports={launchRequest}
