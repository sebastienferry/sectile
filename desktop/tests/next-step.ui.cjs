const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('console next step rechecks task state, guards active history and handles failures',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'taskflow-next-step-'))
 let stage='clarified',active=false,failRead=false,failLaunch=false,delayRead=0,launches=[]
 const runs=()=>[
  {id:'old',taskId:'task-a',taskKey:'#1',projectId:'project-a',skill:'clarify',status:'completed'},
  {id:'other',taskId:'task-b',taskKey:'#2',projectId:'project-a',skill:'clarify',status:'completed'},
  ...(launches.length?[{id:'new',taskId:'task-a',taskKey:'#1',projectId:'project-a',skill:'specify',status:active?'queued':'completed'}]:[])
 ]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Project A',path:'/tmp/project'}]));return}
  if(req.url==='/desktop/project?id=project-a'){res.end(JSON.stringify({configured:true,server:{skills:['clarify','specify','implement','create_pr'].map(id=>({id}))}}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs()));return}
  if(req.url.startsWith('/desktop/tasks?')){
   if(req.method==='POST'){
    let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
     if(failLaunch){res.writeHead(500);res.end(JSON.stringify({error:'Launch rejected'}));return}
     launches.push(JSON.parse(raw));active=true;res.end(JSON.stringify({status:'queued'}))
    });return
   }
   const response=JSON.stringify([{id:'task-a',key:'#1',labels:['#'+stage]},{id:'task-b',key:'#2',labels:['#reviewed']}])
   setTimeout(()=>{if(failRead){res.writeHead(503);res.end(JSON.stringify({error:'Offline'}))}else res.end(response)},delayRead);return
  }
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,TASKFLOW_DESKTOP_DATA_DIR:root,TASKFLOW_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  const button=page.locator('#next-step'),status=page.locator('#next-step-status')
  const selectA=()=>page.locator('.local-task').filter({has:page.getByRole('button',{name:'Open #1 in TaskFlow',exact:true})}).locator('.run').click()
  const selectB=()=>page.locator('.local-task').filter({has:page.getByRole('button',{name:'Open #2 in TaskFlow',exact:true})}).locator('.run').click()
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
  await page.locator('#execution-history').selectOption('old')
  assert.equal(await button.isDisabled(),true,'An older console cannot bypass an active run')
  active=false;stage='reviewed';await selectB();await selectA()
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
  await page.screenshot({path:path.join(root,'next-step.png')})
  console.log('Next-step screenshot: '+path.join(root,'next-step.png'))
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
