const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// A fake agent answering the workstation routes. The engine catalogue (#510)
// is kept as the agent keeps it: a PUT replaces it and answers the new view.
function fakeAgent({capabilities=['task-engines'],customSkillsUsed=[],defaults={}}={}){
 const state={puts:[],enginePuts:[],refuse:'',engines:{
  catalogue:[
   {id:'e-opus',name:'Claude Opus',provider:'claude',model:'claude-opus-5',skillModels:{implement:'claude-sonnet-5'}},
   {id:'e-codex',name:'Codex',provider:'codex'},
  ],
  default:'e-opus',providers:['claude','codex','agy','custom'],providerModels:{},projects:{p:'e-codex'},taskCounts:{'e-codex':2},
 }}
 const view=()=>({
  globalConfiguration:true,
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
   state.puts.push(input);state.skillSettings={customSkillsWin:input.customSkillsWin,installedSkillSource:input.installedSkillSource}
   res.writeHead(204).end();return
  }
  if(req.url==='/desktop/workstation'){res.end(JSON.stringify(view()));return}
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
 return {app,page,panel:page.locator(category==='AI engines'?'#settings-panel-Engines':'.execution-defaults')}
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
  await panel.getByRole('textbox',{name:'Models offered for codex',exact:true}).fill('gpt-5, o4-mini')
  await panel.getByRole('combobox',{name:'Initialization provider',exact:true}).selectOption('codex')
  await panel.getByRole('button',{name:'Save execution defaults'}).click()
  await expect(panel.locator('.workstation-notice')).toContainText('Execution defaults saved')
  assert.equal(state.puts.length,1)
  assert.equal(state.puts[0].initializationProvider,'codex')
  assert.deepEqual(state.puts[0].skillCommands,{})
  assert.equal(state.puts[0].aiProvider,undefined)
  assert.equal(state.puts[0].aiModel,undefined)
  assert.deepEqual(state.puts[0].aiProviderModels,{codex:['gpt-5','o4-mini']})
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
   ;({app,panel,page}=await openExecutionDefaults(server,root))
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

// The two skill settings are saved through the agent; a reset sends neither,
// which the agent reads as its defaults (#267).
test('the skill settings save through the agent and reset to their defaults',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-skill-settings-ui-'))
 const {state,server}=fakeAgent()
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 let app
 try {
  let panel
  ;({app,panel}=await openExecutionDefaults(server,root))
  const wins=panel.getByRole('group',{name:'Custom project skills win',exact:true})
  await expect(wins.getByRole('button',{name:'Yes',exact:true})).toHaveAttribute('aria-pressed','true')
  const source=panel.getByRole('combobox',{name:'Installed skills source',exact:true})
  await expect(source).toHaveValue('direct')
  await wins.getByRole('button',{name:'No',exact:true}).click()
  await source.selectOption('plugin')
  await panel.getByRole('button',{name:'Save execution defaults'}).click()
  await expect(panel.locator('.workstation-notice')).toContainText('Execution defaults saved')
  assert.equal(state.puts.length,1)
  assert.equal(state.puts[0].customSkillsWin,false)
  assert.equal(state.puts[0].installedSkillSource,'plugin')
  // The panel refills from what the agent now serves.
  await expect(wins.getByRole('button',{name:'No',exact:true})).toHaveAttribute('aria-pressed','true')
  await expect(source).toHaveValue('plugin')

  await panel.getByRole('button',{name:'Reset custom project skills win to default',exact:true}).click()
  await panel.getByRole('button',{name:'Reset installed skills source to default',exact:true}).click()
  await expect(source).toHaveValue('direct')
  await panel.getByRole('button',{name:'Save execution defaults'}).click()
  await expect(panel.locator('.workstation-notice')).toContainText('Execution defaults saved')
  assert.equal(state.puts.length,2)
  assert.equal(state.puts[1].customSkillsWin,undefined)
  assert.equal(state.puts[1].installedSkillSource,undefined)
 } finally {
  await app?.close()
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})

test('the engines section adds, edits, reorders, makes default and removes engines',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-engines-ui-'))
 const {state,server}=fakeAgent()
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 let app
 try {
  let panel
  ;({app,panel}=await openExecutionDefaults(server,root,'AI engines'))
  const engines=panel.getByRole('list',{name:'Engines'})
  await expect(engines.locator('.engine-item')).toHaveCount(2)
  await expect(engines.locator('.engine-item').first()).toContainText('Claude Opus')
  await expect(engines.locator('.engine-item').first()).toContainText('Default')
  await expect(engines.locator('.engine-item').nth(1)).toContainText('codex · provider default')
  // The default engine cannot be removed, the other one can.
  await expect(panel.getByRole('button',{name:'Remove Claude Opus',exact:true})).toBeDisabled()
  await expect(panel.getByRole('button',{name:'Move Claude Opus up',exact:true})).toBeDisabled()

  // Add an engine through the editor.
  await panel.getByRole('button',{name:'Add an engine'}).click()
  const editor=panel.getByRole('group',{name:'Engine editor'})
  await editor.getByRole('textbox',{name:'Engine name',exact:true}).fill('Claude Sonnet')
  await editor.getByRole('combobox',{name:'AI Provider',exact:true}).selectOption('claude')
  await editor.getByRole('textbox',{name:'AI Model',exact:true}).fill('claude-sonnet-5')
  await editor.getByRole('button',{name:'Save engine'}).click()
  await expect(engines.locator('.engine-item')).toHaveCount(3)
  const added=state.enginePuts.at(-1)
  assert.equal(added.default,'e-opus')
  assert.deepEqual(added.catalogue.at(-1),{id:'',name:'Claude Sonnet',provider:'claude',model:'claude-sonnet-5',skillModels:{},command:'',commandAutonomous:''})

  // An invalid model is refused before it is sent.
  await panel.getByRole('button',{name:'Edit Codex',exact:true}).click()
  await editor.getByRole('textbox',{name:'AI Model',exact:true}).fill('gpt 5')
  const puts=state.enginePuts.length
  await editor.getByRole('button',{name:'Save engine'}).click()
  await expect(panel.locator('.engines-notice')).toContainText('Invalid model')
  assert.equal(state.enginePuts.length,puts)
  await editor.getByRole('button',{name:'Cancel'}).click()

  // Reorder and make default are saved at once.
  await panel.getByRole('button',{name:'Move Claude Sonnet up',exact:true}).click()
  await expect(engines.locator('.engine-item').nth(1)).toContainText('Claude Sonnet')
  assert.deepEqual(state.enginePuts.at(-1).catalogue.map(engine=>engine.name),['Claude Opus','Claude Sonnet','Codex'])
  await panel.getByRole('button',{name:'Make Codex the default engine',exact:true}).click()
  await expect(engines.locator('.engine-item').nth(2)).toContainText('Default')
  assert.equal(state.enginePuts.at(-1).default,'e-codex')

  // Removing an engine says what points at it, and asks first.
  await panel.getByRole('button',{name:'Remove Claude Opus',exact:true}).click()
  await expect(engines.locator('.engine-confirm')).toContainText('Remove Claude Opus? No project or task uses it.')
  await panel.getByRole('button',{name:'Keep Claude Opus',exact:true}).click()
  await expect(engines.locator('.engine-confirm')).toHaveCount(0)
  await panel.getByRole('button',{name:'Make Claude Opus the default engine',exact:true}).click()
  await panel.getByRole('button',{name:'Remove Codex',exact:true}).click()
  await expect(engines.locator('.engine-confirm')).toContainText('project Sectile will use the workstation default engine; 2 tasks will use their project default engine.')
  await panel.getByRole('button',{name:'Confirm the removal of Codex',exact:true}).click()
  await expect(engines.locator('.engine-item')).toHaveCount(2)
  assert.deepEqual(state.enginePuts.at(-1).catalogue.map(engine=>engine.name),['Claude Opus','Claude Sonnet'])

  // A refusal from the agent is shown as it gave it.
  state.refuse='engine "X": the name is already used by engine "x"'
  await panel.getByRole('button',{name:'Move Claude Sonnet up',exact:true}).click()
  await expect(panel.locator('.engines-notice')).toContainText('Not saved: engine "X"')
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
  await expect(panel.getByRole('button',{name:'Add an engine'})).toBeHidden()
 } finally {
  await app?.close()
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})
