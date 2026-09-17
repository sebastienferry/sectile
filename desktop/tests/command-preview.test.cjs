const {test}=require('node:test')
const assert=require('node:assert/strict')

async function load(){return import('../src/command-preview.mjs')}

test('claude with no template gives the two attested command lines',async()=>{
 const {commandPreview}=await load()
 assert.equal(commandPreview('claude','','claude-opus-5',false).command,"claude --model claude-opus-5 '{prompt}'")
 assert.equal(commandPreview('claude','','claude-opus-5',true).command,"claude -p --permission-mode bypassPermissions --model claude-opus-5 '{prompt}'")
})

test('an unset model drops the flag rather than passing an empty one',async()=>{
 const {commandPreview,modelArgs}=await load()
 assert.deepEqual(modelArgs('claude',''),[])
 assert.deepEqual(modelArgs('agy','claude-opus-5'),[])
 assert.equal(commandPreview('claude','','',false).command,"claude '{prompt}'")
 assert.equal(commandPreview('claude','','',true).command,"claude -p --permission-mode bypassPermissions '{prompt}'")
})

test('a template without the mode marker cannot serve an autonomous launch',async()=>{
 const {commandPreview,templateCarriesMode}=await load()
 const template="claude -p '{prompt}'"
 assert.equal(templateCarriesMode(template),false)
 assert.equal(commandPreview('claude',template,'',false).command,template)
 const autonomous=commandPreview('claude',template,'',true)
 assert.equal(autonomous.command,'')
 assert.match(autonomous.error,/\{mode:AUTONOMOUS\|INTERACTIVE\}/)
})

test('the mode marker keeps its left side for a headless launch',async()=>{
 const {commandPreview}=await load()
 const template="claude {mode:-p --permission-mode bypassPermissions|} --model {model} '{prompt}'"
 assert.equal(commandPreview('claude',template,'claude-opus-5',false).command,"claude --model 'claude-opus-5' '{prompt}'")
 assert.equal(commandPreview('claude',template,'claude-opus-5',true).command,"claude -p --permission-mode bypassPermissions --model 'claude-opus-5' '{prompt}'")
})

// A flag left with nothing behind it consumes the next word, so an unset model
// takes its option with it rather than becoming an empty argument.
test('an unset model removes the slot and the option it belongs to',async()=>{
 const {commandPreview,dropModelSlot}=await load()
 assert.equal(commandPreview('claude',"claude --model {model} '{prompt}'",'',false).command,"claude '{prompt}'")
 assert.equal(dropModelSlot("claude --model '{model}' '{prompt}'"),"claude '{prompt}'")
 assert.equal(dropModelSlot("mycli {model} '{prompt}'"),"mycli '{prompt}'")
})

test('both modes are rendered as labelled lines',async()=>{
 const {previewLines}=await load()
 const lines=previewLines('claude','','claude-opus-5')
 assert.deepEqual(lines.map(line=>line.label),['Interactive','Autonomous'])
 assert.ok(lines.every(line=>line.ok))
 const refused=previewLines('agy','','')
 assert.equal(refused[0].ok,true)
 assert.equal(refused[1].ok,false)
 assert.match(refused[1].text,/no attested headless mode/)
})

// A command written for headless use is what an autonomous launch runs, and it
// needs no marker of its own; the interactive one is left untouched.
test('a dedicated autonomous command serves headless launches on its own',async()=>{
 const {commandPreview,previewLines}=await load()
 const interactive="claude '{prompt}'"
 const autonomous="claude -p --permission-mode bypassPermissions '{prompt}'"
 assert.equal(commandPreview('claude',interactive,'',false,autonomous).command,interactive)
 assert.equal(commandPreview('claude',interactive,'',true,autonomous).command,autonomous)
 assert.equal(commandPreview('claude',interactive,'',true).command,'')
 assert.deepEqual(previewLines('claude',interactive,'',autonomous).map(l=>l.text),[interactive,autonomous])
})
