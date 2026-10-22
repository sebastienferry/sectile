const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const {WebSocketServer}=require('ws')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('orchestration shows live agents, authoritative capacity, workflow stages and console navigation',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-orchestration-'))
 let online=true,capacity=3,discoveryAvailable=true,metadata=true,controlsAvailable=true,saveFailure=false,evaluateFailure=false
 let settings={orchestratorEnabled:false,orchestratorMaxConcurrent:1,orchestrationFile:'AGENTS.md',guidelines:'# Local guidelines'},evaluations=0,saves=0
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
    let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{saves++;if(saveFailure){res.writeHead(503).end(JSON.stringify({error:'Saving is temporarily unavailable'}));return}settings={...settings,...JSON.parse(raw)};res.end(JSON.stringify(settings))});return
   }
   evaluations++;if(evaluateFailure){res.writeHead(503).end(JSON.stringify({error:'Evaluation is temporarily unavailable'}));return}
   setTimeout(()=>res.end(JSON.stringify({projectId:'idle',activeRunsAfter:1,maxConcurrent:settings.orchestratorMaxConcurrent,launched:[{key:'IDLE-1'}],skipped:[{key:'IDLE-2',reason:'task carries blocker label blocked'}]})),500);return
  }
  if(req.url.startsWith('/desktop/project?')){res.end(JSON.stringify({server:{projectName:'Idle',skills:[]},path:'/tmp/idle',configured:true,parallelism:4,aiProvider:'claude'}));return}
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
  const idle=page.getByRole('region',{name:'Orchestration for Idle',exact:true})
  await expect(idle).toHaveCount(0)
  await expect(section.getByRole('button',{name:/Set up/})).toHaveCount(0)
  await page.getByRole('button',{name:'Actions for Idle',exact:true}).click()
  await page.getByRole('menuitem',{name:'Project settings…',exact:true}).click()
  await page.locator('#project-tab-Orchestration').click()
  const form=page.getByRole('form',{name:'Orchestration settings for Idle'})
  await expect(form).toBeVisible()
  await expect(page.getByRole('button',{name:'Save local configuration',exact:true})).toBeHidden()
  await expect(form.getByLabel('Guidelines file')).toHaveValue('AGENTS.md')
  await expect(form.getByLabel('Guidelines preview')).toHaveText('# Local guidelines')
  await expect(form.getByRole('button',{name:'Evaluate backlog'})).toBeDisabled()
  await form.getByLabel('Enable orchestration').check()
  await form.getByLabel('Concurrency limit').fill('3')
  await form.getByLabel('Guidelines file').fill('docs/orchestration.md')
  await expect(form.getByRole('button',{name:'Evaluate backlog'})).toBeDisabled()
  saveFailure=true
  await form.getByRole('button',{name:'Save orchestration settings'}).click()
  await expect(form.getByRole('alert')).toContainText('Saving is temporarily unavailable')
  await expect(form.getByLabel('Guidelines file')).toHaveValue('docs/orchestration.md')
  saveFailure=false
  await form.getByRole('button',{name:'Save orchestration settings'}).click()
  await expect(form.getByRole('button',{name:'Evaluate backlog'})).toBeEnabled()
  assert.equal(settings.orchestratorEnabled,true)
  assert.equal(settings.orchestratorMaxConcurrent,3)
  assert.equal(settings.orchestrationFile,'docs/orchestration.md')
  assert.equal(saves,2)
  settings.guidelines='# Updated file contents'
  await form.getByRole('button',{name:'Reload file'}).click()
  await expect(form.getByLabel('Guidelines preview')).toHaveText('# Updated file contents')
  await page.screenshot({path:path.join(root,'orchestration-settings.png')})
  evaluateFailure=true
  await form.getByRole('button',{name:'Evaluate backlog'}).click()
  await expect(form.getByRole('alert')).toContainText('Evaluation is temporarily unavailable')
  evaluateFailure=false
  await form.getByRole('button',{name:'Evaluate backlog'}).click()
  await expect(form.getByRole('button',{name:'Evaluate backlog'})).toBeDisabled()
  await expect(form.getByRole('status')).toContainText('1 launched · 1 skipped · 1 / 3 slots used')
  await expect(form.getByRole('status')).toContainText('IDLE-2: task carries blocker label blocked')
  assert.equal(evaluations,2)
  await page.locator('.configuration-page').getByRole('button',{name:'Back',exact:true}).click()
  await expect(idle).toBeVisible()
  await expect(idle.getByRole('button',{name:'Evaluate backlog'})).toBeEnabled()
  await expect(idle.getByRole('checkbox')).toHaveCount(0)
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
  await idle.getByRole('button',{name:'Evaluate backlog'}).click()
  await expect(idle.getByRole('alert')).toContainText('Update and restart the local agent')
  settings.orchestratorEnabled=false;runs=[]
  await page.evaluate(()=>{const now=Date.now;Date.now=()=>now()+16000})
  await expect(section).toBeHidden()
  runs=[{id:'manual',taskId:'task-a',taskKey:'DEMO-1',projectId:'demo',status:'running',skill:'clarify'}]
  await expect(section).toBeVisible()
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
