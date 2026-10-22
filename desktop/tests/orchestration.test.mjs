import { test } from 'node:test'
import assert from 'node:assert/strict'
import { orchestrationGroups, orchestrationProfileId, orchestrationProfileRuns, orchestrationProfileState } from '../src/orchestration.mjs'

test('orchestration includes enabled idle projects and live manual agents, excluding discussions and history',()=>{
 const projects=[{id:'idle',name:'Idle',orchestratorEnabled:true},{id:'busy',name:'Busy'},{id:'unused',name:'Unused'},{id:'removed',name:'Removed',disconnected:true,orchestratorEnabled:true}]
 const run=(id,projectId,status,extra={})=>({id,projectId,status,taskId:id,skill:'clarify',...extra})
 const runs=[run('working','busy','running'),run('waiting','busy','waiting'),run('queued','busy','queued'),run('finished','busy','completed'),run('discussion','busy','running',{skill:'discuss'}),run('console','busy','running',{kind:'console'}),run('macro','busy','running',{macroKey:'M-1'}),run('removed','removed','running')]
 const groups=orchestrationGroups(projects,runs)
 assert.deepEqual(groups.map(group=>group.project.id),['busy','idle'])
 assert.deepEqual(groups[0].agents.map(run=>run.id),['working','waiting','queued'])
 assert.deepEqual(groups[1].agents,[])
})


test('mapped projects stay out of orchestration until enabled or running an agent',()=>{
 const projects=[{id:'mapped',name:'Mapped',path:'/work/repo'},{id:'unmapped',name:'Unmapped'}]
 assert.deepEqual(orchestrationGroups(projects,[]),[])
})

test('agent overview matches stable profile IDs across renames without taking another profile or project runs',()=>{
 const snapshot=id=>'Backlog orchestration: preflight\nProject eligibility guidelines (mandatory):\nOrchestration agent: Former name ['+id+']\n\nLocal policy'
 const run=(id,projectId,prompt,createdAt)=>({id,projectId,prompt,createdAt,taskId:id,skill:'clarify',status:'failed'})
 const runs=[run('older','p',snapshot('docs'),'2026-10-22T09:00:00Z'),run('newer','p',snapshot('docs'),'2026-10-22T10:00:00Z'),run('test','p',snapshot('tests'),'2026-10-22T11:00:00Z'),run('other','q',snapshot('docs'),'2026-10-22T12:00:00Z'),run('manual','p','Orchestration agent: Former name [docs]\n\n','2026-10-22T13:00:00Z')]
 assert.deepEqual(orchestrationProfileRuns('p','docs',runs).map(run=>run.id),['newer','older'])
 assert.equal(orchestrationProfileId(runs[4]),'')
 assert.equal(orchestrationProfileId({prompt:snapshot('docs')}),'docs')
 assert.equal(orchestrationProfileId({prompt:'Manual prompt\n'+snapshot('docs')}),'')
 assert.equal(orchestrationProfileId({prompt:'Backlog orchestration: preflight\nProject eligibility guidelines (mandatory):\nLegacy local instructions'}),'default')
 assert.equal(orchestrationProfileId({prompt:snapshot('not valid')}),'')
 assert.deepEqual(orchestrationProfileRuns('p','docs',runs,[{activityId:'manual'}]).map(run=>run.id),['manual','newer','older'])
})

test('agent overview prioritizes live waits and preserves authentication failures until a later run',()=>{
 const failed={id:'failed',status:'failed',authenticationRequired:true}
 assert.equal(orchestrationProfileState([failed]).label,'Sign-in needed')
 const waiting={id:'waiting',status:'running',waitingSince:'2026-10-22T09:00:00Z'}
 assert.equal(orchestrationProfileState([failed,waiting]).label,'Waiting for you')
 assert.equal(orchestrationProfileState([{id:'new',status:'running'},failed]).label,'Running')
 assert.equal(orchestrationProfileState([{id:'done',status:'completed'},failed]).label,'Ready')
 assert.equal(orchestrationProfileState([failed],{pending:true}).label,'Evaluating backlog')
 assert.equal(orchestrationProfileState([],{error:'Unavailable'}).label,'Needs attention')
 assert.equal(orchestrationProfileState([]).label,'Ready')
})
