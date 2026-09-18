const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('console next step rechecks task state, guards active history and handles failures',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-next-step-'))
 let stage='clarified',prUrl=null,active=false,failRead=false,failLaunch=false,delayRead=0,launches=[]
 const runs=()=>[
  {id:'old',taskId:'task-a',taskKey:'#1',projectId:'project-a',skill:'clarify',status:'completed'},
  {id:'other',taskId:'task-b',taskKey:'#2',projectId:'project-a',skill:'clarify',status:'completed'},
  ...launches.map((launch,index)=>({id:'new-'+index,taskId:'task-a',taskKey:'#1',projectId:'project-a',skill:launch.skillID,status:active&&index===launches.length-1?'queued':'completed'}))
 ]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Project A',path:'/tmp/project'}]));return}
  if(req.url==='/desktop/project?id=project-a'){res.end(JSON.stringify({configured:true,server:{prCreationStage:'implemented',skills:['clarify','specify','implement','adjust'].map(id=>({id}))}}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs()));return}
  if(req.url.startsWith('/desktop/tasks?')){
   if(req.method==='POST'){
    let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
     if(failLaunch){res.writeHead(500);res.end(JSON.stringify({error:'Launch rejected'}));return}
     launches.push(JSON.parse(raw));active=true;res.end(JSON.stringify({status:'queued'}))
    });return
   }
   const response=JSON.stringify([{id:'task-a',key:'#1',labels:['#'+stage],...(prUrl?{prUrl}:{})},{id:'task-b',key:'#2',labels:['#reviewed']}])
   setTimeout(()=>{if(failRead){res.writeHead(503);res.end(JSON.stringify({error:'Offline'}))}else res.end(response)},delayRead);return
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
  const button=page.locator('#next-step'),status=page.locator('#next-step-status')
  const selectA=()=>page.locator('.local-task').filter({has:page.getByRole('button',{name:'Open #1 in Sectile',exact:true})}).locator('.run').click()
  const selectB=()=>page.locator('.local-task').filter({has:page.getByRole('button',{name:'Open #2 in Sectile',exact:true})}).locator('.run').click()
  await page.getByRole('button',{name:'Next: Specify',exact:true}).waitFor()
  stage='specified'
  await button.click()
  await page.getByRole('button',{name:'Next: Implement',exact:true}).waitFor()
  assert.equal(launches.length,0,'A changed step requires another click')
  failLaunch=true;await button.click()
  await page.waitForFunction(()=>document.querySelector('#next-step-status').textContent.includes('Could not launch'))
  assert.equal(await button.isEnabled(),true,'Launch errors allow retry')
  failLaunch=false;await button.dblclick()
  await page.waitForFunction(()=>document.querySelector('#next-step-status').textContent.includes('Execution in progress'))
  assert.equal(launches.length,1)
  assert.deepEqual(launches[0],{taskID:'task-a',skillID:'implement',prompt:''})
  // Ending the execution and launching the next step never apply at the same time.
  assert.equal(await page.locator('#stop').isEnabled(),true,'A running execution can be ended')
  assert.equal(await button.isDisabled(),true,'The next step waits for the execution to end')
  await page.locator('#execution-history').selectOption('old')
  assert.equal(await button.isDisabled(),true,'An older console cannot bypass an active run')
  active=false
  await page.waitForFunction(()=>document.querySelector('#stop').disabled)
  // Ending the execution is what makes the next step available again.
  await page.waitForFunction(()=>!document.querySelector('#next-step').disabled)
  // After implementation, a task without a pull request recovers it through the creation owner...
  stage='implemented';await selectB();await selectA()
  await page.getByRole('button',{name:'Next: Create PR',exact:true}).waitFor()
  await button.click()
  await page.waitForFunction(()=>document.querySelector('#next-step-status').textContent.includes('Execution in progress'))
  assert.equal(launches.length,2)
  assert.deepEqual(launches[1],{taskID:'task-a',skillID:'implement',prompt:''},'The missing pull request is recovered by the creation owner, never by create_pr')
  active=false
  await page.waitForFunction(()=>!document.querySelector('#next-step').disabled)
  // ...and once the task records one, the next step is to adjust it.
  prUrl='https://example.test/pull/1';await selectB();await selectA()
  await page.getByRole('button',{name:'Next: Adjust',exact:true}).waitFor()
  await page.waitForFunction(()=>document.querySelector('#next-step-status').textContent.includes('implemented'))
  prUrl=null
  stage='reviewed';await selectB();await selectA()
  await page.waitForFunction(()=>document.querySelector('#next-step-status').textContent.includes('Awaiting human merge'))
  assert.equal(await button.isHidden(),true)
  stage='finished';await selectA()
  await page.waitForFunction(()=>document.querySelector('#next-step-status').textContent.includes('Task finished'))
  failRead=true;await selectA()
  await page.getByRole('button',{name:'Retry',exact:true}).waitFor()
  assert.equal(await button.isHidden(),true)
  failRead=false;stage='new';await page.getByRole('button',{name:'Retry',exact:true}).click()
  await page.getByRole('button',{name:'Next: Clarify',exact:true}).waitFor()
  delayRead=300;await selectA();delayRead=0;await selectB()
  await page.waitForTimeout(400)
  assert.match(await status.textContent(),/#2.*Awaiting human merge/)
  assert.equal(await button.isHidden(),true,'Late task A metadata cannot change task B action')
  await selectA();await page.getByRole('button',{name:'Next: Clarify',exact:true}).waitFor()
  await page.setViewportSize({width:720,height:600})
  const bounds=await page.locator('#task-status').boundingBox(),terminal=await page.locator('#terminal').boundingBox()
  assert.ok(bounds.y>=terminal.y+terminal.height-1)
  const action=await button.boundingBox()
  assert.ok(action.x>=0&&action.x+action.width<=720&&action.y+action.height<=600,'Next action stays inside the narrow viewport')
  // The action belongs to the execution controls, not to the status line it describes.
  assert.equal(await page.locator('#toolbar #next-step').count(),1,'The next action sits in the execution toolbar')
  assert.equal(await page.locator('#task-status button').count(),0,'The footer keeps the status text alone')
  assert.equal(await page.evaluate(()=>document.querySelector('#next-step').nextElementSibling.id),'retry-next-step')
  // Closing the current step comes before launching the next one, in the order the user acts.
  assert.equal(await page.evaluate(()=>document.querySelector('#stop').nextElementSibling.id),'next-step','The closing control precedes the next action')
  assert.equal(await page.evaluate(()=>document.querySelector('#stop').previousElementSibling.id),'save-log')
  assert.equal(await page.evaluate(()=>!!document.querySelector('#stop').querySelector('path[d*="M7 7 17 17"]')),false,'The closing control drops the cross glyph')
  await page.screenshot({path:path.join(root,'next-step.png')})
  console.log('Next-step screenshot: '+path.join(root,'next-step.png'))
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
