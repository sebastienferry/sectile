// Browser regression for the admin Trackers screen (#741, D11): the real
// AppContext and AdminView, with only the network replaced by an in-page fake
// of the API. Run with Playwright available:
//   PLAYWRIGHT_MODULE=file:///absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/trackers-admin.browser.mjs
//
// What it guards: an admin sees the Trackers section, with the projects that
// select each tracker; creating a tracker sends its source and lists it; the
// deletion of a tracker a project selects reads its refusal beside it. A
// member sees no Trackers section and never reads the admin tracker routes.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

const harness = `
const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const stamp = '2026-10-06T00:00:00Z';
window.role = new URLSearchParams(location.search).get('role') || 'admin';
window.fake = {
  adminReads: 0,
  created: null,
  trackers: [
    { id: 'gode', name: 'GODE', provider: 'jira', site: '', scope: 'GODE', identity: 'jira|acme.atlassian.net|GODE', autoSyncEnabled: false, autoSyncIntervalMin: 5 },
  ],
  projects: [
    {
      id: 'da',
      name: 'Delivery admin',
      slug: 'da',
      color: 'indigo',
      icon: 'Folder',
      issueTracker: 'jira',
      githubRepo: '',
      isDefault: true,
      description: '',
      enabledViews: [],
      bookmarked: true,
      taskCount: 0,
      label: 'delivery-admin',
      trackers: [{ trackerId: 'gode', identity: 'jira|acme.atlassian.net|GODE' }],
      defaultTrackerId: 'gode',
      createdAt: stamp,
      updatedAt: stamp,
    },
  ],
};
window.fetch = async (input, init = {}) => {
  const url = new URL(typeof input === 'string' ? input : input.url, location.origin);
  const method = (init.method || 'GET').toUpperCase();
  const body = init.body ? JSON.parse(init.body) : null;
  if (url.pathname === '/api/me') return json({ userId: 'usr_me', signedIn: true, identityProvider: false, mode: 'local', role: window.role });
  if (url.pathname === '/api/projects') return json(fake.projects);
  if (url.pathname.startsWith('/api/admin/trackers')) {
    fake.adminReads++;
    const id = url.pathname.split('/')[4];
    if (!id && method === 'GET') return json(fake.trackers);
    if (!id && method === 'POST') {
      fake.created = body;
      const tracker = { ...body, id: 'new-' + fake.trackers.length, identity: body.provider + '||' + body.scope, name: body.name || body.scope };
      fake.trackers.push(tracker);
      return json(tracker, 201);
    }
    if (id && method === 'DELETE') {
      const linked = fake.projects.some(p => (p.trackers || []).some(ref => ref.trackerId === id));
      if (linked) return json({ error: 'ce tracker est encore sélectionné par un projet' }, 409);
      fake.trackers = fake.trackers.filter(t => t.id !== id);
      return json({ deleted: true });
    }
    return json([]);
  }
  if (url.pathname === '/api/trackers') return json(fake.trackers.map(({ id, name, provider, site, scope, identity }) => ({ id, name, provider, site, scope, identity })));
  if (url.pathname === '/api/settings') return json({ userName: 'Alice', language: 'fr', aiProvider: 'claude' });
  if (url.pathname === '/api/tasks/facets') return json({ sprints: [], teams: [], macros: [], assignees: [], trackerStatuses: [], statuses: [], sources: [], issueTypes: [], labels: [], total: 0 });
  if (url.pathname === '/api/admin/tracker-credentials') return json([]);
  if (url.pathname === '/api/admin/stats') return json({ users: { total: 1, admins: 1, blocked: 0, active: 1 }, runs: { active: 0, byStatus: {} }, windowMinutes: 15, generatedAt: stamp });
  if (/stats|settings|status|oauth/.test(url.pathname)) return json({});
  return json([]);
};
window.EventSource = class { addEventListener() {} close() {} };
window.confirm = () => true;
const React = await import('react');
const { createRoot } = await import('react-dom/client');
const { AppProvider } = await import('/src/context/AppContext.tsx');
const { AdminView } = await import('/src/components/AdminView.tsx');
await import('/src/index.css');
createRoot(document.getElementById('root')).render(
  React.createElement(AppProvider, null, React.createElement('div', { style: { height: '100vh' } }, React.createElement(AdminView)))
);
`;

