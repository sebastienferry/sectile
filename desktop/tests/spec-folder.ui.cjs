const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The specifications folder shares the code repository behind a checkbox on
// every project (#484), is never flagged as required, and the detected kind
// names the folder it was detected on.
test('desktop specifications folder shares the code repository and names what it detected',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-spec-ui-')),saves=[]
 let project={configured:true,path:'/test/repo',specPath:'',specDefault:'/test/repo',specKind:'git',aiProvider:'agy',server:{projectId:'p',projectName:'Test project',aiProvider:'agy',skills:[]}}
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify(project));return}
  if(url.pathname==='/desktop/projects'){
   if(req.method==='POST'){
    let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{
     const input=JSON.parse(body);saves.push(input)
     // The agent stores a plain folder as typed and says it is one.
     project={...project,specPath:input.specPath,specKind:input.specPath?'folder':'git'}
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
   await page.getByRole('button',{name:'Actions for Test project',exact:true}).click();await page.getByRole('menuitem',{name:'Project settings…',exact:true}).click()
   return {
    field:page.getByRole('textbox',{name:'Specifications folder',exact:true}),
    same:page.getByRole('checkbox',{name:'Specifications live in the code repository',exact:true}),
    kind:page.getByRole('status',{name:'Specifications folder kind',exact:true}),
    row:page.locator('.setting-row',{hasText:'Specifications folder'})
   }
  }
  const save=()=>page.getByRole('button',{name:'Save local configuration',exact:true}).click()

  // No override: the box is ticked, the folder hidden, and the kind names the
  // checkout it was detected on. The removed repository layout is not shown.
  let {field,same,kind,row}=await open()
  await expect(page.locator('.setting-row',{hasText:'Repository layout'})).toHaveCount(0)
  await expect(same).toBeChecked()
  await expect(field).toBeHidden()
  await expect(row).toContainText('in the local repository')
  await expect(kind).toHaveText('Git repository · /test/repo')

  // Unticking reveals the folder; a saved one shows its own kind and path.
  await same.uncheck()
  await expect(field).toBeVisible()
  await expect(field).toBeFocused()
  await field.fill('/test/own-specs')
  await expect(kind).toBeHidden()
  await save()
  await expect.poll(()=>saves.length).toBe(1)
  assert.equal(saves[0].specPath,'/test/own-specs')
  await expect(kind).toHaveText('Folder, not a Git repository · /test/own-specs')
  await expect(same).not.toBeChecked()

  // Ticking again drops the override.
  await same.check()
  await expect(field).toBeHidden()
  await save()
  await expect.poll(()=>saves.length).toBe(2)
  assert.equal(saves[1].specPath,'')
  await expect(kind).toHaveText('Git repository · /test/repo')
  await page.keyboard.press('Escape')

  // A project that used to be multi-repo reopens the same way: the box is
  // offered and nothing is flagged as required.
  ;({field,same,kind,row}=await open())
  await expect(same).toBeVisible()
  await expect(same).toBeChecked()
  await expect(row).not.toContainText('Required')
  await same.uncheck()
  await expect(field).toHaveAttribute('placeholder','Folder holding the specifications')
  await field.fill('/test/plain-specs')
  await save()
  await expect.poll(()=>saves.length).toBe(3)
  assert.equal(saves[2].specPath,'/test/plain-specs')
  await expect(kind).toHaveText('Folder, not a Git repository · /test/plain-specs')
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
