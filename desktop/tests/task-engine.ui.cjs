const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The ticket table shows the engine each task runs and switches it with one
// click (#510). The fake agent stores the choices as the real one does.
function fakeAgent({capabilities=['task-engines']}={}){
 const state={puts:[],launches:[],events:[],failPut:false,catalogue:[
  {id:'e-opus',name:'Claude Opus',provider:'claude',model:'claude-opus-5'},
  {id:'e-codex',name:'Codex',provider:'codex',model:''},
  {id:'e-agy',name:'Antigravity',provider:'agy',model:''},
 ],projectDefault:'e-opus',tasks:{a2:'e-codex'}}
 const tasks=[
  {id:'a1',key:'#1',title:'First open task',status:'to_clarify',priority:'medium'},
  {id:'a2',key:'#2',title:'Second open task',status:'to_clarify',priority:'low'},
  {id:'a3',key:'#3',title:'Clarified task',status:'to_specify',priority:'low'},
 ]
 const view=()=>({projectDefault:state.projectDefault,catalogue:state.catalogue,tasks:state.tasks})
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Project A',path:'/tmp/A'}]));return}
  if(url.pathname==='/desktop/runs'){res.end('[]');return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{skills:[{id:'clarify'}]}}));return}
  if(url.pathname==='/desktop/tasks'&&req.method==='POST'){
   let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
    state.launches.push(JSON.parse(raw));state.events.push('launch');res.end(JSON.stringify({status:'queued'}))
   });return
  }
  if(url.pathname==='/desktop/tasks'){res.end(JSON.stringify(tasks));return}
  if(url.pathname==='/desktop/task-engines'&&req.method==='PUT'){
   let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
    const input=JSON.parse(raw);state.puts.push(input);state.events.push('engine')
    if(state.failPut){res.writeHead(404,{'Content-Type':'text/plain'}).end('this engine is no longer in the catalogue');return}
    if(input.engineId===state.projectDefault)delete state.tasks[input.taskId];else state.tasks[input.taskId]=input.engineId
    res.end(JSON.stringify(view()))
   });return
  }
  if(url.pathname==='/desktop/task-engines'){res.end(JSON.stringify(view()));return}
  res.writeHead(404).end('{}')
 })
 return {state,server}
}

async function openTickets(server,root){
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 const app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
 const page=await app.firstWindow();page.setDefaultTimeout(7000)
 await page.getByRole('button',{name:'▾ Project A',exact:true}).waitFor()
 await page.getByRole('button',{name:'Actions for Project A',exact:true}).click()
 await page.getByRole('menuitem',{name:'Open tasks',exact:true}).click()
 await page.locator('.ticket-row').first().waitFor()
 return {app,page}
}

