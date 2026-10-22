const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The Sandbox category of a project's settings (#700): the state, the two
// sandbox lists and the two rule lists, saved with the project's other local
// settings and read back on reopening.
async function withDesktop(project,run,workstation=WORKSTATION,{others=[],disconnected=[]}={}){
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-sandbox-ui-'))
 const saved=[],workstationSaves=[],promotions=[]
 let current=project,global=workstation
 const server=http.createServer((req,res)=>{
  if(req.headers.authorization!=='Bearer test-secret'){res.writeHead(401).end();return}
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/projects'&&req.method==='POST'){let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
   const body=JSON.parse(raw);saved.push(body)
   if(body.claudeSandbox)current={...current,claudeSandbox:body.claudeSandbox}
   res.writeHead(204).end()
  });return}
  // The workstation Sandbox values and the move of a rule up (#730).
  if(req.url==='/desktop/workstation/sandbox'&&req.method==='PUT'){let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
   const body=JSON.parse(raw);workstationSaves.push(body)
   global={...global,claudeSandbox:body.claudeSandbox,projects:body.projects}
   res.end(JSON.stringify(global))
  });return}
  if(req.url==='/desktop/workstation/sandbox'){res.end(JSON.stringify(global));return}
  if(req.url==='/desktop/project/sandbox/promote'&&req.method==='POST'){let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
   const body=JSON.parse(raw);promotions.push(body)
   const own={...current.claudeSandbox,allow:current.claudeSandbox.allow.filter(rule=>rule!==body.rule)}
   const inherited={...current.claudeSandboxGlobal,allow:[...current.claudeSandboxGlobal.allow,body.rule]}
   current={...current,claudeSandbox:own,claudeSandboxGlobal:inherited}
   res.end(JSON.stringify({claudeSandbox:own,claudeSandboxGlobal:inherited,claudeSandboxCovered:true}))
  });return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Example project',path:'/tmp/sandbox-worktree'},...others]));return}
  if(req.url==='/desktop/project?id=project-a'){res.end(JSON.stringify(current));return}
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:['repositories'],disconnectedProjects:disconnected}));return}
  if(req.url==='/desktop/runs'){res.end('[]');return}
  if(req.url.startsWith('/desktop/repositories')){res.end('[]');return}
  if(req.url.startsWith('/desktop/tasks?')){res.end('[]');return}
  if(req.url==='/desktop/version'){res.end('{"version":"test"}');return}
  res.writeHead(404).end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}),{mode:0o600})
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({executablePath:process.env.SECTILE_DESKTOP_EXECUTABLE,args:process.env.SECTILE_DESKTOP_EXECUTABLE?[]:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  const open=async()=>{
   await page.getByRole('button',{name:'Actions for Example project',exact:true}).click()
   await page.getByRole('menuitem',{name:'Project settings…',exact:true}).click()
  }
  const close=async()=>{
   await page.locator('.configuration-page').getByRole('button',{name:'Back',exact:true}).click()
   await expect(page.locator('.configuration-page')).toHaveCount(0)
  }
  const save=async()=>{
   const before=saved.length
   await page.getByRole('button',{name:'Save local configuration',exact:true}).click()
   await expect.poll(()=>saved.length).toBe(before+1)
   return saved.at(-1)
  }
  const openWorkstation=async()=>{
   await page.locator('#settings').click()
   await page.getByRole('tab',{name:'AI engines',exact:true}).click()
   await page.locator('#settings-tab-Sandbox').click()
  }
  await run({page,open,close,save,saved,openWorkstation,workstationSaves,promotions})
 }finally{
  if(app)await app.close()
  server.close()
 }
}

const EMPTY={enabled:null,allowedDomains:[],excludedCommands:[],allowWrite:[],allow:[],deny:[]}
const WORKSTATION={claudeSandbox:EMPTY,projects:[],platformSandbox:true}
const PROJECT={server:{projectName:'Example project',skills:[]},path:'/tmp/sandbox-worktree',configured:true,aiProvider:'claude',
 claudeSandbox:{enabled:null,allowedDomains:[],excludedCommands:[],allowWrite:[],allow:[],deny:[]},platformSandbox:true,claudeSettingsPath:'/home/me/.config/sectile/claude/project-a.json'}

