const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')
const {WebSocketServer}=require('ws')

test('Claude chat renders structured output safely and sends messages without a PTY',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-conversation-ui-'))
 const source={id:'source',taskId:'source',taskKey:'#1',projectId:'project',skill:'implement',status:'completed',directory:'/tmp/project',sessionId:'source'}
 const chat={id:'chat',taskId:'',projectId:'project',kind:'console',provider:'claude',conversation:true,headless:true,status:'running',directory:'/tmp/project'}
 const runs=[source],events=[{kind:'notice',text:'Claude Code conversation · experimental',detail:'Edits accepted; approvals unavailable.'}]
 let busy=false,readOnly=false,attachments=0,message=''
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,capabilities:['claude-conversation']}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Project',path:'/tmp/project'}]));return}
  if(req.url.startsWith('/desktop/project?')){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end(JSON.stringify([{id:'source',key:'#1',title:'Source execution',labels:['#specified']}]));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url==='/desktop/conversation'&&req.method==='POST'){
   let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{assert.equal(JSON.parse(body).sourceRunId,'source');runs.push(chat);res.writeHead(201).end(JSON.stringify(chat))});return
  }
  if(req.url==='/desktop/conversation?id=chat'){
   if(req.method==='GET'){res.end(JSON.stringify({id:'chat',events:events.map(e=>JSON.stringify(e)),version:events.length,busy,readOnly}));return}
   let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{
    message=JSON.parse(body).message;busy=true
    events.push({kind:'user',text:message},{kind:'assistant',text:'## Result\n**Safe output**\n```html\n<img src=x onerror="window.hostile=true">\n```'},{kind:'tool',text:'Read',detail:'<script>window.hostile=true</script>'})
    res.writeHead(202).end(JSON.stringify({accepted:true}))
   });return
  }
  if(req.url==='/desktop/stop?id=chat'){readOnly=true;busy=false;chat.status='canceled';res.writeHead(204).end();return}
  res.writeHead(404).end()
 })
 const ws=new WebSocketServer({server});ws.on('connection',socket=>{attachments++;socket.send('source console\r\n')})
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(8000)
  await page.locator('.run').filter({hasText:'Source execution'}).click()
  await expect.poll(()=>attachments).toBeGreaterThan(0)
  const before=attachments
  await page.getByRole('button',{name:'Claude chat (test)',exact:true}).click()
  await expect(page.locator('.conversation')).toBeVisible()
  await expect(page.locator('.xterm')).toBeHidden()
  await expect(page.getByRole('button',{name:'Detach to native terminal',exact:true})).toBeHidden()
  const input=page.getByLabel('Message Claude Code')
  await expect(input).toBeEnabled()
  await input.fill('Review the project <script>window.hostile=true</script>')
  await page.getByRole('button',{name:'Send',exact:true}).click()
  await expect(page.locator('.conversation-assistant')).toContainText('Safe output')
  await expect(input).toHaveValue('')
  await expect(page.locator('#error')).toHaveText('')
  await expect(page.locator('.conversation-assistant pre code')).toHaveText('<img src=x onerror="window.hostile=true">')
  await page.locator('.conversation-tool summary').click()
  await expect(page.locator('.conversation-tool pre')).toContainText('<script>')
  assert.equal(await page.evaluate(()=>window.hostile),undefined)
  assert.equal(attachments,before)
  assert.equal(message,'Review the project <script>window.hostile=true</script>')
  await expect(page.getByRole('button',{name:'Send',exact:true})).toBeDisabled()
  busy=false
  await expect(page.getByRole('button',{name:'Send',exact:true})).toBeEnabled()
  await page.setViewportSize({width:1000,height:750})
  await page.screenshot({path:path.join(root,'conversation.png')})
  await page.getByRole('button',{name:'Stop execution',exact:true}).click()
  await expect(input).toBeDisabled()
  await expect(page.locator('.conversation-status')).toHaveText('Read-only history')
  await page.locator('.run').filter({hasText:'Source execution'}).click()
  await expect(page.locator('.conversation')).toBeHidden()
  await expect(page.locator('.xterm')).toBeVisible()
  console.log('Conversation screenshot: '+path.join(root,'conversation.png'))
 }finally{if(app)await app.close();ws.close();await new Promise(resolve=>server.close(resolve))}
})
