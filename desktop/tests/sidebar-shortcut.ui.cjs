const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// #474: Cmd+B on macOS, Ctrl+B elsewhere, toggles the projects sidebar as its
// button does. Cmd+B never reaches the terminal; Ctrl+B stays with a focused
// terminal outside macOS; nothing moves behind a dialog.
test('Cmd+B / Ctrl+B toggles the projects sidebar and leaves the terminal its keys',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-sidebar-shortcut-'))
 const runs=[{id:'run-42',projectId:'project',taskId:'task-42',taskKey:'#42',skill:'implement',directory:'/tmp/repo',status:'running',createdAt:'2026-09-25T06:00:05Z'}]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://example.test/sectile/',capabilities:[]}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Project',path:'/tmp/repo'}]));return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(url.pathname==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(url.pathname==='/desktop/tasks'){res.end('[]');return}
  if(url.pathname==='/desktop/run-result'){res.end(JSON.stringify({activity:null}));return}
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}),{mode:0o600})
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  // What the terminal sends to the run, seen from the main process.
  await app.evaluate(({ipcMain})=>{globalThis.inputs=[];ipcMain.on('terminal-input',(_,data)=>globalThis.inputs.push(data))})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  const workspace=page.locator('#workspace'),toggle=page.locator('#toggle-sidebar')
  await expect(page.locator('#runs .local-task')).toHaveCount(1)
  const hidden=()=>workspace.evaluate(el=>el.classList.contains('sidebar-hidden'))
  const stored=()=>page.evaluate(()=>localStorage.getItem('sidebarCollapsed'))
  const inputs=()=>app.evaluate(()=>globalThis.inputs)
  const platform=value=>page.evaluate(value=>{
   Object.defineProperty(Navigator.prototype,'platform',{get:()=>value,configurable:true})
   Object.defineProperty(Navigator.prototype,'userAgentData',{get:()=>({platform:value}),configurable:true})
  },value)
  const focusTerminal=async()=>{
   await page.locator('#terminal .xterm').click()
   await page.waitForFunction(()=>!!document.activeElement?.closest('.xterm'))
  }

  // ---------- macOS: Cmd+B toggles like the button ----------
  await platform('MacIntel')
  assert.equal(await hidden(),false)
  await page.locator('#title').click()
  await page.keyboard.press('Meta+b')
  await expect.poll(hidden).toBe(true)
  assert.equal(await stored(),'true','the state is remembered as the button does')
  await expect(toggle).toHaveAttribute('aria-expanded','false')
  await expect(toggle).toHaveAttribute('title','Show projects (⌘B)')
  await expect(toggle).toHaveAttribute('aria-keyshortcuts','Meta+B')

  // Cmd+B from the terminal toggles and sends it nothing.
  await focusTerminal()
  await page.keyboard.press('Meta+b')
  await expect.poll(hidden).toBe(false)
  assert.equal(await stored(),'false')
  await expect(toggle).toHaveAttribute('title','Hide projects (⌘B)')
  // Ctrl+B on macOS is the terminal's.
  await page.keyboard.press('Control+b')
  await expect.poll(inputs).toEqual(['\x02'])
  assert.equal(await hidden(),false,'Ctrl+B does not toggle on macOS')

  // ---------- Behind a dialog, nothing moves ----------
  await page.locator('#title').click()
  await page.keyboard.press('Meta+k')
  await expect(page.locator('#project-dialog')).toHaveJSProperty('open',true)
  await page.keyboard.press('Meta+b')
  assert.equal(await hidden(),false,'the chord does nothing behind a dialog')
  await page.keyboard.press('Escape')
  await expect(page.locator('#project-dialog')).toHaveJSProperty('open',false)

  // ---------- Linux: Ctrl+B outside the terminal, the terminal keeps it ----------
  await platform('Linux x86_64')
  await page.locator('#title').click()
  await page.keyboard.press('Control+b')
  await expect.poll(hidden).toBe(true)
  await expect(toggle).toHaveAttribute('title','Show projects (Ctrl+B)')
  await expect(toggle).toHaveAttribute('aria-keyshortcuts','Control+B')
  await page.keyboard.press('Control+b')
  await expect.poll(hidden).toBe(false)
  await focusTerminal()
  await page.keyboard.press('Control+b')
  await expect.poll(inputs).toEqual(['\x02','\x02'])
  assert.equal(await hidden(),false,'a focused terminal keeps Ctrl+B')
 }finally{
  await app?.close();server.close();fs.rmSync(root,{recursive:true,force:true})
 }
})
