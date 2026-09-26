const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The desktop project settings list the folders attached to a project on this
// workstation (#484): each one says what it is, a folder gone since is still
// listed and can be removed, a refusal shows the agent's message, and a
// checkout of a project repository refreshes Other repositories.
test('desktop project settings attach and remove folders',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-attached-folders-')),posted=[],removed=[]
 let folders=[]
 let mapped=''
 const project={configured:true,path:'/test/repo',specPath:'',specDefault:'/test/repo',specKind:'git',aiProvider:'agy',server:{projectId:'p',projectName:'Test project',aiProvider:'agy',skills:[]}}
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify(project));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'p',name:'Test project',path:'/test/repo'}]));return}
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://example.test',capabilities:['repositories','attached-folders']}));return}
  if(url.pathname==='/desktop/runs'){res.end('[]');return}
  if(url.pathname==='/desktop/settings'){res.end('{"aiProvider":"agy"}');return}
  if(url.pathname==='/desktop/repositories'){
   res.end(JSON.stringify([{url:'git@github.com:o/a.git',identity:'github.com/o/a',path:'/test/repo',code:true},{url:'git@github.com:o/b.git',identity:'github.com/o/b',path:mapped,code:false}]));return
  }
  if(url.pathname==='/desktop/folders'){
   if(req.method==='GET'){res.end(JSON.stringify(folders));return}
   if(req.method==='DELETE'){
    const gone=url.searchParams.get('path');removed.push(gone);folders=folders.filter(folder=>folder.path!==gone)
    res.statusCode=204;res.end();return
   }
   let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{
    const input=JSON.parse(body);posted.push(input)
    if(input.path==='/test/repo'){res.statusCode=409;res.setHeader('Content-Type','text/plain');res.end('/test/repo is already the project\'s local repository\n');return}
    if(input.path==='/src/b'){mapped='/src/b';res.end(JSON.stringify({mappedAs:'github.com/o/b'}));return}
    folders=[...folders,{path:input.path,kind:'folder',remote:'',identity:''}]
    res.statusCode=204;res.end()
   });return
  }
  res.end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'private'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 const choose=async folder=>app.evaluate(({dialog},chosen)=>{dialog.showOpenDialog=async()=>({canceled:false,filePaths:[chosen]})},folder)
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  const open=async()=>{
   await page.getByRole('button',{name:'Actions for Test project',exact:true}).click()
   await page.getByRole('menuitem',{name:'Project settings…',exact:true}).click()
  }
  const list=page.getByRole('list',{name:'Attached folders',exact:true})
  const status=page.locator('.attached-folder-status')
  const add=page.getByRole('button',{name:'Add folder…',exact:true})

  // An empty list at first, and Other repositories on a project that used to
  // be mono-repo.
  await open()
  await expect(list).toContainText('No attached folder')
  await expect(page.locator('.setting-row',{hasText:'Other repositories'})).toBeVisible()

  // A plain folder is attached at once and listed with what it is.
  await choose('/notes/acme');await add.click()
  await expect(list.locator('.attached-folder')).toHaveCount(1)
  await expect(list).toContainText('/notes/acme')
  await expect(list).toContainText('Folder, not a Git repository')
  assert.deepEqual(posted[0],{projectId:'p',path:'/notes/acme'})

  // A refusal shows the agent's message and lists nothing more.
  await choose('/test/repo');await add.click()
  await expect(status).toHaveText("/test/repo is already the project's local repository")
  await expect(list.locator('.attached-folder')).toHaveCount(1)

  // A checkout of a project repository becomes its folder.
  await choose('/src/b');await add.click()
  await expect(status).toContainText('github.com/o/b')
  await expect(page.getByRole('textbox',{name:'Folder of github.com/o/b',exact:true})).toHaveValue('/src/b')
  await expect(list.locator('.attached-folder')).toHaveCount(1)
  await page.keyboard.press('Escape')

  // Each kind reads as what it is, a folder gone since included, and it can go.
  folders=[
   {path:'/src/lib',kind:'git',remote:'git@github.com:o/lib.git',identity:'github.com/o/lib'},
   {path:'/src/scratch',kind:'git',remote:'',identity:''},
   {path:'/src/gone',kind:'missing',remote:'',identity:''},
   {path:'/src/other-b',kind:'git',remote:'git@github.com:o/b.git',identity:'github.com/o/b',duplicate:'github.com/o/b'},
  ]
  await open()
  await expect(list.locator('.attached-folder')).toHaveCount(4)
  await expect(list).toContainText('Git repository · git@github.com:o/lib.git')
  await expect(list).toContainText('Git repository, no remote')
  await expect(list).toContainText('Folder not found')
  await expect(list).toContainText('Duplicate of the folder of github.com/o/b')
  await page.getByRole('button',{name:'Remove /src/gone',exact:true}).click()
  await expect(list.locator('.attached-folder')).toHaveCount(3)
  assert.deepEqual(removed,['/src/gone'])
  await expect(list).not.toContainText('/src/gone')
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
