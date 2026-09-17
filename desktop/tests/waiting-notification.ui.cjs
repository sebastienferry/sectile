const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// Drives the real application against a stub agent and checks that a session
// which starts waiting raises a real notification, once, carrying the glyph the
// task list uses for that same state.
test('a waiting session raises one native notification carrying the shared glyph',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-waiting-'))
 const runs=[{id:'run-1',taskId:'t1',taskKey:'#174',skill:'implement',projectId:'p',directory:'/tmp/repo',sessionId:'run-1',status:'running'}]
 let alerts=[]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:[]}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'p',name:'Project',path:'/tmp/repo'}]));return}
  if(req.url==='/desktop/project?id=p'){res.end(JSON.stringify({configured:true,parallelism:2,server:{aiProvider:'claude',skills:[]}}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url==='/desktop/session-alert'){const pending=alerts;alerts=[];res.end(JSON.stringify(pending));return}
  if(req.url.startsWith('/desktop/run-result')){res.writeHead(404).end();return}
  if(req.url.startsWith('/desktop/tasks?')){res.end('[]');return}
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(15000)
  await page.waitForFunction(()=>document.querySelector('#setup')?.hidden===true)

  // Record what reaches the platform without replacing it: the real constructor
  // still runs, so this asserts a banner was genuinely raised.
  await page.evaluate(()=>{
   window.__raised=[]
   const Real=window.Notification
   class Spy extends Real{constructor(title,options){window.__raised.push({title,...options});super(title,options)}}
   Object.defineProperty(Spy,'permission',{get:()=>Real.permission})
   Spy.requestPermission=Real.requestPermission?.bind(Real)
   window.Notification=Spy
  })
  // The run is already known, so this is a transition and not a first sighting.
  runs[0].waitingSince=new Date().toISOString()
  await page.waitForFunction(()=>window.__raised?.length>0)
  const raised=await page.evaluate(()=>window.__raised)
  assert.equal(raised.length,1,'expected exactly one banner, got '+JSON.stringify(raised))
  assert.match(raised[0].body,/waiting for you/)
  assert.match(raised[0].icon,/^data:image\/svg\+xml;base64,/)
  // The glyph must be the shared waiting definition, not any other state's.
  const {runStateIconDataUrl}=await import('../../shared/runStates.ts')
  assert.equal(raised[0].icon,runStateIconDataUrl('waiting'))

  // Several more polls of the same state must stay silent.
  await page.waitForTimeout(5000)
  assert.equal((await page.evaluate(()=>window.__raised.length)),1,'a repeated poll raised the banner again')

  // A session Sectile did not launch is announced from its own report.
  alerts=[{session:'other-worktree',state:'waiting',at:new Date().toISOString()}]
  await page.waitForFunction(()=>window.__raised.length>1)
  const second=(await page.evaluate(()=>window.__raised))[1]
  assert.equal(second.title,'other-worktree')
  assert.match(second.body,/waiting for you/)

  // And the turn ending raises the other banner, with the other glyph.
  delete runs[0].waitingSince;runs[0].status='completed'
  await page.waitForFunction(()=>window.__raised.length>2)
  const third=(await page.evaluate(()=>window.__raised))[2]
  assert.match(third.body,/finished its turn/)
  assert.equal(third.icon,runStateIconDataUrl('completed'))
 } finally {
  await app?.close().catch(()=>{})
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})
