// Isolated browser regression (#809): the Timeline shows a ticket's macro key
// before its macro title chip, as a link when the project's macro list gives
// the macro an address, with a button that copies the key.
// Real SprintTimelineView and its children; useApp is mocked in every component.
// Run with Playwright available: node tests/sprint-timeline-macro-key.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
//
// What it guards: AC-6 in the sprint cards and the backlog cards (backlog
// open) and in the sprint list rows (backlog closed); a click on the key does
// not open the ticket; AC-7, one macro-list request per project shown, not one
// per ticket, and none again when tickets change within the same projects.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

const harness = `import React from 'react'; import {createRoot} from 'react-dom/client'; import {SprintTimelineView} from '/src/components/SprintTimelineView.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.calls=[];window.macroFetches=[];
const project={id:'p',name:'Demo',issueTracker:'jira',jiraProject:'PE',sprints:[{id:'1',name:'Sprint 1',state:'active',startDate:'2026-10-01',endDate:'2026-10-20'}]};
const task=(id,extra)=>({id,key:id,title:'Ticket '+id,status:'todo',priority:'medium',labels:[],assignee:'',projectId:'p',source:'jira',createdAt:'',updatedAt:'',...extra});
const data={
  tasks:[
    task('PE-11',{sprint:'Sprint 1',parentKey:'M-3',parentTitle:'Macro trois'}),
    task('PE-12',{sprint:'Sprint 1',parentKey:'M-1',parentTitle:'Macro un'}),
    task('PE-13',{parentKey:'M-3',parentTitle:'Macro trois'}),
    task('PE-14',{}),
    task('QA-1',{projectId:'q',parentKey:'M-3',parentTitle:'Autre macro'}),
  ],
  projects:[project],currentProject:project,settings:{language:'fr',userName:'Alice'},t:translations.fr,
  fetchProjectMacros:async id=>{window.macroFetches.push(id);return id==='p'?[{projectId:'p',key:'M-3',title:'Macro trois',externalUrl:'https://acme.atlassian.net/browse/M-3'},{projectId:'p',key:'M-1',title:'Macro un'}]:[{projectId:'q',key:'M-3',title:'Autre macro'}]},
  setSelectedTask:t=>window.calls.push(['setSelectedTask',t&&t.id]),
  addToast:t=>window.calls.push(['toast',t.title]),
};
window.ctx=new Proxy(data,{get:(target,key)=>key in target?target[key]:(...args)=>{window.calls.push([String(key)]);return Promise.resolve([])}});
window.clip=[];
Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.clip.push(text)}}});
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<div style={{height:'900px',display:'flex'}}><SprintTimelineView/></div>);
window.render();`;

const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{
    name: 'fixture', enforce: 'pre',
    transform(code, id) {
      if (id.includes('/src/components/')) return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/, 'const useApp = () => window.ctx');
    },
    configureServer(s) {
      s.middlewares.use(async (req, res, next) => {
        if (req.url !== '/fixture') return next();
        res.setHeader('Content-Type', 'text/html');
        res.end(await s.transformIndexHtml(req.url, `<div id="root"></div><script type="module" src="/fixture.tsx"></script>`));
      });
    },
    resolveId(id) { if (id === '/fixture.tsx') return root + id; },
    load(id) { if (id === root + '/fixture.tsx') return harness; },
  }],
});
await server.listen();

let browser;
try {
  browser = await chromium.launch({ headless: true, channel: 'chrome' });
  const page = await browser.newPage({ viewport: { width: 1500, height: 1000 } });
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`);

  const keys = key => page.locator(`[data-macro-key="${key}"]`);
  const linksOf = key => keys(key).locator('a');
  await page.getByText('Ticket PE-11').first().waitFor();
  await page.waitForFunction(() => document.querySelectorAll('[data-macro-key="M-3"] a').length > 0);

  // AC-7: one request per project with a macro, whatever the ticket count.
  assert.deepEqual((await page.evaluate(() => window.macroFetches)).sort(), ['p', 'q']);

  // Backlog open: PE-11 in its sprint card, PE-13 and QA-1 in the backlog.
  assert.equal(await keys('M-3').count(), 3, 'PE-11, PE-13 and QA-1 show their macro key');
  assert.equal(await linksOf('M-3').count(), 2, "project q's M-3 has no address and stays plain");
  for (const href of await linksOf('M-3').evaluateAll(as => as.map(a => a.getAttribute('href')))) {
    assert.equal(href, 'https://acme.atlassian.net/browse/M-3');
  }
  assert.equal(await keys('M-1').count(), 1);
  assert.equal(await linksOf('M-1').count(), 0, 'a macro without an address is plain text');
  assert.equal(await keys('M-1').getByRole('button', { name: 'Copier M-1' }).count(), 1);
  assert.equal(await page.locator('[data-macro-key]').count(), 4, 'a ticket without a macro shows no key');

  // The key comes before the macro title chip.
  const before = await page.evaluate(() => {
    const ref = document.querySelector('[data-macro-key="M-1"]');
    const chip = [...document.querySelectorAll('span')].find(s => s.textContent === 'Macro un');
    return Boolean(ref && chip && ref.compareDocumentPosition(chip) & Node.DOCUMENT_POSITION_FOLLOWING);
  });
  assert.ok(before, 'the macro key sits before the title chip');

  // A click on the copy button copies the key and does not open the ticket.
  await keys('M-1').getByRole('button', { name: 'Copier M-1' }).click();
  await page.waitForFunction(() => window.clip.length === 1);
  assert.deepEqual(await page.evaluate(() => window.clip), ['M-1']);
  await linksOf('M-3').first().evaluate(a => a.addEventListener('click', e => e.preventDefault()));
  await linksOf('M-3').first().click();
  assert.deepEqual(await page.evaluate(() => window.calls.filter(c => c[0] === 'setSelectedTask')), [], 'no ticket detail opens');

  // Backlog closed: the sprint list row of PE-11 carries the key too.
  await page.getByTitle(/^Masquer le Backlog/).click();
  await page.waitForFunction(() => document.querySelectorAll('[data-macro-key]').length === 2);
  assert.equal(await linksOf('M-3').count(), 1);
  assert.equal(await keys('M-1').count(), 1);
  await keys('M-1').getByRole('button', { name: 'Copier M-1' }).click();
  await page.waitForFunction(() => window.clip.length === 2);
  assert.deepEqual(await page.evaluate(() => window.calls.filter(c => c[0] === 'setSelectedTask')), []);

  // Tickets that change within the same projects fetch nothing again.
  await page.evaluate(() => { window.ctx.tasks = window.ctx.tasks.map(t => ({ ...t, title: t.title + '!' })); window.render(); });
  await page.getByText('Ticket PE-11!').first().waitFor();
  await page.waitForTimeout(200);
  assert.equal((await page.evaluate(() => window.macroFetches)).length, 2);

  assert.deepEqual(errors, []);
  console.log('sprint-timeline-macro-key: ok');
} finally {
  await browser?.close();
  await server.close();
}
