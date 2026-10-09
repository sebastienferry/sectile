const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('a task that changed two repositories lists its pull requests in a menu, the primary one first',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-pr-repositories-'))
 const app_='https://github.com/example/app/pull/79',deploy='https://gitlab.com/example/deploy/-/merge_requests/7'
 const single='https://github.com/example/app/pull/80'
 const runs=[
  {id:'run-1',taskId:'1',taskKey:'#1',projectId:'project-a',skill:'implement',status:'completed'},
  {id:'run-2',taskId:'2',taskKey:'#2',projectId:'project-a',skill:'implement',status:'completed'},
 ]
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
   {id:'2',key:'#2',title:'One repository',prUrl:single,labels:['#implemented'],prLinks:[{url:single,repository:'github.com/example/app',state:'open'}]},
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
  await page.locator('.local-task .run[data-run-id=run-1]').click()
  await expect(page.locator('#selected-pr')).toHaveText('PR #79')
  await expect(page.locator('#selected-pr-others')).toHaveCount(0)
  const more=page.locator('#selected-pr-more'),menu=page.locator('#selected-pr-menu')
  await expect(more).toBeVisible()
  await expect(more).toHaveText('+1')
  await expect(more).toHaveAttribute('aria-haspopup','menu')
  await expect(more).toHaveAttribute('aria-expanded','false')
  await expect(more).toHaveAttribute('aria-label','Pull requests of this task')
  await more.click()
  await expect(more).toHaveAttribute('aria-expanded','true')
  const items=menu.getByRole('menuitem')
  await expect(items).toHaveText(['app PR #79','deploy MR !7'])
  await expect(items.nth(0)).toBeFocused()
  await expect(items.nth(1)).toHaveAttribute('aria-label','Open MR !7 in gitlab.com/example/deploy — State unknown: no GitLab token')
  await expect(items.nth(1)).toHaveAttribute('title',/State unknown: no GitLab token — https:\/\/gitlab\.com/)
  // The arrow keys wrap at both ends.
  await page.keyboard.press('ArrowUp')
  await expect(items.nth(1)).toBeFocused()
  await page.keyboard.press('ArrowDown')
  await expect(items.nth(0)).toBeFocused()
  await items.nth(1).click()
  await expect(menu).toBeHidden()
  await expect(more).toBeFocused()
  await expect.poll(()=>app.evaluate(()=>globalThis.opened)).toEqual([deploy])
  // Escape closes the menu and gives the focus back to the chevron.
  await page.keyboard.press('ArrowDown')
  await expect(items.nth(0)).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(menu).toBeHidden()
  await expect(more).toBeFocused()
  await expect(more).toHaveAttribute('aria-expanded','false')
  // A press outside closes it too.
  await more.click()
  await expect(menu).toBeVisible()
  await page.locator('#title').click()
  await expect(menu).toBeHidden()
  // Selecting another execution closes an open menu; one pull request keeps
  // its button alone. The click is dispatched without a pointer press, which
  // would close the menu on its own.
  await more.click()
  await expect(menu).toBeVisible()
  await page.locator('.local-task .run[data-run-id=run-2]').dispatchEvent('click')
  await expect(page.locator('#selected-pr')).toHaveText('PR #80')
  await expect(menu).toBeHidden()
  await expect(more).toBeHidden()
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
