const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The specifications folder shows what a mono-repo project inherits, flags a
// multi-repo project without one while still saving, and shows the detected
// kind of the folder the agent stored.
test('desktop specifications folder inherits, flags and shows its kind',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-spec-ui-')),saves=[]
 let project={configured:true,monoRepo:true,path:'/test/repo',specPath:'',specDefault:'/test/repo',specKind:'git',aiProvider:'agy',server:{projectId:'p',projectName:'Test project',aiProvider:'agy',skills:[]}}
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify(project));return}
  if(url.pathname==='/desktop/projects'){
   if(req.method==='POST'){
    let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{
     const input=JSON.parse(body);saves.push(input)
     // The agent stores a plain folder as typed and says it is one.
     project={...project,specPath:input.specPath,specKind:input.specPath?'folder':'unset'}
     res.statusCode=204;res.end()
    });return
   }
   res.end(JSON.stringify([{id:'p',name:'Test project',path:'/test/repo'}]));return
  }
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://example.test'}));return}
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
   await page.getByRole('button',{name:'Configure Test project',exact:true}).click()
   return {field:page.getByRole('textbox',{name:'Specifications folder',exact:true}),kind:page.getByRole('status',{name:'Specifications folder kind',exact:true}),row:page.locator('.setting-row',{has:page.getByRole('textbox',{name:'Specifications folder',exact:true})})}
  }

  // Mono-repo, no override: empty, the checkout as placeholder, inherited.
  let {field,kind,row}=await open()
  await expect(field).toHaveValue('')
  await expect(field).toHaveAttribute('placeholder','/test/repo')
  await expect(row).toContainText('Inherited from the local repository')
  await expect(kind).toHaveText('Git repository')
  await page.keyboard.press('Escape')

  // Multi-repo, nothing set: flagged as required, and saving still works.
  project={...project,monoRepo:false,specDefault:'',specKind:'unset'}
  ;({field,kind,row}=await open())
  await expect(field).toHaveAttribute('placeholder','Required for a multi-repo project')
  await expect(field).toHaveAttribute('aria-invalid','true')
  await expect(row).toContainText('Required')
  // An empty kind is hidden, and so out of the accessibility tree.
  await expect(page.locator('.spec-kind')).toBeHidden()

  // Saving a plain folder refreshes the kind from what the agent stored.
  await field.fill('/test/plain-specs')
  await page.getByRole('button',{name:'Save local configuration',exact:true}).click()
  await expect.poll(()=>saves.length).toBe(1)
  assert.equal(saves[0].specPath,'/test/plain-specs')
  await expect(kind).toHaveText('Folder (not a Git repository)')
  await expect(field).toHaveAttribute('aria-invalid','false')
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
