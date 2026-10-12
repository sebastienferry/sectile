// Browser regression for the run's project choice (#741): the real App and
// AppContext, with only the network replaced by an in-page fake of the API.
// Run with Playwright available:
//   PLAYWRIGHT_MODULE=file:///absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/run-project-picker.browser.mjs
//
// Guards project choice before workstation discovery and admission, board project
// defaults, picker dismissal without a launch, busy refusal, and legacy retry
// project choice. Every accepted task request carries the selected deviceId.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

const harness = `
const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const stamp = '2026-10-06T00:00:00Z';
const project = (id, name, label) => ({
  id,
  name,
  slug: id,
  color: 'indigo',
  icon: 'Folder',
  issueTracker: 'jira',
  jiraProject: 'GODE',
  githubRepo: '',
  isDefault: id === 'a',
  description: '',
  enabledViews: [],
  bookmarked: true,
  taskCount: 2,
  label,
  trackers: [{ trackerId: 'gode', identity: 'jira|acme.atlassian.net|GODE' }],
  defaultTrackerId: 'gode',
  createdAt: stamp,
  updatedAt: stamp,
});
const task = (id, key, title, projectIds) => ({
  id,
  key,
  title,
  projectId: projectIds[0],
  projectIds,
  trackerId: 'gode',
  labels: ['#new', ...projectIds.map(p => p === 'a' ? 'alpha' : 'beta')],
  description: '',
  status: 'to_clarify',
  priority: 'medium',
  source: 'jira',
  position: 0,
  createdAt: stamp,
  updatedAt: stamp,
});
window.fake = {
  runs: [],
  retries: [],
  // A failed run recorded before #741, without the project it ran for.
  activities: [{ id: 'old', taskId: 't1', taskKey: 'GODE-1', taskTitle: 'Shared story', skillId: 'clarify', skillName: 'clarify', action: 'run', status: 'failed', summary: '', output: '', steps: [], createdAt: stamp }],
  projects: [project('a', 'Alpha', 'alpha'), project('b', 'Beta', 'beta')],
  tasks: [
    task('t1', 'GODE-1', 'Shared story', ['a', 'b']),
    task('t2', 'GODE-2', 'Busy story', ['a', 'b']),
    task('t3', 'GODE-3', 'Unattended story', ['a', 'b']),
  ],
};
window.fetch = async (input, init = {}) => {
  const url = new URL(typeof input === 'string' ? input : input.url, location.origin);
  const method = (init.method || 'GET').toUpperCase();
  const body = init.body ? JSON.parse(init.body) : null;
  if (url.pathname === '/api/me') return json({ userId: 'usr_me', signedIn: true, identityProvider: false, mode: 'local', role: 'member' });
  if (url.pathname === '/api/projects') return json(fake.projects);
  const run = /^\\/api\\/tasks\\/([^/]+)\\/run-skill$/.exec(url.pathname);
  if (run && method === 'POST') {
    const id = decodeURIComponent(run[1]);
    fake.runs.push({ task: id, ...body });
    if (id === 't2') return json({ error: 'A run of clarify is still active on this task.', active: { id: 'busy' } }, 409);
    // The server's answer to an unattended launch (autonomous, a batch):
    // refused, the candidates listed, never to be retried for one of them.
    if (id === 't3' && !body.projectId) {
      return json({ error: 'lancement automatique refusé', candidates: [{ id: 'a', name: 'Alpha' }, { id: 'b', name: 'Beta' }], unattended: true }, 400);
    }
    if (!body.projectId) {
      return json({ error: 'ce ticket appartient à plusieurs projets', candidates: [{ id: 'a', name: 'Alpha' }, { id: 'b', name: 'Beta' }], unattended: false }, 409);
    }
    const t = fake.tasks.find(item => item.id === id);
    return json({
      task: t,
      activity: {
        id: 'act-' + fake.runs.length,
        taskId: id,
        skillId: body.skillId,
        skillName: body.skillId,
        action: 'run',
        status: 'queued',
        summary: '',
        output: '',
        steps: [],
        createdAt: stamp,
        runProjectId: body.projectId,
      },
    });
  }
  const retry = /^\\/api\\/activities\\/([^/]+)\\/retry$/.exec(url.pathname);
  if (retry && method === 'POST') {
    fake.retries.push({ id: decodeURIComponent(retry[1]), ...(body || {}) });
    if (!body?.projectId) {
      return json({ error: 'ce ticket appartient à plusieurs projets', candidates: [{ id: 'a', name: 'Alpha' }, { id: 'b', name: 'Beta' }], unattended: false }, 409);
    }
    return json({ ...fake.activities[0], id: 'retried', status: 'queued', runProjectId: body.projectId });
  }
  if (url.pathname === '/api/activities') return json(fake.activities);
  if (url.pathname === '/api/tasks/facets') return json({
    sprints: [],
    teams: [],
    macros: [],
    assignees: [],
    trackerStatuses: [],
    statuses: [],
    sources: [],
    issueTypes: [],
    labels: [],
    total: fake.tasks.length,
  });
  if (url.pathname === '/api/tasks') return json(fake.tasks);
  if (url.pathname === '/api/trackers') return json([{ id: 'gode', name: 'GODE', provider: 'jira', site: '', scope: 'GODE', identity: 'jira|acme.atlassian.net|GODE' }]);
  if (url.pathname === '/api/settings') return json({ userName: 'Alice', language: 'fr', aiProvider: 'claude' });
  if (url.pathname === '/api/agent/status') return json({ availableWorkstations: [{ deviceId: 'mac', label: 'Mac' }], agents: [] });
  if (/stats|settings|status/.test(url.pathname)) return json({});
  return json([]);
};
window.EventSource = class { addEventListener() {} close() {} };
window.confirm = () => true;
const React = await import('react');
const { createRoot } = await import('react-dom/client');
const { App } = await import('/src/App.tsx');
await import('/src/index.css');
createRoot(document.getElementById('root')).render(React.createElement(App));
`;

