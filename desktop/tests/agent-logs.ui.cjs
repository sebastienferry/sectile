const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('agent logs work offline, render literal snapshots and recover from read failures',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-agent-logs-ui-'))
 const file=path.join(root,'agent.log')
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let application
 try{
  application=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await application.firstWindow();page.setDefaultTimeout(7000)
  await page.locator('#setup').waitFor()
  const button=page.getByRole('button',{name:'Agent logs',exact:true})
  await button.focus();await page.keyboard.press('Enter')
  await page.getByText('No desktop agent log exists yet.',{exact:true}).waitFor()
  assert.equal(await page.locator('.agent-log-source').textContent(),file)
  const refresh=page.getByRole('button',{name:'Refresh',exact:true})
  const output=page.getByLabel('Agent log contents')
  fs.writeFileSync(file,'')
  await refresh.click();await page.getByText('The agent log is empty.',{exact:true}).waitFor()
  const literal='<img src=x onerror="window.injected=true">\n\x1b[31mLaunch failed\n'
  fs.writeFileSync(file,literal)
  await refresh.click();await expect(output).toHaveText(literal.replace('\x1b[31m',''))
  assert.equal(fs.readFileSync(file,'utf8'),literal)
  assert.equal(await output.locator('img').count(),0)
  assert.equal(await page.evaluate(()=>window.injected),undefined)
  fs.appendFileSync(file,'Retried\n')
  await refresh.click();await expect(output).toHaveText(literal.replace('\x1b[31m','')+'Retried\n')
  fs.writeFileSync(file,'Earlier\n'.repeat(40000)+'Newest output\n')
  await refresh.click();await page.getByText('Showing the latest 256 KiB; earlier output omitted.',{exact:true}).waitFor()
  assert.ok((await output.textContent()).endsWith('Newest output\n'))
  assert.ok(Buffer.byteLength(await output.textContent())<=256*1024)
  assert.equal(await output.evaluate(el=>Math.abs(el.scrollHeight-el.clientHeight-el.scrollTop)<2),true)
  const bounds=await output.boundingBox(),pane=await page.locator('#agent-log-pane').boundingBox(),sidebar=await page.locator('aside').boundingBox()
  assert.ok(pane.x>=sidebar.x+sidebar.width,'Logs stay beside the sidebar')
  assert.ok(bounds.height>400,'Logs use available height')
  assert.ok(bounds.y+bounds.height<=pane.y+pane.height,'Output stays inside workspace')
  assert.equal(await page.locator('#project-dialog').evaluate(el=>el.open),false)
  await expect(page.getByRole('button',{name:'Close logs',exact:true})).toBeVisible()
  await output.focus();assert.equal(await output.evaluate(el=>el===document.activeElement),true)
  await page.screenshot({path:path.join(root,'agent-logs.png')})
  console.log('Agent logs screenshot: '+path.join(root,'agent-logs.png'))
  fs.unlinkSync(file);fs.mkdirSync(file)
  await refresh.click();await page.getByText('Unable to read agent log:',{exact:false}).waitFor()
  await expect(output).toHaveText('');await expect(refresh).toBeEnabled()
  fs.rmdirSync(file);fs.writeFileSync(file,'Recovered\n')
  await refresh.click();await expect(output).toHaveText('Recovered\n')
  await page.keyboard.press('Escape')
  await expect(page.locator('#agent-log-pane')).not.toBeVisible()
  await expect(page.locator('#setup')).toBeVisible()
  await expect(button).toBeFocused()
 }finally{if(application)await application.close()}
})

