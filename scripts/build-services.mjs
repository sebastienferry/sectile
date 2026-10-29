import { createHash } from 'node:crypto';
import { execFileSync, spawnSync } from 'node:child_process';
import { chmodSync, cpSync, copyFileSync, mkdirSync, readFileSync, readdirSync, renameSync, rmSync, statSync, writeFileSync } from 'node:fs';
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

function makeTreeWritable(path) {
  const info = statSync(path);
  if (info.isDirectory()) {
    chmodSync(path, 0o755);
    for (const entry of readdirSync(path)) makeTreeWritable(join(path, entry));
  } else {
    chmodSync(path, 0o644);
  }
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

const goNoticesPath = join(source, 'THIRD_PARTY_NOTICES.md');
const goLicensePath = join(source, 'third_party_licenses');
const goNotices = readFileSync(goNoticesPath, 'utf8');
const expectedGoModHash = goNotices.match(/`go\.mod` SHA-256 is `([a-f0-9]{64})`/)?.[1];
const actualGoModHash = createHash('sha256').update(readFileSync(join(source, 'go.mod'))).digest('hex');
if (!expectedGoModHash || expectedGoModHash !== actualGoModHash) {
  throw new Error('Go third-party notices do not match the selected source go.mod dependency graph.');
}

const goNoticesForTauri = goNotices
  .replaceAll('third_party_licenses/', 'third_party_licenses/go/')
  .replace('Sectile source code in this repository is licensed under MIT; see [`LICENSE`](LICENSE).', 'Sectile Go source code is licensed under MIT; see [`third_party_licenses/go/sectile/LICENSE`](third_party_licenses/go/sectile/LICENSE).');
const localGoNoticesPath = join(root, 'THIRD_PARTY_NOTICES_GO.md');
if (readFileSync(localGoNoticesPath, 'utf8') !== goNoticesForTauri) {
  writeFileSync(localGoNoticesPath, goNoticesForTauri);
}
const localGoLicensePath = join(root, 'third_party_licenses/go');
rmSync(localGoLicensePath, { recursive: true, force: true });
cpSync(goLicensePath, localGoLicensePath, { recursive: true });
mkdirSync(join(localGoLicensePath, 'sectile'), { recursive: true });
copyFileSync(join(source, 'LICENSE'), join(localGoLicensePath, 'sectile/LICENSE'));
makeTreeWritable(localGoLicensePath);

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

  copyFileSync(goNoticesPath, join(staging, 'THIRD_PARTY_NOTICES.md'));
  cpSync(goLicensePath, join(staging, 'third_party_licenses'), { recursive: true });
  makeTreeWritable(join(staging, 'THIRD_PARTY_NOTICES.md'));
  makeTreeWritable(join(staging, 'third_party_licenses'));

  const manifest = {
    schemaVersion: 1,
    source: { commit, date, tag: tag || null, dirty },
    releaseEligible,
    services: artifacts,
  };
  writeFileSync(join(staging, 'build-manifest.json'), `${JSON.stringify(manifest, null, 2)}\n`);

  mkdirSync(output, { recursive: true });
  const names = [...artifacts.map(({ name }) => name), 'build-manifest.json', 'THIRD_PARTY_NOTICES.md', 'third_party_licenses'];
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
