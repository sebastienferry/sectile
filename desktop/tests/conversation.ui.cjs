const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')
const {WebSocketServer}=require('ws')
// The status indicator is the ::before of the status; idle draws none.
const indicator=(status,property)=>status.evaluate((el,property)=>getComputedStyle(el,'::before')[property],property)
const token=(page,name)=>page.evaluate(name=>{const probe=document.createElement('div');probe.style.color=`var(${name})`;document.body.append(probe);const value=getComputedStyle(probe).color;probe.remove();return value},name)

test('Claude chat renders structured output safely and sends messages without a PTY',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-conversation-ui-'))
 const source={id:'source',taskId:'source',taskKey:'#1',projectId:'project',skill:'implement',status:'completed',directory:'/tmp/project',sessionId:'source'}
 const chat={id:'chat',taskId:'chat-task',projectId:'project',kind:'task',provider:'claude',conversation:true,headless:true,status:'running',directory:'/tmp/project'}
 const runs=[source,chat],events=[{kind:'notice',text:'Claude Code conversation · experimental',detail:'Edits accepted; approvals unavailable.'}]
 let pollsWithSince=0,sentMode='',mcpState={status:'needs-auth',detail:'! Needs authentication'},mcpChecks=0,busy=false,partial='',interrupts=0,joined=[],terminals=[],approvals=[],decisions=[],readOnly=false,foreign=false,attachments=0,message='',effort='',context
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,capabilities:['claude-conversation','conversation-controls','conversation-queue']}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Project',path:'/tmp/project'}]));return}
  if(req.url.startsWith('/desktop/project?')){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end(JSON.stringify([{id:'source',key:'#1',title:'Source execution',labels:['#specified']}]));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url==='/desktop/conversation-terminal'){let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{terminals.push(JSON.parse(body).runId);res.end(JSON.stringify({opened:true}))});return}
  if(req.url.startsWith('/desktop/conversation?id=chat')){
   // Like the agent, an unchanged history is not sent to a poll that shows it.
   const since=new URL(req.url,'http://localhost').searchParams.get('since')
   if(since!==null)pollsWithSince++
   if(req.method==='GET'){res.end(JSON.stringify({id:foreign?'other':'chat',...(since===String(events.length)?{}:{events:events.map(e=>JSON.stringify(e))}),version:events.length,busy,readOnly,effort,context,partial,approvals,sectileMcp:mcpState,commands:[{name:'clarify-issue',description:'Clarify a <b>ticket</b>',argumentHint:'<KEY>'},{name:'specify-issue',description:'Write the spec'},{name:'code-review'}]}));return}
   let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{
    const sent=JSON.parse(body)
    if(sent.checkMcp){mcpChecks++;mcpState={status:'connected',detail:'✔ Connected'};res.writeHead(202).end(JSON.stringify({accepted:true}));return}
    if(sent.approval){decisions.push(sent.approval);const asked=approvals.find(item=>item.id===sent.approval.id);approvals=approvals.filter(item=>item.id!==sent.approval.id);events.push({kind:'approval',text:sent.approval.decision,tool:asked.tool,toolId:asked.toolUseId});res.end(JSON.stringify({accepted:true}));return}
    if(busy&&sent.message){joined.push(sent.message);events.push({kind:'user',text:sent.message});res.writeHead(202).end(JSON.stringify({accepted:true,joined:true}));return}
    if(JSON.parse(body).interrupt){interrupts++;busy=false;partial='';events.push({kind:'notice',text:'Interrupted'});res.writeHead(202).end(JSON.stringify({accepted:true}));return}
    ({message,effort,mode:sentMode}=JSON.parse(body));busy=true;context={used:150000,window:200000}
    events.push({kind:'user',text:message},{kind:'command_output',text:'Current session: 5% used\n  65% of your usage\nsectile - ✔ Connected\nwiz - ! Needs authentication\nold - ✗ Failed <b>x</b>'},{kind:'assistant',text:'## Result\n**Safe output**\n```html\n<img src=x onerror="window.hostile=true">\n```\n\n| Name | Value |\n| --- | --- |\n| a | 1 |\n\n- [x] done\n\n[plan](../plan.md) ![diagram](x.png)'},{kind:'tool',text:'Read',detail:'<script>window.hostile=true</script>'},{kind:'tool',text:'Edit',tool:'Edit',detail:'src/app.js',input:{file_path:'/tmp/project/src/app.js',old_string:'keep\n<b>old</b>',new_string:'keep\nnew'}},{kind:'tool',text:'Bash',tool:'Bash',toolId:'t-bash',detail:'go test',input:{command:'go test ./...',description:'Run the tests'}},{kind:'tool_result',toolId:'t-bash',text:'ok  tasks <i>1.2s</i>'},{kind:'tool',text:'Grep',tool:'Grep',toolId:'t-grep',input:{pattern:'TODO'}},{kind:'tool_result',toolId:'t-grep',text:'No matches',error:true},{kind:'tool',text:'TodoWrite',tool:'TodoWrite',input:{todos:[{content:'Read the code',status:'completed'},{content:'Fix it',status:'in_progress'}]}})
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
  await expect(page.getByRole('button',{name:'Claude chat (test)',exact:true})).toHaveCount(0)
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'Appearance',exact:true}).click()
  const views=page.getByRole('group',{name:'Claude consoles'})
  await expect(views.getByRole('button',{name:'Terminal',exact:true})).toHaveAttribute('aria-pressed','true')
  await views.getByRole('button',{name:'Conversation',exact:true}).click()
  await expect(views.getByRole('button',{name:'Conversation',exact:true})).toHaveAttribute('aria-pressed','true')
  assert.equal(JSON.parse(fs.readFileSync(path.join(root,'desktop.json'),'utf8')).consoleView,'conversation')
  await page.keyboard.press('Escape')
  await page.locator('.run[data-run-id=chat]').click()
  await expect(page.locator('.conversation')).toBeVisible()
  await page.locator('#rerun').click()
  await expect(page.getByRole('combobox',{name:'Relaunch skill'})).toBeVisible()
  await page.getByRole('combobox',{name:'Relaunch skill'}).selectOption('discuss')
  await page.keyboard.press('Escape')
  await expect(page.locator('.xterm')).toBeHidden()
  await expect(page.getByRole('button',{name:'Detach to native terminal',exact:true})).toBeHidden()
  const input=page.getByLabel('Message Claude Code')
  await expect(input).toBeEnabled()
  await expect(page.locator('.conversation-context')).toBeHidden()
  // Polls send the version they show, so an unchanged history is not resent.
  await expect.poll(()=>pollsWithSince).toBeGreaterThan(0)
  // The Sectile MCP chip says whether Claude reaches Sectile, and checks again.
  const mcpChip=page.locator('.conversation-mcp')
  await expect(mcpChip).toHaveAttribute('data-status','needs-auth')
  await expect(mcpChip).toHaveAttribute('aria-label','Sectile MCP: needs authentication')
  await mcpChip.click()
  await expect.poll(()=>mcpChecks).toBe(1)
  await expect(mcpChip).toHaveAttribute('data-status','connected')
  // Slash commands complete from what Claude offers, without sending.
  const commandList=page.getByRole('listbox',{name:'Slash commands'})
  await input.fill('/iss')
  await expect(commandList.getByRole('option')).toHaveCount(2)
  await input.fill('/cla')
  await expect(commandList.getByRole('option')).toHaveCount(1)
  await expect(commandList.getByRole('option')).toContainText('/clarify-issue<KEY>Clarify a <b>ticket</b>')
  assert.equal(await page.locator('.conversation-commands b').count(),0)
  await input.fill('/')
  await expect(commandList.getByRole('option')).toHaveCount(3)
  await input.press('ArrowDown');await input.press('Enter')
  await expect(input).toHaveValue('/specify-issue ')
  await expect(commandList).toBeHidden()
  await input.fill('/co');await expect(commandList).toBeVisible()
  await input.press('Escape');await expect(commandList).toBeHidden();await expect(input).toHaveValue('/co')
  // A message starting with ! is marked as a shell command.
  await input.fill('!ls');await expect(page.locator('.conversation-composer')).toHaveClass(/conversation-shell-mode/)
  await input.fill('');await expect(page.locator('.conversation-composer')).not.toHaveClass(/conversation-shell-mode/)
  await page.getByLabel('Effort',{exact:true}).selectOption('high')
  await expect(page.getByLabel('Permission mode',{exact:true})).toHaveValue('acceptEdits')
  await page.getByLabel('Permission mode',{exact:true}).selectOption('plan')
  await expect(page.locator('.conversation-effort rect.lit')).toHaveCount(3)
  await input.fill('Review the project <script>window.hostile=true</script>')
  await page.getByRole('button',{name:'Send',exact:true}).click()
  await expect(page.locator('.conversation-assistant')).toContainText('Safe output')
  await expect(input).toHaveValue('')
  await expect(page.locator('#error')).toHaveText('')
  // While Claude works, a dot pulses before the status, still under reduced motion.
  const status=page.locator('.conversation-status')
  await expect(status).toHaveText('Claude Code is working…')
  await expect(status).toHaveAttribute('data-kind','working')
  assert.equal(await indicator(status,'animationName'),'run-state-pulse')
  await page.emulateMedia({reducedMotion:'reduce'})
  assert.equal(await indicator(status,'animationName'),'none')
  assert.notEqual(await indicator(status,'content'),'none')
  await page.emulateMedia({reducedMotion:'no-preference'})
  assert.equal(await indicator(status,'animationName'),'run-state-pulse')
  // An error is idle even while Claude works.
  foreign=true
  await expect(status).toHaveText('The agent returned another conversation.')
  await expect(status).toHaveAttribute('data-kind','idle')
  assert.equal(await indicator(status,'content'),'none')
  foreign=false
  await expect(status).toHaveText('Claude Code is working…')
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
  // A local command's output keeps its layout, its status lines tinted.
  const output=page.locator('.conversation-command-output')
  await expect(output).toContainText('Current session: 5% used\n  65% of your usage')
  await expect(output.locator('.command-ok')).toHaveText('sectile - ✔ Connected')
  await expect(output.locator('.command-warn')).toHaveText('wiz - ! Needs authentication')
  await expect(output.locator('.command-failed')).toHaveText('old - ✗ Failed <b>x</b>')
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
  assert.equal(sentMode,'plan')
  await expect(page.locator('.conversation-context')).toHaveAttribute('aria-label','150,000 of 200,000 context tokens used (75%)')
  await expect(page.locator('.conversation-context')).not.toHaveClass(/full/)
  await expect(page.getByLabel('Effort',{exact:true})).toHaveValue('high')
  // While Claude answers, Stop answer appears and a message can still be sent.
  await expect(page.getByRole('button',{name:'Stop answer',exact:true})).toBeVisible()
  await expect(page.getByRole('button',{name:'Send',exact:true})).toBeEnabled()
  await input.fill('Also check the docs')
  await input.press('Enter')
  await expect.poll(()=>joined).toEqual(['Also check the docs'])
  await expect(input).toHaveValue('')
  await expect(page.locator('.conversation-user').last()).toContainText('Also check the docs')
  await page.getByRole('button',{name:'Open a terminal',exact:true}).click()
  await expect.poll(()=>terminals).toEqual(['chat'])
  // A tool call Claude may not make waits in its card for the owner.
  approvals=[{id:'req-1',toolUseId:'t-bash',tool:'Bash',description:'Run the tests',reason:'This command requires approval',input:{command:'go test ./...'},suggestions:[{type:'addRules',behavior:'allow',rules:[{toolName:'Bash',ruleContent:'go test *'}]}]},{id:'req-2',tool:'mcp__tracker__close_issue',input:{key:'<b>#1</b>'}}]
  const asking=page.locator('.tool-card-asking')
  await expect(asking).toHaveCount(2)
  await expect(page.locator('.conversation-status')).toHaveText('Waiting for your approval')
  // Claude waits for the owner: the status and a still dot take the waiting colour.
  const waiting=await token(page,'--waiting')
  await expect(status).toHaveAttribute('data-kind','asking')
  await expect(status).toHaveCSS('color',waiting)
  assert.equal(await indicator(status,'animationName'),'none')
  assert.equal(await indicator(status,'backgroundColor'),waiting)
  await expect(card('Bash').getByText('This command requires approval',{exact:true})).toBeVisible()
  await card('Bash').getByText('Always allow: proposed access for this project',{exact:true}).click()
  await expect(card('Bash').locator('.tool-approval pre')).toContainText('project (this workstation)')
  await expect(card('Bash').getByRole('button',{name:'Always allow',exact:true})).toBeVisible()
  await expect(card('close_issue').getByRole('button',{name:'Always allow',exact:true})).toHaveCount(0)
  await expect(card('close_issue').locator('.tool-approval-question')).toHaveText('Allow mcp__tracker__close_issue?')
  assert.equal(await page.locator('.conversation b').count(),0)
  await card('Bash').getByRole('button',{name:'Always allow',exact:true}).click()
  await expect.poll(()=>decisions).toEqual([{id:'req-1',decision:'always'}])
  await expect(card('Bash').locator('.tool-card-status')).toHaveText('always allowed')
  await card('close_issue').getByRole('button',{name:'Deny',exact:true}).click()
  await expect.poll(()=>decisions.length).toBe(2)
  await expect(asking).toHaveCount(0)
  await expect(card('close_issue')).toHaveCount(0)
  // Claude's question is answered in its card, one choice or one's own words.
  approvals=[{id:'ask-1',tool:'AskUserQuestion',input:{questions:[{question:'Which color?',header:'Color',options:[{label:'Red',description:'Warm <b>tone</b>'},{label:'Blue'}],multiSelect:false},{question:'Which days?',options:[{label:'Mon'},{label:'Tue'}],multiSelect:true}]}}]
  const question=page.locator('.tool-question')
  await expect(question).toBeVisible()
  await expect(page.locator('.conversation-status')).toHaveText('Claude is asking you a question')
  await expect(status).toHaveAttribute('data-kind','asking')
  await expect(status).toHaveCSS('color',waiting)
  assert.equal(await indicator(status,'animationName'),'none')
  await expect(question.locator('input[type=radio]')).toHaveCount(2)
  await expect(question.locator('input[type=checkbox]')).toHaveCount(2)
  await expect(question.locator('small')).toHaveText('Warm <b>tone</b>')
  await question.getByRole('button',{name:'Answer',exact:true}).click()
  await expect(question.locator('.tool-question-hint')).toHaveText('Answer every question')
  await question.getByLabel('Blue').check()
  await question.getByLabel('Mon').check();await question.getByLabel('Tue').check()
  await question.getByLabel('Other answer to: Which days?').fill('Fri')
  await question.getByRole('button',{name:'Answer',exact:true}).click()
  await expect.poll(()=>decisions.at(-1)).toEqual({id:'ask-1',decision:'answer',answers:{'Which color?':'Blue','Which days?':'Mon, Tue, Fri'}})
  await expect(question).toHaveCount(0)
  // Once answered, Claude resumes and the dot pulses again.
  await expect(status).toHaveText('Claude Code is working…')
  assert.equal(await indicator(status,'animationName'),'run-state-pulse')
  assert.equal(await page.locator('.conversation b').count(),0)
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
  // Stop answer ends the turn and keeps the conversation open.
  await page.getByRole('button',{name:'Stop answer',exact:true}).click()
  await expect.poll(()=>interrupts).toBe(1)
  await expect(page.getByRole('button',{name:'Send',exact:true})).toBeEnabled()
  await expect(page.getByRole('button',{name:'Stop answer',exact:true})).toBeHidden()
  await expect(input).toBeEnabled()
  await expect(status).toHaveText('Ready')
  await expect(status).toHaveAttribute('data-kind','idle')
  assert.equal(await indicator(status,'content'),'none')
  await page.setViewportSize({width:1000,height:750})
  await page.screenshot({path:path.join(root,'conversation.png')})
  await page.getByRole('button',{name:'Stop execution',exact:true}).click()
  await expect(input).toBeDisabled()
  await expect(page.getByLabel('Effort',{exact:true})).toBeDisabled()
  await expect(page.locator('.conversation-status')).toHaveText('Read-only history')
  await expect(status).toHaveAttribute('data-kind','idle')
  assert.equal(await indicator(status,'content'),'none')
  await page.locator('.run').filter({hasText:'Source execution'}).click()
  await expect(page.locator('.conversation')).toBeHidden()
  await expect(page.locator('.xterm')).toBeVisible()
  console.log('Conversation screenshot: '+path.join(root,'conversation.png'))
 }finally{if(app)await app.close();ws.close();await new Promise(resolve=>server.close(resolve))}
})
