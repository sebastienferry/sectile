const {test}=require('node:test')
const assert=require('node:assert/strict')

async function load(){return import('../src/workflow.mjs')}

const project={configured:true,server:{skills:[{id:'implement'},{id:'handoff'}]}}

test('a reviewed task on a project offering handoff proposes its closing step',async()=>{
 const {closingStep}=await load()
 assert.deepEqual(closingStep({labels:['#reviewed']},project),{skillId:'handoff',label:'Close the task'})
 assert.deepEqual(closingStep({status:'to_close'},project),{skillId:'handoff',label:'Close the task'})
})

test('any other stage has nothing to close',async()=>{
 const {closingStep}=await load()
 for(const stage of ['new','clarified','specified','implemented','finished'])
  assert.equal(closingStep({labels:['#'+stage]},project),null,stage+' offers no closing step')
})

test('an unusable project or task offers nothing',async()=>{
 const {closingStep}=await load()
 const reviewed={labels:['#reviewed']}
 assert.equal(closingStep(null,project),null,'no task')
 assert.equal(closingStep(reviewed,null),null,'no project')
 assert.equal(closingStep(reviewed,{configured:false,server:{skills:[{id:'handoff'}]}}),null,'unconfigured project')
 assert.equal(closingStep(reviewed,{configured:true,server:{skills:[{id:'implement'}]}}),null,'handoff unavailable')
 assert.equal(closingStep(reviewed,{configured:true}),null,'no skill list')
})
