const {test}=require('node:test')
const assert=require('node:assert/strict')

test('queue uses daemon order, retains repeated tasks, and ignores finished runs',async()=>{
 const {orderedQueueRuns}=await import('../src/queue.mjs')
 const runs=[
  {id:'new',taskId:'same',status:'queued',queueSequence:3,createdAt:'2026-09-13T01:00:00Z'},
  {id:'active',taskId:'same',status:'running',queueSequence:1},
  {id:'old',taskId:'same',status:'queued',queueSequence:2,createdAt:'2026-09-13T02:00:00Z'},
  {id:'done',status:'completed',queueSequence:0},
  {id:'canceled',status:'canceled',queueSequence:1},
  {id:'canceling',status:'queued',cancelRequested:true,queueSequence:1}
 ]
 const snapshot=structuredClone(runs)
 assert.deepEqual(orderedQueueRuns(runs).map(run=>run.id),['old','new'])
 assert.deepEqual(runs,snapshot)
 assert.deepEqual(orderedQueueRuns([...runs].reverse()).map(run=>run.id),['old','new'])
})

test('older agents fall back to submission time',async()=>{
 const {orderedQueueRuns}=await import('../src/queue.mjs')
 const runs=[{id:'b',status:'queued',createdAt:'2026-09-13T02:00:00Z'},{id:'a',status:'queued',createdAt:'2026-09-13T01:00:00Z'}]
 assert.deepEqual(orderedQueueRuns(runs).map(run=>run.id),['a','b'])
 assert.deepEqual(orderedQueueRuns([]),[])
})
