const {test}=require('node:test')
const assert=require('node:assert/strict')

async function load(){return import('../src/run-engine.mjs')}

test('a run names the engine and the model it ran against',async()=>{
 const {runEngine}=await load()
 assert.equal(runEngine({provider:'claude',model:'claude-opus-5'}),'claude · claude-opus-5')
})

test('a run whose provider takes no model names the provider alone',async()=>{
 const {runEngine}=await load()
 assert.equal(runEngine({provider:'agy',model:''}),'agy')
})

test('a run recorded before the engine was tracked names nothing',async()=>{
 const {runEngine}=await load()
 assert.equal(runEngine({}),'')
 assert.equal(runEngine(undefined),'')
 assert.equal(runEngine({provider:'  ',model:'  '}),'')
})
