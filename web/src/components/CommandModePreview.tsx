import { commandPreview } from '../lib/commandTemplate'

/**
 * The two command lines a launch produces, side by side.
 *
 * A single template field hides that the two execution modes run different
 * commands: interactive opens the CLI, autonomous adds its non-interactive
 * approval flag. Showing both is what makes the difference configurable at all,
 * and it is where a template that cannot run headless says so before a run.
 */
export function CommandModePreview({
  provider,
  template,
  model,
  autonomousTemplate = '',
}: {
  provider: string
  template: string
  model: string
  /** The command written for headless launches, when one is configured. */
  autonomousTemplate?: string
}) {
  const modes: { label: string; autonomous: boolean }[] = [
    { label: 'Interactif', autonomous: false },
    { label: 'Autonome', autonomous: true },
  ]

  return (
    <div className="mt-2 rounded-xl border border-[var(--border-color)] bg-[var(--bg-primary)] p-2.5 space-y-1.5">
      <div className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
        Commande exécutée
      </div>
      {modes.map(mode => {
        const preview = commandPreview(provider, template, model, mode.autonomous, autonomousTemplate)
        return (
          <div key={mode.label} className="flex items-start gap-2">
            <span className="shrink-0 w-16 pt-0.5 text-[10px] font-semibold text-[var(--text-secondary)]">
              {mode.label}
            </span>
            {preview.command ? (
              <code className="min-w-0 flex-1 break-all font-mono text-[10.5px] text-[var(--text-primary)]">
                {preview.command}
              </code>
            ) : (
              <span className="min-w-0 flex-1 text-[10.5px] leading-relaxed" style={{ color: 'var(--status-warn)' }}>
                {preview.error}
              </span>
            )}
          </div>
        )
      })}
    </div>
  )
}
