const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The Changelog settings pane is where somebody goes to find out what they have
// installed. Two versions, because the app and the agent are distributed
// separately, and the release notes right under them. The category is reached
// from the settings sidebar, below the workstation controls.
test('changelog settings report both versions and the release notes',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-settings-version-'))
 let agentVersion={version:'v9.9.9'},agentReachable=true
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://example.test'}));return}
  if(['/desktop/runs','/desktop/projects'].includes(req.url)){res.end('[]');return}
  if(req.url==='/desktop/version'){
   if(!agentReachable){res.writeHead(503).end('{}');return}
   res.end(JSON.stringify(agentVersion));return
  }
  res.writeHead(404).end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 const {version:packagedVersion}=require('../package.json')
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)

  await page.locator('#settings').click()
  await expect(page.getByRole('tab',{name:'User profile',exact:true})).toHaveAttribute('aria-selected','true')
  await expect(page.getByRole('tab')).toHaveText(['User profile','Agent connection','Execution defaults','Agent logs','Changelog'])
  await expect(page.locator('#settings-panel-Profile .setting-name')).toHaveText(['Sectile server','Workstation','Profile and API keys'])
  const bounds=await page.locator('#project-dialog').boundingBox()
  assert.ok(bounds.width>840,'Workstation settings use the enlarged dialog')
  await page.screenshot({path:path.join(root,'settings-profile.png')})
  console.log('Settings screenshot: '+path.join(root,'settings-profile.png'))
  await page.getByRole('tab',{name:'Agent connection',exact:true}).click()
  const actions=page.locator('.settings-agent-actions')
  const start=actions.getByRole('button',{name:'Start agent',exact:true})
  const stop=actions.getByRole('button',{name:'Stop agent',exact:true})
  const restart=actions.getByRole('button',{name:'Restart agent',exact:true})
  await expect(start).toBeDisabled();await expect(stop).toBeEnabled();await expect(restart).toBeEnabled()
  // Mock the process boundary: exercise the controls without touching a real agent.
  await app.evaluate(({ipcMain})=>{
   globalThis.settingsAgentActions=[]
   for(const name of ['start','shutdown','restart','connect'])ipcMain.removeHandler(name)
   ipcMain.handle('connect',()=>false)
   ipcMain.handle('restart',()=>{globalThis.settingsAgentActions.push('restart');return true})
   ipcMain.handle('shutdown',()=>{globalThis.settingsAgentActions.push('stop');return true})
   ipcMain.handle('start',()=>{globalThis.settingsAgentActions.push('start');return true})
  })
  await restart.click();await expect(restart).toBeEnabled()
  await stop.click();await expect(start).toBeEnabled();await expect(stop).toBeDisabled();await expect(restart).toBeDisabled()
  await expect(page.locator('.settings-connection-status')).toHaveText('Unreachable')
  await start.click();await expect(stop).toBeEnabled();await expect(start).toBeDisabled()
  await expect(page.locator('.settings-connection-status')).toHaveText('Connected')
  assert.deepEqual(await app.evaluate(()=>globalThis.settingsAgentActions),['restart','stop','start'])
  await page.screenshot({path:path.join(root,'settings-connection.png')})
  await page.getByRole('tab',{name:'Changelog',exact:true}).click()
  const rows=page.locator('.settings-versions .version-value')
  await expect(rows.nth(0)).toHaveText(packagedVersion)
  await expect(rows.nth(1)).toHaveText('v9.9.9')

  // The changelog is inlined at build time, so it is readable with no network
  // and with the agent stopped.
  const releases=page.locator('.changelog .changelog-release')
  await expect(releases.first()).toBeVisible()
  assert.ok(await releases.count()>0,'no release rendered')
  await expect(page.locator('.changelog-heading')).toContainText('Release notes')
  await expect(page.locator('.changelog').first()).toContainText('Versioning and releases')

  // The panel's only way out is the cross: nothing is saved here, so the
  // footer stays hidden rather than repeating that cross as a bar of its own.
  await expect(page.locator('.dialog-footer')).toBeHidden()
  await page.locator('#close-dialog').click()

  // An agent that is not answering has no version to give. Saying so beats an
  // ellipsis that reads as a load which never finishes.
  agentReachable=false
  await page.locator('#settings').click()
  await expect(page.locator('.settings-versions .version-value').nth(1)).toHaveText('not running')
  await expect(page.locator('.settings-versions .version-value').nth(0)).toHaveText(packagedVersion)
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})

// An agent that is not the binary this app bundles is offered a restart once,
// and marked as outdated in the settings until it is replaced. The dialog is
// answered Cancel: nothing is restarted, and the mark stays.
test('an agent that is not the bundled binary is marked outdated',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-settings-outdated-'))
 const bundled=path.join(root,'sectile-agent')
 fs.writeFileSync(bundled,'bundled agent build')
 const bundledSha=require('node:crypto').createHash('sha256').update('bundled agent build').digest('hex')
 let agentVersion={version:'v9.9.9'},restarts=0
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://example.test'}));return}
  if(['/desktop/runs','/desktop/projects'].includes(req.url)){res.end('[]');return}
  if(req.url==='/desktop/version'){res.end(JSON.stringify(agentVersion));return}
  if(['/desktop/restart','/desktop/shutdown'].includes(req.url)){restarts++;res.end('{}');return}
  res.writeHead(404).end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1',SECTILE_DESKTOP_TEST_AGENT_BINARY:bundled};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  // Record the restart offers and answer them Cancel.
  await app.evaluate(({dialog})=>{
   globalThis.restartOffers=[]
   dialog.showMessageBox=async(_,options)=>{globalThis.restartOffers.push(options.detail);return {response:0}}
  })
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  // A legacy agent cannot say which binary it runs.
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'Changelog',exact:true}).click()
  const mark=page.locator('.settings-versions .version-outdated')
  await expect(mark).toHaveText('outdated: restart the local agent')
  await expect.poll(()=>app.evaluate(()=>globalThis.restartOffers.length)).toBe(1)
  const [detail]=await app.evaluate(()=>globalThis.restartOffers)
  assert.match(detail,/^The running agent is not the one bundled with this app\./)
  assert.equal(restarts,0,'a refused restart restarted the agent')

  // Reconnecting to the same agent does not ask again.
  await page.locator('#close-dialog').click()
  await page.evaluate(()=>window.localAgent.connect())
  await page.waitForTimeout(300)
  assert.equal(await app.evaluate(()=>globalThis.restartOffers.length),1,'the refused restart was offered again')

  // The bundled binary itself carries no mark and no offer.
  agentVersion={version:'v9.9.9',binarySha256:bundledSha}
  await page.evaluate(()=>window.localAgent.connect())
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'Changelog',exact:true}).click()
  await expect(page.locator('.settings-versions .version-value').nth(1)).toHaveText('v9.9.9')
  await expect(mark).toHaveCount(0)
  assert.equal(await app.evaluate(()=>globalThis.restartOffers.length),1)
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
