const {test}=require('node:test')
const assert=require('node:assert/strict')

test('next step follows canonical labels, status aliases and configured skills',async()=>{
 const {taskStage,nextTaskStep}=await import('../src/workflow.mjs')
 const project={configured:true,server:{skills:['clarify','specify','implement','create_pr'].map(id=>({id}))}}
 for(const [stage,skill] of Object.entries({new:'clarify',clarified:'specify',specified:'implement',implemented:'create_pr'})){
  assert.equal(nextTaskStep({labels:['#'+stage]},project).skillId,skill)
 }
 for(const [status,stage] of Object.entries({to_clarify:'new',to_specify:'clarified',to_implement:'specified',in_progress:'specified',to_test:'implemented',to_validate:'implemented',to_close:'reviewed',done:'finished'}))assert.equal(taskStage({status}),stage)
 assert.equal(taskStage({labels:['#new',' ##IMPLEMENTED ']}),'implemented')
 assert.equal(taskStage({status:'done',labels:['#new']}),'finished')
 assert.equal(nextTaskStep({status:'reviewed'},project).message,'Awaiting human merge')
 assert.equal(nextTaskStep({status:'finished'},project).skillId,undefined)
 assert.equal(nextTaskStep({status:'new'},{configured:false}).skillId,undefined)
 assert.equal(nextTaskStep({status:'new'},{configured:true,server:{skills:[]}}).skillId,undefined)
 assert.equal(nextTaskStep({status:'unknown'},project).skillId,undefined)
})
