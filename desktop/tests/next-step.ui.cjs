const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('console next step rechecks task state, guards active history and handles failures',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-next-step-'))
 let stage='clarified',prUrl=null,active=false,failRead=false,failLaunch=false,delayRead=0,launches=[],transitions=[],extra=[],projectSkills=['clarify','specify','implement','adjust','handoff','pickup']
 const runs=()=>[
  {id:'old',taskId:'task-a',taskKey:'#1',projectId:'project-a',skill:'clarify',status:'completed'},
  {id:'other',taskId:'task-b',taskKey:'#2',projectId:'project-a',skill:'clarify',status:'completed'},
  // One run per launch, each with its own id: the renderer only treats a launch as started once a run it had not seen appears.
  ...launches.map((launch,index)=>({id:'new-'+index,taskId:'task-a',taskKey:'#1',projectId:'project-a',skill:launch.skillID,status:active&&index===launches.length-1?'queued':'completed'})),
  // Executions launched elsewhere, such as a pickup chain or the Tickets pane menu.
  ...extra.map(run=>({taskId:'task-a',taskKey:'#1',projectId:'project-a',...run}))
 ]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:['transition-stage']}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Project A',path:'/tmp/project'}]));return}
  if(req.url==='/desktop/project?id=project-a'){res.end(JSON.stringify({configured:true,server:{prCreationStage:'implemented',skills:projectSkills.map(id=>({id}))}}));return}
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
  if(req.url.startsWith('/desktop/tasks/transition?')){
   let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
    const data=JSON.parse(raw);transitions.push(data)
    stage=data.stage
    res.end(JSON.stringify({success:true,stage:data.stage}))
   });return
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
  const button=page.locator('#next-step'),chain=page.locator('#pickup-chain'),badge=page.locator('#next-step-label'),status=page.locator('#next-step-status')
  const selectA=()=>page.locator('.local-task').filter({has:page.getByRole('button',{name:'Open #1 in Sectile',exact:true})}).locator('.run').click()
  const selectB=()=>page.locator('.local-task').filter({has:page.getByRole('button',{name:'Open #2 in Sectile',exact:true})}).locator('.run').click()
  await page.getByRole('button',{name:'Next: Specify',exact:true}).waitFor()
  assert.equal(await badge.textContent(),'Next: Specify','The badge names the next step')
  assert.equal(await badge.isVisible(),true,'The badge is visible with the button')
  assert.equal(await chain.isVisible(),true,'The full chain button is visible when pickup is available')
  assert.equal(await chain.isEnabled(),true,'The full chain button is enabled when idle')
  assert.equal(await chain.getAttribute('aria-label'),'Pickup (full chain)')
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
  assert.equal(await chain.isDisabled(),true,'The full chain button waits for the execution to end')
  assert.equal(await chain.isVisible(),true,'The full chain button stays visible during an execution')
  assert.equal(await button.getAttribute('aria-label'),'Current: Implement','The button names the running skill')
  assert.equal(await badge.textContent(),'Current: Implement','The badge names the running skill')
  await page.locator('#execution-history').selectOption('old')
  assert.equal(await button.isDisabled(),true,'An older console cannot bypass an active run')
  assert.equal(await chain.isDisabled(),true,'An older console still disables full chain')
  assert.equal(await button.getAttribute('aria-label'),'Current: Implement','An older console still names the active run')
  assert.equal(await badge.textContent(),'Current: Implement','An older console still names the active run')
  // The execution completes and moves the stage: the button proposes the step that follows.
  stage='implemented';active=false
  await page.waitForFunction(()=>document.querySelector('#stop').disabled)
  // Ending the execution is what makes the next step available again.
  // After implementation, a task without a pull request recovers it through the creation owner...
  await page.getByRole('button',{name:'Next: Create PR',exact:true}).waitFor()
  await page.waitForFunction(()=>!document.querySelector('#next-step').disabled)
  await button.click()
  await page.waitForFunction(()=>document.querySelector('#next-step-status').textContent.includes('Execution in progress'))
  assert.equal(launches.length,2)
  assert.deepEqual(launches[1],{taskID:'task-a',skillID:'implement',prompt:''},'The missing pull request is recovered by the creation owner, never by create_pr')
  assert.equal(await button.getAttribute('aria-label'),'Current: Implement','The button names the skill launched, not the step label')
  assert.equal(await badge.textContent(),'Current: Implement','The badge names the skill launched, not the step label')
  // An execution that ends without moving the stage proposes the same step again.
  active=false
  await page.getByRole('button',{name:'Next: Create PR',exact:true}).waitFor()
  await page.waitForFunction(()=>!document.querySelector('#next-step').disabled)
  // ...and once the task records one, the next step is to adjust it, and mark-reviewed is available.
  prUrl='https://example.test/pull/1';await selectB();await selectA()
  await page.getByRole('button',{name:'Next: Adjust',exact:true}).waitFor()
  await page.waitForFunction(()=>document.querySelector('#next-step-status').textContent.includes('implemented'))
  await page.getByRole('button',{name:'Mark reviewed',exact:true}).waitFor()
  assert.equal(await page.locator('#mark-reviewed').isEnabled(),true)

  // Declare code as reviewed opens confirmation dialog
  await page.locator('#mark-reviewed').click()
  await page.waitForSelector('#project-dialog[open]')
  assert.match(await page.locator('#dialog-body h2').textContent(),/Declare #1 as reviewed\?/)
  assert.match(await page.locator('#dialog-body p').first().textContent(),/proposes Handoff after human merge/)
  await page.getByRole('button',{name:'Confirm',exact:true}).click()
  await page.waitForFunction(()=>!document.querySelector('#project-dialog').open)
  assert.equal(transitions.length,1)
  assert.equal(transitions[0].stage,'reviewed')

  // In reviewed stage, Next: Handoff is proposed and Mark reviewed is hidden
  await page.getByRole('button',{name:'Next: Handoff',exact:true}).waitFor()
  assert.equal(await page.locator('#mark-reviewed').isHidden(),true)
  prUrl=null
  stage='finished';await selectA()
  await page.waitForFunction(()=>document.querySelector('#next-step-status').textContent.includes('Task finished'))
  assert.equal(await button.isHidden(),true)
  assert.equal(await chain.isHidden(),true)
  assert.equal(await badge.isHidden(),true)
  failRead=true;await selectA()
  await page.getByRole('button',{name:'Retry',exact:true}).waitFor()
  assert.equal(await button.isHidden(),true)
  assert.equal(await chain.isHidden(),true)
  assert.equal(await badge.isHidden(),true)
  failRead=false;stage='new';await page.getByRole('button',{name:'Retry',exact:true}).click()
  await page.getByRole('button',{name:'Next: Clarify',exact:true}).waitFor()
  delayRead=300;await selectA();delayRead=0;await selectB()
  await page.waitForTimeout(400)
  assert.match(await status.textContent(),/#2.*reviewed.*Ready for the next step/)
  assert.equal(await page.getByRole('button',{name:'Next: Handoff',exact:true}).isVisible(),true,'Late task A metadata cannot change task B action')
  await selectA();await page.getByRole('button',{name:'Next: Clarify',exact:true}).waitFor()
  // A skill other than the stage step names the button, without any reselection.
  extra=[{id:'pickup',skill:'pickup',status:'queued',createdAt:'2026-09-26T10:00:00Z'}]
  await page.getByRole('button',{name:'Current: Pickup',exact:true}).waitFor()
  assert.equal(await button.isDisabled(),true)
  assert.equal(await chain.isDisabled(),true)
  // Several active executions: the most recent one names the button.
  extra=[extra[0],{id:'adjust',skill:'adjust',status:'preparing',createdAt:'2026-09-26T10:05:00Z'}]
  await page.getByRole('button',{name:'Current: Adjust',exact:true}).waitFor()
  extra=[{id:'pickup',skill:'pickup',status:'completed',createdAt:'2026-09-26T10:00:00Z'}]
  await page.getByRole('button',{name:'Next: Clarify',exact:true}).waitFor()
  assert.equal(await button.isEnabled(),true)
  assert.equal(await chain.isEnabled(),true)
  // A finished task proposes no step, yet still names what runs on it.
  stage='finished';extra=[{id:'discuss',skill:'discuss',status:'running',createdAt:'2026-09-26T10:10:00Z'}];await selectA()
  await page.getByRole('button',{name:'Current: Discuss',exact:true}).waitFor()
  assert.equal(await button.isDisabled(),true)
  assert.equal(await chain.isHidden(),true)
  stage='new';extra=[{...extra[0],status:'completed'}];await selectB();await selectA()
  await page.getByRole('button',{name:'Next: Clarify',exact:true}).waitFor()
  // An active execution without a skill never leaves the next step enabled: the stage step names it.
  extra=[...extra,{id:'blank',skill:'',status:'running',createdAt:'2026-09-26T10:15:00Z'}]
  await page.getByRole('button',{name:'Current: Clarify',exact:true}).waitFor()
  assert.equal(await button.isDisabled(),true)
  assert.equal(await chain.isDisabled(),true)
  extra=[extra[0],{...extra[1],status:'completed'}]
  await page.getByRole('button',{name:'Next: Clarify',exact:true}).waitFor()

  // Full-chain button (US2, US3, FR5-FR10)
  assert.equal(await chain.isVisible(),true)
  assert.equal(await chain.isEnabled(),true)
  assert.equal(await chain.getAttribute('aria-label'),'Pickup (full chain)')

  // Clicking >> posts pickup with mode autonomous and selects the new console (US2.2)
  const launchCountBefore=launches.length
  await chain.click()
  await page.waitForFunction(()=>document.querySelector('#next-step-status').textContent.includes('Execution in progress'))
  assert.equal(launches.length,launchCountBefore+1)
  assert.deepEqual(launches[launchCountBefore],{taskID:'task-a',skillID:'pickup',prompt:'',mode:'autonomous'})
  assert.equal(await button.isDisabled(),true)
  assert.equal(await chain.isDisabled(),true)
  assert.equal(await badge.textContent(),'Current: Pickup')

  // A failed >> launch shows 'Could not launch full chain' and re-enables both buttons (US2.5)
  active=false
  await page.waitForFunction(()=>!document.querySelector('#pickup-chain').disabled)
  failLaunch=true;await chain.click()
  await page.waitForFunction(()=>document.querySelector('#next-step-status').textContent.includes('Could not launch full chain'))
  assert.equal(await chain.isEnabled(),true,'Failed pickup launch allows retry')
  assert.equal(await button.isEnabled(),true,'Failed pickup launch leaves next step enabled')
  failLaunch=false

  // A stage change between display and click does not abandon a >> launch (FR8)
  stage='specified'
  const countBeforeStageChange=launches.length
  await chain.click()
  await page.waitForFunction(()=>document.querySelector('#next-step-status').textContent.includes('Execution in progress'))
  assert.equal(launches.length,countBeforeStageChange+1,'Stage change does not abandon full chain launch')
  assert.deepEqual(launches[countBeforeStageChange],{taskID:'task-a',skillID:'pickup',prompt:'',mode:'autonomous'})
  active=false
  await page.waitForFunction(()=>!document.querySelector('#pickup-chain').disabled)

  // An active run detected during recheck abandons >> launch (FR8)
  extra=[{id:'active-now',taskId:'task-a',taskKey:'#1',projectId:'project-a',skill:'clarify',status:'running',createdAt:'2026-09-26T12:00:00Z'}]
  const countBeforeActiveAbandon=launches.length
  await chain.click()
  await page.waitForFunction(()=>document.querySelector('#pickup-chain').disabled)
  assert.equal(launches.length,countBeforeActiveAbandon,'An active run abandons full-chain launch')
  extra=[]
  await page.waitForFunction(()=>!document.querySelector('#pickup-chain').disabled)

  // >> is hidden without a pickup skill (US3.2)
  projectSkills=['clarify','specify','implement','adjust','handoff']
  await selectB();await selectA()
  await page.waitForFunction(()=>document.querySelector('#pickup-chain').hidden)
  assert.equal(await chain.isHidden(),true,'>> is hidden without pickup skill')
  projectSkills=['clarify','specify','implement','adjust','handoff','pickup']
  await selectB();await selectA()
  await page.waitForFunction(()=>!document.querySelector('#pickup-chain').hidden)
  assert.equal(await chain.isVisible(),true)

  await page.setViewportSize({width:720,height:600})
  const bounds=await page.locator('#task-status').boundingBox(),terminal=await page.locator('#terminal').boundingBox()
  assert.ok(bounds.y>=terminal.y+terminal.height-1)
  const action=await button.boundingBox()
  assert.ok(action.x>=0&&action.x+action.width<=720&&action.y+action.height<=600,'Next action stays inside the narrow viewport')
  // The action belongs to the execution controls, not to the status line it describes.
  assert.equal(await page.locator('#toolbar #next-step').count(),1,'The next action sits in the execution toolbar')
  assert.equal(await page.locator('#task-status button').count(),0,'The footer keeps the status text alone')
  assert.equal(await page.evaluate(()=>document.querySelector('#stop').nextElementSibling.id),'next-step','The closing control precedes the next action')
  assert.equal(await page.evaluate(()=>document.querySelector('#next-step').nextElementSibling.id),'pickup-chain')
  assert.equal(await page.evaluate(()=>document.querySelector('#pickup-chain').nextElementSibling.id),'next-step-label')
  assert.equal(await page.evaluate(()=>document.querySelector('#next-step-label').nextElementSibling.id),'mark-reviewed')
  assert.equal(await page.evaluate(()=>document.querySelector('#mark-reviewed').nextElementSibling.id),'retry-next-step')
  assert.equal(await page.evaluate(()=>document.querySelector('#retry-next-step').nextElementSibling.id),'force-next-step')
  // Closing the current step comes before launching the next one, in the order the user acts.
  assert.equal(await page.evaluate(()=>document.querySelector('#stop').nextElementSibling.id),'next-step','The closing control precedes the next action')
  assert.equal(await page.evaluate(()=>document.querySelector('#stop').previousElementSibling.id),'save-log')
  assert.equal(await page.evaluate(()=>!!document.querySelector('#stop').querySelector('path[d*="M7 7 17 17"]')),false,'The closing control drops the cross glyph')
  const screenshotDir=process.env.SECTILE_SCREENSHOT_DIR||root
  await page.emulateMedia({colorScheme:'light'})
  await page.screenshot({path:path.join(screenshotDir,'next-step-light.png')})
  console.log('Next-step screenshot (light): '+path.join(screenshotDir,'next-step-light.png'))
  await page.emulateMedia({colorScheme:'dark'})
  await page.screenshot({path:path.join(screenshotDir,'next-step-dark.png')})
  console.log('Next-step screenshot (dark): '+path.join(screenshotDir,'next-step-dark.png'))
  await page.screenshot({path:path.join(screenshotDir,'next-step.png')})
  console.log('Next-step screenshot: '+path.join(screenshotDir,'next-step.png'))
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
