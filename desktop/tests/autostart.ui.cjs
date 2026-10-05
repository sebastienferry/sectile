const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// After a reboot no agent runs, so the desktop starts it itself with the key it
// saved (#716). The agent is named as a file that does not exist: a start that
// gets as far as the spawn fails there, safely, and no process outlives a test.
function autostartEnv(root){
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1',SECTILE_DESKTOP_TEST_AGENT_BINARY:path.join(root,'no-agent')}
 delete env.ELECTRON_RUN_AS_NODE
 return env
}
const launch=env=>electron.launch({executablePath:process.env.SECTILE_DESKTOP_EXECUTABLE,args:process.env.SECTILE_DESKTOP_EXECUTABLE?[]:[path.resolve(__dirname,'..')],env})
const listen=server=>new Promise(resolve=>server.listen(0,'127.0.0.1',()=>resolve('http://127.0.0.1:'+server.address().port)))
// An address nothing listens on: a port that was just released.
async function closedAddress(){
 const server=http.createServer()
 const address=await listen(server)
 await new Promise(resolve=>server.close(resolve))
 return address
}
// A Sectile server that records the key presented where checkServer probes it.
function sectileServer(status){
 const presented=[]
 const server=http.createServer((req,res)=>{
  presented.push(req.headers.authorization)
  if(new URL(req.url,'http://localhost').pathname==='/api/v1/agent/projects'&&status===200){res.writeHead(200,{'Content-Type':'application/json'});res.end(JSON.stringify({projects:[]}));return}
  res.writeHead(status===200?404:status,{'Content-Type':'application/json'}).end('{}')
 })
 return {server,presented}
}
// The state a reboot leaves: a saved key and the info file of an agent that is gone.
async function rebootedData(root,server){
 if(server)fs.writeFileSync(path.join(root,'settings.json'),JSON.stringify({server,apiKey:'stored-key'}),{mode:0o600})
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:await closedAddress(),token:'gone'}),{mode:0o600})
}
async function waitFor(condition){
 for(let attempt=0;attempt<100&&!condition();attempt++)await new Promise(resolve=>setTimeout(resolve,100))
}

test('a saved key the server accepts starts the agent unasked, and a failed start leaves the control clickable',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-autostart-ok-'))
 const {server,presented}=sectileServer(200)
 const address=await listen(server)
 await rebootedData(root,address)
 let application
 try{
  application=await launch(autostartEnv(root))
  const page=await application.firstWindow();page.setDefaultTimeout(10000)
  await waitFor(()=>presented.includes('Bearer stored-key'))
  assert.ok(presented.includes('Bearer stored-key'),'the saved key reaches the server with no click')
  // The spawn fails: the reason is in the form, and the control is clickable again.
  const reason=page.locator('#start .start-reason')
  await expect(reason).toContainText(/ENOENT|no-agent/)
  const button=page.locator('#start button[type=submit]')
  await expect(button).toHaveText('Start local agent')
  await expect(button).toBeEnabled()
  await expect(page.locator('#pair-again')).toHaveJSProperty('open',false)
  await expect(page.locator('#pair-again summary')).toHaveText('Pair again')
 }finally{
  if(application)await application.close().catch(()=>{})
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})

test('a saved key the server refuses asks for a new pairing, with the reason',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-autostart-401-'))
 const {server,presented}=sectileServer(401)
 const address=await listen(server)
 await rebootedData(root,address)
 let application
 try{
  application=await launch(autostartEnv(root))
  const page=await application.firstWindow();page.setDefaultTimeout(10000)
  const reason=page.locator('#start .start-reason')
  await expect(reason).toHaveText('This API key was revoked or is unknown. Pair again to get a new key.')
  assert.ok(presented.includes('Bearer stored-key'))
  await expect(page.locator('#pair-again')).toHaveJSProperty('open',true)
  await expect(page.getByLabel('Pairing code',{exact:true})).toBeVisible()
  // Signing in through the browser is the other way to pair again (#717).
  await expect(page.getByRole('button',{name:'Sign in with your browser',exact:true})).toBeEnabled()
  const button=page.locator('#start button[type=submit]')
  await expect(button).toHaveText('Connect')
  await expect(button).toBeEnabled()
  // A sign-in that fails after the refusal keeps the pairing form open: no browser opens here, and the stand-in fails.
  await application.evaluate(({shell})=>{shell.openExternal=()=>new Promise((_,reject)=>setTimeout(()=>reject(Error('no browser in this test')),1500))})
  const signIn=page.getByRole('button',{name:'Sign in with your browser',exact:true})
  await signIn.click()
  await expect(reason).toContainText('Waiting for the sign-in')
  await expect(reason).toContainText('revoked')
  await expect(page.locator('#pair-again')).toHaveJSProperty('open',true)
  await expect(signIn).toBeDisabled()
  await expect(reason).toContainText('Could not open the browser')
  await expect(reason).toContainText('revoked')
  await expect(reason).not.toContainText('Waiting')
  await expect(page.locator('#pair-again')).toHaveJSProperty('open',true)
  await expect(page.getByLabel('Pairing code',{exact:true})).toBeVisible()
  await expect(button).toHaveText('Connect')
  await expect(button).toBeEnabled()
  await expect(signIn).toBeEnabled()
 }finally{
  if(application)await application.close().catch(()=>{})
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})

