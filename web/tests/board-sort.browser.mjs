// Browser regression for the card sort selector (#402): the real App and
// AppContext, with only the network replaced by an in-page fake of the API.
// Run with Playwright available:
//   PLAYWRIGHT_MODULE=/absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/board-sort.browser.mjs
//
// What it guards: the Board toolbar offers the selector next to Workflow /
// Statuts and starts on priority; Epic keeps an epic's cards together, the
// most urgent epic first; the direction button flips the order and names it;
// the Backlog shows the same remembered sort in place of its "Priorité" button,
// and a change there reaches the Board; a reload keeps it; a flat table header
// click sorts that table only, and a selector change drops it.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

// Every ticket sits in the same column, so the order of the titles on the page
// is the order of that column.
const harness = `
const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const project = { id: 'a', name: 'Alpha', slug: 'a', color: 'indigo', icon: 'Folder', issueTracker: 'local', isDefault: true, githubRepo: '', description: '', repoPath: '', enabledViews: [], bookmarked: true, taskCount: 4 };
const task = (key, priority, parentKey, updatedAt) => ({ id: 't' + key.slice(1), projectId: 'a', key, title: 'Card ' + key.slice(1), labels: [], assignee: '', description: '', status: 'to_clarify', priority, parentKey, source: 'local', position: 0, createdAt: '2026-01-01T00:00:00Z', updatedAt });
// The tickets arrive in an order that matches none of the sorts.
const tasks = [
  task('#42', 'high', 'M-1', '2026-03-01T00:00:00Z'),
  task('#7', 'medium', undefined, '2026-04-01T00:00:00Z'),
  task('#402', 'urgent', 'M-2', '2026-02-01T00:00:00Z'),
  task('#9', 'low', 'M-1', '2026-01-01T00:00:00Z'),
];
window.fetch = async (input) => {
  const url = new URL(typeof input === 'string' ? input : input.url, location.origin);
  if (url.pathname === '/api/projects') return json([project]);
  if (url.pathname === '/api/tasks') return json(tasks);
  if (url.pathname === '/api/tasks/facets') return json({ sprints: [], teams: [], macros: [], assignees: [], trackerStatuses: [], statuses: [], sources: [], issueTypes: [], labels: [], total: tasks.length });
  if (url.pathname === '/api/me/assignee-identities') return json({ signedIn: true, fallback: ['Alice'], trackers: [] });
  if (url.pathname === '/api/settings') return json({ userName: 'Alice', language: 'fr', aiProvider: 'claude' });
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

  // The keys of the cards, or of the Backlog rows, in the order they are on screen.
  const order = () => page.evaluate(() => Array.from(document.querySelectorAll('main *, #root *'))
    .filter(el => el.children.length === 0 && /^Card \d+$/.test(el.textContent.trim()))
    .map(el => '#' + el.textContent.trim().slice(5)));
  const expectOrder = async (expected, message) => {
    await page.waitForFunction(
      want => JSON.stringify(Array.from(document.querySelectorAll('#root *'))
        .filter(el => el.children.length === 0 && /^Card \d+$/.test(el.textContent.trim()))
        .map(el => '#' + el.textContent.trim().slice(5))) === want,
      JSON.stringify(expected),
    ).catch(() => {});
    assert.deepEqual(await order(), expected, message);
  };
  const field = page.getByTestId('board-sort-field');
  const direction = page.getByTestId('board-sort-direction');
  const stored = () => page.evaluate(() => JSON.parse(localStorage.getItem('sectile_board_sort') || 'null'));

  await page.goto(`${base}/board`);
  await page.getByText('Card 402').first().waitFor();

  // ---------- US1.1, D11: the Board selector, priority by default ----------
  const toolbar = page.getByRole('group', { name: 'Workflow / Statuts' }).locator('..');
  assert.equal(await toolbar.getByTestId('board-sort-field').count(), 1, 'the selector sits next to Workflow / Statuts');
  assert.equal(await field.inputValue(), 'priority');
  assert.equal(await direction.getAttribute('aria-label'), 'Décroissant');
  assert.deepEqual(await field.locator('option').allTextContents(), ['Priorité', 'Epic', 'Clé', 'Dernière mise à jour']);
  await expectOrder(['#402', '#42', '#7', '#9'], 'priority, highest first');
  assert.equal(await stored(), null, 'nothing is stored until the viewer picks a sort');

  // ---------- US2.2: Epic keeps an epic together, the most urgent epic first ----------
  await field.selectOption('epic');
  await expectOrder(['#402', '#42', '#9', '#7'], 'epic, highest first');
  assert.deepEqual(await stored(), { field: 'epic', asc: false });

  // ---------- US3.1, US3.4: the direction button flips priority ----------
  await field.selectOption('priority');
  await direction.click();
  await expectOrder(['#9', '#7', '#42', '#402'], 'priority, lowest first');
  assert.equal(await direction.getAttribute('aria-label'), 'Croissant');
  assert.equal(await direction.getAttribute('title'), 'Croissant');

  // ---------- US1.6: a pick starts in its natural direction ----------
  await field.selectOption('epic');
  assert.equal(await direction.getAttribute('aria-label'), 'Décroissant', 'epic starts highest first');

  // ---------- US1.2, US4.1, US5.1: the Backlog shows the Board's choice ----------
  await page.getByTitle('Backlog', { exact: true }).click();
  await page.getByText('tâches dans le backlog').waitFor();
  assert.equal(await field.inputValue(), 'epic');
  assert.equal(await direction.getAttribute('aria-label'), 'Décroissant');
  assert.equal(await page.getByRole('button', { name: 'Priorité', exact: true }).count(), 0, 'the "Priorité" button is gone');
  assert.equal(await page.getByTitle('Trier par priorité').count(), 0, 'the "Priorité" button is gone');
  await expectOrder(['#402', '#42', '#9', '#7'], 'the grouped Backlog follows epic');

  // ---------- US4.2: a change in the Backlog reaches the Board ----------
  await field.selectOption('key');
  await expectOrder(['#7', '#9', '#42', '#402'], 'the grouped Backlog follows key');
  await page.getByTitle('Board', { exact: true }).click();
  await page.getByRole('group', { name: 'Workflow / Statuts' }).waitFor();
  assert.equal(await field.inputValue(), 'key');
  await expectOrder(['#7', '#9', '#42', '#402'], 'the Board follows the sort chosen in the Backlog');

  // ---------- US4.3: a reload keeps the sort ----------
  await page.reload();
  await page.getByText('Card 402').first().waitFor();
  assert.equal(await field.inputValue(), 'key');
  assert.equal(await direction.getAttribute('aria-label'), 'Croissant');
  await expectOrder(['#7', '#9', '#42', '#402'], 'the reloaded Board keeps key, oldest first');

  // ---------- US5.2-US5.4: a flat table header overrides the selector for that table ----------
  await field.selectOption('epic');
  await page.getByTitle('Backlog', { exact: true }).click();
  await page.getByText('tâches dans le backlog').waitFor();
  await page.getByLabel('Grouper par statut').uncheck();
  await expectOrder(['#402', '#42', '#9', '#7'], 'the flat table follows the selector');
  const keyHeader = page.locator('th').filter({ hasText: 'Clé' });
  await keyHeader.click();
  await expectOrder(['#7', '#9', '#42', '#402'], 'a key header click sorts by key, ascending');
  await keyHeader.click();
  await expectOrder(['#402', '#42', '#9', '#7'], 'a second click flips it');
  assert.equal(await field.inputValue(), 'epic', 'the selector still shows the remembered sort');
  assert.deepEqual(await stored(), { field: 'epic', asc: false }, 'a header click is never remembered');

  // Without a header click, a click on the column the selector sorts by flips it.
  await field.selectOption('priority');
  await expectOrder(['#402', '#42', '#7', '#9'], 'the flat table follows priority');
  await page.locator('th').filter({ hasText: 'Priorité' }).click();
  await expectOrder(['#9', '#7', '#42', '#402'], 'the priority header flips the selector order');

  // ---------- US5.5: a selector change drops the header sort ----------
  await field.selectOption('epic');
  await direction.click();
  await expectOrder(['#42', '#9', '#402', '#7'], 'epic reversed: groups flip, orphans stay last');
  await keyHeader.click();
  await expectOrder(['#7', '#9', '#42', '#402']);
  await direction.click();
  await expectOrder(['#402', '#42', '#9', '#7'], 'the direction button drops the header sort');

  // ---------- US5.7: the grouped view never follows the header sort ----------
  await keyHeader.click();
  await expectOrder(['#7', '#9', '#42', '#402']);
  await page.getByLabel('Grouper par statut').check();
  await expectOrder(['#402', '#42', '#9', '#7'], 'the grouped view follows the selector');

  assert.deepEqual(errors, [], 'no page error');
  console.log('board-sort browser regression: passed');
} finally {
  await browser?.close();
  await server.close();
}
