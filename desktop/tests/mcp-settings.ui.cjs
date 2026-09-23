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
  await page.getByRole('tab',{name:'Agents CLI',exact:true}).click()
  const section=page.locator('.mcp-settings')
  await expect(section.getByRole('radio',{name:'Streamable HTTP',exact:true})).toBeChecked()
  await expect(section.locator('pre').first()).toContainText('serverUrl')
  await page.getByRole('combobox',{name:'AI Provider',exact:true}).selectOption('codex')
  await expect(section.locator('pre').first()).toContainText('http_headers')
  await section.getByRole('combobox',{name:'MCP connection target'}).selectOption('local')
  await expect(section.locator('pre').first()).toContainText('127.0.0.1:4567/mcp')
  await expect(section.locator('pre').first()).not.toContainText('Authorization')
  await section.getByRole('radio',{name:'STDIO',exact:true}).check()
  if(process.env.SECTILE_MCP_SCREENSHOT) await page.screenshot({path:process.env.SECTILE_MCP_SCREENSHOT})
  assert.equal(writes.length,0)
  await section.getByRole('button',{name:'Update provider configuration'}).click()
  await expect(section.getByRole('status')).toContainText('Updated /test/codex')
  assert.deepEqual(writes,[{provider:'codex',target:'local',transport:'stdio'}])
  await page.getByRole('combobox',{name:'AI Provider',exact:true}).selectOption('custom')
  await expect(section.getByRole('button',{name:'Update provider configuration'})).toBeDisabled()
 } finally {
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
