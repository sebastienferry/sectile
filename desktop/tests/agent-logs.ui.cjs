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
  application=await electron.launch({args:[path.resolve(__dirname,'..')],env,colorScheme:'dark'})
  const page=await application.firstWindow();page.setDefaultTimeout(7000)
  await page.locator('#setup').waitFor()
  // A stopped agent hides the sidebar, so the connection screen carries the one
  // entry point to the diagnostics that explain why it is stopped.
  const button=page.getByRole('button',{name:'Agent logs',exact:true})
  await button.focus();await page.keyboard.press('Enter')
  await expect(page.getByRole('tab',{name:'Agent logs',exact:true})).toHaveAttribute('aria-selected','true')
  await page.getByText('No desktop agent log exists yet.',{exact:true}).waitFor()
  await page.getByRole('tab',{name:'Agent connection',exact:true}).click()
  await expect(page.locator('.settings-connection-status')).toHaveText('Unreachable')
  await expect(page.locator('.settings-connection-status .connection-dot')).toHaveCSS('background-color','rgb(255, 155, 0)')
  await page.getByRole('tab',{name:'Agent logs',exact:true}).click()
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
  const bounds=await output.boundingBox(),panel=await page.locator('#project-dialog').boundingBox()
  assert.equal(await page.locator('#project-dialog').evaluate(el=>el.open),true)
  assert.ok(bounds.height>150,'Logs stretch to the height the panel can spare')
  assert.ok(bounds.y+bounds.height<=panel.y+panel.height+1,'Output stays inside the dialog')
  await output.focus();assert.equal(await output.evaluate(el=>el===document.activeElement),true)
  await page.screenshot({path:path.join(root,'agent-logs.png')})
  console.log('Agent logs screenshot: '+path.join(root,'agent-logs.png'))
  fs.unlinkSync(file);fs.mkdirSync(file)
  await refresh.click();await page.getByText('Unable to read agent log:',{exact:false}).waitFor()
  await expect(output).toHaveText('');await expect(refresh).toBeEnabled()
  fs.rmdirSync(file);fs.writeFileSync(file,'Recovered\n')
  await refresh.click();await expect(output).toHaveText('Recovered\n')
  await page.keyboard.press('Escape')
  await expect(page.getByLabel('Agent log contents')).toHaveCount(0)
  await expect(page.locator('#setup')).toBeVisible()
  await expect(button).toBeFocused()
 }finally{if(application)await application.close()}
})

test('agent logs leave the console alone and ignore late reads after the panel moves on',async()=>{
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
  // The panel is a dialog over the workspace, not a pane replacing it: the
  // selected execution and its console survive a look at the diagnostics.
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'Agent logs',exact:true}).click()
  await expect(page.getByLabel('Agent log contents')).toHaveText('Connected diagnostic\n')
  runStatus='running'
  await page.locator('.run[data-status=running]').waitFor()
  await page.keyboard.press('Escape')
  assert.equal(await page.locator('#title').textContent(),selected)
  await expect(page.locator('#workspace article')).toBeVisible()
  assert.equal(mutations,0)
  // The connect form is one tab away, and returns to the connection screen when
  // the panel closes.
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'Agent connection',exact:true}).click()
  await expect(page.getByRole('tab',{name:'Agent logs',exact:true})).toHaveAttribute('aria-selected','false')
  await expect(page.locator('#project-dialog #start')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.locator('#setup #start')).toHaveCount(1)
  await expect(page.locator('#workspace')).toBeVisible()
  // A read that lands after the panel closed writes into a detached node.
  await application.evaluate(({ipcMain})=>{
   ipcMain.removeHandler('agent-logs')
   ipcMain.handle('agent-logs',()=>new Promise(resolve=>{globalThis.completeLog=()=>resolve({path:'late.log',text:'Late result'})}))
  })
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'Agent logs',exact:true}).click()
  await page.getByText('Loading agent log…',{exact:true}).waitFor()
  await expect(page.getByRole('button',{name:'Refresh',exact:true})).toBeDisabled()
  await page.keyboard.press('Escape')
  await page.locator('#settings').click()
  await application.evaluate(()=>globalThis.completeLog())
  await expect(page.locator('#dialog-body h2')).toHaveText('Settings')
  await expect(page.getByRole('tab',{name:'User profile',exact:true})).toHaveAttribute('aria-selected','true')
  assert.equal(await page.getByText('Late result',{exact:false}).count(),0)
  await expect(page.getByLabel('Agent log contents')).toHaveText('')
  assert.equal(mutations,0)
  await page.keyboard.press('Escape')
  await application.evaluate(({ipcMain})=>{
   ipcMain.removeHandler('agent-logs');ipcMain.handle('agent-logs',()=>({path:'agent.log',text:'Offline diagnostic'}))
  })
  // A dropped agent falls back to the connection screen, which still reaches
  // the diagnostics that say why it dropped.
  available=false
  await expect(page.locator('#connection')).toHaveText('Not connected')
  await expect(page.locator('#setup')).toBeVisible()
  await page.getByRole('button',{name:'Agent logs',exact:true}).click()
  await expect(page.getByLabel('Agent log contents')).toHaveText('Offline diagnostic')
  await page.keyboard.press('Escape')
  await expect(page.locator('#setup')).toBeVisible()
 }finally{
  if(application)await application.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
