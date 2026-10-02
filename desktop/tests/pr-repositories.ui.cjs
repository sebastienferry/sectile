const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('a task that changed two repositories shows both pull requests, the primary one first',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-pr-repositories-'))
 const app_='https://github.com/example/app/pull/79',deploy='https://gitlab.com/example/deploy/-/merge_requests/7'
 const runs=[{id:'run-1',taskId:'1',taskKey:'#1',projectId:'project-a',skill:'implement',status:'completed'}]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project-a',name:'Example project',path:'/tmp/project'}]));return}
  if(req.url==='/desktop/project?id=project-a'){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end(JSON.stringify([
   // The server keeps the primary repository's pull request last.
   {id:'1',key:'#1',title:'Two repositories',prUrl:app_,labels:['#implemented'],prLinks:[
    {url:deploy,repository:'gitlab.com/example/deploy',missingToken:'gitlab'},
    {url:app_,repository:'github.com/example/app',state:'open'},
   ]},
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
  const primary=page.getByRole('button',{name:/^Open PR #79 for #1 — Open$/})
  await primary.waitFor()
  const badge=page.locator('.local-task .pr-others')
  await expect(badge).toHaveText('+1')
  assert.match(await badge.getAttribute('title'),/gitlab\.com\/example\/deploy: MR !7 \(State unknown: no GitLab token\)/)
  await page.locator('.local-task .run').first().click()
  await expect(page.locator('#selected-pr')).toHaveText('PR #79')
  const other=page.locator('#selected-pr-others button')
  await expect(other).toHaveCount(1)
  await expect(other).toHaveText('deploy MR !7')
  await expect(other).toHaveAttribute('aria-label','Open MR !7 in gitlab.com/example/deploy — State unknown: no GitLab token')
  await other.click()
  await expect.poll(()=>app.evaluate(()=>globalThis.opened)).toEqual([deploy])
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
