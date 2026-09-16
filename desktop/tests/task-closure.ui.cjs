const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('stopping a reviewed execution offers to close the task and launches handoff',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-task-closure-'))
 let stage='implemented',stopped=false,failLaunch=true,launches=[]
 const runs=()=>[{id:'run-1',taskId:'task-a',taskKey:'#1',projectId:'project-a',skill:'adjust',sessionId:'run-1',status:stopped?'canceled':'running'}]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Project A',path:'/tmp/project'}]));return}
  if(req.url==='/desktop/project?id=project-a'){res.end(JSON.stringify({configured:true,server:{skills:['adjust','handoff'].map(id=>({id}))}}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs()));return}
  if(req.url==='/desktop/stop?id=run-1'){stopped=true;res.writeHead(204).end();return}
  if(req.url.startsWith('/desktop/tasks?')){
   if(req.method==='POST'){
    let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
     if(failLaunch){res.writeHead(500);res.end(JSON.stringify({error:'Launch rejected'}));return}
     launches.push(JSON.parse(raw));res.end(JSON.stringify({status:'queued'}))
    });return
   }
   res.end(JSON.stringify([{id:'task-a',key:'#1',labels:['#'+stage]}]));return
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
  const stop=page.locator('#stop'),dialog=page.locator('#project-dialog')

  // A task still short of the reviewed stage has nothing to close.
  await stop.waitFor();await page.waitForFunction(()=>!document.querySelector('#stop').disabled)
  await stop.click()
  await page.waitForFunction(()=>document.querySelector('#skill-result').textContent.includes('canceled'))
  assert.equal(await dialog.isVisible(),false,'A non-reviewed task raises no closing dialog')

  // The same stop on a reviewed task proposes its closing step.
  stopped=false;stage='reviewed'
  await page.waitForFunction(()=>!document.querySelector('#stop').disabled)
  await stop.click()
  await page.getByRole('heading',{name:'Close #1?',exact:true}).waitFor()
  const confirm=page.getByRole('button',{name:'Close the task',exact:true})

  await confirm.click()
  await page.waitForFunction(()=>document.querySelector('#dialog-body').textContent.includes('Launch rejected'))
  assert.equal(launches.length,0)
  assert.equal(await confirm.isEnabled(),true,'A failed launch can be retried')

  failLaunch=false
  await confirm.click()
  await page.waitForFunction(()=>!document.querySelector('#project-dialog').open)
  assert.deepEqual(launches,[{taskID:'task-a',skillID:'handoff',prompt:''}])
 }finally{
  if(app)await app.close()
  server.close();fs.rmSync(root,{recursive:true,force:true})
 }
})
