import test from 'node:test'
import assert from 'node:assert/strict'
import {runFolderOutcome,offersRunFolder} from '../src/run-folders.mjs'

test('each outcome of adding a folder from a run says when the run sees it',()=>{
 assert.equal(runFolderOutcome('/n',{typed:false,appliesAt:'next-turn'}),'Attached /n: Claude sees it from your next message')
 assert.equal(runFolderOutcome('/n',{typed:true,appliesAt:'now'}),'Attached /n and typed /add-dir into the session')
 assert.equal(runFolderOutcome('/n',{typed:false,appliesAt:'next-launch'}),'Attached /n: the discussion sees it at its next launch')
 assert.equal(runFolderOutcome('/c',{mappedAs:'github.com/o/c',typed:true,appliesAt:'now'}),"/c is a checkout of github.com/o/c: it is now that repository's folder; typed /add-dir into the session")
})

test('only a live conversation or a running Sectile discussion offers the action',()=>{
 const discussion={skill:'discuss',status:'running',sessionId:'s'}
 assert.equal(offersRunFolder(discussion,true),true)
 assert.equal(offersRunFolder(discussion,false),false)
 assert.equal(offersRunFolder({...discussion,status:'completed'},true),false)
 assert.equal(offersRunFolder({...discussion,skill:'implement'},true),false)
 assert.equal(offersRunFolder({...discussion,externalTerminal:'iterm'},true),false)
 assert.equal(offersRunFolder({...discussion,externalTerminal:'iterm'},true,true),true)
 assert.equal(offersRunFolder({...discussion,headless:true},true),false)
 assert.equal(offersRunFolder({conversation:true,status:'running'},true),true)
 assert.equal(offersRunFolder({conversation:true,status:'canceled'},true),false)
 assert.equal(offersRunFolder(null,true),false)
})

test('a free console and a run detached to the native terminal offer it on an agent that serves them',()=>{
 const console={kind:'console',status:'running',sessionId:'c',provider:'claude'}
 assert.equal(offersRunFolder(console,true,true),true)
 assert.equal(offersRunFolder(console,true,false),false)
 assert.equal(offersRunFolder(console,false,true),false)
 assert.equal(offersRunFolder({...console,externalTerminal:'ghostty'},true,true),true)
 assert.equal(offersRunFolder({...console,status:'preparing'},true,true),false)
 assert.equal(offersRunFolder({...console,sessionId:''},true,true),false)
 assert.equal(offersRunFolder({...console,status:'completed'},true,true),false)
 assert.equal(offersRunFolder({skill:'discuss',status:'running',sessionId:'s',headless:true},true,true),false)
 assert.equal(offersRunFolder({skill:'implement',status:'running',sessionId:'s',externalTerminal:'ghostty'},true,true),false)
})

test('the outcome names the native terminal and what a free console waits for',()=>{
 assert.equal(runFolderOutcome('/n',{typed:true,appliesAt:'now'},{terminal:'Ghostty'}),'Attached /n and typed /add-dir into the session in Ghostty')
 assert.equal(runFolderOutcome('/n',{typed:true,appliesAt:'now'},{console:true}),'Attached /n and typed /add-dir into the session')
 assert.equal(runFolderOutcome('/n',{typed:false,appliesAt:'next-launch'},{console:true}),'Attached /n: a new Project prompt or a relaunch sees it')
 assert.equal(runFolderOutcome('/n',{typed:false,appliesAt:'next-launch'},{console:true,terminal:'iTerm'}),'Attached /n: a new Project prompt or a relaunch sees it')
 assert.equal(runFolderOutcome('/n',{typed:false,appliesAt:'next-launch'},{terminal:'iTerm'}),'Attached /n: the discussion sees it at its next launch')
 assert.equal(runFolderOutcome('/c',{mappedAs:'github.com/o/c',typed:true,appliesAt:'now'},{terminal:'Ghostty'}),"/c is a checkout of github.com/o/c: it is now that repository's folder; typed /add-dir into the session in Ghostty")
})
