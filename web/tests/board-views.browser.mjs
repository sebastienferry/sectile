// Browser regression for saved board views (#387): the real App and AppContext,
// with only the network replaced by an in-page fake of the API.
// Run with Playwright available:
//   PLAYWRIGHT_MODULE=/absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/board-views.browser.mjs
//
// What it guards: creating a view from the sidebar opens it and names it in the
// address; the task list is asked with viewId and cards name their project;
// filters are remembered per view apart from projects; a direct link opens the
// view, a foreign one falls back with a message; editing reloads the board,
// deleting leaves it; a ticket created from a view picks its project among the
// view's and starts with the label of a single-label view.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

// The fake API keeps its state in the page, so a reload starts it over; the
// tests that need a view to exist before the first request seed it through
// localStorage (`fakeSeed`).
const harness = `
const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const project = (id, name, bookmarked = true) => ({ id, name, slug: id, color: 'indigo', icon: 'Folder', issueTracker: 'local', isDefault: id === 'a', githubRepo: '', description: '', repoPath: '', enabledViews: [], bookmarked, taskCount: 1 });
const task = (id, projectId, key, title, labels, assignee = '') => ({ id, projectId, key, title, labels, assignee, description: '', status: 'to_clarify', priority: 'medium', source: 'local', position: 0, createdAt: '2026-09-23T00:00:00Z', updatedAt: '2026-09-23T00:00:00Z' });
const seed = JSON.parse(localStorage.getItem('fakeSeed') || '{}');
window.fake = {
  // Gamma is not bookmarked: a view selects it all the same.
  projects: [project('a', 'Alpha'), project('b', 'Beta'), project('c', 'Gamma', false)],
  tasks: [
    task('t1', 'a', '#42', 'Shared story', ['platform']),
    task('t2', 'b', '#42', 'Shared story', ['Platform']),
    task('t3', 'a', '#7', 'Alpha only', ['platform-x'], 'Alice'),
    task('t4', 'b', '#8', 'Beta mine', ['platform'], 'Alice'),
    task('t5', 'c', '#9', 'Gamma platform', ['platform']),
  ],
  views: seed.views || [],
  requests: [],
  nextId: 1,
};
const stamp = '2026-09-23T00:00:00Z';
window.fetch = async (input, init = {}) => {
  const url = new URL(typeof input === 'string' ? input : input.url, location.origin);
  const method = (init.method || 'GET').toUpperCase();
  const body = init.body ? JSON.parse(init.body) : null;
  fake.requests.push({ method, path: url.pathname, query: url.search, body });
  const q = url.searchParams;
  if (url.pathname === '/api/projects') return json(fake.projects);
  if (url.pathname === '/api/me/board-views' && method === 'GET') return json(fake.views);
  if (url.pathname === '/api/me/board-views' && method === 'POST') {
    const view = { id: 'v' + fake.nextId++, name: body.name, projectIds: body.projectIds, labels: body.labels, createdAt: stamp, updatedAt: stamp };
    fake.views.push(view);
    return json(view, 201);
  }
  const viewMatch = url.pathname.match(/^\\/api\\/me\\/board-views\\/(.+)$/);
  if (viewMatch) {
    const view = fake.views.find(v => v.id === decodeURIComponent(viewMatch[1]));
    if (!view) return json({ error: 'vue introuvable' }, 404);
    if (method === 'PATCH') { Object.assign(view, body); return json(view); }
    if (method === 'DELETE') { fake.views = fake.views.filter(v => v !== view); return new Response(null, { status: 204 }); }
    return json(view);
  }
  if ((url.pathname === '/api/tasks' || url.pathname === '/api/tasks/facets') && method === 'GET') {
    let list = fake.tasks;
    const viewId = q.get('viewId');
    if (viewId) {
      const view = fake.views.find(v => v.id === viewId);
      if (!view) return json({ error: 'vue introuvable' }, 404);
      const wanted = view.labels.map(l => l.toLowerCase());
      list = list.filter(t => view.projectIds.includes(t.projectId) && (wanted.length === 0 || t.labels.some(l => wanted.includes(l.toLowerCase()))));
    } else if (q.get('projectId')) {
      list = list.filter(t => t.projectId === q.get('projectId'));
    }
    if (q.get('assignee')) list = list.filter(t => t.assignee === q.get('assignee'));
    // The server resolves "me" (#468); the fake knows it is Alice.
    if (q.get('mine') === '1') list = list.filter(t => t.assignee === 'Alice');
    if (url.pathname === '/api/tasks/facets') {
      return json({ sprints: [], teams: [], macros: [], assignees: [], trackerStatuses: [], statuses: [], sources: [], issueTypes: [], labels: [{ value: 'platform', count: 3 }], total: list.length });
    }
    return json(list);
  }
  if (url.pathname === '/api/me/assignee-identities') return json({ signedIn: true, fallback: ['Alice', 'a@b.c'], trackers: [] });
  if (url.pathname === '/api/settings') return json({ userName: 'Alice', language: 'fr' });
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
  const context = await browser.newContext({ viewport: { width: 1280, height: 860 } });
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));

  const taskRequests = () => page.evaluate(() => fake.requests.filter(r => r.method === 'GET' && r.path === '/api/tasks').map(r => r.query));
  const lastTaskQuery = async () => new URLSearchParams((await taskRequests()).at(-1) || '');
  const waitForTaskQuery = predicate => page.waitForFunction(
    src => { const q = fake.requests.filter(r => r.method === 'GET' && r.path === '/api/tasks').at(-1); return q ? new Function('q', src)(new URLSearchParams(q.query)) : false },
    `return (${predicate})(q)`,
  );

  await page.goto(`${base}/board`);
  await page.getByRole('button', { name: 'Nouvelle vue', exact: true }).waitFor();

  // ---------- S1: create a view from the sidebar, it opens ----------
  await page.getByRole('button', { name: 'Nouvelle vue', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await dialog.waitFor();
  // Refused while incomplete, with the reason.
  await dialog.getByRole('button', { name: 'Créer la vue' }).click();
  assert.match(await dialog.getByRole('alert').textContent(), /nom/i);
  await dialog.getByLabel('Nom *').fill('Platform');
  await dialog.getByRole('button', { name: 'Créer la vue' }).click();
  assert.match(await dialog.getByRole('alert').textContent(), /projet/i);
  await dialog.getByRole('checkbox', { name: 'Alpha' }).check();
  await dialog.getByRole('checkbox', { name: 'Beta' }).check();
  await dialog.getByLabel('Labels').fill('platform');
  await dialog.getByLabel('Labels').press('Enter');
  await dialog.getByRole('button', { name: 'Créer la vue' }).click();
  await dialog.waitFor({ state: 'detached' });

  const created = await page.evaluate(() => fake.requests.find(r => r.method === 'POST' && r.path === '/api/me/board-views').body);
  assert.deepEqual(created, { name: 'Platform', projectIds: ['a', 'b'], labels: ['platform'] });
  await waitForTaskQuery(q => q.get('viewId') === 'v1');
  let query = await lastTaskQuery();
  assert.equal(query.get('projectId'), null, 'a view replaces the project scope');
  assert.equal(new URL(page.url()).searchParams.get('view'), 'v1', 'the address names the open view');
  await page.locator('[data-board-view="v1"] button[aria-current="page"]').waitFor();
  await page.locator('[data-open-board-view="v1"]').waitFor();

  // ---------- S6: one remote story in two projects, two badged cards ----------
  await page.getByText('Shared story').first().waitFor();
  assert.equal(await page.getByText('Shared story').count(), 2);
  assert.equal(await page.locator('[data-card-project="a"]').count(), 1, 'Alpha cards carry their project');
  assert.equal(await page.getByText('Alpha only').count(), 0, 'a label that only contains the view label stays out');
  assert.equal(await page.locator('[data-card-project="b"]').count(), 2, 'Beta cards carry their project');
  assert.equal(await page.getByText('Gamma platform').count(), 0, 'a project outside the view stays out');

  // ---------- S8: filters are remembered per view, apart from projects ----------
  // My Tasks sends no name: the server knows who "me" is on each tracker (#468).
  const myTasks = page.getByRole('button', { name: 'Mes tâches' });
  await myTasks.click();
  await waitForTaskQuery(q => q.get('viewId') === 'v1' && q.get('mine') === '1' && !q.get('assignee'));
  assert.equal(await page.evaluate(() => JSON.parse(localStorage.getItem('sectile_filters_view_v1') || '{}').mine), '1');
  assert.equal(await myTasks.getAttribute('aria-pressed'), 'true');
  await page.evaluate(() => localStorage.setItem('sectile_filters_a', JSON.stringify({})));
  // Leaving through the project switcher drops the view and its filters.
  await page.locator('button:has-text("Tous les projets")').first().click();
  await page.locator('[class*="group/item"]').filter({ hasText: 'Alpha' }).first().click();
  // Switching scope sends one request with the previous filters before the
  // destination's are restored, as a project switch always did: what matters
  // is where the board settles.
  await waitForTaskQuery(q => q.get('projectId') === 'a' && !q.get('viewId') && !q.get('mine'));
  assert.equal(new URL(page.url()).searchParams.get('view'), null, 'leaving the view clears the address');
  await page.locator('[data-board-view="v1"] button').first().click();
  await waitForTaskQuery(q => q.get('viewId') === 'v1' && q.get('mine') === '1');
  // The header shows one chip named after the button, whose × turns it off.
  // The chips only show on a wide screen.
  await page.setViewportSize({ width: 1600, height: 860 });
  const chip = page.locator('header span').filter({ hasText: /^Mes tâches$/ });
  await chip.waitFor();
  assert.equal(await chip.count(), 1);
  await page.setViewportSize({ width: 1280, height: 860 });

  // A stage shortcut clears My Tasks, as it clears the person filter. They
  // are the sidebar's in the workflow grouping.
  const grouping = page.getByRole('group', { name: 'Workflow / Statuts' });
  await grouping.getByRole('button', { name: 'Workflow' }).click();
  await page.getByTitle('À tester', { exact: true }).first().click();
  await waitForTaskQuery(q => q.get('viewId') === 'v1' && q.get('status') === 'to_test' && !q.get('mine'));
  assert.equal(await myTasks.getAttribute('aria-pressed'), 'false');
  await page.getByTitle('Toutes les tâches', { exact: true }).first().click();
  await waitForTaskQuery(q => q.get('viewId') === 'v1' && !q.get('status') && !q.get('mine'));
  await grouping.getByRole('button', { name: 'Statuts' }).click();

  // A person remembered by an earlier version stays a person filter.
  await page.evaluate(() => localStorage.setItem('sectile_filters_view_v1', JSON.stringify({ assignee: 'Alice' })));
  await page.locator('button:has-text("Tous les projets")').first().click();
  await page.locator('[class*="group/item"]').filter({ hasText: 'Alpha' }).first().click();
  await waitForTaskQuery(q => q.get('projectId') === 'a' && !q.get('viewId'));
  await page.locator('[data-board-view="v1"] button').first().click();
  await waitForTaskQuery(q => q.get('viewId') === 'v1' && q.get('assignee') === 'Alice' && !q.get('mine'));
  assert.equal(await myTasks.getAttribute('aria-pressed'), 'false', 'a remembered name is not My Tasks');

  // ---------- S12: editing the open view reloads its board ----------
  // Turning My Tasks on replaces the person filter, and off again clears both.
  await myTasks.click();
  await waitForTaskQuery(q => q.get('viewId') === 'v1' && q.get('mine') === '1' && !q.get('assignee'));
  await page.setViewportSize({ width: 1600, height: 860 });
  await chip.locator('button').click();
  await waitForTaskQuery(q => q.get('viewId') === 'v1' && !q.get('mine') && !q.get('assignee'));
  assert.equal(await myTasks.getAttribute('aria-pressed'), 'false');
  await page.setViewportSize({ width: 1280, height: 860 });
  const before = (await taskRequests()).length;
  await page.locator('[data-open-board-view="v1"]').click();
  await dialog.waitFor();
  assert.equal(await dialog.getByLabel('Nom *').inputValue(), 'Platform');
  await dialog.getByRole('checkbox', { name: 'Beta' }).uncheck();
  await dialog.getByRole('button', { name: 'Enregistrer' }).click();
  await dialog.waitFor({ state: 'detached' });
  await page.waitForFunction(n => fake.requests.filter(r => r.method === 'GET' && r.path === '/api/tasks').length > n, before);
  await page.locator('[data-card-project="b"]').first().waitFor({ state: 'detached' });

  // ---------- S15: a ticket from a single-label view ----------
  await page.getByTitle(/\(N\)$/).first().click();
  const quickAdd = page.locator('form').filter({ has: page.getByLabel('Projet du ticket') });
  await quickAdd.waitFor();
  const projectSelect = quickAdd.getByLabel('Projet du ticket');
  // The opening effect resets the form one render after the modal mounts.
  await page.waitForFunction(() => document.querySelector('select[aria-label="Projet du ticket"]')?.value === '');
  const options = await projectSelect.locator('option:not([disabled])').allTextContents();
  assert.deepEqual(options, ['Alpha'], "only the view's projects are offered");
  assert.equal(await quickAdd.getByText('#platform', { exact: true }).count(), 1, 'the single label is prefilled');
  await quickAdd.locator('input[type="text"]').first().fill('New from view');
  assert.equal(await quickAdd.locator('button[type="submit"]').isDisabled(), true, 'submitting waits for a project');
  await page.keyboard.press('Escape');

  // ---------- S10: a direct link opens the view ----------
  const cold = await context.newPage();
  cold.on('pageerror', e => errors.push(e.message));
  await cold.addInitScript(() => localStorage.setItem('fakeSeed', JSON.stringify({ views: [{ id: 'v9', name: 'Linked', projectIds: ['c'], labels: [], createdAt: '', updatedAt: '' }] })));
  await cold.goto(`${base}/board?view=v9`);
  await cold.locator('[data-open-board-view="v9"]').waitFor();
  const coldFirst = await cold.evaluate(() => fake.requests.find(r => r.path === '/api/tasks').query);
  assert.equal(new URLSearchParams(coldFirst).get('viewId'), 'v9', 'the first request already names the view');
  // Gamma is not bookmarked: the all-projects bookmark filter must not apply.
  await cold.getByText('Gamma platform').waitFor();

  // ---------- S11: a foreign or missing view falls back, and says so ----------
  await cold.goto(`${base}/board?view=someone-else`);
  await cold.getByText('Vue indisponible').waitFor();
  await cold.waitForFunction(() => !new URL(location.href).searchParams.get('view'));
  await cold.waitForFunction(() => { const q = fake.requests.filter(r => r.path === '/api/tasks').at(-1); return q && !new URLSearchParams(q.query).get('viewId') });

  // ---------- Empty view: every project deleted ----------
  await cold.addInitScript(() => localStorage.setItem('fakeSeed', JSON.stringify({ views: [{ id: 'v8', name: 'Orphan', projectIds: [], labels: [], createdAt: '', updatedAt: '' }] })));
  await cold.goto(`${base}/board?view=v8`);
  await cold.locator('[data-empty-board-view]').waitFor();
  await cold.locator('[data-empty-board-view]').getByRole('button', { name: 'Modifier la vue' }).click();
  await cold.getByRole('dialog').waitFor();
  await cold.close();

  // ---------- S13: deleting the open view leaves it ----------
  await page.locator('[data-open-board-view="v1"]').click();
  await dialog.waitFor();
  await dialog.getByRole('button', { name: 'Supprimer la vue' }).click();
  await dialog.waitFor({ state: 'detached' });
  await page.getByText('Vue supprimée').waitFor();
  assert.equal(await page.locator('[data-board-view="v1"]').count(), 0);
  await page.waitForFunction(() => !new URL(location.href).searchParams.get('view'));
  await waitForTaskQuery(q => !q.get('viewId'));

  // ---------- Narrow screen: the dialog fits ----------
  await page.setViewportSize({ width: 390, height: 780 });
  await page.getByRole('button', { name: 'Nouvelle vue', exact: true }).click();
  const box = await dialog.boundingBox();
  assert.ok(box && box.x >= 0 && box.x + box.width <= 390 && box.y + box.height <= 780, `dialog overflows: ${JSON.stringify(box)}`);
  await page.keyboard.press('Escape');

  assert.deepEqual(errors, [], 'no page error');
  console.log('board-views browser regression: passed');
} finally {
  await browser?.close();
  await server.close();
}
