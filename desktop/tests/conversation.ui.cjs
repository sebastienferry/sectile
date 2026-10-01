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
 let busy=false,partial='',readOnly=false,attachments=0,message='',effort='',context
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
   if(req.method==='GET'){res.end(JSON.stringify({id:'chat',events:events.map(e=>JSON.stringify(e)),version:events.length,busy,readOnly,effort,context,partial}));return}
   let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{
    ({message,effort}=JSON.parse(body));busy=true;context={used:150000,window:200000}
    events.push({kind:'user',text:message},{kind:'assistant',text:'## Result\n**Safe output**\n```html\n<img src=x onerror="window.hostile=true">\n```\n\n| Name | Value |\n| --- | --- |\n| a | 1 |\n\n- [x] done\n\n[plan](../plan.md) ![diagram](x.png)'},{kind:'tool',text:'Read',detail:'<script>window.hostile=true</script>'},{kind:'tool',text:'Edit',tool:'Edit',detail:'src/app.js',input:{file_path:'/tmp/project/src/app.js',old_string:'keep\n<b>old</b>',new_string:'keep\nnew'}},{kind:'tool',text:'Bash',tool:'Bash',toolId:'t-bash',detail:'go test',input:{command:'go test ./...',description:'Run the tests'}},{kind:'tool_result',toolId:'t-bash',text:'ok  tasks <i>1.2s</i>'},{kind:'tool',text:'Grep',tool:'Grep',toolId:'t-grep',input:{pattern:'TODO'}},{kind:'tool_result',toolId:'t-grep',text:'No matches',error:true},{kind:'tool',text:'TodoWrite',tool:'TodoWrite',input:{todos:[{content:'Read the code',status:'completed'},{content:'Fix it',status:'in_progress'}]}})
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
  // The conversation view is opt-in: the terminal is the default.
  await expect(page.getByRole('button',{name:'Claude chat (test)',exact:true})).toBeHidden()
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'Appearance',exact:true}).click()
  const views=page.getByRole('group',{name:'Claude consoles'})
  await expect(views.getByRole('button',{name:'Terminal',exact:true})).toHaveAttribute('aria-pressed','true')
  await views.getByRole('button',{name:'Conversation',exact:true}).click()
  await expect(views.getByRole('button',{name:'Conversation',exact:true})).toHaveAttribute('aria-pressed','true')
  assert.equal(JSON.parse(fs.readFileSync(path.join(root,'settings.json'),'utf8')).consoleView,'conversation')
  await page.keyboard.press('Escape')
  await page.getByRole('button',{name:'Claude chat (test)',exact:true}).click()
  await expect(page.locator('.conversation')).toBeVisible()
  await expect(page.locator('.xterm')).toBeHidden()
  await expect(page.getByRole('button',{name:'Detach to native terminal',exact:true})).toBeHidden()
  const input=page.getByLabel('Message Claude Code')
  await expect(input).toBeEnabled()
  await expect(page.locator('.conversation-context')).toBeHidden()
  await page.getByLabel('Effort',{exact:true}).selectOption('high')
  await expect(page.locator('.conversation-effort rect.lit')).toHaveCount(3)
  await input.fill('Review the project <script>window.hostile=true</script>')
  await page.getByRole('button',{name:'Send',exact:true}).click()
  await expect(page.locator('.conversation-assistant')).toContainText('Safe output')
  await expect(input).toHaveValue('')
  await expect(page.locator('#error')).toHaveText('')
  await expect(page.locator('.conversation-assistant pre code')).toHaveText('<img src=x onerror="window.hostile=true">')
  // Claude's Markdown is rendered like a changed file's: tables, task lists,
  // inert relative links and images left as their alt text.
  await expect(page.locator('.conversation-assistant h2')).toHaveText('Result')
  await expect(page.locator('.conversation-assistant table th')).toHaveText(['Name','Value'])
  await expect(page.locator('.conversation-assistant input[type=checkbox]')).toBeDisabled()
  await expect(page.locator('.conversation-assistant .md-link-inert')).toHaveText('plan')
  await expect(page.locator('.conversation-assistant .md-image')).toHaveText('[diagram] (x.png)')
  assert.equal(await page.locator('.conversation img,.conversation [src]').count(),0)
  await expect(page.locator('.conversation-user .conversation-plain')).toHaveText('Review the project <script>window.hostile=true</script>')
  const card=name=>page.locator('.tool-card').filter({has:page.locator('.tool-card-name',{hasText:name})})
  await card('Read').locator('summary').click()
  await expect(card('Read').locator('pre')).toContainText('<script>')
  // An edit is a diff, closed until asked; a todo list is open at once.
  await expect(card('Edit').locator('.tool-card-note')).toHaveText('+1 −1')
  await expect(card('Edit').locator('pre')).toBeHidden()
  await card('Edit').locator('summary').click()
  await expect(card('Edit').locator('.diff-delete')).toHaveText('- <b>old</b>')
  await expect(card('Edit').locator('.diff-add')).toHaveText('+ new')
  await expect(card('Todos').locator('.tool-card-target')).toHaveText('1 of 2 done')
  await expect(card('Todos').locator('input[type=checkbox]').first()).toBeChecked()
  assert.equal(await page.locator('.conversation b').count(),0)
  // A result is drawn in the card of the call it answers, never on its own.
  await card('Bash').locator('summary').click()
  await expect(card('Bash').locator('.tool-card-code')).toHaveText('$ go test ./...')
  await expect(card('Bash').locator('.tool-card-result')).toHaveText('ok  tasks <i>1.2s</i>')
  await expect(card('Grep')).toHaveClass(/tool-card-failed/)
  await expect(card('Grep').locator('.tool-card-status')).toHaveText('failed')
  assert.equal(await page.locator('.conversation-tool_result,.conversation i').count(),0)
  assert.equal(await page.evaluate(()=>window.hostile),undefined)
  assert.equal(attachments,before)
  assert.equal(message,'Review the project <script>window.hostile=true</script>')
  assert.equal(effort,'high')
  await expect(page.locator('.conversation-context')).toHaveAttribute('aria-label','150,000 of 200,000 context tokens used (75%)')
  await expect(page.locator('.conversation-context')).not.toHaveClass(/full/)
  await expect(page.getByLabel('Effort',{exact:true})).toHaveValue('high')
  await expect(page.getByRole('button',{name:'Send',exact:true})).toBeDisabled()
  // The reply in progress streams after the history, rendered, and leaves
  // the history untouched; it disappears once the turn ends.
  const history=await page.locator('.conversation-event').count()
  partial='Writing **the** ans'
  await expect(page.locator('.conversation-partial strong:not(.conversation-speaker)')).toHaveText('the')
  partial='Writing **the** answer\n\n- one <i>x</i>'
  await expect(page.locator('.conversation-partial li')).toHaveText('one <i>x</i>')
  await expect(page.locator('.conversation-partial')).toHaveCount(1)
  assert.equal(await page.locator('.conversation-event:not(.conversation-partial)').count(),history)
  assert.equal(await page.locator('.conversation i').count(),0)
  partial=''
  await expect(page.locator('.conversation-partial')).toHaveCount(0)
  busy=false
  await expect(page.getByRole('button',{name:'Send',exact:true})).toBeEnabled()
  await page.setViewportSize({width:1000,height:750})
  await page.screenshot({path:path.join(root,'conversation.png')})
  await page.getByRole('button',{name:'Stop execution',exact:true}).click()
  await expect(input).toBeDisabled()
  await expect(page.getByLabel('Effort',{exact:true})).toBeDisabled()
  await expect(page.locator('.conversation-status')).toHaveText('Read-only history')
  await page.locator('.run').filter({hasText:'Source execution'}).click()
  await expect(page.locator('.conversation')).toBeHidden()
  await expect(page.locator('.xterm')).toBeVisible()
  console.log('Conversation screenshot: '+path.join(root,'conversation.png'))
 }finally{if(app)await app.close();ws.close();await new Promise(resolve=>server.close(resolve))}
})
