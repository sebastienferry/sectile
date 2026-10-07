const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('MCP settings explain both transports and apply the selected provider and target',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-mcp-ui-'))
 const writes=[]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url.startsWith('/desktop/mcp?')) {
   const provider=new URL(req.url,'http://localhost').searchParams.get('provider')
   const reply=()=>res.end(JSON.stringify({path:'/test/'+provider,server:'https://sectile.example.test',localURL:'http://127.0.0.1:4567',choice:{target:'remote',transport:'http'}}))
   if(req.method==='POST') {let data='';req.on('data',chunk=>data+=chunk);req.on('end',()=>{writes.push({provider,...JSON.parse(data)});reply()})} else reply()
   return
  }
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://sectile.example.test'}));return}
  // Workstation defaults supply the initial provider for MCP deployment.
  if(req.url==='/desktop/workstation'){res.end(JSON.stringify({defaults:{},effective:{aiProvider:'agy',useWorktrees:true,parallelism:1,aiProviderModels:{}},providerModels:{},setupProviders:['claude','codex','agy'],seeded:{}}));return}
  if(['/desktop/runs','/desktop/projects'].includes(req.url)){res.end('[]');return}
  if(req.url==='/desktop/version'){res.end('{"version":"test"}');return}
  res.writeHead(404).end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'private'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try {
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'Deployment',exact:true}).click()
  await expect(page.locator('#settings-panel-AgentCli .mcp-settings')).toHaveCount(0)
  const section=page.locator('#settings-panel-Deployment .mcp-settings')
  const snippet=section.locator('.mcp-snippet pre'),command=section.locator('.mcp-command')
  await expect(section.getByRole('button',{name:'Remote HTTP (default)',exact:true})).toHaveAttribute('aria-pressed','true')
  await expect(snippet.first()).toContainText('serverUrl')
  // Antigravity has no CLI, so no command card (#667).
  await expect(command).toHaveCount(0)
  await page.getByRole('combobox',{name:'MCP provider',exact:true}).selectOption('codex')
  await expect(snippet.first()).toContainText('http_headers')
  await expect(command.locator('pre')).toHaveText('codex mcp add sectile --url https://sectile.example.test/mcp --bearer-token-env-var SECTILE_API_KEY')
  await expect(command).toContainText('Export SECTILE_API_KEY')
  await section.getByRole('button',{name:'Local HTTP proxy',exact:true}).click()
  await expect(snippet.first()).toContainText('127.0.0.1:4567')
  await expect(snippet.first()).not.toContainText('Authorization')
  await expect(snippet).toHaveCount(1)
  await expect(command.locator('pre')).toHaveText('codex mcp add sectile --url http://127.0.0.1:4567/mcp')
  await expect(snippet).toContainText('url =')
  await expect(snippet).not.toContainText('command =')
  if(process.env.SECTILE_MCP_SCREENSHOT) await page.screenshot({path:process.env.SECTILE_MCP_SCREENSHOT})
  assert.equal(writes.length,0)
  await section.getByRole('button',{name:'Update provider configuration'}).click()
  await expect(section.getByRole('status')).toContainText('Updated /test/codex')
  assert.deepEqual(writes,[{provider:'codex',target:'local',transport:'http'}])
  await section.getByRole('button',{name:'STDIO',exact:true}).click()
  await expect(snippet).toContainText('command =')
  await expect(command.locator('pre')).toHaveText("codex mcp add sectile --env 'SECTILE_AGENT_TOKEN=<SECTILE_API_KEY>' -- sectile-agent mcp --url https://sectile.example.test")
  await expect(command).toContainText('Codex replaces an existing sectile entry.')
  await section.getByRole('button',{name:'Update provider configuration'}).click()
  await expect(section.getByRole('status')).toContainText('Updated /test/codex')
  assert.deepEqual(writes[1],{provider:'codex',target:'remote',transport:'stdio'})
  await section.getByRole('button',{name:'Remote HTTP (default)',exact:true}).click()
  await expect(snippet).toContainText('https://sectile.example.test/mcp')
  await section.getByRole('button',{name:'Update provider configuration'}).click()
  await expect(section.getByRole('status')).toContainText('Updated /test/codex')
  assert.deepEqual(writes[2],{provider:'codex',target:'remote',transport:'http'})
  // Claude gets the two-line block; copying it writes nothing on disk.
  await page.getByRole('combobox',{name:'MCP provider',exact:true}).selectOption('claude')
  await expect(command.locator('pre')).toHaveText(`claude mcp remove --scope user sectile
claude mcp add --transport http --scope user sectile https://sectile.example.test/mcp --header 'Authorization: Bearer <SECTILE_API_KEY>'`)
  await expect(command).toContainText('Run both lines.')
  await command.getByRole('button',{name:'Copy command',exact:true}).click()
  await expect(section.getByRole('status')).toHaveText('Command copied.')
  assert.equal(await app.evaluate(({clipboard})=>clipboard.readText()),await command.locator('pre').textContent())
  await section.locator('.mcp-snippet').getByRole('button').click()
  await expect(section.getByRole('status')).toContainText('Example copied.')
  assert.equal(JSON.parse(await app.evaluate(({clipboard})=>clipboard.readText())).mcpServers.sectile.url,'https://sectile.example.test/mcp')
  assert.equal(writes.length,3)
  assert.deepEqual(await page.getByRole('combobox',{name:'MCP provider',exact:true}).locator('option').evaluateAll(options=>options.map(option=>option.value)),['agy','claude','codex'])
 } finally {
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})

