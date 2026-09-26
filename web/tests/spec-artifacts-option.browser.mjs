// Isolated browser regression: the real ProjectModal, mocked useApp.
// Run with Playwright available: node tests/spec-artifacts-option.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation (its ESM entry point).
//
// What it guards (#487): the "Keep specifications out of the repository"
// option reads the project's specArtifacts setting, is unchecked by default,
// says that committed specifications stay in the history, and saves keep or
// drop explicitly.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

const modalHarness = `import React from 'react'; import {createRoot} from 'react-dom/client'; import {ProjectModal} from '/src/components/ProjectModal.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
const noop=()=>{};window.saved=null;
window.project={id:'p1',name:'Demo',slug:'demo',color:'indigo',icon:'Folder',issueTracker:'local',isDefault:true,githubRepo:'',description:'',repoPath:'',enabledViews:[]};
window.ctx={isProjectModalOpen:true,setIsProjectModalOpen:noop,get editingProject(){return window.project},setEditingProject:noop,createProject:async p=>{window.saved=p},updateProject:async(id,p)=>{window.saved=p},deleteProject:async()=>{},fetchProjectIssueTypes:async()=>[],setIsTrackerSetupOpen:noop,userCredentials:[],refreshUserCredentials:async()=>{},settings:{userName:'Alice',density:'standard',theme:'dark',aiProvider:'claude'},t:translations.en};
const app=createRoot(document.getElementById('root'));
window.open=(project)=>{window.project=project;app.render(<ProjectModal key={project.specArtifacts||'none'}/>)};
window.open(window.project);`;

const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{
    name: 'fixture', enforce: 'pre',
    transform(code, id) {
      // Only the data source is simulated: the component under test is the shipped one.
      if (id.endsWith('/ProjectModal.tsx')) {
        return code.replace("import { useApp } from '../context/AppContext'", 'const useApp = () => window.ctx');
      }
      if (id.endsWith('/useCurrentUser.ts')) {
        return 'export const useCurrentUser = () => ({ user: { id: "u1", displayName: "Alice", email: "a@b.c", role: "admin" } })';
      }
    },
    configureServer(s) {
      s.middlewares.use(async (req, res, next) => {
        if (req.url !== '/modal') return next();
        res.setHeader('Content-Type', 'text/html');
        res.end(await s.transformIndexHtml(req.url, '<div id="root"></div><script type="module" src="/modal.tsx"></script>'));
      });
    },
    resolveId(id) { if (id === '/modal.tsx') return root + id; },
    load(id) { if (id === root + '/modal.tsx') return modalHarness; },
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

  await page.goto(`${base}/modal`);
  await page.getByRole('button', { name: 'Agentic workflow' }).click();
  const option = page.getByRole('checkbox', { name: /Keep specifications out of the repository/ });
  await option.waitFor();

  // An existing project without the field keeps its artefacts.
  assert.equal(await option.isChecked(), false, 'the option must be unchecked by default');
  assert.ok(await page.getByText('Those already committed stay in the history.').isVisible(), 'the help text must say committed specifications stay');

  const save = async () => {
    await page.evaluate(() => document.getElementById('project-modal-form').requestSubmit());
    await page.waitForTimeout(300);
    return page.evaluate(() => window.saved.specArtifacts);
  };
  assert.equal(await save(), 'keep', 'an unchecked option saves keep explicitly');

  await option.check();
  assert.equal(await save(), 'drop', 'a checked option saves drop');

  // A project that drops its artefacts opens with the option checked.
  await page.evaluate(() => window.open({ ...window.project, specArtifacts: 'drop' }));
  await page.getByRole('button', { name: 'Agentic workflow' }).click();
  await option.waitFor();
  assert.equal(await option.isChecked(), true, 'a drop project must show the option checked');
  await option.uncheck();
  assert.equal(await save(), 'keep', 'unchecking saves keep');

  assert.deepEqual(errors, [], 'the page reported errors');
  console.log('spec-artifacts-option: OK');
} finally {
  if (browser) await browser.close();
  await server.close();
}
