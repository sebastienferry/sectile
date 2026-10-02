const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The Sandbox category of a project's settings (#700): the state, the two
// sandbox lists and the two rule lists, saved with the project's other local
// settings and read back on reopening.
async function withDesktop(project,run){
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-sandbox-ui-'))
 const saved=[]
 let current=project
 const server=http.createServer((req,res)=>{
  if(req.headers.authorization!=='Bearer test-secret'){res.writeHead(401).end();return}
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/projects'&&req.method==='POST'){let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
   const body=JSON.parse(raw);saved.push(body)
   if(body.claudeSandbox)current={...current,claudeSandbox:body.claudeSandbox}
   res.writeHead(204).end()
  });return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Example project',path:'/tmp/sandbox-worktree'}]));return}
  if(req.url==='/desktop/project?id=project-a'){res.end(JSON.stringify(current));return}
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:['repositories']}));return}
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
   await page.getByRole('tab',{name:'Sandbox',exact:true}).click()
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
  await run({page,open,close,save,saved})
 }finally{
  if(app)await app.close()
  server.close()
 }
}

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
