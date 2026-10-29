// Opt-in real tracker smoke test. Set SECTILE_TEST_SERVER_BINARY and GITLAB_TOKEN.
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { mkdtemp, mkdir, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const binary = process.env.SECTILE_TEST_SERVER_BINARY;
assert.ok(binary, 'Build a disposable server on a free test-only local runtime port');
assert.ok(process.env.GITLAB_TOKEN, 'GITLAB_TOKEN is required for the authorized GitLab test');
const ghToken = spawnSync('gh', ['auth', 'token'], { encoding: 'utf8' });
assert.equal(ghToken.status, 0, 'Authenticate gh before the authorized GitHub test');
const portInfo = spawnSync(binary, ['--local-runtime-info'], { encoding: 'utf8' });
assert.equal(portInfo.status, 0);
const { port } = JSON.parse(portInfo.stdout);
const origin = `http://127.0.0.1:${port}`;
const data = await mkdtemp(join(tmpdir(), 'sectile-live-trackers-'));
const marker = randomUUID().slice(0, 12);
const title = provider => `[Sectile V1 integration test] ${provider} ${marker}`;
const updated = provider => `${title(provider)} updated`;
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
const cli = (command, args, env = process.env) => {
  const result = spawnSync(command, args, { encoding: 'utf8', env, timeout: 20000 });
  if (result.status !== 0) throw new Error(`${command} ${args[0]} failed: ${result.stderr.slice(0, 500)}`);
  return JSON.parse(result.stdout);
};
const gitlabPath = 'projects/85844068/issues';
const gitlab = args => cli('glab', ['api', ...args]);
const github = args => cli('gh', ['api', ...args]);
const created = {};
let child;
let token = '';
async function request(route, method = 'GET', body, headers = {}) {
  const response = await fetch(origin + route, {
    method, headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json', ...headers },
    body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(20000),
  });
  const value = await response.json();
  if (!response.ok) throw new Error(`${method} ${route}: HTTP ${response.status} ${JSON.stringify(value).slice(0, 500)}`);
  return value;
}
async function until(check, maxMs = 60000) {
  const deadline = Date.now() + maxMs;
  let last;
  while (Date.now() < deadline) {
    try { return await check(); } catch (error) { last = error; await delay(500); }
  }
  throw last ?? new Error('Timed out');
}
async function cleanRemote(provider) {
  if (provider === 'github') {
    const issues = created.github ? [{ number: created.github, title: title(provider) }] : cli('gh', ['issue', 'list', '--repo', 'sebastienferry/sectile', '--state', 'all', '--search', marker, '--json', 'number,title']);
    for (const issue of issues.filter(item => item.title.includes(marker))) {
      const found = github([`repos/sebastienferry/sectile/issues/${issue.number}`]);
      github(['graphql', '-f', 'query=mutation($id:ID!){deleteIssue(input:{issueId:$id}){clientMutationId}}', '-F', `id=${found.node_id}`]);
      await until(async () => {
        const result = spawnSync('gh', ['api', `repos/sebastienferry/sectile/issues/${issue.number}`], { encoding: 'utf8' });
        assert.equal(result.status, 1);
      }, 10000);
      console.log(`GitHub #${issue.number}: deleted`);
    }
  } else {
    const issues = created.gitlab ? [{ iid: created.gitlab, title: title(provider) }] : gitlab([`${gitlabPath}?search=${encodeURIComponent(marker)}&scope=all&state=all`]);
    for (const issue of issues.filter(item => item.title.includes(marker))) {
      const deletion = spawnSync('glab', ['api', '-X', 'DELETE', `${gitlabPath}/${issue.iid}`], { encoding: 'utf8', timeout: 20000 });
      assert.equal(deletion.status, 0, `GitLab deletion failed: ${deletion.stderr.slice(0, 500)}`);
      await until(async () => {
        const result = spawnSync('glab', ['api', `${gitlabPath}/${issue.iid}`], { encoding: 'utf8' });
        assert.notEqual(result.status, 0);
      }, 10000);
      console.log(`GitLab !${issue.iid}: deleted`);
    }
  }
}
try {
  await mkdir(join(data, 'server'));
  const localKey = randomUUID();
  child = spawn(binary, [], { cwd: data, stdio: 'ignore', env: {
    ...process.env, HOME: data, PORT: String(port), SECTILE_SERVER_HOST: '127.0.0.1',
    SECTILE_SERVER_DATA_DIR: join(data, 'server'), DB_PATH: join(data, 'server/tasks.db'),
    SECTILE_LOCAL_RUNTIME_KEY: localKey,
  } });
  await until(() => request('/api/local-runtime', 'GET', undefined, { Authorization: `Bearer ${localKey}` }), 30000);
  const signIn = await fetch(origin + '/auth/local', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email: 'live-tracker-fixture@example.invalid' }) });
  assert.equal(signIn.status, 200);
  const cookie = signIn.headers.get('set-cookie').split(';')[0];
  const code = await request('/api/pairing-codes', 'POST', {}, { Cookie: cookie });
  const pair = await request('/api/v1/agent/pair', 'POST', { code: code.code, label: 'Live tracker fixture' });
  token = pair.token;
  for (const provider of ['github', 'gitlab']) {
    const siteUrl = provider === 'github' ? 'https://api.github.com' : 'https://gitlab.com/api/v4';
    const scope = provider === 'github' ? 'sebastienferry/sectile' : 'smartadserver/private/shared-platform/taskativ';
    const credential = provider === 'github' ? ghToken.stdout.trim() : process.env.GITLAB_TOKEN;
    await request('/api/me/tracker-credentials', 'PUT', { tracker: provider, siteUrl, token: credential });
    const tracker = await request('/api/admin/trackers', 'POST', { name: `${provider} fixture`, provider, site: siteUrl, scope, autoSyncEnabled: false });
    const project = await request('/api/projects', 'POST', { name: `${provider} fixture`, trackers: [{ trackerId: tracker.id }], defaultTrackerId: tracker.id });
    try {
      const task = await request('/api/tasks', 'POST', { projectId: project.id, trackerId: tracker.id, title: title(provider), description: 'Temporary Sectile V1 integration test; automatically deleted.', requireRemoteCreation: true });
      const issueNumber = Number(task.key.match(/\d+$/)?.[0]);
      assert.ok(issueNumber, `Missing remote issue number: ${task.key}`);
      created[provider] = issueNumber;
      const remote = () => provider === 'github' ? github([`repos/sebastienferry/sectile/issues/${issueNumber}`]) : gitlab([`${gitlabPath}/${issueNumber}`]);
      assert.equal(remote().title, title(provider));
      await request(`/api/tasks/${task.id}`, 'PATCH', { title: updated(provider) });
      await until(async () => {
        const activities = await request(`/api/tasks/${task.id}/activities`);
        const sync = activities.find(item => item.skillId === 'tracker_update');
        assert.ok(sync, 'Tracker update activity missing');
        assert.equal(sync.status, 'completed', `Tracker update status: ${sync.status} ${sync.error ?? ''}`);
        assert.equal(remote().title, updated(provider));
      });
      console.log(`${provider}: create, queued update and direct remote read passed (${task.key})`);
    } finally {
      await cleanRemote(provider);
    }
  }
} finally {
  if (child && child.exitCode === null) {
    child.kill('SIGTERM');
    await Promise.race([new Promise(resolve => child.once('exit', resolve)), delay(3000)]);
    if (child.exitCode === null) child.kill('SIGKILL');
  }
  await rm(data, { recursive: true, force: true });
}
