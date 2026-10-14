// Isolated browser regression (#809): the macro key of the Roadmap and the
// Timeline. Real MacroKeyRef, mocked useApp.
// Run with Playwright available: node tests/macro-key-ref.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
//
// What it guards: a link only when the macro has an address; the copy button
// copies the key alone with the ticket detail's toasts; and no click, press or
// drag begun on the link or the button reaches the clickable, draggable row
// around it. The drags are real mouse drags: a synthetic dragstart dispatched
// on a child would not show which element the browser picks as drag source.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

const harness = `import React from 'react'; import {createRoot} from 'react-dom/client'; import {MacroKeyRef} from '/src/components/MacroKeyRef.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.toasts=[];window.rowEvents={click:0,mousedown:0,dragstart:0};
window.clip=[];window.refuseClipboard=false;
Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{if(window.refuseClipboard) throw new Error('denied'); window.clip.push(text)}}});
window.ctx={t:translations.fr,addToast:t=>window.toasts.push([t.type,t.title,t.description])};
const row=(id,children)=>(
  <div data-row={id} draggable onClick={()=>window.rowEvents.click++} onMouseDown={()=>window.rowEvents.mousedown++}
    onDragStart={()=>window.rowEvents.dragstart++} style={{padding:'24px',margin:'12px',border:'1px solid gray',width:'400px'}}>
    {children}<span style={{marginLeft:'40px'}}>row body</span>
  </div>
);
createRoot(document.getElementById('root')).render(<div>
  {row("linked",<MacroKeyRef macroKey="M-3" url="https://github.com/acme/app/milestone/3"/>)}
  {row("plain",<MacroKeyRef macroKey="M-1"/>)}
  <div data-drop style={{height:'80px',margin:'12px',border:'1px dashed gray'}}>drop</div>
</div>);`;

const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{
    name: 'fixture', enforce: 'pre',
    transform(code, id) {
      if (id.includes('/src/components/')) return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/, 'const useApp = () => window.ctx');
    },
    configureServer(s) {
      s.middlewares.use(async (req, res, next) => {
        if (req.url !== '/fixture') return next();
        res.setHeader('Content-Type', 'text/html');
        res.end(await s.transformIndexHtml(req.url, `<div id="root"></div><script type="module" src="/fixture.tsx"></script>`));
      });
    },
    resolveId(id) { if (id === '/fixture.tsx') return root + id; },
    load(id) { if (id === root + '/fixture.tsx') return harness; },
  }],
});
await server.listen();

let browser;
try {
  browser = await chromium.launch({ headless: true, channel: 'chrome' });
  const page = await browser.newPage({ viewport: { width: 900, height: 600 } });
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`);

  const linked = page.locator('[data-row="linked"]');
  const plain = page.locator('[data-row="plain"]');
  const parent = () => page.evaluate(() => ({ ...window.rowEvents }));
  const reset = () => page.evaluate(() => { window.rowEvents = { click: 0, mousedown: 0, dragstart: 0 }; window.toasts = []; window.clip = []; });
  await linked.getByText('M-3').waitFor();

  // A link with the address, the external-link icon inside; none without one.
  const link = linked.getByRole('link');
  assert.equal(await link.getAttribute('href'), 'https://github.com/acme/app/milestone/3');
  assert.equal(await link.getAttribute('target'), '_blank');
  assert.equal(await link.getAttribute('rel'), 'noreferrer');
  assert.equal(await link.getAttribute('title'), 'Ouvrir M-3 dans le tracker distant');
  assert.equal(await plain.getByRole('link').count(), 0);
  assert.equal(await plain.getByText('M-1', { exact: true }).count(), 1);

  // The button copies the key alone, swaps its icon for a check, toasts.
  await plain.getByRole('button', { name: 'Copier M-1' }).click();
  await page.waitForFunction(() => window.clip.length === 1);
  assert.deepEqual(await page.evaluate(() => window.clip), ['M-1']);
  assert.deepEqual(await page.evaluate(() => window.toasts), [['success', 'Identifiant copié', 'M-1 a été copié dans le presse-papiers.']]);
  await plain.locator('svg.text-emerald-400').waitFor();
  assert.deepEqual(await parent(), { click: 0, mousedown: 0, dragstart: 0 }, 'the copy click stays in the component');
  await plain.locator('svg.text-emerald-400').waitFor({ state: 'detached', timeout: 4000 });

  // A refused clipboard: error toast, no check.
  await reset();
  await page.evaluate(() => { window.refuseClipboard = true; });
  await linked.getByRole('button', { name: 'Copier M-3' }).click();
  await page.waitForFunction(() => window.toasts.length === 1);
  assert.equal((await page.evaluate(() => window.toasts))[0][1], 'Copie impossible');
  assert.equal(await linked.locator('svg.text-emerald-400').count(), 0);
  await page.evaluate(() => { window.refuseClipboard = false; });

  // The link opens through the browser, and its click goes no further.
  await reset();
  await link.evaluate(a => a.addEventListener('click', e => e.preventDefault()));
  await link.click();
  await link.click({ modifiers: ['ControlOrMeta'] });
  assert.deepEqual(await parent(), { click: 0, mousedown: 0, dragstart: 0 });

  // The row body still answers.
  await linked.getByText('row body').click();
  assert.equal((await parent()).click, 1, 'a click elsewhere on the row reaches it');

  // Real drags from the button and the link start no drag of the row.
  await reset();
  const drop = page.locator('[data-drop]');
  await plain.getByRole('button', { name: 'Copier M-1' }).dragTo(drop);
  await link.dragTo(drop);
  assert.equal((await parent()).dragstart, 0, 'no drag of the row begins on the key');
  await linked.getByText('row body').dragTo(drop);
  assert.equal((await parent()).dragstart, 1, 'a drag of the row body still starts');

  assert.deepEqual(errors, []);
  console.log('macro-key-ref: ok');
} finally {
  await browser?.close();
  await server.close();
}
