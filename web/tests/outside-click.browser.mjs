// Isolated browser regression for #266: the real useBackdropDismiss hook, driven
// with a real mouse, on two stacked dialogs.
// Run with Playwright available: node tests/outside-click.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
// Normalised to forward slashes: Vite reports plugin ids that way, including
// on Windows, and the fixture below is matched on that id.
const root = fileURLToPath(new URL('..', import.meta.url)).replace(/\\/g, '/').replace(/\/$/, '');

// The inner dialog is rendered inside the outer one, the way the expanded
// specification reader lives inside TaskDetailModal: one click must still close
// one dialog.
const harness = `import React from 'react'; import {createRoot} from 'react-dom/client'; import {useBackdropDismiss} from '/src/hooks/useBackdropDismiss.ts';
window.dismissed=[];
const backdropStyle=(z)=>({position:'fixed',inset:0,zIndex:z,display:'flex',alignItems:'center',justifyContent:'center',background:'rgba(0,0,0,0.6)'});
function Dialog({name,z,children}){
  const backdrop=useBackdropDismiss(()=>window.dismissed.push(name));
  return <div data-testid={name+'-backdrop'} style={backdropStyle(z)} {...backdrop}>
    <div data-testid={name+'-panel'} style={{width:320,height:220,background:'#fff'}}>{children}</div>
  </div>;
}
window.stacked=false;
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<Dialog name="outer" z={50}>{window.stacked?<Dialog name="inner" z={60}/>:null}</Dialog>);
window.render();`;

const server = await createServer({
  root,
  configFile: root + '/vite.config.ts',
  server: { port: 0, host: '127.0.0.1' },
  plugins: [{
    name: 'fixture',
    enforce: 'pre',
    configureServer(s) {
      s.middlewares.use(async (req, res, next) => {
        if (req.url === '/fixture') {
          res.setHeader('Content-Type', 'text/html');
          res.end(await s.transformIndexHtml('/fixture', '<div id="root"></div><script type="module" src="/fixture.tsx"></script>'));
        } else next();
      });
    },
    // The React plugin re-imports the module by its absolute id for Fast
    // Refresh, so both spellings have to resolve to the virtual fixture.
    resolveId(id) { if (id === '/fixture.tsx' || id === root + '/fixture.tsx') return root + '/fixture.tsx' },
    load(id) { if (id === root + '/fixture.tsx') return harness },
  }],
});
await server.listen();

let browser;
try {
  browser = await chromium.launch({ headless: true, channel: 'chrome' });
  const page = await browser.newPage({ viewport: { width: 900, height: 700 } });
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`);
  await page.getByTestId('outer-panel').waitFor();

  const dismissed = () => page.evaluate(() => window.dismissed);
  const reset = () => page.evaluate(() => { window.dismissed = [] });
  const centre = async (testId) => {
    const box = await page.getByTestId(testId).boundingBox();
    return { x: box.x + box.width / 2, y: box.y + box.height / 2 };
  };
  // A point on the backdrop, well clear of the centred panel.
  const beside = { x: 40, y: 40 };
  const drag = async (from, to) => {
    await page.mouse.move(from.x, from.y);
    await page.mouse.down();
    await page.mouse.move(to.x, to.y);
    await page.mouse.up();
  };

  const panel = await centre('outer-panel');

  // A press and a release beside the dialog close it.
  await drag(beside, beside);
  assert.deepEqual(await dismissed(), ['outer'], 'a click beside the dialog closes it');
  await reset();

  // A click inside the dialog changes nothing.
  await drag(panel, panel);
  assert.deepEqual(await dismissed(), [], 'a click inside the dialog leaves it open');

  // A selection started inside the dialog and released on the backdrop reports
  // the backdrop as the click target: the press is what tells them apart.
  await drag(panel, beside);
  assert.deepEqual(await dismissed(), [], 'a drag out of the dialog leaves it open');

  // And the other way round.
  await drag(beside, panel);
  assert.deepEqual(await dismissed(), [], 'a drag into the dialog leaves it open');

  // Stacked: the click lands on the top dialog, which closes alone.
  await page.evaluate(() => { window.stacked = true; window.render() });
  await page.getByTestId('inner-panel').waitFor();
  await drag(beside, beside);
  assert.deepEqual(await dismissed(), ['inner'], 'one click closes exactly one layer');

  assert.deepEqual(errors, [], 'no page error');
  console.log('outside-click.browser.mjs: ok');
} finally {
  await browser?.close();
  await server.close();
}
