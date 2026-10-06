// Browser regression for the per-tracker backlog (#741): the real App and
// AppContext, with only the network replaced by an in-page fake of the API.
// Run with Playwright available:
//   PLAYWRIGHT_MODULE=file:///absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/tracker-backlog.browser.mjs
//
// What it guards: the sidebar has one "Hors projet" entry per tracker a
// visible project selects; an entry lists the tracker's tickets no project
// shows; "Ajouter au projet…" offers only the labelled projects of that
// tracker, sends the choice and the ticket leaves the list; a tracker whose
// projects carry no label says so instead of offering a project.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

const harness = `
const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const stamp = '2026-10-06T00:00:00Z';
const ref = id => ({ trackerId: id, identity: 'jira|acme.atlassian.net|' + id.toUpperCase() });
const project = (id, name, label, trackers) => ({ id, name, slug: id, color: 'indigo', icon: 'Folder', issueTracker: 'jira', jiraProject: trackers[0].toUpperCase(), githubRepo: '', isDefault: id === 'da', description: '', enabledViews: [], bookmarked: true, taskCount: 0, label, trackers: trackers.map(ref), defaultTrackerId: trackers[0], createdAt: stamp, updatedAt: stamp });
const ticket = (id, key, title, labels) => ({ id, key, title, projectId: '', projectIds: [], trackerId: 'gode', labels, description: '', status: 'to_clarify', priority: 'medium', source: 'jira', position: 0, createdAt: stamp, updatedAt: stamp });
window.fake = {
  labelled: [],
  backlogReads: [],
  projects: [project('da', 'Delivery admin', 'delivery-admin', ['gode']), project('ba', 'Bidder admin', 'bidder', ['gode']), project('ops', 'Ops', '', ['be'])],
  backlog: { gode: [ticket('t9', 'GODE-9', 'Orphan ticket', ['bug']), ticket('t10', 'GODE-10', 'Another orphan', [])], be: [{ ...ticket('t20', 'BE-20', 'Unreachable', []), trackerId: 'be' }] },
};
window.fetch = async (input, init = {}) => {
  const url = new URL(typeof input === 'string' ? input : input.url, location.origin);
  const method = (init.method || 'GET').toUpperCase();
  const body = init.body ? JSON.parse(init.body) : null;
  if (url.pathname === '/api/me') return json({ userId: 'usr_me', signedIn: true, identityProvider: false, mode: 'local', role: 'member' });
  if (url.pathname === '/api/projects') return json(fake.projects);
  if (url.pathname === '/api/trackers') return json([
    { id: 'gode', name: 'GODE', provider: 'jira', site: '', scope: 'GODE', identity: 'jira|acme.atlassian.net|GODE' },
    { id: 'be', name: 'BE', provider: 'jira', site: '', scope: 'BE', identity: 'jira|acme.atlassian.net|BE' },
    { id: 'lone', name: 'LONE', provider: 'jira', site: '', scope: 'LONE', identity: 'jira|acme.atlassian.net|LONE' },
  ]);
  const backlog = /^\\/api\\/trackers\\/([^/]+)\\/backlog$/.exec(url.pathname);
  if (backlog && method === 'GET') {
    fake.backlogReads.push(backlog[1]);
    return json(fake.backlog[backlog[1]] || []);
  }
  const label = /^\\/api\\/trackers\\/([^/]+)\\/backlog\\/([^/]+)\\/project$/.exec(url.pathname);
  if (label && method === 'POST') {
    fake.labelled.push({ tracker: label[1], task: label[2], ...body });
    fake.backlog[label[1]] = fake.backlog[label[1]].filter(t => t.id !== label[2]);
    return json({ task: { id: label[2] }, activity: null });
  }
  if (url.pathname === '/api/tasks/facets') return json({ sprints: [], teams: [], macros: [], assignees: [], trackerStatuses: [], statuses: [], sources: [], issueTypes: [], labels: [], total: 0 });
  if (url.pathname === '/api/settings') return json({ userName: 'Alice', language: 'fr', aiProvider: 'claude' });
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

let browser;
try {
  browser = await chromium.launch({ headless: true, channel: 'chrome' });
  const context = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  await context.addInitScript(() => { localStorage.setItem('sectile_selected_project_id', 'da'); });
  const page = await context.newPage();
  page.setDefaultTimeout(15000);
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));

  await page.goto(`${base}/board`);
  const entries = page.locator('[data-sidebar-backlogs]');
  await entries.waitFor();
  await entries.getByText('Hors projet').waitFor();
  // One entry per tracker a visible project selects; LONE has none.
  assert.deepEqual(
    await entries.locator('[data-backlog-entry]').evaluateAll(nodes => nodes.map(n => n.getAttribute('data-backlog-entry'))),
    ['gode', 'be'],
  );

  // ---------- The tracker's tickets in no project ----------
  await entries.locator('[data-backlog-entry="gode"]').click();
  const view = page.locator('[data-tracker-backlog="gode"]');
  await view.waitFor();
  await view.getByText('Hors projet · GODE').waitFor();
  await view.locator('[data-backlog-task="t9"]').waitFor();
  assert.equal(await view.locator('[data-backlog-task]').count(), 2);
  // React's development mode may run the effect twice: every read is GODE's.
  assert.deepEqual([...new Set(await page.evaluate(() => window.fake.backlogReads))], ['gode']);

  // Only the labelled projects selecting the tracker are offered.
  const select = view.getByLabel('Ajouter au projet… GODE-9');
  const offered = await select.locator('option').evaluateAll(options => options.map(o => o.value).filter(Boolean));
  assert.deepEqual(offered, ['da', 'ba']);
  await select.selectOption('ba');
  await view.locator('[data-backlog-task="t9"]').waitFor({ state: 'detached' });
  assert.deepEqual(await page.evaluate(() => window.fake.labelled), [{ tracker: 'gode', task: 't9', projectId: 'ba' }]);
  await page.getByText('GODE-9 ajouté à Bidder admin').first().waitFor();
  assert.equal(await view.locator('[data-backlog-task]').count(), 1);

  // ---------- A tracker whose projects carry no label ----------
  await entries.locator('[data-backlog-entry="be"]').click();
  const be = page.locator('[data-tracker-backlog="be"]');
  await be.waitFor();
  await be.locator('[data-backlog-task="t20"]').waitFor();
  await be.getByText("Aucun projet de ce tracker n'a de label").waitFor();
  assert.equal(await be.locator('select').count(), 0, 'no project to label into, no action');

  assert.deepEqual(errors, []);
  console.log('tracker-backlog: OK');
} finally {
  await browser?.close();
  await server.close();
}
