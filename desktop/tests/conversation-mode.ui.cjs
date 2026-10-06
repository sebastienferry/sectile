const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The permission mode new Claude conversations start in sits under Claude
// consoles. Desktop keeps it in its own file and hands it to an agent that
// announces it, on connection and on each change; the setting is disabled in
// the terminal view and for an agent that predates it. A new conversation's
// composer starts on the mode the agent reports.
test('the conversation permission mode is chosen under Claude consoles and handed to the agent',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-conversation-mode-ui-'))
 fs.writeFileSync(path.join(root,'desktop.json'),JSON.stringify({consoleView:'conversation',conversationMode:'auto'}))
 const chat={id:'chat',projectId:'project',kind:'console',provider:'claude',conversation:true,headless:true,status:'running',directory:'/tmp/project'}
 let capable=true,refuse=false,reported='auto'
 const sent=[]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,capabilities:['claude-conversation','conversation-controls','console-view-default',...(capable?['conversation-mode-default']:[])]}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Project',path:'/tmp/project'}]));return}
  if(req.url.startsWith('/desktop/project?')){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end('[]');return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify([chat]));return}
  if(req.url==='/desktop/console-view'&&req.method==='PUT'){req.resume();req.on('end',()=>res.end('{}'));return}
  if(req.url.startsWith('/desktop/conversation?id=chat')&&req.method==='GET'){res.end(JSON.stringify({id:'chat',events:[],version:0,busy:false,readOnly:false,mode:reported}));return}
  if(req.url==='/desktop/conversation-mode'&&req.method==='PUT'){
   let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{
    const {mode}=JSON.parse(body);sent.push(mode)
    if(refuse){res.writeHead(500).end('{}');return}
    res.end(JSON.stringify({mode}))
   });return
  }
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 const saved=()=>JSON.parse(fs.readFileSync(path.join(root,'desktop.json'),'utf8')).conversationMode
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(8000)
  // The connection hands the saved setting over.
  await expect.poll(()=>sent[0]).toBe('auto')
  // A new conversation's composer starts on the mode the agent reports.
  await page.locator('.run[data-run-id=chat]').click()
  await expect(page.locator('.conversation')).toBeVisible()
  await expect(page.getByLabel('Permission mode',{exact:true})).toHaveValue('auto')

  const openAppearance=async()=>{
   await page.locator('#settings').click()
   await page.getByRole('tab',{name:'Appearance',exact:true}).click()
  }
  await openAppearance()
  const select=page.getByRole('combobox',{name:'Conversation permission mode'})
  const row=page.locator('.setting-row').filter({has:select})
  // It sits right under Claude consoles, carries the Claude mark, and offers the composer's modes.
  await expect(page.locator('.setting-row').filter({has:page.getByRole('group',{name:'Claude consoles'})}).locator('xpath=following-sibling::section[1]')).toContainText('Conversation permission mode')
  await expect(row.locator('svg.claude-mark path')).toHaveCount(1)
  await expect(select).toHaveValue('auto')
  await expect(select).toBeEnabled()
  assert.deepEqual(await select.locator('option').allTextContents(),['Ask before edits','Accept edits','Auto mode','Plan mode'])
  // A change is saved and handed over at once.
  let count=sent.length
  await select.selectOption('plan')
  await expect.poll(()=>sent.slice(count)).toEqual(['plan'])
  assert.equal(saved(),'plan')
  // An agent that refuses it does not stop the save, nor shows an error.
  refuse=true;count=sent.length
  await select.selectOption('default')
  await expect.poll(()=>sent.slice(count)).toEqual(['default'])
  assert.equal(saved(),'default')
  await expect(page.locator('[data-tone="error"]')).toHaveCount(0)
  refuse=false
  // The terminal view opens no conversation: the setting is disabled and says why.
  const views=page.getByRole('group',{name:'Claude consoles'})
  await views.getByRole('button',{name:'Terminal',exact:true}).click()
  await expect(select).toBeDisabled()
  await expect(row).toContainText('choose Conversation above')
  await views.getByRole('button',{name:'Conversation',exact:true}).click()
  await expect(select).toBeEnabled()
  await page.keyboard.press('Escape')

  // An agent that predates the setting is sent nothing and the setting is disabled.
  capable=false;count=sent.length
  await openAppearance()
  await expect(select).toBeDisabled()
  await expect(row).toContainText('Update and restart the local agent')
  await expect(select).toHaveValue('default')
  await page.waitForTimeout(300)
  assert.deepEqual(sent.slice(count),[])
  // Every save went to Desktop's own file; the agent's settings.json was never written.
  assert.equal(fs.existsSync(path.join(root,'settings.json')),false)
 }finally{if(app)await app.close();await new Promise(resolve=>server.close(resolve))}
})
