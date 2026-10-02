import {test} from 'node:test'
import assert from 'node:assert/strict'
import {addEntry,bothLists,fromStored,launchesGetSettings,sandboxPayload,sandboxState} from '../src/sandbox-settings.mjs'

// The lists of the Sandbox category (#700) normalise their entries as the
// agent does: trimmed, never twice, never empty.
test('an entry is trimmed, added once, and refused when empty',()=>{
 assert.deepEqual(addEntry([],'  registry.npmjs.org ').list,['registry.npmjs.org'])
 const twice=addEntry(['Read'],'Read ')
 assert.deepEqual(twice.list,['Read'])
 assert.match(twice.error,/already in the list/)
 for(const empty of ['','   ',null]){
  const refused=addEntry(['Read'],empty)
  assert.deepEqual(refused.list,['Read'])
  assert.ok(refused.error)
 }
})

test('a rule in both lists is named',()=>{
 assert.deepEqual(bothLists(['Read','Bash(git push:*)'],['Bash(git push:*)']),['Bash(git push:*)'])
 assert.deepEqual(bothLists(['Read'],[]),[])
})

test('the state reads inherited, on or off, and saves as null, true or false',()=>{
 assert.equal(sandboxState(undefined),'Inherited')
 assert.equal(sandboxState(true),'On')
 assert.equal(sandboxState(false),'Off')
 const values=fromStored({enabled:false,allowedDomains:['a.example']})
 assert.deepEqual(sandboxPayload(values),{enabled:false,allowedDomains:['a.example'],allowWrite:[],allow:[],deny:[]})
 assert.equal(sandboxPayload(fromStored(null)).enabled,null)
})

// The preview shows the settings argument only when the agent would write a
// file: on Windows the sandbox values alone write none.
test('a launch gets settings only when its values state something',()=>{
 assert.equal(launchesGetSettings(fromStored({}),true),false)
 assert.equal(launchesGetSettings(fromStored({enabled:true}),true),true)
 assert.equal(launchesGetSettings(fromStored({enabled:true,allowedDomains:['a']}),false),false)
 assert.equal(launchesGetSettings(fromStored({deny:['Read']}),false),true)
})
