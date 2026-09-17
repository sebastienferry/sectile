const {test}=require('node:test')
const assert=require('node:assert/strict')

async function load(){
 const mode=await import('../src/skill-mode.mjs')
 const console_=await import('../src/run-console.mjs')
 return {...mode,...console_}
}

test('the configured mode is the default and sends no override',async()=>{
 const {SKILL_MODE_OPTIONS,launchModeOverride}=await load()
 assert.equal(SKILL_MODE_OPTIONS[0].value,'')
 assert.equal(launchModeOverride(''),'')
 assert.equal(launchModeOverride(undefined),'')
 assert.equal(launchModeOverride(null),'')
})

test('an explicit choice travels with the launch',async()=>{
 const {launchModeOverride}=await load()
 assert.equal(launchModeOverride('interactive'),'interactive')
 assert.equal(launchModeOverride('autonomous'),'autonomous')
 assert.equal(launchModeOverride(' Autonomous '),'autonomous')
})

test('an unrecognized value is dropped rather than sent',async()=>{
 const {launchModeOverride}=await load()
 for(const value of ['headless','auto','non_interactive','1'])assert.equal(launchModeOverride(value),'')
})

test('the dialogs offer exactly the three states',async()=>{
 const {SKILL_MODE_OPTIONS}=await load()
 assert.deepEqual(SKILL_MODE_OPTIONS.map(option=>option.value),['','interactive','autonomous'])
})

test('modeSelect builds a control defaulting to the configured mode',async()=>{
 const {modeSelect}=await load()
 const document={createElement:tag=>({tag,children:[],attributes:{},value:'',textContent:'',append(...nodes){this.children.push(...nodes)},setAttribute(name,value){this.attributes[name]=value}})}
 const select=modeSelect(document,'Execution mode')
 assert.equal(select.attributes['aria-label'],'Execution mode')
 assert.deepEqual(select.children.map(option=>option.value),['','interactive','autonomous'])
 assert.equal(select.value,'')
})

// An autonomous run has no terminal on purpose. Presenting it as a run whose
// console is missing would send the user hunting for a launch error.
test('an autonomous run is explained, not reported as consoleless',async()=>{
 const {consoleNotice,needsConsoleNotice}=await load()
 const run={status:'running',headless:true,sessionId:''}
 assert.equal(needsConsoleNotice(run),true)
 assert.match(consoleNotice(run),/Autonomous execution/)
 assert.doesNotMatch(consoleNotice(run),/No console is available/)
})

test('the other console notices are unchanged',async()=>{
 const {consoleNotice,needsConsoleNotice}=await load()
 assert.match(consoleNotice({status:'queued'}),/Execution queued/)
 assert.match(consoleNotice({status:'preparing'}),/Preparing execution/)
 assert.match(consoleNotice({status:'canceled'}),/canceled before a console/)
 assert.match(consoleNotice({status:'failed'}),/No console is available/)
 assert.equal(needsConsoleNotice({status:'running',sessionId:'s1'}),false)
})

test('the project default offers three options and delegates on the empty one',async()=>{
 const {PROJECT_MODE_OPTIONS}=await load()
 assert.deepEqual(PROJECT_MODE_OPTIONS.map(o=>o.value),['interactive','autonomous',''])
 // The delegating option comes last: it is the fallback, not a mode.
 assert.equal(PROJECT_MODE_OPTIONS.at(-1).label,'Per-skill choice')
})
