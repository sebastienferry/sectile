const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The execution history drop-down of the toolbar (#525) is a ghost control:
// unboxed at rest, the hover background on hover, the accent ring on keyboard
// focus, in both themes, while every other select keeps its boxed look.
const TRANSPARENT='rgba(0, 0, 0, 0)'

function agent(){
 const run=(id,taskId,skill,hour)=>({id,taskId,taskKey:'#'+taskId,projectId:'project',skill,status:'completed',sessionId:id,directory:'/tmp/example/worktree',createdAt:`2026-09-13T${hour}:00:00Z`})
 const runs=[run('old','1','clarify','09'),run('new','1','implement','10'),run('solo','2','specify','11')]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Example project',path:'/tmp/example'}]));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end('[]');return}
  res.writeHead(404).end()
 })
 return new Promise(resolve=>server.listen(0,'127.0.0.1',()=>resolve(server)))
}

// Resolves a theme token to the computed colour a property would get from it.
const token=(page,name)=>page.evaluate(name=>{const probe=document.createElement('div');probe.style.color=`var(${name})`;document.body.append(probe);const value=getComputedStyle(probe).color;probe.remove();return value},name)
const style=(locator,...properties)=>locator.evaluate((el,properties)=>{const computed=getComputedStyle(el);return Object.fromEntries(properties.map(property=>[property,computed[property]]))},properties)

for(const colorScheme of ['dark','light']){
 test(`execution history is a ghost control in the ${colorScheme} theme`,async()=>{
  const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-execution-history-'))
  const server=await agent()
  fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
  const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
  let app
  try{
   app=await electron.launch({args:[path.resolve(__dirname,'..')],env,colorScheme})
   const page=await app.firstWindow();page.setDefaultTimeout(7000)
   const task=key=>page.locator('.local-task').filter({has:page.getByRole('button',{name:`Open ${key} in Sectile`,exact:true})}).locator('.run')
   const history=page.locator('#execution-history')
   await task('#1').click()
   await expect(history).toBeVisible()
   assert.deepEqual(await history.locator('option').allTextContents(),['1 · clarify · completed','2 · implement · completed'])

   const rest=await style(history,'borderTopStyle','borderTopWidth','backgroundColor','backgroundImage','appearance','color','maxWidth','textOverflow')
   assert.equal(rest.borderTopStyle,'none')
   assert.equal(rest.borderTopWidth,'0px')
   assert.equal(rest.backgroundColor,TRANSPARENT)
   assert.equal(rest.appearance,'none')
   assert.equal(rest.maxWidth,'170px')
   assert.equal(rest.textOverflow,'ellipsis')
   // The chevron is drawn in the muted text colour of the active theme.
   const muted=await token(page,'--text-muted')
   assert.equal((rest.backgroundImage.match(/linear-gradient/g)||[]).length,2)
   assert.ok(rest.backgroundImage.includes(muted),rest.backgroundImage)
   assert.equal(rest.color,await token(page,'--text'))
   assert.equal((await style(history.locator('option').first(),'backgroundColor')).backgroundColor,await token(page,'--surface'))

   await history.hover()
   await expect(history).toHaveCSS('background-color',await token(page,'--hover-bg'))
   assert.ok((await style(history,'backgroundImage')).backgroundImage.includes(muted))
   await page.locator('#terminal').hover()
   await expect(history).toHaveCSS('background-color',TRANSPARENT)

   // Keyboard focus from the preceding toolbar control draws the accent ring.
   await page.locator('#worktree').focus()
   await page.keyboard.press('Tab')
   await expect(history).toBeFocused()
   await expect(history).toHaveCSS('outline-style','solid')
   await expect(history).toHaveCSS('outline-color',await token(page,'--accent'))
   await expect(history).toHaveCSS('outline-offset','2px')

   // Every other select keeps the boxed look of the global rule.
   const other=await page.evaluate(()=>{const select=document.createElement('select');document.body.append(select);const computed=getComputedStyle(select);const value={style:computed.borderTopStyle,width:computed.borderTopWidth};select.remove();return value})
   assert.deepEqual(other,{style:'solid',width:'1px'})

   await history.selectOption('old')
   await expect(page.locator('#title')).toContainText('clarify')
   await page.screenshot({path:path.join(root,`execution-history-${colorScheme}.png`)})
   console.log('Execution history screenshot: '+path.join(root,`execution-history-${colorScheme}.png`))
   await task('#2').click()
   await expect(history).toBeHidden()
  }finally{
   await app?.close()
   server.close()
  }
 })
}
