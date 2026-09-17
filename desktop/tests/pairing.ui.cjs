const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The connect form is the only place a pairing code can be spent, so this is the
// test that keeps the code path reachable: the exchange used to exist and be
// tested, while no interface ever called it.
test('a pairing code typed in the connect form is exchanged for a device token',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-pairing-'))
 const presented=[]
 let paired
 const server=http.createServer((req,res)=>{
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/api/v1/agent/pair'&&req.method==='POST'){
   let raw='';req.on('data',data=>raw+=data);req.on('end',()=>{
    paired=JSON.parse(raw)
    res.writeHead(201,{'Content-Type':'application/json'})
    res.end(JSON.stringify({token:'device-token',deviceId:'dev_1',userId:'default'}))
   });return
  }
  if(url.pathname==='/api/v1/agent/projects'){
   presented.push(req.headers.authorization)
   if(req.headers.authorization!=='Bearer device-token'){res.writeHead(401).end();return}
   res.writeHead(200,{'Content-Type':'application/json'});res.end(JSON.stringify({projects:[]}));return
  }
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 const address='http://127.0.0.1:'+server.address().port
 // No agent-connection.json: the app has nothing to connect to and shows the form.
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let application
 try{
  application=await electron.launch({executablePath:process.env.SECTILE_DESKTOP_EXECUTABLE,args:process.env.SECTILE_DESKTOP_EXECUTABLE?[]:[path.resolve(__dirname,'..')],env})
  const page=await application.firstWindow()
  await page.getByRole('heading',{name:'Connect to Sectile'}).waitFor()
  await page.getByLabel('Sectile server',{exact:true}).fill(address)
  await page.getByLabel('Pairing code',{exact:true}).fill('code-from-the-web-interface')
  // With no agent running the form's button reads 'Start local agent'; target the
  // form's own button rather than a label that depends on the agent's state.
  await page.locator('#start button').click()
  // The exchange and the credential check both happen before the agent binary is
  // spawned, so they are observable even though no agent can start here.
  for(let attempt=0;attempt<100&&!(paired&&presented.includes('Bearer device-token'));attempt++){
   await new Promise(resolve=>setTimeout(resolve,100))
  }
  assert.deepEqual(paired,{code:'code-from-the-web-interface',label:os.hostname()})
  assert.ok(presented.includes('Bearer device-token'),'the exchanged token is what reaches the server, not the code')
  assert.ok(!presented.includes('Bearer code-from-the-web-interface'),'a pairing code must never be sent as a bearer token')
 }finally{
  if(application)await application.close().catch(()=>{})
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})
