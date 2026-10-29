import { chmodSync, cpSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const lock = JSON.parse(readFileSync(join(root, 'package-lock.json'), 'utf8'));
const output = join(root, 'third_party_licenses/javascript');
const packages = Object.entries(lock.packages)
  .filter(([path, pkg]) => path.startsWith('node_modules/') && !pkg.dev && pkg.license)
  .sort(([left], [right]) => left.localeCompare(right));

rmSync(output, { recursive: true, force: true });
mkdirSync(output, { recursive: true });

const lines = [
  '# JavaScript production dependency notices',
  '',
  'These are the production npm packages included in the Tauri webview bundle. Package names, exact locked versions, and SPDX license expressions come from `package-lock.json`. License files are copied from each installed package.',
  '',
  '| Package | Version | Declared license | License text |',
  '| --- | --- | --- | --- |',
];

for (const [lockPath, pkg] of packages) {
  const name = lockPath.slice('node_modules/'.length);
  const source = join(root, lockPath);
  const destination = join(output, name);
  const licenseFiles = readdirSync(source, { withFileTypes: true })
    .filter(entry => entry.isFile() && /^(LICENSE|COPYING)/i.test(entry.name) && entry.name.toLowerCase() !== 'license.spdx')
    .map(entry => entry.name)
    .sort();

  if (licenseFiles.length === 0) {
    throw new Error(`No package license file found for ${name}`);
  }

  mkdirSync(destination, { recursive: true });
  for (const file of licenseFiles) {
    const copied = join(destination, file);
    cpSync(join(source, file), copied);
    chmodSync(copied, 0o644);
  }
  const links = licenseFiles
    .map(file => '[`' + file + '`](third_party_licenses/javascript/' + name + '/' + file + ')')
    .join(', ');
  lines.push('| `' + name + '` | `' + pkg.version + '` | `' + pkg.license + '` | ' + links + ' |');
}

writeFileSync(join(root, 'THIRD_PARTY_NOTICES_JS.md'), `${lines.join('\n')}\n`);
console.log(`Recorded ${packages.length} JavaScript production dependencies.`);