const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{
    name: 'fixture', enforce: 'pre',
    transform(code, id) {
      if (id.endsWith('/useCurrentUser.ts')) {
        return 'export const useCurrentUser = () => ({ user: { userId: "usr_me", displayName: "Alice", email: "a@b.c", role: "member", signedIn: true }, loading: false, error: "", reload: async () => {}, rename: async () => "" })';
      }
    },
    configureServer(s) {
      s.middlewares.use(async (req, res, next) => {
        if (!req.url.startsWith('/board')) return next();
        res.setHeader('Content-Type', 'text/html');
        res.end(await s.transformIndexHtml(req.url, `<div id="root"></div><script type="module" src="/harness.tsx"></script>`));
      });
    },
    resolveId(id) { if (id === '/harness.tsx') return root + id; },
    load(id) { if (id === root + '/harness.tsx') return harness; },
  }],
});
await server.listen();
const base = `http://127.0.0.1:${server.httpServer.address().port}`;
const STEP = "Avancer d'un pas : Clarifier les exigences (clarify-issue)";

let browser;
try {
  browser = await chromium.launch({ headless: true, channel: 'chrome' });
  const errors = [];

  // ---------- All projects: one card, two chips, the picker asks ----------
  const all = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  await all.addInitScript(() => { localStorage.setItem('sectile_selected_project_id', 'all'); });
  const page = await all.newPage();
  page.setDefaultTimeout(15000);
  page.on('pageerror', e => errors.push(e.message));
  await page.goto(`${base}/board`);
  await page.getByText('Shared story').first().waitFor();
  // The step buttons live on the detailed cards.
  await page.getByTitle('Afficher les cartes détaillées', { exact: true }).click();
  await page.getByTitle(STEP, { exact: true }).first().waitFor();
  assert.equal(await page.getByText('Shared story').count(), 1, 'a ticket of two projects is one card');
  const shared = page.locator('div').filter({ hasText: 'Shared story' }).filter({ has: page.locator('[data-card-project="a"]') }).filter({ has: page.locator('[data-card-project="b"]') });
  assert.ok(await shared.count() > 0, 'the card names both projects');

  const launch = async title => {
    const target = page.locator('div').filter({ hasText: title }).filter({ has: page.getByTitle(STEP, { exact: true }) }).last();
    await target.getByTitle(STEP, { exact: true }).click();
  };
  const runs = () => page.evaluate(() => window.fake.runs);

  await launch('Shared story');
  const picker = page.locator('[data-run-project-picker]');
  await picker.waitFor();
  await picker.getByText('GODE-1 appartient à plusieurs projets').waitFor();
  assert.equal(await picker.locator('[data-run-project]').count(), 2);
  await picker.getByRole('button', { name: 'Beta' }).click();
  await picker.waitFor({ state: 'detached' });
  await page.waitForFunction(() => window.fake.runs.length === 1);
  let seen = await runs();
  assert.equal(seen[0].projectId, 'b', 'project choice precedes admission');
  assert.equal(seen[0].deviceId, 'mac');
  assert.equal(seen[0].skillId, 'clarify');
  assert.equal(await page.getByText("Skill ajoutée à la file d'exécution !").count(), 1, 'the queued toast is said once, after the accepted launch');

  // Closing the picker gives the launch up.
  await launch('Shared story');
  await picker.waitFor();
  await page.keyboard.press('Escape');
  await picker.waitFor({ state: 'detached' });
  await page.waitForTimeout(300);
  assert.equal((await runs()).length, 1, 'a closed picker launches nothing more');

  // Project preparation precedes admission; a busy refusal opens no further picker.
  await launch('Busy story');
  await picker.waitFor();
  await picker.getByRole('button', { name: 'Alpha' }).click();
  await page.waitForFunction(() => window.fake.runs.length === 2);
  await page.waitForTimeout(300);
  assert.equal(await picker.count(), 0, 'a busy refusal opens no picker');

  await all.close();

  // ---------- A project's board names its project, nobody is asked ----------
  const alpha = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  await alpha.addInitScript(() => { localStorage.setItem('sectile_selected_project_id', 'a'); });
  const board = await alpha.newPage();
  board.setDefaultTimeout(15000);
  board.on('pageerror', e => errors.push(e.message));
  await board.goto(`${base}/board`);
  await board.getByText('Shared story').first().waitFor();
  if (await board.getByTitle('Afficher les cartes détaillées', { exact: true }).count()) {
    await board.getByTitle('Afficher les cartes détaillées', { exact: true }).click();
  }
  await board.getByTitle(STEP, { exact: true }).first().waitFor();
  assert.equal(await board.locator('[data-card-project]').count(), 0, 'a project board shows no project chips');
  const target = board.locator('div').filter({ hasText: 'Shared story' }).filter({ has: board.getByTitle(STEP, { exact: true }) }).last();
  await target.getByTitle(STEP, { exact: true }).click();
  await board.waitForFunction(() => window.fake.runs.length === 1);
  seen = await board.evaluate(() => window.fake.runs);
  assert.equal(seen[0].projectId, 'a', 'the launch names the project of the board');
  assert.equal(await board.locator('[data-run-project-picker]').count(), 0);
  await alpha.close();

  // ---------- A retry without a project asks, like a launch ----------
  const history = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  await history.addInitScript(() => {
    localStorage.setItem('sectile_selected_project_id', 'all');
    localStorage.setItem('sectile_active_view', 'activities');
  });
  const activities = await history.newPage();
  activities.setDefaultTimeout(15000);
  activities.on('pageerror', e => errors.push(e.message));
  await activities.goto(`${base}/board`);
  const retryButton = activities.getByTitle('Relancer cette skill', { exact: true }).first();
  await retryButton.waitFor();
  const retries = () => activities.evaluate(() => window.fake.retries);
  await retryButton.click();
  const retryPicker = activities.locator('[data-run-project-picker]');
  await retryPicker.waitFor();
  await retryPicker.getByText('GODE-1 appartient à plusieurs projets').waitFor();
  await retryPicker.getByRole('button', { name: 'Beta' }).click();
  await retryPicker.waitFor({ state: 'detached' });
  await activities.waitForFunction(() => window.fake.retries.length === 1);
  const retried = await retries();
  assert.deepEqual(retried[0], { id: 'old', projectId: 'b', deviceId: 'mac' }, 'retry resolves project and workstation before admission');

  // Closing project preparation gives the retry up.
  await retryButton.click();
  await retryPicker.waitFor();
  await activities.keyboard.press('Escape');
  await retryPicker.waitFor({ state: 'detached' });
  await activities.waitForTimeout(300);
  assert.equal((await retries()).length, 1, 'a closed picker retries nothing more');
  await history.close();

  assert.deepEqual(errors, []);
  console.log('run-project-picker: OK');
} finally {
  await browser?.close();
  await server.close();
}