test('agent logs preserve the connected execution and ignore late reads after another dialog opens',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-agent-logs-connected-'))
 fs.writeFileSync(path.join(root,'agent.log'),'Connected diagnostic\n')
 let mutations=0,runStatus='failed',available=true
 const server=http.createServer((req,res)=>{
  if(req.method!=='GET')mutations++
  res.setHeader('Content-Type','application/json')
  if(!available){res.writeHead(503).end();return}
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify([{id:'run',taskId:'task',taskKey:'#71',projectId:'p',skill:'pickup',status:runStatus}]));return}
  if(req.url==='/desktop/projects'){res.end('[]');return}
  if(req.url.startsWith('/desktop/tasks?')){res.end(JSON.stringify([{id:'task',key:'#71',labels:['#new']}]));return}
  if(req.url==='/desktop/project?id=p'){res.end(JSON.stringify({server:{skills:[]}}));return}
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let application
 try{
  application=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await application.firstWindow();page.setDefaultTimeout(7000)
  await expect(page.locator('#title')).toHaveText('#71 · pickup')
  const selected=await page.locator('#title').textContent()
  await page.getByRole('button',{name:'Agent logs',exact:true}).click()
  await expect(page.getByLabel('Agent log contents')).toHaveText('Connected diagnostic\n')
  runStatus='running'
  await page.locator('.run[data-status=running]').waitFor()
  await expect(page.locator('#agent-log-pane')).toBeVisible()
  const pane=page.locator('#agent-log-pane')
  const sidebar=page.locator('aside'),width=(await sidebar.boundingBox()).width
  await page.locator('#sidebar-resizer').focus();await page.keyboard.press('ArrowRight')
  assert.ok((await sidebar.boundingBox()).width>width)
  const resized=(await sidebar.boundingBox()).width
  await page.locator('#toggle-sidebar').click();await expect(sidebar).not.toBeVisible()
  await page.getByRole('button',{name:'Close logs',exact:true}).click()
  await expect(page.getByRole('button',{name:'Agent logs',exact:true})).toBeFocused()
  await expect(page.locator('#workspace article')).toBeVisible()
  await page.getByRole('button',{name:'Agent logs',exact:true}).click()
  await expect(sidebar).not.toBeVisible()
  await page.locator('#toggle-sidebar').click();await expect(sidebar).toBeVisible()
  assert.equal((await sidebar.boundingBox()).width,resized)
  await page.locator('.run.selected').click()
  await expect(pane).not.toBeVisible()
  assert.equal(await page.locator('#title').textContent(),selected)
  assert.equal(mutations,0)
  await page.getByRole('button',{name:'Agent logs',exact:true}).click()
  await page.getByRole('button',{name:'Local agent',exact:true}).click()
  await expect(page.locator('#setup')).toBeVisible()
  await expect(pane).not.toBeVisible()
  await page.getByRole('button',{name:'Local agent',exact:true}).click()
  await application.evaluate(({ipcMain})=>{
   ipcMain.removeHandler('agent-logs')
   ipcMain.handle('agent-logs',()=>new Promise(resolve=>{globalThis.completeLog=()=>resolve({path:'late.log',text:'Late result'})}))
  })
  await page.getByRole('button',{name:'Agent logs',exact:true}).click()
  await page.getByText('Loading agent log…',{exact:true}).waitFor()
  await expect(page.getByRole('button',{name:'Refresh',exact:true})).toBeDisabled()
  await page.keyboard.press('Escape')
  await page.getByRole('button',{name:'Profile',exact:true}).click()
  await application.evaluate(()=>globalThis.completeLog())
  await expect(page.locator('#dialog-body h2')).toHaveText('Profile')
  assert.equal(await page.getByLabel('Agent log contents').count(),0)
  assert.equal(mutations,0)
  await page.keyboard.press('Escape')
  await application.evaluate(({ipcMain})=>{
   ipcMain.removeHandler('agent-logs');ipcMain.handle('agent-logs',()=>({path:'agent.log',text:'Offline diagnostic'}))
  })
  await page.getByRole('button',{name:'Agent logs',exact:true}).click()
  available=false
  await expect(page.locator('#connection')).toHaveText('Local agent stopped')
  await expect(pane).toBeVisible()
  await expect(page.getByLabel('Agent log contents')).toHaveText('Offline diagnostic')
  await page.keyboard.press('Escape')
  await expect(page.locator('#setup')).toBeVisible()
 }finally{
  if(application)await application.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
