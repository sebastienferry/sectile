import { test } from 'node:test'
import assert from 'node:assert/strict'
import { commandPreview, previewLines } from '../src/command-preview.mjs'

test('AI model identifier validation', () => {
  const MODEL_REGEX = /^[A-Za-z0-9][A-Za-z0-9._:@/-]*$/
  const isValid = val => {
    const trimmed = String(val || '').trim()
    if (trimmed === '') return true
    return MODEL_REGEX.test(trimmed)
  }

  // Valid identifiers
  assert.equal(isValid(''), true)
  assert.equal(isValid('   '), true)
  assert.equal(isValid('claude-3-5-sonnet'), true)
  assert.equal(isValid('gpt-4o'), true)
  assert.equal(isValid('gemini-1.5-pro'), true)
  assert.equal(isValid('meta-llama/Llama-3-70b-instruct:latest'), true)
  assert.equal(isValid('org.ai/model_v1@q4_0'), true)

  // Invalid identifiers (spaces, shell metacharacters, leading dash, special chars)
  assert.equal(isValid('claude 3.5'), false)
  assert.equal(isValid('model; rm -rf /'), false)
  assert.equal(isValid('-leading-dash'), false)
  assert.equal(isValid('model$var'), false)
  assert.equal(isValid('model|pipe'), false)
  assert.equal(isValid('model`backtick`'), false)
  assert.equal(isValid('(model)'), false)
  assert.equal(isValid('model>out'), false)
})

test('command template auto-synchronization on provider change', () => {
  const KNOWN_PRESETS = [
    '',
    "/path/to/custom-cli {mode:-p|-i} '{prompt}'",
    "claude --model {model} '{prompt}'",
    'agy --dangerously-skip-permissions --model {model} "{prompt}"',
    "codex --model {model} '{prompt}'",
  ]

  const syncCommand = (prevCmd, newProvider) => {
    let cmd = prevCmd
    let autoCmd = ''
    if (cmd.trim() === '' || KNOWN_PRESETS.includes(cmd.trim())) {
      if (newProvider === 'custom') {
        cmd = "/path/to/custom-cli {mode:-p|-i} '{prompt}'"
      } else {
        cmd = ''
      }
      autoCmd = ''
    }
    return { cmd, autoCmd }
  }

  // Switching from default to custom sets default custom command
  assert.deepEqual(syncCommand('', 'custom'), {
    cmd: "/path/to/custom-cli {mode:-p|-i} '{prompt}'",
    autoCmd: '',
  })

  // Switching from custom default back to codex clears command
  assert.deepEqual(syncCommand("/path/to/custom-cli {mode:-p|-i} '{prompt}'", 'codex'), {
    cmd: '',
    autoCmd: '',
  })

  // Custom user-typed template is preserved across provider switches
  const userCustom = "my-agent --prompt '{prompt}' --model {model}"
  assert.deepEqual(syncCommand(userCustom, 'claude'), {
    cmd: userCustom,
    autoCmd: '',
  })
})

test('previewLines generates previews for all supported providers and models', () => {
  const providers = ['agy', 'claude', 'codex', 'gemini', 'cursor', 'vibe', 'custom']
  for (const provider of providers) {
    const lines = previewLines(provider, provider === 'custom' ? "custom '{prompt}'" : '', 'test-model')
    assert.equal(lines.length, 2)
    assert.equal(lines[0].label, 'Interactive')
    assert.equal(lines[1].label, 'Autonomous')
    assert.equal(lines[0].ok, true, `interactive preview failed for ${provider}: ${lines[0].text}`)
  }
})

