import type { AIProvider } from '../types'

/**
 * Values sent by the project modal for its optional agent override.
 *
 * The API distinguishes an absent field (keep the existing value) from an
 * explicit empty value (clear it). Disabling the override must therefore send
 * empty provider and model fields rather than omitting them.
 */
export function projectAgentSettings(
  useCustomAgent: boolean,
  aiProvider: AIProvider | '',
  aiModel: string,
): { aiProvider: AIProvider | ''; aiModel: string } {
  return {
    aiProvider: useCustomAgent ? aiProvider : '',
    aiModel: useCustomAgent ? aiModel.trim() : '',
  }
}

/** A persisted provider or model means the modal should show the project override. */
export function hasProjectAgentOverride(aiProvider?: string, aiModel?: string): boolean {
  return Boolean(aiProvider || aiModel)
}
