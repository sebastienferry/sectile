const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron}=require('@playwright/test')
const {WebSocketServer}=require('ws')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('free consoles launch without prompts, reconnect and stop independently',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-free-console-'))
 const runs=[],launches=[],resultReads=[];let failLaunch=true,inputs='',attachments=0,taskWrites=0,taskReads=0
 const server=http.createServer((req,res)=>{
  assert.equal(req.headers.authorization,'Bearer test-secret');res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:['free-console']}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'p',name:'Project',path:'/tmp/repo'}]));return}
  if(req.url==='/desktop/project?id=p'){res.end(JSON.stringify({configured:true,parallelism:2,server:{aiProvider:'claude',skills:[]}}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url.startsWith('/desktop/run-result')){resultReads.push(req.url);res.writeHead(404).end();return}
  if(req.url.startsWith('/desktop/tasks?')){if(req.method==='POST')taskWrites++;else taskReads++;res.end('[]');return}
  if(req.url==='/desktop/consoles'){
   let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
    if(failLaunch){res.writeHead(409);res.end('Project unavailable');return}
    const input=JSON.parse(raw);launches.push(input);const id='console-'+launches.length
    const run={id,kind:'console',provider:input.provider,projectId:input.projectId,taskId:'',skill:'',directory:'/tmp/repo',sessionId:id,status:'running'}
    runs.push(run);res.writeHead(202);res.end(JSON.stringify(run))
   });return
  }
  if(req.url.startsWith('/desktop/stop?id=')){runs.find(run=>run.id===new URL(req.url,'http://local').searchParams.get('id')).status='canceled';res.writeHead(204).end();return}
  res.writeHead(404).end()
 })
 const ws=new WebSocketServer({noServer:true})
 server.on('upgrade',(req,socket,head)=>ws.handleUpgrade(req,socket,head,client=>{
  attachments++;client.send(Buffer.from('Interactive agent ready\r\n'))
  client.on('message',raw=>{const message=JSON.parse(raw);if(message.type==='input')inputs+=message.data})
 }))
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,TASKFLOW_DESKTOP_DATA_DIR:root,TASKFLOW_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 const open=async()=>{app=await electron.launch({args:[path.resolve(__dirname,'..')],env});const page=await app.firstWindow();page.setDefaultTimeout(7000);return page}
 try{
  let page=await open()
  await page.getByRole('button',{name:'Open agent console in Project',exact:true}).click()
  const provider=page.getByLabel('Console agent')
  await page.waitForFunction(()=>document.querySelector('[aria-label="Console agent"]').value==='claude')
  assert.equal(await page.locator('dialog textarea').count(),0)
  await page.getByRole('button',{name:'Open console',exact:true}).click()
  await page.getByText('Project unavailable',{exact:false}).waitFor()
  assert.equal(await page.getByRole('button',{name:'Open console',exact:true}).isEnabled(),true)
  failLaunch=false;await provider.selectOption('codex');await page.getByRole('button',{name:'Open console',exact:true}).click()
  await page.waitForFunction(()=>document.querySelector('#title').textContent==='Codex console')
  assert.deepEqual(launches,[{projectId:'p',provider:'codex'}])
  assert.equal(await page.locator('#next-step').isHidden(),true);assert.equal(await page.locator('.task-number').count(),0)
  assert.equal(await page.locator('#selected-pr').isHidden(),true)
  await page.locator('.xterm-helper-textarea').pressSequentially('hello agent');await page.locator('.xterm-helper-textarea').press('Enter')
  await page.waitForTimeout(150);assert.match(inputs,/hello agent\r/)
  await page.getByRole('button',{name:'Open agent console in Project',exact:true}).click();await page.getByRole('button',{name:'Open console',exact:true}).click()
  await page.waitForFunction(()=>document.querySelectorAll('.local-task').length===2)
  assert.equal(launches[1].provider,'claude');assert.equal(await page.locator('#execution-history').isHidden(),true)
  assert.equal(resultReads.length,0);assert.equal(taskWrites,0);assert.equal(taskReads,0)
  const before=attachments
  await app.close();app=null;page=await open()
  await page.locator('.local-task .run').first().waitFor();assert.equal(await page.locator('.local-task').count(),2)
  await page.locator('.local-task .run').first().click();await page.getByRole('button',{name:'Stop execution',exact:true}).click()
  await page.waitForFunction(()=>document.querySelector('#skill-result').textContent.includes('Console stopped'))
  assert.ok(attachments>before);assert.equal(runs[0].status,'canceled');assert.equal(runs[1].status,'running')
  await page.getByRole('button',{name:'Relaunch',exact:true}).click()
  await page.waitForFunction(()=>document.querySelector('[aria-label="Console agent"]').value==='codex')
  await page.getByRole('button',{name:'Open console',exact:true}).click()
  await page.waitForFunction(()=>document.querySelectorAll('.local-task').length===3)
  assert.deepEqual(launches[2],{projectId:'p',provider:'codex'});assert.equal(resultReads.length,0);assert.equal(taskWrites,0);assert.equal(taskReads,0)
  await page.screenshot({path:path.join(root,'free-console.png')});console.log('Free-console screenshot: '+path.join(root,'free-console.png'))
 }finally{
  if(app)await app.close()
  for(const client of ws.clients)client.terminate()
  await new Promise(resolve=>ws.close(resolve));await new Promise(resolve=>server.close(resolve))
 }
})
