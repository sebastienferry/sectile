const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('declare code as reviewed transitions task from ticket row, handles capability check and errors, and proposes handoff',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-declare-reviewed-'))
 let stage='implemented',transitions=[],capabilities=['transition-stage'],activeRuns=[]
 const tasks=[
  {id:'task-1',key:'#1',title:'Implemented Task',status:'to_test',labels:['#implemented'],prUrl:'https://github.com/org/repo/pull/10'},
  {id:'task-2',key:'#2',title:'Specified Task',status:'to_implement',labels:['#specified']}
 ]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Project A',path:'/tmp/project'}]));return}
  if(req.url==='/desktop/project?id=project-a'){res.end(JSON.stringify({configured:true,server:{skills:['clarify','specify','implement','adjust','handoff'].map(id=>({id}))}}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(activeRuns));return}
  if(req.url.startsWith('/desktop/tasks?')){
   const currentTasks=tasks.map(t=>t.id==='task-1'?{...t,labels:['#'+stage],status:stage==='reviewed'?'to_close':t.status}:t)
   res.end(JSON.stringify(currentTasks))
   return
  }
  if(req.url.startsWith('/desktop/tasks/transition?')){
   let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
    const data=JSON.parse(raw)
    transitions.push(data)
    stage=data.stage
    res.end(JSON.stringify({success:true,stage:data.stage}))
   })
   return
  }
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)

  // Open tickets pane for Project A
  const openTasksBtn=page.getByRole('button',{name:'Open tasks in Project A',exact:true})
  await openTasksBtn.waitFor()
  await openTasksBtn.click()
  await page.waitForSelector('.ticket-row')

  const row1=page.locator('.ticket-row[data-task-id="task-1"]')
  const row2=page.locator('.ticket-row[data-task-id="task-2"]')

  // Row 2 is specified: its menu should not have "Declare code as reviewed…"
  await row2.locator('.ticket-more').click()
  assert.equal(await row2.locator('.ticket-menu').getByRole('menuitem',{name:'Declare code as reviewed…'}).count(),0)
  await page.keyboard.press('Escape')

  // Row 1 is implemented: its menu has "Declare code as reviewed…"
  await row1.locator('.ticket-more').click()
  const declareBtn=row1.locator('.ticket-menu').getByRole('menuitem',{name:'Declare code as reviewed…'})
  assert.equal(await declareBtn.count(),1)
  assert.equal(await declareBtn.isEnabled(),true)

  // Test active execution disables the menu item and offers Detach to native terminal
  activeRuns=[{id:'run-act',taskId:'task-1',projectId:'project-a',skill:'adjust',status:'running'}]
  // Wait for poll to refresh row
  await page.waitForFunction(()=>document.querySelector('.ticket-row[data-task-id="task-1"] .ticket-run')?.disabled)
  assert.equal(await declareBtn.isDisabled(),true,'Active run disables Declare code as reviewed')
  await page.keyboard.press('Escape')
  await row1.locator('.ticket-more').click()
  const detachTicketBtn=row1.locator('.ticket-menu').getByRole('menuitem',{name:'Detach to native terminal'})
  assert.equal(await detachTicketBtn.count(),1)
  assert.equal(await detachTicketBtn.isVisible(),true)
  await page.keyboard.press('Escape')
  activeRuns=[]
  await page.waitForFunction(()=>document.querySelector('.ticket-row[data-task-id="task-1"] .ticket-run')?.disabled===false)
  await row1.locator('.ticket-more').click()
  assert.equal(await row1.locator('.ticket-menu').getByRole('menuitem',{name:'Detach to native terminal'}).count(),0)
  assert.equal(await declareBtn.isEnabled(),true)

  // Test Escape dismisses dialog without transitioning
  await declareBtn.click()
  await page.waitForSelector('#project-dialog[open]')
  assert.match(await page.locator('#dialog-body h2').textContent(),/Declare #1 as reviewed\?/)
  assert.match(await page.locator('#dialog-body p').first().textContent(),/This transitions the task to #reviewed and proposes Handoff after human merge\./)
  await page.keyboard.press('Escape')
  await page.waitForFunction(()=>document.querySelector('#project-dialog')?.open===false)
  assert.equal(transitions.length,0,'Escape closes dialog without transition')

  // Test outdated agent capability error
  capabilities=[]
  await row1.locator('.ticket-more').click()
  await row1.locator('.ticket-menu').getByRole('menuitem',{name:'Declare code as reviewed…'}).click()
  await page.waitForSelector('#project-dialog[open]')
  const confirmBtn=page.locator('#dialog-body button')
  await confirmBtn.click()
  await page.waitForFunction(()=>document.querySelector('#dialog-body [role=status]')?.textContent.includes('does not support stage transitions'))
  assert.equal(await confirmBtn.isEnabled(),true,'Confirm re-enabled on capability failure')
  assert.equal(transitions.length,0)

  // Test successful transition with capability present
  capabilities=['transition-stage']
  await confirmBtn.click()
  await page.waitForFunction(()=>document.querySelector('#project-dialog')?.open===false)
  assert.equal(transitions.length,1)
  assert.equal(transitions[0].stage,'reviewed')

  // The ticket row refreshed: stage is reviewed, run action is Run: Handoff
  await page.waitForFunction(()=>document.querySelector('.ticket-row[data-task-id="task-1"] .ticket-stage')?.textContent.includes('reviewed'))
  const runBtn=row1.locator('.ticket-run')
  assert.equal(await runBtn.textContent(),'Run: Handoff')

  // The menu on row 1 no longer shows "Declare code as reviewed…"
  await row1.locator('.ticket-more').click()
  assert.equal(await row1.locator('.ticket-menu').getByRole('menuitem',{name:'Declare code as reviewed…'}).count(),0)
 }finally{
  if(app)await app.close()
  server.close();fs.rmSync(root,{recursive:true,force:true})
 }
})
