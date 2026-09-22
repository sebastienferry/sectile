const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// An autonomous run has no PTY, so the console pane reads what the agent
// buffered for it. What is checked here is the pane, not the transport: output
// shown on selection, a later chunk appended without reprinting the first, a
// finished run showing its tail, a run that printed nothing saying so, and a
// queued run keeping its own notice.
test('the console pane shows a headless run live, read-only',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-headless-console-'))
 const transcripts={'run-live':'first chunk\n','run-done':'the tail before the exit\n','run-empty':''}
 const runs=[
  {id:'run-live',taskId:'task-1',taskKey:'#48',projectId:'project',skill:'implement',status:'running',headless:true,directory:'/tmp/example/worktree'},
  {id:'run-done',taskId:'task-2',taskKey:'#49',projectId:'project',skill:'implement',status:'completed',headless:true,directory:'/tmp/example/worktree'},
  {id:'run-empty',taskId:'task-3',taskKey:'#50',projectId:'project',skill:'implement',status:'running',headless:true,directory:'/tmp/example/worktree'},
  {id:'run-queued',taskId:'task-4',taskKey:'#51',projectId:'project',skill:'implement',status:'queued',headless:true,directory:'/tmp/example/worktree'},
 ]
 let attachments=0
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Example project',path:'/tmp/example'}]));return}
  if(req.url==='/desktop/project?id=project'){res.end(JSON.stringify({server:{skills:[{id:'implement',name:'Implement'}]}}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url.startsWith('/desktop/run-output?')){
   const query=new URL(req.url,'http://local').searchParams
   const id=query.get('id'),offset=Number(query.get('offset')||0)
   const text=transcripts[id]??''
   res.end(JSON.stringify({output:text.slice(offset),offset:text.length,status:runs.find(run=>run.id===id).status,truncated:false,headless:true}))
   return
  }
  if(req.url.startsWith('/desktop/run-result?')){res.writeHead(404).end('Run not found\n');return}
  if(req.url.startsWith('/desktop/tasks?')){res.end('[]');return}
  res.writeHead(404).end()
 })
 server.on('upgrade',(req,socket)=>{attachments++;socket.destroy()})
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 const pane=page=>page.locator('.xterm-screen').innerText()
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  await page.locator('.run[data-run-id="run-live"]').click()
  await expect.poll(()=>pane(page)).toContain('read-only, no terminal to answer')
  await expect.poll(()=>pane(page)).toContain('first chunk')

  // A later chunk is appended; what is already on screen is not written twice.
  transcripts['run-live']+='second chunk\n'
  await expect.poll(()=>pane(page),{timeout:10000}).toContain('second chunk')
  assert.equal((await pane(page)).split('first chunk').length-1,1,'the first chunk was reprinted')

  await page.locator('.run[data-run-id="run-empty"]').click()
  await expect.poll(()=>pane(page)).toContain('Nothing printed yet.')
  assert.equal((await pane(page)).includes('first chunk'),false,'the pane kept the previous run on screen')

  await page.locator('.run[data-run-id="run-done"]').click()
  await expect.poll(()=>pane(page)).toContain('the tail before the exit')

  // A queued run is not a headless run with output, and keeps its own notice.
  await page.locator('.run[data-run-id="run-queued"]').click()
  await expect.poll(()=>pane(page)).toContain('Waiting for a console')

  // Read-only: no PTY is ever opened for any of them.
  assert.equal(attachments,0,'a headless run must not open a console WebSocket')
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
