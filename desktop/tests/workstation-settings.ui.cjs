const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// A fake agent answering the workstation routes. The engine catalogue (#510)
// is kept as the agent keeps it: a PUT replaces it and answers the new view.
function fakeAgent({capabilities=['task-engines'],customSkillsUsed=[],defaults={},installed={claude:true,codex:true,agy:false}}={}){
 const state={puts:[],enginePuts:[],refuse:'',engines:{
  catalogue:[
   {id:'e-opus',name:'Claude Opus',provider:'claude',model:'claude-opus-5',skillModels:{implement:'claude-sonnet-5'}},
   {id:'e-codex',name:'Codex',provider:'codex'},
  ],
  default:'e-opus',providers:['claude','codex','agy','custom'],providerModels:{},projects:{p:'e-codex'},taskCounts:{'e-codex':2},
  installed,
 }}
 const view=()=>({
  globalConfiguration:true,
  skillCommands:[{id:"clarify",name:"Clarify",command:"/clarify-issue"},{id:"implement",name:"Implement",command:"/implement-issue"}],
  defaults:{editorCommand:'zed',setupProviders:null,...defaults,...state.skillSettings},
  effective:{defaultEngine:{id:'e-opus',name:'Claude Opus',provider:'claude',model:'claude-opus-5'},editorCommand:'zed',useWorktrees:true,parallelism:1,aiProviderModels:{}},
  providerModels:{claude:['claude-opus-5','claude-sonnet-5'],codex:['gpt-5']},setupProviders:['claude','codex','agy'],seeded:{},
  customSkillsUsed,
 })
 const body=req=>new Promise(resolve=>{let data='';req.on('data',chunk=>data+=chunk);req.on('end',()=>resolve(JSON.parse(data)))})
 const server=http.createServer(async(req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/workstation'&&req.method==='PUT'){
   const input=await body(req)
   if(state.refuse){res.writeHead(400,{'Content-Type':'text/plain'}).end(state.refuse);return}
   // The skill settings read back as saved, so a reset is seen to reset.
   state.puts.push(input);state.skillSettings={customSkillsWin:input.customSkillsWin,installedSkillSource:input.installedSkillSource,aiProviderModels:input.aiProviderModels,skillCommands:input.skillCommands}
   res.writeHead(204).end();return
  }
  // A held read answers only once the test releases it, so the panel can be
  // edited while the refill that follows a save is still on its way.
  if(req.url==='/desktop/workstation'){await state.heldRead;res.end(JSON.stringify(view()));return}
  if(req.url==='/desktop/engines'&&req.method==='PUT'){
   const input=await body(req)
   state.enginePuts.push(input)
   if(state.refuse){res.writeHead(400,{'Content-Type':'text/plain'}).end(state.refuse);return}
   const catalogue=input.catalogue.map((engine,i)=>engine.id?engine:{...engine,id:'e-new-'+i})
   const ids=catalogue.map(engine=>engine.id)
   state.engines={...state.engines,catalogue,default:input.default,projects:Object.fromEntries(Object.entries(state.engines.projects).filter(([,id])=>ids.includes(id)))}
   res.end(JSON.stringify(state.engines));return
  }
  if(req.url==='/desktop/engines'){res.end(JSON.stringify(state.engines));return}
  if(req.url.startsWith('/desktop/mcp?')){res.end(JSON.stringify({path:'/test',server:'https://sectile.example.test',localURL:'http://127.0.0.1:4567',choice:{target:'remote',transport:'http'}}));return}
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://sectile.example.test',capabilities}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'p',name:'Sectile',path:'/p',configured:true}]));return}
  if(req.url==='/desktop/runs'){res.end('[]');return}
  if(req.url.startsWith('/desktop/project?')){res.end(JSON.stringify({configured:true,parallelism:1}));return}
  if(req.url==='/desktop/version'){res.end('{"version":"test"}');return}
  res.writeHead(404).end('{}')
 })
 return {state,server}
}

async function openExecutionDefaults(server,root,category='Execution defaults'){
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'private'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 const app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
 const page=await app.firstWindow();page.setDefaultTimeout(10000)
 await page.locator('#settings').click()
 await page.getByRole('tab',{name:category,exact:true}).click()
 return {app,page,panel:page.locator(category==='AI engines'?'#settings-panel-Engines':category==='Skills'?'#settings-panel-Skills':'.execution-defaults')}
}

