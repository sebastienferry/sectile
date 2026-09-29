// Isolated browser regression (#612): the card's model indicator opens the model list, and the full card copies its
// step's prompt. Real TaskCard, its children and styles; useApp is mocked in every component.
// Run with Playwright available: node tests/card-model-menu.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);
const harness=`import React from 'react'; import {createRoot} from 'react-dom/client'; import {TaskCard} from '/src/components/TaskCard.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.calls=[]; window.condensed=false;
const spy=(name)=>(...args)=>{window.calls.push([name,...args.map(x=>x?.id||x)])};
const project={id:'p'};
window.ctx={settings:{density:'standard'}, projects:[project],currentProject:project,activities:[], t:translations.fr, skillLabel:x=>x,skillCommand:(id,command)=>command,fetchActivities:async()=>{},isPinned:()=>false, parentFilter:null, advanceTask:async(...args)=>{spy('advance')(...args)},addToast:toast=>window.calls.push(['toast',toast.title,toast.description]),...Object.fromEntries(['setSelectedTask','togglePin','setParentFilter','setChatTask','runSkill','setIsTerminalPanelOpen'].map(n=>[n,spy(n)]))};
window.engine={state:'reported',provider:'claude',model:'opus',models:['opus','sonnet','haiku'],modelSlot:true};
window.task={id:'fixture',key:'#612',title:'Card',projectId:'p',status:'to_clarify',priority:'medium',source:'github',labels:['new'],description:'Body'};
window.clip=[]; window.refuseClipboard=false;
Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{if(window.refuseClipboard) throw new Error('denied'); window.clip.push(text)}}});
const app=createRoot(document.getElementById('root'));window.render=()=>{document.body.className='density-'+window.ctx.settings.density;app.render(<div style={{width:300,margin:20}}><TaskCard key={window.condensed?'c':'f'} task={{...window.task}} engine={window.engine} compact={window.condensed}/></div>)};window.render();`;
const server=await createServer({root,resolve:{preserveSymlinks},configFile:root+'/vite.config.ts',server:{port:0,host:'127.0.0.1'},plugins:[{name:'fixture',enforce:'pre',transform(code,id){if(id.includes('/src/components/')) return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/,'const useApp = () => window.ctx');},configureServer(s){s.middlewares.use(async(req,res,next)=>{if(req.url==='/fixture'){res.setHeader('Content-Type','text/html');res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/fixture.tsx"></script>'))}else next()})},resolveId(id){if(id==='/fixture.tsx')return root+'/fixture.tsx'},load(id){if(id===root+'/fixture.tsx')return harness}}]});
await server.listen();
let browser;
try {
  browser=await chromium.launch({headless:true,channel:'chrome'});
  const page=await browser.newPage({viewport:{width:900,height:700}});
  page.setDefaultTimeout(10000);
  const errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`);
  await page.evaluate(()=>localStorage.clear());
  await page.getByRole('button',{name:'Actions',exact:true}).waitFor();

  const indicator=()=>page.getByRole('button',{name:/^Modèle (configuré|retenu)|^Choisir le modèle/});
  const modelMenu=()=>page.getByRole('menu',{name:'Modèle des lancements'});
  const radio=name=>modelMenu().getByRole('menuitemradio',{name});
  const calls=name=>page.evaluate(n=>calls.filter(c=>c[0]===n),name);
  const focusedName=()=>page.evaluate(()=>document.activeElement?.getAttribute('aria-label')||document.activeElement?.textContent);
  const rerender=(fn,arg)=>page.evaluate(fn,arg);
  // count() does not wait for React to render again: wait for the element to go first.
  const absent=async(locator,message)=>{await locator.first().waitFor({state:'detached'});assert.equal(await locator.count(),0,message)};

  // AC1: the indicator opens the model list; choosing launches nothing.
  assert.equal(await indicator().getAttribute('aria-label'),'Modèle configuré : opus');
  assert.equal(await indicator().getAttribute('aria-haspopup'),'menu');
  await indicator().click();
  await modelMenu().waitFor();
  assert.equal(await indicator().getAttribute('aria-expanded'),'true');
  assert.deepEqual(await modelMenu().getByRole('menuitemradio').allTextContents(),['opus (modèle configuré)','sonnet','haiku']);
  assert.equal(await radio('opus (modèle configuré)').getAttribute('aria-checked'),'true');
  assert.equal(await focusedName(),'opus (modèle configuré)','the model in effect takes the focus');
  await page.keyboard.press('ArrowDown');
  assert.equal(await focusedName(),'sonnet');
  await page.keyboard.press('ArrowUp');await page.keyboard.press('ArrowUp');
  assert.equal(await focusedName(),'haiku','ArrowUp wraps to the last model');
  await page.keyboard.press('Home');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  await modelMenu().waitFor({state:'detached'});
  assert.equal(await indicator().getAttribute('aria-label'),'Modèle retenu pour cette tâche : sonnet');
  assert(await indicator().evaluate(e=>e.className.includes('accent-text')));
  assert.equal((await calls('advance')).length,0,'choosing a model launches nothing');
  assert.equal(await page.evaluate(()=>JSON.parse(localStorage.getItem('sectile_launch_models')).fixture),'sonnet');
  assert.equal((await calls('setSelectedTask')).length,0,'the indicator does not open the task');

  // The launch then uses the pick, from the card's own > button as from the menu.
  await page.getByRole('button',{name:/^Avancer d'un pas/}).click();
  await page.waitForFunction(()=>calls.filter(c=>c[0]==='advance').length===1);
  await page.getByRole('button',{name:'Actions',exact:true}).click();
  await page.getByRole('button',{name:'Avancer en interactif'}).click();
  assert.deepEqual((await calls('advance')).map(c=>c.slice(1)),[['fixture',false,undefined,'sonnet'],['fixture',false,'interactive','sonnet']]);

  // AC2: one selection behind both lists.
  await page.getByRole('button',{name:'Actions',exact:true}).click();
  await page.getByRole('button',{name:/^Modèle des lancements/}).click();
  assert.equal(await radio('sonnet').getAttribute('aria-checked'),'true','the sub-list shows the pick made from the indicator');
  await radio('haiku').click();
  assert.equal(await indicator().getAttribute('aria-label'),'Modèle retenu pour cette tâche : haiku');
  await indicator().click();
  assert.equal(await radio('haiku').getAttribute('aria-checked'),'true','the indicator shows the pick made from the sub-list');
  assert.equal(await radio('sonnet').getAttribute('aria-checked'),'false');

  // AC6: Escape gives the focus back to the indicator; one popup at a time.
  await page.keyboard.press('Escape');
  await modelMenu().waitFor({state:'detached'});
  assert.equal(await indicator().evaluate(e=>e===document.activeElement),true);
  await indicator().click();
  await modelMenu().waitFor();
  await page.getByRole('button',{name:'Actions',exact:true}).click();
  await modelMenu().waitFor({state:'detached'});
  await page.keyboard.press('Escape');
  await indicator().click();
  await page.mouse.click(700,650);
  await modelMenu().waitFor({state:'detached'});

  // Choosing the configured entry clears the pick.
  await indicator().click();
  await radio('opus (modèle configuré)').click();
  assert.equal(await indicator().getAttribute('aria-label'),'Modèle configuré : opus');
  assert.equal(await page.evaluate(()=>JSON.parse(localStorage.getItem('sectile_launch_models')).fixture),undefined);
  assert.equal((await calls('setSelectedTask')).length,0);

  // AC3: the icon copies what the menu's step entry copies, and says so.
  const copyIcon=command=>page.getByRole('button',{name:`Copier ${command}`,exact:true});
  const copyFromMenu=async command=>{
    await page.getByRole('button',{name:'Actions',exact:true}).click();
    await page.getByTitle(`Copier ${command} pour cette tâche`).click();
    await page.keyboard.press('Escape');
  };
  for (const [labels,command] of [[['new'],'/clarify-issue'],[['clarified'],'/specify-issue'],[['reviewed'],'/handoff-issue']]) {
    await rerender(l=>{task.labels=l;clip=[];calls=[];render()},labels);
    await copyIcon(command).click();
    await copyFromMenu(command);
    const [fromIcon,fromMenu]=await page.evaluate(()=>clip);
    assert.ok(fromIcon.startsWith(`${command} fixture. Use Sectile MCP`),fromIcon);
    assert.ok(fromIcon.endsWith('Project primary key: p.'));
    assert.equal(fromIcon,fromMenu,`${command}: the icon and the menu copy the same prompt`);
    assert.deepEqual((await calls('toast')).map(c=>c.slice(1)),[['Prompt copié',`${command} pour #612, à coller dans votre assistant`]]);
    assert.equal((await calls('setSelectedTask')).length,0,'the icon does not open the task');
  }
  await rerender(()=>{task.labels=['finished'];render()});
  await absent(page.getByRole('button',{name:/^Copier \//}),'a finished task has no step to copy');

  // AC4: a refused clipboard still shows the prompt.
  await rerender(()=>{task.labels=['clarified'];refuseClipboard=true;clip=[];calls=[];render()});
  await copyIcon('/specify-issue').click();
  const panel=page.getByRole('dialog',{name:/Presse-papiers bloqué/});
  await panel.waitFor();
  assert.ok((await panel.locator('pre').textContent()).startsWith('/specify-issue fixture. Use Sectile MCP'));
  assert.equal((await calls('toast')).length,0);
  await page.keyboard.press('Escape');
  await panel.waitFor({state:'detached'});
  assert.equal(await copyIcon('/specify-issue').evaluate(e=>e===document.activeElement),true,'focus returns to the icon');
  // A scroll closes it too, without pulling the focus back to the card.
  await copyIcon('/specify-issue').click();
  await panel.waitFor();
  await page.evaluate(()=>{document.activeElement.blur();window.dispatchEvent(new Event('scroll'))});
  await panel.waitFor({state:'detached'});
  assert.equal(await copyIcon('/specify-issue').evaluate(e=>e===document.activeElement),false,'a scroll leaves the focus alone');
  await rerender(()=>{refuseClipboard=false;render()});

  // AC5: nothing to pick, nothing clickable.
  await rerender(()=>{engine={state:'unknown'};render()});
  await absent(indicator());
  await page.getByText('?',{exact:true}).waitFor();
  await rerender(()=>{engine={state:'reported',provider:'codex',model:'gpt-5',models:['gpt-5'],modelSlot:false};render()});
  await absent(page.getByText('?',{exact:true}),'the slot-less engine is rendered');
  await absent(indicator(),'a command line without a model slot offers no list');
  await rerender(()=>{engine={state:'reported',provider:'claude',models:['opus','sonnet'],modelSlot:true};render()});
  assert.equal(await indicator().getAttribute('aria-label'),'Choisir le modèle des lancements');
  assert.equal(await indicator().locator('svg').count(),1,'a chip stands in for the unknown model');
  await indicator().click();
  assert.deepEqual(await modelMenu().getByRole('menuitemradio').allTextContents(),['(modèle configuré)','opus','sonnet']);
  await page.keyboard.press('Escape');

  // Condensed card: the same menu from its indicator, and no copy icon.
  await rerender(()=>{condensed=true;ctx.settings.density='compact';engine={state:'reported',provider:'claude',model:'opus',models:['opus','sonnet'],modelSlot:true};calls=[];render()});
  await indicator().click();
  await radio('sonnet').click();
  assert.equal(await indicator().getAttribute('aria-label'),'Modèle retenu pour cette tâche : sonnet');
  assert.equal((await calls('advance')).length,0);
  assert.equal((await calls('setSelectedTask')).length,0);
  await absent(page.getByRole('button',{name:/^Copier \//}),'the condensed card has no copy icon');

  assert.deepEqual(errors,[]);
  console.log('PASS: indicator menu, keyboard and focus, pick without launch, shared selection, single popup, configured reset, copy icon prompt and toast, finished task, refused clipboard and its scroll close, passive indicators, chip placeholder, condensed card.');
} finally {
  await browser?.close();
  await server.close();
}
