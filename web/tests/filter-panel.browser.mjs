// Real board toolbar and filters; mocked context.
// Run with PLAYWRIGHT_MODULE pointing to an installed Playwright module.
// SCREENSHOT_DIR, when set, receives a capture of the toolbar at each width.
import assert from 'node:assert/strict'
import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const fixtureId = root + '/filter-panel-fixture.tsx'
const harness = `
import React from 'react';
import {createRoot} from 'react-dom/client';
import {BoardView} from '/src/components/BoardView.tsx';
import {translations} from '/src/locales/translations.ts';
import '/src/index.css';
window.fetch=async()=>new Response(JSON.stringify({agents:[]}));
window.EventSource=class {addEventListener(){} close(){}};
const set=key=>value=>{ctx[key]=value;render()};
const projects=[{id:'p',name:'PE'}];
window.ctx={tasks:[{id:'t1',key:'PE-1',title:'Ticket 1',projectId:'p',status:'to_clarify',priority:'medium',source:'jira',labels:[],issueType:'Story'}],
 boardSort:{field:'key',asc:true},boardCardDisplayMode:'expanded',projects,currentProject:projects[0],activities:[],activeTasks:new Set(['t1']),
 boardGrouping:'workflow',hideDone:false,isPinned:()=>false,settings:{language:'fr',density:'compact'},t:translations.fr,skillLabel:x=>x,skillCommand:(id,command)=>command,
 pinnedTasks:[],pinnedOnly:false,setPinnedOnly:set('pinnedOnly'),activeOnly:false,setActiveOnly:set('activeOnly'),
 priorityFilter:null,setPriorityFilter:set('priorityFilter'),
 taskFacets:{scope:'p',sprints:['2026-Q3-06 PE','2026-Q3-07 PE'],teams:['Platform'],macros:[{key:'PE-1065',title:'Legacy cleanup',count:3}],noMacroCount:4,
  assignees:['Ada'],unassignedCount:2,trackerStatuses:[{value:'To Do',count:5},{value:'In Progress',count:2}],statuses:[],sources:[],labels:[],
  issueTypes:[{value:'Story',count:4},{value:'Task',count:3},{value:'Epic',count:1}],total:7},
 sprintFilter:null,setSprintFilter:set('sprintFilter'),teamFilter:null,setTeamFilter:set('teamFilter'),assigneeFilter:null,setAssigneeFilter:set('assigneeFilter'),
 parentFilter:null,setParentFilter:set('parentFilter'),availableParents:[],trackerStatusFilters:[],setTrackerStatusFilters:set('trackerStatusFilters'),
 issueTypeFilters:[],setIssueTypeFilters:set('issueTypeFilters'),availableAssignees:['Ada'],unassignedFilterValue:'__unassigned__',
 setEditingProject:()=>{},setIsProjectModalOpen:()=>{},
 setBoardGrouping:set('boardGrouping'),setBoardSort:set('boardSort'),toggleBoardCardDisplayMode:()=>{},setSelectedTask:()=>{}};
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<React.StrictMode><div style={{height:600}}><BoardView/></div></React.StrictMode>);render();`
const server = await createServer({
 root,resolve:{preserveSymlinks},configFile:root+'/vite.config.ts',server:{port:0,host:'127.0.0.1'},
 plugins:[{name:'filter-panel-fixture',enforce:'pre',
  transform(code,id){
   if(id.includes('/src/components/'))return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/, 'const useApp=()=>window.ctx')
  },
  configureServer(s){s.middlewares.use(async(req,res,next)=>{
   if(req.url!=='/fixture')return next()
   res.setHeader('Content-Type','text/html')
   res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/filter-panel-fixture.tsx"></script>'))
  })},
  resolveId(id){if(id==='/filter-panel-fixture.tsx')return fixtureId},
  load(id){if(id===fixtureId)return harness},
 }],
})
await server.listen()
let browser
try {
 browser=await chromium.launch({headless:true,channel:'chrome'})
 const page=await browser.newPage({viewport:{width:1100,height:700}})
 page.setDefaultTimeout(8000)
 const errors=[]
 page.on('pageerror',error=>errors.push(error.message))
 await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`)
 const toolbar=page.locator('.border-b').first()
 const filters=page.getByTitle('Filtrer par statut, type, macro, sprint, équipe ou personne')
 await filters.waitFor()

 const rows=async()=>toolbar.evaluate(el=>new Set([...el.querySelectorAll('button')].filter(b=>b.offsetParent).map(b=>{const r=b.getBoundingClientRect();return Math.round((r.top+r.height/2)/8)})).size)
 assert.equal(await rows(),1,'the toolbar holds on one line at 1100px')
 if(process.env.SCREENSHOT_DIR)await toolbar.screenshot({path:process.env.SCREENSHOT_DIR+'/toolbar-1100.png'})

 await filters.click()
 await page.getByText('Personne',{exact:true}).waitFor()
 // A press in the lookup's portaled list must not close the panel that holds it.
 await page.getByPlaceholder('Tous sprints').click()
 await page.getByText('2026-Q3-06 PE',{exact:true}).click()
 await page.waitForFunction(()=>ctx.sprintFilter==='2026-Q3-06 PE')
 assert.equal(await page.getByText('Personne',{exact:true}).count(),1,'picking a sprint keeps the panel open')
 await page.getByRole('button',{name:/In Progress/}).click()
 await page.waitForFunction(()=>ctx.trackerStatusFilters.join()==='In Progress')
 assert.equal((await filters.innerText()).replace(/\s+/g,' '),'Filtres 2','the button counts the two narrowing dimensions')
 if(process.env.SCREENSHOT_DIR)await page.screenshot({path:process.env.SCREENSHOT_DIR+'/panel-open.png'})

 await page.getByRole('button',{name:'Réinitialiser'}).click()
 await page.waitForFunction(()=>ctx.sprintFilter===null&&ctx.trackerStatusFilters.length===0)
 await page.keyboard.press('Escape')
 await page.getByText('Personne',{exact:true}).waitFor({state:'detached'})
 await filters.click()
 await page.getByText('Personne',{exact:true}).waitFor()
 await page.mouse.click(5,650)
 await page.getByText('Personne',{exact:true}).waitFor({state:'detached'})

 await page.setViewportSize({width:1600,height:700})
 if(process.env.SCREENSHOT_DIR)await toolbar.screenshot({path:process.env.SCREENSHOT_DIR+'/toolbar-1600.png'})
 assert.deepEqual(errors,[])
 console.log('PASS: one-line toolbar, filter panel keeps open on lookup picks, counts dimensions, resets, closes on Escape and outside press')
} finally {
 await browser?.close()
 await server.close()
}
