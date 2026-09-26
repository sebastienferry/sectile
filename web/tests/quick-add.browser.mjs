// Browser regression for the quick add dialog (#445): the real App and
// AppContext, with only the network replaced by an in-page fake of the API.
// Run with Playwright available:
//   PLAYWRIGHT_MODULE=/absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/quick-add.browser.mjs
//
// What it guards: two columns on a wide window, stacked on a narrow one; no
// destination, the creation names no source; the macro field offers the
// project's open macros, follows the board's macro filter, starts over on a
// project change, and attaches after the creation, a refusal being a warning;
// the after-saving choice is exclusive, reset at every opening, and either
// opens the ticket and rewrites it, or clarifies it from the board.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

const harness = `
const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const stamp = '2026-09-25T00:00:00Z';
const project = (id, name, issueTracker) => ({ id, name, slug: id, color: 'indigo', icon: 'Folder', issueTracker, githubRepo: issueTracker === 'github' ? 'org/' + id : '', jiraProject: issueTracker === 'jira' ? 'BETA' : '', isDefault: id === 'a', description: '', repoPath: '', enabledViews: [], bookmarked: true, taskCount: 1 });
const task = (id, projectId, key, title) => ({ id, projectId, key, title, labels: [], description: '', status: 'to_clarify', priority: 'medium', source: 'github', position: 0, createdAt: stamp, updatedAt: stamp });
const macro = (projectId, key, title, closed = false) => ({ projectId, key, title, closed, horizon: '', description: '', todos: [], updatedAt: stamp });
window.fake = {
  projects: [project('a', 'Alpha', 'github'), project('b', 'Beta', 'jira')],
  // Under M-7, so the filtered board shows it.
  tasks: [{ ...task('t1', 'a', '#1', 'Existing story'), parentKey: 'M-7', parentType: 'macro' }],
  macros: {
    a: [macro('a', 'M-7', 'Ux improvements'), macro('a', 'M-3', 'Shipped', true), macro('a', 'M-2', 'Onboarding')],
    b: [macro('b', 'BETA-20', 'Beta epic')],
  },
  refuseAttach: false,
  requests: [],
  nextId: 100,
};
window.fetch = async (input, init = {}) => {
  const url = new URL(typeof input === 'string' ? input : input.url, location.origin);
  const method = (init.method || 'GET').toUpperCase();
  const body = init.body ? JSON.parse(init.body) : null;
  fake.requests.push({ method, path: url.pathname, query: url.search, body });
  if (url.pathname === '/api/projects') return json(fake.projects);
  const macros = url.pathname.match(/^\\/api\\/projects\\/([^/]+)\\/macros$/);
  if (macros) return json(fake.macros[decodeURIComponent(macros[1])] || []);
  if (url.pathname === '/api/tasks' && method === 'POST') {
    if (body.title.includes('refused')) return json({ error: 'tracker down' }, 500);
    const id = 'n' + fake.nextId;
    const created = { ...task(id, body.projectId, '#' + fake.nextId++, body.title), description: body.description };
    fake.tasks.unshift(created);
    return json(created, 201);
  }
  const sub = url.pathname.match(/^\\/api\\/tasks\\/([^/]+)\\/(macro|run-skill)$/);
  if (sub && method === 'POST') {
    const t = fake.tasks.find(t => t.id === decodeURIComponent(sub[1]));
    if (sub[2] === 'macro') {
      if (fake.refuseAttach) return json({ error: 'milestone missing' }, 400);
      Object.assign(t, { parentKey: body.macroKey, parentType: 'macro' });
      return json({ queued: true, task: t, activity: { id: 'op-' + t.id, taskId: t.id, skillId: 'tracker_op', status: 'queued', createdAt: stamp } }, 202);
    }
    return json({ task: t, activity: { id: 'run-' + t.id + '-' + body.skillId, taskId: t.id, skillId: body.skillId, status: 'queued', output: '', steps: [], createdAt: stamp } });
  }
  if (url.pathname === '/api/tasks' || url.pathname === '/api/tasks/facets') {
    if (url.pathname === '/api/tasks/facets') return json({ sprints: [], teams: [], macros: [], assignees: [], trackerStatuses: [], statuses: [], sources: [], issueTypes: [], labels: [], total: fake.tasks.length });
    return json(fake.tasks);
  }
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
  // The board is on Alpha, filtered on its macro M-7.
  await context.addInitScript(() => {
    localStorage.setItem('sectile_selected_project_id', 'a');
    localStorage.setItem('sectile_filters_a', JSON.stringify({ parent: 'M-7' }));
  });
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));

  const requests = (method, pattern) => page.evaluate(
    ([m, src]) => fake.requests.filter(r => r.method === m && new RegExp(src).test(r.path)),
    [method, pattern.source],
  );
  const dialog = page.getByRole('dialog', { name: 'Ajout rapide' });
  const macroSelect = dialog.getByLabel('Macro', { exact: true });
  const followUp = name => dialog.getByRole('radio', { name, exact: true });
  const open = async () => {
    await page.getByTitle(/\(N\)$/).first().click();
    await dialog.waitFor();
    // The opening effect resets the form one render after the modal mounts,
    // then Alpha's macros arrive: none, M-2 and M-7. A choice left from the
    // previous opening would keep this waiting, and fail it.
    await page.waitForFunction(() => {
      const select = document.querySelector('#quick-add-macro');
      const none = document.querySelector('input[name="quick-add-follow-up"][value="none"]');
      return select !== null && !select.disabled && select.options.length === 3 && none !== null && none.checked === true;
    });
  };
  const fillTitle = title => dialog.locator('input[type="text"]').first().fill(title);

  await page.goto(`${base}/board`);
  await page.getByText('Existing story').first().waitFor();

  // ---------- Layout: two columns, the description on the left ----------
  await open();
  const main = await dialog.locator('[data-quick-add-column="main"]').boundingBox();
  const details = await dialog.locator('[data-quick-add-column="details"]').boundingBox();
  assert.ok(main && details && details.x >= main.x + main.width, `columns side by side: ${JSON.stringify({ main, details })}`);
  const wide = (await dialog.boundingBox()).width; assert.ok(wide >= 700, `the dialog is wide: ${wide}`);
  assert.equal(await dialog.locator('[data-quick-add-column="main"] textarea').count(), 1, 'the description is in the left column');
  assert.ok((await dialog.locator('textarea').boundingBox()).height >= 200, 'the description is tall');
  assert.equal(await dialog.getByRole('button', { name: 'Écrire' }).count(), 1, 'the Markdown editor is used');
  assert.equal(await dialog.getByLabel('Projet du ticket').count(), 1, 'the project is on the right');

  // ---------- No destination ----------
  assert.equal(await dialog.getByText('Destination', { exact: true }).count(), 0);
  assert.equal(await dialog.getByRole('button', { name: /Sectile/ }).count(), 0);

  // ---------- Macro: open ones only, the board filter pre-selected ----------
  const offered = await macroSelect.locator('option').allTextContents();
  assert.deepEqual(offered, ['Aucune macro', 'M-2 · Onboarding', 'M-7 · Ux improvements'], 'closed macros are not offered');
  assert.equal(await macroSelect.inputValue(), 'M-7', 'the board macro filter is pre-selected');

  // ---------- After saving: exclusive, none by default ----------
  assert.equal(await followUp('Rien').isChecked(), true);
  await followUp('Clarifier').check();
  await followUp('Reformuler en user story').check();
  assert.equal(await followUp('Clarifier').isChecked(), false, 'one choice at a time');

  // ---------- A project change starts the macro over ----------
  await dialog.getByLabel('Projet du ticket').selectOption('b');
  await page.waitForFunction(() => [...document.querySelectorAll('#quick-add-macro option')].some(o => o.value === 'BETA-20'));
  assert.equal(await macroSelect.inputValue(), '', 'the selection resets');
  assert.deepEqual(await macroSelect.locator('option').allTextContents(), ['Aucune macro', 'BETA-20 · Beta epic']);
  assert.ok((await requests('GET', /^\/api\/projects\/b\/macros$/)).length >= 1, 'the macros of the new project are asked for');

  // ---------- Closing forgets the choice ----------
  await page.keyboard.press('Escape');
  await dialog.waitFor({ state: 'detached' });
  await open();
  assert.equal(await followUp('Rien').isChecked(), true, 'the after-saving choice is not remembered');
  assert.equal(await macroSelect.inputValue(), 'M-7');

  // ---------- Save with a macro and a rewrite ----------
  await fillTitle('Rewrite me');
  await followUp('Reformuler en user story').check();
  await dialog.locator('button[type="submit"]').click();
  await dialog.waitFor({ state: 'detached' });
  const [creation] = await requests('POST', /^\/api\/tasks$/);
  assert.equal(creation.body.title, 'Rewrite me');
  assert.equal(creation.body.projectId, 'a');
  assert.equal('source' in creation.body, false, 'the creation names no source');
  assert.equal('macroKey' in creation.body, false, 'the macro is not sent with the creation');
  assert.equal('parentKey' in creation.body, false, 'no local-only parent');
  const attach = await requests('POST', /^\/api\/tasks\/n100\/macro$/);
  assert.deepEqual(attach.map(r => r.body), [{ macroKey: 'M-7' }], 'the new ticket is attached through the tracker path');
  await page.waitForFunction(() => fake.requests.some(r => r.path === '/api/tasks/n100/run-skill'));
  const rewrite = await requests('POST', /^\/api\/tasks\/n100\/run-skill$/);
  assert.deepEqual(rewrite.map(r => r.body.skillId), ['rewrite_story']);
  await page.getByRole('button', { name: 'Reformuler la story' }).waitFor();
  await page.keyboard.press('Escape');
  await page.getByRole('button', { name: 'Reformuler la story' }).waitFor({ state: 'detached' });

  // ---------- No macro and a clarification: stays on the board ----------
  await open();
  await macroSelect.selectOption('');
  await fillTitle('Clarify me');
  await followUp('Clarifier').check();
  await dialog.locator('button[type="submit"]').click();
  await dialog.waitFor({ state: 'detached' });
  await page.waitForFunction(() => fake.requests.some(r => r.path === '/api/tasks/n101/run-skill'));
  const clarify = await requests('POST', /^\/api\/tasks\/n101\/run-skill$/);
  assert.deepEqual(clarify.map(r => r.body.skillId), ['clarify']);
  assert.equal(clarify[0].body.mode, undefined, "the project's execution settings apply");
  assert.equal((await requests('POST', /^\/api\/tasks\/n101\/macro$/)).length, 0, 'no macro, no attachment');
  assert.equal(await page.getByRole('button', { name: 'Reformuler la story' }).count(), 0, 'the detail modal stays closed');

  // ---------- Nothing after saving ----------
  await open();
  await fillTitle('Just save');
  await dialog.locator('button[type="submit"]').click();
  await dialog.waitFor({ state: 'detached' });
  await page.waitForFunction(() => fake.requests.some(r => r.path === '/api/tasks/n102/macro'));
  assert.equal((await requests('POST', /^\/api\/tasks\/n102\/run-skill$/)).length, 0, 'no skill launched');

  // ---------- A refused attachment is a warning, the ticket stays ----------
  await page.evaluate(() => { fake.refuseAttach = true; });
  await open();
  await fillTitle('Orphan');
  await followUp('Clarifier').check();
  await dialog.locator('button[type="submit"]').click();
  await dialog.waitFor({ state: 'detached' });
  await page.getByText('Ticket créé, mais non rattaché à M-7').waitFor();
  await page.waitForFunction(() => fake.requests.some(r => r.path === '/api/tasks/n103/run-skill'));

  // ---------- A refused creation launches nothing and keeps the text ----------
  await open();
  await fillTitle('refused one');
  await followUp('Clarifier').check();
  await dialog.locator('button[type="submit"]').click();
  await page.waitForFunction(() => fake.requests.filter(r => r.method === 'POST' && r.path === '/api/tasks').length === 5);
  await page.getByText('La création a échoué').first().waitFor();
  await dialog.waitFor();
  assert.equal(await dialog.locator('input[type="text"]').first().inputValue(), 'refused one');
  const skillCalls = await requests('POST', /\/run-skill$/);
  assert.equal(skillCalls.length, 3, 'only the three successful creations launched a skill');
  await page.keyboard.press('Escape');
  await dialog.waitFor({ state: 'detached' });

  // ---------- Narrow screen: the columns stack and the dialog fits ----------
  await page.setViewportSize({ width: 390, height: 780 });
  await open();
  const box = await dialog.boundingBox();
  assert.ok(box && box.x >= 0 && box.x + box.width <= 390 && box.y >= 0 && box.y + box.height <= 780, `dialog overflows: ${JSON.stringify(box)}`);
  const narrowMain = await dialog.locator('[data-quick-add-column="main"]').boundingBox();
  const narrowDetails = await dialog.locator('[data-quick-add-column="details"]').boundingBox();
  assert.ok(narrowDetails.y >= narrowMain.y + narrowMain.height, 'the columns stack');
  assert.ok(await dialog.locator('button[type="submit"]').isVisible(), 'the footer stays reachable');
  await page.keyboard.press('Escape');

  assert.deepEqual(errors, [], 'no page error');
  console.log('quick-add browser regression: passed');
} finally {
  await browser?.close();
  await server.close();
}
