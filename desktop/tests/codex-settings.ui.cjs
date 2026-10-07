const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('Codex settings persists the reviewer without changing sandbox settings',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-codex-settings-'))
 let reviewer='user',refuse=false
 const saved=[]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,capabilities:['codex-settings']}));return}
  if(req.url==='/desktop/projects'||req.url==='/desktop/runs'){res.end('[]');return}
  if(req.url==='/desktop/codex-settings'){
   if(req.method==='GET'){res.end(JSON.stringify({approvalsReviewer:reviewer}));return}
   let body='';req.on('data',data=>body+=data);req.on('end',()=>{
    const value=JSON.parse(body);saved.push(value)
    if(refuse){res.writeHead(500).end('Refused');return}
    reviewer=value.approvalsReviewer;res.end(JSON.stringify({approvalsReviewer:reviewer}))
   });return
  }
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(8000)
  await page.locator('#settings').click()
  await expect(page.getByRole('tab',{name:'General',exact:true}).first()).toHaveAttribute('aria-selected','true')
  await expect(page.getByRole('tab',{name:'User profile',exact:true})).toHaveCount(0)
  await expect(page.getByRole('tab',{name:'Appearance',exact:true})).toHaveCount(0)
  const general=page.locator('#settings-panel-Profile')
  await expect(general).toContainText('Profile and API keys');await expect(general).toContainText('Theme')
  await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].setBounds({width:1600,height:900}))
  await expect.poll(async()=>Math.round((await general.boundingBox()).width)).toBeGreaterThan(1200)
  await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].setBounds({width:1000,height:750}))
  await expect.poll(async()=>Math.round((await general.boundingBox()).width)).toBeLessThan(900)
  const search=page.getByRole('searchbox',{name:'Search settings'})
  await search.fill('approval reviewer')
  await expect(page.getByRole('combobox',{name:'Approval reviewer',exact:true})).toBeVisible()
  await expect(general).toBeHidden()
  await search.fill('nothing-matches-this-setting')
  await expect(page.getByRole('status').filter({hasText:'No settings found'})).toBeVisible()
  await search.fill('theme')
  await expect(page.getByRole('group',{name:'Appearance',exact:true})).toBeVisible()
  await expect(general.locator('.setting-row').filter({hasText:'Profile and API keys'})).toBeHidden()
  await search.fill('')
  await expect(general.locator('.setting-row').filter({hasText:'Profile and API keys'})).toBeVisible()
  await page.getByRole('tab',{name:'Codex settings',exact:true}).click()
  const select=page.getByRole('combobox',{name:'Approval reviewer',exact:true})
  await expect(select).toBeEnabled();await expect(select).toHaveValue('user')
  await select.selectOption('auto_review')
  await expect(page.getByRole('tabpanel',{name:'Codex settings'})).toContainText('Codex settings saved')
  assert.deepEqual(saved,[{approvalsReviewer:'auto_review'}])
  await page.keyboard.press('Escape')
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'Codex settings',exact:true}).click()
  await expect(select).toHaveValue('auto_review')
  refuse=true;await select.selectOption('user')
  await expect(page.getByRole('tabpanel',{name:'Codex settings'})).toContainText('Not saved:')
  assert.equal(reviewer,'auto_review')
  await expect(select).toHaveValue('auto_review')
 }finally{if(app)await app.close();await new Promise(resolve=>server.close(resolve))}
})
