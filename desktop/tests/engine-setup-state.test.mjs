import {test} from 'node:test'
import assert from 'node:assert/strict'
import {setupReducer,supportedEngines,startupSetupPrompt,deploymentSucceeded,setupIncomplete} from '../src/engine-setup-state.mjs'
const view=(provider='codex',status='pending')=>({schemaVersion:1,provider,server:'https://sectile.test',steps:Object.fromEntries(['skills','mcp','permissions','sdd'].map(step=>[step,{status}]))})
test('selection generations reject late reads and preserve independent outcomes',()=>{
 let state={provider:'codex',generation:1,view:view(),busy:true}
 const late={type:'read',provider:'codex',generation:1,view:view('codex','success')}
 state=setupReducer(state,{type:'select',provider:'claude'})
 assert.equal(setupReducer(state,late),state)
 const partial=view('claude');partial.steps.skills.status='success';partial.steps.sdd.status='skipped'
 state=setupReducer(state,{type:'read',provider:'claude',generation:2,view:partial})
 assert.equal(state.view.steps.skills.status,'success');assert.equal(state.view.steps.sdd.status,'skipped')
 assert.equal(setupReducer(state,{type:'end',provider:'codex',generation:1}),state)
})
test('structured HTTP success cannot hide deployment failure',()=>{
 assert.equal(deploymentSucceeded({success:true,skills:{status:'failed'}},'skills'),false)
 assert.equal(deploymentSucceeded({success:true,skills:{status:'success'}},'skills'),true)
 assert.equal(deploymentSucceeded({},'sdd'),false)
 assert.equal(deploymentSucceeded({installed:true,error:'failed'},'sdd'),false)
 assert.equal(deploymentSucceeded({alreadyInit:true},'sdd'),true)
})
test('unsupported catalogue and unavailable reads do not imply incomplete setup',()=>{
 assert.deepEqual(supportedEngines({catalogue:[{provider:'custom'}]}),[])
 assert.equal(setupIncomplete({}),false)
 assert.equal(setupIncomplete(view('codex','unknown')),false)
 assert.equal(setupIncomplete(view('codex','success')),false)
 assert.equal(setupIncomplete({...view(),server:''}),false)
})
test('startup retries unknown reads, deduplicates shared providers and prompts once per launch',async()=>{
 let unavailable=true,calls=[],prompts=[]
 const api={engines:async()=>({catalogue:[{provider:'codex'},{provider:'codex'},{provider:'custom'},{provider:'claude'}]}),setupState:async provider=>{calls.push(provider);return view(provider,unavailable?'unknown':provider==='claude'?'success':'pending')}}
 const check=startupSetupPrompt(),notify=providers=>prompts.push(providers)
 await check(api,notify);assert.deepEqual(prompts,[]);assert.deepEqual(calls,['codex','claude'])
 unavailable=false;await check(api,notify);await check(api,notify)
 assert.deepEqual(prompts,[['codex']]);assert.equal(calls.length,4)
 await startupSetupPrompt()(api,notify);assert.equal(prompts.length,2)
})
test('malformed old capability and rejected providers leave startup silent',async()=>{
 const prompts=[]
 await startupSetupPrompt()({engines:async()=>({catalogue:[{provider:'codex'}]}),setupState:async()=>({})},p=>prompts.push(p))
 await startupSetupPrompt()({engines:async()=>({catalogue:[{provider:'codex'}]}),setupState:async()=>{throw Error('offline')}},p=>prompts.push(p))
 assert.deepEqual(prompts,[])
})

test('read failures discard stale completion until configuration can be observed again',()=>{
 const state={provider:'codex',generation:1,view:view('codex','success'),busy:false}
 const next=setupReducer(state,{type:'read',provider:'codex',generation:1,view:null})
 assert.equal(next.view,null)
 assert.equal(setupIncomplete(next.view),false)
})