test('each task shows its engine and a click moves it to the next one',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-task-engine-'))
 const {state,server}=fakeAgent()
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 let app
 try{
  let page
  ;({app,page}=await openTickets(server,root))
  await expect(page.locator('.tickets-table th[data-column="engine"]')).toHaveText('Engine')
  const first=page.getByRole('button',{name:/^Engine for #1:/})
  const second=page.getByRole('button',{name:/^Engine for #2:/})
  // A task never switched runs its project default engine, unhighlighted.
  await expect(first).toHaveAttribute('aria-label','Engine for #1: Claude Opus - claude · claude-opus-5 (project default)')
  await expect(first).toHaveText('Cl')
  assert.equal(await first.getAttribute('data-off-default'),null)
  // A switched task is highlighted.
  await expect(second).toHaveText('Cx')
  await expect(second).toHaveAttribute('data-off-default','true')
  await expect(second).toHaveAttribute('title','Codex - codex · provider default')

  await first.click()
  await expect(first).toHaveText('Cx')
  await expect(first).toHaveAttribute('data-off-default','true')
  // The icon moves at once; the agent hears of it right after.
  await expect.poll(()=>state.puts.length).toBe(1)
  assert.deepEqual(state.puts.at(-1),{projectId:'project-a',taskId:'a1',engineId:'e-codex'})

  // Keyboard: Enter and Space activate it; the last engine wraps to the first.
  await second.focus();await page.keyboard.press('Enter')
  await expect(second).toHaveText('Ag')
  await page.keyboard.press('Space')
  await expect(second).toHaveText('Cl')
  await expect(second).not.toHaveAttribute('data-off-default','true')
  await expect.poll(()=>state.puts.length).toBe(3)
  assert.deepEqual(state.puts.at(-1),{projectId:'project-a',taskId:'a2',engineId:'e-opus'})
  assert.equal(state.tasks.a2,undefined)

  // A refusal reverts the icon to what the agent stores and says why.
  state.failPut=true
  await first.click()
  await expect(page.locator('.tickets-status')).toContainText('Could not switch the engine of #1: this engine is no longer in the catalogue')
  await expect(first).toHaveText('Cx')
  state.failPut=false

  // With a single engine, a click changes nothing.
  state.catalogue=[state.catalogue[0]];state.tasks={}
  await page.getByRole('button',{name:'Search',exact:true}).click()
  await expect(page.getByRole('button',{name:/^Engine for #1:/})).toHaveText('Cl')
  const puts=state.puts.length
  await page.getByRole('button',{name:/^Engine for #1:/}).click()
  await expect(page.getByRole('button',{name:/^Engine for #1:/})).toHaveText('Cl')
  assert.equal(state.puts.length,puts)
 }finally{
  await app?.close()
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})

test('an agent without per-task engines shows no engine column',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-task-engine-old-'))
 const {server}=fakeAgent({capabilities:[]})
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 let app
 try{
  let page
  ;({app,page}=await openTickets(server,root))
  await expect(page.locator('.tickets-table th[data-column="pr"]')).toHaveCount(1)
  await expect(page.locator('.tickets-table th[data-column="engine"]')).toHaveCount(0)
  await expect(page.locator('.engine-toggle')).toHaveCount(0)
 }finally{
  await app?.close()
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})

// The Launch dialog (#786) picks the engine among the catalogue; the choice
// sticks to the task and is stored before the launch.
test('the Launch dialog picks an engine that sticks to the task',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-launch-engine-'))
 const {state,server}=fakeAgent()
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 let app
 try{
  let page
  ;({app,page}=await openTickets(server,root))
  const dialog=page.locator('#project-dialog')
  const openLaunch=async key=>{
   await page.getByRole('button',{name:'More actions for '+key,exact:true}).click()
   await page.getByRole('menuitem',{name:'Launch…',exact:true}).click()
   await expect(dialog.locator('h2').first()).toHaveText('Launch '+key)
  }
  // The row entry preselects the next workflow skill, and the task's engine.
  await openLaunch('#1')
  const engine=dialog.getByRole('combobox',{name:'Launch AI engine',exact:true})
  await expect(dialog.getByRole('combobox',{name:'Launch skill',exact:true})).toHaveValue('clarify')
  await expect(dialog.getByRole('textbox',{name:'Launch instructions',exact:true})).toHaveValue('')
  await expect(engine).toHaveValue('e-opus')
  assert.deepEqual(await engine.locator('option').allTextContents(),[
   'Claude Opus - claude · claude-opus-5 (project default)','Codex - codex · provider default','Antigravity - agy · provider default',
  ])
  // Leaving the engine as it is stores nothing.
  await dialog.getByRole('button',{name:'Launch',exact:true}).click()
  await expect.poll(()=>state.launches.length).toBe(1)
  assert.equal(state.puts.length,0)
  assert.equal(state.launches[0].taskID,'a1');assert.equal(state.launches[0].skillID,'clarify')

  // Another engine is stored first, then the launch goes as before.
  await openLaunch('#1')
  await engine.selectOption('e-agy')
  await dialog.getByRole('button',{name:'Launch',exact:true}).click()
  await expect.poll(()=>state.launches.length).toBe(2)
  assert.deepEqual(state.puts.at(-1),{projectId:'project-a',taskId:'a1',engineId:'e-agy'})
  assert.deepEqual(state.events.slice(-2),['engine','launch'])
  assert.deepEqual(Object.keys(state.launches[1]).filter(field=>/engine/i.test(field)),[])
  await expect(page.getByRole('button',{name:/^Engine for #1:/})).toHaveText('Ag')

  // A task with no next workflow skill opens on a discussion; picking the
  // project default clears its switch.
  await openLaunch('#2')
  await expect(dialog.getByRole('combobox',{name:'Launch skill',exact:true})).toHaveValue('clarify')
  await dialog.getByRole('button',{name:'Close',exact:true}).click()
  await openLaunch('#3')
  await expect(dialog.getByRole('combobox',{name:'Launch skill',exact:true})).toHaveValue('discuss')
  await expect(engine).toHaveValue('e-opus')
  state.tasks.a3='e-codex'
  await dialog.getByRole('button',{name:'Close',exact:true}).click()
  await page.getByRole('button',{name:'Search',exact:true}).click()
  await openLaunch('#3')
  await expect(engine).toHaveValue('e-codex')
  await engine.selectOption('e-opus')
  await dialog.getByRole('button',{name:'Launch',exact:true}).click()
  await expect.poll(()=>state.launches.length).toBe(3)
  assert.equal(state.tasks.a3,undefined)
  assert.equal(state.launches[2].skillID,'discuss')

  // A refusal to store the engine launches nothing and says why.
  state.failPut=true
  await openLaunch('#2')
  await engine.selectOption('e-agy')
  await dialog.getByRole('button',{name:'Launch',exact:true}).click()
  await expect(dialog.getByRole('status')).toHaveText('Could not switch the engine: this engine is no longer in the catalogue')
  await expect(dialog.getByRole('button',{name:'Launch',exact:true})).toBeEnabled()
  assert.equal(state.launches.length,3)
 }finally{
  await app?.close()
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})

test('an agent without per-task engines launches from the dialog with no engine select',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-launch-engine-old-'))
 const {state,server}=fakeAgent({capabilities:[]})
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 let app
 try{
  let page
  ;({app,page}=await openTickets(server,root))
  const dialog=page.locator('#project-dialog')
  await page.getByRole('button',{name:'More actions for #1',exact:true}).click()
  await page.getByRole('menuitem',{name:'Launch…',exact:true}).click()
  await expect(dialog.getByRole('combobox',{name:'Launch skill',exact:true})).toHaveValue('clarify')
  await expect(dialog.getByRole('combobox',{name:'Launch AI engine',exact:true})).toHaveCount(0)
  await dialog.getByRole('button',{name:'Launch',exact:true}).click()
  await expect.poll(()=>state.launches.length).toBe(1)
  assert.equal(state.puts.length,0)
 }finally{
  await app?.close()
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})
