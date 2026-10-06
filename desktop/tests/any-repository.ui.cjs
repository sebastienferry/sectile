const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The desktop project settings turn on the Any repository option (#737) and
// set the folder the agent clones into, defaulting to the one the agent names.
test('desktop project settings set the Any repository option',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-any-repository-')),saved=[]
 const project={configured:true,path:'/test/repo',specPath:'',specDefault:'/test/repo',specKind:'git',aiProvider:'agy',anyRepository:false,clonesPath:'',clonesDefault:'/test',server:{projectId:'p',projectName:'Test project',aiProvider:'agy',skills:[]}}
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify(project));return}
  if(url.pathname==='/desktop/projects'){
   if(req.method==='POST'){let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{saved.push(JSON.parse(body));res.end('{}')});return}
   res.end(JSON.stringify([{id:'p',name:'Test project',path:'/test/repo'}]));return
  }
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://example.test',capabilities:['repositories','attached-folders']}));return}
  if(url.pathname==='/desktop/runs'){res.end('[]');return}
  if(url.pathname==='/desktop/settings'){res.end('{"aiProvider":"agy"}');return}
  if(url.pathname==='/desktop/repositories'){res.end(JSON.stringify([{url:'git@github.com:o/a.git',identity:'github.com/o/a',path:'/test/repo',code:true}]));return}
  if(url.pathname==='/desktop/folders'){res.end('[]');return}
  res.end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'private'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  await page.getByRole('button',{name:'Actions for Test project',exact:true}).click()
  await page.getByRole('menuitem',{name:'Project settings…',exact:true}).click()
  await page.getByRole('tab',{name:'Folders',exact:true}).click()
  const option=page.getByRole('checkbox',{name:'Let tasks change any repository',exact:true})
  const clones=page.getByRole('textbox',{name:'Clones folder',exact:true})
  const row=page.locator('.setting-row',{hasText:'Any repository'})

  // Off by default: no clones folder to set.
  await expect(option).not.toBeChecked()
  await expect(clones).toBeHidden()
  await expect(row).toContainText('only the repositories listed above')

  // On: the clones folder shows the agent's default until one is typed.
  await option.check()
  await expect(clones).toBeVisible()
  await expect(clones).toHaveAttribute('placeholder','/test')
  await expect(row).toContainText('cloned into /test')
  await clones.fill('/src/clones')
  await expect(row).toContainText('cloned into /src/clones')
  await page.getByRole('button',{name:'Save local configuration',exact:true}).click()
  await expect.poll(()=>saved.length).toBe(1)
  assert.equal(saved[0].anyRepository,true)
  assert.equal(saved[0].clonesPath,'/src/clones')
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
