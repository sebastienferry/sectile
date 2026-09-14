const {packager} = require('@electron/packager')
const path = require('node:path')
const {agentName} = require('./runtime.cjs')

packager({
 dir: path.resolve(__dirname, '..'),
 name: 'TaskFlow',
 out: path.resolve(__dirname, '../release'),
 overwrite: true,
 extraResource: path.resolve(__dirname, '../bin', agentName()),
 ignore: /^\/(bin|tests|release[^/]*)(\/|$)/,
}).then(paths => console.log(paths.join('\n'))).catch(error => {
 console.error(error)
 process.exitCode = 1
})
