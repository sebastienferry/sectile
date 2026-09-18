import type { AIProvider, Project, UserSettings } from '../types'

/**
 * Models Sectile ships for each engine, used while the settings configure none.
 * Configuring a provider replaces its list rather than adding to it, so a model
 * removed in the interface really disappears from the launch surfaces.
 * Engines that take no model flag are absent on purpose.
 */
export const DEFAULT_PROVIDER_MODELS: Partial<Record<AIProvider, string[]>> = {
  claude: ['claude-opus-5', 'claude-sonnet-5', 'claude-haiku-4-5'],
  codex: ['gpt-5-codex', 'gpt-5', 'o4-mini'],
  gemini: ['gemini-2.5-pro', 'gemini-2.5-flash'],
  cursor: ['auto', 'claude-sonnet-5', 'gpt-5'],
}

/**
 * The models offered for one provider: what the global settings configure for
 * it, or the shipped list when they configure nothing. This is the single
 * source of what can be picked at launch and of what the configuration fields
 * suggest, so a model added once is available everywhere.
 */
export function providerModels(settings: Partial<Pick<UserSettings, 'aiProviderModels'>> | undefined, provider: AIProvider | '' | undefined): string[] {
  if (!provider) return []
  // Une liste vide est un choix, « ne rien proposer pour ce moteur », et non une
  // absence de réglage : seule l'absence de clé retombe sur la liste livrée.
  const configured = settings?.aiProviderModels?.[provider]
  if (configured) return configured
  return DEFAULT_PROVIDER_MODELS[provider as AIProvider] || []
}

/**
 * The model the configured levels resolve for one task and one skill: the most
 * specific statement wins, so a per-skill entry outranks a bare model whatever
 * level it sits on, and the project outranks the global settings.
 *
 * The workstation override is invisible from here: only the agent reads it,
 * which is why the launch surfaces say so rather than presenting this as the
 * certain answer.
 */
export function resolveConfiguredModel(
  project: Partial<Pick<Project, 'aiModel' | 'aiSkillModels'>> | undefined,
  settings: Partial<Pick<UserSettings, 'aiModel' | 'aiSkillModels'>> | undefined,
  skillId?: string,
): string {
  const skill = (skillId || '').trim()
  if (skill) {
    const fromProject = (project?.aiSkillModels?.[skill] || '').trim()
    if (fromProject) return fromProject
    const fromSettings = (settings?.aiSkillModels?.[skill] || '').trim()
    if (fromSettings) return fromSettings
  }
  return (project?.aiModel || '').trim() || (settings?.aiModel || '').trim()
}

/** The provider a task runs on: its project's, else the global setting. */
export function taskProvider(
  project: Partial<Pick<Project, 'aiProvider'>> | undefined,
  settings: Partial<Pick<UserSettings, 'aiProvider'>> | undefined,
): AIProvider | '' {
  return (project?.aiProvider || settings?.aiProvider || '') as AIProvider | ''
}

/**
 * Formes courtes des familles connues, pour l'indicateur d'une carte : quatre
 * caractères au plus, sinon il vole la place des boutons d'action. Un modèle
 * hors de cette table garde ses quatre premiers caractères distinctifs.
 */
const SHORT_MODEL_NAMES: Record<string, string> = {
  opus: 'OPUS',
  sonnet: 'SONN',
  haiku: 'HAIK',
  codex: 'CDX',
  flash: 'FLSH',
  pro: 'PRO',
  mini: 'MINI',
  auto: 'AUTO',
}

/** Segments qui nomment le fournisseur plutôt que le modèle. */
const VENDOR_SEGMENTS = new Set(['claude', 'anthropic', 'gemini', 'google', 'openai', 'gpt', 'o'])

/**
 * L'étiquette courte d'un modèle, telle qu'une carte l'affiche à côté de ses
 * boutons : « claude-opus-5 » devient OPUS, « gpt-5-codex » devient CDX. Vide
 * pour un modèle vide, ce qui veut dire qu'il n'y a rien à annoncer.
 */
export function shortModelLabel(model: string): string {
  const segments = (model || '')
    .trim()
    .toLowerCase()
    .split('/')
    .pop()!
    .split(/[-_.:]/)
    .filter(Boolean)
  if (segments.length === 0) return ''
  for (const segment of segments) {
    if (SHORT_MODEL_NAMES[segment]) return SHORT_MODEL_NAMES[segment]
  }
  // Rien de connu : le premier segment qui ne nomme pas le fournisseur, sinon
  // les deux premiers collés, ce qui garde « gpt-5 » lisible en GPT5.
  const distinctive = segments.find(segment => !VENDOR_SEGMENTS.has(segment))
  const base = distinctive || segments.slice(0, 2).join('')
  return base.slice(0, 4).toUpperCase()
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
