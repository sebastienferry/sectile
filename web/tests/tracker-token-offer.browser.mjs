// Browser regression for the offer to add a missing tracker token (#645): the
// real App and AppContext, with only the network replaced by an in-page fake of
// the API. Run with Playwright available:
//   PLAYWRIGHT_MODULE=/absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/tracker-token-offer.browser.mjs
//
// What it guards: a creation refused for want of a GitHub token shows a
// notification whose button opens the profile on the tracker credentials with
// the GitHub entry open; a queued write of the signed-in person that failed for
// the same reason offers it too, naming its ticket, while another person's
// keeps the generic failure; once closed, the offer no longer steers the
// profile opened from the sidebar.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

const harness = `
const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const stamp = '2026-09-29T00:00:00Z';
const activity = (id, taskKey, userId) => ({ id, taskId: 't1', taskKey, skillId: 'tracker_op', skillName: 'Écriture tracker', action: 'move', status: 'queued', summary: '', output: '', steps: [], createdAt: stamp, userId });
window.fake = {
  projects: [{ id: 'a', name: 'Alpha', slug: 'a', color: 'indigo', icon: 'Folder', issueTracker: 'github', githubRepo: 'org/a', jiraProject: '', isDefault: true, description: '', repoPath: '', enabledViews: [], bookmarked: true, taskCount: 1 }],
  tasks: [{ id: 't1', projectId: 'a', key: '#1', title: 'Existing story', labels: [], description: '', status: 'to_clarify', priority: 'medium', source: 'github', position: 0, createdAt: stamp, updatedAt: stamp }],
  // Queued writes of the signed-in person and of somebody else. The test
  // fails them once the poller has seen them queued.
  activities: [activity('mine', '#1', 'usr_me'), activity('theirs', '#2', 'usr_other')],
  activityReads: 0,
};
window.fetch = async (input, init = {}) => {
  const url = new URL(typeof input === 'string' ? input : input.url, location.origin);
  const method = (init.method || 'GET').toUpperCase();
  const body = init.body ? JSON.parse(init.body) : null;
  if (url.pathname === '/api/me') return json({ userId: 'usr_me', signedIn: true, identityProvider: false, mode: 'local', role: 'admin' });
  if (url.pathname === '/api/projects') return json(fake.projects);
  if (url.pathname === '/api/tasks' && method === 'POST') {
    return json({ error: 'no personal GitHub token for this user: add one in Profile → Tracker credentials, or the work would be attributed to the server account', code: 'tracker_credential_missing', tracker: 'github' }, 403);
  }
  if (url.pathname === '/api/tasks/facets') return json({ sprints: [], teams: [], macros: [], assignees: [], trackerStatuses: [], statuses: [], sources: [], issueTypes: [], labels: [], total: fake.tasks.length });
  if (url.pathname === '/api/tasks') return json(fake.tasks);
  if (url.pathname === '/api/activities') { fake.activityReads++; return json(fake.activities); }
  if (url.pathname === '/api/me/tracker-credentials') return json([]);
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
        return 'export const useCurrentUser = () => ({ user: { userId: "usr_me", displayName: "Alice", email: "a@b.c", role: "admin", signedIn: true }, loading: false, error: "", reload: async () => {}, rename: async () => "" })';
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
  await context.addInitScript(() => {
    localStorage.setItem('sectile_selected_project_id', 'a');
  });
  const page = await context.newPage();
  page.setDefaultTimeout(15000);
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));

  const offer = page.getByRole('button', { name: 'Ajouter mon jeton GitHub' });
  const profile = page.locator('[role="dialog"][aria-modal="true"]').filter({ hasText: 'Identifiants Trackers' });
  const tab = name => profile.getByRole('button', { name, exact: true });
  const entry = label => profile.locator('div.rounded-xl').filter({ has: page.getByRole('button', { name: new RegExp('^' + label) }) });
  const isActive = async locator => /accent-text/.test(await locator.getAttribute('class'));

  await page.goto(`${base}/board`);
  await page.getByText('Existing story').first().waitFor();

  // ---------- A creation refused for want of a token offers to add it ----------
  await page.getByTitle(/\(N\)$/).first().click();
  const quickAdd = page.getByRole('dialog', { name: 'Ajout rapide' });
  await quickAdd.waitFor();
  await quickAdd.locator('input[type="text"]').first().fill('Needs a token');
  await quickAdd.locator('button[type="submit"]').click();
  await page.getByText('Écriture refusée par le tracker').first().waitFor();
  await page.getByText("Vous n'avez pas de jeton GitHub personnel").first().waitFor();
  assert.equal(await offer.count(), 1, 'the refusal carries the offer');
  await page.keyboard.press('Escape');
  await quickAdd.waitFor({ state: 'detached' });

  // ---------- The offer opens the profile on the GitHub credentials ----------
  await offer.click();
  await profile.waitFor();
  assert.ok(await isActive(tab('Identifiants Trackers')), 'the profile opens on the tracker credentials');
  assert.equal(await entry('GitHub').locator('input[type="password"]').count(), 1, 'the GitHub entry is open');
  assert.equal(await entry('Jira').locator('input[type="password"]').count(), 0, 'the other entries stay closed');
  assert.equal(await offer.count(), 0, 'using the offer closes its notification');

  // ---------- Once closed, the offer no longer steers the profile ----------
  // Opened from the sidebar, the profile keeps the tab last shown, as before
  // the offer existed: pick another one, close, reopen, it stays there.
  await tab('Compte').click();
  await profile.getByRole('button', { name: /fermer|close/i }).first().click();
  await profile.waitFor({ state: 'detached' });
  await page.getByTitle('Profil & Préférences').first().click();
  await profile.waitFor();
  assert.ok(await isActive(tab('Compte')), 'the sidebar opening is not sent back to the tracker credentials');
  await profile.getByRole('button', { name: /fermer|close/i }).first().click();
  await profile.waitFor({ state: 'detached' });

  // ---------- A queued write refused for want of a token offers it too ----------
  // The poller must have seen both activities queued before they fail.
  await page.waitForFunction(() => window.fake.activityReads >= 3, null, { timeout: 30000 });
  await page.evaluate(() => {
    for (const act of window.fake.activities) Object.assign(act, { status: 'failed', credentialMissing: 'github', error: 'no personal GitHub token' });
  });
  await page.getByText("#1 : l'écriture sur GitHub a été refusée").first().waitFor({ timeout: 30000 });
  assert.equal(await offer.count(), 1, "only the signed-in person's activity offers the token");
  await page.getByText('(#2)').first().waitFor();

  assert.deepEqual(errors, [], 'no page error');
  console.log('tracker-token-offer browser regression: passed');
} finally {
  await browser?.close();
  await server.close();
}
