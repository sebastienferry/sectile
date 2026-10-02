const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// Submitting the connect form ends by spawning the agent, detached, so it would
// outlive the test and keep talking to a server that is gone. These tests stop
// at the pairing exchange, which happens before the spawn: the agent is named as
// a file that does not exist, so the spawn fails and no process is left behind,
// whether the test passes or not.
function pairingEnv(root){
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1',SECTILE_DESKTOP_TEST_AGENT_BINARY:path.join(root,'no-agent')}
 delete env.ELECTRON_RUN_AS_NODE
 return env
}

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
 const env=pairingEnv(root)
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

// A pairing code is single use and expires within ten minutes. Spending one on
// an address the application was going to reject anyway costs the user a round
// trip to the web interface, so the address is checked first.
test('a malformed server address is refused before the pairing code is spent',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-pairing-url-'))
 const pairRequests=[]
 const server=http.createServer((req,res)=>{
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/api/v1/agent/pair'&&req.method==='POST'){
   let raw='';req.on('data',data=>raw+=data);req.on('end',()=>{
    pairRequests.push(JSON.parse(raw))
    res.writeHead(201,{'Content-Type':'application/json'})
    res.end(JSON.stringify({token:'device-token',deviceId:'dev_1',userId:'default'}))
   });return
  }
  if(url.pathname==='/api/v1/agent/projects'){
   if(req.headers.authorization!=='Bearer device-token'){res.writeHead(401).end();return}
   res.writeHead(200,{'Content-Type':'application/json'});res.end(JSON.stringify({projects:[]}));return
  }
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 const port=server.address().port
 const env=pairingEnv(root)
 let application
 try{
  application=await electron.launch({executablePath:process.env.SECTILE_DESKTOP_EXECUTABLE,args:process.env.SECTILE_DESKTOP_EXECUTABLE?[]:[path.resolve(__dirname,'..')],env})
  const page=await application.firstWindow()
  await page.getByRole('heading',{name:'Connect to Sectile'}).waitFor()
  const code=page.getByLabel('Pairing code',{exact:true})
  await page.getByLabel('Sectile server',{exact:true}).fill('http://user:secret@127.0.0.1:'+port)
  await code.fill('code-worth-keeping')
  await page.locator('#start button').click()
  await page.locator('#error').filter({hasText:'HTTP or HTTPS server URL'}).waitFor()
  assert.deepEqual(pairRequests,[],'no pairing request leaves the machine on a rejected address')
  assert.equal(await code.inputValue(),'code-worth-keeping','the code stays in the form, unspent')

  // The very same code now works, which is what "unspent" has to mean.
  await page.getByLabel('Sectile server',{exact:true}).fill('http://127.0.0.1:'+port)
  await page.locator('#start button').click()
  for(let attempt=0;attempt<100&&!pairRequests.length;attempt++)await new Promise(resolve=>setTimeout(resolve,100))
  assert.deepEqual(pairRequests,[{code:'code-worth-keeping',label:os.hostname()}])
 }finally{
  if(application)await application.close().catch(()=>{})
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})

// With no agent running and no key stored, the setup screen says why rather
// than leaving the user to guess why nothing started (#717).
test('launching without a key explains why',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-no-key-'))
 let application
 try{
  application=await electron.launch({executablePath:process.env.SECTILE_DESKTOP_EXECUTABLE,args:process.env.SECTILE_DESKTOP_EXECUTABLE?[]:[path.resolve(__dirname,'..')],env:pairingEnv(root)})
  const page=await application.firstWindow()
  await page.getByRole('heading',{name:'Connect to Sectile'}).waitFor()
  await page.locator('#setup-reason').filter({hasText:'No API key is stored on this workstation. Sign in to connect it.'}).waitFor()
 }finally{
  if(application)await application.close().catch(()=>{})
  fs.rmSync(root,{recursive:true,force:true})
 }
})

test('a sign-in button is offered above the pairing form',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-sign-in-button-'))
 let application
 try{
  application=await electron.launch({executablePath:process.env.SECTILE_DESKTOP_EXECUTABLE,args:process.env.SECTILE_DESKTOP_EXECUTABLE?[]:[path.resolve(__dirname,'..')],env:pairingEnv(root)})
  const page=await application.firstWindow()
  const button=page.getByRole('button',{name:'Sign in with your browser',exact:true})
  await button.waitFor()
  const above=await page.evaluate(()=>{
   const button=document.querySelector('#browser-sign-in'),form=document.querySelector('#start')
   return Boolean(button.compareDocumentPosition(form)&Node.DOCUMENT_POSITION_FOLLOWING)
  })
  assert.ok(above,'the sign-in button comes before the pairing form')
 }finally{
  if(application)await application.close().catch(()=>{})
  fs.rmSync(root,{recursive:true,force:true})
 }
})

// The browser is stood in for by the main process's shell.openExternal: it
// follows the server's /auth/workstation redirect to the loopback callback, as a
// signed-in browser would. The run stops, like the tests above, at the spawn of
// an agent binary that does not exist.
test('browser sign-in end to end',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-browser-sign-in-'))
 const presented=[]
 let paired
 const server=http.createServer((req,res)=>{
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/auth/workstation'){
   const callback='http://127.0.0.1:'+url.searchParams.get('port')+'/callback?'+new URLSearchParams({code:'code-from-the-browser',state:url.searchParams.get('state')})
   res.writeHead(302,{Location:callback}).end();return
  }
  if(url.pathname==='/api/v1/agent/pair'&&req.method==='POST'){
   let raw='';req.on('data',data=>raw+=data);req.on('end',()=>{
    paired=JSON.parse(raw)
    res.writeHead(201,{'Content-Type':'application/json'})
    res.end(JSON.stringify({token:'device-token',deviceId:'dev_new',userId:'default'}))
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
 // A workstation paired before on this server, whose key is gone: its device is
 // what the new key replaces.
 fs.writeFileSync(path.join(root,'settings.json'),JSON.stringify({server:address,deviceId:'dev_old'}),{mode:0o600})
 let application
 try{
  application=await electron.launch({executablePath:process.env.SECTILE_DESKTOP_EXECUTABLE,args:process.env.SECTILE_DESKTOP_EXECUTABLE?[]:[path.resolve(__dirname,'..')],env:pairingEnv(root)})
  await application.evaluate(({shell})=>{
   shell.openExternal=async href=>{
    const response=await fetch(href,{redirect:'manual'})
    await fetch(response.headers.get('location'))
   }
  })
  const page=await application.firstWindow()
  await page.locator('#setup-reason').filter({hasText:'No API key is stored'}).waitFor()
  assert.equal(await page.getByLabel('Sectile server',{exact:true}).inputValue(),address,'the stored server is offered')
  await page.getByRole('button',{name:'Sign in with your browser',exact:true}).click()
  for(let attempt=0;attempt<100&&!presented.includes('Bearer device-token');attempt++)await new Promise(resolve=>setTimeout(resolve,100))
  assert.deepEqual(paired,{code:'code-from-the-browser',label:os.hostname(),deviceId:'dev_old'})
  assert.ok(presented.includes('Bearer device-token'),'the redeemed key is what reaches the server')
  const saved=JSON.parse(fs.readFileSync(path.join(root,'settings.json'),'utf8'))
  assert.equal(saved.apiKey,'device-token')
  assert.equal(saved.deviceId,'dev_new')
  assert.equal(saved.secret,undefined)
 }finally{
  if(application)await application.close().catch(()=>{})
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})
