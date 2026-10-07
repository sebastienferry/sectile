import test from 'node:test'
import assert from 'node:assert/strict'
import { chosenFolder, folderRoleLabel, menuFolders } from '../src/folder-menu.mjs'

test('a run offers its folders only when it has more than one', () => {
 const two = [{ path: '/wt', name: 'app', role: 'primary' }, { path: '/docs', name: 'docs', role: 'context' }]
 assert.deepEqual(menuFolders({ folders: two }), two)
 assert.deepEqual(menuFolders({ folders: two.slice(0, 1) }), [])
 assert.deepEqual(menuFolders({ directory: '/wt' }), [], 'an older agent sends no list')
 assert.deepEqual(menuFolders({ folders: 'nope' }), [])
 assert.deepEqual(menuFolders(null), [])
 assert.deepEqual(menuFolders({ folders: [two[0], { name: 'empty' }, null] }), [], 'a folder without a path is not offered')
})

test('each folder says its role', () => {
 assert.equal(folderRoleLabel({ role: 'primary' }), 'primary')
 assert.equal(folderRoleLabel({ role: 'changed' }), 'changed')
 assert.equal(folderRoleLabel({ role: 'context' }), 'context')
 assert.equal(folderRoleLabel({ role: 'spec' }), 'specifications')
 assert.equal(folderRoleLabel({ role: 'local' }), 'attached')
 assert.equal(folderRoleLabel({ role: 'context', attached: true }), 'attached')
 assert.equal(folderRoleLabel({ role: 'future' }), 'future')
})

test('a chosen folder is one the run lists, other than its own directory', () => {
 const folders = [{ path: '/wt', name: 'app', role: 'primary' }, { path: '/docs', name: 'docs', role: 'context' }]
 const run = { directory: '/wt', folders }
 assert.equal(chosenFolder(run, '/docs'), folders[1])
 assert.equal(chosenFolder(run, '/wt'), null, 'the primary folder is the run directory')
 assert.equal(chosenFolder(run, '/gone'), null, 'a folder that left the list')
 assert.equal(chosenFolder(run, undefined), null)
 assert.equal(chosenFolder({ directory: '/wt', folders: folders.slice(0, 1) }, '/wt'), null)
 assert.equal(chosenFolder({ directory: '/wt' }, '/docs'), null, 'an older agent sends no list')
})