// The workstation level of every execution setting is edited from Settings,
// through the local agent, which is its only writer (#305). The engine lives in
// the engine catalogue since #510: the defaults carry none.
test('execution defaults are read from and saved through the agent, which may refuse them',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-workstation-ui-'))
 const {state,server}=fakeAgent()
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 let app
 try {
  let panel,page
  ;({app,panel,page}=await openExecutionDefaults(server,root))
  await expect(panel.locator('.engines-section')).toHaveCount(0)
  // The editor is a picker (#535): a stored preset loads as that preset.
  await expect(panel.getByRole('combobox',{name:'Editor',exact:true})).toHaveValue('zed')
  await expect(panel.getByRole('textbox',{name:'Custom editor command',exact:true})).toBeHidden()
  await expect(panel.getByRole('combobox',{name:'Initialization provider',exact:true})).toHaveCount(0)
  await expect(panel.getByText('Extra setup providers',{exact:true})).toHaveCount(0)
  for(const retired of ['gemini','cursor','vibe'])await expect(panel.getByRole('textbox',{name:'Models offered for '+retired,exact:true})).toHaveCount(0)
  await expect(panel.getByRole('textbox',{name:'Models offered for codex',exact:true})).toHaveCount(0)
  await panel.getByRole('button',{name:'Save execution defaults'}).click()
  await expect(panel.locator('.workstation-notice')).toContainText('Execution defaults saved')
  assert.equal(state.puts.length,1)
  assert.equal(state.puts[0].initializationProvider,undefined)
  assert.equal(state.puts[0].skillCommands,undefined)
  assert.equal(state.puts[0].aiProvider,undefined)
  assert.equal(state.puts[0].aiModel,undefined)
  assert.equal(state.puts[0].aiProviderModels,undefined)
  assert.equal(state.puts[0].setupProviders,null)
  assert.equal(state.puts[0].editorCommand,'zed')

  // A value the agent refuses is reported with its reason.
  state.refuse='parallelism must be between 1 and 10'
  await panel.getByRole('button',{name:'Save execution defaults'}).click()
  await expect(panel.locator('.workstation-notice')).toContainText('Not saved: parallelism must be between 1 and 10')
  assert.equal(state.puts.length,1)

  // A command that matches no preset is a custom one, saved as typed.
  state.refuse=''
  const editor=panel.getByRole('combobox',{name:'Editor',exact:true})
  await editor.selectOption({label:'Custom command…'})
  const custom=panel.getByRole('textbox',{name:'Custom editor command',exact:true})
  await expect(custom).toBeVisible()
  await custom.fill('  cursor -n  ')
  await panel.getByRole('button',{name:'Save execution defaults'}).click()
  await expect(panel.locator('.workstation-notice')).toContainText('Execution defaults saved')
  assert.equal(state.puts[1].editorCommand,'cursor -n')
  // The reset control returns the row to None. The save refills the panel,
  // which moves the rows: the control is activated from the keyboard.
  await panel.getByRole('button',{name:'Reset editor to default',exact:true}).focus()
  await page.keyboard.press('Enter')
  await expect(editor).toHaveValue('')
  await expect(custom).toBeHidden()
 } finally {
  await app?.close()
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})

// An unset parallelism means five executions (#594); an explicit 1 stays a
// choice the panel reads back as the workstation's.
test('parallel executions default to five and keep an explicit value',async()=>{
 for(const [defaults,readout,hint] of [[{},'5 executions','Default · 5 executions'],[{parallelism:1},'1 execution','Workstation default']]){
  const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-parallelism-ui-'))
  const {server}=fakeAgent({defaults})
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
  let app
  try {
   let panel,page
   ;({app,panel,page}=await openExecutionDefaults(server,root))
   const slider=panel.getByRole('slider',{name:'Parallel executions',exact:true})
   // A filter's inner locator is relative to the row, so it starts from the page.
   const row=panel.locator('.setting-row').filter({has:page.getByRole('slider',{name:'Parallel executions',exact:true})})
   await expect(slider).toHaveValue(String(parseInt(readout)))
   await expect(row.locator('.slider-value')).toHaveText(readout)
   await expect(row.locator('.setting-text p')).toHaveText(hint)
   // The reset returns the row to the default, not to one execution.
   await slider.fill('2')
   await expect(row.locator('.setting-text p')).toHaveText('Workstation default')
   await panel.getByRole('button',{name:'Reset parallel executions to default',exact:true}).click()
   await expect(slider).toHaveValue('5')
   await expect(row.locator('.setting-text p')).toHaveText('Default · 5 executions')
  } finally {
   await app?.close()
   server.close()
   fs.rmSync(root,{recursive:true,force:true})
  }
 }
})

