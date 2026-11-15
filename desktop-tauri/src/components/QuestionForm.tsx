import { useId, useRef, useState, type KeyboardEvent } from "react";
import type { Approval } from "../lib/conversation";
interface Question { question: string; header?: string; multiSelect: boolean; isSecret: boolean; options: { label: string; description?: string }[] }
const record = (value: unknown): value is Record<string, unknown> => !!value && typeof value === "object" && !Array.isArray(value);
export function readQuestions(input: unknown): Question[] | undefined {
  if (!record(input) || !Array.isArray(input.questions) || !input.questions.length || input.questions.length > 32) return;
  const seen = new Set<string>();
  const questions: Question[] = [];
  for (const item of input.questions) {
    if (!record(item) || typeof item.question !== "string" || !item.question.trim() || seen.has(item.question)) return;
    seen.add(item.question);
    const options: Question["options"] = [];
    for (const option of Array.isArray(item.options) ? item.options : []) {
      if (record(option) && typeof option.label === "string" && option.label.trim() && !options.some(value => value.label === option.label)) {
        options.push({ label: option.label, description: typeof option.description === "string" ? option.description : undefined });
      }
    }
    questions.push({ question: item.question, header: typeof item.header === "string" ? item.header : undefined, multiSelect: item.multiSelect === true, isSecret: item.isSecret === true, options });
  }
  return questions;
}
// Answer sent for a question the owner skipped on its own (#904): agents
// require one non-empty answer per question, and the literal tells them apart.
export const SKIPPED_ANSWER = "Skipped by the owner";
// An option the agent recommends ends its label with "(Recommended)"; the
// suffix is shown as a badge, and the original label is what goes back.
export function recommendedLabel(label: string): { text: string; recommended: boolean } {
  const match = /\s*\(Recommended\)\s*$/.exec(label);
  if (!match || !label.slice(0, match.index).trim()) return { text: label, recommended: false };
  return { text: label.slice(0, match.index), recommended: true };
}
type Draft = { selected: string[]; custom: string; skipped: boolean };
type TabState = "to answer" | "answered" | "skipped";
const emptyDraft: Draft = { selected: [], custom: "", skipped: false };
export function QuestionForm({ approval, disabled, onAnswer, onSkip }: {
  approval: Approval; disabled: boolean;
  onAnswer: (answers: Record<string, string>) => Promise<void>;
  onSkip: () => Promise<void>;
}) {
  const questions = readQuestions(approval.input);
  const prefix = useId();
  const [values, setValues] = useState<Record<number, Draft>>({});
  const [active, setActive] = useState(0);
  const [pending, setPending] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState("");
  const sending = useRef(false);
  const tabs = useRef<(HTMLButtonElement | null)[]>([]);
  // Arrow keys move the selection inside a radio group: that browses the
  // options, it does not choose one, so it must not move to the next tab.
  const arrowed = useRef(false);
  const count = questions?.length ?? 0;
  const current = Math.min(active, Math.max(count - 1, 0));
  const joined = (value: Draft | undefined) => [...(value?.selected ?? []), value?.custom.trim() ?? ""].filter(Boolean).join(", ");
  const state = (index: number): TabState => values[index]?.skipped ? "skipped" : joined(values[index]) ? "answered" : "to answer";
  const answers = Object.fromEntries((questions ?? []).map((question, index) => [question.question, values[index]?.skipped ? SKIPPED_ANSWER : joined(values[index])]));
  const size = Object.entries(answers).reduce((total, [key, value]) => total + new TextEncoder().encode(key + value).length, 0);
  const valid = !!questions && Object.values(answers).every(answer => answer.trim()) && size <= 16384;
  const locked = disabled || pending || submitted;
  const update = (index: number, draft: Draft) => setValues(previous => ({ ...previous, [index]: draft }));
  const advance = (index: number) => setActive(Math.min(index + 1, count - 1));
  function select(index: number, focus = true) {
    setActive(index);
    if (focus) tabs.current[index]?.focus();
  }
  function moveTab(event: KeyboardEvent<HTMLDivElement>) {
    const target = { ArrowRight: (current + 1) % count, ArrowLeft: (current - 1 + count) % count, Home: 0, End: count - 1 }[event.key];
    if (target === undefined) return;
    event.preventDefault();
    select(target);
  }
  async function send(skip = false) {
    if (sending.current || locked || (!skip && !valid)) return;
    sending.current = true; setPending(true); setError("");
    try {
      if (skip) await onSkip(); else await onAnswer(answers);
      setValues({}); setSubmitted(true);
    } catch {
      // Do not echo provider errors or submitted values: answers can be secret.
      setError("Answer not confirmed. Refresh status before trying again.");
      if (questions?.some(question => question.isSecret)) setValues({});
    } finally { sending.current = false; setPending(false); }
  }
  const panel = (question: Question, index: number) => {
    const value = values[index] ?? emptyDraft;
    return <fieldset key={question.question} disabled={locked}>
      <legend>{question.header && <small>{question.header} · </small>}{question.question}</legend>
      {question.multiSelect && !question.isSecret && <p className="muted">Multiple choices allowed.</p>}
      {!question.isSecret && question.options.map(option => {
        const label = recommendedLabel(option.label);
        return <label className={label.recommended ? "question-option recommended" : "question-option"} key={option.label}>
          <input type={question.multiSelect ? "checkbox" : "radio"} name={`${prefix}-${index}`} value={option.label} checked={value.selected.includes(option.label)} onChange={event => {
            const selected = question.multiSelect ? event.target.checked ? [...value.selected, option.label] : value.selected.filter(item => item !== option.label) : [option.label];
            update(index, { selected, custom: question.multiSelect ? value.custom : "", skipped: false });
            if (!question.multiSelect && !arrowed.current) advance(index);
            arrowed.current = false;
          }} onKeyDown={event => { arrowed.current = event.key.startsWith("Arrow"); }} />
          <span><strong>{label.text}{label.recommended && <span className="badge question-badge">Recommended</span>}</strong>{option.description && <small>{option.description}</small>}</span>
        </label>;
      })}
      <label className="question-free"><span>{question.options.length && !question.isSecret ? "Other answer" : "Your answer"}</span>
        <input aria-label={`Free response: ${question.question}`} type={question.isSecret ? "password" : "text"} autoComplete="off" maxLength={16000} value={value.custom} onChange={event => {
          update(index, { selected: question.multiSelect && !question.isSecret ? value.selected : [], custom: event.target.value, skipped: false });
        }} />
      </label>
      <div className="question-skip">
        <button type="button" aria-pressed={value.skipped} onClick={() => { update(index, { ...emptyDraft, skipped: true }); advance(index); }}>{value.skipped ? "Question skipped" : "Skip this question"}</button>
      </div>
    </fieldset>;
  };
  const label = (question: Question, index: number) => question.header?.trim() || `Q${index + 1}`;
  return <section className="prompt-approval question-card" aria-label="Agent questions">
    <h3>Your answer is needed</h3>
    {!questions && <p role="alert">This question format is not supported. You can skip the request.</p>}
    <form onSubmit={event => { event.preventDefault(); void send(); }}>
      {questions && count > 1 && <div className="question-tabs" role="tablist" aria-label="Questions" onKeyDown={moveTab}>
        {questions.map((question, index) => <button key={question.question} ref={element => { tabs.current[index] = element; }} type="button" role="tab" id={`${prefix}-tab-${index}`} aria-controls={`${prefix}-panel`} aria-selected={index === current} tabIndex={index === current ? 0 : -1} aria-label={`${label(question, index)}, ${state(index)}`} data-state={state(index)} onClick={() => select(index, false)}>
          <span>{label(question, index)}</span><span className="question-tab-state" aria-hidden="true">{state(index) === "answered" ? "✓" : state(index) === "skipped" ? "Skipped" : ""}</span>
        </button>)}
      </div>}
      {questions && count > 1 && <div role="tabpanel" id={`${prefix}-panel`} aria-labelledby={`${prefix}-tab-${current}`}>{panel(questions[current], current)}</div>}
      {questions && count === 1 && panel(questions[0], 0)}
      {size > 16384 && <p role="alert">Answers exceed the 16 KB limit.</p>}
      {error && <p role="alert">{error}</p>}
      <div className="question-actions">
        <button type="button" disabled={locked} onClick={() => void send(true)}>Skip request</button>
        <button className="primary" type="submit" disabled={locked || !valid}>{pending ? "Sending…" : submitted ? "Answer sent" : "Send answers"}</button>
      </div>
    </form>
  </section>;
}
