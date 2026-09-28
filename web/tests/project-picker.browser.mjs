// Browser regression for the project picker and the project overview (#582):
// the real App and AppContext, with only the network replaced by an in-page
// fake of the API.
// Run with Playwright available:
//   PLAYWRIGHT_MODULE=/absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/project-picker.browser.mjs
//
// What it guards: the empty-search menu (recents, 6 favorites, the "more" link,
// no scrollbar), the history across a reload, search on descriptions with
// accents and its 6-row cap, the arrows, Enter and the two-step Escape, the
// overview's filters, star and keyboard opening, the unchanged "All projects"
// and "/" shortcut, and a storage that refuses the history.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

const harness = `
const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const stamp = '2026-09-25T00:00:00Z';
const base = { color: 'indigo', icon: 'Folder', issueTracker: 'github', githubRepo: '', jiraProject: '', isDefault: false, description: '', repoPath: '', enabledViews: [], bookmarked: false, taskCount: 1, createdAt: stamp, updatedAt: stamp };
const favorites = ['Alpha', 'Bravo', 'Charlie', 'Delta', 'Echo', 'Foxtrot', 'Golf', 'Hotel', 'India']
  .map(name => ({ ...base, id: name.toLowerCase(), name, slug: name.toLowerCase(), githubRepo: 'org/' + name.toLowerCase(), bookmarked: true }));
const projects = [
  ...favorites,
  { ...base, id: 'billing', name: 'Billing', slug: 'billing', issueTracker: 'jira', jiraProject: 'BIL', description: 'Monthly invoicing for every publisher and every market we serve' },
  { ...base, id: 'inventory', name: 'Inventory', slug: 'inventory', issueTracker: 'gitlab', gitlabProject: 'exchange/inventory' },
  { ...base, id: 'notes', name: 'Notes', slug: 'notes', issueTracker: 'local', description: 'Équipe paiement et réconciliation' },
];
const tasks = [{ id: 't1', projectId: 'alpha', key: '#1', title: 'Existing story', labels: [], description: '', status: 'to_clarify', priority: 'medium', source: 'github', position: 0, createdAt: stamp, updatedAt: stamp }];
window.fetch = async (input, init) => {
  const url = new URL(typeof input === 'string' ? input : input.url, location.origin);
  const toggle = /^\\/api\\/me\\/project-bookmarks\\/([^/]+)\\/toggle$/.exec(url.pathname);
  if (toggle) {
    const p = projects.find(p => p.id === toggle[1]);
    p.bookmarked = !p.bookmarked;
    return json({ bookmarked: p.bookmarked });
  }
  if (url.pathname === '/api/projects') return json(projects.map(p => ({ ...p })));
  if (url.pathname === '/api/tasks/facets') return json({ sprints: [], teams: [], macros: [], assignees: [], trackerStatuses: [], statuses: [], sources: [], issueTypes: [], labels: [], total: tasks.length });
  if (url.pathname === '/api/tasks') return json(tasks);
  if (url.pathname === '/api/settings') return json({ userName: 'Alice', language: 'en', aiProvider: 'claude' });
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

  // ---------- Main journey ----------
  {
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    await context.addInitScript(() => {
      if (!sessionStorage.getItem('seeded')) {
        localStorage.setItem('sectile.language', 'en');
        localStorage.setItem('sectile_selected_project_id', 'alpha');
        sessionStorage.setItem('seeded', '1');
      }
    });
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    const errors = [];
    page.on('pageerror', e => errors.push(e.message));

    const trigger = page.locator('button[aria-haspopup="listbox"]');
    const listbox = page.locator('#project-picker-listbox');
    const search = page.getByRole('combobox', { name: 'Search a project' });
    const options = listbox.getByRole('option');
    const current = async () => (await trigger.innerText()).split('\n')[0].trim();
    const openPicker = async () => {
      await trigger.click();
      await search.waitFor();
      assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('role')), 'combobox', 'opening focuses the search');
    };
    const sectionOf = name => listbox.getByRole('group', { name, exact: true });
    const rowNames = locator => locator.locator('[role="option"] span.truncate:first-child').allInnerTexts();

    await page.goto(`${base}/board`);
    await page.getByText('Existing story').first().waitFor();
    assert.equal(await current(), 'Alpha');

    // Empty search, empty history: 6 favorites A–Z and the "more" link, nothing else.
    await openPicker();
    assert.equal(await sectionOf('Recent').count(), 0, 'no Recent section without history');
    assert.deepEqual(await rowNames(sectionOf('Favorites')), ['Alpha', 'Bravo', 'Charlie', 'Delta', 'Echo', 'Foxtrot']);
    assert.equal(await options.last().innerText(), '3 more favorites in the overview');
    assert.ok(await page.getByRole('button', { name: /Browse projects…\s*12/ }).isVisible(), 'Browse projects shows the count');
    assert.ok(await page.getByRole('button', { name: /All projects/ }).isVisible());

    // The menu never scrolls, and nothing sticks out of it.
    const overflow = await page.evaluate(() => {
      const menu = document.getElementById('project-picker-listbox').parentElement;
      const box = menu.getBoundingClientRect();
      const found = [];
      for (const el of [menu, ...menu.querySelectorAll('*')]) {
        const style = getComputedStyle(el);
        if (/(auto|scroll)/.test(style.overflowX + style.overflowY)) found.push('scrollable ' + el.tagName);
        const r = el.getBoundingClientRect();
        if (r.width && (r.left < box.left - 0.5 || r.right > box.right + 0.5)) found.push('outside ' + el.textContent.slice(0, 30));
      }
      if (box.bottom > window.innerHeight) found.push('below the window');
      return found;
    });
    assert.deepEqual(overflow, [], 'no scroll area and nothing outside the menu');

    // Search on an accented description; Enter opens the first result.
    await search.fill('equipe');
    assert.deepEqual(await rowNames(sectionOf('Other projects')), ['Notes']);
    assert.equal(await listbox.locator('mark').first().innerText(), 'Équipe', 'the folded match is highlighted in the description');
    await search.press('Enter');
    await listbox.waitFor({ state: 'detached' });
    assert.equal(await current(), 'Notes');

    // The opening survives a reload, and so does the non-favorite selection.
    await page.reload();
    // The projects arrive after the first render, and would take a
    // non-favorite selection away when they do: wait past them.
    await page.waitForFunction(() => /\d+ task/.test(document.querySelector('button[aria-haspopup="listbox"]')?.textContent ?? ''));
    assert.equal(await current(), 'Notes', 'a non-favorite selection survives a reload');
    await openPicker();
    assert.deepEqual(await rowNames(sectionOf('Recent')), ['Notes']);

    // More than 6 matches: 6 rows, then the link, which opens the overview pre-filtered.
    await search.fill('o');
    assert.equal(await options.count(), 7, '6 project rows and the link');
    const more = await options.last().innerText();
    assert.match(more, /^\d+ more matches in the overview$/);
    await search.press('ArrowUp');
    assert.equal(await search.getAttribute('aria-activedescendant'), await options.last().getAttribute('id'), '↑ from nothing goes to the last option');
    await search.press('Enter');
    const dialog = page.getByRole('dialog', { name: /Project overview/ });
    await dialog.waitFor();
    const filter = dialog.getByRole('textbox', { name: 'Filter projects' });
    assert.equal(await filter.inputValue(), 'o');
    assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('aria-label')), 'Filter projects');

    // Tracker filter, star toggle, Escape without opening anything.
    await filter.fill('');
    await dialog.getByRole('button', { name: /^GitLab\s*1$/ }).click();
    const cards = dialog.getByRole('button', { name: /^Open / });
    assert.equal(await cards.count(), 1);
    assert.equal(await cards.first().getAttribute('aria-label'), 'Open Inventory');
    await dialog.getByRole('button', { name: /^All\s*12$/ }).click();
    assert.equal(await cards.count(), 12);
    const notesCard = dialog.getByRole('button', { name: 'Open Notes' });
    assert.match(await notesCard.innerText(), /opened .+ ago/, 'an opened project says when');
    assert.doesNotMatch(await dialog.getByRole('button', { name: 'Open Billing' }).innerText(), /opened/, 'a never opened project says nothing');
    const inventoryStar = dialog.getByRole('button', { name: 'Open Inventory' }).getByRole('button', { name: 'Add to favorites' });
    await inventoryStar.click();
    await dialog.getByRole('button', { name: 'Open Inventory' }).getByRole('button', { name: 'Remove from favorites' }).waitFor();
    assert.ok(await dialog.isVisible(), 'the star does not open the project');
    await page.keyboard.press('Escape');
    await dialog.waitFor({ state: 'detached' });
    assert.equal(await current(), 'Notes', 'Escape changes nothing');

    // Browse projects…, then Enter on a focused card.
    await openPicker();
    await page.getByRole('button', { name: /Browse projects…/ }).click();
    await dialog.waitFor();
    assert.equal(await filter.inputValue(), '', 'Browse opens unfiltered');
    await dialog.getByRole('button', { name: 'Open Billing' }).focus();
    await page.keyboard.press('Enter');
    await dialog.waitFor({ state: 'detached' });
    assert.equal(await current(), 'Billing');

    // Arrows and Enter: Recent is now Billing, Notes.
    await openPicker();
    assert.deepEqual(await rowNames(sectionOf('Recent')), ['Billing', 'Notes']);
    await search.press('ArrowDown');
    await search.press('ArrowDown');
    await search.press('Enter');
    await listbox.waitFor({ state: 'detached' });
    assert.equal(await current(), 'Notes');

    // Escape clears the query first, then closes and gives the focus back.
    await openPicker();
    await search.fill('zzz');
    assert.match(await listbox.innerText(), /No project contains “zzz”\.\s*Search covers the name, slug, description, repository and tracker\./);
    await search.press('Escape');
    assert.equal(await search.inputValue(), '');
    assert.ok(await listbox.isVisible(), 'the first Escape keeps the menu open');
    await search.press('Escape');
    await listbox.waitFor({ state: 'detached' });
    assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('aria-haspopup')), 'listbox', 'the switcher has the focus');

    // Unfavoriting the current project keeps it selected, and it moves to Recent.
    await openPicker();
    await search.fill('inventory');
    await search.press('Enter');
    await listbox.waitFor({ state: 'detached' });
    await openPicker();
    // Inventory sorts past the 6 favorites the empty menu shows: find it.
    await search.fill('inventory');
    await sectionOf('Favorites').getByRole('option').filter({ hasText: 'Inventory' }).getByRole('button', { name: 'Remove from favorites' }).click();
    await sectionOf('Other projects').getByRole('option').filter({ hasText: 'Inventory' }).waitFor();
    await search.press('Escape');
    await sectionOf('Recent').getByRole('option').filter({ hasText: 'Inventory' }).waitFor();
    assert.equal(await current(), 'Inventory');
    await search.press('Escape');
    await listbox.waitFor({ state: 'detached' });

    // "/" still focuses the global search; "All projects" still selects all.
    await page.locator('body').click({ position: { x: 900, y: 600 } });
    await page.keyboard.press('/');
    assert.equal(await page.evaluate(() => document.activeElement?.id), 'global-search-input');
    await page.locator('body').click({ position: { x: 900, y: 600 } });
    await openPicker();
    await page.getByRole('button', { name: /All projects/ }).click();
    assert.equal(await current(), 'All projects');
    assert.equal(await page.evaluate(() => JSON.parse(localStorage.getItem('sectile_recent_project_ids'))[0].id), 'inventory', '"All projects" is not an opening');

    assert.deepEqual(errors, [], 'no page error');
    await context.close();
    console.log('ok - picker and overview');
  }

  // ---------- A storage that refuses the history ----------
  {
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    await context.addInitScript(() => {
      localStorage.setItem('sectile.language', 'en');
      const get = Storage.prototype.getItem;
      const set = Storage.prototype.setItem;
      Storage.prototype.getItem = function (key) {
        if (key === 'sectile_recent_project_ids') throw new DOMException('blocked', 'SecurityError');
        return get.call(this, key);
      };
      Storage.prototype.setItem = function (key, value) {
        if (key === 'sectile_recent_project_ids') throw new DOMException('full', 'QuotaExceededError');
        return set.call(this, key, value);
      };
    });
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    const errors = [];
    page.on('pageerror', e => errors.push(e.message));
    await page.goto(`${base}/board`);
    const trigger = page.locator('button[aria-haspopup="listbox"]');
    await trigger.waitFor();
    await trigger.click();
    const search = page.getByRole('combobox', { name: 'Search a project' });
    await search.fill('billing');
    await search.press('Enter');
    await trigger.click();
    const recent = page.locator('#project-picker-listbox').getByRole('group', { name: 'Recent', exact: true });
    assert.match(await recent.innerText(), /Billing/, 'the session keeps its own history');
    assert.deepEqual(errors, [], 'no page error without storage');
    await context.close();
    console.log('ok - storage refused');
  }

  console.log('project-picker: all checks passed');
} finally {
  await browser?.close();
  await server.close();
}
