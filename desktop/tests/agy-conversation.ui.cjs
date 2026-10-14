const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('Antigravity conversation offers models, efforts, permission modes, slash skills and tool cards',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-agy-ui-'))
 const chat={id:'agy-chat',projectId:'p',kind:'console',provider:'agy',conversation:true,headless:true,status:'running',directory:'/tmp/project',model:'gemini-3.8-pro'}
 const events=[
  {kind:'notice',text:'Antigravity conversation · experimental',detail:'Workspace edits are accepted by default.'},
  {kind:'assistant',text:'**Antigravity response**'},
  {kind:'tool',tool:'Edit',text:'Edit',toolId:'agy-step-2',input:{file_path:'/tmp/project/app.js',old_string:'old',new_string:'new'}},
  {kind:'tool_result',toolId:'agy-step-2',text:'Replaced 1 occurrence'}
 ]
 let busy=false,messages=[]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,capabilities:['claude-conversation','codex-conversation','agy-conversation','conversation-controls','conversation-queue']}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'p',name:'Project',path:'/tmp/project'}]));return}
  if(req.url.startsWith('/desktop/project?')){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end('[]');return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify([chat]));return}
  if(req.url.startsWith('/desktop/conversation?id=agy-chat')){
   if(req.method==='GET'){res.end(JSON.stringify({id:'agy-chat',provider:'agy',events:events.map(e=>JSON.stringify(e)),version:events.length,busy,mode:'accept-edits',model:'gemini-3.8-pro',effort:'',readOnly:false,commands:[{name:'clarify-issue',description:'Clarify a ticket'},{name:'help',description:'Show available commands'}],sectileMcp:{status:'connected'},approvals:[]}));return}
   let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{
    const input=JSON.parse(body)
    messages.push(input);events.push({kind:'user',text:input.message});busy=false
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
  await page.locator('.run').filter({hasText:'Antigravity'}).click()
  const input=page.getByLabel('Message Antigravity',{exact:true})
  await expect(input).toBeEnabled()
  await expect(page.locator('.conversation-assistant')).toContainText('Antigravity response')
  await expect(page.locator('.conversation-assistant .conversation-speaker').first()).toHaveText('Antigravity')
  await expect(page.getByLabel('Permission mode',{exact:true})).toHaveValue('accept-edits')
  assert.deepEqual(await page.getByLabel('Permission mode',{exact:true}).locator('option').allTextContents(),['Ask / follow rules','Accept edits','Auto-approve all tools','Plan mode'])
  assert.deepEqual(await page.getByLabel('Effort',{exact:true}).locator('option').allTextContents(),['Default effort','Low','Medium','High','Xhigh','Max'])
  const diff=page.locator('.tool-card').filter({hasText:'Edit'})
  await diff.locator('summary').click()
  await expect(diff.locator('.diff-add')).toContainText('new')
  await input.fill('/cl')
  const completion=page.getByRole('listbox',{name:'Slash commands'})
  await expect(completion.getByRole('option')).toContainText('/clarify-issue')
  await input.press('Enter');await expect(input).toHaveValue('/clarify-issue ')
  await page.getByLabel('Effort',{exact:true}).selectOption('high')
  await page.getByLabel('Permission mode',{exact:true}).selectOption('auto')
  await input.fill('/clarify-issue #42');await page.getByRole('button',{name:'Send',exact:true}).click()
  await expect.poll(()=>messages.length).toBe(1)
  assert.equal(messages[0].model,'gemini-3.8-pro')
  assert.equal(messages[0].mode,'auto')
  assert.equal(messages[0].effort,'high')
  assert.equal(messages[0].message,'/clarify-issue #42')
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
