// Isolated browser regression (#663): a macro's todos are reworded in place and reordered by keyboard, buttons or
// drag; the status line under them says where the list is copied on the tracker and offers to publish it again.
// Real RoadmapView, its children and styles; useApp is mocked in every component.
// Run with Playwright available: node tests/roadmap-todos.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);
const harness=`import React from 'react'; import {createRoot} from 'react-dom/client'; import {RoadmapView} from '/src/components/RoadmapView.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.calls=[];
const project={id:'p',name:'Platform',issueTracker:'jira',jiraProject:'PE',sprints:[]};
const meta=(key,title,todos,todosMirror)=>({projectId:'p',key,title,horizon:'now',description:'',framingComment:'',todos,updatedAt:'',status:'open',closed:false,todosMirror});
window.macros=[
  meta('PE-1','Epic one',[
    {id:'a',text:'Ligne A',done:false},
    {id:'b',text:'Ligne B',done:true,storyKey:'PE-12'},
    {id:'c',text:'Ligne C',done:false},
  ],{kind:'jira_comment',upToDate:false}),
  meta('PE-2','Epic two',[{id:'z',text:'Seule',done:false}],{kind:'jira_comment',upToDate:false,error:'tracker returned HTTP 401',credentialMissing:'jira'}),
  meta('M-3','Local epic',[{id:'y',text:'Locale',done:false}],{kind:'',reason:'macro locale, sans épic Jira',upToDate:false}),
  meta('PE-4','Epic four',[{id:'x',text:'Copiée',done:false}],{kind:'jira_comment',upToDate:true,url:'https://site/browse/PE-4?focusedCommentId=9'}),
];
const data={tasks:[],projects:[project],currentProject:project,settings:{language:'fr'},t:translations.fr,assigneeFilter:null,myTasksOnly:false,sprintFilter:null,teamFilter:null,labelFilter:null,pinnedOnly:false,searchQuery:'',activeJobCount:0,activities:[],
  fetchProjectMacros:async()=>window.macros.map(m=>({...m})),pendingHorizonPushes:async()=>[],addToast:toast=>window.calls.push(['toast',toast.title,toast.description]),
  // A save answers the macro as stored, its copy now waiting to be published.
  saveMacroMeta:async(pid,key,patch)=>{
    window.calls.push(['saveMacroMeta',key,JSON.parse(JSON.stringify(patch))]);
    const current=window.macros.find(m=>m.key===key);
    const saved={...current,...patch,todosMirror:{...current.todosMirror,upToDate:false}};
    window.macros=window.macros.map(m=>m.key===key?saved:m);
    return saved;
  },
  republishMacroTodos:async(pid,key)=>{window.calls.push(['republishMacroTodos',pid,key]);return true},
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
  const saves=()=>calls('saveMacroMeta');
  const texts=()=>panel().locator('[data-testid="macro-todo"] [data-todo-control="text"]').allTextContents();
  const textButton=text=>panel().getByRole('button',{name:`Reformuler « ${text} »`});
  const editor=text=>panel().getByRole('textbox',{name:`Reformuler « ${text} »`});
  const status=()=>panel().locator('[data-testid="todos-mirror"]');

  await row('Epic one').click();
  await page.getByRole('button',{name:'Framing',exact:true}).click();
  await panel().getByText('Ligne A').waitFor();
  assert.deepEqual(await texts(),['Ligne A','Ligne B','Ligne C']);

  // US1.3: Escape leaves the wording as it was and saves nothing.
  await textButton('Ligne A').click();
  await editor('Ligne A').fill('Ligne A changée');
  await editor('Ligne A').press('Escape');
  await textButton('Ligne A').waitFor();
  assert.equal((await saves()).length,0,'Escape saves nothing');

  // US1.4: an emptied line comes back as it was; it is not deleted.
  await textButton('Ligne A').click();
  await editor('Ligne A').fill('   ');
  await editor('Ligne A').press('Enter');
  await textButton('Ligne A').waitFor();
  assert.equal((await saves()).length,0,'a blank wording saves nothing');
  assert.deepEqual(await texts(),['Ligne A','Ligne B','Ligne C']);

  // US1.1, US1.2, US1.5: Enter saves the trimmed wording, and only it; the attached line keeps its key.
  await textButton('Ligne B').click();
  assert.equal(await editor('Ligne B').inputValue(),'Ligne B');
  assert.equal(await editor('Ligne B').evaluate(el=>el.selectionStart),'Ligne B'.length,'the cursor is at the end');
  await editor('Ligne B').fill('  Ligne B reformulée ');
  await editor('Ligne B').press('Enter');
  await textButton('Ligne B reformulée').waitFor();
  let [save]=await saves();
  assert.deepEqual(save[2].todos,[
    {id:'a',text:'Ligne A',done:false},
    {id:'b',text:'Ligne B reformulée',done:true,storyKey:'PE-12'},
    {id:'c',text:'Ligne C',done:false},
  ]);
  assert.equal(await page.evaluate(()=>document.activeElement?.dataset?.todoId),'b','the reworded line keeps the focus');

  // US2.3: the ends of the list cannot move further.
  assert.ok(await panel().getByRole('button',{name:'Monter « Ligne A »'}).isDisabled());
  assert.ok(await panel().getByRole('button',{name:'Descendre « Ligne C »'}).isDisabled());
  assert.ok(!(await panel().getByRole('button',{name:'Descendre « Ligne A »'}).isDisabled()));

  // US2.2: Alt+Down moves the focused line, which keeps the focus.
  await textButton('Ligne A').focus();
  await page.keyboard.press('Alt+ArrowDown');
  await page.waitForFunction(()=>window.calls.filter(c=>c[0]==='saveMacroMeta').length===2);
  await page.waitForFunction(()=>document.activeElement?.dataset?.todoId==='a');
  assert.deepEqual(await texts(),['Ligne B reformulée','Ligne A','Ligne C']);
  // US2.6: a move changes nothing else on any line.
  save=(await saves())[1];
  assert.deepEqual(save[2].todos.map(t=>t.id),['b','a','c']);
  assert.equal(save[2].todos[0].storyKey,'PE-12');

  // US2.2: the buttons move too, and the button keeps the focus.
  await panel().getByRole('button',{name:'Descendre « Ligne A »'}).click();
  await page.waitForFunction(()=>window.calls.filter(c=>c[0]==='saveMacroMeta').length===3);
  assert.deepEqual(await texts(),['Ligne B reformulée','Ligne C','Ligne A']);
  // Ligne A is now last: its Descendre is disabled, so the focus falls back on its text.
  await page.waitForFunction(()=>document.activeElement?.dataset?.todoId==='a');

  // US2.1: a drag by the handle drops the line at the place of another.
  const handles=panel().locator('[data-testid="macro-todo"] span[draggable="true"]');
  assert.equal(await handles.count(),3);
  await handles.nth(2).dragTo(panel().locator('[data-testid="macro-todo"]').nth(0));
  await page.waitForFunction(()=>window.calls.filter(c=>c[0]==='saveMacroMeta').length===4);
  assert.deepEqual(await texts(),['Ligne A','Ligne B reformulée','Ligne C']);

  // US5.1, US5.5: the copy is waiting; republishing queues it at once.
  assert.equal(await status().getAttribute('data-state'),'pending');
  await status().getByText('Publication en attente').waitFor();
  await status().getByRole('button',{name:'Republier'}).click();
  await page.waitForFunction(()=>window.calls.some(c=>c[0]==='republishMacroTodos'));
  assert.deepEqual((await calls('republishMacroTodos'))[0],['republishMacroTodos','p','PE-1']);

  // US2.3: a single todo offers no handle and no move control; US5.4: a failure names its reason and offers the token.
  await row('Epic two').click();
  await panel().getByText('Seule',{exact:true}).waitFor();
  assert.equal(await panel().locator('[data-testid="macro-todo"] span[draggable]').count(),0);
  assert.equal(await panel().getByRole('button',{name:/^Monter|^Descendre/}).count(),0);
  assert.equal(await status().getAttribute('data-state'),'failed');
  await status().getByText('Échec de publication : tracker returned HTTP 401').waitFor();
  await status().getByRole('button',{name:'Ajouter mon jeton Jira'}).click();
  assert.deepEqual((await calls('openTrackerCredentials'))[0],['openTrackerCredentials','jira']);
  assert.equal(await status().getByRole('button',{name:'Republier'}).count(),1);

  // AC4: a local-only macro says why, and offers nothing to publish.
  await row('Local epic').click();
  await panel().getByText('Locale',{exact:true}).waitFor();
  await status().getByText('Reste dans Sectile : macro locale, sans épic Jira').waitFor();
  assert.equal(await status().getByRole('button').count(),0);

  // US5.1: an up-to-date copy links the comment.
  await row('Epic four').click();
  await panel().getByText('Copiée',{exact:true}).waitFor();
  await status().getByText('Recopiée sur PE-4').waitFor();
  assert.equal(await status().locator('a').getAttribute('href'),'https://site/browse/PE-4?focusedCommentId=9');
  assert.equal(await status().getByRole('button').count(),0);

  assert.deepEqual(errors,[]);
  console.log('roadmap-todos.browser: ok');
} finally {
  await browser?.close();
  await server.close();
}
