const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// Open terminal in a project's menu (#761) follows Project prompt, names the
// project and never a path, is disabled on a project with no local folder, and
// is left out against an agent that cannot open it.
test('a project menu opens a native terminal on the project',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-project-terminal-'))
 const state={capabilities:['task-engines','project-terminal'],opened:[],refuse:''}
 const body=req=>new Promise(resolve=>{let data='';req.on('data',chunk=>data+=chunk);req.on('end',()=>resolve(JSON.parse(data||'{}')))})
 const server=http.createServer(async(req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:state.capabilities}));return}
  if(url.pathname==='/desktop/workstation'){res.end(JSON.stringify({defaults:{},effective:{useWorktrees:true,parallelism:1,aiProviderModels:{}},providerModels:{},setupProviders:['claude'],seeded:{}}));return}
  if(url.pathname==='/desktop/engines'){res.end(JSON.stringify({catalogue:[{id:'e',name:'Claude',provider:'claude'}],default:'e',providers:['claude'],providerModels:{},projects:{},taskCounts:{}}));return}
  if(url.pathname==='/desktop/project-terminal'&&req.method==='POST'){
   const input=await body(req)
   if(state.refuse){res.writeHead(409,{'Content-Type':'text/plain'}).end(state.refuse+'\n');return}
   state.opened.push(input);res.end(JSON.stringify({opened:true,terminal:'kitty',directory:'/tmp/example'}));return
  }
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'mapped',name:'Mapped',path:'/tmp/example',configured:true},{id:'remote',name:'Remote',path:'',configured:false}]));return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  // A project with no local folder is listed through its executions only.
  if(url.pathname==='/desktop/runs'){res.end(JSON.stringify([{id:'old',projectId:'remote',taskId:'t',taskKey:'#1',sessionId:'old',skill:'implement',status:'exited',createdAt:'2026-10-06T10:00:00Z'}]));return}
  if(url.pathname==='/desktop/tasks'){res.end('[]');return}
  if(url.pathname==='/desktop/version'){res.end('{"version":"test"}');return}
  res.writeHead(404).end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}),{mode:0o600})
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  const menu=name=>page.getByRole('menu',{name:'Actions for '+name,exact:true})
  const openMenu=async name=>{await page.getByRole('button',{name:'Actions for '+name,exact:true}).click();await expect(menu(name)).toBeVisible()}
  const item=name=>menu(name).getByRole('menuitem',{name:'Open terminal',exact:true})

  // The item follows Project prompt, and a click names the project only.
  await openMenu('Mapped')
  await expect(item('Mapped')).toBeEnabled()
  const labels=await menu('Mapped').getByRole('menuitem').allTextContents()
  assert.equal(labels[labels.indexOf('Project prompt')+1],'Open terminal')
  await item('Mapped').click()
  await expect.poll(()=>state.opened.length).toBe(1)
  assert.deepEqual(state.opened[0],{projectId:'mapped'})
  await expect(menu('Mapped')).toBeHidden()

  // A refusal is shown as the agent gave it.
  state.refuse='project mapped is disconnected; add it again in the desktop before launching'
  await openMenu('Mapped');await item('Mapped').click()
  await expect(page.locator('#error')).toHaveText(state.refuse)
  state.refuse=''
  assert.equal(state.opened.length,1,'A refused open is not recorded as opened')

  // A project with no local folder shows the item disabled, as Project prompt.
  await page.keyboard.press('Escape')
  await openMenu('remote')
  await expect(item('remote')).toBeDisabled()
  await expect(menu('remote').getByRole('menuitem',{name:'Project prompt',exact:true})).toBeDisabled()
  await page.keyboard.press('Escape')

  // An agent that cannot open it gets no item at all.
  state.capabilities=['task-engines'];await page.reload()
  await openMenu('Mapped')
  await expect(menu('Mapped').getByRole('menuitem',{name:'Project prompt',exact:true})).toBeVisible()
  await expect(item('Mapped')).toHaveCount(0)
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
