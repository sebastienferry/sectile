// Isolated browser regression: real Sidebar and ProjectModal, mocked useApp.
// Run with Playwright available: node tests/optional-views.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
// It must name the ESM entry point, not the package directory, e.g.
//   PLAYWRIGHT_MODULE=../desktop/node_modules/playwright/index.mjs node tests/optional-views.browser.mjs
//
// What it guards: Triage, Roadmap and Timeline are hidden unless the current
// project enabled them, and the settings toggles survive several clicks in one
// render pass (a stale closure there silently dropped every click but the last).
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const root = fileURLToPath(new URL('..', import.meta.url)).replace(/\/$/, '');

const sidebarHarness = `import React from 'react'; import {createRoot} from 'react-dom/client'; import {Sidebar} from '/src/components/Sidebar.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
const noop=()=>{};
window.project={id:'p1',name:'Demo',slug:'demo',color:'indigo',icon:'Folder',issueTracker:'local',isDefault:true,githubRepo:'',description:'',repoPath:'',enabledViews:[]};
window.ctx={projects:[window.project],selectedProjectId:'p1',get currentProject(){return window.project},activities:[],activeJobCount:0,activeView:'board',setActiveView:v=>{window.lastView=v},statusFilter:null,priorityFilter:null,labelFilter:null,assigneeFilter:null,sourceFilter:'all',sidebarCollapsed:false,isSyncing:false,settings:{userName:'Alice',density:'standard',theme:'dark'},teams:[],tasks:[],taskFacets:{statuses:[],sources:[],trackerStatuses:[],total:0},boardGrouping:'status',trackerStatusFilters:[],t:translations.fr,...Object.fromEntries(['setSelectedProjectId','setIsProjectModalOpen','setEditingProject','toggleProjectBookmark','setStatusFilter','setPriorityFilter','setLabelFilter','setAssigneeFilter','setSourceFilter','setSidebarCollapsed','setIsProfileOpen','setIsAdminOpen','setTrackerStatusFilters'].map(n=>[n,noop]))};
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<div style={{height:'100vh',display:'flex'}}><Sidebar/></div>);
window.setViews=(v)=>{window.project={...window.project,enabledViews:v};window.render()};
window.collapse=(on)=>{window.ctx.sidebarCollapsed=on;window.render()};
window.render();`;

const modalHarness = `import React from 'react'; import {createRoot} from 'react-dom/client'; import {ProjectModal} from '/src/components/ProjectModal.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
const noop=()=>{};window.saved=null;
window.project={id:'p1',name:'Demo',slug:'demo',color:'indigo',icon:'Folder',issueTracker:'local',isDefault:true,githubRepo:'',description:'',repoPath:'',enabledViews:['roadmap']};
window.ctx={isProjectModalOpen:true,setIsProjectModalOpen:noop,get editingProject(){return window.project},setEditingProject:noop,createProject:async p=>{window.saved=p},updateProject:async(id,p)=>{window.saved=p},deleteProject:async()=>{},fetchProjectIssueTypes:async()=>[],setIsTrackerSetupOpen:noop,userCredentials:[],refreshUserCredentials:async()=>{},settings:{userName:'Alice',density:'standard',theme:'dark'},t:translations.fr};
const app=createRoot(document.getElementById('root'));app.render(<ProjectModal/>);`;

const server = await createServer({
  root, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{
    name: 'fixture', enforce: 'pre',
    transform(code, id) {
      // Le vrai contexte tirerait le réseau et la session : seule la source des
      // données est simulée, le composant testé reste celui qui est livré.
      if (id.endsWith('/Sidebar.tsx') || id.endsWith('/ProjectModal.tsx')) {
        return code.replace("import { useApp } from '../context/AppContext'", 'const useApp = () => window.ctx');
      }
      if (id.endsWith('/useCurrentUser.ts')) {
        return 'export const useCurrentUser = () => ({ user: { id: "u1", displayName: "Alice", email: "a@b.c", role: "admin" } })';
      }
    },
    configureServer(s) {
      s.middlewares.use(async (req, res, next) => {
        const entry = req.url === '/sidebar' ? '/sidebar.tsx' : req.url === '/modal' ? '/modal.tsx' : null;
        if (!entry) return next();
        res.setHeader('Content-Type', 'text/html');
        res.end(await s.transformIndexHtml(req.url, `<div id="root"></div><script type="module" src="${entry}"></script>`));
      });
    },
    resolveId(id) { if (id === '/sidebar.tsx' || id === '/modal.tsx') return root + id; },
    load(id) {
      if (id === root + '/sidebar.tsx') return sidebarHarness;
      if (id === root + '/modal.tsx') return modalHarness;
    },
  }],
});
await server.listen();
const base = `http://127.0.0.1:${server.httpServer.address().port}`;

