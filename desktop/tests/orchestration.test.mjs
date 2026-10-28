import { test } from 'node:test'
import assert from 'node:assert/strict'
import { isOrchestrationSession, orchestrationGroups, orchestrationProfileRuns, orchestrationProfileState } from '../src/orchestration.mjs'

const prompt='Backlog orchestration: preflight\nProject eligibility guidelines (mandatory):\nOrchestration agent: Docs [docs]\n\nPolicy'
const projects=[{id:'p',name:'Project',orchestratorEnabled:true},{id:'q',name:'Disabled'},{id:'gone',name:'Disconnected',disconnected:true,orchestratorEnabled:true}]

test('current discovery has no authoritative dedicated-session producer; workers never grant membership',()=>{
 for(const status of ['preparing','pending','queued','running','waiting','completed','failed','canceled']){
  for(const skill of ['clarify','specify','implement','adjust','handoff','discuss','orchestrate']){
   for(const extra of [{},{restored:true},{waitingSince:'2026-10-22T09:00:00Z'},{kind:'console'},{macroKey:'M-1'},{taskId:undefined},{role:'orchestration',orchestrationAgentId:'docs'}]){
    const runs=['p','q','gone'].map(projectId=>({id:projectId,projectId,taskId:'task',skill,status,prompt,sessionId:'session',authenticationRequired:true,...extra}))
    const snapshot=structuredClone(runs)
    assert.ok(runs.every(run=>!isOrchestrationSession(run)))
    const groups=orchestrationGroups(projects,runs)
    assert.deepEqual(groups.map(group=>group.project.id),['p'])
    assert.deepEqual(groups[0].agents,[])
    assert.deepEqual(orchestrationProfileRuns('p','docs',runs,[{activityId:'p'}]),[])
    assert.deepEqual(orchestrationProfileState(runs),{id:'completed',label:'Ready',run:undefined})
    assert.deepEqual(runs,snapshot)
   }
  }
 }
})

test('enablement and refreshed or restored worker history cannot retain a disabled project',()=>{
 const runs=[{id:'worker',projectId:'p',status:'running',prompt,restored:true}]
 assert.equal(orchestrationGroups(projects,runs).length,1)
 assert.deepEqual(orchestrationGroups(projects.map(project=>({...project,orchestratorEnabled:false})),runs),[])
 assert.equal(orchestrationGroups(projects,[]).length,1)
 assert.deepEqual(orchestrationProfileRuns('q','docs',runs),[])
 assert.deepEqual(orchestrationProfileRuns('p','renamed',runs),[])
})

test('policy evaluation state remains independent of worker failures and waiting',()=>{
 const workers=[{status:'failed',authenticationRequired:true},{status:'running',waitingSince:'2026-10-22T09:00:00Z'}]
 assert.deepEqual(orchestrationProfileState(workers,{pending:true}),{id:'queued',label:'Evaluating backlog'})
 assert.equal(orchestrationProfileState(workers,{error:'Unavailable'}).label,'Needs attention')
 assert.equal(orchestrationProfileState(workers).label,'Ready')
})
