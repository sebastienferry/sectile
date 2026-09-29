// Isolated browser regression: the real RoadmapView, mocked useApp.
// Run with Playwright available: node tests/roadmap-epic-labels.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
// It must name the ESM entry point, not the package directory, e.g.
//   PLAYWRIGHT_MODULE=/absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/roadmap-epic-labels.browser.mjs
//
// What it guards (#626): the row shows an epic's free labels and never the
// labels of the roadmap's axes; the toolbar filter narrows the list to the
// epics carrying a picked label; the panel edits the free labels, refuses an
// axis label or a label with a space before anything is sent, and shows the
// labels read-only on an epic Sectile cannot label.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

const harness = `import React from 'react'; import {createRoot} from 'react-dom/client'; import {RoadmapView} from '/src/components/RoadmapView.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
const noop=()=>{};
window.edits=[];window.toasts=[];
const meta=(key,title,labels,extra)=>({projectId:'p1',key,title,horizon:'now',description:'',todos:[],updatedAt:'',labelsWritable:true,labels,...extra});
window.metas=[
  meta('PE-1','Billing',['domain-billing','client-acme','roadmap:now','priority:p1','2026-Q3']),
  meta('PE-2','Grafana decision',['client-acme'],{labelsWritable:false}),
  meta('PE-3','Okta rollout',[]),
];
const project={id:'p1',name:'Demo',slug:'demo',color:'indigo',icon:'Folder',issueTracker:'jira',jiraProject:'PE',isDefault:true,githubRepo:'',description:'',repoPath:'',enabledViews:['roadmap'],sprints:[]};
window.ctx={
  tasks:[],
  projects:[project],currentProject:project,settings:{userName:'Alice',language:'fr',density:'standard',theme:'dark'},
  t:translations.fr,activeJobCount:0,searchQuery:'',
  fetchProjectMacros:async()=>window.metas,
  pendingHorizonPushes:async()=>[],
  saveMacroMeta:async()=>null,
  editMacroLabels:async(pid,key,patch)=>{window.edits.push({key,patch});return true},
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
  await page.getByText('Grafana decision').first().waitFor();

  const rowOf = key => page.locator('div.rounded-xl.border').filter({ has: page.locator('span', { hasText: new RegExp(`^${key}$`) }) }).first();
  const lastEdit = () => page.evaluate(() => window.edits[window.edits.length - 1] || null);
  const editCount = () => page.evaluate(() => window.edits.length);
  const lastToast = () => page.evaluate(() => window.toasts[window.toasts.length - 1] || null);
  const visibleKeys = () => page.evaluate(() => {
    const seen = [];
    for (const row of document.querySelectorAll('div.rounded-xl.border')) {
      const key = [...row.querySelectorAll('span')].map(s => s.textContent.trim()).find(t => /^PE-\d$/.test(t));
      if (key && !seen.includes(key)) seen.push(key);
    }
    return seen;
  });

  // US2: free labels as badges, axis labels left out.
  const billing = rowOf('PE-1');
  assert.ok(await billing.getByText('domain-billing', { exact: true }).isVisible(), 'PE-1 shows domain-billing');
  assert.ok(await billing.getByText('client-acme', { exact: true }).isVisible(), 'PE-1 shows client-acme');
  for (const axis of ['roadmap:now', 'priority:p1', '2026-Q3']) {
    assert.equal(await billing.getByText(axis, { exact: true }).count(), 0, `PE-1 must not show ${axis} as a label`);
  }
  const bare = await rowOf('PE-3').textContent();
  assert.ok(!/domain-billing|client-acme|roadmap:|priority:/.test(bare), 'PE-3 shows no label');

  // US3: the filter narrows the list, the chip clears it.
  await page.getByRole('button', { name: 'Labels', exact: true }).click();
  await page.getByRole('menuitemcheckbox', { name: /domain-billing/ }).click();
  await page.waitForFunction(() => !document.body.textContent.includes('Grafana decision'));
  assert.deepEqual(await visibleKeys(), ['PE-1']);
  await page.getByRole('menuitemcheckbox', { name: /client-acme/ }).click();
  await page.waitForFunction(() => document.body.textContent.includes('Grafana decision'));
  assert.deepEqual((await visibleKeys()).sort(), ['PE-1', 'PE-2'], 'OR: either label keeps an epic');
  await page.keyboard.press('Escape');
  await page.getByRole('button', { name: 'label domain-billing', exact: true }).click();
  await page.getByRole('button', { name: 'label client-acme', exact: true }).click();
  await page.getByText('Okta rollout').first().waitFor();

  // US4: edit PE-1's free labels from the panel.
  await billing.click();
  const input = page.getByPlaceholder('Ajouter un label…');
  await input.fill('roadmap:later');
  await input.press('Enter');
  assert.match((await lastToast()).description, /appartient à un axe de la roadmap/);
  await input.fill('2026-Q4');
  await input.press('Enter');
  assert.match((await lastToast()).description, /appartient à un axe de la roadmap/);
  await input.fill('client acme');
  await input.press('Enter');
  assert.match((await lastToast()).description, /espace/);
  await input.fill('Client-Acme');
  await input.press('Enter');
  assert.match((await lastToast()).description, /porte déjà/);
  assert.equal(await editCount(), 0, 'a refused label must not be sent');

  await input.fill('team-x');
  await input.press('Enter');
  assert.deepEqual(await lastEdit(), { key: 'PE-1', patch: { add: ['team-x'] } });
  await page.getByRole('button', { name: 'Retirer le label client-acme' }).click();
  assert.deepEqual(await lastEdit(), { key: 'PE-1', patch: { remove: ['client-acme'] } });
  assert.equal(await page.getByRole('button', { name: 'Retirer le label roadmap:now' }).count(), 0, 'an axis label has no chip');

  // US4.10: an epic Sectile cannot label shows its labels read-only.
  await rowOf('PE-2').click();
  await page.getByText("Cette macro n'est pas un épic de ce projet").waitFor();
  assert.equal(await page.getByPlaceholder('Ajouter un label…').count(), 0, 'no input on a read-only epic');
  assert.equal(await page.getByRole('button', { name: 'Retirer le label client-acme' }).count(), 0, 'no remove control on a read-only epic');

  assert.deepEqual(errors, []);
  console.log('roadmap-epic-labels: ok');
} finally {
  await browser?.close();
  await server.close();
}
