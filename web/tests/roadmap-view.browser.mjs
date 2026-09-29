// Isolated browser regression (#629): the roadmap reopens as it was left, hides its panel, gives an expanded panel
// the whole view, copies and opens the macro's own page, and writes the framing in a full-screen editor.
// Real RoadmapView, its children and styles; useApp is mocked in every component.
// Run with Playwright available: node tests/roadmap-view.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);
const harness=`import React from 'react'; import {createRoot} from 'react-dom/client'; import {RoadmapView} from '/src/components/RoadmapView.tsx'; import {MarkdownEditor} from '/src/components/Markdown.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.calls=[];
const project={id:'p',name:'Sectile',issueTracker:'github',sprints:[]};
const meta=(key,title,horizon,extra={})=>({projectId:'p',key,title,horizon,description:'',framingComment:'',todos:[],updatedAt:'',status:'open',closed:false,...extra});
window.macros=[meta('M-1','Macro un','now'),meta('M-2','Macro deux','now'),meta('M-3','Macro trois','later',{externalUrl:'https://github.com/acme/app/milestone/3'}),meta('M-4','Macro quatre','later')];
const data={tasks:[],projects:[project],currentProject:project,settings:{language:'fr'},t:translations.fr,assigneeFilter:null,myTasksOnly:false,sprintFilter:null,teamFilter:null,labelFilter:null,pinnedOnly:false,searchQuery:'',activeJobCount:0,activities:[],
  fetchProjectMacros:async()=>window.macros.map(m=>({...m})),pendingHorizonPushes:async()=>[],addToast:toast=>window.calls.push(['toast',toast.title,toast.description])};
window.ctx=new Proxy(data,{get:(target,key)=>key in target?target[key]:(...args)=>{window.calls.push([String(key),...args]);return Promise.resolve([])}});
window.clip=[]; window.refuseClipboard=false;
Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{if(window.refuseClipboard) throw new Error('denied'); window.clip.push(text)}}});
// What a dialog behind the full-screen editor would hear: useEscapeKey listens on the document in the capture phase.
window.escapesBehind=0; document.addEventListener('keydown',e=>{if(e.key==='Escape') window.escapesBehind++},true);
const app=createRoot(document.getElementById('root'));
window.mounted=true;
window.render=()=>app.render(window.mounted?<div style={{height:'700px',display:'flex'}}><RoadmapView/></div>:null);
const plain=createRoot(document.getElementById('plain'));
window.plainValue='';
window.renderPlain=()=>plain.render(<MarkdownEditor value={window.plainValue} onChange={v=>{window.plainValue=v;window.renderPlain()}}/>);
window.render();window.renderPlain();`;
const server=await createServer({root,resolve:{preserveSymlinks},configFile:root+'/vite.config.ts',server:{port:0,host:'127.0.0.1'},plugins:[{name:'fixture',enforce:'pre',transform(code,id){if(id.includes('/src/components/')) return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/,'const useApp = () => window.ctx');},configureServer(s){s.middlewares.use(async(req,res,next)=>{if(req.url==='/fixture'){res.setHeader('Content-Type','text/html');res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><div id="plain" data-testid="plain"></div><script type="module" src="/fixture.tsx"></script>'))}else next()})},resolveId(id){if(id==='/fixture.tsx')return root+'/fixture.tsx'},load(id){if(id===root+'/fixture.tsx')return harness}}]});
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

  // A tab reads its label and its count ("LATER 2"); the classify buttons of the rows read the label alone.
  const tabButton=name=>page.getByRole('button',{name:new RegExp('^'+name+' \\d+$')});
  const activeTab=()=>page.evaluate(()=>[...document.querySelectorAll('button')].find(b=>/^(NOW|NEXT|LATER)/.test(b.textContent||'')&&getComputedStyle(b).fontWeight==='700')?.textContent?.replace(/\d+$/,'').trim());
  const panel=()=>page.locator('aside');
  const panelKey=()=>panel().locator('span.font-mono').first().textContent();
  const row=title=>page.locator('div.cursor-pointer').filter({hasText:title}).first();
  const remount=async()=>{await page.evaluate(()=>{mounted=false;render()});await panel().waitFor({state:'detached'});await page.evaluate(()=>{mounted=true;render()});await tabButton('NOW').waitFor()};
  const calls=name=>page.evaluate(n=>calls.filter(c=>c[0]===n),name);
  const absent=async(locator,message)=>{await locator.first().waitFor({state:'detached'});assert.equal(await locator.count(),0,message)};

  // US1: tab, selection and folded sections survive leaving the view.
  await panel().waitFor();
  assert.equal(await activeTab(),'NOW');
  assert.equal(await panelKey(),'M-1','the first macro is shown by default');
  await tabButton('LATER').click();
  await row('Macro quatre').click();
  await page.waitForFunction(()=>document.querySelector('aside span.font-mono')?.textContent==='M-4');
  await remount();
  await panel().waitFor();
  assert.equal(await activeTab(),'LATER','the tab is remembered');
  assert.equal(await panelKey(),'M-4','the selected macro is remembered');
  assert.equal(await page.evaluate(()=>localStorage.getItem('sectile_roadmap_selected_key:p')),'M-4');

  const descriptionHeading=await page.evaluate(()=>window.ctx.t.planning.roadmap.framing.descriptionHeading);
  const descriptionToggle=()=>panel().getByText(descriptionHeading,{exact:true}).first();
  const maximizeButtons=()=>panel().getByRole('button',{name:"Agrandir l'éditeur"});
  assert.equal(await maximizeButtons().count(),2,'both framing editors offer the full-screen editor');
  await descriptionToggle().click();
  await page.waitForFunction(()=>localStorage.getItem('sectile_roadmap_description_open')==='0');
  await remount();
  await panel().waitFor();
  assert.equal(await maximizeButtons().count(),1,'the folded Description stays folded, the Framing notes stay open');
  await descriptionToggle().click();
  await page.waitForFunction(()=>localStorage.getItem('sectile_roadmap_description_open')==='1');

  // A remembered macro that is not on the tab falls back to the first one, and is not erased.
  await tabButton('NOW').click();
  await panel().waitFor();
  assert.equal(await panelKey(),'M-1');
  assert.equal(await page.evaluate(()=>localStorage.getItem('sectile_roadmap_selected_key:p')),'M-4','the fallback does not erase the choice');

  // US5: copy and open the macro's own page, or its reference when it has none.
  assert.equal(await panel().getByRole('link').count(),0,'a macro without a page has no tracker link');
  await panel().getByRole('button',{name:'Copier',exact:true}).click();
  await page.waitForFunction(()=>clip.length===1);
  assert.deepEqual(await page.evaluate(()=>clip),['M-1: Macro un']);
  assert.deepEqual((await calls('toast')).map(c=>c.slice(1)),[['Référence copiée','M-1: Macro un']]);
  await tabButton('LATER').click();
  await row('Macro trois').click();
  await page.waitForFunction(()=>document.querySelector('aside span.font-mono')?.textContent==='M-3');
  assert.equal(await panel().getByRole('link').first().getAttribute('href'),'https://github.com/acme/app/milestone/3','the tracker link opens the macro itself');
  await page.evaluate(()=>{clip=[];calls=[]});
  await panel().getByRole('button',{name:'Copier',exact:true}).click();
  await page.waitForFunction(()=>clip.length===1);
  assert.deepEqual(await page.evaluate(()=>clip),['https://github.com/acme/app/milestone/3']);
  assert.deepEqual((await calls('toast')).map(c=>c[1]),['Lien copié']);
  await page.evaluate(()=>{refuseClipboard=true;calls=[]});
  await panel().getByRole('button',{name:'Copier',exact:true}).click();
  await page.waitForFunction(()=>calls.some(c=>c[0]==='toast'));
  const [refused]=await calls('toast');
  assert.equal(refused[1],'Copie impossible');
  assert.ok(refused[2].includes('https://github.com/acme/app/milestone/3'),'the refusal gives the text to copy by hand');
  await page.evaluate(()=>{refuseClipboard=false});

  // US4: the full-screen editor edits the same draft, shows its preview, and Escape closes it alone.
  await maximizeButtons().first().click();
  const dialog=page.getByRole('dialog',{name:descriptionHeading});
  await dialog.waitFor();
  await dialog.locator('textarea').fill('## Cadrage plein écran');
  await dialog.getByRole('heading',{name:'Cadrage plein écran'}).waitFor();
  const escapesBefore=await page.evaluate(()=>escapesBehind);
  await page.keyboard.press('Escape');
  await dialog.waitFor({state:'detached'});
  assert.equal(await page.evaluate(()=>escapesBehind),escapesBefore,'Escape closes the full-screen editor and reaches nothing behind it');
  assert.equal(await panel().locator('textarea').first().inputValue(),'## Cadrage plein écran','the section shows what was typed');
  // The focus comes back on the next frame, once the overlay is gone.
  await page.waitForFunction(()=>document.activeElement?.getAttribute('aria-label')==="Agrandir l'éditeur");
  await panel().getByRole('button',{name:/Enregistrer/}).first().waitFor();
  assert.equal((await calls('saveMacroMeta')).length,0,'nothing is saved until Save is clicked');
  await maximizeButtons().first().click();
  await dialog.waitFor();
  await dialog.getByRole('button',{name:'Réduire'}).click();
  await dialog.waitFor({state:'detached'});
  assert.equal(await page.getByTestId('plain').getByRole('button',{name:"Agrandir l'éditeur"}).count(),0,'an editor that does not ask for it offers no full-screen editor');

  // US3: an expanded panel takes the whole view.
  await panel().getByRole('button',{name:'Plein écran'}).click();
  await absent(tabButton('NOW'),'the horizon tabs leave with the list');
  await absent(page.getByRole('button',{name:/^Condensé|^Condensed/}),'the toolbar leaves with the list');
  await remount();
  await panel().waitFor();
  assert.equal(await tabButton('NOW').count(),0,'the expanded state is remembered');
  await panel().getByRole('button',{name:'Réduire',exact:true}).click();
  await tabButton('NOW').waitFor();
  assert.equal(await activeTab(),'LATER','collapsing brings the toolbar back as it was');

  // US2: hiding the panel gives the list the width, and survives selection and remount.
  await panel().getByRole('button',{name:'Plein écran'}).click();
  await panel().getByRole('button',{name:'Masquer le panneau'}).click();
  await panel().waitFor({state:'detached'});
  await tabButton('LATER').waitFor();
  assert.equal(await page.evaluate(()=>localStorage.getItem('sectile_roadmap_panel_expanded')),'0','hiding clears the expanded state');
  const rail=()=>page.getByRole('button',{name:'Afficher le panneau'});
  await rail().waitFor();
  assert.equal(await page.getByRole('separator').count(),0,'the split handle leaves with the panel');
  await row('Macro quatre').click();
  assert.equal(await panel().count(),0,'selecting another macro does not bring the panel back');
  await remount();
  await rail().waitFor();
  assert.equal(await panel().count(),0,'the hidden panel is remembered');
  await rail().click();
  await panel().waitFor();
  assert.equal(await panelKey(),'M-4','the panel comes back on the selected macro');
  await absent(rail(),'the rail leaves once the panel is back');
  await panel().getByRole('button',{name:'Masquer le panneau'}).click();
  await rail().click();
  await panel().getByRole('button',{name:'Plein écran'}).waitFor();
  assert.equal(await page.evaluate(()=>localStorage.getItem('sectile_roadmap_panel_hidden')),'0');

  assert.deepEqual(errors,[]);
  console.log('roadmap-view: ok');
} finally {
  await browser?.close();
  await server.close();
}
