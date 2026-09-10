/** Match the provider-specific invocation sent by the PTY endpoint. */
export function terminalSkillCommand(provider: string, command: string, override?: string): string {
  const name = (override?.trim() || command.trim()).replace(/^\//, '')
  return provider.trim().toLowerCase() === 'codex' ? name : `/${name}`
}
