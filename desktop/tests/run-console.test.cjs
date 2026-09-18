const {test}=require('node:test')
const assert=require('node:assert/strict')

// What the console pane does with an autonomous run.
//
// Such a run has no terminal on purpose, and used to be answered with a sentence
// saying so. When its engine reports what it is doing, the pane attaches to that
// trace instead — read-only, because nobody is answering the run.

async function load(){
 return import('../src/run-console.mjs')
}

test('an autonomous run reporting what it does is attached to, not explained',async()=>{
 const {needsConsoleNotice,readOnlyConsole}=await load()
 const run={status:'running',headless:true,trace:true,sessionId:''}
 assert.equal(needsConsoleNotice(run),false)
 assert.equal(readOnlyConsole(run),true)
})

// The agent and the desktop are versioned apart, and an agent that cannot trace
// says nothing about one. Its runs keep the notice they always had.
test('an autonomous run with no trace keeps its notice',async()=>{
 const {needsConsoleNotice,consoleNotice,readOnlyConsole}=await load()
 for(const run of [
  {status:'running',headless:true,sessionId:''},
  {status:'running',headless:true,trace:false,sessionId:''},
 ]){
  assert.equal(needsConsoleNotice(run),true)
  assert.equal(readOnlyConsole(run),false)
  assert.match(consoleNotice(run),/Autonomous execution/)
 }
})

// A trace belongs to a run that started: before that there is nothing to watch,
// and the queue says so.
test('a queued or preparing run is still announced as waiting',async()=>{
 const {needsConsoleNotice,consoleNotice}=await load()
 for(const status of ['queued','preparing']){
  const run={status,headless:true,trace:true}
  assert.equal(needsConsoleNotice(run),true)
  assert.match(consoleNotice(run),status==='queued'?/Execution queued/:/Preparing execution/)
 }
})

test('a console run is unaffected',async()=>{
 const {needsConsoleNotice,readOnlyConsole}=await load()
 assert.equal(needsConsoleNotice({status:'running',sessionId:'s1'}),false)
 assert.equal(readOnlyConsole({status:'running',sessionId:'s1'}),false)
 assert.equal(needsConsoleNotice({status:'running',sessionId:''}),true)
})

// A finished run is still worth reading: the agent replays what it showed for as
// long as it remembers the run.
test('a finished autonomous run still shows what it did',async()=>{
 const {needsConsoleNotice}=await load()
 assert.equal(needsConsoleNotice({status:'completed',headless:true,trace:true}),false)
 assert.equal(needsConsoleNotice({status:'failed',headless:true,trace:true}),false)
})
