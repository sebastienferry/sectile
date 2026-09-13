const {test}=require('node:test')
const assert=require('node:assert/strict')
const run=(id,status,createdAt,startedAt)=>({id,taskId:id,status,createdAt,startedAt})
const time=hour=>`2026-09-13T${hour}:00:00Z`

test('task order prioritizes state and actual start without mutating histories',async()=>{
 const {orderedTaskGroups}=await import('../src/task-order.mjs')
 const active=run('active','running',time('01'),time('08'))
 const newer=run('newer','running',time('05'),time('07'))
 const groups=[
  [run('finished','completed',time('12'),time('11'))],
  [run('queued','queued',time('10'))],
  [run('next','queued',time('12')),active,run('previous','completed',time('11'))],
  [newer],
  [run('preparing','preparing',time('09'),time('23'))],
  [run('failed','failed',time('12'),time('10'))],
  [run('canceled','canceled',time('03'))]
 ]
 const snapshot=structuredClone(groups)
 assert.deepEqual(orderedTaskGroups(groups).map(group=>group.run.id),['preparing','active','newer','queued','finished','failed','canceled'])
 assert.deepEqual(groups,snapshot)
 assert.equal(orderedTaskGroups(groups)[1].run,active)
})

test('fallbacks, equivalent instants and identity ties ignore response order',async()=>{
 const {orderedTaskGroups}=await import('../src/task-order.mjs')
 const groups=[
  [run('unknown-b','completed','bad','bad')],
  [run('legacy','completed',time('08'))],
  [run('invalid-start','completed',time('09'),'bad')],
  [run('unknown-a','completed')],
  [run('tie-b','completed',time('01'),'2026-09-13T12:00:00+02:00')],
  [run('tie-a','completed',time('01'),time('10'))],
  [{...run('run-z','running',time('02')),taskId:'same'},{...run('run-a','running',time('02')),taskId:'same'}]
 ]
 const expected=['run-a','tie-a','tie-b','invalid-start','legacy','unknown-a','unknown-b']
 for(let i=0;i<groups.length;i++){
  assert.deepEqual(orderedTaskGroups(groups).map(group=>group.run.id),expected)
  groups.push(groups.shift())
  for(const executions of groups)executions.reverse()
 }
 assert.deepEqual(orderedTaskGroups([]),[])
})
