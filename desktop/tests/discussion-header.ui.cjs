const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')
const {WebSocketServer}=require('ws')

test('the discussion header carries identity, state, a copyable worktree and icon controls',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-discussion-header-'))
 const worktree='/tmp/example/worktree-42'
 const otherWorktree='/tmp/example/worktree-7'
 let runs=[
  {id:'current',projectId:'project',taskId:'a',taskKey:'#82',sessionId:'current',skill:'implement',status:'running',directory:worktree,createdAt:'2026-09-13T10:00:00Z'},
  {id:'other',projectId:'project',taskId:'b',taskKey:'#7',sessionId:'other',skill:'clarify',status:'completed',directory:otherWorktree,createdAt:'2026-09-13T09:00:00Z'},
 ]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Example project',path:'/tmp/example',configured:true}]));return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{skills:[{id:'implement',name:'Implement'}]}}));return}
  if(url.pathname==='/desktop/run-result'){res.end(JSON.stringify({activity:null}));return}
  if(url.pathname==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(url.pathname==='/desktop/tasks'){res.end(JSON.stringify([{id:'a',key:'#82',title:'Rework the discussion header',labels:['#specified'],prUrl:'https://github.com/example/repo/pull/82'},{id:'b',key:'#7',title:'Another task',labels:['#new']}]));return}
  res.writeHead(404).end()
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

  // Identity and state share one line; the state is the shared vocabulary.
  await expect(page.locator('#title')).toHaveText('#82 · Rework the discussion header · implement')
  await expect(page.locator('#run-state')).toHaveText('Running')
  await expect(page.locator('#run-state')).toHaveAttribute('title','Process: Running')
  assert.equal(await page.locator('#run-state svg').count(),1,'The state keeps the shared glyph beside its label')
  // The state comes first, in the header as in the sidebar row and the tickets pane.
  assert.ok(await page.evaluate(()=>document.querySelector('#run-state').compareDocumentPosition(document.querySelector('#title'))&Node.DOCUMENT_POSITION_FOLLOWING),'The header state precedes the title')
  const row=page.locator('.local-task .run').first()
  assert.ok(await row.evaluate(el=>el.querySelector('.run-state').compareDocumentPosition(el.querySelector('strong'))&Node.DOCUMENT_POSITION_FOLLOWING),'The row state precedes the title')
  await expect(row.locator('.run-state')).toHaveAttribute('title',/^Process: /)

  // The worktree is a control: it says what it holds and what a click does.
  const worktreeButton=page.getByRole('button',{name:worktree,exact:true})
  await expect(worktreeButton).toBeVisible()
  await expect(worktreeButton).toHaveAttribute('title','Copy this path')
  assert.equal(await worktreeButton.evaluate(el=>getComputedStyle(el).userSelect),'text','The path stays selectable for a manual copy')
  await worktreeButton.click()
  await expect(page.locator('#worktree-copied')).toHaveText('Copied')
  assert.equal(await app.evaluate(({clipboard})=>clipboard.readText()),worktree)
  // Keyboard focus, so :focus-visible applies as it would for a keyboard user.
  await worktreeButton.focus();await page.keyboard.press('Shift+Tab');await page.keyboard.press('Tab')
  await expect(worktreeButton).toBeFocused()
  assert.equal(await worktreeButton.evaluate(el=>getComputedStyle(el).outlineStyle),'solid','The path keeps a visible focus outline')

  // The utility controls are icons, and keep their wording for hover and
  // assistive technology. The workflow action keeps its label.
  for(const [selector,name] of [['#save-log','Export log'],['#rerun','Relaunch'],['#view-console','Console'],['#view-changes','Changes'],['#selected-pr','Open PR #82 — State unknown']]){
   const control=page.locator(selector)
   await expect(control).toHaveAttribute('title',/.+/)
   assert.equal(await control.getAttribute('aria-label'),name)
   assert.equal(await control.locator('svg').count(),1,selector+' draws a glyph')
  }
  assert.equal((await page.locator('#save-log').textContent()).trim(),'','Export log is icon-only')
  assert.equal((await page.locator('#view-changes').textContent()).trim(),'','The view switch is icon-only')
  // The pull request keeps its number: it identifies which PR, not just that one exists.
  assert.equal((await page.locator('#selected-pr').textContent()).trim(),'PR #82')

  // The confirmation belongs to the path it was given for: another selection
  // must not inherit it.
  await worktreeButton.click()
  await expect(page.locator('#worktree-copied')).toHaveText('Copied')
  await page.locator('.local-task').filter({has:page.getByRole('button',{name:'Open #7 in Sectile',exact:true})}).locator('.run').click()
  await expect(page.getByRole('button',{name:otherWorktree,exact:true})).toBeVisible()
  // Read without polling: the path and its confirmation are written together,
  // so the new path being on screen already settles the confirmation.
  assert.equal(await page.locator('#worktree-copied').textContent(),'','Another selection does not inherit the confirmation')
  await expect(page.locator('#run-state')).toHaveText('Finished')
  await page.locator('.local-task').filter({has:page.getByRole('button',{name:'Open #82 in Sectile',exact:true})}).locator('.run').click()
  await expect(worktreeButton).toBeVisible()

  // Every control stays inside a narrow window and reachable from the keyboard.
  await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].setSize(800,600))
  await expect.poll(()=>page.evaluate(()=>window.innerWidth)).toBe(800)
  for(const selector of ['#worktree','#execution-history','#view-console','#view-changes','#selected-pr','#rerun','#save-log','#stop']){
   const control=page.locator(selector)
   if(await control.isHidden())continue
   assert.ok(await control.evaluate(el=>{const r=el.getBoundingClientRect();return r.left>=0&&r.right<=innerWidth&&r.bottom<=innerHeight}),selector+' stays inside the window')
   if(await control.isEnabled()){await control.focus();await expect(control).toBeFocused()}
  }
  await page.screenshot({path:path.join(root,'discussion-header.png')})
  console.log('Discussion header screenshot: '+path.join(root,'discussion-header.png'))

  // Without a selected execution the header offers neither state nor path.
  runs=[];await page.reload()
  await expect(page.locator('#title')).toHaveText('Select an execution')
  await expect(page.locator('#run-state')).toBeHidden()
  await expect(page.locator('#worktree')).toBeHidden()
  await expect(page.locator('#worktree-copied')).toHaveText('')
 }finally{
  if(app)await app.close()
  for(const client of ws.clients)client.terminate()
  await new Promise(resolve=>ws.close(resolve))
  await new Promise(resolve=>server.close(resolve))
 }
})
