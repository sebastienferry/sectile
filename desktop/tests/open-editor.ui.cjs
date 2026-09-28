const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')
const {WebSocketServer}=require('ws')

// The worktree line offers the configured editor (#535): the button exists once
// an editor is chosen and the agent can open one, and it names the run, never
// the path, so the agent decides which folder is opened.
test('the worktree opens in the configured editor from the toolbar',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-open-editor-'))
 const worktree='/tmp/example/worktree-535'
 const state={editor:'cursor',capabilities:['task-engines','open-editor'],opened:[],refuse:'',puts:[]}
 const runs=[{id:'current',projectId:'project',taskId:'a',taskKey:'#535',sessionId:'current',skill:'implement',status:'running',directory:worktree,createdAt:'2026-09-26T10:00:00Z'}]
 const body=req=>new Promise(resolve=>{let data='';req.on('data',chunk=>data+=chunk);req.on('end',()=>resolve(JSON.parse(data||'{}')))})
 const server=http.createServer(async(req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:state.capabilities}));return}
  if(url.pathname==='/desktop/workstation'&&req.method==='PUT'){const input=await body(req);state.puts.push(input);state.editor=input.editorCommand||'';res.writeHead(204).end();return}
  if(url.pathname==='/desktop/workstation'){res.end(JSON.stringify({defaults:state.editor?{editorCommand:state.editor}:{},effective:{useWorktrees:true,parallelism:1,aiProviderModels:{}},providerModels:{},setupProviders:['claude'],seeded:{}}));return}
  if(url.pathname==='/desktop/engines'){res.end(JSON.stringify({catalogue:[{id:'e',name:'Claude',provider:'claude'}],default:'e',providers:['claude'],providerModels:{},projects:{},taskCounts:{}}));return}
  if(url.pathname==='/desktop/open-editor'&&req.method==='POST'){
   const input=await body(req)
   if(state.refuse){res.writeHead(410,{'Content-Type':'text/plain'}).end(state.refuse+'\n');return}
   state.opened.push(input);res.end(JSON.stringify({editor:state.editor,directory:worktree}));return
  }
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Example project',path:'/tmp/example',configured:true}]));return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{skills:[{id:'implement',name:'Implement'}]}}));return}
  if(url.pathname==='/desktop/run-result'){res.end(JSON.stringify({activity:null}));return}
  if(url.pathname==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(url.pathname==='/desktop/tasks'){res.end(JSON.stringify([{id:'a',key:'#535',title:'Allow to open code',labels:['#specified']}]));return}
  if(url.pathname==='/desktop/version'){res.end('{"version":"test"}');return}
  res.writeHead(404).end('{}')
 })
 const ws=new WebSocketServer({noServer:true})
 server.on('upgrade',(req,socket,head)=>ws.handleUpgrade(req,socket,head,client=>client.send(Buffer.from('Console output\r\n'))))
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}),{mode:0o600})
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  const open=page.locator('#open-editor')

  // A chosen editor puts a named button right after the path, which still copies.
  await expect(page.locator('#directory')).toHaveText(worktree)
  await expect(open).toBeVisible()
  await expect(open).toHaveAttribute('aria-label','Open in Cursor')
  await expect(open).toHaveAttribute('title','Open in Cursor')
  assert.ok(await page.evaluate(()=>document.querySelector('#worktree').nextElementSibling===document.querySelector('#open-editor')),'The button follows the path')
  await expect(page.locator('#worktree')).toHaveAttribute('title','Copy this path')

  // A click names the selected run; the agent resolves the folder.
  await open.click()
  await expect.poll(()=>state.opened.length).toBe(1)
  assert.deepEqual(state.opened[0],{runId:'current'})

  // A refusal is shown as the agent gave it.
  state.refuse='The worktree no longer exists: '+worktree
  await open.click()
  await expect(page.locator('#error')).toHaveText('The worktree no longer exists: '+worktree)
  state.refuse=''
  assert.equal(state.opened.length,1,'A refused open is not recorded as opened')

  // Without an editor the path stands alone, as before.
  state.editor='';await page.reload()
  await expect(page.locator('#directory')).toHaveText(worktree)
  await expect(open).toBeHidden()

  // Choosing a preset and saving brings the button back, without a restart.
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'Execution defaults',exact:true}).click()
  const panel=page.locator('.execution-defaults')
  const picker=panel.getByRole('combobox',{name:'Editor',exact:true})
  await expect(picker).toHaveValue('')
  await picker.selectOption({label:'VS Code'})
  await panel.getByRole('button',{name:'Save execution defaults'}).click()
  await expect(panel.locator('.workstation-notice')).toContainText('Execution defaults saved')
  assert.equal(state.puts.at(-1).editorCommand,'code')
  await expect(open).toHaveAttribute('aria-label','Open in VS Code')
  await expect(open).not.toHaveAttribute('hidden','')

  // Back to None: the button goes away again.
  await picker.selectOption({label:'None'})
  await panel.getByRole('button',{name:'Save execution defaults'}).click()
  await expect.poll(()=>state.puts.length).toBe(2)
  assert.equal(state.puts[1].editorCommand,undefined,'None stores no editor')
  await expect(open).toHaveAttribute('hidden','')

  // An agent that cannot open an editor gets no button, whatever the settings.
  state.editor='zed';state.capabilities=['task-engines'];await page.reload()
  await expect(page.locator('#directory')).toHaveText(worktree)
  await expect(open).toBeHidden()
 }finally{
  if(app)await app.close()
  for(const client of ws.clients)client.terminate()
  await new Promise(resolve=>ws.close(resolve))
  await new Promise(resolve=>server.close(resolve))
 }
})
