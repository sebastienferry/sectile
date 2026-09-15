const {test}=require('node:test')
const assert=require('node:assert/strict')
const run=(id,status,createdAt,startedAt)=>({id,taskId:id,status,createdAt,startedAt})
const time=hour=>`2026-09-13T${hour}:00:00Z`

test('task order prioritizes state and submission anchor without mutating histories',async()=>{
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
 assert.deepEqual(orderedTaskGroups(groups).map(group=>group.run.id),['preparing','newer','active','queued','failed','finished','canceled'])
 assert.deepEqual(groups,snapshot)
 assert.equal(orderedTaskGroups(groups)[2].run,active)
})

test('actual start time never changes the row order',async()=>{
 const {orderedTaskGroups}=await import('../src/task-order.mjs')
 const order=groups=>orderedTaskGroups(groups).map(group=>group.run.id)
 const queued=[[run('first','queued',time('09'))],[run('second','queued',time('08'))]]
 assert.deepEqual(order(queued),['first','second'])
 // The older submission starts last: it must keep its position, not jump ahead.
 const started=[[run('first','running',time('09'),time('10'))],[run('second','running',time('08'),time('11'))]]
 assert.deepEqual(order(started),['first','second'])
})

test('relaunching a listed task keeps the group anchored on its first submission',async()=>{
 const {orderedTaskGroups}=await import('../src/task-order.mjs')
 const older=[{...run('older-run','completed',time('02')),taskId:'older'}]
 const newest=[run('newest','completed',time('05'))]
 assert.deepEqual(orderedTaskGroups([older,newest]).map(group=>group.run.id),['newest','older-run'])
 // A relaunch adds a newer execution and promotes the state group, but the
 // anchor stays on the first submission.
 const relaunched=[...older,{...run('relaunch','completed',time('09')),taskId:'older'}]
 assert.deepEqual(orderedTaskGroups([relaunched,newest]).map(group=>group.run.id),['newest','relaunch'])
 const running=[...older,{...run('relaunch','running',time('09')),taskId:'older'}]
 const ordered=orderedTaskGroups([running,newest])
 assert.deepEqual(ordered.map(group=>group.run.id),['relaunch','newest'])
 assert.equal(ordered[0].run.status,'running')
})

test('fallbacks, equivalent instants and identity ties ignore response order',async()=>{
 const {orderedTaskGroups}=await import('../src/task-order.mjs')
 const groups=[
  [run('unknown-b','completed','bad','bad')],
  [run('legacy','completed',time('08'))],
  [run('invalid-start','completed',time('09'),'bad')],
  [run('unknown-a','completed')],
  [run('tie-b','completed','2026-09-13T03:00:00+02:00')],
  [run('tie-a','completed',time('01'))],
  [{...run('run-z','running',time('02')),taskId:'same'},{...run('run-a','running',time('02')),taskId:'same'}]
 ]
 const expected=['run-a','invalid-start','legacy','tie-a','tie-b','unknown-a','unknown-b']
 for(let i=0;i<groups.length;i++){
  assert.deepEqual(orderedTaskGroups(groups).map(group=>group.run.id),expected)
  groups.push(groups.shift())
  for(const executions of groups)executions.reverse()
 }
 // A group keeps a usable anchor when only some of its executions have one.
 assert.deepEqual(orderedTaskGroups([
  [{...run('partial-a','completed','bad'),taskId:'partial'},{...run('partial-b','completed',time('06')),taskId:'partial'}],
  [run('plain','completed',time('04'))]
 ]).map(group=>group.run.id),['partial-b','plain'])
 assert.deepEqual(orderedTaskGroups([]),[])
})
