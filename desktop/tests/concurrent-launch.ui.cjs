const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The launches the test asserts on are recorded by the stub server, not by the
// page, so waiting on the window alone can read them one click behind.
async function until(condition,message,timeout=7000){
 const deadline=Date.now()+timeout
 while(!condition()){
  if(Date.now()>deadline)throw Error(message)
  await new Promise(resolve=>setTimeout(resolve,25))
 }
}

// A launch refused because another session is already running the task must say
// so, and offer the one gesture that gets through. A 409 that carries no active
// run - a finished task, a disconnected agent - is not forceable and gets no
// control at all.
test('a refused concurrent launch names the active run and offers a forced retry',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-concurrent-launch-'))
 // 'conflict' answers with the active-run refusal, 'plain' with a 409 that carries no marker.
 let refusal='conflict',launches=[],started=0
 // One run per launch that the server accepted: the renderer only stops
 // treating a launch as pending once a run it had not seen appears.
 const runs=()=>[
  {id:'run-a',taskId:'task-a',taskKey:'#1',projectId:'project-a',skill:'clarify',status:'completed'},
  ...Array.from({length:started},(_,index)=>({id:'run-new-'+index,taskId:'task-a',taskKey:'#1',projectId:'project-a',skill:'implement',status:'completed'}))
 ]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:[]}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Project A',path:'/tmp/project'}]));return}
  if(req.url==='/desktop/project?id=project-a'){res.end(JSON.stringify({configured:true,server:{prCreationStage:'implemented',skills:['clarify','specify','implement','adjust','handoff'].map(id=>({id}))}}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs()));return}
  if(req.url.startsWith('/desktop/tasks?')){
   if(req.method==='POST'){
    let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
     const body=JSON.parse(raw);launches.push(body)
     if(refusal==='plain'){res.writeHead(409);res.end('This task is finished. Reopen it on the server before launching an execution.');return}
     if(refusal==='conflict'&&!body.force){
      res.writeHead(409)
      res.end(JSON.stringify({error:'A run of implement started at 2026-09-21T10:00:00Z is still active on this task.',activeRunId:'run-x'}))
      return
     }
     started++;res.end(JSON.stringify({status:'queued'}))
    });return
   }
   res.end(JSON.stringify([{id:'task-a',key:'#1',labels:['#specified']}]));return
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
  const button=page.locator('#next-step'),force=page.locator('#force-next-step'),status=page.locator('#next-step-status')
  await page.getByRole('button',{name:'Next: Implement',exact:true}).waitFor()
  assert.equal(await force.isHidden(),true,'Nothing is offered before a refusal')

  await button.click()
  await page.waitForFunction(()=>document.querySelector('#next-step-status').textContent.includes('is still active on this task'))
  assert.match(await status.textContent(),/A run of implement started at/)
  await force.waitFor()
  await until(()=>launches.length===1,'the plain launch never reached the server')
  assert.equal(launches[0].force,undefined,'A plain launch carries no force')

  await force.click()
  await until(()=>launches.length===2,'the forced launch never reached the server')
  await page.waitForFunction(()=>document.querySelector('#force-next-step').hidden)
  assert.equal(launches[1].force,true,'The forced retry re-sends the same launch with force')
  assert.equal(launches[1].skillID,launches[0].skillID)

  // A 409 with no active run is a plain failure: the message shows, the gesture does not.
  refusal='plain'
  await page.waitForFunction(()=>!document.querySelector('#next-step').disabled)
  await button.click()
  await page.waitForFunction(()=>document.querySelector('#next-step-status').textContent.includes('Could not launch next step'))
  assert.equal(await force.isHidden(),true,'A refusal without an active run offers no forced retry')
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
