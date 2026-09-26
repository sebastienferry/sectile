const {test} = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const assetsDir = path.resolve(__dirname, '../assets')

test('icon assets exist with valid headers and sizes', () => {
	const svgPath = path.join(assetsDir, 'icon.svg')
	assert.ok(fs.existsSync(svgPath), 'icon.svg must exist')
	const svgContent = fs.readFileSync(svgPath, 'utf8')
	assert.ok(svgContent.includes('<svg'), 'icon.svg must contain <svg>')
	assert.ok(svgContent.includes('viewBox='), 'icon.svg must declare a viewBox')
	assert.ok(svgContent.includes('linearGradient'), 'icon.svg must declare stream and badge gradients')
	assert.ok(
		svgContent.includes('sectile-badge-grad') || svgContent.includes('badge-grad'),
		'icon.svg must define the badge gradient'
	)

	const pngPath = path.join(assetsDir, 'icon.png')
	assert.ok(fs.existsSync(pngPath), 'icon.png must exist')
	const pngBuffer = fs.readFileSync(pngPath)
	assert.ok(pngBuffer.length > 1024, 'icon.png must be at least 1 KB')
	const pngMagic = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])
	assert.ok(pngBuffer.subarray(0, 8).equals(pngMagic), 'icon.png must begin with PNG magic bytes')

	const icnsPath = path.join(assetsDir, 'icon.icns')
	assert.ok(fs.existsSync(icnsPath), 'icon.icns must exist')
	const icnsBuffer = fs.readFileSync(icnsPath)
	assert.ok(icnsBuffer.length > 1024, 'icon.icns must be at least 1 KB')
	const icnsMagic = Buffer.from([0x69, 0x63, 0x6e, 0x73]) // 'icns'
	assert.ok(icnsBuffer.subarray(0, 4).equals(icnsMagic), 'icon.icns must begin with icns magic bytes')

	const icoPath = path.join(assetsDir, 'icon.ico')
	assert.ok(fs.existsSync(icoPath), 'icon.ico must exist')
	const icoBuffer = fs.readFileSync(icoPath)
	assert.ok(icoBuffer.length > 1024, 'icon.ico must be at least 1 KB')
	const icoMagic = Buffer.from([0x00, 0x00, 0x01, 0x00])
	assert.ok(icoBuffer.subarray(0, 4).equals(icoMagic), 'icon.ico must begin with Windows ICO header')
})

test('packager configuration sets branded icon path without extension', () => {
	const {packageOptions} = require('../electron/package-options.cjs')
	for (const platform of [undefined, 'darwin', 'linux', 'win32']) {
		assert.equal(
			packageOptions({platform}).icon,
			path.resolve(__dirname, '../assets/icon'),
			'packager must be given ../assets/icon, packager adds the platform extension'
		)
	}
})

test('electron runtime configures window icon and darwin dock icon', () => {
	const mainCjs = fs.readFileSync(path.resolve(__dirname, '../electron/main.cjs'), 'utf8')
	assert.match(
		mainCjs,
		/icon:\s*path\.join\(__dirname,\s*['"]\.\.\/assets\/icon\.png['"]\)/,
		'BrowserWindow options must include icon pointing to icon.png'
	)
	assert.match(
		mainCjs,
		/process\.platform\s*===\s*['"]darwin['"]\s*&&\s*app\.dock/,
		'main.cjs must guard macOS dock icon setup with platform check'
	)
	assert.match(
		mainCjs,
		/app\.dock\.setIcon\(path\.join\(__dirname,\s*['"]\.\.\/assets\/icon\.png['"]\)\)/,
		'main.cjs must invoke app.dock.setIcon with icon.png'
	)
})

test('desktop HTML declares SVG favicon link', () => {
	const indexHtml = fs.readFileSync(path.resolve(__dirname, '../index.html'), 'utf8')
	assert.match(indexHtml, /<link[^>]*rel=["']icon["'][^>]*>/i, 'index.html must include a favicon link')
	assert.match(indexHtml, /type=["']image\/svg\+xml["']/i, 'index.html favicon must declare image/svg+xml type')
	assert.match(indexHtml, /href=["']\.\/assets\/icon\.svg["']/i, 'index.html favicon must reference icon.svg')
})

test('desktop in-app header remains text-only without embedded visual icon', () => {
	const mainJs = fs.readFileSync(path.resolve(__dirname, '../src/main.js'), 'utf8')
	assert.ok(
		mainJs.includes('<strong id="app-title">Sectile Desktop</strong>'),
		'desktop header must retain clean text-only title'
	)
})
