const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('a project groups its tasks by stage from its menu and every key shows its stage',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-stage-grouping-'))
 const run=(projectId,id,status,second,extra={})=>({projectId,id,taskId:'task-'+id,taskKey:'#'+id,skill:'implement',directory:'/tmp/'+projectId,status,createdAt:'2026-09-25T06:00:0'+second+'Z',...extra})
 const runs=[
  run('alpha','reviewed','completed',5),
  run('alpha','fresh','completed',4),
  run('alpha','spec','completed',3),
  {...run('alpha','console','running',2),kind:'console',provider:'claude',taskId:'',taskKey:'',skill:'',sessionId:'console'},
  run('beta','later','completed',5),
  run('beta','early','completed',4),
 ]
 const tasks={
  alpha:[
   {id:'task-reviewed',key:'#reviewed',title:'Reviewed',labels:['#reviewed'],status:'to_close'},
   {id:'task-fresh',key:'#fresh',title:'Fresh',labels:['#new'],status:'to_clarify'},
   {id:'task-spec',key:'#spec',title:'Spec',labels:['#specified'],status:'to_implement'},
  ],
  beta:[
   {id:'task-later',key:'#later',title:'Later',labels:['#finished'],status:'done'},
   {id:'task-early',key:'#early',title:'Early',labels:['#new'],status:'to_clarify'},
  ],
 }
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://example.test/sectile/',capabilities:['free-console']}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'alpha',name:'Alpha',path:'/tmp/alpha'},{id:'beta',name:'Beta',path:'/tmp/beta'}]));return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(url.pathname==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(url.pathname==='/desktop/tasks'){res.end(JSON.stringify(tasks[url.searchParams.get('projectId')]||[]));return}
  if(url.pathname==='/desktop/run-result'){res.end(JSON.stringify({activity:null}));return}
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}),{mode:0o600})
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  const order=project=>page.locator('#runs .project-group').filter({has:page.locator(`.project-more[data-project-id="${project}"]`)}).locator('.local-task .run-state').evaluateAll(states=>states.map(state=>state.dataset.runId))
  const key=id=>page.locator(`#runs .local-task:has(.run-state[data-run-id="${id}"]) .task-number`)
  const groupByStage=project=>page.locator(`.project-more[data-project-id="${project}"]`).click().then(()=>page.getByRole('menuitemcheckbox',{name:'Group by stage'}))

  // Every key carries the tint of its stage, grouping or not, once the tasks are read.
  await expect(key('reviewed')).toHaveAttribute('data-stage','reviewed')
  await expect(key('fresh')).toHaveAttribute('data-stage','new')
  await expect(key('spec')).toHaveAttribute('data-stage','specified')
  await expect(key('later')).toHaveAttribute('data-stage','finished')
  await expect(key('spec')).toHaveAttribute('title','Open task in Sectile · Stage: specified')
  assert.equal(await key('console').count(),0,'A free console has no key to tint')
  assert.deepEqual(await order('alpha'),['console','reviewed','fresh','spec'],'Off by default: the usual order')

  const entry=await groupByStage('alpha')
  await expect(entry).toHaveAttribute('aria-checked','false')
  await entry.click()
  await expect.poll(()=>order('alpha')).toEqual(['fresh','spec','reviewed','console'])
  assert.deepEqual(await order('beta'),['later','early'],'Another project keeps its order')
  await expect(await groupByStage('alpha')).toHaveAttribute('aria-checked','true')
  await page.keyboard.press('Escape')
  await page.mouse.move(0,0)
  // Kept for a look at the tints in both themes.
  const shots=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-stage-tints-'))
  for(const theme of ['dark','light']){
   await page.emulateMedia({colorScheme:theme})
   await page.locator('#runs').screenshot({path:path.join(shots,theme+'.png')})
  }
  console.log('Stage tint screenshots: '+shots)

  // The choice is remembered on the workstation.
  await page.reload()
  await expect.poll(()=>order('alpha')).toEqual(['fresh','spec','reviewed','console'])
  assert.deepEqual(await order('beta'),['later','early'])

  // Choosing it again restores the usual order.
  await (await groupByStage('alpha')).click()
  await expect.poll(()=>order('alpha')).toEqual(['console','reviewed','fresh','spec'])
 }finally{
  await app?.close()
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})
