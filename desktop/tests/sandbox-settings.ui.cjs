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
   await page.locator('#project-tab-Sandbox').click()
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
   await page.locator('#settings-tab-Sandbox').click()
  }
  await run({page,open,close,save,saved,openWorkstation,workstationSaves,promotions})
 }finally{
  if(app)await app.close()
  server.close()
 }
}

const EMPTY={enabled:null,allowedDomains:[],allowWrite:[],allow:[],deny:[]}
const WORKSTATION={claudeSandbox:EMPTY,projects:[],platformSandbox:true}
const PROJECT={server:{projectName:'Example project',skills:[]},path:'/tmp/sandbox-worktree',configured:true,aiProvider:'claude',
 claudeSandbox:{enabled:null,allowedDomains:[],allowWrite:[],allow:[],deny:[]},platformSandbox:true,claudeSettingsPath:'/home/me/.config/sectile/claude/project-a.json'}

test('the Sandbox category edits, saves and reads back the project values',async()=>{
 await withDesktop(PROJECT,async({page,open,close,save,saved})=>{
  await open()
  const state=page.getByRole('group',{name:'Claude Code sandbox',exact:true})
  const stateRow=page.locator('.setting-row').filter({has:state})
  await expect(state.getByRole('button',{name:'Inherited',exact:true})).toHaveAttribute('aria-pressed','true')
  await expect(stateRow.locator('.setting-text p').first()).toHaveText('Inherited · Claude Code’s own settings decide')
  await expect(page.getByRole('list',{name:'Allowed network domains',exact:true})).toHaveText('None')
  const preview=page.locator('#project-panel-Sandbox .command-preview')
  await expect(preview).not.toContainText('--settings')

  const addTo=async(list,value)=>{
   await page.getByRole('textbox',{name:'New entry for '+list,exact:true}).fill(value)
   await page.getByRole('button',{name:'Add to '+list,exact:true}).click()
  }
  await state.getByRole('button',{name:'On',exact:true}).click()
  await addTo('Allowed network domains','registry.npmjs.org')
  await addTo('Extra writable paths','~/.cache/go-build')
  await addTo('Allow rules','Bash(make test:*)')
  // Enter adds the entry rather than submitting the settings.
  await page.getByRole('textbox',{name:'New entry for Deny rules',exact:true}).fill('Bash(git push:*)')
  await page.getByRole('textbox',{name:'New entry for Deny rules',exact:true}).press('Enter')
  assert.equal(saved.length,0)

  // A duplicate is not added, an empty entry is refused with a message.
  await addTo('Allowed network domains',' registry.npmjs.org ')
  const domains=page.getByRole('list',{name:'Allowed network domains',exact:true})
  await expect(domains.locator('li')).toHaveCount(1)
  await expect(page.locator('.setting-row').filter({has:domains}).locator('.sandbox-status')).toHaveText('registry.npmjs.org is already in the list.')
  await addTo('Allow rules','   ')
  const allowRow=page.locator('.setting-row').filter({has:page.getByRole('list',{name:'Allow rules',exact:true})})
  await expect(allowRow.locator('.sandbox-status')).toHaveText('Enter a value before adding it.')

  // A rule in both lists is said to be denied.
  await addTo('Allow rules','Bash(git push:*)')
  await expect(page.locator('.sandbox-overlap')).toHaveText('In both lists: Bash(git push:*). The deny rule wins, as it does in Claude Code.')
  await page.getByRole('button',{name:'Remove Bash(git push:*) from Allow rules',exact:true}).click()
  await expect(page.locator('.sandbox-overlap')).toBeHidden()

  await expect(preview).toContainText("--settings='/home/me/.config/sectile/claude/project-a.json'")
  const body=await save()
  assert.deepEqual(body.claudeSandbox,{enabled:true,allowedDomains:['registry.npmjs.org'],allowWrite:['~/.cache/go-build'],allow:['Bash(make test:*)'],deny:['Bash(git push:*)']})
  // The base is what the dialog read, so the agent keeps a rule added meanwhile.
  assert.deepEqual(body.claudeSandboxBase,{enabled:null,allowedDomains:[],allowWrite:[],allow:[],deny:[]})

  // Reopened, the category shows what was saved; a removed entry is gone
  // from the next save.
  await close();await open()
  await expect(page.getByRole('group',{name:'Claude Code sandbox',exact:true}).getByRole('button',{name:'On',exact:true})).toHaveAttribute('aria-pressed','true')
  await expect(page.getByRole('list',{name:'Extra writable paths',exact:true})).toContainText('~/.cache/go-build')
  await page.getByRole('button',{name:'Remove registry.npmjs.org from Allowed network domains',exact:true}).click()
  const second=await save()
  assert.deepEqual(second.claudeSandbox.allowedDomains,[])
  assert.deepEqual(second.claudeSandboxBase.allowedDomains,['registry.npmjs.org'])
 })
})

test('on Windows the sandbox part is disabled and the rules stay editable',async()=>{
 await withDesktop({...PROJECT,platformSandbox:false},async({page,open})=>{
  await open()
  const state=page.getByRole('group',{name:'Claude Code sandbox',exact:true})
  await expect(state.getByRole('button',{name:'On',exact:true})).toBeDisabled()
  await expect(page.locator('.setting-row').filter({has:state}).locator('.setting-text p').first()).toHaveText('Claude Code’s sandbox does not run on Windows: only the permission rules below apply.')
  await expect(page.getByRole('textbox',{name:'New entry for Allowed network domains',exact:true})).toBeDisabled()
  await expect(page.getByRole('textbox',{name:'New entry for Extra writable paths',exact:true})).toBeDisabled()
  await expect(page.getByRole('textbox',{name:'New entry for Allow rules',exact:true})).toBeEnabled()
  await expect(page.getByRole('textbox',{name:'New entry for Deny rules',exact:true})).toBeEnabled()
 })
})

