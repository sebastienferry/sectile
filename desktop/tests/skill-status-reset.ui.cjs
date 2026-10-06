const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// A console left open after its skill completed must not keep its ✓ once
// another skill runs on the task (#586): neither a skill started inside it
// (path A) nor a new execution launched beside it (path B).
test('the skill badge drops a completed verdict once another skill runs',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-skill-status-reset-'))
 const run=(id,taskId,skill,status,hour)=>({id,taskId,taskKey:'#'+taskId,projectId:'project',skill,status,sessionId:id,directory:'/tmp/example/worktree',createdAt:`2026-10-06T${hour}:00:00Z`})
 // Path B on #1: the clarify console stays open while specify is queued.
 // Path A on #2: the clarify console goes on to run specify itself.
 const runs=[run('old','1','clarify','running','09'),run('new','1','specify','queued','10'),run('same','2','clarify','running','08')]
 let successor=null
 const results={
  old:()=>({activity:{id:'old',taskId:'1',skillId:'clarify',status:'completed'},successor:null,task:{labels:['clarified']}}),
  new:()=>({activity:null,successor:null,task:{labels:['clarified']}}),
  same:()=>({activity:{id:'same',taskId:'2',skillId:'clarify',status:'completed'},successor,task:{labels:['clarified']}})
 }
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Example project',path:'/tmp/example'}]));return}
  if(req.url==='/desktop/project?id=project'){res.end(JSON.stringify({parallelism:3}));return}
  if(req.url.startsWith('/desktop/run-result?')){const id=new URL(req.url,'http://local').searchParams.get('id');if(!results[id]){res.writeHead(404).end();return}res.end(JSON.stringify(results[id]()));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end('[]');return}
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  const header=page.locator('#skill-result')
  // Path B: the row is still led by the running console, but its skill badge
  // speaks for the queued execution, which has no verdict yet.
  await expect(page.locator('.local-task .run[data-run-id="old"]')).toHaveCount(1)
  await expect(page.locator('.task-skill-status[data-run-id="new"]')).toHaveCount(1)
  await expect(page.locator('.task-skill-status[data-run-id="old"]')).toHaveCount(0)
  await expect(page.locator('.task-skill-status[data-run-id="new"]')).toHaveText('')
  // The old console, selected, keeps its own verdict.
  await page.locator('.local-task .run[data-run-id="old"]').click()
  await expect(header).toHaveText('✓ Skill completed')
  // Path A: the console's own skill completed...
  const same=page.locator('.task-skill-status[data-run-id="same"]')
  await page.locator('.local-task .run[data-run-id="same"]').click()
  await expect(same).toHaveText('✓')
  await expect(header).toHaveText('✓ Skill completed')
  // ...then a skill started inside it clears the verdict, past the settling
  // window, within a couple of polls.
  await page.evaluate(()=>{const real=Date.now;Date.now=()=>real()+31000})
  successor={id:'next',taskId:'2',skillId:'specify-issue',status:'running'}
  await expect(same).toHaveText('',{timeout:5000})
  await expect(header).toBeHidden()
  successor={...successor,waitingSince:'2026-10-06T10:00:00Z'}
  await expect(same).toHaveText('?',{timeout:5000})
  successor={id:'next',taskId:'2',skillId:'specify-issue',status:'completed'}
  await expect(header).toHaveText('◷ Awaiting stage validation',{timeout:5000})
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
