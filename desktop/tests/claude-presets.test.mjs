import {test} from 'node:test'
import assert from 'node:assert/strict'
import {PRESETS,RECOMMENDED,applicableLists,applyPreset,listsEmpty,presetApplied,presetHasEntries,removePreset} from '../src/claude-presets.mjs'
import {addEntry,fromStored} from '../src/sandbox-settings.mjs'

const LISTS=['allowedDomains','allowWrite','allow','deny']
const preset=id=>PRESETS.find(p=>p.id===id)
const empty=()=>fromStored(null)

// The catalogue of #745 holds entries the panel itself would accept, each
// once, written in the rule form Claude Code documents.
test('every preset entry is one the lists accept, once, in the documented rule form',()=>{
 assert.equal(new Set(PRESETS.map(p=>p.id)).size,PRESETS.length)
 for(const id of RECOMMENDED)assert.ok(preset(id),id)
 for(const p of PRESETS){
  assert.ok(p.name&&p.description,p.id)
  for(const list of LISTS){
   assert.ok(Array.isArray(p[list]),p.id+' '+list)
   let accepted=[]
   for(const entry of p[list]){
    const added=addEntry(accepted,entry)
    assert.equal(added.error,undefined,p.id+': '+entry)
    assert.equal(added.list.at(-1),entry,p.id+': '+entry+' is not trimmed')
    accepted=added.list
   }
  }
  for(const rule of [...p.allow,...p.deny]){
   assert.match(rule,/^(Bash|Read)\(.+\)$/,rule)
   // `:*` is a trailing wildcard only; anywhere else Claude Code reads a colon.
   assert.ok(!/:\*./.test(rule),rule)
  }
  // Claude Code warns about an allow rule with a * before its subcommand:
  // an allow rule has no * but a trailing one.
  for(const rule of p.allow)assert.match(rule,/^Bash\([^*]+( \*)?\)$/,rule)
 }
})

test('the deny preset leaves out routine cleanups and the catalogue no broad host',()=>{
 const deny=preset('dangerous').deny
 for(const rule of ['Bash(terraform apply *)','Bash(sudo *)','Bash(git push --force*)','Read(~/.ssh/**)'])assert.ok(deny.includes(rule),rule)
 assert.ok(!deny.some(rule=>/rm -rf|reset --hard/.test(rule)))
 assert.ok(!PRESETS.some(p=>p.allowedDomains.includes('storage.googleapis.com')))
 // The preset's guardrail warning (US6).
 assert.match(preset('dangerous').description,/not a security boundary/)
})

test('applying a preset appends its missing entries in order and keeps the rest',()=>{
 const before={...empty(),allow:['Read','Bash(go test *)'],allowedDomains:['example.com']}
 const after=applyPreset(before,preset('go'),true)
 assert.deepEqual(after.allow,['Read','Bash(go test *)','Bash(go build *)','Bash(go vet *)','Bash(go mod *)','Bash(gofmt *)','Bash(golangci-lint *)'])
 assert.deepEqual(after.allowedDomains,['example.com','proxy.golang.org','sum.golang.org'])
 assert.deepEqual(after.allowWrite,preset('go').allowWrite)
 assert.deepEqual(before.allow,['Read','Bash(go test *)'],'the input is not mutated')
 assert.deepEqual(applyPreset(after,preset('go'),true),after,'a second apply adds nothing')
 assert.equal(after.state,before.state)
})

test('on Windows a preset applies its rules only',()=>{
 assert.deepEqual(applicableLists(false),['allow','deny'])
 const after=applyPreset(empty(),preset('go'),false)
 assert.deepEqual(after.allow,preset('go').allow)
 assert.deepEqual(after.allowedDomains,[])
 assert.deepEqual(after.allowWrite,[])
 assert.equal(presetApplied(after,preset('go'),false),true)
 assert.equal(presetApplied(after,preset('go'),true),false)
 const rulesOnly={...preset('go'),id:'paths',allow:[],deny:[]}
 assert.equal(presetHasEntries(rulesOnly,false),false)
 assert.equal(presetApplied(applyPreset(empty(),rulesOnly,false),rulesOnly,false),false)
})

test('a preset reads applied until one of its entries is removed',()=>{
 const applied=applyPreset(empty(),preset('rust'),true)
 assert.equal(presetApplied(applied,preset('rust'),true),true)
 assert.equal(presetApplied({...applied,allowWrite:[]},preset('rust'),true),false)
 assert.equal(presetApplied(empty(),preset('rust'),true),false)
})

test('removing a preset keeps what another applied preset holds',()=>{
 const shared={id:'shared',name:'Shared',description:'d',allow:['Bash(go test *)','Bash(make test)'],deny:[],allowedDomains:['proxy.golang.org'],allowWrite:[]}
 const catalogue=[...PRESETS,shared]
 let values=applyPreset(applyPreset(empty(),preset('go'),true),shared,true)
 values=removePreset(values,preset('go'),true,catalogue)
 assert.deepEqual(values.allow,['Bash(go test *)','Bash(make test)'])
 assert.deepEqual(values.allowedDomains,['proxy.golang.org'])
 assert.deepEqual(values.allowWrite,[])
 // A preset that is only partly present (no Bash(make test)) is not applied,
 // so it does not protect the entry it shares.
 values=removePreset(applyPreset(empty(),preset('go'),true),preset('go'),true,catalogue)
 assert.deepEqual(values.allow,[])
 assert.deepEqual(values.allowedDomains,[])
})

test('removing a preset takes a hand-typed duplicate with it',()=>{
 const typed={...empty(),deny:['Bash(sudo *)','Bash(own rule)']}
 const applied=applyPreset(typed,preset('dangerous'),true)
 const removed=removePreset(applied,preset('dangerous'),true)
 assert.deepEqual(removed.deny,['Bash(own rule)'])
})

test('the lists are empty whatever the state',()=>{
 assert.equal(listsEmpty(empty()),true)
 assert.equal(listsEmpty({...empty(),state:'On'}),true)
 assert.equal(listsEmpty({...empty(),allowWrite:['~/.npm']}),false)
})
