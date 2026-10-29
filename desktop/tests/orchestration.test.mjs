import { test } from 'node:test'
import assert from 'node:assert/strict'
import { isOrchestrationSession, orchestrationDecisionLine, orchestrationGroups, orchestrationProfileRuns, orchestrationProfileState } from '../src/orchestration.mjs'

const prompt='Backlog orchestration: launched by orchestration agent "Docs" (docs).\nWhy this ticket now: unblocks the API.'
const projects=[{id:'p',name:'Project',orchestratorEnabled:true},{id:'q',name:'Disabled'},{id:'gone',name:'Disconnected',disconnected:true,orchestratorEnabled:true}]
const session=(extra={})=>({id:'s',projectId:'p',scope:'orchestrator',profileId:'docs',skill:'orchestrate_backlog',status:'running',createdAt:'2026-10-29T09:00:00Z',...extra})

test('only the agent-marked orchestrator session is one; the workers it launched never are',()=>{
 for(const status of ['preparing','pending','queued','running','waiting','completed','failed','canceled']){
  for(const skill of ['clarify','specify','implement','adjust','handoff','discuss','orchestrate']){
   for(const extra of [{},{restored:true},{waitingSince:'2026-10-22T09:00:00Z'},{kind:'console'},{macroKey:'M-1'},{taskId:undefined},{role:'orchestration',orchestrationAgentId:'docs'},{profileId:'docs'}]){
    const runs=['p','q','gone'].map(projectId=>({id:projectId,projectId,taskId:'task',skill,status,prompt,sessionId:'session',authenticationRequired:true,...extra}))
    const snapshot=structuredClone(runs)
    assert.ok(runs.every(run=>!isOrchestrationSession(run)))
    const groups=orchestrationGroups(projects,runs)
    assert.deepEqual(groups.map(group=>group.project.id),['p'])
    assert.deepEqual(groups[0].agents,[])
    assert.deepEqual(orchestrationProfileRuns('p','docs',runs),[])
    assert.deepEqual(orchestrationProfileState(runs),{id:'completed',label:'Ready',run:undefined})
    assert.deepEqual(runs,snapshot)
   }
  }
 }
})

test('a session belongs to its project and its profile',()=>{
 const runs=[session(),session({id:'old',status:'completed',createdAt:'2026-10-29T08:00:00Z'}),session({id:'other',profileId:'tests'}),session({id:'elsewhere',projectId:'q'})]
 assert.deepEqual(orchestrationProfileRuns('p','docs',runs).map(run=>run.id),['s','old'])
 assert.deepEqual(orchestrationProfileRuns('p','tests',runs).map(run=>run.id),['other'])
 assert.deepEqual(orchestrationProfileRuns('p','renamed',runs),[])
 // A live session keeps a disabled project's group visible until it ends.
 assert.deepEqual(orchestrationGroups(projects,runs).map(group=>group.project.id),['q','p'])
})

test('enablement and refreshed or restored worker history cannot retain a disabled project',()=>{
 const runs=[{id:'worker',projectId:'p',status:'running',prompt,restored:true}]
 assert.equal(orchestrationGroups(projects,runs).length,1)
 assert.deepEqual(orchestrationGroups(projects.map(project=>({...project,orchestratorEnabled:false})),runs),[])
 assert.equal(orchestrationGroups(projects,[]).length,1)
 assert.deepEqual(orchestrationProfileRuns('q','docs',runs),[])
})

test('a started agent reads as started between reviews, and as reviewing during one',()=>{
 assert.deepEqual(orchestrationProfileState([],{pending:true}),{id:'queued',label:'Starting'})
 assert.equal(orchestrationProfileState([],{started:true}).label,'Started')
 assert.equal(orchestrationProfileState([session()],{started:true}).label,'Reviewing backlog')
 assert.equal(orchestrationProfileState([session({status:'completed'})],{started:true}).label,'Started')
 assert.equal(orchestrationProfileState([session({status:'completed'})]).label,'Ready')
 assert.equal(orchestrationProfileState([session({status:'failed',authenticationRequired:true})],{started:true}).label,'Sign-in needed')
 assert.equal(orchestrationProfileState([],{error:'Unavailable'}).label,'Needs attention')
 // Ticket workers never change the profile's state.
 const workers=[{status:'failed',authenticationRequired:true},{status:'running',waitingSince:'2026-10-22T09:00:00Z'}]
 assert.equal(orchestrationProfileState(workers).label,'Ready')
})

test('a decision names the ticket and why, and flags a deferral the ticket outgrew',()=>{
 assert.deepEqual(orchestrationDecisionLine({taskKey:'A-1',title:'Validate requests',decision:'launched',reason:'Unblocks the API.',unchangedSinceDecision:true}),{ticket:'A-1 · Validate requests',what:'Launched',reason:'Unblocks the API.',changed:false})
 assert.equal(orchestrationDecisionLine({taskKey:'A-2',decision:'deferred',reason:'No acceptance criteria.',unchangedSinceDecision:true}).changed,false)
 assert.equal(orchestrationDecisionLine({taskKey:'A-2',decision:'deferred',reason:'No acceptance criteria.',unchangedSinceDecision:false}).changed,true)
 // A launch is never "changed since": the ticket moving is the chain at work.
 assert.equal(orchestrationDecisionLine({taskKey:'A-1',decision:'launched',unchangedSinceDecision:false}).changed,false)
 assert.equal(orchestrationDecisionLine({}).ticket,'Task')
})
