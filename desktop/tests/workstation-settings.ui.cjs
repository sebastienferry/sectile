const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The workstation level of every execution setting is edited from Settings,
// through the local agent, which is its only writer (#305).
test('execution defaults are read from and saved through the agent, which may refuse them',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-workstation-ui-'))
 const puts=[]
 let refuse=''
 const view=()=>({
  defaults:{aiProvider:'claude',aiModel:'claude-opus-5',editorCommand:'zed',setupProviders:null},
  effective:{aiProvider:'claude',aiModel:'claude-opus-5',editorCommand:'zed',useWorktrees:true,parallelism:1,aiProviderModels:{}},
  providerModels:{claude:['claude-opus-5','claude-sonnet-5'],codex:['gpt-5']},setupProviders:['claude','codex','agy'],seeded:{},
 })
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/workstation'&&req.method==='PUT'){
   let data='';req.on('data',chunk=>data+=chunk);req.on('end',()=>{
    if(refuse){res.writeHead(400,{'Content-Type':'text/plain'}).end(refuse);return}
    puts.push(JSON.parse(data));res.writeHead(204).end()
   })
   return
  }
  if(req.url==='/desktop/workstation'){res.end(JSON.stringify(view()));return}
  if(req.url.startsWith('/desktop/mcp?')){res.end(JSON.stringify({path:'/test',server:'https://sectile.example.test',localURL:'http://127.0.0.1:4567',choice:{target:'remote',transport:'http'}}));return}
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://sectile.example.test'}));return}
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
  await page.getByRole('tab',{name:'Execution defaults',exact:true}).click()
  const panel=page.locator('.execution-defaults')
  await expect(panel.getByRole('combobox',{name:'AI Provider',exact:true})).toHaveValue('claude')
  await expect(panel.getByRole('textbox',{name:'AI Model',exact:true})).toHaveValue('claude-opus-5')
  await expect(panel.getByRole('textbox',{name:'Editor command',exact:true})).toHaveValue('zed')

  await panel.getByRole('textbox',{name:'AI Model',exact:true}).fill('claude-sonnet-5')
  await panel.getByRole('textbox',{name:'Models offered for codex',exact:true}).fill('gpt-5, o4-mini')
  await panel.getByRole('button',{name:'Save execution defaults'}).click()
  await expect(panel.getByRole('status')).toContainText('Execution defaults saved')
  assert.equal(puts.length,1)
  assert.equal(puts[0].aiProvider,'claude')
  assert.equal(puts[0].aiModel,'claude-sonnet-5')
  assert.deepEqual(puts[0].aiProviderModels,{codex:['gpt-5','o4-mini']})
  assert.equal(puts[0].setupProviders,null)

  // A value the agent refuses is reported with its reason.
  refuse='parallelism must be between 1 and 10'
  await panel.getByRole('button',{name:'Save execution defaults'}).click()
  await expect(panel.getByRole('status')).toContainText('Not saved: parallelism must be between 1 and 10')
  assert.equal(puts.length,1)
 } finally {
  await app?.close()
  server.close()
  fs.rmSync(root,{recursive:true,force:true})
 }
})
