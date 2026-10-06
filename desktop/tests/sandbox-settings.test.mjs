import {test} from 'node:test'
import assert from 'node:assert/strict'
import {addEntry,bothLists,fromStored,launchesGetSettings,resolvedValues,sandboxPayload,sandboxState,whitelistSummary} from '../src/sandbox-settings.mjs'

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

// A covered project applies the workstation entries then its own, and its
// state over the workstation's unless it inherits (#730).
test('a project lays its values over the workstation ones',()=>{
 const workstation=fromStored({enabled:true,allowedDomains:['registry.npmjs.org'],allow:['Read'],deny:['Bash(git push:*)']})
 const own=fromStored({allow:['Grep','Read'],allowWrite:['~/.cache']})
 assert.deepEqual(resolvedValues(own,workstation),{state:'On',allowedDomains:['registry.npmjs.org'],allowWrite:['~/.cache'],allow:['Read','Grep'],deny:['Bash(git push:*)']})
 assert.equal(resolvedValues(fromStored({enabled:false}),workstation).state,'Off')
 assert.deepEqual(resolvedValues(own,null),own)
})

test('the preview of a project with only workstation values carries the settings',()=>{
 const resolved=resolvedValues(fromStored(null),fromStored({deny:['Bash(rm:*)']}))
 assert.equal(launchesGetSettings(resolved,true),true)
 assert.equal(launchesGetSettings(fromStored(null),true),false)
})

test('a rule allowed by the workstation and denied by the project is named',()=>{
 const resolved=resolvedValues(fromStored({deny:['Read']}),fromStored({allow:['Read']}))
 assert.deepEqual(bothLists(resolved.allow,resolved.deny),['Read'])
})

test('the whitelist says when it covers every project',()=>{
 assert.match(whitelistSummary([]),/every project/i)
 assert.match(whitelistSummary(['p']),/only to the checked projects/)
 assert.match(whitelistSummary([],true),/still apply to every project/)
})
