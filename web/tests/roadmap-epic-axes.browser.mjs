// Isolated browser regression: the real RoadmapView, mocked useApp.
// Run with Playwright available: node tests/roadmap-epic-axes.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
// It must name the ESM entry point, not the package directory, e.g.
//   PLAYWRIGHT_MODULE=/absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/roadmap-epic-axes.browser.mjs
//
// What it guards (#627): the row shows the epic's own priority and never its
// children's; the panel sets the priority and validates the quarter before
// sending it; a macro whose epic carries no labels says so; the priority
// filter and sort reorder the tab; the seeding writes only the ticked values.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

const harness = `import React from 'react'; import {createRoot} from 'react-dom/client'; import {RoadmapView} from '/src/components/RoadmapView.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
const noop=()=>{};
window.saves=[];window.toasts=[];
const meta=(key,title,extra)=>({projectId:'p1',key,title,horizon:'now',description:'',todos:[],updatedAt:'',labelsWritable:true,...extra});
window.metas=[
  meta('PE-1','2026.Q4 [P2] - Platform AI'),
  meta('PE-2','Grafana decision',{priority:'p0',labelsWritable:false}),
  meta('PE-3','2027 Q1 - [P1] Okta rollout'),
];
const project={id:'p1',name:'Demo',slug:'demo',color:'indigo',icon:'Folder',issueTracker:'jira',jiraProject:'PE',isDefault:true,githubRepo:'',description:'',repoPath:'',enabledViews:['roadmap'],sprints:[]};
window.ctx={
  tasks:[{id:'t1',key:'PE-10',title:'Urgent child',status:'new',priority:'urgent',parentKey:'PE-1',projectId:'p1',labels:[]}],
  projects:[project],currentProject:project,settings:{userName:'Alice',language:'fr',density:'standard',theme:'dark'},
  t:translations.fr,activeJobCount:0,searchQuery:'',
  fetchProjectMacros:async()=>window.metas,
  pendingHorizonPushes:async()=>[],
  saveMacroMeta:async(pid,key,patch,options)=>{
    window.saves.push({key,patch,options:options||null});
    const current=window.metas.find(m=>m.key===key);
    const next={...current,...patch};
    window.metas=[...window.metas.filter(m=>m.key!==key),next];
    return next;
  },
  addToast:t=>window.toasts.push(t),
  ...Object.fromEntries(['setSelectedTask','setAssigneeFilter','setMyTasksOnly','setSprintFilter','setTeamFilter','setLabelFilter','setPinnedOnly','setSearchQuery'].map(n=>[n,noop])),
};
const app=createRoot(document.getElementById('root'));
app.render(<div style={{height:'100vh',display:'flex'}}><RoadmapView/></div>);`;

const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{
    name: 'fixture', enforce: 'pre',
    transform(code, id) {
      // Only the data source is simulated: every component that reads the
      // context gets the fixture, the components under test stay the shipped ones.
      if (id.includes('/src/components/') && code.includes("import { useApp } from '../context/AppContext'")) {
        return code.replace("import { useApp } from '../context/AppContext'", 'const useApp = () => window.ctx');
      }
    },
    configureServer(s) {
      s.middlewares.use(async (req, res, next) => {
        if (req.url !== '/roadmap') return next();
        res.setHeader('Content-Type', 'text/html');
        res.end(await s.transformIndexHtml(req.url, `<div id="root"></div><script type="module" src="/roadmap.tsx"></script>`));
      });
    },
    resolveId(id) { if (id === '/roadmap.tsx') return root + id; },
    load(id) { if (id === root + '/roadmap.tsx') return harness; },
  }],
});
await server.listen();
const base = `http://127.0.0.1:${server.httpServer.address().port}`;

