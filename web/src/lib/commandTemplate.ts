/**
 * The command line a launch would run, in each execution mode.
 *
 * The agent builds the real thing (internal/agent/agent_config.go); this mirrors
 * it so the settings screens can show both modes before anything runs. Drifting
 * from the agent shows a wrong preview, never a wrong run, but the point of the
 * preview is to be right: keep the two in step.
 */

/** How a template says which words depend on the mode: {mode:AUTONOMOUS|INTERACTIVE}. */
export const TEMPLATE_MODE_PLACEHOLDER = '{mode:'

/** Stand-in for the instructions, kept literal so the preview stays readable. */
const PROMPT = "'{prompt}'"

/** The optional slot a command template may carry for the resolved model. */
const MODEL_PLACEHOLDER = '{model}'

/** Providers whose model is passed as --model; the others take none. */
const MODEL_FLAG_PROVIDERS = new Set(['claude', 'codex', 'gemini', 'cursor'])

export interface CommandPreview {
  /** The command line, empty when the configuration cannot produce one. */
  command: string
  /** Why no command line exists, for the mode that cannot run. */
  error?: string
}

/**
 * A template owns the mode only by declaring the placeholder. Without it the
 * template can only run what its author wrote, which is why the agent refuses
 * an autonomous launch rather than reinterpreting the command silently.
 */
export function templateCarriesMode(template: string): boolean {
  return template.includes(TEMPLATE_MODE_PLACEHOLDER)
}

/** Picks one side of every mode placeholder, autonomous on the left. */
export function resolveTemplateMode(template: string, autonomous: boolean): string {
  let rest = template
  let out = ''
  for (;;) {
    const start = rest.indexOf(TEMPLATE_MODE_PLACEHOLDER)
    if (start < 0) return out + rest
    const end = rest.indexOf('}', start)
    if (end < 0) return out + rest
    const body = rest.slice(start + TEMPLATE_MODE_PLACEHOLDER.length, end)
    const bar = body.indexOf('|')
    const chosen = autonomous
      ? (bar < 0 ? body : body.slice(0, bar))
      : (bar < 0 ? '' : body.slice(bar + 1))
    out += rest.slice(0, start) + chosen
    rest = rest.slice(end + 1)
  }
}

/** The --model pair to splice in, empty when the provider or the model has none. */
export function modelArgs(provider: string, model: string): string[] {
  const resolved = model.trim()
  if (!resolved || !MODEL_FLAG_PROVIDERS.has(provider.trim().toLowerCase())) return []
  return ['--model', resolved]
}

