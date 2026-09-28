// Real board and cards; mocked context operations and engine responses.
// Run with PLAYWRIGHT_MODULE pointing to an installed Playwright module.
import assert from 'node:assert/strict'
import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const fixtureId = root + '/progressive-board-fixture.tsx'
const harness = `
import React from 'react';
import {createRoot} from 'react-dom/client';
import {BoardView} from '/src/components/BoardView.tsx';
import {translations} from '/src/locales/translations.ts';
import '/src/index.css';
window.translations=translations;window.calls=[];window.engineReads=[];window.provider='claude';
window.fetch=async url=>{
 if(url.endsWith('/engine')) {
  engineReads.push(url);await new Promise(resolve=>setTimeout(resolve,50));
  return new Response(JSON.stringify({state:'reported',provider,model:provider,models:[provider],modelSlot:true}));
 }
 return new Response(JSON.stringify({agents:[]}));
};
window.EventSource=class {addEventListener(){} close(){}};
window.allTasks=Array.from({length:105},(_,i)=>({id:'t'+(i+1),key:'#'+(i+1),title:'Ticket '+(i+1),projectId:i<70?'p':'q',status:'to_clarify',priority:'medium',source:'local',labels:['new'],issueType:'Story'}));
const projects=[{id:'p',name:'First'},{id:'q',name:'Second'}];
window.ctx={tasks:allTasks,boardSort:{field:'key',asc:true},boardCardDisplayMode:'condensed',projects,currentProject:projects[0],activities:[],activeTasks:new Set(),boardGrouping:'workflow',hideDone:false,isPinned:()=>false,settings:{language:'fr',density:'compact'},t:translations.fr,skillLabel:x=>x,skillCommand:(id,command)=>command,parentFilter:null,
startBatchPickup:async ids=>{calls.push(['batch',ids]);return false},
moveTaskWorkflowStage:async(...args)=>calls.push(['move',...args]),
setBoardGrouping:value=>{ctx.boardGrouping=value;render()},
setBoardSort:value=>{ctx.boardSort=value;render()},
toggleBoardCardDisplayMode:()=>{ctx.boardCardDisplayMode=ctx.boardCardDisplayMode==='condensed'?'expanded':'condensed';render()},
setSelectedTask:task=>calls.push(['details',task.id])};
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<React.StrictMode><div style={{height:600}}><BoardView/></div></React.StrictMode>);render();`
const server = await createServer({
 root,resolve:{preserveSymlinks},configFile:root+'/vite.config.ts',server:{port:0,host:'127.0.0.1'},
 plugins:[{name:'progressive-board-fixture',enforce:'pre',
  transform(code,id){
   if(id.endsWith('/TaskFilters.tsx'))return 'export const TaskFilters=()=>null'
   if(id.includes('/src/components/'))return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/, 'const useApp=()=>window.ctx')
  },
  configureServer(s){s.middlewares.use(async(req,res,next)=>{
   if(req.url!=='/fixture')return next()
   res.setHeader('Content-Type','text/html')
   res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/progressive-board-fixture.tsx"></script>'))
  })},
  resolveId(id){if(id==='/progressive-board-fixture.tsx')return fixtureId},
  load(id){if(id===fixtureId)return harness},
 }],
})
await server.listen()
let browser
try {
 browser=await chromium.launch({headless:true,channel:'chrome'})
 const page=await browser.newPage({viewport:{width:1600,height:850}})
 page.setDefaultTimeout(8000)
 const errors=[]
 page.on('pageerror',error=>errors.push(error.message))
 const url=`http://127.0.0.1:${server.httpServer.address().port}/fixture`
 await page.goto(url)
 const cards=page.locator('[draggable="true"]')
 const firstColumn=page.locator('.kanban-column').first()
 const scroll=firstColumn.locator('.overflow-y-auto')
 const expectCount=count=>page.waitForFunction(n=>document.querySelectorAll('[draggable="true"]').length===n,count)
 await page.getByText('Ticket 1',{exact:true}).waitFor()
 await page.waitForFunction(()=>engineReads.length===2)
 assert.equal(await cards.count(),20,'only the first group mounts')
 assert.equal(await firstColumn.getByText('105',{exact:true}).count(),1,'count includes unmounted cards')
 assert.deepEqual(await page.evaluate(()=>engineReads.slice().sort()),['/api/projects/p/engine','/api/projects/q/engine'],'one read per project, including projects beyond the first page')
 await cards.first().getByRole('checkbox').click()
 await page.getByRole('button',{name:'Lot',exact:true}).waitFor()
 await scroll.evaluate(el=>{el.scrollTop=el.scrollHeight})
 await expectCount(40)
 assert.equal(await page.evaluate(()=>engineReads.length),2,'new cards start no requests')
 await page.getByRole('button',{name:'Lot',exact:true}).click()
 assert.deepEqual(await page.evaluate(()=>calls.at(-1)),['batch',['t1']],'selection survives loading')
 const top=await scroll.evaluate(el=>el.scrollTop)
 await page.evaluate(()=>{provider='codex';window.dispatchEvent(new Event('focus'))})
 await page.waitForFunction(()=>engineReads.length===4)
 await page.waitForFunction(()=>document.body.textContent.includes('CDX'))
 assert.equal(await cards.count(),40)
 assert.equal(await scroll.evaluate(el=>el.scrollTop),top,'refresh preserves scrolling')
 await page.evaluate(()=>{ctx.tasks=ctx.tasks.map(t=>({...t,description:'Updated'}));render()})
 assert.equal(await cards.count(),40)
 await cards.first().evaluate(el=>{
  const dataTransfer=new DataTransfer()
  el.dispatchEvent(new DragEvent('dragstart',{bubbles:true,dataTransfer}))
  document.querySelectorAll('.kanban-column')[1].dispatchEvent(new DragEvent('drop',{bubbles:true,dataTransfer}))
 })
 await page.waitForFunction(()=>calls.some(call=>call[0]==='move'))
 assert.deepEqual(await page.evaluate(()=>calls.find(call=>call[0]==='move').slice(0,3)),['move','t1','clarified'])
 for(const expected of [60,80,100,105]){
  await scroll.evaluate(el=>{el.scrollTop=el.scrollHeight})
  await expectCount(expected)
 }
 assert.equal(await page.getByRole('button',{name:'Afficher plus de tâches'}).count(),0)
 assert.equal(await page.evaluate(()=>engineReads.length),4)
 await page.getByTestId('board-sort-direction').click()
 await page.getByText('Ticket 105',{exact:true}).waitFor()
 assert.equal(await cards.count(),20)
 assert.equal(await scroll.evaluate(el=>el.scrollTop),0,'sorting resets the page')
 await page.evaluate(()=>{ctx.tasks=allTasks.filter(t=>t.id==='t99');render()})
 await page.getByText('Ticket 99',{exact:true}).waitFor()
 await expectCount(1)
 assert.equal(await cards.count(),1,'filtering can find an unmounted task')
 await page.getByRole('button',{name:'Lot',exact:true}).waitFor({state:'hidden'})
 await page.evaluate(()=>{ctx.tasks=allTasks;ctx.boardGrouping='status';render()})
 await expectCount(20)
 await page.evaluate(()=>{
  ctx.currentProject={...ctx.currentProject,issueTracker:'jira',trackerColumns:[{name:'Known',statuses:['Open']}]}
  ctx.tasks=allTasks.map(t=>({...t,trackerStatus:'Unmapped'}));render()
 })
 await page.getByText('Non classé',{exact:true}).waitFor()
 assert.equal(await cards.count(),20)
 await page.addInitScript(()=>{window.IntersectionObserver=undefined})
 await page.goto(url)
 await page.getByText('Ticket 1',{exact:true}).waitFor()
 const more=page.getByRole('button',{name:'Afficher plus de tâches'})
 await more.focus()
 await page.keyboard.press('Enter')
 await expectCount(40)
 await page.evaluate(()=>{ctx.settings.language='en';ctx.t=translations.en;render()})
 await page.getByRole('button',{name:'Show more tasks'}).waitFor()
 assert.deepEqual(errors,[])
 console.log('PASS: progressive scrolling, engine reads and refresh, counts, selection, drag payload, sorting, filtering, status/fallback columns and keyboard fallback')
} finally {
 await browser?.close()
 await server.close()
}
