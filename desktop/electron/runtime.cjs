const path = require('node:path')
const fs = require('node:fs')

function agentName(platform = process.platform) {
 return platform === 'win32' ? 'sectile-agent.exe' : 'sectile-agent'
}

function resolveAgentBinary({packaged, resourcesPath, directory, platform = process.platform}) {
 const name = agentName(platform)
 const candidates = packaged
  ? [path.join(resourcesPath, name)]
  : [path.resolve(directory, '../bin', name), path.resolve(directory, '../../bin', name)]
 const binary = candidates.find(candidate => fs.existsSync(candidate))
 if (!binary) throw Error('The bundled Sectile agent is missing. Rebuild or reinstall the app.')
 return binary
}

module.exports = {agentName, resolveAgentBinary}
