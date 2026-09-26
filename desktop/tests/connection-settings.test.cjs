const test=require('node:test')
const assert=require('node:assert/strict')
const {connectionUpdates,connectionView}=require('../electron/connection-settings.cjs')

// The desktop writes connection keys only: the execution sections of the
// shared settings file belong to the agent (#305).
test('save-settings keeps the connection keys and drops every execution key',()=>{
 const kept=connectionUpdates({server:'https://s',deviceId:'laptop',secret:'x',apiKey:'k',binary:'/bin/a',repo:'/r',
  aiProvider:'claude',defaults:{aiModel:'opus'},projectSettings:{p:{}},projects:{p:'/r'},terminal:'ghostty',layout:2})
 assert.deepEqual(Object.keys(kept).sort(),['apiKey','binary','deviceId','repo','secret','server'])
 assert.deepEqual(connectionUpdates(null),{})
})

test('the renderer reads the connection facts only, never the stored key',()=>{
 const view=connectionView({server:'https://s',deviceId:'laptop',secret:'x',apiKey:'k',defaults:{aiModel:'opus'},projectSettings:{}},'token')
 assert.deepEqual(view,{server:'https://s',deviceId:'laptop',token:'token'})
})
