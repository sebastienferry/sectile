const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('desktop installs skills and registers MCP each on its own, with partial results and retry',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-init-ui-')),writes=[],mcpWrites=[]
 let release
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/project'){
   if(req.method==='POST'){
    writes.push(Object.fromEntries(url.searchParams))
    release=()=>{res.end(JSON.stringify({success:writes.length>1,mcp:{status:'not_run',message:'Not part of this step'},skills:{status:writes.length>1?'success':'failed',message:writes.length>1?'3 installed in ~/.codex/skills':'Permission denied'}}));release=null}
    return
   }
   res.end(JSON.stringify({configured:true,path:'/test/repo',aiProvider:'agy',server:{projectId:'p',projectName:'Test project',aiProvider:'agy',skills:[]}}));return
  }
  if(url.pathname==='/desktop/mcp'){
   if(req.method==='POST'){let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{mcpWrites.push({provider:url.searchParams.get('provider'),...JSON.parse(body)});res.end(JSON.stringify({choice:JSON.parse(body),path:'/home/me/.codex/config.toml'}))});return}
   res.end(JSON.stringify({choice:{target:'local',transport:'http'},path:'/home/me/.codex/config.toml'}));return
  }
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'p',name:'Test project',path:'/test/repo'}]));return}
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://example.test'}));return}
  if(url.pathname==='/desktop/runs'){res.end('[]');return}
  if(url.pathname==='/desktop/settings'){res.end('{"aiProvider":"agy"}');return}
  if(url.pathname==='/desktop/workstation'){res.end('{"globalConfiguration":true,"defaults":{"initializationProvider":"codex"}}');return}
  if(url.pathname==='/desktop/engines'){res.end('{"catalogue":[{"id":"codex","name":"Codex","provider":"codex"}],"default":"codex"}');return}
  res.end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'private'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  await page.getByRole('button',{name:'Actions for Test project',exact:true}).click();await page.getByRole('menuitem',{name:'Project settings…',exact:true}).click()
  await page.getByRole('tab',{name:'Deployment',exact:true}).click()
  await expect(page.getByRole('combobox',{name:'Deployment project',exact:true})).toHaveValue('p')
  await expect(page.getByRole('tab',{name:'Server',exact:true})).toHaveCount(0)
  await expect(page.getByRole('tab',{name:'AI agent',exact:true})).toHaveCount(0)
  const skills=page.getByRole('button',{name:'Install skills',exact:true}),results=page.locator('.setup-result')
  await expect(page.getByRole('combobox',{name:'Setup AI engine',exact:true})).toHaveValue('codex')
  await expect(page.getByRole('button',{name:'Install SDD in project',exact:true})).toBeVisible()
  await expect(page.getByRole('button',{name:'Set up engine globally',exact:true})).toHaveCount(0)
  assert.equal(writes.length,0)
  // Skills install on their own, and say how it went; a failure can be retried.
  await skills.click()
  await expect.poll(()=>writes.length).toBe(1)
  await expect(skills).toBeDisabled()
  assert.deepEqual(writes[0],{id:'p',action:'provider-skills',provider:'codex'})
  release()
  await expect(results.first()).toHaveText('Failed · Permission denied')
  await expect(skills).toBeEnabled()
  await skills.click();await expect.poll(()=>writes.length).toBe(2);release()
  await expect(results.first()).toHaveText('Installed · 3 installed in ~/.codex/skills')
  assert.equal(mcpWrites.length,0,'installing skills registered no MCP')
  // MCP registers on its own, over HTTP, to the remote server or the local agent;
  // the panel opens on the target the provider is registered with.
  const target=page.getByRole('group',{name:'MCP target',exact:true})
  await expect(target.getByRole('button',{name:'Local agent',exact:true})).toHaveAttribute('aria-pressed','true')
  await target.getByRole('button',{name:'Remote server',exact:true}).click()
  await page.getByRole('button',{name:'Register MCP',exact:true}).click()
  await expect.poll(()=>mcpWrites).toEqual([{provider:'codex',target:'remote',transport:'http'}])
  await expect(results.nth(1)).toHaveText('Registered in /home/me/.codex/config.toml (remote server, HTTP). Restart the AI engine to reconnect.')
  assert.equal(writes.length,2,'registering MCP installed no skills')
 }finally{
  if(release)release()
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
