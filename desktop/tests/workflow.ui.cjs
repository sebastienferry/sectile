const {test}=require('node:test')
const assert=require('node:assert/strict')

test('next step follows canonical labels, status aliases and configured skills',async()=>{
 const {taskStage,nextTaskStep}=await import('../src/workflow.mjs')
 const project={configured:true,server:{skills:['clarify','specify','implement','adjust'].map(id=>({id}))}}
 for(const [stage,skill] of Object.entries({new:'clarify',clarified:'specify',specified:'implement'})){
  assert.equal(nextTaskStep({labels:['#'+stage]},project).skillId,skill)
 }
 // After implementation, the pull request record decides between adjusting it and recovering it.
 const withPR={labels:['#implemented'],prUrl:'https://example.test/pr/1'},withoutPR={labels:['#implemented']}
 assert.deepEqual([nextTaskStep(withPR,project).skillId,nextTaskStep(withPR,project).label],['adjust','Adjust'])
 assert.deepEqual([nextTaskStep(withoutPR,project).skillId,nextTaskStep(withoutPR,project).label],['implement','Create PR'])
 assert.equal(nextTaskStep({labels:['#implemented'],prUrl:'  '},project).skillId,'implement','A blank link is no pull request')
 assert.equal(nextTaskStep(withoutPR,{...project,server:{...project.server,prCreationStage:'implemented'}}).skillId,'implement')
 assert.equal(nextTaskStep(withoutPR,{...project,server:{...project.server,prCreationStage:'specified'}}).skillId,'specify')
 assert.equal(nextTaskStep(withoutPR,{...project,server:{...project.server,prCreationStage:'specified'}}).label,'Create PR')
 const withoutAdjust={configured:true,server:{skills:['implement'].map(id=>({id}))}}
 assert.equal(nextTaskStep(withPR,withoutAdjust).skillId,undefined)
 assert.equal(nextTaskStep(withPR,withoutAdjust).message,'Next skill is unavailable: Adjust')
 assert.equal(nextTaskStep(withoutPR,{configured:true,server:{skills:[]}}).message,'Next skill is unavailable: Create PR')
 for(const [status,stage] of Object.entries({to_clarify:'new',to_specify:'clarified',to_implement:'specified',in_progress:'specified',to_test:'implemented',to_validate:'implemented',to_close:'reviewed',done:'finished'}))assert.equal(taskStage({status}),stage)
 assert.equal(taskStage({labels:['#new',' ##IMPLEMENTED ']}),'implemented')
 assert.equal(nextTaskStep({status:'reviewed'},project).message,'Next skill is unavailable: Handoff')
 const projectWithHandoff={...project,server:{...project.server,skills:[...project.server.skills,{id:'handoff'}]}}
 assert.equal(nextTaskStep({status:'reviewed'},projectWithHandoff).skillId,'handoff')
 assert.equal(nextTaskStep({status:'reviewed'},projectWithHandoff).label,'Handoff')
 assert.equal(nextTaskStep({status:'reviewed'},projectWithHandoff).message,'Ready for the next step')
 assert.equal(nextTaskStep({status:'finished'},project).skillId,undefined)
 assert.equal(nextTaskStep({status:'new'},{configured:false}).skillId,undefined)
 assert.equal(nextTaskStep({status:'new'},{configured:true,server:{skills:[]}}).skillId,undefined)
 assert.equal(nextTaskStep({status:'unknown'},project).skillId,undefined)
})