const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{
    name: 'fixture', enforce: 'pre',
    transform(code, id) {
      // The role comes from the page URL, so one server serves both readers.
      if (id.endsWith('/useCurrentUser.ts')) {
        return 'export const useCurrentUser = () => ({ user: { userId: "usr_me", displayName: "Alice", email: "a@b.c", role: window.role, signedIn: true }, loading: false, error: "", reload: async () => {}, rename: async () => "" })';
      }
    },
    configureServer(s) {
      s.middlewares.use(async (req, res, next) => {
        if (!req.url.startsWith('/admin')) return next();
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
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await context.newPage();
  page.setDefaultTimeout(15000);
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));

  // ---------- An admin sees the trackers and who selects them ----------
  await page.goto(`${base}/admin?role=admin`);
  const panel = page.locator('[data-admin-trackers]');
  await panel.waitFor();
  const gode = panel.locator('[data-tracker="gode"]');
  await gode.waitFor();
  await gode.getByText('Projets : Delivery admin').waitFor();

  // ---------- Creating a tracker sends its source ----------
  await panel.getByRole('button', { name: 'Nouveau tracker' }).click();
  const form = panel.locator('[data-tracker-create]');
  await form.waitFor();
  await form.getByRole('button', { name: 'Créer' }).click();
  await form.getByRole('alert').filter({ hasText: 'Choisissez le fournisseur' }).waitFor();
  await form.getByLabel('Fournisseur').selectOption('github');
  await form.getByLabel('Espace, dépôt ou projet').fill('acme');
  await form.getByRole('button', { name: 'Créer' }).click();
  await form.getByRole('alert').filter({ hasText: 'owner/repo' }).waitFor();
  await form.getByLabel('Espace, dépôt ou projet').fill('https://github.com/acme/app.git');
  await form.getByRole('button', { name: 'Créer' }).click();
  await form.waitFor({ state: 'detached' });
  const created = await page.evaluate(() => window.fake.created);
  assert.equal(created.provider, 'github');
  assert.equal(created.scope, 'acme/app', 'the scope is sent as the server stores it');
  const fresh = panel.locator('[data-tracker="new-1"]');
  await fresh.waitFor();
  await fresh.getByText('Aucun projet ne le sélectionne').waitFor();
  // The new tracker opens on its configuration.
  await fresh.locator('[data-tracker-editor]').waitFor();

  // ---------- A tracker a project selects cannot be deleted ----------
  await gode.getByRole('button', { name: 'Supprimer' }).click();
  await gode.getByRole('alert').filter({ hasText: 'Suppression refusée' }).waitFor();
  assert.equal(await panel.locator('[data-tracker="gode"]').count(), 1, 'the refused tracker stays listed');
  // An unused one can.
  await fresh.getByRole('button', { name: 'Supprimer' }).click();
  await fresh.waitFor({ state: 'detached' });

  // ---------- A member sees no Trackers section ----------
  const member = await context.newPage();
  member.on('pageerror', e => errors.push(e.message));
  await member.goto(`${base}/admin?role=member`);
  await member.locator('[data-admin-view], p').first().waitFor();
  await member.waitForTimeout(500);
  assert.equal(await member.locator('[data-admin-trackers]').count(), 0, 'a member sees no Trackers section');
  assert.equal(await member.evaluate(() => window.fake.adminReads), 0, 'a member never reads the admin tracker routes');

  assert.deepEqual(errors, []);
  console.log('trackers-admin: OK');
} finally {
  await browser?.close();
  await server.close();
}
