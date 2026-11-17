import { contextUsage } from "../domain/context-usage";

// A 240° arc from 150° to 390° on a 16×16 viewBox, radius 6: open at the bottom.
const ARC = "M2.804 11 A6 6 0 1 1 13.196 11";
const ARC_LENGTH = 6 * (240 * Math.PI / 180);

/** The share of the model's context window the conversation uses, beside the model selector (#915). */
export function ContextUsageMeter({ context }: { context?: { used: number; window: number } }) {
  const usage = contextUsage(context);
  if (!usage) return null;
  return <span
    className={`context-meter${usage.warning ? " warning" : ""}`}
    role="progressbar"
    aria-valuemin={0}
    aria-valuemax={100}
    aria-valuenow={usage.percent}
    aria-label={`Context usage: ${usage.label}`}
    title={usage.label}
  >
    <svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
      <path className="context-meter-track" d={ARC} />
      <path className="context-meter-fill" d={ARC} strokeDasharray={ARC_LENGTH} strokeDashoffset={ARC_LENGTH * (1 - usage.percent / 100)} />
    </svg>
    <span className="context-meter-percent">{usage.percent}%</span>
  </span>;
}
