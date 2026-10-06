// Isolated browser regression (#735): the framing panel's import row offers an upload button next to tasks.md and
// spec.md; the picked file is sent as that source, a file the server would refuse is never sent, and the buttons
// stay disabled while an import runs.
// Real RoadmapView, its children and styles; useApp is mocked in every component.
// Run with Playwright available: node tests/roadmap-slicing-upload.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);
const harness=`import React from 'react'; import {createRoot} from 'react-dom/client'; import {RoadmapView} from '/src/components/RoadmapView.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.calls=[];
const project={id:'p',name:'Platform',issueTracker:'jira',jiraProject:'PE',sprints:[]};
window.macros=[{projectId:'p',key:'PE-1',title:'Epic one',horizon:'now',description:'',framingComment:'Why',todos:[],updatedAt:'',status:'open',closed:false,
  todosMirror:{kind:'jira_comment',upToDate:true},framingMirror:{kind:'jira_comment',upToDate:true}}];
// An import answers once the test releases it, so the pending state can be observed.
window.release=null;
const data={tasks:[],projects:[project],currentProject:project,settings:{language:'en'},t:translations.en,assigneeFilter:null,myTasksOnly:false,sprintFilter:null,teamFilter:null,labelFilter:null,pinnedOnly:false,searchQuery:'',activeJobCount:0,activities:[],
  fetchProjectMacros:async()=>window.macros.map(m=>({...m})),pendingHorizonPushes:async()=>[],addToast:toast=>window.calls.push(['toast',toast.type,toast.title,toast.description]),
  produceMacroSlicing:(pid,key,source,upload)=>{
    window.calls.push(['produceMacroSlicing',pid,key,source,upload||null]);
    return new Promise(resolve=>{window.release=()=>{window.release=null;resolve({...window.macros[0],todos:[{id:'t1',text:'Group',done:false,sourceKind:source}]})}});
  }};
window.ctx=new Proxy(data,{get:(target,key)=>key in target?target[key]:(...args)=>{window.calls.push([String(key),...args]);return Promise.resolve(null)}});
const app=createRoot(document.getElementById('root'));
app.render(<div style={{height:'800px',display:'flex'}}><RoadmapView/></div>);`;
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

  const calls=name=>page.evaluate(n=>calls.filter(c=>c[0]===n),name);
  const uploadTasks=()=>page.getByRole('button',{name:'Import a local file as tasks.md'});
  const uploadSpec=()=>page.getByRole('button',{name:'Import a local file as spec.md'});
  const pick=(source,file)=>page.locator(`[data-testid="slicing-upload-${source}"]`).setInputFiles(file);
  const released=async()=>{await page.waitForFunction(()=>window.release!==null);await page.evaluate(()=>window.release());};

  await page.locator('div.cursor-pointer').filter({hasText:'Epic one'}).first().click();
  await page.getByRole('tab',{name:'Framing',exact:true}).click();

  // US1.1, US2.1, FR-1: one upload per file source, none for the stories.
  await uploadTasks().waitFor();
  await uploadSpec().waitFor();
  assert.equal(await page.locator('[data-testid="slicing-upload-stories"]').count(),0);

  // US1.5, FR-2: the file is sent as the source of the button, whatever its name.
  const tasks='## 1. First group\n\n- [ ] 1.1 Do it.\n';
  await pick('tasks',{name:'plan-v2.md',mimeType:'text/markdown',buffer:Buffer.from(tasks)});
  await page.waitForFunction(()=>window.release!==null);
  // US5.1: both upload buttons are disabled while the import runs.
  assert.equal(await uploadTasks().isDisabled(),true);
  assert.equal(await uploadSpec().isDisabled(),true);
  await page.evaluate(()=>window.release());
  await page.waitForFunction(()=>!document.querySelector('button[aria-label="Import a local file as spec.md"]')?.disabled);
  assert.deepEqual((await calls('produceMacroSlicing'))[0],['produceMacroSlicing','p','PE-1','tasks',{fileName:'plan-v2.md',content:tasks}]);

  // US2.2: same for spec.md.
  const spec='### Requirement: Something\n';
  await pick('spec',{name:'spec.md',mimeType:'text/markdown',buffer:Buffer.from(spec)});
  await released();
  assert.deepEqual((await calls('produceMacroSlicing'))[1],['produceMacroSlicing','p','PE-1','spec',{fileName:'spec.md',content:spec}]);

  // US5.3: picking the same file again imports again.
  await page.waitForFunction(()=>!document.querySelector('button[aria-label="Import a local file as tasks.md"]')?.disabled);
  await pick('tasks',{name:'plan-v2.md',mimeType:'text/markdown',buffer:Buffer.from(tasks)});
  await released();
  assert.equal((await calls('produceMacroSlicing')).length,3);

  // US4.1, US4.2, FR-3: an oversized or non-UTF-8 file is refused before anything is sent.
  await page.waitForFunction(()=>!document.querySelector('button[aria-label="Import a local file as tasks.md"]')?.disabled);
  await pick('tasks',{name:'big.md',mimeType:'text/markdown',buffer:Buffer.alloc((1<<20)+1,0x61)});
  await page.waitForFunction(()=>window.calls.some(c=>c[0]==='toast'&&c[3]==='The file exceeds the 1 MiB limit.'));
  await pick('spec',{name:'binary.md',mimeType:'text/markdown',buffer:Buffer.from([0x23,0x20,0xff,0xfe,0x41])});
  await page.waitForFunction(()=>window.calls.some(c=>c[0]==='toast'&&c[3]==='The file is not UTF-8 text.'));
  const refusals=(await calls('toast')).filter(c=>c[1]==='error');
  assert.deepEqual(refusals.map(c=>c[2]),['Slicing not produced','Slicing not produced']);
  assert.equal((await calls('produceMacroSlicing')).length,3);

  assert.deepEqual(errors,[]);
  console.log('roadmap-slicing-upload.browser: ok');
} finally {
  await browser?.close();
  await server.close();
}