// A Claude Code entry Sectile did not write, holding a key this workstation does
// not use, is only reported; Repair rewrites it on request (#716).
test('MCP settings offer to repair an outdated Claude Code entry',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-mcp-repair-ui-'))
 const writes=[]
 let repaired=false
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url.startsWith('/desktop/mcp?')) {
   const provider=new URL(req.url,'http://localhost').searchParams.get('provider')
   const reply=()=>res.end(JSON.stringify({path:'/test/'+provider,server:'https://sectile.example.test',localURL:'http://127.0.0.1:4567',choice:{target:'remote',transport:'http'},
    ...(provider==='claude'?{entries:repaired?[{scope:'user',managed:true,keyMatches:true,urlMatches:true,stale:false}]:[{scope:'user',managed:false,keyMatches:true,urlMatches:true,stale:false},{scope:'project',project:'/work/app',managed:false,keyMatches:false,urlMatches:true,stale:true}],needsRepair:!repaired}:{})}))
   if(req.method==='POST') {let data='';req.on('data',chunk=>data+=chunk);req.on('end',()=>{const body=JSON.parse(data);writes.push({provider,...body});if(body.repair)repaired=true;reply()})} else reply()
   return
  }
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://sectile.example.test'}));return}
  if(req.url==='/desktop/workstation'){res.end(JSON.stringify({defaults:{},effective:{aiProvider:'agy',useWorktrees:true,parallelism:1,aiProviderModels:{}},providerModels:{},setupProviders:['claude','codex','agy'],seeded:{}}));return}
  if(['/desktop/runs','/desktop/projects'].includes(req.url)){res.end('[]');return}
  if(req.url==='/desktop/version'){res.end('{"version":"test"}');return}
  res.writeHead(404).end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'private'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try {
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  // The launch reports the outdated entry once, before anybody opens the settings.
  await expect(page.locator('#error')).toContainText('Open Settings → Deployment → MCP configuration to repair it.')
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'Deployment',exact:true}).click()
  const section=page.locator('.mcp-settings'),row=section.locator('.mcp-repair')
  await expect(section.getByRole('button',{name:'Remote HTTP (default)',exact:true})).toHaveAttribute('aria-pressed','true')
  // The row is for Claude Code only.
  await expect(row).toBeHidden()
  await page.getByRole('combobox',{name:'MCP provider',exact:true}).selectOption('claude')
  await expect(row).toBeVisible()
  await expect(row.locator('p')).toHaveText('Project /work/app register sectile with a key this workstation does not use, so Claude Code cannot connect. Repair writes this workstation\'s key and removes the outdated project entries.')
  assert.equal(writes.length,0,'nothing is written before Repair is clicked')
  await row.getByRole('button',{name:'Repair',exact:true}).click()
  await expect(section.getByRole('status')).toHaveText('Repaired. Restart Claude Code to reconnect.')
  await expect(row).toBeHidden()
  assert.deepEqual(writes,[{provider:'claude',target:'remote',transport:'http',repair:true}])
 } finally {
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
