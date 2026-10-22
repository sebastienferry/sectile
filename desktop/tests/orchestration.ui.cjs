const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const {WebSocketServer}=require('ws')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('orchestration shows live agents, authoritative capacity, workflow stages and console navigation',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-orchestration-'))
 let online=true,capacity=3,discoveryAvailable=true,metadata=true,controlsAvailable=true,saveFailure=false,evaluateFailure=false
 let settings={orchestratorEnabled:false,orchestratorMaxConcurrent:1,orchestratorPrompt:''},evaluations=0,saves=0
 const attachments=[]
 let runs=[{id:'run-a',taskId:'task-a',taskKey:'DEMO-1',projectId:'demo',skill:'clarify',engineName:'Implementation agent',provider:'claude',model:'sonnet',status:'running',headless:true,trace:true,createdAt:'2026-10-22T09:00:00Z'},
 {id:'run-b',taskId:'task-b',taskKey:'DEMO-2',projectId:'demo',skill:'specify',provider:'codex',status:'running',waitingSince:'2026-10-22T09:01:00Z',headless:true,trace:true,createdAt:'2026-10-22T09:00:00Z'},
 {id:'run-c',taskId:'task-c',taskKey:'DEMO-3',projectId:'demo',skill:'implement',provider:'claude',status:'queued',createdAt:'2026-10-22T09:02:00Z'}]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(!online){res.writeHead(503).end();return}
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:controlsAvailable?['orchestration-controls']:[]}));return}
  if(req.url==='/desktop/projects'&&!discoveryAvailable){res.writeHead(503).end();return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'demo',name:'Demo',path:'/tmp/demo',...(metadata?{orchestratorEnabled:true,orchestratorMaxConcurrent:4,orchestratorActiveRuns:capacity}:{})},{id:'idle',name:'Idle',path:'/tmp/idle',...settings,orchestratorActiveRuns:0}]));return}
  if(req.url.startsWith('/desktop/orchestration?')){
   if(req.method==='GET'){res.end(JSON.stringify(settings));return}
   if(req.method==='PUT'){
    let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{saves++;if(saveFailure){res.writeHead(503).end(JSON.stringify({error:'Saving is temporarily unavailable'}));return}settings=JSON.parse(raw);res.end(JSON.stringify(settings))});return
   }
   evaluations++;if(evaluateFailure){res.writeHead(503).end(JSON.stringify({error:'Evaluation is temporarily unavailable'}));return}
   setTimeout(()=>res.end(JSON.stringify({projectId:'idle',activeRunsAfter:1,maxConcurrent:settings.orchestratorMaxConcurrent,launched:[{key:'IDLE-1'}],skipped:[{key:'IDLE-2',reason:'task carries blocker label blocked'}]})),500);return
  }
  if(req.url.startsWith('/desktop/project?')){res.end(JSON.stringify({parallelism:4}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end(JSON.stringify([{id:'task-a',title:'Validate requests',labels:['#new']},{id:'task-b',title:'Define schema',labels:['#clarified']},{id:'task-c',title:'Implement API',labels:['#specified']}]));return}
  res.writeHead(404).end()
 })
 const sockets=new WebSocketServer({server})
 sockets.on('connection',(socket,req)=>{attachments.push(new URL(req.url,'http://localhost').searchParams.get('id'));socket.send('Orchestrated agent console ready\r\n')})
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env,colorScheme:'dark'})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  const section=page.getByRole('region',{name:'Orchestration',exact:true})
  const demo=page.getByRole('region',{name:'Orchestration for Demo',exact:true})
  await expect(section).toBeVisible()
  assert.equal(await page.evaluate(()=>document.querySelector('#orchestration').nextElementSibling.textContent.startsWith('PROJECTS')),true)
  await expect(demo.locator('.orchestration-capacity')).toHaveText('3 / 4 slots')
  await expect(demo.locator('.orchestration-agent')).toHaveCount(3)
  await expect(demo.locator('[data-run-id="run-a"]')).toContainText('Stage: new')
  await expect(demo.locator('[data-run-id="run-a"]')).toContainText('Implementation agent')
  await expect(demo.locator('[data-run-id="run-b"]')).toContainText('Waiting for you')
  await expect(demo.locator('[data-run-id="run-c"]')).toContainText('Queued')
  await expect(page.getByRole('region',{name:'Orchestration for Idle'})).toContainText('No active agents on this workstation')
  const idle=page.getByRole('region',{name:'Orchestration for Idle',exact:true})
  await idle.getByRole('button',{name:'Set up orchestration for Idle'}).click()
  const form=idle.getByRole('form',{name:'Orchestration settings for Idle'})
  await expect(form.getByRole('button',{name:'Evaluate backlog'})).toBeDisabled()
  await form.getByLabel('Enable orchestration').check()
  await form.getByLabel('Concurrency limit').fill('3')
  await form.getByLabel('Eligibility guidelines').fill('Only small fixes with tests')
  // Polling must not replace a draft, move the caret, or enable launch before save.
  await expect(form.getByRole('button',{name:'Evaluate backlog'})).toBeDisabled()
  await expect(form.getByLabel('Eligibility guidelines')).toHaveValue('Only small fixes with tests')
  saveFailure=true
  await form.getByRole('button',{name:'Save settings'}).click()
  await expect(form.getByRole('alert')).toContainText('Saving is temporarily unavailable')
  await expect(form.getByLabel('Eligibility guidelines')).toHaveValue('Only small fixes with tests')
  saveFailure=false
  await form.getByRole('button',{name:'Save settings'}).click()
  await expect(form.getByRole('button',{name:'Evaluate backlog'})).toBeEnabled()
  assert.deepEqual(settings,{orchestratorEnabled:true,orchestratorMaxConcurrent:3,orchestratorPrompt:'Only small fixes with tests'})
  assert.equal(saves,2)
  await page.screenshot({path:path.join(root,'orchestration-setup.png')})
  evaluateFailure=true
  await form.getByRole('button',{name:'Evaluate backlog'}).click()
  await expect(form.getByRole('alert')).toContainText('Evaluation is temporarily unavailable')
  evaluateFailure=false
  await form.getByRole('button',{name:'Evaluate backlog'}).click()
  await expect(form.getByRole('button',{name:'Evaluate backlog'})).toBeDisabled()
  await expect(idle.getByRole('status')).toContainText('1 launched · 1 skipped · 1 / 3 slots used')
  await expect(idle.getByRole('status')).toContainText('IDLE-2: task carries blocker label blocked')
  assert.equal(evaluations,2)
  await idle.getByRole('button',{name:'Set up orchestration for Idle'}).click()
  await idle.getByRole('button',{name:'Set up orchestration for Idle'}).click()
  await expect(form.getByLabel('Concurrency limit')).toHaveValue('3')
  await expect(form.getByLabel('Eligibility guidelines')).toHaveValue('Only small fixes with tests')
  await idle.getByRole('button',{name:'Set up orchestration for Idle'}).click()
  await demo.locator('[data-run-id="run-b"]').click()
  await expect(page.locator('#execution-history')).toHaveValue('run-b')
  await expect(page.locator('#title')).toContainText('DEMO-2')
  await expect.poll(()=>attachments.includes('run-b')).toBe(true)
  await expect(page.locator('#terminal')).toContainText('Orchestrated agent console ready')
  await expect(demo.locator('[data-run-id="run-b"]')).toHaveAttribute('aria-pressed','true')
  await page.locator('#title').hover()
  await page.screenshot({path:path.join(root,'orchestration-dark.png')})
  await page.emulateMedia({colorScheme:'light'})
  await page.screenshot({path:path.join(root,'orchestration-light.png')})
  runs=runs.map(run=>run.id==='run-a'?{...run,status:'completed'}:run.id==='run-b'?{...run,waitingSince:undefined}:run)
  await expect(demo.locator('.orchestration-agent')).toHaveCount(2)
  await expect(demo.locator('[data-run-id="run-b"]')).toContainText('Running')
  // Project usage may include sessions on another workstation, independently
  // of the local run count. Refresh its discovery snapshot on the next poll.
  capacity=4
  await page.evaluate(()=>{const now=Date.now;Date.now=()=>now()+16000})
  await expect(demo.locator('.orchestration-capacity')).toHaveText('4 / 4 slots')
  discoveryAvailable=false
  await page.evaluate(()=>{const now=Date.now;Date.now=()=>now()+16000})
  await expect(page.locator('#orchestration .orchestration-scope')).toContainText('last known usage')
  await expect(demo.locator('progress')).not.toHaveAttribute('value')
  // Older servers do not supply capacity fields, but local agents remain usable.
  discoveryAvailable=true;metadata=false
  await page.evaluate(()=>{const now=Date.now;Date.now=()=>now()+16000})
  await expect(demo.locator('.orchestration-capacity')).toHaveText('Capacity unavailable')
  await expect(demo.locator('[data-run-id="run-b"]')).toBeEnabled()
  controlsAvailable=false
  await demo.getByRole('button',{name:'Set up orchestration for Demo'}).click()
  await expect(demo.getByRole('alert')).toContainText('Update and restart the local agent')
  online=false
  await page.evaluate(()=>document.querySelector('#settings').click())
  await expect(page.locator('#orchestration .orchestration-scope')).toHaveText('Local agent offline')
  await expect(page.locator('#orchestration .orchestration-agent').first()).toBeDisabled()
  console.log('Orchestration screenshots: '+root)
 }finally{
  if(app)await app.close()
  for(const socket of sockets.clients)socket.terminate()
  sockets.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