// The workstation Sandbox category (#730): its values and the projects they
// cover, saved through their own endpoint.
test('the workstation Claude settings category saves its values and its whitelist',async()=>{
 await withDesktop(PROJECT,async({page,openWorkstation,workstationSaves})=>{
  await openWorkstation()
  const panel=page.locator('#settings-panel-Sandbox')
  const whitelist=page.getByRole('group',{name:'Projects the Claude settings apply to',exact:true})
  const checkbox=whitelist.getByRole('checkbox',{name:'Example project',exact:true})
  // The projects are hidden at first, so the row is found by its radio group.
  const hint=panel.locator('.setting-row').filter({has:page.getByRole('radiogroup',{name:'Projects the Claude settings cover',exact:true})}).locator('.setting-text p').first()
  // Every project is covered, so the projects stay out of sight until asked for.
  await expect(panel.getByRole('radio',{name:'All projects',exact:true})).toBeChecked()
  await expect(checkbox).toBeHidden()
  await expect(hint).toContainText('Every project')
  await panel.getByRole('radio',{name:'Only these projects',exact:true}).check()
  await expect(checkbox).not.toBeChecked()
  await expect(hint).toContainText('Check at least one project')
  await expect(panel.locator('.command-preview')).toHaveCount(0)
  await panel.getByRole('group',{name:'Claude Code sandbox',exact:true}).getByRole('button',{name:'On',exact:true}).click()
  await panel.getByRole('textbox',{name:'New entry for Deny rules',exact:true}).fill('Bash(git push:*)')
  await panel.getByRole('button',{name:'Add to Deny rules',exact:true}).click()
  await checkbox.check()
  await expect(hint).toContainText('only to the checked projects')
  await panel.getByRole('button',{name:'Save Claude settings',exact:true}).click()
  await expect.poll(()=>workstationSaves.length).toBe(1)
  assert.deepEqual(workstationSaves[0],{claudeSandbox:{...EMPTY,enabled:true,deny:['Bash(git push:*)']},claudeSandboxBase:EMPTY,projects:['project-a']})
  await expect(panel.getByRole('status')).toHaveText('Claude settings saved')
 })
})

test('on Windows the workstation sandbox part is disabled and the rules and whitelist stay editable',async()=>{
 await withDesktop(PROJECT,async({page,openWorkstation})=>{
  await openWorkstation()
  const panel=page.locator('#settings-panel-Sandbox')
  const state=panel.getByRole('group',{name:'Claude Code sandbox',exact:true})
  for(const name of ['Inherited','On','Off'])await expect(state.getByRole('button',{name,exact:true})).toBeDisabled()
  // has: takes a locator relative to the row, so the group is not scoped to the panel.
  const stateRow=panel.locator('.setting-row').filter({has:page.getByRole('group',{name:'Claude Code sandbox',exact:true})})
  await expect(stateRow.locator('.setting-text p').first()).toHaveText('Claude Code’s sandbox does not run on Windows: only the permission rules below apply.')
  await expect(panel.getByRole('textbox',{name:'New entry for Allowed network domains',exact:true})).toBeDisabled()
  await expect(panel.getByRole('textbox',{name:'New entry for Commands outside the sandbox',exact:true})).toBeDisabled()
  await expect(panel.getByRole('textbox',{name:'New entry for Extra writable paths',exact:true})).toBeDisabled()
  await expect(panel.getByRole('textbox',{name:'New entry for Allow rules',exact:true})).toBeEnabled()
  // A preset of commands outside the sandbox has nothing to apply here (#764).
  await expect(panel.getByRole('button',{name:'Apply preset Outside the sandbox (git, gh, glab)',exact:true})).toBeHidden()
  await expect(panel.getByRole('textbox',{name:'New entry for Deny rules',exact:true})).toBeEnabled()
  await panel.getByRole('radio',{name:'Only these projects',exact:true}).check()
  const whitelist=panel.getByRole('group',{name:'Projects the Claude settings apply to',exact:true})
  await expect(whitelist.getByRole('checkbox',{name:'Example project',exact:true})).toBeEnabled()
 },{...WORKSTATION,platformSandbox:false})
})