// A project's custom skill that ran is a passive signal (#267): a badge on the
// settings button and a notice beside the setting that allows it.
test('custom skills used show as a badge and a notice, and not when none ran',async()=>{
 for(const used of [[{projectId:'p',projectName:'Sectile',skillId:'implement',directory:'code-issue',lastRun:'2026-09-28T08:00:00Z'}],[]]){
  const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-custom-skills-ui-'))
  const {server}=fakeAgent({customSkillsUsed:used})
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
  let app
  try {
   let panel,page
   ;({app,panel,page}=await openExecutionDefaults(server,root,'Skills'))
   const notice=panel.getByRole('note')
   if(used.length){
    await expect(page.locator('#settings')).toHaveClass(/custom-skills-used/)
    await expect(page.locator('#settings')).toHaveAttribute('aria-label','Settings, custom skills used')
    await expect(notice).toBeVisible()
    await expect(notice).toContainText('Custom skills used')
    await expect(notice).toContainText('Sectile · code-issue')
   }else{
    await expect(page.locator('#settings')).not.toHaveClass(/custom-skills-used/)
    await expect(page.locator('#settings')).toHaveAttribute('aria-label','Settings')
    await expect(notice).toBeHidden()
   }
  } finally {
   await app?.close()
   server.close()
   fs.rmSync(root,{recursive:true,force:true})
  }
 }
})

// The custom project skills win setting is saved through the agent; a reset sends undefined,
// which the agent reads as its defaults (#267).
test('the skill settings save through the agent and reset to their defaults',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-skill-settings-ui-'))
 const {state,server}=fakeAgent()
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 let app
 try {
  let panel
  ;({app,panel}=await openExecutionDefaults(server,root,'Skills'))
  const wins=panel.getByRole('group',{name:'Custom project skills win',exact:true})
  await expect(wins.getByRole('button',{name:'Yes',exact:true})).toHaveAttribute('aria-pressed','true')
  await wins.getByRole('button',{name:'No',exact:true}).click()
  await panel.getByRole('button',{name:'Save skills'}).click()
  await expect(panel.locator('.workstation-notice')).toContainText('Skills saved')
  assert.equal(state.puts.length,1)
  assert.equal(state.puts[0].customSkillsWin,false)
  // The panel refills from what the agent now serves.
  await expect(wins.getByRole('button',{name:'No',exact:true})).toHaveAttribute('aria-pressed','true')

  // An overlay scrollbar (macOS "Automatic") takes no layout space and is drawn
  // over the scrolling panel's right edge, so the reset buttons must keep a
  // gutter of their own. Measured against the content edge, the check holds
  // whatever the platform's scrollbar style.
  const reset=panel.getByRole('button',{name:'Reset custom project skills win to default',exact:true})
  const {gap,hit}=await reset.evaluate(button=>{
   const scroller=button.closest('section[id^="settings-panel-"]')
   scroller.scrollTop=scroller.scrollHeight
   button.scrollIntoView({block:'nearest'})
   const panel=scroller.getBoundingClientRect(),box=button.getBoundingClientRect()
   const contentRight=panel.left+scroller.clientLeft+scroller.clientWidth
   const under=document.elementFromPoint(box.right-1,box.top+box.height/2)
   return {gap:contentRight-box.right,hit:button.contains(under)}
  })
  assert.ok(gap>=15,`"Reset custom project skills win to default" ends ${gap}px from the panel's content edge, under an overlay scrollbar`)
  assert.ok(hit,`"Reset custom project skills win to default" is not the element under its rightmost pixel`)

  await reset.click()
  await panel.getByRole('button',{name:'Save skills'}).click()
  await expect(panel.locator('.workstation-notice')).toContainText('Skills saved')
  assert.equal(state.puts.length,2)
  assert.equal(state.puts[1].customSkillsWin,undefined)
 } finally {
  await app?.close()
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})

// The refill that follows a save neither announces "saved" before it lands nor
// undoes a setting edited while it was on its way.
test('a setting edited while the saved settings reload is kept',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-skill-settings-refill-ui-'))
 const {state,server}=fakeAgent()
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 let app
 try {
  let panel
  ;({app,panel}=await openExecutionDefaults(server,root,'Skills'))
  const wins=panel.getByRole('group',{name:'Custom project skills win',exact:true})
  await wins.getByRole('button',{name:'No',exact:true}).click()
  let release
  state.heldRead=new Promise(resolve=>{release=resolve})
  await panel.getByRole('button',{name:'Save skills'}).click()
  await expect.poll(()=>state.puts.length).toBe(1)
  await expect(panel.locator('.workstation-notice')).toHaveText('Saving…')
  await panel.getByRole('button',{name:'Reset custom project skills win to default',exact:true}).click()
  await expect(wins.getByRole('button',{name:'Yes',exact:true})).toHaveAttribute('aria-pressed','true')
  release()
  await expect(panel.locator('.workstation-notice')).toContainText('Skills saved')
  await expect(wins.getByRole('button',{name:'Yes',exact:true})).toHaveAttribute('aria-pressed','true')
 } finally {
  await app?.close()
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})

