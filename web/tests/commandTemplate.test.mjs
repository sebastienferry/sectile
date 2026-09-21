import assert from 'node:assert/strict'
import { test } from 'node:test'
import { commandPreview, resolveTemplateMode, templateCarriesMode, modelArgs, dropModelSlot } from '../src/lib/commandTemplate.ts'
import { CLAUDE_STREAM_FILTER } from '../src/lib/autonomousStream.ts'

// The autonomous fallback renders the provider's event stream, so its expected
// command line is built from the same constant the preview splices in: the test
// is about the invocation, not about the filter, which autonomousStream.test.mjs
// runs for real.
const claudeAutonomous = model =>
  'claude -p --permission-mode bypassPermissions --output-format stream-json --verbose' +
  (model ? ' --model ' + model : '') + " '{prompt}' | " + CLAUDE_STREAM_FILTER

test('claude without a template yields the two attested command lines', () => {
  const model = 'claude-opus-5'
  assert.equal(
    commandPreview('claude', '', model, false).command,
    "claude --model claude-opus-5 '{prompt}'"
  )
  assert.equal(
    commandPreview('claude', '', model, true).command,
    claudeAutonomous('claude-opus-5')
  )
})

test('an unset model drops the flag instead of passing an empty one', () => {
  assert.deepEqual(modelArgs('claude', ''), [])
  assert.deepEqual(modelArgs('agy', 'claude-opus-5'), [])
  assert.equal(commandPreview('claude', '', '', false).command, "claude '{prompt}'")
  assert.equal(
    commandPreview('claude', '', '', true).command,
    claudeAutonomous('')
  )
})

test('a template without the mode placeholder cannot serve an autonomous launch', () => {
  const template = `claude -p '{prompt}'`
  assert.equal(templateCarriesMode(template), false)
  assert.equal(commandPreview('claude', template, '', false).command, template)
  const autonomous = commandPreview('claude', template, '', true)
  assert.equal(autonomous.command, '')
  assert.match(autonomous.error, /\{mode:AUTONOMOUS\|INTERACTIVE\}/)
})

test('the mode placeholder picks the autonomous side on the left', () => {
  const template = `claude {mode:-p --permission-mode bypassPermissions|} --model {model} '{prompt}'`
  assert.equal(templateCarriesMode(template), true)
  assert.equal(
    commandPreview('claude', template, 'claude-opus-5', false).command,
    "claude --model 'claude-opus-5' '{prompt}'"
  )
  assert.equal(
    commandPreview('claude', template, 'claude-opus-5', true).command,
    "claude -p --permission-mode bypassPermissions --model 'claude-opus-5' '{prompt}'"
  )
})

// A flag left with nothing behind it consumes the next word rather than
// disappearing, so an unset model takes its option with it.
test('an unset model removes the slot and the option it belongs to', () => {
  assert.equal(commandPreview('claude', `claude --model {model} '{prompt}'`, '', false).command, "claude '{prompt}'")
  assert.equal(dropModelSlot(`claude --model '{model}' '{prompt}'`), "claude '{prompt}'")
  assert.equal(dropModelSlot(`claude --model={model} '{prompt}'`), "claude '{prompt}'")
  assert.equal(dropModelSlot(`mycli {model} '{prompt}'`), "mycli '{prompt}'")
})

test('a configured model reaches the slot with the template quoting', () => {
  assert.equal(
    commandPreview('claude', `claude --model '{model}' '{prompt}'`, "o'hara", false).command,
    "claude --model 'o'\\''hara' '{prompt}'"
  )
})

test('a placeholder with no separator leaves the interactive side empty', () => {
  assert.equal(resolveTemplateMode('agy {mode:-p} x', true), 'agy -p x')
  assert.equal(resolveTemplateMode('agy {mode:-p} x', false), 'agy  x')
})

test('a named provider ignores a template that carries no instructions', () => {
  assert.equal(commandPreview('claude', 'claude', 'm', false).command, "claude --model m '{prompt}'")
  assert.equal(commandPreview('custom', 'my-cli', '', false).command, 'my-cli')
})

test('a provider with no attested headless mode refuses rather than guessing', () => {
  const autonomous = commandPreview('gemini', '', '', true)
  assert.equal(autonomous.command, '')
  assert.match(autonomous.error, /no attested headless mode/)
  assert.equal(commandPreview('gemini', '', 'g', false).command, "gemini --model g '{prompt}'")
})

// A command written for headless use is what an autonomous launch runs, and it
// needs no marker of its own; the interactive one is left untouched.
test('a dedicated autonomous command serves headless launches on its own', () => {
  const interactive = `claude '{prompt}'`
  const autonomous = `claude -p --permission-mode bypassPermissions '{prompt}'`
  assert.equal(commandPreview('claude', interactive, '', false, autonomous).command, interactive)
  assert.equal(commandPreview('claude', interactive, '', true, autonomous).command, autonomous)
  // Without it the general command still has to declare that it can.
  assert.equal(commandPreview('claude', interactive, '', true).command, '')
})

// The two markers can share one token. The prompt is what the command exists to
// carry, so the model leaves alone rather than taking it along.
test('a model glued to the prompt leaves the prompt behind', () => {
  assert.equal(dropModelSlot('cli --opt={model}{prompt}'), 'cli --opt={prompt}')
  assert.equal(dropModelSlot('cli {model}{prompt}'), 'cli{prompt}')
})