// A disconnected project keeps its settings and its place in the whitelist, so
// it is still listed, marked hidden: left out, the next save would drop it. A
// project hidden from the sidebar only is listed under its own name.
test('the workstation whitelist keeps a disconnected project and a project hidden from the sidebar',async()=>{
 const others=[{id:'project-b',name:'Gone project',configured:true},{id:'project-c',name:'Quiet project',path:'/tmp/quiet-worktree'}]
 await withDesktop(PROJECT,async({page,openWorkstation,workstationSaves})=>{
  await page.getByRole('button',{name:'Actions for Quiet project',exact:true}).click()
  await page.getByRole('menuitem',{name:'Hide from sidebar',exact:true}).click()
  await expect(page.getByRole('button',{name:'Actions for Quiet project',exact:true})).toHaveCount(0)
  await expect(page.getByRole('button',{name:'Actions for Gone project',exact:true})).toHaveCount(0)
  await openWorkstation()
  const panel=page.locator('#settings-panel-Sandbox')
  const whitelist=panel.getByRole('group',{name:'Projects the Claude settings apply to',exact:true})
  await expect(panel.getByRole('radio',{name:'Only these projects',exact:true})).toBeChecked()
  await expect(whitelist.getByRole('checkbox')).toHaveCount(3)
  await expect(whitelist.getByRole('checkbox',{name:'Gone project (hidden)',exact:true})).toBeChecked()
  await expect(whitelist.getByRole('checkbox',{name:'Quiet project',exact:true})).toBeChecked()
  await whitelist.getByRole('checkbox',{name:'Example project',exact:true}).check()
  await panel.getByRole('button',{name:'Save Claude settings',exact:true}).click()
  await expect.poll(()=>workstationSaves.length).toBe(1)
  assert.deepEqual([...workstationSaves[0].projects].sort(),['project-a','project-b','project-c'])
 },{...WORKSTATION,projects:['project-b','project-c']},{others,disconnected:['project-b']})
})

// The presets of the workstation Claude settings (#745): applying one copies
// its entries into the lists, which the save keeps like any other edit.
const presetsOf=async()=>(await import('../src/claude-presets.mjs')).PRESETS
// has: takes a locator relative to the card, so it starts from the page.
const presetCard=(panel,name)=>panel.locator('.claude-preset').filter({has:panel.page().locator('.claude-preset-head strong').getByText(name,{exact:true})})

test('an empty workstation applies the recommended presets in one click',async()=>{
 const presets=await presetsOf()
 const common=presets.find(p=>p.id==='common'),dangerous=presets.find(p=>p.id==='dangerous')
 await withDesktop(PROJECT,async({page,openWorkstation,workstationSaves})=>{
  await openWorkstation()
  const panel=page.locator('#settings-panel-Sandbox')
  const recommended=panel.getByRole('button',{name:'Apply recommended',exact:true})
  await expect(recommended).toBeVisible()
  await expect(panel.locator('.claude-preset')).toHaveCount(presets.length)
  await recommended.click()
  await expect(recommended).toBeHidden()
  for(const name of ['Common','Dangerous actions']){
   await expect(presetCard(panel,name).locator('.sandbox-origin')).toBeVisible()
   await expect(panel.getByRole('button',{name:'Remove preset '+name,exact:true})).toBeVisible()
  }
  await expect(presetCard(panel,'Go').locator('.sandbox-origin')).toBeHidden()
  await expect(panel.getByRole('list',{name:'Deny rules',exact:true}).getByText('Bash(terraform apply *)',{exact:true})).toBeVisible()
  await panel.getByRole('button',{name:'Save Claude settings',exact:true}).click()
  await expect.poll(()=>workstationSaves.length).toBe(1)
  assert.deepEqual(workstationSaves[0].claudeSandbox,{...EMPTY,allow:common.allow,deny:dangerous.deny,allowedDomains:common.allowedDomains})
 })
})

