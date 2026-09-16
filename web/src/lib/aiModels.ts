import type { AIProvider } from '../types'

/**
 * Suggested models per engine. This is presentation data only: the field accepts
 * any identifier, so a model released after this build needs no Sectile release.
 * Engines that take no model flag are absent on purpose.
 */
export const AI_MODEL_SUGGESTIONS: Partial<Record<AIProvider, string[]>> = {
  claude: ['claude-opus-5', 'claude-sonnet-5', 'claude-haiku-4-5'],
  codex: ['gpt-5-codex', 'gpt-5', 'o4-mini'],
  gemini: ['gemini-2.5-pro', 'gemini-2.5-flash'],
  cursor: ['auto', 'claude-sonnet-5', 'gpt-5'],
}

/** Engines Sectile passes `--model` to. The others ignore a configured model. */
export function providerTakesModel(provider?: AIProvider | ''): boolean {
  return provider === 'claude' || provider === 'codex' || provider === 'gemini' || provider === 'cursor'
}

/**
 * Same shape check as the server: an identifier has to be placeable on a command
 * line as a single word. Validating here turns a server 400 into an inline hint.
 */
const MODEL_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._:@/-]*$/

export function isValidModel(value: string): boolean {
  const trimmed = value.trim()
  return trimmed === '' || MODEL_PATTERN.test(trimmed)
}

/** A template owns the whole command line, so no flag is injected beside it. */
export function templateGovernsCommand(provider: AIProvider | '' | undefined, template: string): boolean {
  return template.trim() !== '' && (provider === 'custom' || template.includes('{prompt}'))
}