test('skills panel allows configuring models per skill and persists them on the engine',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-skill-models-ui-'))
 const {state,server}=fakeAgent()
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 let app
 try {
  let panel
  ;({app,panel}=await openExecutionDefaults(server,root,'Skills'))

  const engineSelect=panel.getByRole('combobox',{name:'AI engine for skill models',exact:true})
  await expect(engineSelect).toBeVisible()
  await expect(engineSelect).toHaveValue('e-opus')

  const baseInput=panel.getByRole('combobox',{name:'Engine default model',exact:true})
  await expect(baseInput).toHaveValue('claude-opus-5')

  const clarifyInput=panel.getByRole('combobox',{name:'Model for skill clarify',exact:true})
  const implementInput=panel.getByRole('combobox',{name:'Model for skill implement',exact:true})
  await expect(clarifyInput).toHaveValue('')
  await expect(clarifyInput).toHaveAttribute('placeholder','Inherit (claude-opus-5)')
  await expect(implementInput).toHaveValue('claude-sonnet-5')

  await clarifyInput.fill('claude-haiku-4-5')
  await panel.getByRole('button',{name:'Save skills'}).click()
  await expect(panel.locator('.workstation-notice')).toContainText('Skills saved')

  assert.equal(state.enginePuts.length,1)
  const savedEngine=state.enginePuts[0].catalogue.find(e=>e.id==='e-opus')
  assert.deepEqual(savedEngine.skillModels,{clarify:'claude-haiku-4-5',implement:'claude-sonnet-5'})

  // Resetting returns all skill models to engine default
  await panel.getByRole('button',{name:'Reset skill models to engine default',exact:true}).click()
  await expect(clarifyInput).toHaveValue('')
  await expect(implementInput).toHaveValue('')

  await panel.getByRole('button',{name:'Save skills'}).click()
  await expect(panel.locator('.workstation-notice')).toContainText('Skills saved')
  assert.equal(state.enginePuts.length,2)
  const resetEngine=state.enginePuts[1].catalogue.find(e=>e.id==='e-opus')
  assert.equal(resetEngine.skillModels,undefined)
 } finally {
  await app?.close()
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})

test('the engines section presents supported engines, indicates installation status, and sets default engine',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-engines-ui-'))
 const {state,server}=fakeAgent({installed:{claude:true,codex:true,agy:false}})
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 let app
 try {
  let panel
  ;({app,panel}=await openExecutionDefaults(server,root,'AI engines'))
  const engines=panel.getByRole('list',{name:'Engines'})
  // All 3 supported engines are presented: Claude, Codex, AntiGravity.
  await expect(engines.locator('.engine-item')).toHaveCount(3)

  // Claude is the default:
  const claudeItem=engines.locator('.engine-item').nth(0)
  await expect(claudeItem).toContainText('Claude Opus')
  await expect(claudeItem.locator('.engine-default')).toContainText('Default')
  await expect(claudeItem.locator('.engine-installed-note')).toBeVisible()

  // Codex is installed and not default:
  const codexItem=engines.locator('.engine-item').nth(1)
  await expect(codexItem).toContainText('Codex')
  await expect(codexItem).not.toHaveClass(/not-installed/)
  await expect(codexItem.locator('.engine-installed-note')).toBeVisible()
  const makeDefaultBtn=codexItem.getByRole('button',{name:'Make Codex the default engine',exact:true})
  await expect(makeDefaultBtn).toBeVisible()

  // AntiGravity is not installed:
  const agyItem=engines.locator('.engine-item').nth(2)
  await expect(agyItem).toContainText('AntiGravity')
  await expect(agyItem).toHaveClass(/not-installed/)
  await expect(agyItem.locator('.engine-not-installed-note')).toHaveText('Engine is not installed on this workstation')
  await expect(agyItem.getByRole('button',{name:/Make .* the default engine/})).toHaveCount(0)

  // Making Codex default saves the choice:
  await makeDefaultBtn.click()
  await expect(panel.locator('.engines-notice')).toContainText('Default engine saved')
  await expect(codexItem.locator('.engine-default')).toContainText('Default')
  assert.equal(state.enginePuts.at(-1).default,'e-codex')

  // A refusal from the agent is shown as it gave it:
  state.refuse='local agent is offline'
  await claudeItem.getByRole('button',{name:'Make Claude Opus the default engine',exact:true}).click()
  await expect(panel.locator('.engines-notice')).toContainText('Not saved: local agent is offline')
 } finally {
  await app?.close()
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})

