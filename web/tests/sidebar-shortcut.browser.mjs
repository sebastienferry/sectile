// Browser regression for the sidebar shortcut (#474): the real App and
// AppContext, with only the network replaced by an in-page fake of the API.
// Run with Playwright available:
//   PLAYWRIGHT_MODULE=/absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/sidebar-shortcut.browser.mjs
//
// What it guards, once as macOS and once as Linux: the platform's chord toggles
// the sidebar and cancels the browser's own action, from a plain field too; the
// other platform's modifier does not; bare B still opens the board; a modal and
// the Markdown editor's bold win; the state survives a reload; the tooltips and
// the command palette name the platform's chord.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

const harness = `
const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const stamp = '2026-09-25T00:00:00Z';
const project = { id: 'a', name: 'Alpha', slug: 'a', color: 'indigo', icon: 'Folder', issueTracker: 'github', githubRepo: 'org/a', jiraProject: '', isDefault: true, description: '', repoPath: '', enabledViews: [], bookmarked: true, taskCount: 1 };
const tasks = [{ id: 't1', projectId: 'a', key: '#1', title: 'Existing story', labels: [], description: '', status: 'to_clarify', priority: 'medium', source: 'github', position: 0, createdAt: stamp, updatedAt: stamp }];
window.fetch = async input => {
  const url = new URL(typeof input === 'string' ? input : input.url, location.origin);
  if (url.pathname === '/api/projects') return json([project]);
  if (url.pathname === '/api/tasks/facets') return json({ sprints: [], teams: [], macros: [], assignees: [], trackerStatuses: [], statuses: [], sources: [], issueTypes: [], labels: [], total: tasks.length });
  if (url.pathname === '/api/tasks') return json(tasks);
  if (url.pathname === '/api/settings') return json({ userName: 'Alice', language: 'fr', aiProvider: 'claude' });
  if (/stats|settings|status/.test(url.pathname)) return json({});
  return json([]);
};
window.EventSource = class { addEventListener() {} close() {} };
// Whether the last key press was cancelled, read once every handler has run.
window.addEventListener('keydown', e => setTimeout(() => { window.lastKeyPrevented = e.defaultPrevented }));
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

const platforms = [
  { name: 'macOS', platform: 'MacIntel', uaPlatform: 'macOS', chord: 'Meta+b', other: 'Control+b', label: '⌘B', aria: 'Meta+B' },
  { name: 'Linux', platform: 'Linux x86_64', uaPlatform: 'Linux', chord: 'Control+b', other: 'Meta+b', label: 'Ctrl+B', aria: 'Control+B' },
];

let browser;
try {
  browser = await chromium.launch({ headless: true, channel: 'chrome' });
  for (const p of platforms) {
    const context = await browser.newContext({ viewport: { width: 1280, height: 860 } });
    await context.addInitScript(({ platform, uaPlatform }) => {
      Object.defineProperty(Navigator.prototype, 'platform', { get: () => platform, configurable: true });
      Object.defineProperty(Navigator.prototype, 'userAgentData', { get: () => ({ platform: uaPlatform }), configurable: true });
      localStorage.setItem('sectile_selected_project_id', 'a');
    }, p);
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    const errors = [];
    page.on('pageerror', e => errors.push(e.message));

    const sidebar = page.locator('aside:has([aria-keyshortcuts])');
    const collapsed = async () => /\bw-16\b/.test(await sidebar.getAttribute('class'));
    const waitCollapsed = want => page.waitForFunction(
      w => /\bw-16\b/.test(document.querySelector('aside:has([aria-keyshortcuts])')?.className ?? '') === w, want);
    const press = async key => {
      await page.evaluate(() => { window.lastKeyPrevented = undefined });
      await page.keyboard.press(key);
      await page.waitForFunction(() => window.lastKeyPrevented !== undefined);
      return page.evaluate(() => window.lastKeyPrevented);
    };

    await page.goto(`${base}/board`);
    await page.getByText('Existing story').first().waitFor();
    await page.locator('body').click({ position: { x: 900, y: 600 } });

    // ---------- Fresh browser: expanded, tooltips name the chord ----------
    assert.equal(await collapsed(), false, `${p.name}: nothing stored starts expanded`);
    const toggle = sidebar.locator('[aria-keyshortcuts]');
    assert.equal(await toggle.getAttribute('aria-keyshortcuts'), p.aria);
    assert.equal(await toggle.getAttribute('title'), `Replier / Déplier le menu (${p.label})`);

    // ---------- The chord toggles and cancels the browser's action ----------
    assert.equal(await press(p.chord), true, `${p.name}: the chord is cancelled`);
    await waitCollapsed(true);
    assert.equal(await sidebar.locator('[aria-keyshortcuts]').getAttribute('title'), `Sectile - Replier / Déplier le menu (${p.label})`);
    await press(p.chord);
    await waitCollapsed(false);

    // ---------- The other platform's modifier is left alone ----------
    assert.equal(await press(p.other), false, `${p.name}: ${p.other} is not cancelled`);
    assert.equal(await collapsed(), false);

    // ---------- From a plain field: toggles, types nothing ----------
    const search = page.locator('#global-search-input');
    await search.fill('abc');
    await search.focus();
    await press(p.chord);
    await waitCollapsed(true);
    assert.equal(await search.inputValue(), 'abc', 'nothing typed in the field');
    await press(p.chord);
    await waitCollapsed(false);
    await search.fill('');
    await page.locator('body').click({ position: { x: 900, y: 600 } });

    // ---------- Bare B still opens the board ----------
    await page.evaluate(() => localStorage.setItem('sectile_active_view', 'list'));
    await page.keyboard.press('l');
    await page.waitForFunction(() => localStorage.getItem('sectile_active_view') === 'list');
    await page.keyboard.press('b');
    await page.waitForFunction(() => localStorage.getItem('sectile_active_view') === 'board');
    assert.equal(await collapsed(), false, 'bare B does not toggle');

    // ---------- A modal wins, and the Markdown editor keeps bold ----------
    await page.keyboard.press('n');
    const dialog = page.getByRole('dialog', { name: 'Ajout rapide' });
    await dialog.waitFor();
    const description = dialog.locator('textarea').first();
    await description.focus();
    assert.equal(await press(p.chord), true, 'bold cancels the key');
    assert.match(await description.inputValue(), /\*\*/, 'bold is inserted');
    await dialog.locator('input[type="text"]').first().focus();
    await press(p.chord);
    assert.equal(await collapsed(), false, `${p.name}: the chord does nothing behind a modal`);
    await page.keyboard.press('Escape');
    await dialog.waitFor({ state: 'detached' });

    // ---------- Command palette ----------
    await page.keyboard.press(p.name === 'macOS' ? 'Meta+k' : 'Control+k');
    // The palette focuses its field one tick after opening; a letter typed
    // before would go to the bare-letter shortcuts instead.
    await page.waitForFunction(() => document.activeElement?.tagName === 'INPUT' && document.activeElement.id !== 'global-search-input');
    await page.keyboard.type('sidebar');
    const command = page.locator('div.cursor-pointer', { has: page.locator('span', { hasText: /^Replier \/ Déplier le menu$/ }) });
    await command.waitFor();
    assert.equal(await command.locator('kbd').textContent(), p.label, `${p.name}: the palette shows ${p.label}`);
    await command.click();
    await waitCollapsed(true);

    // ---------- Remembered across a reload ----------
    await page.reload();
    await page.getByText('Existing story').first().waitFor();
    assert.equal(await collapsed(), true, `${p.name}: collapsed survives a reload`);
    await sidebar.locator('[aria-keyshortcuts]').click();
    await waitCollapsed(false);
    await page.reload();
    await page.getByText('Existing story').first().waitFor();
    assert.equal(await collapsed(), false, `${p.name}: expanded survives a reload`);

    assert.deepEqual(errors, [], `${p.name}: no page error`);
    await context.close();
    console.log(`ok - ${p.name}`);
  }
  console.log('sidebar-shortcut: all checks passed');
} finally {
  await browser?.close();
  await server.close();
}
