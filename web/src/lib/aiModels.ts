import type { EngineReport } from '../types'

/** The report of a project no agent of the caller serves. */
export const UNKNOWN_ENGINE: EngineReport = Object.freeze({ state: 'unknown' }) as EngineReport

/**
 * Reads the body of GET /api/projects/{id}/engine. Anything that is not a
 * report with state "reported" (an older server, an error body) reads as
 * unknown, so the web never announces an engine it was not told about.
 */
export function normalizeEngineReport(data: unknown): EngineReport {
  if (!data || typeof data !== 'object') return UNKNOWN_ENGINE
  const raw = data as Record<string, unknown>
  if (raw.state !== 'reported') return UNKNOWN_ENGINE
  const text = (value: unknown) => (typeof value === 'string' ? value.trim() : '')
  const skillModels: Record<string, string> = {}
  if (raw.skillModels && typeof raw.skillModels === 'object') {
    for (const [skill, model] of Object.entries(raw.skillModels as Record<string, unknown>)) {
      const value = text(model)
      if (value) skillModels[skill] = value
    }
  }
  const models = Array.isArray(raw.models) ? raw.models.map(text).filter(Boolean) : []
  return {
    state: 'reported',
    provider: text(raw.provider),
    model: text(raw.model),
    skillModels,
    models: [...new Set(models)],
    // An omitted flag is an empty value the server dropped, hence false.
    modelSlot: raw.modelSlot === true,
    headless: raw.headless === true,
    reportedAt: text(raw.reportedAt),
  }
}

/**
 * The model the next run of one skill would use on the caller's workstation,
 * as its capability report states it (#305): the per-skill model, else the
 * project-wide one. Empty when the report is unknown, names no model, or the
 * workstation's command line carries no model at all.
 */
export function reportedModel(report: EngineReport | null | undefined, skillId?: string): string {
  if (!report || report.state !== 'reported' || report.modelSlot !== true) return ''
  const skill = (skillId || '').trim()
  if (skill) {
    const perSkill = (report.skillModels?.[skill] || '').trim()
    if (perSkill) return perSkill
  }
  return (report.model || '').trim()
}

/**
 * The models the launch picker offers for one skill: the reported list minus
 * the model the run would use anyway. Empty, so no picker, when the report is
 * unknown or the command line carries no model.
 */
export function reportedPickerModels(report: EngineReport | null | undefined, skillId?: string): string[] {
  if (!report || report.state !== 'reported' || report.modelSlot !== true) return []
  const configured = reportedModel(report, skillId)
  return (report.models || []).filter(model => model && model !== configured)
}

/**
 * Formes courtes des familles connues, pour l'indicateur d'une carte : quatre
 * caractères au plus, sinon il vole la place des boutons d'action. Un modèle
 * hors de cette table garde ses quatre premiers caractères distinctifs.
 */
const SHORT_MODEL_NAMES: Record<string, string> = {
  fable: 'FABL',
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
