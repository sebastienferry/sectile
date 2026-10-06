// Isolated browser regression (#636): the status line under the framing of a macro says where the framing is copied
// on the tracker (a comment on a Jira epic), offers to publish it again, and says why it stays in Sectile elsewhere.
// Real RoadmapView, its children and styles; useApp is mocked in every component.
// Run with Playwright available: node tests/roadmap-framing.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);
const harness=`import React from 'react'; import {createRoot} from 'react-dom/client'; import {RoadmapView} from '/src/components/RoadmapView.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.calls=[];
const project={id:'p',name:'Platform',issueTracker:'jira',jiraProject:'PE',sprints:[]};
const meta=(key,title,framingMirror)=>({projectId:'p',key,title,horizon:'now',description:'',framingComment:'Pourquoi '+key,todos:[],updatedAt:'',status:'open',closed:false,
  todosMirror:{kind:'jira_comment',upToDate:true},framingMirror});
window.macros=[
  meta('PE-1','Epic one',{kind:'jira_comment',upToDate:false}),
  meta('PE-2','Epic two',{kind:'jira_comment',upToDate:false,error:'tracker returned HTTP 401',credentialMissing:'jira'}),
  meta('M-3','Local epic',{kind:'',reason:'macro locale, sans épic Jira',upToDate:false}),
  meta('PE-4','Epic four',{kind:'jira_comment',upToDate:true,url:'https://site/browse/PE-4?focusedCommentId=9'}),
];
const data={tasks:[],projects:[project],currentProject:project,settings:{language:'fr'},t:translations.fr,assigneeFilter:null,myTasksOnly:false,sprintFilter:null,teamFilter:null,labelFilter:null,pinnedOnly:false,searchQuery:'',activeJobCount:0,activities:[],
  fetchProjectMacros:async()=>window.macros.map(m=>({...m})),pendingHorizonPushes:async()=>[],addToast:toast=>window.calls.push(['toast',toast.title,toast.description]),
  // A save answers the macro as stored, its copy now waiting to be published.
  saveMacroMeta:async(pid,key,patch)=>{
    window.calls.push(['saveMacroMeta',key,JSON.parse(JSON.stringify(patch))]);
    const current=window.macros.find(m=>m.key===key);
    const saved={...current,...patch,framingMirror:{...current.framingMirror,upToDate:false}};
    window.macros=window.macros.map(m=>m.key===key?saved:m);
    return saved;
  },
  republishMacroTodos:async(pid,key)=>{window.calls.push(['republishMacroTodos',pid,key]);return true},
  republishMacroFraming:async(pid,key)=>{window.calls.push(['republishMacroFraming',pid,key]);return true},
  openTrackerCredentials:tracker=>window.calls.push(['openTrackerCredentials',tracker])};
window.ctx=new Proxy(data,{get:(target,key)=>key in target?target[key]:(...args)=>{window.calls.push([String(key),...args]);return Promise.resolve(null)}});
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<div style={{height:'800px',display:'flex'}}><RoadmapView/></div>);
window.render();`;
const server=await createServer({root,resolve:{preserveSymlinks},configFile:root+'/vite.config.ts',server:{port:0,host:'127.0.0.1'},plugins:[{name:'fixture',enforce:'pre',transform(code,id){if(id.includes('/src/components/')) return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/,'const useApp = () => window.ctx');},configureServer(s){s.middlewares.use(async(req,res,next)=>{if(req.url==='/fixture'){res.setHeader('Content-Type','text/html');res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/fixture.tsx"></script>'))}else next()})},resolveId(id){if(id==='/fixture.tsx')return root+'/fixture.tsx'},load(id){if(id===root+'/fixture.tsx')return harness}}]});
await server.listen();
let browser;
try {
  browser=await chromium.launch({headless:true,channel:'chrome'});
  const page=await browser.newPage({viewport:{width:1400,height:1000}});
  page.setDefaultTimeout(10000);
  const errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`);
  await page.evaluate(()=>localStorage.clear());
  await page.reload();

  const panel=()=>page.locator('aside');
  const row=title=>page.locator('div.cursor-pointer').filter({hasText:title}).first();
  const calls=name=>page.evaluate(n=>calls.filter(c=>c[0]===n),name);
  const status=()=>panel().locator('[data-testid="framing-mirror"]');
  const todos=()=>panel().locator('[data-testid="todos-mirror"]');

  // US4.1, US4.5: a copy waiting to be written offers to publish it again, and only the framing is queued.
  await row('Epic one').click();
  await page.getByRole('tab',{name:'Framing',exact:true}).click();
  await status().waitFor();
  assert.equal(await status().getAttribute('data-state'),'pending');
  await status().getByText('Publication en attente').waitFor();
  await status().getByRole('button',{name:'Republier'}).click();
  await page.waitForFunction(()=>window.calls.some(c=>c[0]==='republishMacroFraming'));
  assert.deepEqual((await calls('republishMacroFraming'))[0],['republishMacroFraming','p','PE-1']);
  assert.equal((await calls('republishMacroTodos')).length,0);
  // The todos line is unchanged beside it.
  assert.equal(await todos().getAttribute('data-state'),'upToDate');
  await todos().getByText('Recopiée sur PE-1').waitFor();

  // US4.4: a failure names its reason and offers the missing token.
  await row('Epic two').click();
  await page.waitForFunction(()=>document.querySelector('[data-testid="framing-mirror"]')?.dataset.state==='failed');
  await status().getByText('Échec de publication : tracker returned HTTP 401').waitFor();
  await status().getByRole('button',{name:'Ajouter mon jeton Jira'}).click();
  assert.deepEqual((await calls('openTrackerCredentials'))[0],['openTrackerCredentials','jira']);
  assert.equal(await status().getByRole('button',{name:'Republier'}).count(),1);

  // US3.1: a macro with a local key keeps its framing in Sectile, and offers nothing to publish.
  await row('Local epic').click();
  await page.waitForFunction(()=>document.querySelector('[data-testid="framing-mirror"]')?.dataset.state==='local');
  await status().getByText('Reste dans Sectile : macro locale, sans épic Jira').waitFor();
  assert.equal(await status().getByRole('button').count(),0);

  // US4.1: an up-to-date copy links the comment.
  await row('Epic four').click();
  await page.waitForFunction(()=>document.querySelector('[data-testid="framing-mirror"]')?.dataset.state==='upToDate');
  await status().getByText('Recopié sur PE-4').waitFor();
  assert.equal(await status().locator('a').getAttribute('href'),'https://site/browse/PE-4?focusedCommentId=9');
  assert.equal(await status().getByRole('button').count(),0);

  assert.deepEqual(errors,[]);
  console.log('roadmap-framing.browser: ok');
} finally {
  await browser?.close();
  await server.close();
}
