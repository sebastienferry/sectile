const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')
const {WebSocketServer}=require('ws')

test('TTY header follows metadata and selection without disturbing the console',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-task-header-'))
 const run=(id,taskId,skill,status='completed')=>({id,taskId,taskKey:taskId==='a'?'#82':'',projectId:'project',skill,status,sessionId:id,directory:'/tmp/example/worktree',createdAt:id==='old'?'2026-09-12T10:00:00Z':'2026-09-13T10:00:00Z'})
 let runs=[run('current','a','implement'),run('old','a','clarify'),run('other','full-task-id','specify','running')]
 let reported=null,resultUnavailable=false
 let tasks=[],pending=[],hold=true,fail=false,requests=0,attachments=0,disconnections=0
 const respond=res=>{if(fail)res.writeHead(503).end();else res.end(JSON.stringify(tasks))}
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Example project',path:'/tmp/example'}]));return}
  if(req.url==='/desktop/project?id=project'){res.end(JSON.stringify({server:{skills:[{id:'implement',name:'Implement'}]}}));return}
  if(req.url.startsWith('/desktop/run-result?')){if(resultUnavailable)res.writeHead(503).end();else res.end(JSON.stringify(reported||{activity:null}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url.startsWith('/desktop/tasks?')){requests++;if(hold)pending.push(res);else respond(res);return}
  res.writeHead(404).end()
 })
 const ws=new WebSocketServer({noServer:true})
 server.on('upgrade',(req,socket,head)=>ws.handleUpgrade(req,socket,head,client=>{
  attachments++;client.on('close',()=>disconnections++)
  client.send(Buffer.from('Preserved console output\r\n'))
 }))
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  let page=await app.firstWindow();page.setDefaultTimeout(7000)
  const header=()=>page.locator('#title')
  const select=async identity=>page.locator('.local-task').filter({has:page.getByRole('button',{name:'Open '+identity+' in Sectile',exact:true})}).locator('.run').click()
  const advance=async()=>{
   const before=requests
   await page.evaluate(()=>{window.headerTestTime=(window.headerTestTime||Date.now())+16000;Date.now=()=>window.headerTestTime})
   await expect.poll(()=>requests).toBeGreaterThan(before)
  }
  const release=()=>{hold=false;for(const res of pending)respond(res);pending=[]}
  await expect(header()).toHaveText('#82 · implement')
  await expect.poll(()=>pending.length).toBeGreaterThan(0)
  const otherBadge=page.locator('.task-skill-status[data-run-id="other"]')
  await page.locator('#save-log').focus()
  await expect.poll(()=>attachments).toBe(1)
  const beforeStatus=[attachments,disconnections]
  reported={activity:{id:'other',taskId:'full-task-id',skillId:'specify',status:'completed'},task:{labels:['specified']}}
  await expect(otherBadge).toHaveText('✓')
  await expect(otherBadge).toHaveAttribute('title','specify · Skill completed')
  await expect(page.locator('.task-skill-status[data-run-id="current"]')).not.toHaveText('✓')
  await expect(header()).toHaveText('#82 · implement')
  await expect(page.locator('#save-log')).toBeFocused()
  assert.deepEqual([attachments,disconnections],beforeStatus)
  reported=null
  await expect(otherBadge).toHaveText('◷')
  await select('full-task-id')
  await expect(header()).toHaveText('full-task-id · specify')
  tasks=[{id:'a',title:'Delayed title',prUrl:'https://github.com/example/repo/pull/82'}]
  release()
  await expect(page.locator('#selected-pr')).toBeHidden()
  await expect(page.locator('.run strong').filter({hasText:'Delayed title'})).toHaveCount(1)
  await expect(header()).toHaveText('full-task-id · specify')
  await select('#82')
  await expect(header()).toHaveText('#82 · Delayed title · implement')
  await expect.poll(()=>attachments).toBeGreaterThan(1)
  await page.waitForTimeout(100)
  const counts=[attachments,disconnections]
  // Read actual terminal output through the existing export action.
  await app.evaluate(({ipcMain})=>{ipcMain.removeHandler('save-log');ipcMain.handle('save-log',(_,text)=>{global.headerTestLog=text})})
  const output=async()=>{await page.locator('#save-log').click();return app.evaluate(()=>global.headerTestLog)}
  assert.match(await output(),/Preserved console output/)
  tasks[0].title='<img src=x onerror=alert(1)> Updated title'
  await advance()
  await expect(header()).toHaveText('#82 · '+tasks[0].title+' · implement')
  assert.equal(await header().locator('*').count(),0)
  assert.match(await header().ariaSnapshot(),/Updated title/)
  fail=true;await advance()
  await expect(header()).toContainText('Updated title')
  assert.deepEqual([attachments,disconnections],counts)
  assert.match(await output(),/Preserved console output/)
  fail=false
  for(const title of ['   ',undefined,'#82']){
   tasks[0].title=title;await advance();await expect(header()).toHaveText('#82 · implement')
  }
  tasks=[];await advance();await expect(header()).toHaveText('#82 · implement')
  hold=true;await advance()
  await expect(header()).toHaveText('#82 · implement')
  const longTitle='A very long task title with detailed context '.repeat(20)
  tasks=[{id:'a',title:longTitle,prUrl:'https://github.com/example/repo/pull/82'}]
  release();await expect(header()).toHaveText('#82 · '+longTitle.trim()+' · implement')
  assert.deepEqual([attachments,disconnections],counts)
  await page.locator('#execution-history').selectOption('old')
  await expect(header()).toHaveText('#82 · '+longTitle.trim()+' · clarify')
  await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].setSize(800,600))
  await expect.poll(()=>page.evaluate(()=>window.innerWidth)).toBe(800)
  await expect(header()).toHaveAttribute('title','#82 · '+longTitle.trim()+' · clarify')
  assert.ok(await header().evaluate(el=>el.scrollWidth>el.clientWidth&&getComputedStyle(el).whiteSpace==='nowrap'))
  const selectors=['#execution-history','#selected-pr','#rerun','#save-log','#stop']
  for(const selector of selectors){
   const control=page.locator(selector);await expect(control).toBeVisible()
   assert.ok(await control.evaluate(el=>{const r=el.getBoundingClientRect();return r.left>=0&&r.right<=innerWidth&&r.bottom<=innerHeight}))
   if(await control.isEnabled()){await control.focus();await expect(control).toBeFocused()}
  }
  await page.locator('#rerun').focus();await page.keyboard.press('Tab');await expect(page.locator('#save-log')).toBeFocused()
  await page.screenshot({path:path.join(root,'task-header-narrow.png')})
  console.log('Header screenshot: '+path.join(root,'task-header-narrow.png'))
  await page.getByRole('button',{name:'Actions for #82',exact:true}).click()
  await page.getByRole('textbox',{name:'Local task name'}).fill('Local title')
  await page.getByRole('button',{name:'Rename locally',exact:true}).click()
  await expect(header()).toHaveText('#82 · Local title · clarify')
  tasks[0].title='Changed tracker title';await advance()
  await expect(header()).toHaveText('#82 · Local title · clarify')
  await app.close();app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  page=await app.firstWindow();page.setDefaultTimeout(7000)
  await expect(header()).toHaveText('#82 · Local title · implement')
  // A failed initial request keeps identity usable without a cached title.
  fail=true;runs=[run('other','full-task-id','specify','running')];await page.reload()
  await expect(header()).toHaveText('full-task-id · specify')
  const consoleCounts=[attachments,disconnections]
  reported={activity:{id:'other',taskId:'full-task-id',skillId:'specify',status:'completed'},task:{labels:['specified']}}
  await expect(page.locator('#skill-result')).toHaveText('✓ Skill completed')
  await page.screenshot({path:path.join(root,'skill-completed.png')})
  console.log('Skill-result screenshot: '+path.join(root,'skill-completed.png'))
  assert.equal(runs[0].status,'running')
  assert.deepEqual([attachments,disconnections],consoleCounts)
  reported.task.labels=['clarified']
  await expect(page.locator('#skill-result')).toContainText('Awaiting stage validation')
  reported.activity.status='failed'
  await expect(page.locator('#skill-result')).toHaveText('! Skill failed')
  await expect(page.locator('.task-skill-status')).toHaveText('!')
  resultUnavailable=true
  await expect(page.locator('#skill-result')).toHaveText('◷ In progress')
  await expect(page.locator('.task-skill-status')).toHaveText('◷')
  runs=[];await page.reload()
  await expect(header()).toHaveText('Select an execution')
  await expect(header()).toHaveAttribute('title','Select an execution')
  await expect(page.locator('#skill-result')).toBeHidden()
 }finally{
  if(app)await app.close()
  for(const res of pending)res.end('[]')
  for(const client of ws.clients)client.terminate()
  await new Promise(resolve=>ws.close(resolve))
  await new Promise(resolve=>server.close(resolve))
 }
})