/** Wraps a value so a shell reads it literally, as the agent does. */
function shellQuote(value: string): string {
  return "'" + value.replace(/'/g, "'\\''") + "'"
}

/**
 * Removes every {model} slot a template has nothing to put in, along with the
 * option the slot is the value of. A flag left with nothing behind it does not
 * disappear, it consumes the next word, so removing the pair is the only way the
 * command line survives an unset model. Mirrors agentconfig.DropModelSlot.
 */
export function dropModelSlot(template: string): string {
  for (;;) {
    const at = template.indexOf(MODEL_PLACEHOLDER)
    if (at < 0) return template
    let start = at
    while (start > 0 && !' \t'.includes(template[start - 1])) start--
    let end = at + MODEL_PLACEHOLDER.length
    while (end < template.length && !' \t'.includes(template[end])) end++
    let cut = start
    const before = template.slice(0, start).replace(/[ \t]+$/, '')
    if (before !== '') {
      let option = before.length
      while (option > 0 && !' \t'.includes(before[option - 1])) option--
      if (before.slice(option).startsWith('-')) cut = option
    }
    const head = template.slice(0, cut).replace(/[ \t]+$/, '')
    const tail = template.slice(end).replace(/^[ \t]+/, '')
    template = head === '' ? tail : tail === '' ? head : head + ' ' + tail
  }
}

/**
 * Fills {model} the way the agent does: quoting belongs to the template, so the
 * slot is quoted only where it sits outside quotes.
 */
function expandModel(template: string, model: string): string {
  const value = model.trim()
  if (value === '') return dropModelSlot(template)
  let out = ''
  let quote = ''
  for (let i = 0; i < template.length;) {
    if (template.startsWith(MODEL_PLACEHOLDER, i)) {
      out += quote === "'" ? value.replace(/'/g, "'\\''")
        : quote === '"' ? '"' + shellQuote(value) + '"'
          : shellQuote(value)
      i += MODEL_PLACEHOLDER.length
      continue
    }
    const ch = template[i]
    if (ch === "'" || ch === '"') {
      if (quote === '') quote = ch
      else if (quote === ch) quote = ''
    }
    out += ch
    i++
  }
  return out
}

/**
 * A template runs only when it carries the instructions. A named provider given
 * a template without {prompt} falls back to its own command, because legacy rows
 * hold a bare CLI name there; a custom provider has nothing else to fall back to.
 */
function usesTemplate(provider: string, template: string): boolean {
  return template !== '' && (provider.trim().toLowerCase() === 'custom' || template.includes('{prompt}'))
}

/** Joins the words of an invocation, dropping the ones that resolve to nothing. */
function words(...parts: string[]): string {
  return parts.filter(part => part !== '').join(' ')
}

/**
 * The command line for one mode. An autonomous run also carries the provider's
 * non-interactive approval flag: nobody is there to answer a permission prompt,
 * and without it the CLI is denied the very tools that report the run.
 *
 * `autonomousTemplate` is the command written for headless launches. It answers
 * for itself, needing no {mode:...} marker, because its author wrote it for that
 * mode; only the general command, asked to serve a mode it may not have been
 * written for, has to declare that it can.
 */
export function commandPreview(
  provider: string,
  template: string,
  model: string,
  autonomous: boolean,
  autonomousTemplate = '',
): CommandPreview {
  const cli = provider.trim().toLowerCase()
  const dedicated = autonomousTemplate.trim()
  if (autonomous && usesTemplate(cli, dedicated)) {
    return { command: expandModel(resolveTemplateMode(dedicated, true), model).replace(/ {2,}/g, ' ').trim() }
  }
  const trimmed = template.trim()

  if (usesTemplate(cli, trimmed)) {
    if (autonomous && !templateCarriesMode(trimmed)) {
      return {
        command: '',
        error: 'This command decides the mode itself. Set an autonomous command, or add a {mode:AUTONOMOUS|INTERACTIVE} placeholder to this one.',
      }
    }
    return { command: expandModel(resolveTemplateMode(trimmed, autonomous), model).replace(/ {2,}/g, ' ').trim() }
  }

  const flag = modelArgs(cli, model).join(' ')
  if (autonomous) {
    switch (cli) {
      case 'claude':
        return { command: words('claude', '-p', '--permission-mode', 'bypassPermissions', flag, PROMPT) }
      case 'codex':
        return { command: words('codex', 'exec', flag, PROMPT) }
      case 'vibe':
        return { command: 'vibe -p --auto-approve ' + PROMPT }
      default:
        return {
          command: '',
          error: `${cli || 'This provider'} has no attested headless mode. Run interactively, or write a template carrying {mode:AUTONOMOUS|INTERACTIVE}.`,
        }
    }
  }

  switch (cli) {
    case 'agy':
      return { command: 'agy -i ' + PROMPT }
    case 'claude':
    case 'codex':
    case 'gemini':
      return { command: words(cli, flag, PROMPT) }
    case 'vibe':
      return { command: 'vibe -p ' + PROMPT }
    case 'cursor':
      return { command: words('cursor', 'agent', flag, PROMPT) }
    default:
      return { command: '', error: `Unsupported provider ${cli || '(none)'}: configure an AI command template.` }
  }
}
