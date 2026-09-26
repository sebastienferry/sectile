const path = require('node:path')
const {agentName} = require('./runtime.cjs')

// The targets a release packages. The tokens are Electron's; the release
// scripts map Go's `windows` and `amd64` onto them.
const platforms = ['darwin', 'linux', 'win32']
const archs = ['arm64', 'x64']

const desktopRoot = path.resolve(__dirname, '..')

// packageOptions builds the @electron/packager options for one target. With no
// target it is the host package `make desktop-package` has always produced:
// packager then picks the host platform and arch itself. The agent is the file
// shipped as an extra resource, so it must already carry the name the packaged
// app resolves, the target's agentName().
function packageOptions({platform, arch, agent, out} = {}) {
 if (platform !== undefined && !platforms.includes(platform)) throw Error(`Unknown platform ${platform}, expected one of ${platforms.join(', ')}`)
 if (arch !== undefined && !archs.includes(arch)) throw Error(`Unknown arch ${arch}, expected one of ${archs.join(', ')}`)
 const options = {
  dir: desktopRoot,
  name: 'Sectile',
  out: out ? path.resolve(out) : path.join(desktopRoot, 'release'),
  overwrite: true,
  icon: path.join(desktopRoot, 'assets/icon'),
  extraResource: agent ? path.resolve(agent) : path.join(desktopRoot, 'bin', agentName(platform)),
  ignore: /^\/(bin|tests|release[^/]*)(\/|$)/,
 }
 if (platform !== undefined) options.platform = platform
 if (arch !== undefined) options.arch = arch
 return options
}

module.exports = {archs, packageOptions, platforms}