class MockElement {
  constructor(tag) {
    this.tagName = tag.toUpperCase()
    this.children = []
    this.attributes = {}
    this.style = {}
    this.value = ''
    this.textContent = ''
    this.disabled = false
    this.type = 'text'
    this.placeholder = ''
    this.hidden = false
  }
  setAttribute(k, v) { this.attributes[k] = String(v) }
  getAttribute(k) { return this.attributes[k] }
  removeAttribute(k) { delete this.attributes[k] }
  append(...items) {
    for (const item of items) {
      if (typeof item === 'string') {
        const t = new MockElement('#text')
        t.textContent = item
        this.children.push(t)
      } else if (item) {
        this.children.push(item)
      }
    }
  }
  replaceChildren(...items) {
    this.children = []
    this.append(...items)
  }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null }
  querySelectorAll(sel) {
    const results = []
    const match = el => {
      if (sel.startsWith('#') && el.id === sel.slice(1)) return true
      if (sel.startsWith('.') && el.className?.split(/\s+/).includes(sel.slice(1))) return true
      if (sel.toLowerCase() === el.tagName.toLowerCase()) return true
      if (sel.includes('[') && sel.includes(']')) {
        const [tag, attrPart] = sel.split('[')
        const [k, v] = attrPart.replace(']', '').split('=')
        const val = v ? v.replace(/['"]/g, '') : null
        if (tag && tag.toLowerCase() !== el.tagName.toLowerCase()) return false
        if (val !== null) return el.getAttribute(k) === val
        return el.getAttribute(k) !== undefined
      }
      return false
    }
    const walk = node => {
      for (const child of node.children) {
        if (match(child)) results.push(child)
        walk(child)
      }
    }
    walk(this)
    return results
  }
}

const doc = {
  createElement: tag => new MockElement(tag),
}

// DOM mock to verify openProject dialog rendering, reset controls, validation and mapProject payload
test('project settings dialog renders AI provider and model controls and handles reset, validation, and submission', async () => {
  // Simulate openProject data
  const info = {
    server: {
      projectId: 'proj-1',
      projectName: 'Test Project',
      aiProvider: 'agy',
      aiModel: 'server-base-model',
      useWorktrees: true,
      aiCommandTemplate: 'agy {prompt}',
    },
    path: '/path/to/repo',
    useWorktrees: true,
    worktreeOverride: false,
    parallelism: 2,
    aiProvider: 'claude',
    aiModel: 'claude-opus-5',
    aiProviderOverride: true,
    aiModelOverride: true,
    commandOverride: false,
  }

  const wsSettings = {
    aiProvider: 'agy',
    aiModel: 'ws-base-model',
  }

  let mappedPayload = null
  const api = {
    mapProject: async p => { mappedPayload = p },
  }

  // Build the dialog components matching main.js
  let useWorktrees = info.useWorktrees, inheritWorktrees = !info.worktreeOverride
  let parallelism = info.parallelism || 1

  let selectedProvider = info.aiProvider || wsSettings.aiProvider || 'agy'
  let inheritAiProvider = !info.aiProviderOverride
  const providerSection = doc.createElement('section')
  const providerHeading = doc.createElement('div')
  const providerTitle = doc.createElement('strong')
  providerTitle.textContent = 'AI Provider'
  const providerReset = doc.createElement('button')
  providerReset.setAttribute('aria-label', 'Reset AI provider to workstation default')
  providerHeading.append(providerTitle, providerReset)

  const providerSelect = doc.createElement('select')
  providerSelect.setAttribute('aria-label', 'AI Provider')
  const PROVIDERS = [
    { id: 'agy', label: 'AGY CLI (Google Antigravity)' },
    { id: 'claude', label: 'Claude Code CLI' },
    { id: 'codex', label: 'Codex CLI' },
    { id: 'gemini', label: 'Gemini CLI' },
    { id: 'cursor', label: 'Cursor CLI' },
    { id: 'vibe', label: 'Mistral Vibe CLI' },
    { id: 'custom', label: 'Custom Command' },
  ]
  for (const p of PROVIDERS) {
    const opt = doc.createElement('option')
    opt.value = p.id
    opt.textContent = p.label
    providerSelect.append(opt)
  }
  providerSelect.value = selectedProvider
  const providerHint = doc.createElement('p')
  providerSection.append(providerHeading, providerSelect, providerHint)

  function updateProvider() {
    providerHint.textContent = (inheritAiProvider ? 'Inherited from workstation' : 'Local override') + ' · Workstation default: ' + (wsSettings.aiProvider || 'agy')
  }
  providerReset.onclick = () => {
    selectedProvider = wsSettings.aiProvider || 'agy'
    providerSelect.value = selectedProvider
    inheritAiProvider = true
    updateProvider()
  }

  let selectedModel = info.aiModel ?? wsSettings.aiModel ?? ''
  let inheritAiModel = !info.aiModelOverride
  const modelSection = doc.createElement('section')
  const modelHeading = doc.createElement('div')
  const modelTitle = doc.createElement('strong')
  modelTitle.textContent = 'AI Model'
  const modelReset = doc.createElement('button')
  modelReset.setAttribute('aria-label', 'Reset AI model to workstation default')
  modelHeading.append(modelTitle, modelReset)

  const modelInput = doc.createElement('input')
  modelInput.setAttribute('aria-label', 'AI Model')
  modelInput.value = selectedModel
  const modelHint = doc.createElement('p')
  modelSection.append(modelHeading, modelInput, modelHint)

  const MODEL_REGEX = /^[A-Za-z0-9][A-Za-z0-9._:@/-]*$/
  function validateModel(val) {
    const trimmed = String(val || '').trim()
    if (trimmed === '') return true
    return MODEL_REGEX.test(trimmed)
  }

  function updateModel() {
    modelHint.textContent = (inheritAiModel ? 'Inherited from workstation' : 'Local override') + ' · Workstation default: ' + (wsSettings.aiModel || '(none)')
    if (!validateModel(modelInput.value)) {
      modelHint.textContent = 'Invalid model: must only contain letters, digits, and allowed punctuation (. _ - : @ /)'
      modelInput.setAttribute('aria-invalid', 'true')
    } else {
      modelInput.removeAttribute('aria-invalid')
    }
  }

  modelInput.oninput = () => {
    selectedModel = modelInput.value
    inheritAiModel = false
    updateModel()
  }
  modelReset.onclick = () => {
    selectedModel = wsSettings.aiModel || ''
    modelInput.value = selectedModel
    inheritAiModel = true
    updateModel()
  }

  const command = doc.createElement('textarea')
  command.value = ''
  const autonomousCommand = doc.createElement('textarea')
  autonomousCommand.value = ''
  let inheritCommand = true

  const KNOWN_PRESETS = [
    '',
    "/path/to/custom-cli {mode:-p|-i} '{prompt}'",
    "claude --model {model} '{prompt}'",
    'agy --dangerously-skip-permissions --model {model} "{prompt}"',
    "codex --model {model} '{prompt}'",
  ]
  providerSelect.onchange = () => {
    selectedProvider = providerSelect.value
    inheritAiProvider = false
    if (command.value.trim() === '' || KNOWN_PRESETS.includes(command.value.trim())) {
      if (selectedProvider === 'custom') {
        command.value = "/path/to/custom-cli {mode:-p|-i} '{prompt}'"
      } else {
        command.value = ''
      }
      autonomousCommand.value = ''
    }
    updateProvider()
  }

  updateProvider()
  updateModel()

  const form = doc.createElement('form')
  const notice = doc.createElement('p')
  const save = doc.createElement('button')

  form.onsubmit = async event => {
    if (event?.preventDefault) event.preventDefault()
    if (!validateModel(modelInput.value)) {
      notice.textContent = 'Invalid AI model identifier: must only contain letters, digits, and allowed punctuation (. _ - : @ /)'
      return
    }
    if (selectedProvider === 'custom' && !command.value.includes('{prompt}')) {
      notice.textContent = 'Custom provider requires a command template containing {prompt}'
      return
    }
    save.disabled = true
    try {
      await api.mapProject({
        projectId: 'proj-1',
        path: info.path,
        useWorktrees,
        inheritWorktrees,
        parallelism,
        aiProvider: selectedProvider,
        aiModel: modelInput.value.trim(),
        inheritAiProvider,
        inheritAiModel,
        aiCommandTemplate: command.value,
        aiCommandTemplateAutonomous: autonomousCommand.value,
        inheritCommand,
      })
      notice.textContent = 'Local configuration saved'
    } catch (err) {
      notice.textContent = err.message
    } finally {
      save.disabled = false
    }
  }

  // 1. Verify UI initial rendering
  assert.equal(providerSelect.children.length, 7)
  assert.equal(providerSelect.value, 'claude')
  assert.equal(providerHint.textContent, 'Local override · Workstation default: agy')
  assert.equal(modelInput.value, 'claude-opus-5')
  assert.equal(modelHint.textContent, 'Local override · Workstation default: ws-base-model')

  // 2. Verify submission with initial overrides
  await form.onsubmit()
  assert.deepEqual(mappedPayload, {
    projectId: 'proj-1',
    path: '/path/to/repo',
    useWorktrees: true,
    inheritWorktrees: true,
    parallelism: 2,
    aiProvider: 'claude',
    aiModel: 'claude-opus-5',
    inheritAiProvider: false,
    inheritAiModel: false,
    aiCommandTemplate: '',
    aiCommandTemplateAutonomous: '',
    inheritCommand: true,
  })

  // 3. Verify validation: invalid model
  modelInput.value = 'invalid model with spaces'
  modelInput.oninput()
  assert.equal(modelHint.textContent, 'Invalid model: must only contain letters, digits, and allowed punctuation (. _ - : @ /)')
  assert.equal(modelInput.getAttribute('aria-invalid'), 'true')
  mappedPayload = null
  await form.onsubmit()
  assert.equal(mappedPayload, null)
  assert.match(notice.textContent, /Invalid AI model identifier/)

  // 4. Verify validation: custom provider without {prompt}
  modelInput.value = 'valid-model'
  modelInput.oninput()
  providerSelect.value = 'custom'
  providerSelect.onchange()
  assert.equal(command.value, "/path/to/custom-cli {mode:-p|-i} '{prompt}'")
  command.value = "custom without prompt"
  mappedPayload = null
  await form.onsubmit()
  assert.equal(mappedPayload, null)
  assert.match(notice.textContent, /Custom provider requires a command template containing \{prompt\}/)

  // 5. Verify reset actions
  providerReset.onclick()
  assert.equal(selectedProvider, 'agy')
  assert.equal(providerSelect.value, 'agy')
  assert.equal(providerHint.textContent, 'Inherited from workstation · Workstation default: agy')

  modelReset.onclick()
  assert.equal(selectedModel, 'ws-base-model')
  assert.equal(modelInput.value, 'ws-base-model')
  assert.equal(modelHint.textContent, 'Inherited from workstation · Workstation default: ws-base-model')

  // 6. Submit after reset sends inheritAiProvider and inheritAiModel as true
  command.value = ''
  mappedPayload = null
  await form.onsubmit()
  assert.deepEqual(mappedPayload, {
    projectId: 'proj-1',
    path: '/path/to/repo',
    useWorktrees: true,
    inheritWorktrees: true,
    parallelism: 2,
    aiProvider: 'agy',
    aiModel: 'ws-base-model',
    inheritAiProvider: true,
    inheritAiModel: true,
    aiCommandTemplate: '',
    aiCommandTemplateAutonomous: '',
    inheritCommand: true,
  })
})

test('project settings dialog renders terminal emulator controls and handles reset, custom command, and submission', async () => {
  const info = {
    server: {
      projectId: 'proj-1',
      projectName: 'Test Project',
      externalTerminalCommand: 'terminal',
      useWorktrees: true,
    },
    path: '/path/to/repo',
    useWorktrees: true,
    worktreeOverride: false,
    parallelism: 1,
    terminal: 'ghostty',
    terminalOverride: true,
  }

  let mappedPayload = null
  const api = {
    mapProject: async p => { mappedPayload = p },
  }

  const config = info.server
  let selectedTerminal = info.terminal ?? config.externalTerminalCommand ?? '', inheritTerminal = !info.terminalOverride

  const terminalSection = doc.createElement('section')
  const terminalHeading = doc.createElement('div')
  const terminalTitle = doc.createElement('strong')
  terminalTitle.textContent = 'Terminal emulator'
  const terminalReset = doc.createElement('button')
  terminalReset.setAttribute('aria-label', 'Reset terminal emulator to workstation default')
  terminalHeading.append(terminalTitle, terminalReset)

  const terminalSelect = doc.createElement('select')
  terminalSelect.setAttribute('aria-label', 'Terminal emulator')
  const TERMINALS = [
    { id: '', label: 'Auto-detect (Ghostty, iTerm, Terminal)' },
    { id: 'ghostty', label: 'Ghostty' },
    { id: 'terminal', label: 'Terminal.app' },
    { id: 'iterm', label: 'iTerm' },
    { id: 'custom', label: 'Custom command…' },
  ]
  for (const t of TERMINALS) {
    const opt = doc.createElement('option')
    opt.value = t.id
    opt.textContent = t.label
    terminalSelect.append(opt)
  }
  const customTerminalInput = doc.createElement('input')
  customTerminalInput.setAttribute('aria-label', 'Custom terminal command')

  const standardTerminals = ['', 'ghostty', 'terminal', 'iterm']
  if (selectedTerminal && !standardTerminals.includes(selectedTerminal.toLowerCase())) {
    terminalSelect.value = 'custom'
    customTerminalInput.value = selectedTerminal
    customTerminalInput.hidden = false
  } else {
    terminalSelect.value = selectedTerminal ? selectedTerminal.toLowerCase() : ''
    customTerminalInput.value = ''
    customTerminalInput.hidden = true
  }

  const terminalHint = doc.createElement('p')
  terminalSection.append(terminalHeading, terminalSelect, customTerminalInput, terminalHint)

  function updateTerminal() {
    terminalHint.textContent = (inheritTerminal ? 'Inherited from workstation' : 'Local override') + ' · Default: ' + (config.externalTerminalCommand || 'Auto-detect')
    customTerminalInput.hidden = terminalSelect.value !== 'custom'
  }

  terminalSelect.onchange = () => {
    inheritTerminal = false
    if (terminalSelect.value !== 'custom') {
      selectedTerminal = terminalSelect.value
    } else {
      selectedTerminal = customTerminalInput.value.trim()
    }
    updateTerminal()
  }
  customTerminalInput.oninput = () => {
    inheritTerminal = false
    selectedTerminal = customTerminalInput.value.trim()
  }
  terminalReset.onclick = () => {
    selectedTerminal = config.externalTerminalCommand || ''
    inheritTerminal = true
    if (selectedTerminal && !standardTerminals.includes(selectedTerminal.toLowerCase())) {
      terminalSelect.value = 'custom'
      customTerminalInput.value = selectedTerminal
    } else {
      terminalSelect.value = selectedTerminal ? selectedTerminal.toLowerCase() : ''
      customTerminalInput.value = ''
    }
    updateTerminal()
  }
  updateTerminal()

  const form = doc.createElement('form')
  form.onsubmit = async event => {
    if (event?.preventDefault) event.preventDefault()
    const termToSend = terminalSelect.value === 'custom' ? customTerminalInput.value.trim() : terminalSelect.value
    await api.mapProject({
      projectId: 'proj-1',
      path: info.path,
      terminal: termToSend,
      inheritTerminal,
    })
  }

  // 1. Initial render with local override 'ghostty'
  assert.equal(terminalSelect.children.length, 5)
  assert.equal(terminalSelect.value, 'ghostty')
  assert.equal(customTerminalInput.hidden, true)
  assert.equal(terminalHint.textContent, 'Local override · Default: terminal')

  // 2. Submit with initial override
  await form.onsubmit()
  assert.deepEqual(mappedPayload, {
    projectId: 'proj-1',
    path: '/path/to/repo',
    terminal: 'ghostty',
    inheritTerminal: false,
  })

  // 3. Switch to custom command
  terminalSelect.value = 'custom'
  terminalSelect.onchange()
  assert.equal(customTerminalInput.hidden, false)
  customTerminalInput.value = 'alacritty -e {command}'
  customTerminalInput.oninput()

  mappedPayload = null
  await form.onsubmit()
  assert.deepEqual(mappedPayload, {
    projectId: 'proj-1',
    path: '/path/to/repo',
    terminal: 'alacritty -e {command}',
    inheritTerminal: false,
  })

  // 4. Click reset to workstation/server default
  terminalReset.onclick()
  assert.equal(terminalSelect.value, 'terminal')
  assert.equal(customTerminalInput.hidden, true)
  assert.equal(terminalHint.textContent, 'Inherited from workstation · Default: terminal')

  mappedPayload = null
  await form.onsubmit()
  assert.deepEqual(mappedPayload, {
    projectId: 'proj-1',
    path: '/path/to/repo',
    terminal: 'terminal',
    inheritTerminal: true,
  })
})

test('Agents CLI settings panel renders controls, presets, live preview, validation, and saves via saveSettings', async () => {
  const stored = {
    aiProvider: 'claude',
    aiModel: 'claude-3-7-sonnet',
    aiCommandTemplate: "claude --model {model} '{prompt}'",
    aiCommandTemplateAutonomous: "claude -p --permission-mode bypassPermissions --model {model} '{prompt}'",
  }

  let savedSettings = null
  const api = {
    settings: async () => stored,
    saveSettings: async s => { savedSettings = s; return s },
  }

  // Build the Agents CLI panel components matching openSettings() in main.js
  const cliProviderSelect = doc.createElement('select')
  cliProviderSelect.setAttribute('aria-label', 'AI Provider')
  const CLI_PROVIDERS = [
    { id: 'agy', label: 'AGY CLI (Google Antigravity)' },
    { id: 'claude', label: 'Claude Code CLI' },
    { id: 'codex', label: 'Codex CLI' },
    { id: 'gemini', label: 'Gemini CLI' },
    { id: 'cursor', label: 'Cursor CLI' },
    { id: 'vibe', label: 'Mistral Vibe CLI' },
    { id: 'custom', label: 'Custom Command' },
  ]
  for (const p of CLI_PROVIDERS) {
    const opt = doc.createElement('option')
    opt.value = p.id
    opt.textContent = p.label
    cliProviderSelect.append(opt)
  }

  const cliModelInput = doc.createElement('input')
  cliModelInput.setAttribute('aria-label', 'AI Model')

  const MODEL_REGEX = /^[A-Za-z0-9][A-Za-z0-9._:@/-]*$/
  function validateModel(val) {
    const trimmed = String(val || '').trim()
    if (trimmed === '') return true
    return MODEL_REGEX.test(trimmed)
  }

  const cliCommand = doc.createElement('textarea')
  cliCommand.setAttribute('aria-label', 'Interactive CLI command')

  const cliAutonomousCommand = doc.createElement('textarea')
  cliAutonomousCommand.setAttribute('aria-label', 'Autonomous CLI command')

  const cliPreviewBox = doc.createElement('dl')

  function renderCliPreview() {
    cliPreviewBox.replaceChildren()
    for (const line of previewLines(cliProviderSelect.value, cliCommand.value, cliModelInput.value, cliAutonomousCommand.value)) {
      const term = doc.createElement('dt'); term.textContent = line.label
      const detail = doc.createElement('dd'); detail.textContent = line.text
      if (!line.ok) detail.className = 'command-preview-error'
      cliPreviewBox.append(term, detail)
    }
  }

  cliModelInput.oninput = renderCliPreview
  cliCommand.oninput = renderCliPreview
  cliAutonomousCommand.oninput = renderCliPreview

  const CLI_PRESETS = [
    { label: 'AGY', provider: 'agy', cmd: 'agy --dangerously-skip-permissions --model {model} "{prompt}"', auto: 'agy --dangerously-skip-permissions --model {model} -p "{prompt}"' },
    { label: 'Claude', provider: 'claude', cmd: "claude --model {model} '{prompt}'", auto: "claude -p --permission-mode bypassPermissions --model {model} '{prompt}'" },
    { label: 'Codex', provider: 'codex', cmd: "codex --model {model} '{prompt}'", auto: "codex exec --model {model} '{prompt}'" },
    { label: 'Gemini', provider: 'gemini', cmd: "gemini --model {model} '{prompt}'", auto: "gemini -y --model {model} -p '{prompt}'" },
    { label: 'Vibe', provider: 'vibe', cmd: "vibe '{prompt}'", auto: "vibe -p --auto-approve '{prompt}'" },
    { label: 'Custom', provider: 'custom', cmd: "/path/to/custom-cli {mode:-p|-i} '{prompt}'", auto: '' },
    { label: 'Clear to defaults', provider: 'agy', cmd: '', auto: '' },
  ]

  const presetButtons = CLI_PRESETS.map(preset => {
    const pBtn = doc.createElement('button')
    pBtn.textContent = preset.label
    pBtn.onclick = () => {
      cliProviderSelect.value = preset.provider
      cliCommand.value = preset.cmd
      cliAutonomousCommand.value = preset.auto
      renderCliPreview()
    }
    return pBtn
  })

  cliProviderSelect.onchange = () => {
    const KNOWN = ['', "/path/to/custom-cli {mode:-p|-i} '{prompt}'", "claude --model {model} '{prompt}'", 'agy --dangerously-skip-permissions --model {model} "{prompt}"', "codex --model {model} '{prompt}'", "gemini --model {model} '{prompt}'", "vibe '{prompt}'"]
    if (cliCommand.value.trim() === '' || KNOWN.includes(cliCommand.value.trim())) {
      if (cliProviderSelect.value === 'custom') {
        cliCommand.value = "/path/to/custom-cli {mode:-p|-i} '{prompt}'"
      } else {
        cliCommand.value = ''
      }
      cliAutonomousCommand.value = ''
    }
    renderCliPreview()
  }

  const cliNotice = doc.createElement('p')
  const cliSaveBtn = doc.createElement('button')
  cliSaveBtn.textContent = 'Save Agents CLI settings'
  cliSaveBtn.onclick = async () => {
    if (!validateModel(cliModelInput.value)) {
      cliNotice.textContent = 'Invalid AI model identifier: must only contain letters, digits, and allowed punctuation (. _ - : @ /)'
      return
    }
    if (cliProviderSelect.value === 'custom' && !cliCommand.value.includes('{prompt}')) {
      cliNotice.textContent = 'Custom provider requires a command template containing {prompt}'
      return
    }
    cliSaveBtn.disabled = true
    cliNotice.textContent = 'Saving…'
    try {
      await api.saveSettings({
        aiProvider: cliProviderSelect.value,
        aiModel: cliModelInput.value.trim(),
        aiCommandTemplate: cliCommand.value,
        aiCommandTemplateAutonomous: cliAutonomousCommand.value,
      })
      cliNotice.textContent = 'Agents CLI settings saved'
    } catch (err) {
      cliNotice.textContent = 'Error saving settings: ' + (err.message || String(err))
    } finally {
      cliSaveBtn.disabled = false
    }
  }

  // Populate from stored settings
  cliProviderSelect.value = stored.aiProvider || 'agy'
  cliModelInput.value = stored.aiModel || ''
  cliCommand.value = stored.aiCommandTemplate || ''
  cliAutonomousCommand.value = stored.aiCommandTemplateAutonomous || ''
  renderCliPreview()

  // 1. Initial values loaded
  assert.equal(cliProviderSelect.value, 'claude')
  assert.equal(cliModelInput.value, 'claude-3-7-sonnet')
  assert.equal(cliCommand.value, "claude --model {model} '{prompt}'")
  assert.equal(cliAutonomousCommand.value, "claude -p --permission-mode bypassPermissions --model {model} '{prompt}'")
  assert.equal(cliPreviewBox.children.length, 4) // 2 terms + 2 details

  // 2. Click preset: Codex
  const codexPreset = presetButtons.find(b => b.textContent === 'Codex')
  codexPreset.onclick()
  assert.equal(cliProviderSelect.value, 'codex')
  assert.equal(cliCommand.value, "codex --model {model} '{prompt}'")
  assert.equal(cliAutonomousCommand.value, "codex exec --model {model} '{prompt}'")

  // 3. Save after preset
  await cliSaveBtn.onclick()
  assert.equal(cliNotice.textContent, 'Agents CLI settings saved')
  assert.deepEqual(savedSettings, {
    aiProvider: 'codex',
    aiModel: 'claude-3-7-sonnet',
    aiCommandTemplate: "codex --model {model} '{prompt}'",
    aiCommandTemplateAutonomous: "codex exec --model {model} '{prompt}'",
  })

  // 3b. Click preset: AGY
  const agyPreset = presetButtons.find(b => b.textContent === 'AGY')
  agyPreset.onclick()
  assert.equal(cliProviderSelect.value, 'agy')
  assert.equal(cliCommand.value, 'agy --dangerously-skip-permissions --model {model} "{prompt}"')
  assert.equal(cliAutonomousCommand.value, 'agy --dangerously-skip-permissions --model {model} -p "{prompt}"')

  // 4. Validation error: invalid model
  cliModelInput.value = 'invalid model spaces'
  savedSettings = null
  await cliSaveBtn.onclick()
  assert.equal(savedSettings, null)
  assert.match(cliNotice.textContent, /Invalid AI model identifier/)

  // 5. Validation error: custom provider without {prompt}
  cliModelInput.value = 'valid-model'
  cliProviderSelect.value = 'custom'
  cliCommand.value = 'custom-cmd --no-prompt'
  savedSettings = null
  await cliSaveBtn.onclick()
  assert.equal(savedSettings, null)
  assert.match(cliNotice.textContent, /Custom provider requires a command template containing \{prompt\}/)

  // 6. Fix custom command and save
  cliCommand.value = 'custom-cmd {mode:-p|-i} {prompt}'
  await cliSaveBtn.onclick()
  assert.equal(cliNotice.textContent, 'Agents CLI settings saved')
  assert.deepEqual(savedSettings, {
    aiProvider: 'custom',
    aiModel: 'valid-model',
    aiCommandTemplate: 'custom-cmd {mode:-p|-i} {prompt}',
    aiCommandTemplateAutonomous: 'agy --dangerously-skip-permissions --model {model} -p "{prompt}"',
  })
})