let browser;
try {
  browser = await chromium.launch({ headless: true, channel: 'chrome' });
  const page = await browser.newPage({ viewport: { width: 1400, height: 900 } });
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));

  await page.goto(`${base}/roadmap`);
  await page.getByText('2026.Q4 [P2] - Platform AI').first().waitFor();

  const lastSave = () => page.evaluate(() => window.saves[window.saves.length - 1] || null);
  const saveCount = () => page.evaluate(() => window.saves.length);
  // The rows come before the panel in the document: the first occurrence of
  // each key is its row.
  const rowOrder = () => page.evaluate(() => {
    const seen = [];
    for (const span of document.querySelectorAll('span')) {
      const text = span.textContent.trim();
      if (/^PE-\d$/.test(text) && !seen.includes(text)) seen.push(text);
    }
    return seen;
  });
  const rowOf = key => page.locator('div.rounded-xl.border').filter({ has: page.locator('span', { hasText: new RegExp(`^${key}$`) }) }).first();

  // US1: an urgent child does not make the epic urgent.
  assert.ok(await rowOf('PE-1').getByText('Sans priorité').isVisible(), 'PE-1 should show no priority');
  assert.ok(await rowOf('PE-2').getByText('P0', { exact: true }).isVisible(), 'PE-2 should show its own P0');

  // US2: set the priority from the panel.
  await rowOf('PE-1').click();
  const panelPriority = page.getByRole('group', { name: 'Priorité' });
  await panelPriority.getByRole('button', { name: 'P1', exact: true }).click();
  assert.deepEqual(await lastSave(), { key: 'PE-1', patch: { priority: 'p1' }, options: null });
  await rowOf('PE-1').getByText('P1', { exact: true }).waitFor();
  assert.ok(!(await page.getByText('Conservés dans Sectile').isVisible()), 'a writable epic has no local-only line');

  // US3: an invalid quarter is refused inline and nothing is sent.
  const quarter = page.getByLabel('Trimestre');
  const before = await saveCount();
  await quarter.fill('Q4');
  await quarter.press('Enter');
  await page.getByRole('alert').filter({ hasText: "n'est pas un trimestre" }).waitFor();
  assert.equal(await saveCount(), before, 'an invalid quarter must not be sent');
  await quarter.fill('2026.q4');
  await quarter.press('Enter');
  assert.deepEqual(await lastSave(), { key: 'PE-1', patch: { quarter: '2026-Q4' }, options: null });
  await page.waitForFunction(() => document.getElementById('roadmap-quarter')?.value === '2026-Q4');
  assert.equal(await page.getByRole('alert').count(), 0, 'the message goes once the value is saved');

  // US2.3: a macro whose epic carries no labels says so.
  await rowOf('PE-2').click();
  await page.getByText('Conservés dans Sectile').waitFor();
  assert.equal(await quarter.inputValue(), '', 'the quarter field follows the selected epic');

  // US5: filter on "no priority".
  const filter = page.getByRole('combobox', { name: 'Priorité' });
  await filter.selectOption('none');
  await page.waitForFunction(() => !document.body.textContent.includes('Grafana decision'));
  assert.deepEqual(await rowOrder(), ['PE-3']);
  await page.getByText('priorité Sans priorité').waitFor();
  await filter.selectOption('');

  // US6: sort on the priority, epics without one last.
  await page.getByRole('combobox', { name: 'Ordre du backlog' }).selectOption('priority-desc');
  await page.waitForFunction(() => {
    const keys = [];
    for (const s of document.querySelectorAll('span')) {
      const t = s.textContent.trim();
      if (/^PE-\d$/.test(t) && !keys.includes(t)) keys.push(t);
    }
    return keys.join(',') === 'PE-2,PE-1,PE-3';
  });

  // US4: the seeding proposes only what is missing, and writes only the ticked values.
  await page.getByRole('button', { name: 'Amorcer depuis les titres' }).click();
  const dialog = page.getByRole('dialog', { name: 'Amorcer la priorité et le trimestre' });
  await dialog.waitFor();
  assert.equal(await dialog.getByText('PE-1', { exact: true }).count(), 0, 'PE-1 already has both values');
  assert.equal(await dialog.getByText('PE-2', { exact: true }).count(), 0, 'PE-2 has nothing in its title');
  const boxes = dialog.getByRole('checkbox');
  assert.equal(await boxes.count(), 2, 'PE-3 proposes a priority and a quarter');
  assert.ok(await dialog.getByText('2027-Q1').isVisible());
  await boxes.nth(1).uncheck();
  const savesBeforeSeed = await saveCount();
  await dialog.getByRole('button', { name: 'Poser 1 valeur' }).click();
  await dialog.waitFor({ state: 'detached' });
  assert.equal(await saveCount(), savesBeforeSeed + 1);
  assert.deepEqual(await lastSave(), { key: 'PE-3', patch: { priority: 'p1' }, options: { quiet: true } });
  const toast = await page.evaluate(() => window.toasts[window.toasts.length - 1]);
  assert.equal(toast.type, 'success');
  assert.equal(toast.description, '1 macro mise à jour');

  assert.deepEqual(errors, []);
  console.log('roadmap-epic-axes: ok');
} finally {
  await browser?.close();
  await server.close();
}
