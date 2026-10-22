const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('engine setup uses direct settings with explicit independent changes and no startup assistant',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-setup-ui-'))
 const deployments=[],mcpWrites=[],permissionWrites=[],engineWrites=[]
 let setupReads=0,defaultEngine='a',reviewer='auto_review',agyMode='plan'
 const catalogue=[{id:'a',name:'Codex',provider:'codex'},{id:'b',name:'Codex alternate',provider:'codex'},{id:'c',name:'Claude',provider:'claude'},{id:'g',name:'Antigravity',provider:'agy'}]
 const body=req=>new Promise(resolve=>{let data='';req.on('data',chunk=>data+=chunk);req.on('end',()=>resolve(JSON.parse(data||'{}')))})
 const server=http.createServer(async(req,res)=>{
  res.setHeader('Content-Type','application/json');const url=new URL(req.url,'http://localhost')
  // An older agent may still offer the removed assistant. Desktop must not
  // read or write its setup workflow, even when it reports pending steps.
  if(url.pathname==='/desktop/setup'){
   setupReads++
   res.end(JSON.stringify({schemaVersion:1,server:'https://sectile.test',provider:url.searchParams.get('provider'),steps:Object.fromEntries(['skills','mcp','permissions','sdd'].map(step=>[step,{status:'pending',fingerprint:step}]))}));return
  }
  if(url.pathname==='/desktop/project'){
   if(req.method==='POST'){
    deployments.push(Object.fromEntries(url.searchParams));const installed=deployments.length>1
    res.end(JSON.stringify({success:installed,skills:{status:installed?'success':'failed',message:installed?'Installed for Codex':'Permission denied'}}));return
   }
   res.end(JSON.stringify({configured:true,path:'/test/repo',server:{projectId:'p',projectName:'Source',specFramework:'openspec',skills:[]}}));return
  }
  if(url.pathname==='/desktop/mcp'){
   if(req.method==='POST'){mcpWrites.push({provider:url.searchParams.get('provider'),...await body(req)})}
   res.end(JSON.stringify({choice:{target:'remote',transport:'http'},path:'/redacted/config'}));return
  }
  if(url.pathname==='/desktop/codex-settings'){
   if(req.method==='PUT'){const input=await body(req);permissionWrites.push({provider:'codex',...input});reviewer=input.approvalsReviewer}
   res.end(JSON.stringify({approvalsReviewer:reviewer}));return
  }
  if(url.pathname==='/desktop/agy-settings'){
   if(req.method==='PUT'){const input=await body(req);permissionWrites.push({provider:'agy',...input});agyMode=input.conversationMode}
   res.end(JSON.stringify({conversationMode:agyMode}));return
  }
  if(url.pathname==='/desktop/projects'){res.end('[{"id":"p","name":"Source","path":"/test/repo"}]');return}
  if(url.pathname==='/desktop/status'){res.end('{"connected":true,"server":"https://sectile.test","capabilities":["task-engines","engine-setup"]}');return}
  if(url.pathname==='/desktop/runs'){res.end('[]');return}
  if(url.pathname==='/desktop/workstation'){res.end('{"globalConfiguration":true,"defaults":{}}');return}
  if(url.pathname==='/desktop/engines'){
   if(req.method==='PUT'){const input=await body(req);engineWrites.push(input);defaultEngine=input.default}
   res.end(JSON.stringify({catalogue,default:defaultEngine}));return
  }
  res.end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'private'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  await expect(page.getByRole('button',{name:'Actions for Source',exact:true})).toBeVisible()
  const absentAssistant=()=>expect(page.locator('.engine-setup-toast,.engine-setup,[data-open-engine-setup]')).toHaveCount(0)
  await absentAssistant()
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'AI engines',exact:true}).click()
  await expect(page.getByRole('list',{name:'Engines',exact:true})).toContainText('Codex')
  assert.deepEqual([deployments,mcpWrites,permissionWrites,engineWrites],[[],[],[],[]],'Opening settings must not configure providers')
  await page.getByRole('button',{name:'Make Claude the default engine',exact:true}).click()
  await expect(page.locator('.engine-item[data-provider="claude"] .engine-default')).toHaveText('Default')
  assert.equal(engineWrites.length,1)
  assert.equal(defaultEngine,'c')
  assert.deepEqual([deployments,mcpWrites,permissionWrites],[[],[],[]],'Selecting the default engine must not install tooling or change permissions')

  await page.getByRole('tab',{name:'Deployment',exact:true}).click()
  await page.getByRole('combobox',{name:'Setup AI engine',exact:true}).selectOption('a')
  const install=page.getByRole('button',{name:'Install skills',exact:true}),results=page.locator('.setup-result')
  await install.click()
  await expect(results.first()).toHaveText('Failed · Permission denied')
  await expect(install).toBeEnabled()
  await install.click()
  await expect(results.first()).toHaveText('Installed · Installed for Codex')
  assert.deepEqual(deployments,[{id:'p',action:'provider-skills',provider:'codex'},{id:'p',action:'provider-skills',provider:'codex'}])
  assert.deepEqual([mcpWrites,permissionWrites],[[],[]],'Installing skills must not register MCP or change permissions')
  await page.getByRole('group',{name:'MCP target',exact:true}).getByRole('button',{name:'Local agent',exact:true}).click()
  await page.getByRole('button',{name:'Register MCP',exact:true}).click()
  await expect(results.nth(1)).toContainText('Registered in /redacted/config')
  assert.deepEqual(mcpWrites,[{provider:'codex',target:'local',transport:'http'}])
  assert.deepEqual(permissionWrites,[])
  assert.equal(deployments.length,2)

  await page.getByRole('tab',{name:'AI engines',exact:true}).click()
  await page.getByRole('tab',{name:'Codex settings',exact:true}).click()
  const approval=page.getByRole('combobox',{name:'Approval reviewer',exact:true})
  await expect(approval).toHaveValue('auto_review')
  await approval.selectOption('user')
  await expect(page.locator('#settings-panel-Codex')).toContainText('Codex settings saved')
  await page.getByRole('tab',{name:'Antigravity settings',exact:true}).click()
  await page.getByRole('combobox',{name:'Antigravity conversation permission mode',exact:true}).selectOption('default')
  await expect(page.locator('#settings-panel-Antigravity')).toContainText('Antigravity settings saved')
  assert.deepEqual(permissionWrites,[{provider:'codex',approvalsReviewer:'user'},{provider:'agy',conversationMode:'default'}])
  await page.getByRole('button',{name:'Back',exact:true}).click()
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'AI engines',exact:true}).click()
  await page.getByRole('tab',{name:'Codex settings',exact:true}).click()
  await expect(approval).toHaveValue('user')
  await page.getByRole('tab',{name:'Antigravity settings',exact:true}).click()
  await expect(page.getByRole('combobox',{name:'Antigravity conversation permission mode',exact:true})).toHaveValue('default')
  assert.equal(permissionWrites.length,2,'Reopening settings must not rewrite provider permissions')
  await absentAssistant()
  assert.equal(setupReads,0,'Direct settings must not enter the removed assistant workflow')
 }finally{await app?.close();await new Promise(resolve=>server.close(resolve));fs.rmSync(root,{recursive:true,force:true})}
})
