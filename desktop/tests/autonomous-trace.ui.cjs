const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron}=require('@playwright/test')
const {WebSocketServer}=require('ws')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// An autonomous run reports what it is doing, and the console pane shows it.
//
// The whole point is the pane: the run has no terminal, and until now selecting
// it wrote one sentence saying so for as long as the run lasted. Against a stub
// agent serving a trace, the real application must attach to it, show the prose
// and the tool calls, and keep showing the notice for a run that has none.

test('the console pane shows an autonomous run working, and keeps the notice without a trace',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-trace-'))
 let traced=true,attachments=0,sockets=[],received=''
 const run=()=>({
  id:'run-trace',taskId:'task-1',taskKey:'#261',projectId:'p',skill:'implement',
  directory:'/tmp/worktree',sessionId:'',status:'running',headless:true,...(traced?{trace:true}:{}),
 })
 const server=http.createServer((req,res)=>{
  assert.equal(req.headers.authorization,'Bearer test-secret')
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:[]}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'p',name:'Example project',path:'/tmp/repo'}]));return}
  if(req.url==='/desktop/project?id=p'){res.end(JSON.stringify({configured:true,parallelism:2,path:'/tmp/repo',server:{aiProvider:'claude',skills:[]}}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify([run()]));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end('[]');return}
  if(req.url.startsWith('/desktop/run-result')){res.writeHead(404).end();return}
  res.writeHead(404).end()
 })
 // The agent serves the trace on the route a console is attached to, so the
 // desktop opens the connection it already knew how to open.
 const ws=new WebSocketServer({noServer:true})
 server.on('upgrade',(req,socket,head)=>{
  assert.ok(req.url.startsWith('/desktop/terminal?id=run-trace'),'unexpected attach: '+req.url)
  attachments++
  ws.handleUpgrade(req,socket,head,client=>{
   sockets.push(client)
   client.send(Buffer.from('\x1b[2mAutonomous execution · read-only trace\x1b[0m\r\n'))
   client.send(Buffer.from('Reading the specification.\r\n'))
   client.send(Buffer.from('\x1b[2m▸ Bash · go test ./internal/...\x1b[0m\r\n'))
   client.on('message',data=>{received+=String(data)})
  })
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}),{mode:0o600})
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'}
 delete env.ELECTRON_RUN_AS_NODE
 let application
 const paneText=page=>page.evaluate(()=>document.querySelector('.xterm-rows')?.textContent||'')
 try{
  application=await electron.launch({executablePath:process.env.SECTILE_DESKTOP_EXECUTABLE,args:process.env.SECTILE_DESKTOP_EXECUTABLE?[]:[path.resolve(__dirname,'..')],env})
  const page=await application.firstWindow()
  page.setDefaultTimeout(10000)
  await page.locator('.run[data-status=running]').click()
  await page.waitForFunction(()=>(document.querySelector('.xterm-rows')?.textContent||'').includes('Reading the specification.'))
  const shown=await paneText(page)
  assert.ok(shown.includes('▸ Bash · go test ./internal/...'),'the tool call is not shown: '+shown)
  assert.ok(!shown.includes('no terminal to answer'),'a traced run was answered with the notice')
  assert.ok(attachments>0,'the traced run was never attached to')

  // Nobody is answering this run: the pane is not given the focus, and what a
  // watcher sends is not a way in — the agent discards it.
  assert.equal(await page.evaluate(()=>document.activeElement?.classList.contains('xterm-helper-textarea')||false),false)
  await page.screenshot({path:path.join(root,'autonomous-trace.png')})
  console.log('Screenshot:',path.join(root,'autonomous-trace.png'))

  // An agent that reports no trace — an older one, or an engine whose stream is
  // not read — keeps the notice it always showed, and nothing is attached.
  traced=false
  for(const client of sockets)client.close()
  // The row carries the run it was rendered with, and the sidebar holds its
  // rendering while the pointer is over it, so the cursor leaves and the next
  // poll is awaited before the row is clicked again.
  await page.mouse.move(700,600)
  await page.waitForTimeout(3500)
  const attached=attachments
  await page.locator('.run[data-status=running]').click()
  // The notice wraps across terminal rows, so the pane is read by its first
  // words rather than by the whole sentence.
  await page.waitForFunction(()=>(document.querySelector('.xterm-rows')?.textContent||'').includes('Autonomous execution:'))
  const notice=await paneText(page)
  assert.ok(!notice.includes('Reading the specification.'),'the pane still shows a trace it is no longer attached to')
  assert.equal(attachments,attached,'a run with no trace must not open a connection')
 }finally{
  if(application)await application.close()
  for(const client of ws.clients)client.terminate()
  ws.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
