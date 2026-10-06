import { test } from 'node:test'
import assert from 'node:assert/strict'
import { followedExecution } from '../src/task-order.mjs'

const run=(id,taskId,status,hour,extra={})=>({id,taskId,projectId:'project',status,createdAt:`2026-09-29T${hour}:00:00Z`,...extra})
const keyOf=run=>JSON.stringify([run.projectId,run.kind==='console'?run.id:run.taskId])
const eligible=run=>run.kind!=='console'&&!run.hidden
const follow=(previous,next,selected)=>followedExecution(previous,next,selected,keyOf,eligible)?.id??null

test('follows a new execution of the displayed ticket',()=>{
 const shown=run('a','1','completed','09'),other=run('x','2','completed','08')
 assert.equal(follow([shown,other],[shown,other,run('b','1','running','10')],'a'),'b')
 // A queued execution is followed as soon as it appears.
 assert.equal(follow([shown],[shown,run('b','1','queued','10')],'a'),'b')
 // The displayed execution may still be running: the new one takes over.
 const live=run('a','1','running','09')
 assert.equal(follow([live],[live,run('b','1','queued','10')],'a'),'b')
 // An older execution picked from the history is left too.
 const older=run('old','1','completed','07')
 assert.equal(follow([older,shown],[older,shown,run('b','1','running','10')],'old'),'b')
})

test('follows nothing that is not a new execution of the displayed ticket',()=>{
 const shown=run('a','1','completed','09')
 assert.equal(follow([shown],[shown],'a'),null)
 assert.equal(follow([shown],[shown,run('b','2','running','10')],'a'),null)
 assert.equal(follow([shown],[shown,run('b','1','running','10',{hidden:true})],'a'),null)
 assert.equal(follow([shown],[shown,run('b','1','running','10')],null),null)
 // The first list after a start or a restart is a baseline.
 assert.equal(follow([],[shown,run('b','1','running','10')],'a'),null)
 // A free console on display never moves.
 const free=run('c',undefined,'running','09',{kind:'console'})
 assert.equal(follow([free],[free,run('b','1','running','10')],'c'),null)
})

test('ranks several new executions like the row does',()=>{
 const shown=run('a','1','completed','09')
 const next=[shown,run('queued','1','queued','12'),run('running','1','running','10'),run('done','1','completed','11')]
 assert.equal(follow([shown],next,'a'),'running')
 assert.equal(follow([shown],[shown,run('b','1','queued','10'),run('c','1','queued','11')],'a'),'c')
})
