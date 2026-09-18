const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The tickets pane's custom-instructions form and the relaunch dialog carry a
// one-off execution mode; the row's Run button and the footer next-step button
// do not. What matters on the wire is that an untouched control sends no
// override at all, so a launch nobody made a choice for behaves exactly as it
// did before the setting existed.
test('the custom-instructions form and the relaunch dialog carry a one-off execution mode',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-skill-mode-'))
 const launches=[]
 const runs=[{
  id:'run-1',taskId:'task-a',taskKey:'#7',projectId:'project-a',skill:'specify',
  status:'completed',prompt:'previous instructions',directory:'/tmp/a',createdAt:new Date().toISOString()
 }]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost'),project=url.searchParams.get('projectId')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Project A',path:'/tmp/a'}]));return}
  if(url.pathname==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{skills:[{id:'clarify'},{id:'specify'}]}}));return}
  if(url.pathname==='/desktop/tasks'){
   if(req.method==='POST'){
    let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
     launches.push({project,...JSON.parse(raw)});res.end(JSON.stringify({status:'queued'}))
    });return
   }
   res.end(JSON.stringify([{id:'task-a',key:'#7',title:'Open task',status:'to_clarify',priority:'medium'}]));return
  }
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  const heading=page.getByRole('button',{name:'▾ Project A',exact:true})
  await heading.waitFor()
  await heading.hover()
  await page.getByRole('button',{name:'Open tasks in Project A',exact:true}).click()
  const rows=page.locator('.ticket-row')
  await expect(rows).toHaveCount(1)
  const more=page.getByRole('button',{name:'More actions for #7',exact:true})
  const custom=page.getByRole('menuitem',{name:'Custom instructions…',exact:true})
  const instructions=page.getByRole('textbox',{name:'Custom instructions',exact:true})
  const launch=page.getByRole('button',{name:'Launch',exact:true})

  // The row's Run button carries no mode: the configured one applies.
  await page.getByRole('button',{name:'Run: Clarify',exact:true}).click()
  await expect.poll(()=>launches.length).toBe(1)
  assert.deepEqual(launches[0],{project:'project-a',taskID:'task-a',skillID:'clarify',prompt:''})

  // The control offers the three states and starts on the configured mode.
  await more.click();await custom.click()
  const mode=page.getByRole('combobox',{name:'Execution mode for #7',exact:true})
  await expect(mode).toHaveValue('')
  assert.deepEqual(await mode.locator('option').evaluateAll(list=>list.map(option=>option.value)),['','interactive','autonomous'])

  // An untouched control sends no override.
  await instructions.fill('Summarize the task')
  await launch.click()
  await expect.poll(()=>launches.length).toBe(2)
  assert.deepEqual(launches[1],{project:'project-a',taskID:'task-a',skillID:'custom',prompt:'Summarize the task'})
  await expect(instructions).toHaveCount(0)

  // An explicit choice travels with that launch.
  await more.click();await custom.click()
  await page.getByRole('combobox',{name:'Execution mode for #7',exact:true}).selectOption('autonomous')
  await instructions.fill('Do it headless')
  await launch.click()
  await expect.poll(()=>launches.length).toBe(3)
  assert.deepEqual(launches[2],{project:'project-a',taskID:'task-a',skillID:'custom',prompt:'Do it headless',mode:'autonomous'})

  // The relaunch dialog offers the same choice, on the same per-launch terms.
  await page.getByRole('button',{name:'Close tickets',exact:true}).click()
  await page.getByText('#7',{exact:false}).first().click()
  await page.getByRole('button',{name:'Relaunch',exact:true}).first().click()
  const relaunchMode=page.getByRole('combobox',{name:'Relaunch execution mode',exact:true})
  await expect(relaunchMode).toHaveValue('')
  await relaunchMode.selectOption('interactive')
  await page.getByRole('button',{name:'Launch new execution',exact:true}).click()
  await expect.poll(()=>launches.length).toBe(4)
  assert.equal(launches[3].mode,'interactive')
 }finally{
  if(app)await app.close()
  server.close()
 }
})
