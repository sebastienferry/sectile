const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The agent answers 404 for a run it no longer holds (history cleared, agent
// restarted). The desktop must treat that as "no result", not as an error the
// main process logs with a stack, and must stop asking once the run is cleared.
test('a run the agent no longer holds yields no result and no logged error',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-run-result-missing-'))
 const run=(id,status)=>({id,taskId:'task-'+id,projectId:'project',skill:'implement',status,sessionId:id,directory:'/tmp/example/worktree',createdAt:'2026-09-13T10:00:00Z'})
 let runs=[run('gone','completed'),run('live','running')]
 const resultReads=[];let cleared=false
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Example project',path:'/tmp/example'}]));return}
  if(req.url==='/desktop/project?id=project'){res.end(JSON.stringify({server:{skills:[{id:'implement',name:'Implement'}]}}));return}
  if(req.url.startsWith('/desktop/run-result?')){resultReads.push({id:new URL(req.url,'http://local').searchParams.get('id'),cleared});res.writeHead(404).end('Run not found\n');return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url==='/desktop/history'&&req.method==='DELETE'){cleared=true;runs=runs.filter(item=>item.id!=='gone');res.end(JSON.stringify({removed:['gone']}));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end('[]');return}
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app,mainOutput='';const warnings=[]
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  app.process().stderr.on('data',chunk=>mainOutput+=chunk)
  app.process().stdout.on('data',chunk=>mainOutput+=chunk)
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  page.on('console',message=>{if(message.type()==='warning')warnings.push(message.text())})
  const gone=page.locator('.task-skill-status[data-run-id="gone"]'),live=page.locator('.task-skill-status[data-run-id="live"]')
  // The finished run reports the unconfirmed skill; the running one has nothing
  // to report beyond its run state, and is polled all the same.
  await expect(gone).toHaveText('◷')
  await expect(live).toHaveText('')
  await expect.poll(()=>resultReads.filter(read=>read.id==='gone').length).toBeGreaterThan(0)
  await expect.poll(()=>resultReads.filter(read=>read.id==='live').length).toBeGreaterThan(0)
  // The finished run may legitimately be gone; the live one is the anomaly.
  await expect.poll(()=>warnings.some(text=>text.includes('no run live')&&text.includes('running'))).toBe(true)
  assert.equal(warnings.some(text=>text.includes('no run gone')),false)
  await page.getByRole('button',{name:'Clear finished consoles',exact:true}).click()
  await expect(gone).toHaveCount(0)
  await expect.poll(()=>resultReads.filter(read=>read.cleared&&read.id==='live').length).toBeGreaterThan(0)
  assert.equal(resultReads.filter(read=>read.cleared&&read.id==='gone').length,0)
  assert.doesNotMatch(mainOutput,/Error occurred in handler for 'run-result'/)
  assert.doesNotMatch(mainOutput,/Run not found/)
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
