const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('Codex conversation offers native models, efforts, skills and session approvals',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-codex-ui-'))
 const chat={id:'codex-chat',projectId:'p',kind:'console',provider:'codex',conversation:true,headless:true,status:'running',directory:'/tmp/project',model:'codex-ui'}
 const events=[{kind:'assistant',text:'**Codex response**'},{kind:'tool',tool:'Codex file changes',text:'Codex file changes',toolId:'file',input:{changes:[{path:'/tmp/project/app.js',diff:'-old\n+new'}]}},{kind:'tool_result',toolId:'file',text:'completed'}]
 let busy=false,approvals=[],messages=[],decisions=[]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,capabilities:['claude-conversation','codex-conversation','conversation-controls','conversation-queue']}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'p',name:'Project',path:'/tmp/project'}]));return}
  if(req.url.startsWith('/desktop/project?')){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end('[]');return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify([chat]));return}
  if(req.url.startsWith('/desktop/conversation?id=codex-chat')){
   if(req.method==='GET'){res.end(JSON.stringify({id:'codex-chat',provider:'codex',events:events.map(e=>JSON.stringify(e)),version:events.length,busy,mode:'workspace-write',model:'codex-ui',effort:'',readOnly:false,models:[{model:'codex-ui',efforts:['low','high']},{model:'codex-other',efforts:['minimal','medium']}],commands:[{name:'$code-issue',description:'Implement a ticket'},{name:'mcp',description:'Check MCP servers'}],sectileMcp:{status:'connected'},approvals}));return}
   let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{
    const input=JSON.parse(body)
    if(input.approval){decisions.push(input.approval);approvals=[];busy=false;events.push({kind:'approval',tool:'Bash',toolId:'bash',text:input.approval.decision})}
    else{messages.push(input);events.push({kind:'user',text:input.message});busy=true;approvals=[{id:'codex-7',tool:'Bash',toolUseId:'bash',input:{command:'git status'},choices:[{decision:'allow',label:'Allow'},{decision:'always',label:'Allow for this conversation'},{decision:'deny',label:'Deny'}]}]}
    res.writeHead(202).end(JSON.stringify({accepted:true}))
   });return
  }
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(8000)
  await page.locator('.run').filter({hasText:'Codex'}).click()
  const input=page.getByLabel('Message Codex',{exact:true})
  await expect(input).toBeEnabled()
  await expect(page.locator('.conversation-assistant')).toContainText('Codex response')
  await expect(page.locator('.conversation-speaker').first()).toHaveText('Codex')
  await expect(page.getByLabel('Permission mode',{exact:true})).toHaveValue('workspace-write')
  assert.deepEqual(await page.getByLabel('Permission mode',{exact:true}).locator('option').allTextContents(),['Read only','Workspace edits','Plan mode'])
  await expect.poll(()=>page.getByLabel('Model',{exact:true}).locator('option').allTextContents()).toEqual(['codex-ui','codex-other'])
  assert.deepEqual(await page.getByLabel('Effort',{exact:true}).locator('option').allTextContents(),['Default effort','Low','High'])
  await page.getByLabel('Model',{exact:true}).selectOption('codex-other')
  assert.deepEqual(await page.getByLabel('Effort',{exact:true}).locator('option').allTextContents(),['Default effort','Minimal','Medium'])
  const diff=page.locator('.tool-card').filter({hasText:'Codex file changes'})
  await diff.locator('summary').click()
  await expect(diff.locator('.tool-card-diff')).toContainText('app.js')
  await expect(diff.locator('.diff-add')).toContainText('new')
  await input.fill('$co')
  const completion=page.getByRole('listbox',{name:'Slash commands'})
  await expect(completion.getByRole('option')).toContainText('$code-issue')
  await input.press('Enter');await expect(input).toHaveValue('$code-issue ')
  await page.getByLabel('Effort',{exact:true}).selectOption('medium')
  await page.getByLabel('Permission mode',{exact:true}).selectOption('plan')
  await input.fill('$code-issue #42');await page.getByRole('button',{name:'Send',exact:true}).click()
  await expect.poll(()=>messages.length).toBe(1)
  assert.equal(messages[0].model,'codex-other');assert.equal(messages[0].mode,'plan');assert.equal(messages[0].effort,'medium')
  await expect(page.locator('.conversation-status')).toHaveText('Waiting for your approval')
  await expect(page.getByRole('button',{name:'Always allow',exact:true})).toHaveCount(0)
  await page.getByRole('button',{name:'Allow for this conversation',exact:true}).click()
  await expect.poll(()=>decisions.length).toBe(1)
  assert.deepEqual(decisions[0],{id:'codex-7',decision:'always'})
  await expect(page.locator('.conversation-status')).toHaveText('Ready')
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
