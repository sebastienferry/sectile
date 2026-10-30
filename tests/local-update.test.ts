import assert from 'node:assert/strict';
import test from 'node:test';
import { createServer } from 'node:http';
import { mkdtemp, mkdir, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { stopLocalRuntime } from '../scripts/stop-local-runtime.mjs';

async function fixture(t, options = {}) {
  const root = await mkdtemp(join(tmpdir(), 'sectile-local-update-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const active = join(root, 'generations/companion-v1');
  await mkdir(active, { recursive: true });
  const key = 'fixture-installation-key-'.repeat(2);
  const token = 'fixture-agent-token';
  const requests = [];
  const server = createServer((req, res) => {
    assert.equal(req.headers.authorization, `Bearer ${key}`);
    res.setHeader('Content-Type', 'application/json');
    res.end(JSON.stringify({ protocol: 1, companion: options.companion !== false, ready: true }));
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${server.address().port}`;
  const agent = createServer((req, res) => {
    requests.push(`${req.method} ${req.url}`);
    assert.equal(req.headers.authorization, `Bearer ${token}`);
    if (req.url === '/desktop/status') {
      if (options.redirect) {
        res.writeHead(302, { Location: origin + '/unexpected' });
        res.end();
      } else {
        res.setHeader('Content-Type', 'application/json');
        res.end(JSON.stringify({ server: options.wrongServer ? 'https://fleet.example.invalid' : origin }));
      }
    } else {
      res.writeHead(options.busy ? 409 : 204);
      res.end();
      if (!options.busy && !options.stayAlive) {
        server.close(); server.closeAllConnections();
        agent.close(); agent.closeIdleConnections();
      }
    }
  });
  await new Promise(resolve => agent.listen(0, '127.0.0.1', resolve));
  t.after(() => { server.close(); server.closeAllConnections(); agent.close(); agent.closeAllConnections(); });
  const save = (name, value) => writeFile(join(root, name), JSON.stringify(value));
  await save('runtime-mode.json', { protocol: 1, mode: 'local' });
  await save('runtime-selection.json', { protocol: 1, kind: 'companion', generation: 'generations/companion-v1' });
  await writeFile(join(active, 'installation.json'), JSON.stringify({ port: server.address().port, key }));
  const discovery = join(active, 'agent-connection.json');
  await writeFile(discovery, JSON.stringify({ url: `http://127.0.0.1:${agent.address().port}`, token }));
  return { root, requests, save, discovery };
}

test('Local update stops only the authenticated selected companion and waits for both transports', async t => {
  const f = await fixture(t);
  await stopLocalRuntime(f.root, { timeoutMs: 2000, pollMs: 5 });
  assert.equal(f.requests.filter(item => item === 'POST /desktop/shutdown').length, 1);
});

test('Local update preserves active executions when shutdown is refused', async t => {
  const f = await fixture(t, { busy: true });
  await assert.rejects(stopLocalRuntime(f.root), /exécutions et préparations actives/);
  assert.equal(f.requests.filter(item => item === 'POST /desktop/shutdown').length, 1);
});

test('Local update refuses Fleet mode without contacting its agent', async t => {
  const f = await fixture(t);
  await f.save('runtime-mode.json', { protocol: 1, mode: 'fleet' });
  await assert.rejects(stopLocalRuntime(f.root), /réservée au mode Local/);
  assert.deepEqual(f.requests, []);
});

test('Local update does not fall back from an unknown selected generation', async t => {
  const f = await fixture(t);
  await f.save('runtime-selection.json', { protocol: 1, kind: 'companion', generation: '../../other' });
  await assert.rejects(stopLocalRuntime(f.root), /Sélection du service incompatible/);
  assert.deepEqual(f.requests, []);
});

test('Local update rejects remote discovery before sending credentials', async t => {
  const f = await fixture(t);
  await writeFile(f.discovery, JSON.stringify({ url: 'https://example.invalid', token: 'private' }));
  await assert.rejects(stopLocalRuntime(f.root), /Adresse du compagnon invalide/);
  assert.deepEqual(f.requests, []);
});

test('Local update refuses a standalone or mismatched agent without requesting shutdown', async t => {
  for (const options of [{ companion: false }, { wrongServer: true }, { redirect: true }]) {
    const f = await fixture(t, options);
    await assert.rejects(stopLocalRuntime(f.root));
    assert.ok(!f.requests.includes('POST /desktop/shutdown'));
  }
});

test('Local update does not interpret an acknowledgement as a completed stop', async t => {
  const f = await fixture(t, { stayAlive: true });
  await assert.rejects(stopLocalRuntime(f.root, { timeoutMs: 80, pollMs: 5 }), /Arrêt incomplet/);
});

test('Local update requires an explicit absolute data directory', async () => {
  await assert.rejects(stopLocalRuntime('local-runtime'), /chemin absolu/);
});


test('Local update refuses a null selection rather than falling back to the source root', async t => {
  const f = await fixture(t);
  await f.save('runtime-selection.json', null);
  await assert.rejects(stopLocalRuntime(f.root), /Sélection du service incompatible/);
  assert.deepEqual(f.requests, []);
});
