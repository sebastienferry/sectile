const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')
const {WebSocketServer}=require('ws')

// An execution that works in several folders lists them behind a chevron
// after its path, each item copying its own path (#762). One folder, or an
// agent that sends no list, keeps the path alone, as before.
test('the folders of an execution are copied from a menu after its path',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-folder-menu-'))
 const worktree='/tmp/example/.tasks/worktrees/issue-762'
 const folders=[
  {path:worktree,name:'sectile',role:'primary'},
  {path:'/tmp/example-docs',name:'docs',role:'context'},
  {path:'/tmp/notes',name:'notes',role:'local',attached:true},
  {path:'/tmp/specs/.tasks/worktrees/issue-762',name:'specifications',role:'spec'}
 ]
 const runs=[
  {id:'several',projectId:'project',taskId:'a',taskKey:'#762',sessionId:'several',skill:'implement',status:'running',directory:worktree,folders,createdAt:'2026-10-06T10:00:00Z'},
  {id:'single',projectId:'project',taskId:'b',taskKey:'#763',sessionId:'single',skill:'implement',status:'running',directory:'/tmp/example/.tasks/worktrees/issue-763',folders:[{path:'/tmp/example/.tasks/worktrees/issue-763',name:'sectile',role:'primary'}],createdAt:'2026-10-06T09:00:00Z'},
  {id:'older',projectId:'project',taskId:'c',taskKey:'#764',sessionId:'older',skill:'implement',status:'running',directory:'/tmp/example/.tasks/worktrees/issue-764',createdAt:'2026-10-06T08:00:00Z'}
 ]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:['task-engines']}));return}
  if(url.pathname==='/desktop/workstation'){res.end(JSON.stringify({defaults:{},effective:{useWorktrees:true,parallelism:1,aiProviderModels:{}},providerModels:{},setupProviders:['claude'],seeded:{}}));return}
  if(url.pathname==='/desktop/engines'){res.end(JSON.stringify({catalogue:[{id:'e',name:'Claude',provider:'claude'}],default:'e',providers:['claude'],providerModels:{},projects:{},taskCounts:{}}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Example project',path:'/tmp/example',configured:true}]));return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{skills:[{id:'implement',name:'Implement'}]}}));return}
  if(url.pathname==='/desktop/run-result'){res.end(JSON.stringify({activity:null}));return}
  if(url.pathname==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(url.pathname==='/desktop/tasks'){res.end(JSON.stringify([{id:'a',key:'#762',title:'Folders',labels:['#specified']},{id:'b',key:'#763',title:'One folder',labels:['#specified']},{id:'c',key:'#764',title:'Older agent',labels:['#specified']}]));return}
  if(url.pathname==='/desktop/version'){res.end('{"version":"test"}');return}
  res.writeHead(404).end('{}')
 })
 const ws=new WebSocketServer({noServer:true})
 server.on('upgrade',(req,socket,head)=>ws.handleUpgrade(req,socket,head,client=>client.send(Buffer.from('Console output\r\n'))))
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}),{mode:0o600})
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  await app.evaluate(({clipboard})=>clipboard.writeText('untouched'))
  const chevron=page.locator('#worktree-folders'),menu=page.locator('#worktree-folders-menu')

  // The most recent execution is selected: four folders, so a chevron.
  await expect(page.locator('#directory')).toHaveText(worktree)
  await expect(chevron).toBeVisible()
  await expect(chevron).toHaveAttribute('aria-label','Folders of this execution')
  assert.ok(await page.evaluate(()=>document.querySelector('#worktree').nextElementSibling===document.querySelector('#worktree-folders')),'The chevron follows the path')
  await expect(menu).toBeHidden()

  // It lists every folder, primary first, with its role and path.
  await chevron.click()
  await expect(menu).toBeVisible()
  await expect(chevron).toHaveAttribute('aria-expanded','true')
  const items=menu.getByRole('menuitem')
  await expect(items).toHaveCount(4)
  await expect(items.nth(0)).toBeFocused()
  await expect(items.nth(0).locator('.folder-name')).toHaveText('sectile')
  await expect(items.nth(0).locator('.folder-role')).toHaveText('primary')
  await expect(items.nth(1).locator('.folder-role')).toHaveText('context')
  await expect(items.nth(2).locator('.folder-role')).toHaveText('attached')
  await expect(items.nth(3).locator('.folder-role')).toHaveText('specifications')
  await expect(items.nth(3).locator('.folder-path')).toHaveText('/tmp/specs/.tasks/worktrees/issue-762')

  // Choosing one copies its path alone and confirms it beside the path.
  await items.nth(1).click()
  await expect(menu).toBeHidden()
  await expect(page.locator('#worktree-copied')).toHaveText('Copied')
  assert.equal(await app.evaluate(({clipboard})=>clipboard.readText()),'/tmp/example-docs')

  // The keyboard reaches each item; Escape closes and returns to the chevron.
  await chevron.focus();await page.keyboard.press('ArrowDown')
  await expect(items.nth(0)).toBeFocused()
  await page.keyboard.press('End');await expect(items.nth(3)).toBeFocused()
  await page.keyboard.press('ArrowDown');await expect(items.nth(0)).toBeFocused()
  await page.keyboard.press('ArrowUp');await expect(items.nth(3)).toBeFocused()
  await page.keyboard.press('Home');await page.keyboard.press('ArrowDown');await expect(items.nth(1)).toBeFocused()
  await page.keyboard.press('ArrowDown');await page.keyboard.press('Enter')
  await expect(menu).toBeHidden()
  assert.equal(await app.evaluate(({clipboard})=>clipboard.readText()),'/tmp/notes')
  await chevron.click();await expect(menu).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(menu).toBeHidden()
  await expect(chevron).toBeFocused()

  // A press outside closes it.
  await chevron.click();await expect(menu).toBeVisible()
  await page.locator('#title').click()
  await expect(menu).toBeHidden()

  // The path still copies itself.
  await page.locator('#worktree').click()
  assert.equal(await app.evaluate(({clipboard})=>clipboard.readText()),worktree)

  // One folder, or an agent that sends no list: the path stands alone.
  await page.locator('.local-task').filter({has:page.getByRole('button',{name:'Open #763 in Sectile',exact:true})}).locator('.run').click()
  await expect(page.locator('#directory')).toHaveText('/tmp/example/.tasks/worktrees/issue-763')
  await expect(chevron).toBeHidden()
  await page.locator('.local-task').filter({has:page.getByRole('button',{name:'Open #764 in Sectile',exact:true})}).locator('.run').click()
  await expect(page.locator('#directory')).toHaveText('/tmp/example/.tasks/worktrees/issue-764')
  await expect(chevron).toBeHidden()
 }finally{
  if(app)await app.close()
  for(const client of ws.clients)client.terminate()
  await new Promise(resolve=>ws.close(resolve))
  await new Promise(resolve=>server.close(resolve))
 }
})
