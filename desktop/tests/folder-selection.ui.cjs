const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')
const {WebSocketServer}=require('ws')

// With an agent that serves another folder of a run (#784), the chevron after
// the path selects a folder instead of copying it: the path, its copy, the
// editor and Changes then speak of that folder. The primary folder is the
// default, the choice is kept per execution, and a folder that leaves the list
// gives the toolbar back to the primary folder.
test('the chosen folder drives the path, the editor and Changes',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-folder-selection-'))
 const worktree='/tmp/example/.tasks/worktrees/issue-784',docs='/tmp/example-docs',notes='/tmp/notes'
 const folders=[
  {path:worktree,name:'sectile',role:'primary'},
  {path:docs,name:'docs',role:'context'},
  {path:notes,name:'notes',role:'local',attached:true}
 ]
 const runs=[
  {id:'several',projectId:'project',taskId:'a',taskKey:'#784',sessionId:'several',skill:'implement',status:'running',directory:worktree,folders,createdAt:'2026-10-07T10:00:00Z'},
  {id:'other',projectId:'project',taskId:'b',taskKey:'#785',sessionId:'other',skill:'implement',status:'running',directory:'/tmp/example/.tasks/worktrees/issue-785',createdAt:'2026-10-07T09:00:00Z'}
 ]
 const state={diffs:[],opened:[]}
 const payload=(id,folder)=>({runId:id,taskId:'a',directory:folder||worktree,branch:folder?'main':'feat/784',baseRef:'refs/remotes/origin/main',mergeBase:'1234567',generatedAt:'2026-10-07T10:00:00Z',complete:true,countsPartial:false,isClean:false,filesChanged:1,additions:1,deletions:0,warnings:[],files:[{path:(folder?'docs':'app')+'.txt',status:'added',kind:'text',additions:1,deletions:0,patch:'@@ -0,0 +1 @@\n+line'}]})
 const body=req=>new Promise(resolve=>{let data='';req.on('data',chunk=>data+=chunk);req.on('end',()=>resolve(JSON.parse(data||'{}')))})
 const server=http.createServer(async(req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:['task-engines','git-diff','open-editor','folder-selection']}));return}
  if(url.pathname==='/desktop/workstation'){res.end(JSON.stringify({defaults:{editorCommand:'cursor'},effective:{useWorktrees:true,parallelism:1,aiProviderModels:{}},providerModels:{},setupProviders:['claude'],seeded:{}}));return}
  if(url.pathname==='/desktop/engines'){res.end(JSON.stringify({catalogue:[{id:'e',name:'Claude',provider:'claude'}],default:'e',providers:['claude'],providerModels:{},projects:{},taskCounts:{}}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Example project',path:'/tmp/example',configured:true}]));return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{skills:[{id:'implement',name:'Implement'}]}}));return}
  if(url.pathname==='/desktop/run-result'){res.end(JSON.stringify({activity:null}));return}
  if(url.pathname==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(url.pathname==='/desktop/tasks'){res.end(JSON.stringify([{id:'a',key:'#784',title:'Folders',labels:['#specified']},{id:'b',key:'#785',title:'One folder',labels:['#specified']}]));return}
  if(url.pathname==='/desktop/version'){res.end('{"version":"test"}');return}
  if(url.pathname==='/desktop/git-diff'){
   const id=url.searchParams.get('id'),folder=url.searchParams.get('folder')
   state.diffs.push({id,folder})
   if(folder===notes){res.writeHead(409).end(JSON.stringify({error:{code:'not_a_repository',message:'This folder is not a Git repository.'}}));return}
   res.end(JSON.stringify(payload(id,folder)));return
  }
  if(url.pathname==='/desktop/open-editor'&&req.method==='POST'){const input=await body(req);state.opened.push(input);res.end(JSON.stringify({editor:'cursor',directory:input.folder||worktree}));return}
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
  await app.evaluate(({clipboard})=>clipboard.writeText('untouched'))
  const chevron=page.locator('#worktree-folders'),menu=page.locator('#worktree-folders-menu'),directory=page.locator('#directory')
  const items=menu.getByRole('menuitemradio')

  // The primary folder is selected by default, and Changes reads it as before.
  await expect(directory).toHaveText(worktree)
  await expect(page.locator('#open-editor')).toBeVisible()
  await page.getByRole('button',{name:'Changes',exact:true}).click()
  await expect(page.locator('.diff-status')).toHaveText('Changes loaded.')
  await expect(page.locator('.diff-context')).toContainText(worktree+' · feat/784')
  assert.deepEqual(state.diffs.at(-1),{id:'several',folder:null})

  // The menu is a single choice, the selected folder checked and focused.
  await chevron.click()
  await expect(items).toHaveCount(3)
  await expect(items.nth(0)).toHaveAttribute('aria-checked','true')
  await expect(items.nth(1)).toHaveAttribute('aria-checked','false')
  await expect(items.nth(0)).toBeFocused()
  await expect(items.nth(1)).toHaveAttribute('title','Show '+docs)

  // Choosing a folder selects it: no copy, the path shows it, Changes reloads it.
  await items.nth(1).click()
  await expect(menu).toBeHidden()
  await expect(directory).toHaveText(docs)
  assert.equal(await app.evaluate(({clipboard})=>clipboard.readText()),'untouched')
  await expect(page.locator('.diff-context')).toContainText(docs+' · main')
  await expect(page.getByRole('combobox',{name:'Changed file'})).toHaveValue('docs.txt')
  assert.deepEqual(state.diffs.at(-1),{id:'several',folder:docs})
  await chevron.click()
  await expect(items.nth(1)).toHaveAttribute('aria-checked','true')
  await expect(items.nth(1)).toBeFocused()
  await page.keyboard.press('Escape')

  // The path copies the selected folder, and the editor opens it.
  await page.locator('#worktree').click()
  await expect(page.locator('#worktree-copied')).toHaveText('Copied')
  assert.equal(await app.evaluate(({clipboard})=>clipboard.readText()),docs)
  await page.locator('#open-editor').click()
  await expect.poll(()=>state.opened.length).toBe(1)
  assert.deepEqual(state.opened[0],{runId:'several',folder:docs})

  // A folder Changes cannot read stays selected, with the agent's reason.
  await chevron.click();await items.nth(2).click()
  await expect(directory).toHaveText(notes)
  await expect(page.locator('.diff-error')).toHaveText('This folder is not a Git repository.')
  await expect(page.locator('#open-editor')).toBeVisible()

  // Back to the primary folder: no folder is sent, as with an older desktop.
  await chevron.click();await items.nth(0).click()
  await expect(directory).toHaveText(worktree)
  await expect(page.locator('.diff-status')).toHaveText('Changes loaded.')
  assert.deepEqual(state.diffs.at(-1),{id:'several',folder:null})
  await page.locator('#open-editor').click()
  await expect.poll(()=>state.opened.length).toBe(2)
  assert.deepEqual(state.opened[1],{runId:'several'})

  // The choice belongs to its execution and survives switching to another.
  await chevron.click();await items.nth(1).click()
  await expect(directory).toHaveText(docs)
  await page.locator('.local-task').filter({has:page.getByRole('button',{name:'Open #785 in Sectile',exact:true})}).locator('.run').click()
  await expect(directory).toHaveText('/tmp/example/.tasks/worktrees/issue-785')
  await expect(chevron).toBeHidden()
  await page.locator('.local-task').filter({has:page.getByRole('button',{name:'Open #784 in Sectile',exact:true})}).locator('.run').click()
  await expect(directory).toHaveText(docs)
  await expect(page.locator('.diff-context')).toContainText(docs+' · main')

  // A selected folder that leaves the list gives the toolbar back to the primary.
  runs[0].folders=[folders[0],folders[2]]
  await expect(directory).toHaveText(worktree,{timeout:15000})
  await expect(page.locator('.diff-context')).toContainText(worktree+' · feat/784')
 }finally{
  if(app)await app.close()
  for(const client of ws.clients)client.terminate()
  await new Promise(resolve=>ws.close(resolve))
  await new Promise(resolve=>server.close(resolve))
 }
})
