const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('guided setup retains outcomes through failure, retry, skip and reopen without escalating permissions',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-setup-ui-'))
 const writes=[],permissions=[]
 let releaseMCP
 const steps={skills:{status:'pending',fingerprint:'skills'},mcp:{status:'pending',fingerprint:'mcp'},permissions:{status:'pending',fingerprint:'permissions'},sdd:{status:'pending',fingerprint:'sdd'}}
 const providerSteps={codex:steps,claude:structuredClone(steps),agy:structuredClone(steps)}
 let claudeMode='default',agyMode='plan'
 const view=(provider='codex')=>({schemaVersion:1,server:'https://sectile.test',provider,steps:providerSteps[provider],permissions:provider==='claude'?{mode:claudeMode,sandbox:{platformSandbox:false,claudeSandbox:{},projects:[]},projectSandbox:{}}:provider==='agy'?{conversationMode:agyMode}:{approvalsReviewer:'user'},complete:Object.values(providerSteps[provider]).every(step=>['success','skipped'].includes(step.status))})
 const body=req=>new Promise(resolve=>{let data='';req.on('data',chunk=>data+=chunk);req.on('end',()=>resolve(JSON.parse(data||'{}')))})
 const server=http.createServer(async(req,res)=>{
  res.setHeader('Content-Type','application/json');const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/setup'){
   if(req.method==='POST'){const input=await body(req);writes.push(input);providerSteps[url.searchParams.get('provider')][input.step].status=input.decision==='retained'?'success':'skipped'}
   res.end(JSON.stringify(view(url.searchParams.get('provider'))));return
  }
  if(url.pathname==='/desktop/project'){
   if(req.method==='POST'){
    writes.push(Object.fromEntries(url.searchParams));const count=writes.filter(write=>write.action==='provider-skills').length
    if(count>1)steps.skills.status='success'
    res.end(JSON.stringify({success:count>1,skills:{status:count>1?'success':'failed',message:'failure'}}));return
   }
   res.end(JSON.stringify({configured:true,path:'/test/repo',server:{projectId:'p',projectName:'Source',specFramework:'openspec',skills:[]}}));return
  }
  if(url.pathname==='/desktop/mcp'){
   if(req.method==='POST'){writes.push(await body(req));steps.mcp.status='success';await new Promise(resolve=>{releaseMCP=resolve})}
   res.end(JSON.stringify({choice:{target:'remote',transport:'http'},path:'/redacted/config'}));return
  }
  if(url.pathname==='/desktop/codex-settings'){
   if(req.method==='PUT')permissions.push(await body(req))
   res.end('{"approvalsReviewer":"user"}');return
  }
  if(url.pathname==='/desktop/conversation-mode'){const input=await body(req);claudeMode=input.mode;permissions.push(input);providerSteps.claude.permissions.status='pending';res.end(JSON.stringify({mode:claudeMode}));return}
  if(url.pathname==='/desktop/agy-settings'){if(req.method==='PUT'){const input=await body(req);agyMode=input.conversationMode;permissions.push(input);providerSteps.agy.permissions.status='pending'}res.end(JSON.stringify({conversationMode:agyMode}));return}
  if(url.pathname==='/desktop/projects'){res.end('[{"id":"p","name":"Source","path":"/test/repo"}]');return}
  if(url.pathname==='/desktop/status'){res.end('{"connected":true,"server":"https://sectile.test","capabilities":["engine-setup"]}');return}
  if(url.pathname==='/desktop/runs'){res.end('[]');return}
  if(url.pathname==='/desktop/settings'){res.end('{}');return}
  if(url.pathname==='/desktop/workstation'){res.end('{"globalConfiguration":true,"defaults":{}}');return}
  if(url.pathname==='/desktop/engines'){res.end('{"catalogue":[{"id":"a","name":"Codex","provider":"codex"},{"id":"b","name":"Codex alternate","provider":"codex"},{"id":"c","name":"Claude","provider":"claude"},{"id":"g","name":"Antigravity","provider":"agy"}],"default":"a"}');return}
  res.end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'private'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  await expect(page.locator('.engine-setup-toast')).toBeVisible()
  await expect(page.locator('.engine-setup')).toHaveCount(0)
  await page.getByRole('button',{name:'Ouvrir l’assistant',exact:true}).click()
  const assistant=page.locator('.engine-setup')
  await expect(assistant).toBeVisible()
  await expect(assistant.getByRole('combobox',{name:'Projet source des compétences'})).toHaveValue('')
  await assistant.getByRole('button',{name:'Installer les compétences',exact:true}).click()
  await expect(assistant.getByRole('status').first()).toContainText('Choisissez explicitement')
  assert.equal(writes.length,0)
  await assistant.getByRole('combobox',{name:'Projet source des compétences'}).selectOption('p')
  await assistant.getByRole('button',{name:'Installer les compétences',exact:true}).click()
  await expect(assistant.getByRole('status').first()).toContainText('a échoué')
  await assistant.getByRole('button',{name:'Installer les compétences',exact:true}).click()
  await expect(assistant).toContainText('Compétences Sectile : Configuration vérifiée')
  await assistant.getByRole('button',{name:'Connexion MCP',exact:true}).click()
  await assistant.getByRole('combobox',{name:'Connexion MCP',exact:true}).selectOption('local')
  await assistant.getByRole('button',{name:'Configurer MCP',exact:true}).click()
  await expect(assistant.getByRole('button',{name:'Configurer MCP',exact:true})).toBeDisabled()
  await expect.poll(()=>typeof releaseMCP).toBe('function')
  await assistant.getByRole('button',{name:'Fermer l’assistant',exact:true}).click()
  await page.getByRole('button',{name:'Configurer un moteur IA',exact:true}).click()
  await expect(assistant).toContainText('Connexion MCP : Configuration vérifiée')
  releaseMCP()
  await expect(assistant.getByRole('button',{name:'Configurer MCP',exact:true})).toBeEnabled()
  await assistant.getByRole('button',{name:'Autorisations',exact:true}).click()
  await expect(assistant.getByRole('combobox',{name:'Validation Codex',exact:true})).toHaveValue('user')
  await assistant.getByRole('button',{name:'Conserver les autorisations vérifiées',exact:true}).click()
  await expect(assistant).toContainText('Autorisations : Configuration vérifiée')
  assert.deepEqual(permissions,[])
  await assistant.getByRole('button',{name:'SDD du projet',exact:true}).click()
  await assistant.getByRole('button',{name:'Ignorer le SDD',exact:true}).click()
  await expect(assistant).toContainText('Configuration terminée. Redémarrez')
  await page.screenshot({path:'/tmp/issue842-assistant.png',fullPage:true})
  await assistant.getByRole('button',{name:'Fermer l’assistant',exact:true}).click()
  await page.getByRole('button',{name:'Configurer un moteur IA',exact:true}).click()
  await expect(assistant).toContainText('Configuration terminée. Redémarrez')
  await assistant.getByRole('combobox',{name:'Moteur IA',exact:true}).selectOption('b')
  await expect(assistant).toContainText('Compétences Sectile : Configuration vérifiée')
  assert.equal(writes.filter(write=>write.action==='provider-skills').length,2)
  assert.equal(writes.filter(write=>write.action==='framework').length,0)
  assert.deepEqual(writes.find(write=>write.target),{target:'local',transport:'http'})
  await expect(page.locator('.engine-setup-toast')).toHaveCount(0)
  await assistant.getByRole('combobox',{name:'Moteur IA',exact:true}).selectOption('c')
  await assistant.getByRole('button',{name:'Autorisations',exact:true}).click()
  await expect(assistant).toContainText('indisponible sous Windows')
  await expect(assistant.getByRole('combobox',{name:'Mode d’autorisation Claude'})).toHaveValue('default')
  await assistant.getByRole('combobox',{name:'Mode d’autorisation Claude'}).selectOption('plan')
  await assistant.getByRole('button',{name:'Enregistrer le mode Claude',exact:true}).click()
  await expect(assistant.getByRole('combobox',{name:'Mode d’autorisation Claude'})).toHaveValue('plan')
  await expect(assistant).toContainText('Autorisations : À configurer')
  await assistant.getByRole('button',{name:'Conserver les autorisations vérifiées',exact:true}).click()
  await expect(assistant).toContainText('Autorisations : Configuration vérifiée')
  await assistant.getByRole('combobox',{name:'Moteur IA',exact:true}).selectOption('g')
  await expect(assistant.getByRole('combobox',{name:'Mode d’autorisation Antigravity'})).toHaveValue('plan')
  await assistant.getByRole('combobox',{name:'Mode d’autorisation Antigravity'}).selectOption('default')
  await assistant.getByRole('button',{name:'Enregistrer le mode Antigravity',exact:true}).click()
  await expect(assistant.getByRole('combobox',{name:'Mode d’autorisation Antigravity'})).toHaveValue('default')
  assert.deepEqual(permissions,[{mode:'plan'},{conversationMode:'default'}])
 }finally{await app?.close();await new Promise(resolve=>server.close(resolve));fs.rmSync(root,{recursive:true,force:true})}
})
