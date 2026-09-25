const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// A project folder that is not a Git repository yet is offered one, with an
// empty first commit, under the Local repository and the Specifications
// folder; declining, a Git failure and an agent too old to initialize all
// leave the settings as they were (#481).
test('desktop offers to initialize a project folder as a Git repository',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-git-init-ui-')),saves=[],inits=[]
 let project={configured:true,monoRepo:true,path:'/test/repo',specPath:'',specDefault:'/test/repo',specKind:'git',aiProvider:'agy',server:{projectId:'p',projectName:'Test project',aiProvider:'agy',skills:[]}}
 // What the fake agent knows of each folder; anything else is missing.
 const states={'/test/repo':'ready','/test/plain':'folder','/test/other':'folder','/test/unborn-specs':'unborn'}
 let capable=true,failCommit=false
 const body=req=>new Promise(resolve=>{let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>resolve(JSON.parse(raw||'{}')))})
 const server=http.createServer(async(req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify(project));return}
  if(url.pathname==='/desktop/projects'){
   if(req.method==='POST'){
    const input=await body(req);saves.push(input)
    if(states[input.path]!=='ready'&&states[input.path]!=='unborn'){res.statusCode=400;res.setHeader('Content-Type','text/plain');res.end('Select a local Git repository');return}
    project={...project,path:input.path,specPath:input.specPath}
    res.statusCode=204;res.end();return
   }
   res.end(JSON.stringify([{id:'p',name:'Test project',path:'/test/repo'}]));return
  }
  if(url.pathname==='/desktop/git-init'){
   if(req.method==='POST'){
    const input=await body(req);inits.push(input.path)
    if(failCommit){states[input.path]='unborn';res.statusCode=422;res.setHeader('Content-Type','text/plain');res.end('git commit: exit status 128: Author identity unknown');return}
    const initialized=states[input.path]==='folder'
    states[input.path]='ready'
    res.end(JSON.stringify({path:input.path,state:'ready',initialized,committed:true}));return
   }
   const folder=url.searchParams.get('path')
   res.end(JSON.stringify({path:folder,state:states[folder]||'missing'}));return
  }
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://example.test',capabilities:capable?['repositories','git-init']:['repositories']}));return}
  if(url.pathname==='/desktop/runs'){res.end('[]');return}
  if(url.pathname==='/desktop/settings'){res.end('{"aiProvider":"agy"}');return}
  res.end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'private'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  const open=async()=>{
   await page.getByRole('button',{name:'Actions for Test project',exact:true}).click();await page.getByRole('menuitem',{name:'Project settings…',exact:true}).click()
   const offer=name=>page.getByRole('group',{name:name+' Git initialization',exact:true})
   return {
    local:page.getByRole('textbox',{name:'Local repository',exact:true}),
    spec:page.getByRole('textbox',{name:'Specifications folder',exact:true}),
    localOffer:offer('Local repository'),
    specOffer:offer('Specifications folder'),
    localStatus:page.getByRole('status',{name:'Local repository Git initialization status',exact:true}),
    specStatus:page.getByRole('status',{name:'Specifications folder Git initialization status',exact:true})
   }
  }
  const save=()=>page.getByRole('button',{name:'Save local configuration',exact:true}).click()
  const close=()=>page.keyboard.press('Escape')

  // A ready repository has nothing to offer.
  let f=await open()
  await expect(f.local).toHaveValue('/test/repo')
  await expect(f.localOffer).toBeHidden()

  // A plain folder typed in is offered a repository, with what it means.
  await f.local.fill('/test/plain');await f.local.press('Tab')
  await expect(f.localOffer).toBeVisible()
  await expect(f.localOffer).toContainText('This folder is not a Git repository.')
  for(const point of ['never pushed','needs no remote','create worktrees','without its existing files'])await expect(f.localOffer).toContainText(point)
  await f.localOffer.getByRole('button',{name:'Initialize a Git repository',exact:true}).click()
  await expect(f.localOffer).toBeHidden()
  await expect(f.localStatus).toHaveText('Git repository initialized in /test/plain. Save the local configuration to use it.')
  assert.deepEqual(inits,['/test/plain'])
  // Initializing did not save; saving is the user's own action.
  assert.equal(saves.length,0)
  await save()
  await expect.poll(()=>saves.length).toBe(1)
  assert.equal(saves[0].path,'/test/plain')
  await close()

  // "Not now" hides the offer and saving is refused as it is today.
  f=await open()
  await f.local.fill('/test/other');await f.local.press('Tab')
  await expect(f.localOffer).toBeVisible()
  await f.localOffer.getByRole('button',{name:'Not now',exact:true}).click()
  await expect(f.localOffer).toBeHidden()
  await save()
  await expect.poll(()=>saves.length).toBe(2)
  await expect(page.getByText(/Select a local Git repository$/)).toBeVisible()
  assert.equal(states['/test/other'],'folder')
  await close()

  // A stored specifications folder with no commit is offered one at open,
  // and a Git failure is shown as Git words it, the offer kept for a retry.
  project={...project,path:'/test/plain',specPath:'/test/unborn-specs',specKind:'git'}
  failCommit=true
  f=await open()
  await expect(f.specOffer).toBeVisible()
  await expect(f.specOffer).toContainText('This Git repository has no commit yet.')
  await expect(f.localOffer).toBeHidden()
  await f.specOffer.getByRole('button',{name:'Initialize a Git repository',exact:true}).click()
  await expect(f.specStatus).toHaveText('git commit: exit status 128: Author identity unknown')
  await expect(f.specOffer).toBeVisible()
  await expect(f.specOffer).toContainText('This Git repository has no commit yet.')
  assert.equal(saves.length,2)
  // Following the local repository, the folder offers nothing of its own.
  await page.getByRole('checkbox',{name:'Specifications live in the code repository',exact:true}).check()
  await expect(f.specOffer).toBeHidden()
  await close()

  // An agent that cannot initialize leaves every field as it was.
  capable=false
  project={...project,path:'/test/other',specPath:'/test/unborn-specs'}
  f=await open()
  await expect(f.spec).toHaveValue('/test/unborn-specs')
  await f.local.fill('/test/plain');await f.local.press('Tab')
  await f.local.fill('/test/other');await f.local.press('Tab')
  await page.waitForTimeout(300)
  await expect(f.localOffer).toBeHidden()
  await expect(f.specOffer).toBeHidden()
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
