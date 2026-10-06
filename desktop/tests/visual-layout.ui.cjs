const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// A deterministic visual sweep of every workstation and project settings panel.
// Screenshots stay outside the repository; no real workstation or tracker is used.
test('configuration panels remain usable at desktop and compact window sizes',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-visual-review-'))
 const shots=process.env.SECTILE_VISUAL_REVIEW_DIR||root
 fs.mkdirSync(shots,{recursive:true})
 const project={id:'p',name:'Sectile review project',path:'/tmp/sectile-visual-fixture',configured:true}
 const engines={catalogue:[{id:'claude',name:'Claude development engine',provider:'claude',model:'claude-sonnet-5'},{id:'codex',name:'Codex review engine',provider:'codex',model:'gpt-5'}],default:'claude',providers:['claude','codex','agy','custom'],providerModels:{},projects:{},taskCounts:{}}
 const workstation={globalConfiguration:true,defaults:{editorCommand:'zed'},effective:{defaultEngine:engines.catalogue[0],editorCommand:'zed',useWorktrees:true,parallelism:2,aiProviderModels:{}},providerModels:{claude:['claude-sonnet-5'],codex:['gpt-5']},setupProviders:['claude','codex','agy'],seeded:{},claudeSandbox:{},projects:[]}
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  const routes={
   '/desktop/status':{connected:true,server:'https://sectile.example.test',capabilities:['task-engines','repositories','git-diff']},
   '/desktop/projects':[project],'/desktop/project':{...project,parallelism:2,useWorktrees:true,claudeSandbox:{},server:{skills:[],name:project.name}},
   '/desktop/workstation':workstation,'/desktop/workstation/sandbox':workstation,'/desktop/engines':engines,
   '/desktop/runs':[],'/desktop/tasks':[],'/desktop/repositories':[],
   '/desktop/version':{version:'v0.3.0'},
   '/desktop/mcp':{path:'/tmp/sectile-review/config.json',server:'https://sectile.example.test',localURL:'http://127.0.0.1:4567',choice:{target:'remote',transport:'http'}},
  }
  if(url.pathname in routes){res.end(JSON.stringify(routes[url.pathname]));return}
  res.writeHead(404).end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'fixture-only'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 const measurements=[]
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env,colorScheme:null})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  const errors=[];page.on('pageerror',error=>errors.push(error.message))
  await expect(page.getByRole('button',{name:'Actions for '+project.name,exact:true})).toBeVisible()
  const capture=async name=>{
   await page.mouse.move(750,65)
   await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))))
   await page.screenshot({path:path.join(shots,name+'.png'),scale:'css'})
   const measurement=await page.locator('.settings-content').evaluate(element=>({width:element.clientWidth,scrollWidth:element.scrollWidth,height:element.clientHeight,scrollHeight:element.scrollHeight}))
   measurements.push({name,...measurement})
   const scrolled=await page.locator('.settings-content').evaluate(element=>{
    let moved=false
    for(const item of [element,...element.querySelectorAll('*')]){
     if(['auto','scroll'].includes(getComputedStyle(item).overflowY)&&item.scrollHeight>item.clientHeight+5){item.scrollTop=item.scrollHeight;moved=true}
    }
    return moved
   })
   if(scrolled){
    await page.screenshot({path:path.join(shots,name+'-bottom.png'),scale:'css'})
   }
  }
  for(const [theme,width,height] of [['dark',1240,820],['light',800,650]]){
   await app.evaluate(({BrowserWindow,nativeTheme},{theme,width,height})=>{nativeTheme.themeSource=theme;BrowserWindow.getAllWindows()[0].setSize(width,height)},{theme,width,height})
   await expect.poll(()=>page.evaluate(()=>matchMedia('(prefers-color-scheme: dark)').matches)).toBe(theme==='dark')
   await page.locator('#settings').click()
   for(const category of ['Profile','Appearance','Connection','AgentCli','Engines','Sandbox','Deployment','Logs','Changelog']){
    await page.locator('#settings-tab-'+category).click()
    await expect(page.locator('#settings-tab-'+category)).toHaveAttribute('aria-selected','true')
    await capture(theme+'-'+width+'-'+category)
    if(category==='Engines'){
     await page.getByRole('button',{name:'Add an engine',exact:true}).click()
     await expect(page.getByRole('group',{name:'Engine editor'})).toBeVisible()
     await capture(theme+'-'+width+'-EngineEditor')
     await page.getByRole('group',{name:'Engine editor'}).getByRole('button',{name:'Cancel',exact:true}).click()
    }
   }
   await page.getByRole('button',{name:'Back',exact:true}).click()
   await page.getByRole('button',{name:'Actions for '+project.name,exact:true}).click()
   await page.getByRole('menuitem',{name:'Project settings…',exact:true}).click()
   for(const category of ['Remove','General','Execution','Sandbox']){
    await page.locator('#project-tab-'+category).click()
    await capture(theme+'-'+width+'-project-'+category)
   }
   await page.getByRole('button',{name:'Back',exact:true}).click()
  }
  fs.writeFileSync(path.join(shots,'measurements.json'),JSON.stringify(measurements,null,2))
  console.log('Visual review captures: '+shots)
  assert.deepEqual(errors,[])
  const overflow=measurements.filter(value=>value.scrollWidth>value.width+2)
  assert.deepEqual(overflow,[],'settings must not require horizontal scrolling')
 }finally{await app?.close();await new Promise(resolve=>server.close(resolve))}
})
