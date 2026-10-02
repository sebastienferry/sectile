const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('the command palette opens the Add project dialog of the sidebar',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-palette-add-project-'))
 let projectReads=0
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status')return res.end(JSON.stringify({connected:true,server:'https://example.test'}))
  if(url.pathname==='/desktop/runs')return res.end('[]')
  // A project with a local path is added to this workstation; the other is not yet.
  if(url.pathname==='/desktop/projects'){projectReads++;return res.end(JSON.stringify([{id:'project-a',name:'Project A',path:'/tmp/A'},{id:'project-b',name:'Project B'}]))}
  res.writeHead(404).end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  const filter=page.getByRole('combobox',{name:'Search commands'})
  const command=page.locator('.palette-option',{hasText:'Add project'})
  const heading=page.getByRole('heading',{name:'Add project',exact:true})
  const added=page.getByRole('button',{name:'Project A · Already added',exact:true})
  const available=page.getByRole('button',{name:'Project B',exact:true})
  const close=()=>page.getByRole('button',{name:'Close',exact:true}).click()
  await page.getByRole('button',{name:'▾ Project A',exact:true}).waitFor()

  // The action is listed after Tasks list and filtered like the others.
  await page.keyboard.press('Control+k')
  const actions=await page.locator('.palette-option .palette-label').allTextContents()
  assert.deepEqual(actions.slice(0,3),['New task','Tasks list','Add project'])
  await filter.fill('tasks')
  await expect(command).toBeHidden()
  await filter.fill('ADD project')
  await expect(command).toBeVisible()
  await expect(page.locator('.palette-option',{hasText:'New task'})).toBeHidden()

  // Enter runs it: the dialog replaces the palette and lists the server's projects.
  const reads=projectReads
  await filter.press('Enter')
  await expect(heading).toBeVisible()
  await expect(filter).toHaveCount(0)
  await page.getByText('Discover projects from your Sectile server and configure their local directory.',{exact:true}).waitFor()
  await expect(added).toBeDisabled()
  await expect(available).toBeEnabled()
  assert.ok(projectReads>reads,'the dialog reads the server projects again')
  await close()

  // With the sidebar collapsed the command still opens the dialog, and the sidebar stays collapsed.
  await page.locator('#toggle-sidebar').click()
  await expect(page.locator('#workspace')).toHaveClass(/sidebar-hidden/)
  await page.keyboard.press('Control+k')
  await command.click()
  await expect(heading).toBeVisible()
  await expect(added).toBeDisabled()
  await expect(page.locator('#workspace')).toHaveClass(/sidebar-hidden/)
  await close()

  // The sidebar's + button keeps opening the same dialog.
  await page.locator('#toggle-sidebar').click()
  await page.locator('#add-project').click()
  await expect(heading).toBeVisible()
  await expect(added).toBeDisabled()
  await expect(available).toBeEnabled()
 }finally{
  await app?.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
