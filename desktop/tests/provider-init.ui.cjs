const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('desktop initializes only the selected provider and retains partial results and retry',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-init-ui-')),writes=[]
 let release
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/project'){
   if(req.method==='POST'){
    writes.push(Object.fromEntries(url.searchParams))
    release=()=>{res.end(JSON.stringify({success:writes.length>1,message:writes.length>1?'Initialization complete':'Initialization failed',mcp:{status:'success',message:'Registered'},skills:{status:writes.length>1?'success':'failed',message:writes.length>1?'Installed':'Permission denied'}}));release=null}
    return
   }
   res.end(JSON.stringify({configured:true,path:'/test/repo',aiProvider:'agy',server:{projectId:'p',projectName:'Test project',aiProvider:'agy',skills:[]}}));return
  }
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'p',name:'Test project',path:'/test/repo'}]));return}
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://example.test'}));return}
  if(url.pathname==='/desktop/runs'){res.end('[]');return}
  if(url.pathname==='/desktop/settings'){res.end('{"aiProvider":"agy"}');return}
  res.end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'private'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  await page.getByRole('button',{name:'Configure Test project',exact:true}).click()
  await page.getByRole('tab',{name:'Deployment',exact:true}).click()
  const provider=page.getByRole('combobox',{name:'Initialization provider',exact:true}),button=page.getByRole('button',{name:'Initialize',exact:true}),result=page.locator('.initialization-result')
  await provider.selectOption('codex')
  assert.equal(writes.length,0)
  await button.click()
  await expect.poll(()=>writes.length).toBe(1)
  await expect(button).toBeDisabled();await expect(provider).toBeDisabled()
  assert.deepEqual(writes[0],{id:'p',action:'initialize',provider:'codex'})
  release()
  await expect(result).toContainText('MCP: Success')
  await expect(result).toContainText('Skills: Failed — Permission denied')
  await expect(button).toBeEnabled();await expect(provider).toBeEnabled()
  await button.click();await expect.poll(()=>writes.length).toBe(2);release()
  await expect(result).toContainText('Skills: Success')
  await expect(button).toBeEnabled()
 }finally{
  if(release)release()
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
