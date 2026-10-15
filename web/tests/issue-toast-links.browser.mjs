import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
import assert from 'node:assert/strict'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || '../../desktop/node_modules/playwright/index.mjs')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const harness = `import React from 'react'; import {createRoot} from 'react-dom/client'; import {AppProvider,useApp} from '/src/context/AppContext.tsx'; import {ToastContainer} from '/src/components/ToastContainer.tsx'; import '/src/index.css';
const json=b=>new Response(JSON.stringify(b),{headers:{'Content-Type':'application/json'}});
window.answer={};window.status=200;window.activities=[];window.byId={};
const board=[{id:'L',projectId:'p',key:'L-1',title:'Loaded L',labels:[],externalUrl:'https://tracker.example/L',source:'local'}];
window.fetch=async(input,init={})=>{const p=new URL(input,location.origin).pathname;
if(['POST','PUT','PATCH','DELETE'].includes(init.method))return new Response(JSON.stringify(window.answer),{status:window.status,headers:{'Content-Type':'application/json'}});
if(p==='/api/me')return json({userId:'u',signedIn:true,mode:'local',role:'admin'});
if(p==='/api/settings')return json({language:'en'});
if(p==='/api/tasks/facets')return json({});
if(/stats|status/.test(p))return json({});
if(p==='/api/tasks')return json(board);
if(p==='/api/activities')return json(window.activities);
const one=p.split('/');if(one.length===4&&one[2]==='tasks'&&one[3]!=='pins')return window.byId[one[3]]?json(window.byId[one[3]]):new Response('{}',{status:404});
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
  // #841: every toast about an existing ticket links it, whatever its type.
  await page.evaluate(()=>{window.status=200})
  await invoke('syncSingleTask',['a'],{task:task('a','https://tracker.example/a')})
  await page.getByRole('button',{name:'Open a',exact:true}).click()
  assert.equal(await page.locator('output').textContent(),'Returned a','sync success opens the realigned ticket')
  await page.evaluate(()=>{window.status=500})
  await invoke('updateTask',['L',{}],{error:'Refused by the server'})
  await page.getByText('Refused by the server').waitFor()
  await page.getByRole('button',{name:'Open L-1',exact:true}).click()
  assert.equal(await page.locator('output').textContent(),'Loaded L','an error toast links its ticket from the board')
  // The credential offer comes first, the ticket second.
  await page.evaluate(()=>{window.status=403})
  await invoke('setTaskSprint',['L','s1','Sprint 1'],{code:'tracker_credential_missing',tracker:'jira',error:'No token'})
  const actions=page.locator('.pointer-events-auto button:not([aria-label])')
  await actions.first().waitFor()
  assert.deepEqual(await actions.allTextContents(),['Add my Jira token','Open L-1'])
  // Several tickets: one link each up to five, then a count.
  await page.evaluate(()=>{window.status=200})
  await invoke('setTasksSprint',['p',['L','L','L'],'s1','Sprint 1'],{count:3})
  await page.getByRole('button',{name:'Open L-1',exact:true}).first().waitFor()
  assert.equal(await page.getByRole('button',{name:'Open L-1',exact:true}).count(),3)
  assert.equal(await page.getByText(/^\+\d+ more$/).count(),0)
  await page.evaluate(()=>{ctx.toasts.forEach(t=>ctx.removeToast(t.id))})
  await page.evaluate(()=>ctx.addToast({type:'success',title:'Seven',...(()=>{const r=ctx.ticketLinks([...Array(5).fill({id:'L'}),{id:'x'},{id:'y'}]);return {links:r.links,moreLinks:r.more}})()}))
  await page.getByText('+2 more',{exact:true}).waitFor()
  assert.equal(await page.getByRole('button',{name:'Open L-1',exact:true}).count(),5)
  await invoke('setTasksSprint',['p',['L','L','L','L','L','L','L'],'s1','Sprint 1'],{count:7})
  await page.getByText('+2 more',{exact:true}).waitFor()
  // A ticket off the board is read when its link is clicked; a failed read says so.
  await page.evaluate(()=>{window.byId={U:{id:'U',projectId:'q',key:'U-7',title:'Remote U',labels:[],source:'local'}}})
  await page.evaluate(()=>{ctx.toasts.forEach(t=>ctx.removeToast(t.id));ctx.addToast({type:'info',title:'Lazy',link:ctx.ticketLink({id:'U',key:'U-7'})})})
  await page.getByRole('button',{name:'Open U-7',exact:true}).click()
  await page.waitForFunction(()=>ctx.selectedTask?.title==='Remote U')
  await page.evaluate(()=>{ctx.setSelectedTask(null);ctx.addToast({type:'info',title:'Gone',link:ctx.ticketLink({id:'gone',key:'J-9'})})})
  await page.getByRole('button',{name:'Open J-9',exact:true}).click()
  await page.getByText('Ticket J-9 could not be opened').waitFor()
  assert.equal(await page.evaluate(()=>ctx.selectedTask),null)
  assert.equal(await page.evaluate(()=>ctx.ticketLink({id:'nokey'})),undefined,'a lazy link needs the key')
  // Run outcomes from the poller: a loaded ticket with its tracker icon, an unloaded one read on click.
  await page.evaluate(()=>{ctx.toasts.forEach(t=>ctx.removeToast(t.id))})
  const run=(id,taskId,taskKey,status)=>({id,taskId,taskKey,status,skillId:'implement',skillName:'Implement',action:'',output:'',steps:[],createdAt:'2026-10-15T00:00:00Z'})
  await page.evaluate(async r=>{window.activities=r;await ctx.fetchActivities()},[run('r1','L','L-1','running'),run('r2','U','U-7','running')])
  await page.waitForTimeout(4500)
  await page.evaluate(r=>{window.activities=r},[run('r1','L','L-1','completed'),run('r2','U','U-7','failed')])
  const loadedRun=page.getByRole('button',{name:'Open L-1',exact:true})
  await loadedRun.waitFor({timeout:15000})
  assert.equal(await page.getByRole('link',{name:'Open in the tracker: Open L-1'}).count(),1)
  await page.getByRole('button',{name:'Open U-7',exact:true}).click()
  await page.waitForFunction(()=>ctx.selectedTask?.title==='Remote U')
  await page.evaluate(()=>{window.activities=[]})
  // Deletion offers no link; last, since it takes L off the board.
  await invoke('deleteTask',['L'],{})
  await page.getByText('Task deleted').waitFor()
  assert.equal(await page.getByRole('button',{name:/^Open /}).count(),0)
  assert.deepEqual(errors,[])
  console.log('Issue toast producer and navigation browser checks passed')
} finally {await browser?.close();await server.close()}
