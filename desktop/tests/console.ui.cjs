const {test}=require('node:test')
const assert=require('node:assert/strict')
const { _electron: electron }=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')
const {WebSocketServer}=require('ws')

test('desktop console reconnects, accepts input and stops the owned run',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'taskflow-desktop-test-'))
 let stopped=false,input='',submitted=false,launches=[],available=true,extraRun=false,createdInput=null,serverCommand='codex {prompt}',withoutConsole=false,attachments=0
 const server=http.createServer((req,res)=>{
  if(req.headers.authorization!=='Bearer test-secret'){res.writeHead(401).end();return}
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Example project',path:'/tmp/spec-worktree'},{id:'project-b',name:'Other project',path:'/tmp/other-worktree'}]));return}
  if(req.url==='/desktop/project?id=project-a'||req.url==='/desktop/project?id=project-b'){res.end(JSON.stringify({server:{projectName:'Example project',gitRemoteUrl:'https://example.test/repo.git',specFramework:'openspec',useWorktrees:true,parallelism:2,aiCommandTemplate:serverCommand,skills:[{id:'specify',content:'Specification instructions'}]},monoRepo:true,path:'/tmp/spec-worktree',configured:true,useWorktrees:true}));return}
  if(req.url.startsWith('/desktop/tasks?')){
   if(req.method==='POST'){submitted=true;let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{launches.push({...JSON.parse(raw),projectID:new URL(req.url,'http://localhost').searchParams.get('projectId')});res.end(JSON.stringify({status:'running'}))});return}
   if(createdInput&&new URL(req.url,'http://localhost').searchParams.get('q')==='#49'){res.end(JSON.stringify([{id:'created',key:'#49',projectId:createdInput.projectID,title:createdInput.title,status:'to_clarify'}]));return}
   res.end(JSON.stringify([{id:'task-1',key:'#48',title:'Server specification task',status:'to_implement',labels:['#specified'],trackerStatus:'Ready for development',prUrl:'https://github.com/example/repo/pull/48'}]));return
  }
  if(req.url==='/desktop/create-task'){
   let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{createdInput=JSON.parse(raw);res.writeHead(201);res.end(JSON.stringify({id:'created',key:'#49',projectId:createdInput.projectID,title:createdInput.title}))});return
  }
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:['create-task']}));return}
  if(req.url==='/desktop/runs'&&!available){res.writeHead(503).end();return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify([{id:'run-1',taskId:'task-1',taskKey:'#48',projectId:'project-a',skill:'specify',prompt:'Previous instructions',directory:'/tmp/spec-worktree',sessionId:withoutConsole?'':'run-1',status:withoutConsole?'failed':stopped?'canceled':'running'},...(extraRun?[{id:'run-0',taskId:'task-1',taskKey:'#48',projectId:'project-a',skill:'clarify',status:'completed',sessionId:'run-0'}]:[])]));return}
  if(req.url==='/desktop/stop?id=run-1'){stopped=true;res.writeHead(204).end();return}
  res.writeHead(404).end()
 })
 const ws=new WebSocketServer({noServer:true})
 server.on('upgrade',(req,socket,head)=>{
  attachments++
  if(req.headers.authorization!=='Bearer test-secret'){socket.destroy();return}
  ws.handleUpgrade(req,socket,head,client=>{
   client.send(Buffer.from('Specification console ready\r\n'))
   client.on('message',data=>{const message=JSON.parse(data);if(message.type==='input')input+=message.data})
  })
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}),{mode:0o600})
 const env={...process.env,TASKFLOW_DESKTOP_DATA_DIR:root,TASKFLOW_DESKTOP_TEST:'1'}
 delete env.ELECTRON_RUN_AS_NODE
 let application
 try{
  application=await electron.launch({executablePath:process.env.TASKFLOW_DESKTOP_EXECUTABLE,args:process.env.TASKFLOW_DESKTOP_EXECUTABLE?[]:[path.resolve(__dirname,'..')],env})
  let page=await application.firstWindow()
  await page.getByText('#48 · specify',{exact:true}).waitFor()
  await page.locator('.xterm-screen').waitFor()
  withoutConsole=true
  await page.locator('.run[data-status=failed]').waitFor()
  const attachmentCount=attachments
  await page.locator('.run[data-status=failed]').click()
  await page.waitForTimeout(200)
  assert.equal(attachments,attachmentCount,'A run without a session must not open a WebSocket')
  withoutConsole=false
  await page.locator('.run[data-status=running]').waitFor()
  await page.waitForTimeout(200)
  assert.ok(attachments>attachmentCount,'Console attachment resumes when a session is ready')
  const before=await page.locator('aside').evaluate(element=>element.getBoundingClientRect().width)
  await page.getByRole('separator',{name:'Resize sidebar'}).focus()
  await page.keyboard.press('ArrowRight')
  assert.equal(await page.locator('aside').evaluate(element=>element.getBoundingClientRect().width),before+20)
  await page.getByRole('button',{name:'Open PR #48 for #48',exact:true}).waitFor()
  assert.equal(await page.locator('#selected-pr').textContent(),'PR #48')
  await page.getByRole('button',{name:'Local agent',exact:true}).click()
  await page.getByRole('heading',{name:'Connect to TaskFlow'}).waitFor()
  assert.equal(await page.locator('#setup input').count(),2)
  assert.equal(await page.getByRole('button',{name:'Connect',exact:true}).isDisabled(),true)
  await page.getByRole('button',{name:'Local agent',exact:true}).click()
  await page.locator('#add-project').click()
  assert.equal(await page.getByRole('button',{name:'Example project · Already added',exact:true}).isDisabled(),true)
  await page.getByRole('button',{name:'Close',exact:true}).click()
  await page.getByRole('button',{name:'Configure Example project',exact:true}).click()
  await page.getByRole('tab',{name:'Server',exact:true}).click()
  await page.getByText('Server configuration · Read only',{exact:true}).waitFor()
  await page.getByRole('tab',{name:'Local',exact:true}).click()
  await application.evaluate(({dialog})=>{dialog.showOpenDialog=async()=>({canceled:false,filePaths:['/tmp/chosen-repository']})})
  await page.getByRole('button',{name:'Choose folder…',exact:true}).click()
  await page.waitForFunction(()=>document.querySelector('[aria-label="Local repository"]').value==='/tmp/chosen-repository')
  await page.getByRole('button',{name:'No',exact:true}).click()
  assert.equal(await page.getByRole('button',{name:'2',exact:true}).isDisabled(),true)
  await page.getByRole('button',{name:'Reset worktrees to server default',exact:true}).click()
  assert.equal(await page.getByRole('button',{name:'2',exact:true}).isDisabled(),false)
  await page.getByRole('button',{name:'3',exact:true}).click()
  await page.getByRole('button',{name:'Reset parallelism to server default',exact:true}).click()
  await page.getByRole('textbox',{name:'CLI command',exact:true}).fill('claude {prompt}')
  await page.getByRole('button',{name:'Reset CLI command to server default',exact:true}).click()
  assert.equal(await page.getByRole('textbox',{name:'CLI command',exact:true}).inputValue(),'codex {prompt}')
  await page.getByRole('textbox',{name:'CLI command',exact:true}).fill('local {prompt}')
  serverCommand='updated {prompt}'
  await page.getByRole('button',{name:'Refresh from server',exact:true}).click()
  await page.getByText('Server settings refreshed. Local overrides preserved.',{exact:true}).waitFor()
  assert.equal(await page.getByRole('textbox',{name:'CLI command',exact:true}).inputValue(),'local {prompt}')
  await page.getByRole('button',{name:'Reset CLI command to server default',exact:true}).click()
  assert.equal(await page.getByRole('textbox',{name:'CLI command',exact:true}).inputValue(),'updated {prompt}')
  serverCommand='latest {prompt}'
  await page.getByRole('button',{name:'Refresh from server',exact:true}).click()
  await page.waitForFunction(()=>document.querySelector('[aria-label="CLI command"]').value==='latest {prompt}')


  assert.equal(await page.getByRole('button',{name:'2',exact:true}).getAttribute('aria-pressed'),'true')
  await page.getByRole('button',{name:'Close',exact:true}).click()
  await page.getByRole('button',{name:'Toggle projects',exact:true}).click()
  assert.equal(await page.locator('aside').isVisible(),false)
  await page.getByRole('button',{name:'Toggle projects',exact:true}).click()
  await page.getByRole('button',{name:'Profile',exact:true}).click()
  await page.getByRole('heading',{name:'Profile',exact:true}).waitFor()
  await page.getByRole('button',{name:'Close',exact:true}).click()
  await page.waitForTimeout(300)
  await page.evaluate(()=>window.localAgent.input('hello'))
  await page.waitForTimeout(100)
  assert.equal(input,'hello')
  await page.screenshot({path:path.join(root,'console.png')})
  await page.keyboard.press('Meta+k')
  await page.getByRole('button',{name:'Quick add task',exact:true}).click()
  assert.equal(await page.getByRole('combobox',{name:'Quick add project'}).inputValue(),'project-a')
  await page.getByRole('textbox',{name:'Task title',exact:true}).fill('Created from desktop')
  await page.getByRole('button',{name:'Create task',exact:true}).click()
  await page.getByText('Created #49 · Created from desktop',{exact:true}).waitFor()
  assert.equal(createdInput.projectID,'project-a')
  await page.getByRole('button',{name:'Close',exact:true}).click()
  await page.keyboard.press('Control+k')
  await page.getByRole('button',{name:'Quick add task',exact:true}).waitFor()
  await page.getByRole('button',{name:'Close',exact:true}).click()
  await page.getByRole('button',{name:'New task in Example project',exact:true}).click()
  await page.getByRole('button',{name:'Run an existing ticket',exact:true}).click()
  await page.getByRole('textbox',{name:'Search server tasks'}).fill('48')
  await page.getByRole('button',{name:'Search',exact:true}).click()
  await page.getByText('#48 · Server specification task',{exact:true}).waitFor()
  await page.getByText('Current state: specified · Ready for development',{exact:true}).waitFor()
  await page.getByRole('button',{name:'Launch',exact:true}).click()
  await page.waitForFunction(()=>!document.querySelector('#project-dialog').open)
  assert.equal(submitted,true)
  assert.equal(launches.at(-1).projectID,'project-a')
  assert.equal(launches.at(-1).taskID,'task-1')

  const launchCount=launches.length
  const previousCreated=createdInput
  await page.getByRole('button',{name:'New task in Other project',exact:true}).click()
  await page.getByRole('button',{name:'Run an existing ticket',exact:true}).waitFor()
  await page.getByRole('button',{name:'Quick add task',exact:true}).waitFor()
  await page.screenshot({path:path.join(root,'new-task-choice.png')})
  console.log('Choice screenshot:',path.join(root,'new-task-choice.png'))
  await page.getByRole('button',{name:'Close',exact:true}).click()
  assert.equal(launches.length,launchCount,'Dismissing the choice does not launch')
  assert.equal(createdInput,previousCreated,'Dismissing the choice does not create')
  await page.locator('.run[data-status=running]').click()
  await page.getByRole('button',{name:'New task in Other project',exact:true}).click()
  await page.getByRole('button',{name:'Quick add task',exact:true}).click()
  assert.equal(await page.getByRole('combobox',{name:'Quick add project'}).inputValue(),'project-b')
  await page.getByRole('textbox',{name:'Task title',exact:true}).fill('Created in clicked project')
  await page.getByRole('button',{name:'Create task',exact:true}).click()
  await page.getByText('Created #49 · Created in clicked project',{exact:true}).waitFor()
  assert.equal(createdInput.projectID,'project-b')
  assert.equal(launches.length,launchCount,'Creation requires an explicit launch')
  await page.getByRole('button',{name:'Launch task',exact:true}).click()
  await page.getByText('#49 · Created in clicked project',{exact:true}).waitFor()
  await page.getByRole('button',{name:'Launch',exact:true}).click()
  await page.waitForFunction(()=>!document.querySelector('#project-dialog').open)
  assert.equal(launches.length,launchCount+1)
  assert.equal(launches.at(-1).taskID,'created')
  assert.equal(launches.at(-1).projectID,'project-b')

  await page.getByRole('button',{name:'Stop execution',exact:true}).click()
  await page.locator('.run[data-status=canceled]').waitFor()
  assert.equal(stopped,true)
  await page.getByRole('button',{name:'Relaunch',exact:true}).click()
  assert.equal(await page.getByRole('combobox',{name:'Relaunch skill'}).inputValue(),'specify')
  assert.equal(await page.getByRole('textbox',{name:'Relaunch instructions'}).inputValue(),'Previous instructions')
  await page.getByRole('combobox',{name:'Relaunch skill'}).selectOption('custom')
  await page.getByRole('textbox',{name:'Relaunch instructions'}).fill('Updated instructions')
  await page.getByRole('button',{name:'Launch new execution',exact:true}).click()
  await page.waitForFunction(()=>!document.querySelector('#project-dialog').open)
  assert.equal(launches.at(-1).skillID,'custom')
  assert.equal(launches.at(-1).prompt,'Updated instructions')

  await application.close()
  application=await electron.launch({executablePath:process.env.TASKFLOW_DESKTOP_EXECUTABLE,args:process.env.TASKFLOW_DESKTOP_EXECUTABLE?[]:[path.resolve(__dirname,'..')],env})
  page=await application.firstWindow()
  await page.getByText('#48 · specify',{exact:true}).waitFor()
  await page.locator('.run[data-status=canceled]').waitFor()
  extraRun=true
  await page.waitForFunction(()=>document.querySelector('#execution-history').options.length===2)
  assert.equal(await page.locator('.local-task').count(),1)
  assert.equal(await page.getByRole('combobox',{name:'Execution history'}).locator('option').count(),2)
  await page.getByRole('button',{name:'Actions for #48',exact:true}).click()
  await page.getByRole('textbox',{name:'Local task name'}).fill('Local review')
  await page.getByRole('button',{name:'Rename locally',exact:true}).click()
  await page.getByRole('button',{name:'Actions for Local review',exact:true}).waitFor()
  stopped=false
  await page.locator('.run[data-status=running]').waitFor()
  await page.locator('.local-task').hover()
  await page.getByRole('button',{name:'Stop and archive Local review',exact:true}).click()
  await page.getByRole('button',{name:'Stop and archive',exact:true}).click()
  await page.waitForFunction(()=>!document.querySelector('#project-dialog').open)
  assert.equal(stopped,true)
  assert.equal(await page.locator('.local-task').count(),0)
  available=false
  await page.getByText('Local agent is stopped',{exact:true}).waitFor()
  assert.equal(await page.getByRole('button',{name:'Start agent',exact:true}).isEnabled(),true)
  assert.equal(await page.getByRole('button',{name:'Start local agent',exact:true}).isEnabled(),true)
  console.log('Screenshot:',path.join(root,'console.png'))
 }finally{
  if(application)await application.close()
  for(const client of ws.clients)client.terminate()
  ws.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