// The workstation Sandbox category (#730): its values and the projects they
// cover, saved through their own endpoint.
test('the workstation Sandbox category saves its values and its whitelist',async()=>{
 await withDesktop(PROJECT,async({page,openWorkstation,workstationSaves})=>{
  await openWorkstation()
  const panel=page.locator('#settings-panel-Sandbox')
  const whitelist=page.getByRole('group',{name:'Projects the Sandbox values apply to',exact:true})
  const checkbox=whitelist.getByRole('checkbox',{name:'Example project',exact:true})
  await expect(checkbox).not.toBeChecked()
  await expect(panel.locator('.setting-row').filter({has:whitelist}).locator('.setting-text p').first()).toContainText('every project')
  await expect(panel.locator('.command-preview')).toHaveCount(0)
  await panel.getByRole('group',{name:'Claude Code sandbox',exact:true}).getByRole('button',{name:'On',exact:true}).click()
  await panel.getByRole('textbox',{name:'New entry for Deny rules',exact:true}).fill('Bash(git push:*)')
  await panel.getByRole('button',{name:'Add to Deny rules',exact:true}).click()
  await checkbox.check()
  await expect(panel.locator('.setting-row').filter({has:whitelist}).locator('.setting-text p').first()).toContainText('only to the checked projects')
  await panel.getByRole('button',{name:'Save Sandbox settings',exact:true}).click()
  await expect.poll(()=>workstationSaves.length).toBe(1)
  assert.deepEqual(workstationSaves[0],{claudeSandbox:{...EMPTY,enabled:true,deny:['Bash(git push:*)']},claudeSandboxBase:EMPTY,projects:['project-a']})
  await expect(panel.getByRole('status')).toHaveText('Sandbox settings saved')
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
  await expect(panel.getByRole('textbox',{name:'New entry for Extra writable paths',exact:true})).toBeDisabled()
  await expect(panel.getByRole('textbox',{name:'New entry for Allow rules',exact:true})).toBeEnabled()
  await expect(panel.getByRole('textbox',{name:'New entry for Deny rules',exact:true})).toBeEnabled()
  const whitelist=panel.getByRole('group',{name:'Projects the Sandbox values apply to',exact:true})
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
  const whitelist=panel.getByRole('group',{name:'Projects the Sandbox values apply to',exact:true})
  await expect(whitelist.getByRole('checkbox')).toHaveCount(3)
  await expect(whitelist.getByRole('checkbox',{name:'Gone project (hidden)',exact:true})).toBeChecked()
  await expect(whitelist.getByRole('checkbox',{name:'Quiet project',exact:true})).toBeChecked()
  await whitelist.getByRole('checkbox',{name:'Example project',exact:true}).check()
  await panel.getByRole('button',{name:'Save Sandbox settings',exact:true}).click()
  await expect.poll(()=>workstationSaves.length).toBe(1)
  assert.deepEqual([...workstationSaves[0].projects].sort(),['project-a','project-b','project-c'])
 },{...WORKSTATION,projects:['project-b','project-c']},{others,disconnected:['project-b']})
})

test('a covered project shows what it inherits and moves a rule up',async()=>{
 const covered={...PROJECT,claudeSandbox:{...EMPTY,allow:['Bash(npm test:*)']},claudeSandboxCovered:true,
  claudeSandboxGlobal:{...EMPTY,enabled:true,allow:['Read'],deny:['Bash(git push:*)']}}
 await withDesktop(covered,async({page,open,save,promotions})=>{
  await open()
  const panel=page.locator('#project-panel-Sandbox')
  await expect(panel.locator('.sandbox-coverage')).toContainText('apply to this project')
  const state=page.getByRole('group',{name:'Claude Code sandbox',exact:true})
  await expect(panel.locator('.setting-row').filter({has:state}).locator('.setting-text p').first()).toHaveText('Inherited from the workstation Sandbox settings · On')
  const allow=panel.getByRole('list',{name:'Allow rules',exact:true})
  const inherited=allow.locator('.sandbox-entry-inherited')
  await expect(inherited).toHaveCount(1)
  await expect(inherited).toContainText('Read')
  await expect(inherited.getByRole('button')).toHaveCount(0)
  // A project with only inherited values still hands its launches the file.
  await expect(panel.locator('.command-preview')).toContainText('--settings=')

  await panel.getByRole('button',{name:'Move to global: Bash(npm test:*)',exact:true}).click()
  await expect.poll(()=>promotions.length).toBe(1)
  assert.deepEqual(promotions[0],{projectId:'project-a',rule:'Bash(npm test:*)'})
  await expect(allow.locator('.sandbox-entry-inherited')).toHaveCount(2)
  await expect(allow.locator('.sandbox-entry:not(.sandbox-entry-inherited)')).toHaveCount(0)
  // The project save sends only the project's own values.
  const body=await save()
  assert.deepEqual(body.claudeSandbox.allow,[])
 })
})

test('a project left out of the whitelist says so and inherits nothing',async()=>{
 await withDesktop({...PROJECT,claudeSandboxCovered:false,claudeSandboxGlobal:null},async({page,open})=>{
  await open()
  const panel=page.locator('#project-panel-Sandbox')
  await expect(panel.locator('.sandbox-coverage')).toContainText('do not apply to this project')
  await expect(panel.locator('.sandbox-entry-inherited')).toHaveCount(0)
 })
})

