import assert from 'node:assert/strict'
import { test } from 'node:test'
import { skillModelMapping } from '../src/skill-model-mapping.mjs'

class MockNode {
 constructor(tag){
  this.tagName = tag.toUpperCase()
  this.children = []
  this.attributes = {}
  this.value = ''
  this.textContent = ''
  this.placeholder = ''
  this.disabled = false
 }
 append(...nodes){
  for(const node of nodes){
   this.children.push(node)
  }
 }
 replaceChildren(...nodes){
  this.children = [...nodes]
 }
 setAttribute(key, val){
  this.attributes[key] = String(val)
 }
 getAttribute(key){
  return this.attributes[key]
 }
 removeAttribute(key){
  delete this.attributes[key]
 }
 find(predicate){
  if(predicate(this)) return this
  for(const child of this.children){
   if(child instanceof MockNode){
    const found = child.find(predicate)
    if(found) return found
   }
  }
  return null
 }
 findAll(predicate){
  const out = []
  if(predicate(this)) out.push(this)
  for(const child of this.children){
   if(child instanceof MockNode){
    out.push(...child.findAll(predicate))
   }
  }
  return out
 }
}

globalThis.document = {
 createElement: tag => new MockNode(tag)
}

const validateModel = val => {
 const trimmed = String(val || '').trim()
 if(!trimmed) return true
 return /^[A-Za-z0-9][A-Za-z0-9._:@/-]*$/.test(trimmed)
}

test('skillModelMapping renders inputs and updates fallback text', () => {
 let changed = 0
 const mapping = skillModelMapping({
  validate: validateModel,
  onChange: () => { changed++ }
 })

 const skills = [
  { id: 'clarify', name: 'Clarify' },
  { id: 'implement', name: 'Implement' }
 ]

 mapping.set({ implement: 'claude-sonnet-5' }, skills, {
  engineModel: 'claude-opus-5',
  suggestedModels: ['claude-opus-5', 'claude-sonnet-5', 'claude-haiku-4-5']
 })

 assert.equal(mapping.getEngineModel(), 'claude-opus-5')
 assert.deepEqual(mapping.get(), { implement: 'claude-sonnet-5' })
 assert.equal(mapping.invalid(), false)

 const inputs = mapping.box.findAll(n => n.tagName === 'INPUT' && n.getAttribute('aria-label')?.startsWith('Model for skill '))
 assert.equal(inputs.length, 2)

 const clarifyInput = inputs.find(n => n.getAttribute('aria-label') === 'Model for skill clarify')
 assert.equal(clarifyInput.value, '')
 assert.equal(clarifyInput.placeholder, 'Inherit (claude-opus-5)')

 const implementInput = inputs.find(n => n.getAttribute('aria-label') === 'Model for skill implement')
 assert.equal(implementInput.value, 'claude-sonnet-5')

 // Test invalid model validation
 clarifyInput.value = 'invalid model with spaces'
 clarifyInput.oninput()
 assert.equal(clarifyInput.getAttribute('aria-invalid'), 'true')
 assert.equal(mapping.invalid(), true)
 assert.equal(changed, 1)

 clarifyInput.value = 'claude-haiku-4-5'
 clarifyInput.oninput()
 assert.equal(clarifyInput.getAttribute('aria-invalid'), undefined)
 assert.equal(mapping.invalid(), false)
 assert.equal(changed, 2)
 assert.deepEqual(mapping.get(), { clarify: 'claude-haiku-4-5', implement: 'claude-sonnet-5' })
})
