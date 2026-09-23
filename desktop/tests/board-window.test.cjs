const test = require('node:test')
const assert = require('node:assert')
const {boardOrigin, boardURL, navigation, signedHeaders, authRoute, createBoardWindows} = require('../electron/board-window.cjs')

const origin = 'https://sectile.example.test'
const state = {origin, key: 'sectile_key', webContentsId: 7}

test('the board opens only an HTTP(S) server without credentials in its address', () => {
 assert.strictEqual(boardOrigin('https://sectile.example.test/base/?view=board'), origin)
 assert.strictEqual(boardOrigin('http://127.0.0.1:8090'), 'http://127.0.0.1:8090')
 for (const server of ['file:///tmp/board', 'javascript:alert(1)', 'https://user:secret@example.test/']) {
  assert.throws(() => boardOrigin(server), /Invalid server URL/)
 }
 assert.throws(() => boardOrigin('invalid'))
})

test('the board address keeps the base path and carries the task to open', () => {
 assert.strictEqual(boardURL('https://sectile.example.test/base/?task=old&view=board#x'), 'https://sectile.example.test/base/?view=board')
 assert.strictEqual(boardURL('https://sectile.example.test/base/', '#42'), 'https://sectile.example.test/base/?task=%2342')
})

test('links to the server stay, other web addresses go to the browser, anything else goes nowhere', () => {
 assert.strictEqual(navigation(origin + '/?task=1', origin), 'stay')
 assert.strictEqual(navigation('https://github.com/o/r/pull/1', origin), 'external')
 // Same host, another port or scheme, is another origin.
 assert.strictEqual(navigation('http://sectile.example.test/', origin), 'external')
 assert.strictEqual(navigation('https://sectile.example.test:8443/', origin), 'external')
 for (const target of ['file:///etc/passwd', 'javascript:alert(1)', 'mailto:a@b.c', 'https://u:p@evil.test/', 'not a url']) {
  assert.strictEqual(navigation(target, origin), 'deny', target)
 }
})

test('the key goes to the server origin only, from the board window only', () => {
 const request = extra => ({url: origin + '/api/me', webContentsId: 7, requestHeaders: {Accept: 'application/json'}, ...extra})
 assert.deepStrictEqual(signedHeaders(request(), state), {Accept: 'application/json', Authorization: 'Bearer sectile_key'})
 assert.deepStrictEqual(signedHeaders(request({frame: {url: origin + '/'}}), state).Authorization, 'Bearer sectile_key')
 assert.strictEqual(signedHeaders(request({url: 'https://github.com/api'}), state), null)
 assert.strictEqual(signedHeaders(request({url: 'https://sectile.example.test.evil.test/api/me'}), state), null)
 assert.strictEqual(signedHeaders(request({url: 'http://sectile.example.test/api/me'}), state), null)
 assert.strictEqual(signedHeaders(request({webContentsId: 8}), state), null)
 // A frame of another origin asking the server does not borrow the key.
 assert.strictEqual(signedHeaders(request({frame: {url: 'https://evil.test/'}}), state), null)
 const gone = {get url() { throw Error('Render frame was disposed') }}
 assert.strictEqual(signedHeaders(request({frame: gone}), state), null)
 assert.strictEqual(signedHeaders(request(), {...state, key: ''}), null)
})

test('the sign-in and sign-out routes of the server are the ones refused', () => {
 assert.strictEqual(authRoute(origin + '/auth/logout', origin), true)
 assert.strictEqual(authRoute(origin + '/auth/local', origin), true)
 assert.strictEqual(authRoute(origin + '/api/me', origin), false)
 assert.strictEqual(authRoute('https://idp.example.test/auth/login', origin), false)
})

// Fakes that record what the module asks of Electron.
function electronFakes() {
 const windows = [], sessions = new Map(), opened = []
 class BrowserWindow {
  constructor(options) {
   this.options = options
   this.destroyed = false
   this.handlers = {}
   this.loaded = []
   this.shown = 0
   const events = {}
   this.webContents = {
    id: 100 + windows.length,
    on: (name, fn) => { events[name] = fn },
    emit: (name, ...args) => events[name](...args),
    getURL: () => this.loaded.at(-1) || '',
    setWindowOpenHandler: fn => { this.openHandler = fn }
   }
   windows.push(this)
  }
  loadURL(url) { this.loaded.push(url) }
  show() { this.shown++ }
  focus() {}
  isDestroyed() { return this.destroyed }
  destroy() { this.destroyed = true; this.handlers.closed?.() }
  on(name, fn) { this.handlers[name] = fn }
 }
 const session = {
  fromPartition(name) {
   if (!sessions.has(name)) {
    const ses = {name, cleared: 0, listeners: {}}
    ses.webRequest = {
     onBeforeRequest: fn => { ses.listeners.request = fn },
     onBeforeSendHeaders: fn => { ses.listeners.headers = fn }
    }
    ses.setPermissionRequestHandler = fn => { ses.permissionRequest = fn }
    ses.setPermissionCheckHandler = fn => { ses.permissionCheck = fn }
    ses.clearStorageData = async () => { ses.cleared++ }
    sessions.set(name, ses)
   }
   return sessions.get(name)
  }
 }
 const shell = {openExternal: async url => { opened.push(url) }}
 return {BrowserWindow, session, shell, windows, sessions, opened}
}

