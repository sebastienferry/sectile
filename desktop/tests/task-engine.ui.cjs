const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The ticket table shows the engine each task runs and switches it with one
// click (#510). The fake agent stores the choices as the real one does.
function fakeAgent({capabilities=['task-engines']}={}){
 const state={puts:[],failPut:false,catalogue:[
  {id:'e-opus',name:'Claude Opus',provider:'claude',model:'claude-opus-5'},
  {id:'e-codex',name:'Codex',provider:'codex',model:''},
  {id:'e-agy',name:'Antigravity',provider:'agy',model:''},
 ],projectDefault:'e-opus',tasks:{a2:'e-codex'}}
 const tasks=[
  {id:'a1',key:'#1',title:'First open task',status:'to_clarify',priority:'medium'},
  {id:'a2',key:'#2',title:'Second open task',status:'to_clarify',priority:'low'},
 ]
 const view=()=>({projectDefault:state.projectDefault,catalogue:state.catalogue,tasks:state.tasks})
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Project A',path:'/tmp/A'}]));return}
  if(url.pathname==='/desktop/runs'){res.end('[]');return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{skills:[{id:'clarify'}]}}));return}
  if(url.pathname==='/desktop/tasks'){res.end(JSON.stringify(tasks));return}
  if(url.pathname==='/desktop/task-engines'&&req.method==='PUT'){
   let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
    const input=JSON.parse(raw);state.puts.push(input)
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
