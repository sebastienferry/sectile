export const mcpProviders: Record<string, {label: string; path: string}>
export function mcpSnippet(provider: string, transport: string, server: string, local?: boolean): string
export function mcpCommand(provider: string, transport: string, server: string, local?: boolean): string | null
export function shellQuote(value: string): string