test('one board window, isolated, reused, and signed with the key', () => {
 const fakes = electronFakes()
 const boards = createBoardWindows({...fakes, show: false})
 const window = boards.open({server: origin + '/base/', key: 'k1'})
 const prefs = window.options.webPreferences
 assert.strictEqual(prefs.preload, undefined, 'the page gets no bridge to the desktop')
 assert.strictEqual(prefs.sandbox, true)
 assert.strictEqual(prefs.contextIsolation, true)
 assert.strictEqual(prefs.nodeIntegration, false)
 assert.ok(!prefs.session.name.startsWith('persist:'), 'the board keeps nothing on disk')
 assert.deepStrictEqual(window.loaded, [origin + '/base/'])

 // Opening again brings the same window forward; opening a task loads it there.
 assert.strictEqual(boards.open({server: origin + '/base/', key: 'k1'}), window)
 assert.strictEqual(window.shown, 1)
 assert.strictEqual(boards.open({server: origin + '/base/', key: 'k1', taskId: 't-9'}), window)
 assert.deepStrictEqual(window.loaded, [origin + '/base/', origin + '/base/?task=t-9'])
 assert.strictEqual(fakes.windows.length, 1)

 const ses = prefs.session
 let answer
 ses.listeners.headers({url: origin + '/api/events', webContentsId: window.webContents.id, requestHeaders: {}}, value => { answer = value })
 assert.deepStrictEqual(answer, {requestHeaders: {Authorization: 'Bearer k1'}})
 ses.listeners.headers({url: 'https://github.com/', webContentsId: window.webContents.id, requestHeaders: {}}, value => { answer = value })
 assert.deepStrictEqual(answer, {})
 ses.listeners.request({url: origin + '/auth/logout'}, value => { answer = value })
 assert.deepStrictEqual(answer, {cancel: true})
 ses.listeners.request({url: origin + '/api/me'}, value => { answer = value })
 assert.deepStrictEqual(answer, {cancel: false})

 ses.permissionRequest(null, 'clipboard-sanitized-write', value => { answer = value })
 assert.strictEqual(answer, true)
 for (const permission of ['media', 'geolocation', 'notifications', 'clipboard-read']) {
  ses.permissionRequest(null, permission, value => { answer = value })
  assert.strictEqual(answer, false, permission)
 }
})

test('links leaving the server open in the browser, and the window never follows them', () => {
 const fakes = electronFakes()
 const boards = createBoardWindows({...fakes, show: false})
 const window = boards.open({server: origin, key: 'k1'})
 const navigate = url => { const event = {url, prevented: false, preventDefault() { this.prevented = true }}; window.webContents.emit('will-navigate', event); return event.prevented }
 assert.strictEqual(navigate(origin + '/?task=2'), false)
 assert.strictEqual(navigate('https://github.com/o/r/pull/1'), true)
 assert.strictEqual(navigate('file:///etc/passwd'), true)
 assert.deepStrictEqual(fakes.opened, ['https://github.com/o/r/pull/1'])

 assert.deepStrictEqual(window.openHandler({url: 'https://github.com/o/r/issues/3'}), {action: 'deny'})
 assert.deepStrictEqual(window.openHandler({url: origin + '/?task=5'}), {action: 'deny'})
 assert.strictEqual(window.loaded.at(-1), origin + '/?task=5', 'a new tab on the server opens in the board')
 assert.deepStrictEqual(fakes.opened, ['https://github.com/o/r/pull/1', 'https://github.com/o/r/issues/3'])
})

test('another key or another server replaces the board and clears what it stored', () => {
 const fakes = electronFakes()
 const boards = createBoardWindows({...fakes, show: false})
 const first = boards.open({server: origin, key: 'k1'})
 const second = boards.open({server: origin, key: 'k2'})
 assert.notStrictEqual(second, first)
 assert.strictEqual(first.destroyed, true)
 assert.strictEqual(first.options.webPreferences.session.cleared, 1)
 const third = boards.open({server: 'http://127.0.0.1:8090', key: 'k2'})
 assert.strictEqual(second.destroyed, true)
 assert.notStrictEqual(third.options.webPreferences.session, second.options.webPreferences.session)
 boards.close()
 assert.strictEqual(third.destroyed, true)
 assert.strictEqual(boards.current(), null)
 // A window closed by the user is not reused.
 const fourth = boards.open({server: origin, key: 'k2'})
 fourth.destroy()
 assert.strictEqual(boards.current(), null)
 assert.notStrictEqual(boards.open({server: origin, key: 'k2'}), fourth)
})
