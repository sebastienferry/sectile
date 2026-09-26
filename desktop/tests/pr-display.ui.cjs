const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('PR icons stay inline and open independently with mouse and keyboard',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-pr-display-'))
 const url='https://github.com/example/repo/pull/79'
 let link=url, state='conflicting'
 const runs=['1','2'].map(id=>({id:'run-'+id,taskId:id,taskKey:'#'+id,projectId:'project-a',skill:'clarify',status:'completed'}))
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Example project',path:'/tmp/project'}]));return}
  if(req.url==='/desktop/project?id=project-a'){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end(JSON.stringify([
   {id:'1',key:'#1',title:'A very long task title that must truncate before the pull request icon',prUrl:link,prLinks:link?[{url:link,state}]:[],labels:['#reviewed']},
   {id:'2',key:'#2',title:'Another task without a pull request',labels:['#reviewed']}
  ]));return}
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  await app.evaluate(({shell})=>{globalThis.opened=[];shell.openExternal=async url=>{globalThis.opened.push(url)}})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  const row=id=>page.locator('.local-task').filter({has:page.getByRole('button',{name:'Open #'+id+' in Sectile',exact:true})})
  const pr=page.getByRole('button',{name:/^Open PR #79 for #1 — /})
  await pr.waitFor()
  assert.equal(await row('1').locator('.pr-indicator').count(),1)
  assert.equal(await row('2').locator('.pr-indicator').count(),0)
  assert.equal(await page.locator('.project-group > .pr-indicator').count(),0)
  assert.equal(await pr.textContent(),'')
  assert.match(await pr.getAttribute('title'),/Conflicting/)
  assert.match(await pr.getAttribute('aria-label'),/Conflicting/)
  await page.getByRole('separator',{name:'Resize sidebar'}).focus()
  for(let i=0;i<4;i++)await page.keyboard.press('ArrowLeft')
  assert.equal((await page.locator('aside').boundingBox()).width,210)
  await page.mouse.move(600,400)
  assert.equal(await pr.evaluate(el=>getComputedStyle(el).opacity),'1')
  const bounds=await row('1').boundingBox(),plain=await row('2').boundingBox()
  assert.equal(bounds.height,plain.height,'The PR must not add row height')
  for(const selector of ['.run','.pr-indicator','.task-archive','.task-rename-button']){
   const control=await row('1').locator(selector).boundingBox()
   assert.ok(control.x>=bounds.x&&control.x+control.width<=bounds.x+bounds.width+1,selector+' stays within the row')
   assert.ok(control.y>=bounds.y&&control.y+control.height<=bounds.y+bounds.height+1,selector+' stays on the same line')
  }
  assert.ok(await row('1').locator('strong').evaluate(el=>el.scrollWidth>el.clientWidth),'Long title truncates')
  await row('2').locator('.run').click()
  const selectedTitle=await page.locator('#title').textContent()
  await pr.click()
  await expect.poll(()=>app.evaluate(()=>globalThis.opened)).toEqual([url])
  assert.equal(await page.locator('#title').textContent(),selectedTitle)
  await row('1').locator('.run').focus();await page.keyboard.press('Tab')
  assert.equal(await pr.evaluate(el=>el===document.activeElement&&el.matches(':focus-visible')),true)
  assert.equal(await pr.evaluate(el=>getComputedStyle(el).outlineStyle),'solid')
  await page.keyboard.press('Enter')
  await expect.poll(()=>app.evaluate(()=>globalThis.opened)).toEqual([url,url])
  assert.equal(await page.locator('#title').textContent(),selectedTitle)
  await page.screenshot({path:path.join(root,'pr-inline.png')})
  console.log('PR screenshot: '+path.join(root,'pr-inline.png'))
  await row('1').locator('.run').click()
  assert.equal(await page.locator('#selected-pr').textContent(),'PR #79')
  state='merged'
  await expect(pr).toHaveAttribute('aria-label','Open PR #79 for #1 — Merged',{timeout:20000})
  await expect(page.locator('#selected-pr')).toHaveAttribute('aria-label','Open PR #79 — Merged')
  link=undefined
  // PR metadata refreshes every 15 seconds, independently of the run polling.
  await expect(pr).toHaveCount(0,{timeout:20000})
  assert.equal((await row('1').boundingBox()).height,bounds.height)
  await expect(page.locator('#selected-pr')).toBeHidden()
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
