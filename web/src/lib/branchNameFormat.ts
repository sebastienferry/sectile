/**
 * The branch a project's branch name format gives a task (#621).
 *
 * The server validates the format and the agent renders it
 * (internal/models/branchformat.go); this mirrors both so the project settings
 * can show the branch a format gives before it is saved. Drifting from the Go
 * side shows a wrong example, never a wrong branch, but keep the two in step.
 */

/** Today's branch names, used when a project sets no format. */
export const DEFAULT_BRANCH_NAME_FORMAT = 'feat/{key_lower}'

/** The formats the settings offer in one click, the default first. */
export const BRANCH_NAME_PRESETS = [DEFAULT_BRANCH_NAME_FORMAT, '{key}', 'feat/{key}-{title}'] as const

/** The sample task the settings render a format for, as the server does. */
export const BRANCH_NAME_SAMPLE = { key: 'AUC-1234', title: 'Sample title' } as const

const PLACEHOLDERS = ['key', 'key_lower', 'title']
const SEPARATORS = /[-_./]+/
const TITLE_SLUG_MAX = 30

/** Why a format gives no usable branch; the settings word each reason. */
export type BranchNameProblem = 'brace' | 'placeholder' | 'key' | 'git'

export type BranchNameRender =
  | { ok: true; branch: string }
  | { ok: false; problem: BranchNameProblem; detail?: string }

type Token = { literal: string } | { placeholder: string }

function parse(format: string): Token[] | { problem: BranchNameProblem; detail?: string } {
  const tokens: Token[] = []
  let rest = format
  while (rest !== '') {
    const open = rest.search(/[{}]/)
    if (open < 0) {
      tokens.push({ literal: rest })
      break
    }
    if (rest[open] === '}') return { problem: 'brace' }
    if (open > 0) tokens.push({ literal: rest.slice(0, open) })
    const end = rest.slice(open + 1).search(/[{}]/)
    if (end < 0 || rest[open + 1 + end] === '{') return { problem: 'brace' }
    const name = rest.slice(open + 1, open + 1 + end)
    if (!PLACEHOLDERS.includes(name)) return { problem: 'placeholder', detail: `{${name}}` }
    tokens.push({ placeholder: name })
    rest = rest.slice(open + 1 + end + 1)
  }
  return tokens
}

/** Letters, digits and dashes of a key, anything else a dash, trimmed. */
function keySlug(key: string): string {
  return key.replace(/[^A-Za-z0-9-]/gu, '-').replace(/^-+|-+$/g, '')
}

/** Lower-case title slug, as macro branches cut it. */
export function titleSlug(title: string): string {
  let slug = title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
  if (slug.length > TITLE_SLUG_MAX) slug = slug.slice(0, TITLE_SLUG_MAX).replace(/^-+|-+$/g, '')
  return slug
}

/** The rules of `git check-ref-format --branch` a rendered name can break. */
export function validBranchName(name: string): boolean {
  if (name === '' || name === '@' || name === 'HEAD') return false
  if (/^[-/]|[/.]$/.test(name) || /\.\.|@\{|\/\//.test(name)) return false
  // eslint-disable-next-line no-control-regex
  if (/[\u0000-\u001f\u007f ~^:?*[\\]/.test(name)) return false
  return name.split('/').every(part => !part.startsWith('.') && !part.endsWith('.lock'))
}

/** The branch a format gives a task; an empty format is the default one. */
export function renderBranchNameFormat(format: string, key: string, title: string): BranchNameRender {
  const trimmed = format.trim() || DEFAULT_BRANCH_NAME_FORMAT
  const tokens = parse(trimmed)
  if (!Array.isArray(tokens)) return { ok: false, ...tokens }
  const slug = keySlug(key)
  if (slug === '') return { ok: false, problem: 'git' }
  let out = ''
  let trimNext = false
  for (const token of tokens) {
    let value = 'literal' in token ? token.literal : ''
    if ('placeholder' in token) {
      if (token.placeholder === 'key') value = slug
      if (token.placeholder === 'key_lower') value = slug.toLowerCase()
      if (token.placeholder === 'title') {
        value = titleSlug(title)
        if (value === '') {
          if (out === '' || out.endsWith('/')) trimNext = true
          else out = out.replace(new RegExp(SEPARATORS.source + '$'), '')
          continue
        }
      }
    }
    if (trimNext) {
      value = value.replace(new RegExp('^' + SEPARATORS.source), '')
      trimNext = value === ''
    }
    out += value
  }
  if (trimNext) out = out.replace(new RegExp(SEPARATORS.source + '$'), '')
  return { ok: true, branch: out }
}

/**
 * What the server says of a format on save, rendered for the sample task: the
 * branch it gives, or why it is refused. An empty format is the default.
 */
export function checkBranchNameFormat(format: string): BranchNameRender {
  const trimmed = format.trim()
  const tokens = parse(trimmed || DEFAULT_BRANCH_NAME_FORMAT)
  if (!Array.isArray(tokens)) return { ok: false, ...tokens }
  if (trimmed !== '' && !tokens.some(t => 'placeholder' in t && (t.placeholder === 'key' || t.placeholder === 'key_lower'))) {
    return { ok: false, problem: 'key' }
  }
  const render = renderBranchNameFormat(trimmed, BRANCH_NAME_SAMPLE.key, BRANCH_NAME_SAMPLE.title)
  if (!render.ok) return render
  if (!validBranchName(render.branch) || render.branch === 'main' || render.branch === 'master') {
    return { ok: false, problem: 'git', detail: render.branch }
  }
  return render
}
