// Isolated browser regression (#630): a ticket's request opens its epic on the roadmap, on the epic's tab and
// with its panel shown, once; an epic the roadmap does not hold is refused and sends the user back; the panel
// leads to the epic's tickets. Real RoadmapView, its children and styles; useApp is mocked in every component.
// Run with Playwright available: node tests/roadmap-epic-navigation.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);
const harness=`import React from 'react'; import {createRoot} from 'react-dom/client'; import {RoadmapView} from '/src/components/RoadmapView.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.calls=[];
const project={id:'p',name:'Sectile',issueTracker:'github',sprints:[]};
const meta=(key,title,horizon,extra={})=>({projectId:'p',key,title,horizon,description:'',framingComment:'',todos:[],updatedAt:'',status:'open',closed:false,...extra});
const macros=[meta('M-1','Macro un','now'),meta('M-2','Macro deux','later'),meta('M-3','Macro trois','later',{status:'closed',closed:true})];
const task=(id,title,parentKey)=>({id,key:'#'+id,title,status:'todo',priority:'medium',parentKey,parentTitle:'',projectId:'p',source:'github',labels:[]});
const data={tasks:[task('10','Ticket de deux','M-2')],projects:[project],currentProject:project,settings:{language:'fr'},t:translations.fr,assigneeFilter:null,myTasksOnly:false,sprintFilter:null,teamFilter:null,labelFilter:null,pinnedOnly:false,searchQuery:'',activeJobCount:0,activities:[],isLoading:false,roadmapFocus:null,
  fetchProjectMacros:async()=>macros.map(m=>({...m})),pendingHorizonPushes:async()=>[],addToast:toast=>window.calls.push(['toast',toast.title,toast.description]),
  consumeRoadmapFocus:()=>{window.calls.push(['consumeRoadmapFocus']);data.roadmapFocus=null;queueMicrotask(()=>window.render())},
  setSearchQuery:q=>{window.calls.push(['setSearchQuery',q]);data.searchQuery=q;queueMicrotask(()=>window.render())}};
window.ctx=new Proxy(data,{get:(target,key)=>key in target?target[key]:(...args)=>{window.calls.push([String(key),...args]);return Promise.resolve([])}});
window.focus=(epicKey,from='list',projectId='p')=>{data.roadmapFocus={projectId,epicKey,from};window.render()};
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<div style={{height:'700px',display:'flex'}}><RoadmapView/></div>);
window.render();`;
const server=await createServer({root,resolve:{preserveSymlinks},configFile:root+'/vite.config.ts',server:{port:0,host:'127.0.0.1'},plugins:[{name:'fixture',enforce:'pre',transform(code,id){if(id.includes('/src/components/')) return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/,'const useApp = () => window.ctx');},configureServer(s){s.middlewares.use(async(req,res,next)=>{if(req.url==='/fixture'){res.setHeader('Content-Type','text/html');res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/fixture.tsx"></script>'))}else next()})},resolveId(id){if(id==='/fixture.tsx')return root+'/fixture.tsx'},load(id){if(id===root+'/fixture.tsx')return harness}}]});
await server.listen();
let browser;
try {
  browser=await chromium.launch({headless:true,channel:'chrome'});
  const page=await browser.newPage({viewport:{width:1400,height:900}});
  page.setDefaultTimeout(10000);
  const errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`);
  await page.evaluate(()=>localStorage.clear());
  await page.reload();

  const tabButton=name=>page.getByRole('button',{name:new RegExp('^'+name+' \\d+$')});
  const activeTab=()=>page.evaluate(()=>[...document.querySelectorAll('button')].find(b=>/^(NOW|NEXT|LATER)/.test(b.textContent||'')&&getComputedStyle(b).fontWeight==='700')?.textContent?.replace(/\d+$/,'').trim());
  const panel=()=>page.locator('aside');
  const panelKeyIs=key=>page.waitForFunction(k=>document.querySelector('aside span.font-mono')?.textContent===k,key);
  const panelKey=()=>panel().locator('span.font-mono').first().textContent();
  const row=title=>page.locator('div.cursor-pointer').filter({hasText:title}).first();
  const calls=name=>page.evaluate(n=>calls.filter(c=>c[0]===n),name);
  const clearCalls=()=>page.evaluate(()=>{window.calls.length=0});
  const strings=await page.evaluate(()=>window.ctx.t.planning.roadmap);
  const prioritySelect=()=>page.getByLabel(strings.axes.filterLabel);
  const ticketsButton=()=>panel().getByRole('button',{name:strings.panel.openTickets});

  // US2.1, US2.4: from NOW with a priority filter hiding everything, a request for a LATER epic opens LATER on
  // it and clears the filter.
  await panel().waitFor();
  assert.equal(await activeTab(),'NOW');
  assert.equal(await panelKey(),'M-1');
  await prioritySelect().selectOption('P1');
  await panel().waitFor({state:'detached'});
  await page.evaluate(()=>focus('M-2'));
  await panelKeyIs('M-2');
  assert.equal(await activeTab(),'LATER','the epic horizon tab opens');
  assert.equal(await prioritySelect().inputValue(),'','the priority filter is cleared');
  assert.equal((await calls('consumeRoadmapFocus')).length,1,'the request is consumed once');
  assert.equal(await page.evaluate(()=>localStorage.getItem('sectile_roadmap_selected_key:p')),'M-2','the epic is the remembered selection');

  // US2.8: a later tab change does not bring the epic back.
  await tabButton('NOW').click();
  await panelKeyIs('M-1');
  assert.equal((await calls('consumeRoadmapFocus')).length,1);

  // US4.1, US4.2: "its tickets" on an epic with tickets, none on an empty one.
  assert.equal(await ticketsButton().count(),0,'an epic without tickets offers no way to them');
  await tabButton('LATER').click();
  await row('Macro deux').click();
  await panelKeyIs('M-2');
  await ticketsButton().click();
  assert.deepEqual(await calls('openEpicTickets'),[['openEpicTickets','M-2']]);

  // US2.3: a closed epic shows the closed epics.
  await page.evaluate(()=>focus('M-3'));
  await panelKeyIs('M-3');
  assert.equal(await activeTab(),'LATER');

  // US2.5: a hidden panel is shown again.
  await panel().getByRole('button',{name:strings.panel.hide}).click();
  await panel().waitFor({state:'detached'});
  await page.evaluate(()=>focus('M-2'));
  await panelKeyIs('M-2');

  // US2.4: a search is cleared.
  await page.evaluate(()=>{window.ctx.searchQuery='trois';window.render()});
  await clearCalls();
  await page.evaluate(()=>focus('M-1'));
  await panelKeyIs('M-1');
  assert.deepEqual(await calls('setSearchQuery'),[['setSearchQuery','']]);
  assert.equal(await activeTab(),'NOW');

  // US3: an unknown epic is refused, the user goes back, nothing moves on the roadmap.
  await clearCalls();
  await page.evaluate(()=>focus('PE-9','board'));
  await page.waitForFunction(()=>window.calls.some(c=>c[0]==='toast'));
  const [toast]=await calls('toast');
  assert.equal(toast[1],strings.focus.unknownTitle);
  assert.match(toast[2],/PE-9/,'the refusal names the key');
  assert.deepEqual(await calls('setActiveView'),[['setActiveView','board']],'the user goes back where they were');
  assert.equal(await panelKey(),'M-1','the selection is unchanged');
  assert.equal(await activeTab(),'NOW','the tab is unchanged');

  // A request made on the roadmap itself and refused stays there.
  await clearCalls();
  await page.evaluate(()=>focus('PE-9','roadmap'));
  await page.waitForFunction(()=>window.calls.some(c=>c[0]==='toast'));
  assert.deepEqual(await calls('setActiveView'),[]);

  // A request for another project waits for that project and is not consumed here.
  await clearCalls();
  await page.evaluate(()=>focus('M-2','list','q'));
  await page.waitForTimeout(300);
  assert.deepEqual(await calls('consumeRoadmapFocus'),[]);
  assert.equal(await panelKey(),'M-1');

  assert.deepEqual(errors,[]);
  console.log('roadmap epic navigation: ok');
} finally {
  await browser?.close();
  await server.close();
}
