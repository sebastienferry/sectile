const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The Appearance setting (#507): System, Dark or Light, stored on the
// workstation and applied to the whole window at once, the terminal and the
// window background included, without a restart.
const DARK_BG='rgb(17, 21, 28)',LIGHT_BG='rgb(245, 246, 248)'

function agent(){
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://example.test'}));return}
  if(['/desktop/runs','/desktop/projects'].includes(req.url)){res.end('[]');return}
  if(req.url==='/desktop/version'){res.end(JSON.stringify({version:'v9.9.9'}));return}
  res.writeHead(404).end('{}')
 })
 return new Promise(resolve=>server.listen(0,'127.0.0.1',()=>resolve(server)))
}

async function launch(root){
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 // Playwright emulates a light prefers-color-scheme unless told otherwise; null
 // hands the media query back to nativeTheme, which is what is under test.
 const app=await electron.launch({args:[path.resolve(__dirname,'..')],env,colorScheme:null})
 const page=await app.firstWindow();page.setDefaultTimeout(7000)
 await page.locator('#workspace').waitFor()
 return {app,page}
}
const bodyBackground=page=>page.evaluate(()=>getComputedStyle(document.body).backgroundColor)
// xterm paints its theme background on the scrollable element it wraps the screen in.
const terminalBackground=page=>page.evaluate(()=>{const element=document.querySelector('#terminal .xterm-scrollable-element');return element?getComputedStyle(element).backgroundColor:null})
const windowState=app=>app.evaluate(({BrowserWindow,nativeTheme})=>({source:nativeTheme.themeSource,background:BrowserWindow.getAllWindows()[0].getBackgroundColor().toLowerCase()}))
// Desktop keeps the appearance in its own file, never in the agent's settings.json (#746).
const storedAppearance=root=>JSON.parse(fs.readFileSync(path.join(root,'desktop.json'),'utf8')).appearance
// Bytes for the console as the agent would send them, and the colour a marker is drawn in: computed, since a palette
// colour that already passes comes from a class rule, not an inline style.
const write=(app,text)=>app.evaluate(({BrowserWindow},bytes)=>BrowserWindow.getAllWindows()[0].webContents.send('terminal-output',bytes),[...Buffer.from(text)])
const colorOf=(page,marker)=>page.waitForFunction(
 m=>{const s=[...document.querySelectorAll('#terminal .xterm-rows span')].find(x=>x.textContent.includes(m));return s&&getComputedStyle(s).color},
 marker)
 .then(h=>h.jsonValue())
// WCAG contrast ratio of two rgb(r, g, b) strings.
const luminance=color=>{const [r,g,b]=color.match(/\d+/g).slice(0,3).map(v=>Number(v)/255).map(v=>v<=0.03928?v/12.92:((v+0.055)/1.055)**2.4);return 0.2126*r+0.7152*g+0.0722*b}
const contrast=(a,b)=>{const [x,y]=[luminance(a),luminance(b)].sort((p,q)=>q-p);return (x+0.05)/(y+0.05)}

