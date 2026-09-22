const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The General settings pane is where somebody goes to find out what they have
// installed. Two versions, because the app and the agent are distributed
// separately, and the release notes right under them. It is the category the
// settings panel opens on, reached from the gear at the foot of the sidebar.
test('general settings report both versions and the release notes',async()=>{
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
  await expect(page.getByRole('tab',{name:'General',exact:true})).toHaveAttribute('aria-selected','true')
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
