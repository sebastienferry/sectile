const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')
const {WebSocketServer}=require('ws')

test('desktop disconnects locally, preserves history, and explicitly reconnects',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-disconnect-'))
 const disconnected=new Set()
 let removalError='',supported=true,removals=0,other=false,attached=0,detached=0,failDiscovery=false,failDetails=false
 const server=http.createServer((req,res)=>{
  if(req.headers.authorization!=='Bearer test-secret'){res.writeHead(401).end();return}
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:supported?['remove-project']:[],disconnectedProjects:[...disconnected]}));return}
  if(url.pathname==='/desktop/projects'){
   if(req.method==='DELETE'){
    removals++
    if(removalError){res.writeHead(409).end(removalError);return}
    disconnected.add(url.searchParams.get('id'));res.writeHead(204).end();return
   }
   if(req.method==='POST'){
    let raw='';req.on('data',data=>raw+=data);req.on('end',()=>{disconnected.delete(JSON.parse(raw).projectId);res.writeHead(204).end()});return
   }
   if(failDiscovery){res.writeHead(503).end('Catalog unavailable');return}
   res.end(JSON.stringify(['a',...(other?['b']:[])].map(id=>({id,name:'Project '+id,path:disconnected.has(id)?'':'/tmp/repository',configured:!disconnected.has(id),disconnected:disconnected.has(id)}))));return
  }
  if(url.pathname==='/desktop/project'){
   if(failDetails){res.writeHead(503).end('Server settings unavailable');return}
   const id=url.searchParams.get('id')
   res.end(JSON.stringify({server:{projectName:'Project '+id,skills:[{id:'specify'}]},path:disconnected.has(id)?'':'/tmp/repository',configured:!disconnected.has(id)}));return
  }
  if(url.pathname==='/desktop/runs'){
   res.end(JSON.stringify(['a',...(other?['b']:[])].map(id=>({id:'run-'+id,projectId:id,taskId:'task-'+id,taskKey:'#'+id,sessionId:'run-'+id,skill:'specify',status:'completed',directory:'/tmp/repository'}))));return
  }
  if(url.pathname==='/desktop/tasks'){const id=url.searchParams.get('projectId');res.end(JSON.stringify([{id:'task-'+id,key:'#'+id,labels:['#clarified']}]));return}
  res.writeHead(404).end()
 })
 const ws=new WebSocketServer({noServer:true})
 server.on('upgrade',(req,socket,head)=>ws.handleUpgrade(req,socket,head,client=>{
  attached++;client.send(Buffer.from('Retained console history\r\n'));client.on('close',()=>detached++)
 }))
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}),{mode:0o600})
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let application
 try{
  application=await electron.launch({executablePath:process.env.SECTILE_DESKTOP_EXECUTABLE,args:process.env.SECTILE_DESKTOP_EXECUTABLE?[]:[path.resolve(__dirname,'..')],env})
  const page=await application.firstWindow()
  await page.getByText('#a · specify',{exact:true}).waitFor()
  await page.getByRole('button',{name:'Next: Specify',exact:true}).waitFor()
  const openRemoval=async(id='a')=>{
   await page.getByRole('button',{name:'Configure Project '+id,exact:true}).click()
   await page.getByRole('button',{name:'Remove from desktop',exact:true}).click()
   await page.getByRole('heading',{name:'Remove Project '+id+' from desktop?',exact:true}).waitFor()
  }
  await openRemoval()
  await page.getByRole('button',{name:'Cancel',exact:true}).click()
  assert.equal(removals,0)
  await openRemoval()
  removalError='Stop active executions and wait for them to exit'
  await page.getByRole('button',{name:'Disconnect project',exact:true}).click()
  await page.getByText(removalError,{exact:false}).waitFor()
  assert.equal(disconnected.size,0)
  assert.equal(await page.locator('.project-group').count(),1)
  removalError=''
  supported=false
  await page.getByRole('button',{name:'Disconnect project',exact:true}).click()
  await page.getByText('The running local agent does not support project removal.',{exact:false}).waitFor()
  assert.equal(removals,1)
  supported=true
  const beforeDetach=detached
  failDiscovery=true
  await page.getByRole('button',{name:'Disconnect project',exact:true}).click()
  await page.getByText('Project disconnected, but refreshing projects failed:',{exact:false}).waitFor()
  await page.waitForFunction(()=>document.querySelectorAll('.project-group').length===0)
  assert.equal(await page.locator('#title').textContent(),'Select an execution')
  assert.equal(await page.locator('#directory').textContent(),'')
  assert.equal(await page.locator('#rerun').isHidden(),true)
  assert.equal(await page.locator('#stop').isDisabled(),true)
  assert.equal(await page.locator('#next-step').isHidden(),true)
  assert.equal(await page.locator('#next-step-status').textContent(),'Select a task to see its next step')
  await page.waitForTimeout(100)
  assert.ok(detached>beforeDetach)
  failDiscovery=false
  await page.reload()
  await page.locator('#workspace').waitFor()
  await page.waitForTimeout(2200)
  assert.equal(await page.locator('.project-group').count(),0,'retained history must not restore removed groups after reload')
  await page.evaluate(()=>localStorage.setItem('localTasks',JSON.stringify({'["a","task-a"]':{archivedRuns:['run-a']}})))
  await page.reload()
  await page.locator('#workspace').waitFor()
  await page.locator('#add-project').click()
  await page.getByRole('button',{name:'Project a',exact:true}).click()
  await page.getByRole('textbox',{name:'Local repository',exact:true}).fill('/tmp/repository')
  await page.getByRole('button',{name:'Save local configuration',exact:true}).click()
  await page.getByText('Local configuration saved',{exact:true}).waitFor()
  await page.getByRole('button',{name:'Close',exact:true}).click()
  assert.equal(await page.locator('.local-task').count(),0,'re-add preserves task archives')
  await page.evaluate(()=>localStorage.removeItem('localTasks'))
  await page.reload()
  await page.getByText('#a · specify',{exact:true}).waitFor()
  assert.equal(disconnected.size,0)
  assert.ok(attached>=2,'history is available again')
  // State polling must remove history-only groups without changes to the runs.
  disconnected.add('a')
  await page.waitForFunction(()=>document.querySelectorAll('.project-group').length===0)
  assert.equal(await page.locator('#title').textContent(),'Select an execution')
  disconnected.delete('a');other=true
  await page.getByRole('button',{name:'Configure Project a',exact:true}).waitFor()
  // Reload discovery so the second project's display name is available.
  await page.reload()
  await page.getByRole('button',{name:'Configure Project b',exact:true}).waitFor()
  await page.locator('.run').filter({hasText:'specify'}).last().click()
  await page.getByText('#b · specify',{exact:true}).waitFor()
  const beforeOtherDetach=detached
  failDetails=true
  await openRemoval('a')
  await page.getByRole('button',{name:'Disconnect project',exact:true}).click()
  await page.waitForFunction(()=>!document.querySelector('#project-dialog').open)
  assert.equal(await page.locator('#title').textContent(),'#b · specify')
  assert.equal(detached,beforeOtherDetach)
 }finally{
  if(application)await application.close()
  for(const client of ws.clients)client.terminate()
  ws.close();await new Promise(resolve=>server.close(resolve))
 }
})
