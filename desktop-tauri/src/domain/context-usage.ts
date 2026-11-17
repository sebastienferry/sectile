// How full the conversation's context window is, for the composer gauge (#915).

export interface ContextUsage {
  percent: number;
  warning: boolean;
  /** Accessible label and tooltip: `18% · 36,000 / 200,000`, with a /compact hint when nearly full. */
  label: string;
}

/** Above this share of the window the gauge warns and suggests /compact. */
export const CONTEXT_WARNING_PERCENT = 80;

const count = (value: number) => value.toLocaleString("en-US");

/** Undefined while the provider has not reported a window or nothing is used yet. */
export function contextUsage(context: { used?: number; window?: number } | undefined): ContextUsage | undefined {
  const used = Number(context?.used);
  const window = Number(context?.window);
  if (!(used > 0) || !(window > 0)) return undefined;
  const percent = Math.min(100, Math.max(0, Math.round((used * 100) / window)));
  const warning = percent > CONTEXT_WARNING_PERCENT;
  const label = `${percent}% · ${count(used)} / ${count(window)}${warning ? " · Consider /compact" : ""}`;
  return { percent, warning, label };
}