test('a preset applied then edited by hand offers Apply again, and Remove takes its entries out',async()=>{
 const presets=await presetsOf()
 const go=presets.find(p=>p.id==='go')
 const start={...EMPTY,allow:['Bash(git add *)']}
 await withDesktop(PROJECT,async({page,openWorkstation,workstationSaves})=>{
  await openWorkstation()
  const panel=page.locator('#settings-panel-Sandbox')
  // The lists are not empty, so nothing is recommended.
  await expect(panel.getByRole('button',{name:'Apply recommended',exact:true})).toBeHidden()
  await panel.getByRole('button',{name:'Apply preset Go',exact:true}).click()
  await expect(panel.getByRole('button',{name:'Remove preset Go',exact:true})).toBeVisible()
  await panel.getByRole('button',{name:'Remove '+go.allowWrite[0]+' from Extra writable paths',exact:true}).click()
  await expect(panel.getByRole('button',{name:'Apply preset Go',exact:true})).toBeVisible()
  await panel.getByRole('button',{name:'Apply preset Go',exact:true}).click()
  await panel.getByRole('button',{name:'Remove preset Go',exact:true}).click()
  await expect(panel.getByRole('button',{name:'Apply preset Go',exact:true})).toBeVisible()
  await panel.getByRole('button',{name:'Save Claude settings',exact:true}).click()
  await expect.poll(()=>workstationSaves.length).toBe(1)
  // The rule typed before stays: Go never held it.
  assert.deepEqual(workstationSaves[0].claudeSandbox,start)
 },{...WORKSTATION,claudeSandbox:start})
})

test('on Windows a preset adds its rules only',async()=>{
 const presets=await presetsOf()
 const go=presets.find(p=>p.id==='go')
 await withDesktop(PROJECT,async({page,openWorkstation,workstationSaves})=>{
  await openWorkstation()
  const panel=page.locator('#settings-panel-Sandbox')
  const card=presetCard(panel,'Go')
  await panel.getByLabel('Entries of Go',{exact:true}).click()
  await expect(card).toContainText('do not apply on Windows')
  await panel.getByRole('button',{name:'Apply preset Go',exact:true}).click()
  await expect(panel.getByRole('button',{name:'Remove preset Go',exact:true})).toBeVisible()
  await panel.getByRole('button',{name:'Save Claude settings',exact:true}).click()
  await expect.poll(()=>workstationSaves.length).toBe(1)
  assert.deepEqual(workstationSaves[0].claudeSandbox,{...EMPTY,allow:go.allow})
 },{...WORKSTATION,platformSandbox:false})
})

test('the outside-the-sandbox preset fills the commands outside the sandbox (#764)',async()=>{
 const presets=await presetsOf()
 const outside=presets.find(p=>p.id==='outside-sandbox')
 await withDesktop(PROJECT,async({page,openWorkstation,workstationSaves})=>{
  await openWorkstation()
  const panel=page.locator('#settings-panel-Sandbox')
  const card=presetCard(panel,outside.name)
  await expect(card.locator('.claude-preset-kinds')).toHaveText('7 commands')
  await panel.getByRole('button',{name:'Apply preset '+outside.name,exact:true}).click()
  const list=panel.getByRole('list',{name:'Commands outside the sandbox',exact:true})
  await expect(list.locator('li')).toHaveCount(outside.excludedCommands.length)
  await panel.getByRole('button',{name:'Save Claude settings',exact:true}).click()
  await expect.poll(()=>workstationSaves.length).toBe(1)
  assert.deepEqual(workstationSaves[0].claudeSandbox,{...EMPTY,excludedCommands:outside.excludedCommands})
 })
})

// The Claude settings tab in workstation settings draws the Claude mark in Claude's orange (#778),
// idle or selected. Project settings do not carry a Claude settings tab.
test('the Claude settings tab draws the Claude mark in Claude’s orange',async()=>{
 await withDesktop(PROJECT,async({page,open,close,openWorkstation})=>{
  const orange='rgb(217, 119, 87)'
  await open()
  await expect(page.locator('#project-tab-Sandbox')).toHaveCount(0)
  await expect(page.getByRole('tab',{name:'Claude settings',exact:true})).toHaveCount(0)
  await close()
  await openWorkstation()
  await expect(page.locator('#settings-tab-Sandbox')).toHaveAttribute('aria-selected','true')
  const tabs=page.getByRole('tab',{name:'Claude settings',exact:true})
  assert.equal(await tabs.count(),1)
  const icon=tabs.first().locator('svg')
  await expect(icon).toHaveCount(1)
  await expect(icon).toHaveClass('claude-mark')
  await expect(icon).not.toHaveAttribute('stroke')
  await expect(icon).toHaveCSS('color',orange)
  // The other categories keep their outline, in the tab colour.
  await expect(page.locator('#settings-tab-Logs svg')).toHaveAttribute('stroke','currentColor')
  // The mark next to Conversation permission mode, in the same panel, takes the same orange.
  await expect(page.locator('#settings-panel-Sandbox .setting-name svg.claude-mark')).toHaveCSS('color',orange)
 })
})
