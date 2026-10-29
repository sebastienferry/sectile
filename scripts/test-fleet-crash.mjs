// Opt-in live CLI qualification against disposable Fleet state.
// Run with SECTILE_TEST_REAL_CODEX=1 and/or SECTILE_TEST_REAL_CLAUDE=1.
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { mkdtemp, mkdir, readFile, rm, stat } from 'node:fs/promises';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const providers = [
  ...(process.env.SECTILE_TEST_REAL_CODEX === '1' ? ['codex'] : []),
  ...(process.env.SECTILE_TEST_REAL_CLAUDE === '1' ? ['claude'] : []),
];
assert.ok(providers.length, 'Select a signed-in provider with SECTILE_TEST_REAL_CODEX=1 or SECTILE_TEST_REAL_CLAUDE=1');
assert.notEqual(process.platform, 'win32', 'This SIGKILL qualification requires a Unix process host');
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const binary = name => process.env[`SECTILE_TEST_${name.toUpperCase()}_BINARY`] ?? join(root, 'src-tauri/binaries', `sectile-${name}${process.platform === 'win32' ? '.exe' : ''}`);
const children = [];
const home = process.env.HOME;
assert.ok(home, 'The signed-in CLI home directory is required');
const baseEnv = Object.fromEntries(['PATH', 'SYSTEMROOT', 'WINDIR', 'TMP', 'TEMP', 'USER', 'CODEX_HOME'].filter(key => process.env[key]).map(key => [key, process.env[key]]));
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
async function until(check, timeoutMs = 120000) {
  const deadline = Date.now() + timeoutMs;
  let last;
  while (Date.now() < deadline) {
    try { return await check(); } catch (error) { last = error; await delay(250); }
  }
  throw last ?? new Error('Timed out waiting for the Fleet fixture');
}
async function fetchJSON(origin, route, method = 'GET', body, headers = {}) {
  const response = await fetch(origin + route, {
    method,
    headers: { 'Content-Type': 'application/json', ...headers },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(15000),
  });
  assert.ok(response.ok, `${method} ${route}: HTTP ${response.status}`);
  return response.status === 204 ? null : response.json();
}
const metadata = spawnSync(binary('server'), ['--local-runtime-info'], { encoding: 'utf8', windowsHide: true });
assert.equal(metadata.status, 0, 'The test server must report its fixed Local callback port');
const port = JSON.parse(metadata.stdout).port;
const reservation = createServer();
await new Promise((resolve, reject) => {
  reservation.once('error', () => reject(new Error(`The test server port ${port} is occupied; build a disposable Go server with a different localruntime.Port and localruntime.Origin.`)));
  reservation.listen(port, '127.0.0.1', resolve);
});
await new Promise(resolve => reservation.close(resolve));
const data = await mkdtemp(join(tmpdir(), 'sectile-fleet-crash-'));
function start(name, args, extra) {
  const child = spawn(binary(name), args, { cwd: data, env: { ...baseEnv, ...extra }, stdio: 'ignore', windowsHide: true });
  children.push(child);
  return child;
}
async function exit(child, signal = 'SIGTERM') {
  if (child.exitCode !== null || child.signalCode !== null) return;
  child.kill(signal);
  await Promise.race([new Promise(resolve => child.once('exit', resolve)), delay(3000)]);
  if (child.exitCode === null && child.signalCode === null) child.kill('SIGKILL');
}
let agentProcess;
let connection;
try {
  await mkdir(join(data, 'server'));
  await mkdir(join(data, 'agent'));
  const workspace = join(data, 'workspace');
  await mkdir(workspace);
  assert.equal(spawnSync('git', ['init', workspace], { stdio: 'ignore', windowsHide: true }).status, 0);
  const origin = `http://127.0.0.1:${port}`;
  const localKey = randomUUID();
  start('server', [], {
    HOME: data, USERPROFILE: data, PORT: String(port), SECTILE_SERVER_HOST: '127.0.0.1',
    SECTILE_SERVER_DATA_DIR: join(data, 'server'), DB_PATH: join(data, 'server/tasks.db'),
    SECTILE_LOCAL_RUNTIME_KEY: localKey,
  });
  await until(() => fetchJSON(origin, '/api/local-runtime', 'GET', undefined, { Authorization: `Bearer ${localKey}` }), 30000);
  const signIn = await fetch(origin + '/auth/local', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email: 'fleet-crash@example.invalid' }) });
  assert.equal(signIn.status, 200);
  const cookie = signIn.headers.get('set-cookie').split(';')[0];
  const code = await fetchJSON(origin, '/api/pairing-codes', 'POST', {}, { Cookie: cookie });
  const pair = await fetchJSON(origin, '/api/v1/agent/pair', 'POST', { code: code.code, label: 'Disposable Fleet crash fixture' });
  const apiHeaders = { Authorization: `Bearer ${pair.token}` };
  const project = await fetchJSON(origin, '/api/projects', 'POST', { name: 'Fleet crash fixture', trackers: [], folderOnly: true }, apiHeaders);
  const discovery = join(data, 'agent-connection.json');
  async function startAgent() {
    await rm(discovery, { force: true });
    agentProcess = start('agent', ['--url', origin, '--project', 'all', '--desktop-info', discovery], {
      HOME: home, USERPROFILE: process.env.USERPROFILE ?? home,
      TOKEN: pair.token, SECTILE_PAIRED_DEVICE_ID: pair.deviceId,
      SECTILE_DESKTOP_DATA_DIR: join(data, 'agent'), SECTILE_DISABLE_MCP_REFRESH: '1',
    });
    connection = await until(async () => {
      const found = JSON.parse(await readFile(discovery, 'utf8'));
      const status = await fetchJSON(found.url, '/desktop/status', 'GET', undefined, { Authorization: `Bearer ${found.token}` });
      assert.equal(status.server, origin);
      assert.equal(status.connected, true);
      return found;
    }, 30000);
  }
  async function agent(route, method = 'GET', body) {
    return fetchJSON(connection.url, route, method, body, { Authorization: `Bearer ${connection.token}` });
  }
  await startAgent();
  await agent('/desktop/projects', 'POST', { projectId: project.id, path: workspace, folderOnly: true });
  for (const provider of providers) {
    const model = provider === 'codex' ? 'gpt-6-astra' : 'sonnet';
    const mode = provider === 'codex' ? 'workspace-write' : 'acceptEdits';
    const canary = `CRASH_CONTEXT_${provider.toUpperCase()}`;
    const marker = join(workspace, `${provider}-started.txt`);
    const delayed = join(workspace, `${provider}-delayed.txt`);
    const command = `printf STARTED > ${provider}-started.txt; sleep 90; printf FAILED > ${provider}-delayed.txt`;
    const chat = await agent('/desktop/consoles', 'POST', { projectId: project.id, provider, view: 'conversation' });
    const route = `/desktop/conversation?id=${chat.id}`;
    async function snapshot() {
      const value = await agent(route);
      for (const approval of value.approvals ?? []) {
        assert.equal(approval.tool, 'Bash', 'Only the test shell command may require approval');
        assert.equal(approval.input?.command?.trim(), command, 'Unexpected shell command requested');
        await agent(route, 'POST', { approval: { id: approval.id, decision: 'allow' } });
      }
      return value;
    }
    await agent(route, 'POST', { message: `Remember ${canary}. Reply exactly SESSION_READY without using tools.`, model, mode });
    await until(async () => {
      const value = await snapshot();
      assert.equal(value.busy, false);
      assert.ok((value.events ?? []).some(line => { const event = JSON.parse(line); return event.kind === 'assistant' && event.text?.includes('SESSION_READY'); }));
    });
    await agent(route, 'POST', { message: `Run exactly this shell command in the current folder, then wait for it to finish: ${command}. Do not run another command.` });
    await until(async () => {
      await snapshot();
      assert.equal(await readFile(marker, 'utf8'), 'STARTED');
    });
    const before = await snapshot();
    assert.equal(before.busy, true);
    const startedAt = (await stat(marker)).mtimeMs;
    const originalTools = (before.events ?? []).map(JSON.parse).filter(event => event.kind === 'tool' && JSON.stringify(event).includes(command));
    assert.ok(originalTools.length > 0, 'The original delayed command must be visible in the tool history');
    await exit(agentProcess, 'SIGKILL');
    console.log(`${provider}: agent killed during the tool; waiting past the 90-second marker.`);
    await delay(95000);
    await assert.rejects(readFile(delayed), error => error.code === 'ENOENT');
    await startAgent();
    const restored = await agent(route);
    assert.equal(restored.busy, false);
    assert.equal(restored.readOnly, false);
    assert.equal(restored.model, model);
    assert.equal(restored.mode, mode);
    assert.ok((restored.events ?? []).length >= (before.events ?? []).length);
    assert.equal((await stat(marker)).mtimeMs, startedAt, 'The original tool start marker was written again after restart');
    assert.equal((restored.events ?? []).map(JSON.parse).filter(event => event.kind === 'tool' && JSON.stringify(event).includes(command)).length, originalTools.length, 'The original tool call was replayed after restart');
    const firstMessage = (restored.events ?? []).map(JSON.parse).filter(event => event.kind === 'user' && event.text?.includes(canary));
    assert.equal(firstMessage.length, 1, 'The first turn must not be replayed');
    await agent(route, 'POST', { message: `Use a shell tool to check whether the delayed file from the interrupted turn exists. If absent, reply RESUME_OK and include the context word I asked you to remember. Do not write files.` });
    await until(async () => {
      const value = await snapshot();
      assert.equal(value.busy, false);
      assert.ok((value.events ?? []).some(line => { const event = JSON.parse(line); return event.kind === 'assistant' && event.text?.includes('RESUME_OK') && event.text?.includes(canary); }), 'The resumed answer lost its context or completion marker');
      assert.ok((value.events ?? []).some(line => { const event = JSON.parse(line); return event.kind === 'tool' && JSON.stringify(event).includes(`${provider}-delayed.txt`); }), 'The resumed turn did not check the delayed file with a tool');
    });
    assert.equal((await stat(marker)).mtimeMs, startedAt, 'The original tool was replayed during resume');
    await agent(`/desktop/stop?id=${chat.id}`, 'POST');
    console.log(`${provider}: restored history, model, mode and context; no delayed tool file or replay.`);
  }
} finally {
  for (const child of children.reverse()) await exit(child);
  await rm(data, { recursive: true, force: true });
}
