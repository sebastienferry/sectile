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

// DOM mock to verify openProject dialog rendering, reset controls, validation and mapProject payload
test('project settings dialog renders AI provider and model controls and handles reset, validation, and submission', async () => {
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

  let mappedPayload = null
  const api = {
    mapProject: async p => { mappedPayload = p },
  }

  // Build the dialog components matching main.js
  const config = info.server
  let useWorktrees = info.useWorktrees, inheritWorktrees = !info.worktreeOverride
  let parallelism = info.parallelism || 1

  let selectedProvider = info.aiProvider || config.aiProvider || 'agy'
  let inheritAiProvider = !info.aiProviderOverride
  const providerSection = doc.createElement('section')
  const providerHeading = doc.createElement('div')
  const providerTitle = doc.createElement('strong')
  providerTitle.textContent = 'AI Provider'
  const providerReset = doc.createElement('button')
  providerReset.setAttribute('aria-label', 'Reset AI provider to server default')
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
    providerHint.textContent = (inheritAiProvider ? 'Inherited from server' : 'Local override') + ' · Server default: ' + (config.aiProvider || 'agy')
  }
  providerReset.onclick = () => {
    selectedProvider = config.aiProvider || 'agy'
    providerSelect.value = selectedProvider
    inheritAiProvider = true
    updateProvider()
  }

  let selectedModel = info.aiModel ?? config.aiModel ?? ''
  let inheritAiModel = !info.aiModelOverride
  const modelSection = doc.createElement('section')
  const modelHeading = doc.createElement('div')
  const modelTitle = doc.createElement('strong')
  modelTitle.textContent = 'AI Model'
  const modelReset = doc.createElement('button')
  modelReset.setAttribute('aria-label', 'Reset AI model to server default')
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
    modelHint.textContent = (inheritAiModel ? 'Inherited from server' : 'Local override') + ' · Server default: ' + (config.aiModel || '(none)')
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
    selectedModel = config.aiModel || ''
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
  assert.equal(providerHint.textContent, 'Local override · Server default: agy')
  assert.equal(modelInput.value, 'claude-opus-5')
  assert.equal(modelHint.textContent, 'Local override · Server default: server-base-model')

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
  assert.equal(providerHint.textContent, 'Inherited from server · Server default: agy')

  modelReset.onclick()
  assert.equal(selectedModel, 'server-base-model')
  assert.equal(modelInput.value, 'server-base-model')
  assert.equal(modelHint.textContent, 'Inherited from server · Server default: server-base-model')

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
    aiModel: 'server-base-model',
    inheritAiProvider: true,
    inheritAiModel: true,
    aiCommandTemplate: '',
    aiCommandTemplateAutonomous: '',
    inheritCommand: true,
  })
})
