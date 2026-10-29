import { createHash } from 'node:crypto';
import { execFileSync, spawnSync } from 'node:child_process';
import { mkdirSync, readFileSync, renameSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { resolve, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const source = resolve(process.env.SECTILE_SOURCE_DIR || join(root, '../sectile'));
const output = join(root, 'src-tauri/binaries');
const staging = join(output, `.build-${process.pid}`);
const services = ['server', 'agent', 'companion'];
const extension = process.platform === 'win32' ? '.exe' : '';

function git(args) {
  return execFileSync('git', args, { cwd: source, encoding: 'utf8', windowsHide: true }).trim();
}

function sha256(path) {
  return createHash('sha256').update(readFileSync(path)).digest('hex');
}

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { windowsHide: true, ...options });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
  return result;
}

const commit = git(['rev-parse', 'HEAD']);
const date = git(['show', '-s', '--format=%cI', 'HEAD']);
const dirty = git(['status', '--porcelain']).length > 0;
const tagResult = spawnSync('git', ['describe', '--exact-match', '--tags', 'HEAD'], {
  cwd: source,
  encoding: 'utf8',
  windowsHide: true,
});
const tag = tagResult.status === 0 ? tagResult.stdout.trim() : '';
const expectedCommit = process.env.SECTILE_SOURCE_COMMIT?.trim();
if (expectedCommit && !commit.startsWith(expectedCommit)) {
  throw new Error(`Go source commit ${commit} does not match SECTILE_SOURCE_COMMIT=${expectedCommit}`);
}
if (process.env.SECTILE_REQUIRE_CLEAN_SOURCE === '1' && dirty) {
  throw new Error('Go source tree is dirty; refusing the requested clean-source build.');
}

const version = tag || 'dev';
const releaseEligible = Boolean(tag) && !dirty;
const ldflags = [
  `-X tasks/internal/version.Version=${version}`,
  `-X tasks/internal/version.Commit=${commit}`,
  `-X tasks/internal/version.Date=${date}`,
].join(' ');

rmSync(staging, { recursive: true, force: true });
mkdirSync(staging, { recursive: true });
try {
  const artifacts = [];
  for (const service of services) {
    const name = `sectile-${service}${extension}`;
    const path = join(staging, name);
    run('go', ['build', '-trimpath', '-ldflags', ldflags, '-o', path, `./cmd/${service}`], {
      cwd: source,
      stdio: 'inherit',
    });
    const binaryVersion = run(path, ['--version'], { cwd: source, encoding: 'utf8' }).stdout.trim();
    const expectedVersion = `sectile-${service} ${version} (${commit.slice(0, 12)})`;
    if (binaryVersion !== expectedVersion) {
      throw new Error(`${name} reported ${JSON.stringify(binaryVersion)}; expected ${JSON.stringify(expectedVersion)}`);
    }
    artifacts.push({
      name,
      sha256: sha256(path),
      size: statSync(path).size,
      version: binaryVersion,
    });
  }

  const manifest = {
    schemaVersion: 1,
    source: { commit, date, tag: tag || null, dirty },
    releaseEligible,
    services: artifacts,
  };
  writeFileSync(join(staging, 'build-manifest.json'), `${JSON.stringify(manifest, null, 2)}\n`);

  mkdirSync(output, { recursive: true });
  const names = [...artifacts.map(({ name }) => name), 'build-manifest.json'];
  const backedUp = [];
  const installed = [];
  try {
    for (const name of names) {
      const current = join(output, name);
      const backup = join(staging, `${name}.previous`);
      try {
        renameSync(current, backup);
        backedUp.push({ current, backup });
      } catch (error) {
        if (error.code !== 'ENOENT') throw error;
      }
    }
    for (const name of names) {
      const current = join(output, name);
      renameSync(join(staging, name), current);
      installed.push(current);
    }
  } catch (error) {
    for (const path of installed) rmSync(path, { force: true });
    for (const { current, backup } of backedUp.reverse()) renameSync(backup, current);
    throw error;
  }
  console.log(`Prepared ${services.length} services from ${commit} (${dirty ? 'dirty' : 'clean'}${tag ? `, ${tag}` : ''}).`);
} finally {
  rmSync(staging, { recursive: true, force: true });
}
