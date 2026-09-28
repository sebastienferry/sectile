// Browser regression for the board filters (#581): the real App and
// AppContext, with only the network replaced by an in-page fake of the API that
// can hold chosen answers back, so they arrive out of order.
// Run with Playwright available:
//   PLAYWRIGHT_MODULE=/absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/board-filters.browser.mjs
//
// What it guards: a remembered sprint filters the board even when the
// unfiltered first answer arrives last; a filter change made while an older
// answer is held back wins over it; switching projects keeps each project's
// remembered sprint, team and person, on screen and in storage, while the
// previous project's values are still the only ones known; a remembered value
// gone from its own project is dropped and forgotten, "Unassigned" never is.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

// `fakeSlow` in localStorage holds back matching answers: a list of
// [path, predicate source, delay in ms], the predicate taking the query.
const harness = `
const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const project = (id, name) => ({ id, name, slug: id, color: 'indigo', icon: 'Folder', issueTracker: 'local', isDefault: id === 'a', githubRepo: '', description: '', repoPath: '', enabledViews: [], bookmarked: true, taskCount: 2 });
const task = (id, projectId, key, title, sprint, team, assignee) => ({ id, projectId, key, title, sprint, team, assignee, labels: [], description: '', status: 'to_clarify', priority: 'medium', source: 'local', position: 0, createdAt: '2026-09-28T00:00:00Z', updatedAt: '2026-09-28T00:00:00Z' });
const slow = JSON.parse(localStorage.getItem('fakeSlow') || '[]').map(([path, src, ms]) => [path, new Function('q', 'return (' + src + ')(q)'), ms]);
window.fake = {
  projects: [project('a', 'Alpha'), project('b', 'Beta')],
  tasks: [
    task('a1', 'a', '#1', 'Alpha sprint twelve', 'S12', 'Core', 'Alice'),
    task('a2', 'a', '#2', 'Alpha sprint thirteen', 'S13', 'Ops', 'Bob'),
    task('a3', 'a', '#3', 'Alpha nobody', 'S13', 'Ops', ''),
    task('b1', 'b', '#4', 'Beta sprint twenty', 'S20', 'Web', 'Carol'),
  ],
  requests: [],
};
const distinct = values => [...new Set(values.filter(Boolean))];
window.fetch = async (input, init = {}) => {
  const url = new URL(typeof input === 'string' ? input : input.url, location.origin);
  const method = (init.method || 'GET').toUpperCase();
  fake.requests.push({ method, path: url.pathname, query: url.search });
  const q = url.searchParams;
  const hold = slow.find(([path, match]) => path === url.pathname && match(q));
  if (hold) await new Promise(resolve => setTimeout(resolve, hold[2]));
  if (url.pathname === '/api/projects') return json(fake.projects);
  if (url.pathname === '/api/me/board-views') return json([]);
  if ((url.pathname === '/api/tasks' || url.pathname === '/api/tasks/facets') && method === 'GET') {
    let list = fake.tasks;
    if (q.get('projectId')) list = list.filter(t => t.projectId === q.get('projectId'));
    if (url.pathname === '/api/tasks/facets') {
      return json({
        sprints: distinct(list.map(t => t.sprint)), teams: distinct(list.map(t => t.team)), assignees: distinct(list.map(t => t.assignee)),
        unassignedCount: list.filter(t => !t.assignee).length,
        macros: [], trackerStatuses: [], statuses: [], sources: [], issueTypes: [], labels: [], total: list.length,
      });
    }
    if (q.get('sprint')) list = list.filter(t => t.sprint === q.get('sprint'));
    if (q.get('team')) list = list.filter(t => t.team === q.get('team'));
    if (q.get('assignee') === '__unassigned__') list = list.filter(t => !t.assignee);
    else if (q.get('assignee')) list = list.filter(t => t.assignee === q.get('assignee'));
    return json(list);
  }
  if (url.pathname === '/api/me/assignee-identities') return json({ signedIn: true, fallback: ['Alice'], trackers: [] });
  if (url.pathname === '/api/settings') return json({ userName: 'Alice', language: 'fr' });
  if (/stats|settings|status/.test(url.pathname)) return json({});
  return json([]);
};
window.EventSource = class { addEventListener() {} close() {} };
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
        return 'export const useCurrentUser = () => ({ user: { id: "u1", displayName: "Alice", email: "a@b.c", role: "admin", signedIn: true }, loading: false, error: "", reload: async () => {}, rename: async () => "" })';
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
  const context = await browser.newContext({ viewport: { width: 1600, height: 900 } });
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));

  // Starts the board afresh on `projectId`, with remembered filters per
  // project and the answers to hold back.
  const open = async (projectId, filters, slow = []) => {
    await page.goto(`${base}/board`);
    await page.evaluate(([projectId, filters, slow]) => {
      localStorage.clear();
      localStorage.setItem('sectile_selected_project_id', projectId);
      for (const [scope, value] of Object.entries(filters)) localStorage.setItem(`sectile_filters_${scope}`, JSON.stringify(value));
      localStorage.setItem('fakeSlow', JSON.stringify(slow));
    }, [projectId, filters, slow]);
    await page.reload();
  };
  const stored = scope => page.evaluate(scope => JSON.parse(localStorage.getItem(`sectile_filters_${scope}`) || '{}'), scope);
  const taskQueries = () => page.evaluate(() => fake.requests.filter(r => r.method === 'GET' && r.path === '/api/tasks').map(r => r.query));
  // Waits for every answer held back to have landed, and a little more.
  const settle = ms => page.waitForTimeout(ms + 300);
  const titles = async () => (await page.getByText(/^(Alpha|Beta) /).allTextContents()).sort();
  const sprintInput = () => page.getByPlaceholder('Tous sprints');
  const teamInput = () => page.getByPlaceholder('Toutes équipes');
  // Lookup options are buttons in a portal, named by their label and count.
  const pick = async (input, label) => {
    await input.click();
    await input.fill(label);
    await page.getByRole('button', { name: new RegExp('^' + label) }).last().click();
  };
  const personInput = () => page.locator('input[placeholder="Toutes personnes"], input[placeholder="Toute l\'équipe"]');

  // ---------- US1-1: the unfiltered first answer arrives last ----------
  await open('a', { a: { sprint: 'S12' } }, [['/api/tasks', "q => q.get('projectId') === 'a' && !q.get('sprint')", 800]]);
  await page.getByText('Alpha sprint twelve').waitFor();
  await settle(800);
  assert.ok((await taskQueries()).some(q => !new URLSearchParams(q).get('sprint')), 'the fixture did send an unfiltered first request');
  assert.deepEqual(await titles(), ['Alpha sprint twelve'], 'the late unfiltered answer does not replace the filtered board');
  assert.equal(await sprintInput().inputValue(), 'S12');

  // ---------- US1-2: a filter change wins over an older answer held back ----------
  await open('a', {}, [['/api/tasks', "q => q.get('projectId') === 'a' && q.get('team') === 'Ops'", 800]]);
  await page.getByText('Alpha sprint twelve').waitFor();
  await pick(teamInput(), 'Ops');
  await pick(personInput(), 'Non assigné');
  await page.getByText('Alpha nobody').waitFor();
  await settle(800);
  assert.deepEqual(await titles(), ['Alpha nobody'], 'the team-only answer, landing last, does not replace the team and person board');

  // ---------- US2-1, US2-3: a switch keeps the destination's filters ----------
  // Alpha's values are held back, so Beta's are the only ones known while
  // Alpha's remembered filters come back.
  await open('b', { a: { sprint: 'S12', team: 'Core', assignee: 'Alice' } }, [['/api/tasks/facets', "q => q.get('projectId') === 'a'", 800]]);
  await page.getByText('Beta sprint twenty').waitFor();
  const alpha = page.locator('[class*="group/item"]').filter({ hasText: 'Alpha' }).first();
  if (!(await alpha.isVisible())) await page.locator('button:has-text("Beta")').first().click();
  await alpha.click();
  await page.getByText('Alpha sprint twelve').waitFor();
  await settle(800);
  assert.equal(await sprintInput().inputValue(), 'S12', 'the sprint is kept on screen');
  assert.equal(await teamInput().inputValue(), 'Core', 'the team is kept on screen');
  assert.equal(await personInput().inputValue(), 'Alice', 'the person is kept on screen');
  assert.deepEqual(await stored('a'), { sprint: 'S12', team: 'Core', assignee: 'Alice' }, 'nothing was stored as cleared');
  assert.deepEqual(await titles(), ['Alpha sprint twelve']);
  await page.reload();
  await page.getByText('Alpha sprint twelve').waitFor();
  assert.equal(await sprintInput().inputValue(), 'S12', 'the sprint survives a reload');
  assert.equal(await teamInput().inputValue(), 'Core', 'the team survives a reload');

  // ---------- US2-4, US2-5: a value gone from its own project is dropped ----------
  await open('a', { a: { sprint: 'S11', team: 'Gone', assignee: '__unassigned__' } });
  await page.getByText('Alpha nobody').waitFor();
  await page.waitForFunction(() => {
    const filters = JSON.parse(localStorage.getItem('sectile_filters_a') || '{}');
    return filters.sprint === null && filters.team === null;
  });
  assert.equal(await sprintInput().inputValue(), '', 'a closed sprint is dropped');
  assert.equal((await stored('a')).assignee, '__unassigned__', 'unassigned is never dropped');
  assert.deepEqual(await titles(), ['Alpha nobody']);

  assert.deepEqual(errors, [], 'no page error');
  console.log('board-filters browser regression: passed');
} finally {
  await browser?.close();
  await server.close();
}
