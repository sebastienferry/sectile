const {test}=require('node:test')
const assert=require('node:assert/strict')

const load=()=>import('../src/agent-poll.mjs')

test('a connected app polls its runs',async()=>{
 const {pollAction}=await load()
 assert.equal(pollAction({agentConnected:true,shutdownVisible:true}),'refresh')
})

// The regression: the poll used to read the setup panel, which agentUnavailable
// keeps hidden while the agent-log pane is open. The app then polled runs()
// forever against a connection the main process had already dropped.
test('a disconnected app reconnects whatever the window shows',async()=>{
 const {pollAction}=await load()
 assert.equal(pollAction({agentConnected:false,shutdownVisible:false}),'connect')
})

test('a lifecycle operation is left to finish',async()=>{
 const {pollAction}=await load()
 assert.equal(pollAction({restarting:true,agentConnected:true,shutdownVisible:true}),'idle')
 assert.equal(pollAction({restarting:true,agentConnected:false,shutdownVisible:false}),'idle')
 assert.equal(pollAction({agentConnected:false,shutdownVisible:true}),'idle')
})

test('an undeclared state reconnects rather than polling a dead connection',async()=>{
 const {pollAction}=await load()
 assert.equal(pollAction({}),'connect')
})
