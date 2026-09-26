const {packager} = require('@electron/packager')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const {parseArgs} = require('node:util')
const {agentName} = require('./runtime.cjs')
const {packageOptions} = require('./package-options.cjs')

// node electron/package.cjs [--platform P] [--arch A] [--agent PATH] [--out DIR]
//
// With no argument, the host package in desktop/release/, from desktop/bin.
// A release names its target and hands over the agent it built for it, which
// is named sectile-agent-<os>-<arch>: the agent is copied under the name the
// target's packaged app looks for, executable, into a directory removed once
// packaging is over.
async function main() {
 const {values} = parseArgs({options: {
  platform: {type: 'string'},
  arch: {type: 'string'},
  agent: {type: 'string'},
  out: {type: 'string'},
 }})
 let staging = null
 try {
  let agent
  if (values.agent) {
   if (!fs.existsSync(values.agent)) throw Error(`Agent not found: ${values.agent}`)
   staging = fs.mkdtempSync(path.join(os.tmpdir(), 'sectile-package-'))
   agent = path.join(staging, agentName(values.platform))
   fs.copyFileSync(values.agent, agent)
   fs.chmodSync(agent, 0o755)
  }
  const options = packageOptions({platform: values.platform, arch: values.arch, agent, out: values.out})
  if (!fs.existsSync(options.extraResource)) throw Error(`Agent not found: ${options.extraResource}`)
  const paths = await packager(options)
  console.log(paths.join('\n'))
 } finally {
  if (staging) fs.rmSync(staging, {recursive: true, force: true})
 }
}

main().catch(error => {
 console.error(error.message || error)
 process.exitCode = 1
})
