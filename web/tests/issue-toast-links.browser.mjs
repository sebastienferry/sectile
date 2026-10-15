import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
import assert from 'node:assert/strict'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || '../../desktop/node_modules/playwright/index.mjs')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const harness = `import React from 'react'; import {createRoot} from 'react-dom/client'; import {AppProvider,useApp} from '/src/context/AppContext.tsx'; import {ToastContainer} from '/src/components/ToastContainer.tsx'; import '/src/index.css';
const json=b=>new Response(JSON.stringify(b),{headers:{'Content-Type':'application/json'}});
window.answer={};
window.fetch=async(input,init={})=>{const p=new URL(input,location.origin).pathname;
if(init.method==='POST'||init.method==='PUT')return json(window.answer);
if(p==='/api/me')return json({userId:'u',signedIn:true,mode:'local',role:'admin'});
if(p==='/api/settings')return json({language:'en'});
if(p==='/api/tasks/facets')return json({});
if(/stats|status/.test(p))return json({});
return json([])};
function Probe(){window.ctx=useApp();return <><output>{window.ctx.selectedTask?.title||''}</output><ToastContainer/></>}
createRoot(document.getElementById('root')).render(<AppProvider><Probe/></AppProvider>);`
const server = await createServer({ root, resolve: {preserveSymlinks}, configFile:root+'/vite.config.ts', server:{port:0,host:'127.0.0.1'}, plugins:[{name:'fixture',configureServer(s){s.middlewares.use(async(req,res,next)=>{if(req.url==='/fixture'){res.setHeader('Content-Type','text/html');res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/fixture.tsx"></script>'))}else next()})},resolveId(id){if(id==='/fixture.tsx'||id===root+'/fixture.tsx')return root+'/fixture.tsx'},load(id){if(id===root+'/fixture.tsx')return harness}}] })
await server.listen()
let browser
try {
  browser=await chromium.launch({headless:true,channel:'chrome'})
  const page=await browser.newPage()
  page.setDefaultTimeout(10000)
  await page.context().route('https://tracker.example/**',route=>route.fulfill({body:'Tracker fixture'}))
  const errors=[];page.on('pageerror',e=>errors.push(e.message))
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`)
  await page.waitForFunction(()=>window.ctx)
  const task=(id,url)=>({id,projectId:'p',key:id||'J-9',title:'Returned '+id,labels:[],externalUrl:url,source:'local'})
  const invoke=async(name,args,answer)=>{
    await page.evaluate(()=>{ctx.toasts.forEach(t=>ctx.removeToast(t.id));ctx.setSelectedTask(null)})
    await page.waitForFunction(()=>ctx.selectedTask===null && ctx.toasts.length===0)
    await page.evaluate(async({name,args,answer})=>{window.answer=answer;await ctx[name](...args)},{name,args,answer})
  }
  for(const [name,args] of [['createTask',[{title:'New'}]],['updateTask',['a',{}]],['cloneTask',['a',{},false]],['cloneTask',['a',{},true]]]){
    await invoke(name,args,task('a','https://tracker.example/a'))
    const button=page.getByRole('button',{name:'Open a',exact:true});await button.waitFor()
    assert.equal(await page.locator('output').textContent(), name==='cloneTask' && args[2] ? 'Returned a' : '', 'clone automatic opening choice')
    const tracker=page.getByRole('link');assert.equal(await tracker.getAttribute('target'),'_blank')
    assert.equal(await tracker.getAttribute('rel'),'noopener noreferrer')
    await button.click();assert.equal(await page.locator('output').textContent(),'Returned a')
    assert.equal(await button.count(),0)
  }
  await invoke('updateTask',['b',{}],task('b'))
  assert.equal(await page.getByRole('link').count(),0)
  // Bulk callers keep one independently targeted toast per update.
  await page.evaluate(async()=>{window.answer={id:'c',key:'c',title:'Returned c',labels:[]};await ctx.updateTask('c',{})})
  assert.equal(await page.getByRole('button',{name:/^Open [bc]$/}).count(),2)
  await page.getByRole('button',{name:'Open c',exact:true}).click()
  assert.equal(await page.locator('output').textContent(),'Returned c')
  for(const [name,args] of [['createStoryFromMacroTodo',['p','M-1','todo']],['createStoryUnderMacro',['p','M-1','Story']]]){
    await invoke(name,args,{task:task('','https://tracker.example/J-9'),storyKey:'J-9',notice:'Parent warning'})
    const anchor=page.getByRole('link',{name:'Open in the tracker J-9',exact:true});await anchor.waitFor()
    assert.equal(await anchor.getAttribute('href'),'https://tracker.example/J-9')
    assert.equal(await page.getByRole('button',{name:/^Open /}).count(),0)
    await anchor.focus();assert.equal(await page.evaluate(()=>ctx.selectedTask),null)
    const popupPromise=page.waitForEvent('popup');await anchor.click();const popup=await popupPromise
    await popup.waitForLoadState();assert.equal(popup.url(),'https://tracker.example/J-9');await popup.close()
    assert.equal(await anchor.count(),1,'external activation retains the toast')
    assert.ok(await page.getByText('Parent warning').count())
  }
  const results=[{status:'created',task:task('d')},{status:'skipped',task:task('skip')},{status:'created',task:task('','https://tracker.example/J-9')}]
  await invoke('createStoriesFromMacroTodos',['p','M-1',['x','y']],{results,created:2,skipped:1,failed:0})
  await page.getByRole('button',{name:'Open d',exact:true}).waitFor()
  assert.equal(await page.getByRole('link',{name:'Open in the tracker J-9',exact:true}).count(),1)
  assert.equal(await page.getByRole('button',{name:'Open skip',exact:true}).count(),0)
  await page.getByRole('link').focus()
  await page.getByRole('button',{name:'Open d',exact:true}).focus()
  await page.waitForTimeout(8100)
  assert.equal(await page.getByRole('button',{name:'Open d',exact:true}).count(),1,'keyboard focus pauses batch expiry')
  await invoke('createStoriesFromMacroTodos',['p','M-1',[]],{results,created:2,failed:1})
  assert.equal(await page.getByRole('link').count(),0)
  assert.equal(await page.getByRole('button',{name:/^Open /}).count(),0)
  await invoke('createStoriesFromMacroTodos',['p','M-1',[]],{results:[{status:'skipped'}],created:0,failed:0,skipped:1})
  assert.equal(await page.getByRole('link').count(),0)
  assert.deepEqual(errors,[])
  console.log('Issue toast producer and navigation browser checks passed')
} finally {await browser?.close();await server.close()}
