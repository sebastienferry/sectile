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
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Example project',path:'/tmp/spec-worktree'}]));return}
  if(req.url==='/desktop/project?id=project-a'){res.end(JSON.stringify({server:{projectName:'Example project',gitRemoteUrl:'https://example.test/repo.git',specFramework:'speckit',useWorktrees:true,specArtifacts:'keep',skills:[]},path:'/tmp/spec-worktree',configured:true,useWorktrees:true,specArtifacts:'keep',specArtifactsOverride:false,specArtifactsTracked:tracked}));return}
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(req.url==='/desktop/runs'){res.end('[]');return}
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
  if(await page.locator('#project-dialog[open]').count())await page.locator('#close-dialog').click()
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
