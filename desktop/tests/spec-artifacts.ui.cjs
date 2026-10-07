const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The Specifications row (#487): it inherits the server value, stores a Keep
// or Drop override, resets to the server, and warns when dropping in a
// repository that already tracks specifications.
test('project settings override whether specification artefacts are dropped',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-spec-artifacts-ui-'))
 const saved=[]
 let tracked=0
 const server=http.createServer((req,res)=>{
  if(req.headers.authorization!=='Bearer test-secret'){res.writeHead(401).end();return}
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/projects'&&req.method==='POST'){let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{saved.push(JSON.parse(raw));res.writeHead(204).end()});return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Example project',path:'/tmp/spec-worktree'},{id:'project-b',name:'Other project',path:'/tmp/other-worktree'}]));return}
  if(req.url==='/desktop/project?id=project-b'){res.end(JSON.stringify({server:{projectName:'Other project',skills:[]},path:'/tmp/other-worktree',configured:true}));return}
  if(req.url==='/desktop/project?id=project-a'){res.end(JSON.stringify({server:{projectName:'Example project',gitRemoteUrl:'https://example.test/repo.git',specFramework:'speckit',useWorktrees:true,specArtifacts:'keep',skills:[]},path:'/tmp/spec-worktree',configured:true,useWorktrees:true,specArtifacts:'keep',specArtifactsOverride:false,specArtifactsTracked:tracked}));return}
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
  const openSettings=async()=>{
   await page.getByRole('button',{name:'Actions for Example project',exact:true}).click()
   await page.getByRole('menuitem',{name:'Project settings…',exact:true}).click()
   await page.getByRole('tab',{name:'Execution',exact:true}).click()
  }
  const row=()=>page.locator('.setting-row').filter({has:page.getByRole('group',{name:'Specifications',exact:true})})
  const save=async()=>{
   const before=saved.length
   await page.getByRole('button',{name:'Save local configuration',exact:true}).click()
   await expect.poll(()=>saved.length).toBe(before+1)
   return saved.at(-1)
  }

  await openSettings()
  const configuration=page.locator('.configuration-page')
  await expect(configuration).toBeVisible()
  await expect(page.locator('#workspace>aside')).toBeHidden()
  await expect(page.locator('#sidebar-resizer')).toBeHidden()
  await expect(page.locator('#workspace>article')).toBeHidden()
  await expect(page.locator('#project-dialog[open]')).toHaveCount(0)
  await expect(configuration.getByRole('button',{name:'Back',exact:true})).toBeVisible()
  assert.deepEqual(await configuration.locator('.settings-group-label').allTextContents(),['General','Example project','Other project'])
  const exampleToggle=configuration.getByRole('button',{name:'Example project',exact:true})
  const otherToggle=configuration.getByRole('button',{name:'Other project',exact:true})
  await expect(exampleToggle).toHaveAttribute('aria-expanded','true')
  await expect(otherToggle).toHaveAttribute('aria-expanded','false')
  await exampleToggle.click()
  await expect(configuration.getByRole('tab',{name:'Folders',exact:true})).toHaveCount(0)
  await otherToggle.click()
  await expect(exampleToggle).toHaveAttribute('aria-expanded','false')
  await expect(otherToggle).toHaveAttribute('aria-expanded','true')
  await configuration.getByRole('tab',{name:'Folders',exact:true}).click()
  await expect(configuration.getByRole('button',{name:'Refresh from server',exact:true})).toHaveCount(1)
  await exampleToggle.click()
  await expect(otherToggle).toHaveAttribute('aria-expanded','false')
  await configuration.getByRole('tab',{name:'Execution',exact:true}).click()
  await expect(configuration.getByRole('heading',{name:'Configuration',exact:true})).toHaveCount(0)
  const title=configuration.locator('.configuration-panel-title')
  await expect(title).toHaveText('Execution')
  const titleBounds=await title.boundingBox()
  const refresh=configuration.getByRole('button',{name:'Refresh from server',exact:true})
  await expect(refresh).toHaveCount(1)
  // Reopening project categories from the shared navigation replaces footer actions.
  for(let index=0;index<3;index++){
   await configuration.getByRole('tab',{name:'Folders',exact:true}).click()
   await expect(title).toHaveText('Folders')
   const bounds=await title.boundingBox()
   assert.equal(bounds.y,titleBounds.y)
   assert.equal(bounds.height,titleBounds.height)
   await expect(refresh).toHaveCount(1)
   await expect(configuration.getByRole('button',{name:'Save local configuration',exact:true})).toHaveCount(1)
  }
  await configuration.getByRole('tab',{name:'General',exact:true}).first().click()
  await expect(title).toHaveText('General')
  const globalTitleBounds=await title.boundingBox()
  assert.equal(globalTitleBounds.y,titleBounds.y)
  assert.equal(globalTitleBounds.height,titleBounds.height)
  await expect(refresh).toHaveCount(0)
  await expect(configuration.locator('.settings-group-label')).toHaveText(['General','Example project','Other project'])
  await configuration.getByRole('tab',{name:'Agent logs',exact:true}).click()
  await expect(configuration.getByRole('heading',{name:'Agent logs',exact:true})).toHaveCount(1)
  await configuration.getByRole('tab',{name:'Execution',exact:true}).click()
  await expect(configuration.getByRole('tab',{name:'Execution',exact:true})).toHaveAttribute('aria-selected','true')
  await expect(refresh).toHaveCount(1)
  const keep=page.getByRole('button',{name:'Keep',exact:true}),drop=page.getByRole('button',{name:'Drop',exact:true})
  await expect(keep).toHaveAttribute('aria-pressed','true')
  await expect(row().locator('.setting-text p').first()).toHaveText('Inherited · Server default: Keep')
  await expect(row().locator('.setting-warning')).toBeHidden()

  await drop.click()
  await expect(drop).toHaveAttribute('aria-pressed','true')
  await expect(row().locator('.setting-text p').first()).toHaveText('Local override · Server default: Keep')
  let body=await save()
  assert.equal(body.specArtifacts,'drop')
  assert.equal(body.inheritSpecArtifacts,false)

  // A repository that already tracks specifications is warned about them.
  tracked=4
  await configuration.getByRole('button',{name:'Back',exact:true}).click()
  await expect(configuration).toHaveCount(0)
  await expect(page.locator('#workspace>aside')).toBeVisible()
  await expect(page.locator('#workspace>article')).toBeVisible()
  await openSettings()
  await page.getByRole('button',{name:'Drop',exact:true}).click()
  await expect(row().locator('.setting-warning')).toHaveText("This repository already tracks 4 specification files. They stay in its history; only the next tasks' specifications are dropped.")
  await page.getByRole('button',{name:'Keep',exact:true}).click()
  await expect(row().locator('.setting-warning')).toBeHidden()

  await page.getByRole('button',{name:'Reset specifications to server default',exact:true}).click()
  await expect(row().locator('.setting-text p').first()).toHaveText('Inherited · Server default: Keep')
  body=await save()
  assert.equal(body.inheritSpecArtifacts,true)
 }finally{
  if(app)await app.close()
  server.close()
 }
})