test('an agent without an engine catalogue is asked to update',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-engines-old-ui-'))
 const {server}=fakeAgent({capabilities:[]})
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 let app
 try {
  let panel
  ;({app,panel}=await openExecutionDefaults(server,root,'AI engines'))
  await expect(panel.locator('.engines-section')).toContainText('Update and restart the local agent to manage engines.')
  await expect(panel.locator('.engine-item')).toHaveCount(0)
 } finally {
  await app?.close()
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})


test('provider settings save and reset model lists independently',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-provider-models-'))
 const {state,server}=fakeAgent({defaults:{aiProviderModels:{claude:['claude-opus-5']}}})
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 let app
 try{
  const opened=await openExecutionDefaults(server,root);app=opened.app
  const {page}=opened
  await page.getByRole('tab',{name:'AI engines',exact:true}).click()
  await expect(page.locator('#settings-panel-Engines').getByText('Models offered')).toHaveCount(0)

  await page.getByRole('tab',{name:'Claude settings',exact:true}).click()
  await expect(page.getByRole('textbox',{name:'Models offered for claude',exact:true})).toBeVisible()

  await page.getByRole('tab',{name:'Antigravity settings',exact:true}).click()
  await expect(page.getByRole('textbox',{name:'Models offered for agy',exact:true})).toBeVisible()

  await page.getByRole('tab',{name:'Codex settings',exact:true}).click()
  const panel=page.locator('#settings-panel-Codex .provider-model-lists[data-provider=codex]')
  await panel.getByRole('textbox',{name:'Models offered for codex',exact:true}).fill('gpt-5, o4-mini')
  await panel.getByRole('button',{name:'Save models',exact:true}).click()
  await expect(panel.getByRole('status').filter({hasText:'Models saved'})).toBeVisible()
  assert.deepEqual(state.puts.at(-1).aiProviderModels,{claude:['claude-opus-5'],codex:['gpt-5','o4-mini']})
  assert.equal(state.puts.at(-1).editorCommand,'zed')
  await panel.getByRole('button',{name:'Reset codex models to the shipped list',exact:true}).click()
  await expect(panel.getByRole('textbox',{name:'Models offered for codex',exact:true})).toHaveValue('gpt-5')
  await panel.getByRole('button',{name:'Save models',exact:true}).click()
  await expect(panel.getByRole('status').filter({hasText:'Models saved'})).toBeVisible()
  assert.deepEqual(state.puts.at(-1).aiProviderModels,{claude:['claude-opus-5']})
 }finally{if(app)await app.close();await new Promise(resolve=>server.close(resolve));fs.rmSync(root,{recursive:true,force:true})}
})


test('skill commands map named steps to standard and custom commands',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-command-mapping-'))
 const {state,server}=fakeAgent({defaults:{skillCommands:{implement:'/custom-implement'}}})
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 let app
 try{
  const opened=await openExecutionDefaults(server,root,'Skills');app=opened.app
  const {panel}=opened
  const mapping=panel.getByRole('table',{name:'Skill command mapping'})
  await expect(mapping.getByRole('columnheader')).toHaveText(['Step','Command'])
  const input=mapping.getByRole('textbox',{name:'Command for skill implement',exact:true})
  await expect(input).toHaveAttribute('placeholder','/implement-issue')
  await expect(input).toHaveValue('/custom-implement')
  await input.fill('/my-implement')
  await panel.getByRole('button',{name:'Save skills'}).click()
  await expect(panel.locator('.workstation-notice')).toContainText('Skills saved')
  assert.deepEqual(state.puts.at(-1).skillCommands,{implement:'/my-implement'})
  await expect(input).toHaveValue('/my-implement')
  await opened.page.screenshot({path:'/private/tmp/sectile-command-mapping.png'})
  await panel.getByRole('button',{name:'Reset skill command names to the standard ones'}).click()
  await expect(input).toHaveValue('')
  await panel.getByRole('button',{name:'Save skills'}).click()
  await expect(panel.locator('.workstation-notice')).toContainText('Skills saved')
  assert.deepEqual(state.puts.at(-1).skillCommands,{})
 }finally{if(app)await app.close();await new Promise(resolve=>server.close(resolve));fs.rmSync(root,{recursive:true,force:true})}
})
