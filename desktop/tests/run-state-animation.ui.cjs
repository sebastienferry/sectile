const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('desktop running icons pulse across polls and stop for other states or reduced motion',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-run-animation-'))
 const run={id:'run',taskId:'task',taskKey:'#377',projectId:'project',skill:'implement',directory:'/tmp/repo',status:'running'}
 let polls=0
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Project',path:'/tmp/repo'}]));return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(url.pathname==='/desktop/runs'){polls++;res.end(JSON.stringify([run]));return}
  if(url.pathname==='/desktop/tasks'){res.end('[]');return}
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
  await page.emulateMedia({reducedMotion:'no-preference'})
  const states=page.locator('.local-task .run-state, #run-state')
  const icons=states.locator('svg')
  await expect(states).toHaveCount(2)
  await expect(page.locator('#run-state')).toHaveText('Running')
  for(const icon of await icons.all()){
   await expect.poll(()=>icon.evaluate(el=>el.getAnimations().filter(a=>a.playState==='running').length)).toBe(1)
   const timing=await icon.evaluate(el=>{
    const a=el.getAnimations()[0],t=a.effect.getTiming()
    window.__iconSamples??=[]
    window.__iconSamples.push({el,animation:a,time:a.currentTime})
    return {duration:t.duration,iterations:String(t.iterations),easing:t.easing}
   })
   assert.deepEqual(timing,{duration:2000,iterations:'Infinity',easing:'linear'})
   const before=await icon.evaluate(el=>getComputedStyle(el).transform)
   await expect.poll(()=>icon.evaluate(el=>getComputedStyle(el).transform)).not.toBe(before)
  }
  assert.equal(await page.locator('#run-state .run-state-label').evaluate(el=>getComputedStyle(el).transform),'none','The label must not pulse')
  const startPoll=polls
  await expect.poll(()=>polls).toBeGreaterThan(startPoll+1)
  assert.ok(await page.evaluate(()=>window.__iconSamples.every(({el,animation,time})=>el.isConnected&&el.getAnimations()[0]===animation&&animation.currentTime>time)),'Polling must preserve and advance each running animation')

  await page.emulateMedia({reducedMotion:'reduce'})
  for(const icon of await icons.all())await expect.poll(()=>icon.evaluate(el=>el.getAnimations().length)).toBe(0)
  await expect(page.locator('#run-state')).toHaveText('Running')
  await page.emulateMedia({reducedMotion:'no-preference'})
  for(const icon of await icons.all())await expect.poll(()=>icon.evaluate(el=>el.getAnimations().length)).toBe(1)

  // These changes arrive through the real agent polling path, not DOM edits.
  for(const [status,waitingSince,state,label] of [
   ['running','2026-09-23T06:00:00Z','waiting','Waiting for you'],
   ['running',undefined,'running','Running'],
   ['queued',undefined,'queued','Queued'],
   ['preparing',undefined,'queued','Queued'],
   ['completed',undefined,'completed','Finished'],
   ['failed',undefined,'failed','Failed'],
   ['canceled',undefined,'canceled','Cancelled'],
  ]){
   run.status=status;run.waitingSince=waitingSince
   await expect(page.locator('.local-task .run')).toHaveAttribute('data-status',status)
   for(const element of await states.all())await expect(element).toHaveAttribute('data-run-state',state)
   await expect(page.locator('#run-state')).toHaveText(label)
   for(const icon of await icons.all())assert.equal(await icon.evaluate(el=>el.getAnimations().length),state==='running'?1:0,`${state} animation count`)
  }
 }finally{
  await app?.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
