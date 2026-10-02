const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// A new execution of the ticket on display takes the console over, whatever
// launched it and whatever the console showed (#639); nothing else does.
test('the console follows a new execution of the ticket on display',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-follow-execution-'))
 const run=(id,taskId,skill,status,hour)=>({id,taskId,taskKey:'#'+taskId,projectId:'project',skill,status,sessionId:status==='queued'?'':id,directory:'/tmp/example/worktree',createdAt:`2026-09-29T${hour}:00:00Z`})
 let runs=[run('a','1','clarify','completed','09'),run('x','2','specify','completed','08')]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Example project',path:'/tmp/example'}]));return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{defaultSkillMode:'interactive',skills:[{id:'clarify'}]}}));return}
  if(url.pathname==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(url.pathname==='/desktop/tasks'){res.end('[]');return}
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  const task=key=>page.locator('.local-task').filter({has:page.getByRole('button',{name:`Open ${key} in Sectile`,exact:true})}).locator('.run')
  const history=page.locator('#execution-history'),title=page.locator('#title'),terminal=page.locator('#terminal')
  const add=item=>{runs=[...runs,item]}
  // The pointer leaves the sidebar so that its redraws are not held.
  const away=()=>page.locator('#terminal').hover()

  await task('#1').click();await away()
  await expect(title).toContainText('clarify')

  // A finished execution on display: the new running one takes over.
  add(run('b','1','specify','running','10'))
  await expect(history).toHaveValue('b')
  await expect(title).toContainText('specify')

  // An older execution picked from the history is left for a new queued one,
  // which shows its notice and attaches once it starts.
  await history.selectOption('a')
  await expect(title).toContainText('clarify')
  add(run('c','1','implement','queued','11'))
  await expect(history).toHaveValue('c')
  await expect(terminal).toContainText('Execution queued. Waiting for a console.')
  runs=runs.map(item=>item.id==='c'?{...item,status:'running',sessionId:'c'}:item)
  await expect(task('#1')).toHaveAttribute('data-run-id','c')
  await expect(terminal).not.toContainText('Execution queued.')

  // Another ticket's new execution only updates that ticket's row.
  add(run('y','2','implement','running','12'))
  await expect(task('#2')).toHaveAttribute('data-status','running')
  assert.equal(await history.inputValue(),'c')

  // The switch leaves the Tickets pane open.
  // The menu opens on Open tasks; Enter runs it without waiting on a pointer.
  await page.getByRole('button',{name:'Actions for Example project',exact:true}).click()
  await expect(page.getByRole('menuitem',{name:'Open tasks',exact:true})).toBeFocused()
  await page.keyboard.press('Enter')
  const pane=page.locator('#tickets-pane')
  await expect(pane).toBeVisible()
  add(run('d','1','review','running','13'))
  await expect(history).toHaveValue('d')
  await expect(pane).toBeVisible()
  await page.getByRole('button',{name:'Close tickets',exact:true}).click()

  // A free console on display stays when the ticket gets a new execution.
  add({id:'free',kind:'console',provider:'claude',projectId:'project',taskId:'',skill:'',directory:'/tmp/example',sessionId:'free',status:'running',createdAt:'2026-09-29T14:00:00Z'})
  const free=page.locator('.run[data-run-id="free"]')
  await free.click();await away()
  await expect(free).toHaveClass(/selected/)
  add(run('e','1','handoff','running','15'))
  await expect(task('#1')).toHaveAttribute('data-run-id','e')
  await expect(free).toHaveClass(/selected/)
  await expect(task('#1')).not.toHaveClass(/selected/)
  await page.screenshot({path:path.join(root,'follow-new-execution.png')})
  console.log('Follow-new-execution screenshot: '+path.join(root,'follow-new-execution.png'))
 }finally{
  await app?.close()
  server.close()
 }
})
