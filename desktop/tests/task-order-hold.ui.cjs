const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('sidebar holds its order while the pointer or focus stays on the task list',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-task-hold-'))
 const run=(id,taskId,status,hour)=>({id,taskId,taskKey:'#'+taskId,projectId:'project-a',skill:'clarify',status,createdAt:`2026-09-13T${hour}:00:00Z`})
 // #7 is running and leads; #8 is queued and trails.
 let runs=[run('lead','7','running','10'),run('trailing','8','queued','09')],tasks=[]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Alpha',path:'/tmp/a'}]));return}
  if(req.url==='/desktop/project?id=project-a'){res.end(JSON.stringify({parallelism:3}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end(JSON.stringify(tasks));return}
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  const order=()=>page.locator('.task-number').allTextContents()
  await page.waitForFunction(()=>document.querySelectorAll('.local-task').length===2)
  assert.deepEqual(await order(),['#7','#8'])

  const lead=page.locator('.local-task').filter({has:page.getByRole('button',{name:'Open #7 in Sectile',exact:true})}).locator('.run')
  await lead.hover()
  const before=await lead.boundingBox()
  await expect(page.locator('#orchestration [data-run-id]')).toHaveCount(0)
  // The leading execution finishes: it would drop below the queued task, but
  // the pointer is on the row so the sequence must not change.
  runs=runs.map(item=>item.id==='lead'?{...item,status:'completed'}:item)
  runs.push(run('history','7','completed','11'))
  tasks=[{id:'7',key:'#7',title:'Updated while the sidebar is held',labels:['#clarified'],prUrl:'https://github.com/example/repo/pull/7'}]
  await page.evaluate(()=>{const now=Date.now;Date.now=()=>now()+16000})
  await expect(lead).toHaveAttribute('data-status','completed')
  await expect(lead.locator('strong')).toHaveText('Updated while the sidebar is held')
  await expect(lead.locator('..').locator('.pr-indicator')).toBeVisible()
  await expect(page.locator('#selected-pr')).toContainText('PR #7')
  await expect(page.locator('#execution-history option')).toHaveCount(2)
  assert.deepEqual(await order(),['#7','#8'])
  assert.equal((await lead.boundingBox()).y,before.y,'Orchestration changes must not move the hovered task')
  await expect(page.locator('#orchestration [data-run-id]')).toHaveCount(0)

  // Keyboard focus alone keeps holding the order once the pointer has left.
  await lead.focus()
  await page.locator('#terminal').hover()
  assert.deepEqual(await order(),['#7','#8'])

  await page.locator('#task-actions-more').focus()
  await page.waitForFunction(()=>document.querySelector('.task-number')?.textContent==='#8')
  assert.deepEqual(await order(),['#8','#7'])
  await expect(page.locator('#orchestration [data-run-id]')).toHaveCount(0)
  assert.equal(await page.locator('.run.selected').count(),1)
  await page.screenshot({path:path.join(root,'task-hold.png')})
  console.log('Task-hold screenshot: '+path.join(root,'task-hold.png'))
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