test('an unreachable server does not ask for a pairing: the agent start is attempted anyway',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-autostart-offline-'))
 await rebootedData(root,await closedAddress())
 let application
 try{
  application=await launch(autostartEnv(root))
  const page=await application.firstWindow();page.setDefaultTimeout(10000)
  // The spawn failure is what is shown, not a pairing request.
  const reason=page.locator('#start .start-reason')
  await expect(reason).toContainText(/ENOENT|no-agent/)
  await expect(reason).not.toContainText('Pair again')
  await expect(page.locator('#pair-again')).toHaveJSProperty('open',false)
  const button=page.locator('#start button[type=submit]')
  await expect(button).toHaveText('Start local agent')
  await expect(button).toBeEnabled()
 }finally{
  if(application)await application.close().catch(()=>{})
  fs.rmSync(root,{recursive:true,force:true})
 }
})

test('with no saved key the pairing form says so and nothing is sent',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-autostart-nokey-'))
 const {server,presented}=sectileServer(200)
 await listen(server)
 await rebootedData(root,null)
 let application
 try{
  application=await launch(autostartEnv(root))
  const page=await application.firstWindow();page.setDefaultTimeout(10000)
  await expect(page.locator('#start .start-reason')).toHaveText('This workstation has no saved key yet. Paste a pairing code from your profile in the web interface.')
  await expect(page.getByLabel('Pairing code',{exact:true})).toBeVisible()
  const button=page.locator('#start button[type=submit]')
  await expect(button).toHaveText('Connect')
  await expect(button).toBeEnabled()
  assert.deepEqual(presented,[],'no request without a key')
 }finally{
  if(application)await application.close().catch(()=>{})
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})

// The confirmed stale control: a stop rendered Settings while the stop was still
// in progress, and the Start agent button stayed disabled for good.
test('Settings offers Start agent once a stop has finished',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-autostart-stop-'))
 const info=path.join(root,'agent-connection.json')
 let stopped=false
 const agent=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(stopped){res.writeHead(503).end('{}');return}
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/shutdown'){stopped=true;fs.rmSync(info,{force:true});res.writeHead(204).end();return}
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://example.test'}));return}
  if(['/desktop/runs','/desktop/projects'].includes(url.pathname)){res.end('[]');return}
  if(url.pathname==='/desktop/version'){res.end('{"version":"test"}');return}
  res.writeHead(404).end('{}')
 })
 const address=await listen(agent)
 fs.writeFileSync(info,JSON.stringify({url:address,token:'test-secret'}),{mode:0o600})
 let application
 try{
  application=await launch(autostartEnv(root))
  const page=await application.firstWindow();page.setDefaultTimeout(10000)
  // The stop confirmation is a native dialog: it is answered here.
  await application.evaluate(({dialog})=>{dialog.showMessageBox=async()=>({response:1})})
  await page.locator('#shutdown').click()
  await page.getByText('Local agent is stopped',{exact:true}).waitFor()
  assert.ok(stopped)
  await expect(page.locator('#start button[type=submit]')).toBeEnabled()
  // The sidebar is hidden while the agent is stopped: Settings opens from the setup screen.
  await page.locator('#setup-logs').click()
  await page.getByRole('tab',{name:'Agent connection',exact:true}).click()
  const actions=page.locator('.settings-agent-actions')
  await expect(actions.getByRole('button',{name:'Stop agent',exact:true})).toBeDisabled()
  await expect(actions.getByRole('button',{name:'Start agent',exact:true})).toBeEnabled()
  await expect(page.locator('#start button[type=submit]')).toBeEnabled()
 }finally{
  if(application)await application.close().catch(()=>{})
  await new Promise(resolve=>agent.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
