const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http')
const fs=require('node:fs'),os=require('node:os'),path=require('node:path')
test('Desktop update states remain visible offline and actions use explicit IPC',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-updates-'))
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status')res.end(JSON.stringify({connected:true,server:'https://example.test'}))
  else if(req.url==='/desktop/version')res.end(JSON.stringify({version:'v0.6.0'}))
  else res.end('[]')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  await app.evaluate(({ipcMain})=>{
   globalThis.updateActions=[]
   ipcMain.removeHandler('check-desktop-updates');ipcMain.removeHandler('open-desktop-update')
   ipcMain.handle('check-desktop-updates',()=>{globalThis.updateActions.push('check')})
   ipcMain.handle('open-desktop-update',(_,url)=>{globalThis.updateActions.push(url)})
  })
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  await page.locator('#settings').click();await page.getByRole('tab',{name:'Changelog',exact:true}).click()
  const send=state=>app.evaluate(({BrowserWindow},value)=>BrowserWindow.getAllWindows()[0].webContents.send('desktop-updates',value),state)
  const panel=page.getByRole('region',{name:'Desktop updates'})
  await expect(panel.getByRole('button',{name:'Check for updates'})).toBeEnabled()
  await panel.getByRole('button',{name:'Check for updates'}).click()
  for(const [status,text] of [['checking','Checking for updates'],['up-to-date','is up to date'],['unavailable','no download'],['uncomparable','cannot be compared'],['error','check failed']]){
   await send({status,error:status==='error'?'throttled':null})
   await expect(panel.getByRole('status')).toContainText(text)
   if(status==='checking')await expect(panel.getByRole('button',{name:'Check for updates'})).toBeDisabled()
  }
  const available={version:'0.10.0',releaseUrl:'https://github.com/sebastienferry/sectile/releases/tag/v0.10.0',downloadUrl:'https://github.com/sebastienferry/sectile/releases/download/v0.10.0/sectile-desktop-darwin-arm64.zip'}
  await send({status:'available',available,checkedAt:1000})
  await expect(page.locator('.desktop-update-marker')).toHaveJSProperty('hidden',false)
  await expect(panel).toContainText('0.10.0')
  await panel.getByRole('button',{name:'Release page',exact:true}).click()
  await panel.getByRole('button',{name:'Download Desktop',exact:true}).click()
  assert.deepEqual(await app.evaluate(()=>globalThis.updateActions),['check',available.releaseUrl,available.downloadUrl])
  await send({status:'error',error:'network',available,checkedAt:1000})
  await expect(panel).toContainText('Last known information')
  await expect(page.locator('.desktop-update-marker')).toHaveJSProperty('hidden',false)
  await expect(page.locator('.changelog-release').first()).toBeVisible()
  await expect(page.locator('.settings-versions .version-value').first()).toHaveText(require('../package.json').version)
  await page.getByRole('button',{name:'Back',exact:true}).click()
  await expect(page.locator('.desktop-update-marker')).toBeVisible()
  await send({status:'up-to-date',available:null})
  await expect(page.locator('.desktop-update-marker')).toBeHidden()
 }finally{if(app)await app.close();await new Promise(resolve=>server.close(resolve));fs.rmSync(root,{recursive:true,force:true})}
})
