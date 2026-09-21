export interface CommandPreset {
  label: string
  cmd: string
  autonomous: string
}


function getEnv(key: string, fallback: string, viteFallbackKey?: string): string {
  if (typeof import.meta !== 'undefined' && (import.meta as any).env) {
    const val = (import.meta as any).env[key] || (viteFallbackKey ? (import.meta as any).env[viteFallbackKey] : undefined)
    if (val) return val
  }
  const proc = typeof globalThis !== 'undefined' ? (globalThis as any).process : undefined
  if (proc && proc.env) {
    const val = proc.env[key] || (viteFallbackKey ? proc.env[viteFallbackKey] : undefined)
    if (val) return val
  }
  return fallback
}

export function getCommandPresets(): CommandPreset[] {
  return [
    { label: 'Défaut du fournisseur', cmd: '', autonomous: '' },
    {
      label: 'Claude',
      cmd: getEnv(
        'SECTILE_PRESET_CLAUDE_CMD',
        "claude --model {model} '{prompt}'",
        'VITE_PRESET_CLAUDE_CMD',
      ),
      autonomous: getEnv(
        'SECTILE_PRESET_CLAUDE_AUTONOMOUS',
        "claude -p --permission-mode bypassPermissions --model {model} '{prompt}'",
        'VITE_PRESET_CLAUDE_AUTONOMOUS',
      ),
    },
    {
      label: 'AGY',
      cmd: getEnv(
        'SECTILE_PRESET_AGY_CMD',
        'agy --dangerously-skip-permissions --model {model} "{prompt}"',
        'VITE_PRESET_AGY_CMD',
      ),
      autonomous: getEnv(
        'SECTILE_PRESET_AGY_AUTONOMOUS',
        `agy --dangerously-skip-permissions --model {model} --output-format stream-json -p "{prompt}" | jq -rs 'map(select(.event == "result"))[0].result.response'`,
        'VITE_PRESET_AGY_AUTONOMOUS',
      ),
    },
    {
      label: 'Codex',
      cmd: getEnv(
        'SECTILE_PRESET_CODEX_CMD',
        "codex --model {model} '{prompt}'",
        'VITE_PRESET_CODEX_CMD',
      ),
      autonomous: getEnv(
        'SECTILE_PRESET_CODEX_AUTONOMOUS',
        "codex exec --model {model} '{prompt}'",
        'VITE_PRESET_CODEX_AUTONOMOUS',
      ),
    },
  ]
}

export const COMMAND_PRESETS = getCommandPresets()

