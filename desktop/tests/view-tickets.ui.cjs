const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// A fake local agent with two projects and one saved view over both (#429).
// Project B is not mapped on this workstation.
async function fakeAgent({capabilities}){
 const state={launches:[],viewRequests:[],directories:[],taskRequests:[]}
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  const body=()=>new Promise(resolve=>{let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>resolve(raw?JSON.parse(raw):{}))})
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Project A',path:'/tmp/A'}]));return}
  if(url.pathname==='/desktop/runs'){res.end('[]');return}
  if(url.pathname==='/desktop/project'){
   const id=url.searchParams.get('id')
   res.end(JSON.stringify({configured:id==='project-a',server:{projectName:id==='project-b'?'Project B':'Project A',skills:[{id:'clarify'}]}}));return
  }
  if(url.pathname==='/desktop/views'){
   if(req.method==='POST'){body().then(input=>{state.directories.push(input);res.end(JSON.stringify({directory:input.path,warning:input.path?'/work/other is a checkout of git@github.com:o/other.git, while the view Platform declares github.com/o/platform':''}))});return}
   state.viewRequests.push(req.method);res.end(JSON.stringify([{id:'v1',name:'Platform',projectIds:['project-a','project-b'],labels:[],repository:'git@github.com:o/platform.git',directory:''}]));return
  }
  if(url.pathname==='/desktop/tasks'){
   if(req.method==='POST'){body().then(input=>{state.launches.push({project:url.searchParams.get('projectId'),...input});res.end(JSON.stringify({status:'queued'}))});return}
   state.taskRequests.push(url.search)
   if(url.searchParams.get('viewId')==='v1'){
    res.end(JSON.stringify([
     {id:'a1',key:'#1',title:'Task of A',status:'to_clarify',priority:'high',projectId:'project-a'},
     {id:'b1',key:'#7',title:'Task of B',status:'to_clarify',priority:'low',projectId:'project-b'}
    ]));return
   }
   res.end(JSON.stringify([{id:'a1',key:'#1',title:'Task of A',status:'to_clarify',priority:'high',projectId:'project-a'}]));return
  }
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 return {server,state}
}

async function launchApp(server,root){
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 const app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
 const page=await app.firstWindow();page.setDefaultTimeout(7000)
 await page.getByRole('button',{name:'▾ Project A',exact:true}).waitFor()
 return {app,page}
}

async function openTasksList(page){
 await page.keyboard.press('Control+k')
 await page.getByRole('textbox',{name:'Search commands'}).fill('tasks')
 await page.getByRole('button',{name:'Tasks list',exact:true}).click()
}

test('a saved view opens in the tickets pane, each row with its own project',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-view-tickets-'))
 const {server,state}=await fakeAgent({capabilities:['board-views']})
 let app
 try{
  let page
  ;({app,page}=await launchApp(server,root))
  await app.evaluate(({dialog})=>{dialog.showOpenDialog=async()=>({canceled:false,filePaths:['/work/other']})})
  // A single project no longer opens directly: there is a view to choose.
  await openTasksList(page)
  await page.getByText('Choose the project or the saved view whose tasks you want to browse.',{exact:true}).waitFor()
  await expect(page.getByRole('heading',{name:'Saved views',exact:true})).toBeVisible()
  await expect(page.getByRole('button',{name:'Project A',exact:true})).toBeVisible()
  await page.getByRole('button',{name:'Platform',exact:true}).click()
  await expect(page.getByRole('heading',{name:'Tickets · Platform',exact:true})).toBeVisible()
  const rows=page.locator('.ticket-row')
  await expect(rows).toHaveCount(2)
  assert.match(state.taskRequests.at(-1),/viewId=v1/)
  await expect(page.locator('.tickets-table th[data-column="project"]')).toHaveText('Project')
  const rowA=rows.filter({hasText:'Task of A'}),rowB=rows.filter({hasText:'Task of B'})
  await expect(rowA.locator('.ticket-project')).toHaveText('Project A')
  await expect(rowB.locator('.ticket-project')).toHaveText('Project B')
  // Project B is not mapped here and the view has no folder yet.
  await expect(rowB.locator('.ticket-run')).toBeDisabled()
  await expect(rowB.locator('.ticket-run')).toHaveAttribute('title','Configure the local project to continue')
  await page.getByText('Launches from this view run in each task’s project repository',{exact:true}).waitFor()

  // A launch goes to the row's own project and names the view.
  await rowA.getByRole('button',{name:'Run: Clarify',exact:true}).click()
  await expect.poll(()=>state.launches.length).toBe(1)
  assert.deepEqual(state.launches[0],{project:'project-a',taskID:'a1',skillID:'clarify',prompt:'',viewID:'v1'})

  // A folder of another repository is kept, with the warning.
  await page.getByRole('button',{name:'Local directory…',exact:true}).click()
  await page.getByText(/^Warning: \/work\/other is a checkout of git@github.com:o\/other.git/).waitFor()
  assert.deepEqual(state.directories,[{viewId:'v1',path:'/work/other'}])
  await page.getByText('Launches from this view run in /work/other',{exact:true}).waitFor()
  // With a folder, every row of the view can launch there.
  await expect(rowB.getByRole('button',{name:'Run: Clarify',exact:true})).toBeEnabled()
  await rowB.getByRole('button',{name:'Run: Clarify',exact:true}).click()
  await expect.poll(()=>state.launches.length).toBe(2)
  assert.deepEqual(state.launches[1],{project:'project-b',taskID:'b1',skillID:'clarify',prompt:'',viewID:'v1'})

  await page.getByRole('button',{name:'Clear directory',exact:true}).click()
  await page.getByText('Local directory cleared',{exact:true}).waitFor()
  assert.deepEqual(state.directories.at(-1),{viewId:'v1',path:''})
  await expect(rowB.locator('.ticket-run')).toBeDisabled()
 }finally{
  await app?.close()
  server.close()
 }
})

test('without the agent capability the chooser offers projects only',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-view-tickets-old-'))
 const {server,state}=await fakeAgent({capabilities:[]})
 let app
 try{
  let page
  ;({app,page}=await launchApp(server,root))
  // As before #429: the only project opens directly, and no view is asked for.
  await openTasksList(page)
  await expect(page.getByRole('heading',{name:'Tickets · Project A',exact:true})).toBeVisible()
  await expect(page.locator('.ticket-row')).toHaveCount(1)
  await expect(page.locator('.tickets-table th[data-column="project"]')).toHaveCount(0)
  await expect(page.locator('.tickets-directory')).toHaveCount(0)
  assert.deepEqual(state.viewRequests,[])
 }finally{
  await app?.close()
  server.close()
 }
})