test('the appearance setting switches the whole window live and is kept',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-appearance-'))
 const server=await agent()
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 let app
 try{
  ;({app}=await launch(root));let page=await app.firstWindow()
  // No stored choice: the app follows the system.
  assert.equal((await windowState(app)).source,'system')
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'Appearance',exact:true}).click()
  const group=page.getByRole('group',{name:'Appearance'})
  const button=name=>group.getByRole('button',{name,exact:true})
  await expect(group.getByRole('button')).toHaveText(['System','Dark','Light'])
  await expect(button('System')).toHaveAttribute('aria-pressed','true')

  await button('Light').click()
  await expect(button('Light')).toHaveAttribute('aria-pressed','true')
  await expect(button('System')).toHaveAttribute('aria-pressed','false')
  await expect.poll(()=>bodyBackground(page)).toBe(LIGHT_BG)
  await expect.poll(()=>terminalBackground(page)).toBe(LIGHT_BG)
  assert.deepEqual(await windowState(app),{source:'light',background:'#f5f6f8'})
  assert.equal(storedAppearance(root),'light')
  await page.screenshot({path:path.join(root,'settings-light.png')})
  console.log('Light screenshot: '+path.join(root,'settings-light.png'))

  await button('Dark').click()
  await expect(button('Dark')).toHaveAttribute('aria-pressed','true')
  await expect.poll(()=>bodyBackground(page)).toBe(DARK_BG)
  await expect.poll(()=>terminalBackground(page)).toBe(DARK_BG)
  assert.deepEqual(await windowState(app),{source:'dark',background:'#11151c'})
  assert.equal(storedAppearance(root),'dark')

  // Back on System, the page answers the OS appearance: emulate a light one.
  await button('System').click()
  await expect(button('System')).toHaveAttribute('aria-pressed','true')
  assert.equal(storedAppearance(root),'system')
  await page.emulateMedia({colorScheme:'light'})
  await expect.poll(()=>bodyBackground(page)).toBe(LIGHT_BG)
  await expect.poll(()=>terminalBackground(page)).toBe(LIGHT_BG)
  await page.emulateMedia({colorScheme:'dark'})
  await expect.poll(()=>bodyBackground(page)).toBe(DARK_BG)
  await expect.poll(()=>terminalBackground(page)).toBe(DARK_BG)
  await app.close();app=null

  // A stored Light choice paints the window light before the page loads.
  fs.writeFileSync(path.join(root,'desktop.json'),JSON.stringify({...JSON.parse(fs.readFileSync(path.join(root,'desktop.json'),'utf8')),appearance:'light'}))
  ;({app,page}=await launch(root))
  assert.deepEqual(await windowState(app),{source:'light',background:'#f5f6f8'})
  await expect.poll(()=>bodyBackground(page)).toBe(LIGHT_BG)
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'Appearance',exact:true}).click()
  await expect(page.getByRole('group',{name:'Appearance'}).getByRole('button',{name:'Light',exact:true})).toHaveAttribute('aria-pressed','true')
 }finally{
  await app?.close()
  server.close()
 }
})

// #720: a CLI printing for a dark background - Claude Code's bold questions come in truecolor white - stays readable in
// the light console, and the dark console shows colours as printed.
test('the light console darkens pale text until it reads, the dark one leaves it as printed',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-contrast-'))
 const server=await agent()
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 let app
 try{
  let page;({app,page}=await launch(root))
  // The settings page takes the console's place, and xterm draws nothing in a hidden element: each switch goes back to
  // the console.
  const appearance=async name=>{
   await page.locator('#settings').click()
   await page.getByRole('tab',{name:'Appearance',exact:true}).click()
   const button=page.getByRole('group',{name:'Appearance'}).getByRole('button',{name,exact:true})
   await button.click();await expect(button).toHaveAttribute('aria-pressed','true')
   await page.getByRole('button',{name:'Back',exact:true}).click()
   await page.locator('#terminal').waitFor()
  }

  await appearance('Light')
  await expect.poll(()=>terminalBackground(page)).toBe(LIGHT_BG)
  await write(app,'\x1b[1;38;2;255;255;255mQ720RGB\x1b[0m \x1b[1;37mQ720ANSI\x1b[0m \x1b[38;2;60;60;60mQ720GREY\x1b[0m\r\n')
  const background=await terminalBackground(page)
  for(const marker of ['Q720RGB','Q720ANSI']){
   const color=await colorOf(page,marker)
   assert.ok(contrast(color,background)>=4.5,marker+' '+color+' on '+background)
  }

  // Dark grey fails 4.5 on the dark background: it is left as printed only if the floor really dropped to 1.
  await appearance('Dark')
  await expect.poll(()=>colorOf(page,'Q720RGB')).toBe('rgb(255, 255, 255)')
  assert.equal(await colorOf(page,'Q720GREY'),'rgb(60, 60, 60)')

  await appearance('Light')
  await expect.poll(async()=>contrast(await colorOf(page,'Q720RGB'),background)).toBeGreaterThanOrEqual(4.5)
 }finally{
  await app?.close()
  server.close()
 }
})
