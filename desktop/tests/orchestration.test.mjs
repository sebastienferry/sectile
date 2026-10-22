import { test } from 'node:test'
import assert from 'node:assert/strict'
import { orchestrationGroups } from '../src/orchestration.mjs'

test('orchestration includes enabled idle projects and live manual agents, excluding discussions and history',()=>{
 const projects=[{id:'idle',name:'Idle',orchestratorEnabled:true},{id:'busy',name:'Busy'},{id:'unused',name:'Unused'},{id:'removed',name:'Removed',disconnected:true,orchestratorEnabled:true}]
 const run=(id,projectId,status,extra={})=>({id,projectId,status,taskId:id,skill:'clarify',...extra})
 const runs=[run('working','busy','running'),run('waiting','busy','waiting'),run('queued','busy','queued'),run('finished','busy','completed'),run('discussion','busy','running',{skill:'discuss'}),run('console','busy','running',{kind:'console'}),run('macro','busy','running',{macroKey:'M-1'}),run('removed','removed','running')]
 const groups=orchestrationGroups(projects,runs)
 assert.deepEqual(groups.map(group=>group.project.id),['busy','idle'])
 assert.deepEqual(groups[0].agents.map(run=>run.id),['working','waiting','queued'])
 assert.deepEqual(groups[1].agents,[])
})