let browser;
try {
  browser = await chromium.launch({ headless: true, channel: 'chrome' });
  const page = await browser.newPage({ viewport: { width: 1100, height: 800 } });
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));

  // ---------- Sidebar ----------
  await page.goto(`${base}/sidebar`);
  await page.getByRole('button', { name: 'Board', exact: true }).waitFor();

  const view = name => page.getByRole('button', { name, exact: true });

  // Par défaut, un projet n'affiche aucune des trois vues de planification.
  for (const name of ['Triage', 'Roadmap', 'Timeline']) {
    assert.equal(await view(name).count(), 0, `${name} visible on a project that did not enable it`);
  }

  // Activées, elles apparaissent dans l'ordre de la barre, après Board.
  await page.evaluate(() => window.setViews(['triage', 'roadmap', 'timeline']));
  await view('Triage').waitFor();
  const labels = await page.evaluate(() =>
    Array.from(document.querySelectorAll('button')).map(b => b.textContent.trim()));
  const order = labels.filter(l => ['Board', 'Triage', 'Roadmap', 'Timeline', 'Activités'].includes(l));
  assert.deepEqual(order, ['Board', 'Triage', 'Roadmap', 'Timeline', 'Activités']);

  // Une activation partielle n'en montre qu'une.
  await page.evaluate(() => window.setViews(['roadmap']));
  await view('Roadmap').waitFor();
  assert.equal(await view('Triage').count(), 0);
  assert.equal(await view('Timeline').count(), 0);

  // Le bouton navigue bien vers sa vue.
  await view('Roadmap').click();
  assert.equal(await page.evaluate(() => window.lastView), 'roadmap');

  // Repliée, la barre garde l'icône et une infobulle accessible.
  await page.evaluate(() => { window.setViews(['triage', 'roadmap', 'timeline']); window.collapse(true); });
  await page.waitForTimeout(150);
  const collapsed = await page.evaluate(() =>
    Array.from(document.querySelectorAll('button[title]'))
      .filter(b => /Triage|Roadmap|Timeline/.test(b.getAttribute('title')))
      .map(b => ({ title: b.getAttribute('title'), text: b.textContent.trim(), icon: !!b.querySelector('svg') })));
  assert.equal(collapsed.length, 3);
  for (const entry of collapsed) {
    assert.equal(entry.text, '', `collapsed ${entry.title} still renders its label`);
    assert.ok(entry.icon, `collapsed ${entry.title} lost its icon`);
  }

  // ---------- Réglages du projet ----------
  await page.goto(`${base}/modal`);
  const card = name => page.locator('button[aria-pressed]').filter({ hasText: name });
  await card('Roadmap').waitFor();

  // Le libellé et son explication se touchent dans textContent : la carte est
  // reconnue par le nom qu'elle commence par, pas par un découpage sur l'espace.
  const pressed = async () => page.evaluate(() =>
    Object.fromEntries(Array.from(document.querySelectorAll('button[aria-pressed]'))
      .map(b => [
        ['Triage', 'Roadmap', 'Timeline'].find(n => b.textContent.trim().startsWith(n)),
        b.getAttribute('aria-pressed'),
      ])));

  assert.deepEqual(await pressed(), { Triage: 'false', Roadmap: 'true', Timeline: 'false' });

  // Trois bascules dans le même cycle de rendu : chacune doit compter. Lire
  // l'état d'avant plutôt que le courant les faisait toutes s'annuler.
  await page.evaluate(() => {
    const cards = Array.from(document.querySelectorAll('button[aria-pressed]'));
    cards.forEach(c => c.click());
  });
  await page.waitForTimeout(200);
  assert.deepEqual(await pressed(), { Triage: 'true', Roadmap: 'false', Timeline: 'true' });

  // L'enregistrement envoie la liste normalisée, dans l'ordre canonique.
  await page.evaluate(() => document.getElementById('project-modal-form').requestSubmit());
  await page.waitForTimeout(300);
  assert.deepEqual(await page.evaluate(() => window.saved.enabledViews), ['triage', 'timeline']);

  // Tout éteindre doit produire une liste vide, pas une absence de champ :
  // c'est ce qui distingue « plus aucune vue » de « réglage non touché ».
  await page.evaluate(() => {
    Array.from(document.querySelectorAll('button[aria-pressed]'))
      .filter(c => c.getAttribute('aria-pressed') === 'true').forEach(c => c.click());
  });
  await page.waitForTimeout(200);
  await page.evaluate(() => document.getElementById('project-modal-form').requestSubmit());
  await page.waitForTimeout(300);
  assert.deepEqual(await page.evaluate(() => window.saved.enabledViews), []);

  assert.deepEqual(errors, [], 'the page reported errors');
  console.log('optional-views: OK');
} finally {
  if (browser) await browser.close();
  await server.close();
}
