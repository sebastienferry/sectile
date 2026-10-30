import { useId, useRef, useState } from "react";
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
export function QuestionForm({ approval, disabled, onAnswer, onSkip }: {
  approval: Approval; disabled: boolean;
  onAnswer: (answers: Record<string, string>) => Promise<void>;
  onSkip: () => Promise<void>;
}) {
  const questions = readQuestions(approval.input);
  const prefix = useId();
  const [values, setValues] = useState<Record<number, { selected: string[]; custom: string }>>({});
  const [pending, setPending] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState("");
  const sending = useRef(false);
  const answers = Object.fromEntries((questions ?? []).map((question, index) => {
    const value = values[index];
    return [question.question, [...(value?.selected ?? []), value?.custom.trim() ?? ""].filter(Boolean).join(", ")];
  }));
  const size = Object.entries(answers).reduce((total, [key, value]) => total + new TextEncoder().encode(key + value).length, 0);
  const valid = !!questions && Object.values(answers).every(answer => answer.trim()) && size <= 16384;
  const locked = disabled || pending || submitted;
  async function send(skip = false) {
    if (sending.current || locked || (!skip && !valid)) return;
    sending.current = true; setPending(true); setError("");
    try {
      if (skip) await onSkip(); else await onAnswer(answers);
      setValues({}); setSubmitted(true);
    } catch {
      // Do not echo provider errors or submitted values: answers can be secret.
      setError("Réponse non confirmée. Actualisez le suivi avant de réessayer.");
      if (questions?.some(question => question.isSecret)) setValues({});
    } finally { sending.current = false; setPending(false); }
  }
  return <section className="prompt-approval question-card" aria-label="Questions de l’agent">
    <h3>Votre réponse est attendue</h3>
    {!questions && <p role="alert">Le format de cette question n’est pas pris en charge. Vous pouvez passer la demande.</p>}
    <form onSubmit={event => { event.preventDefault(); void send(); }}>
      {questions?.map((question, index) => {
        const value = values[index] ?? { selected: [], custom: "" };
        return <fieldset key={question.question} disabled={locked}>
          <legend>{question.header && <small>{question.header} · </small>}{question.question}</legend>
          {question.multiSelect && !question.isSecret && <p className="muted">Plusieurs choix possibles.</p>}
          {!question.isSecret && question.options.map(option => <label className="question-option" key={option.label}>
            <input type={question.multiSelect ? "checkbox" : "radio"} name={`${prefix}-${index}`} checked={value.selected.includes(option.label)} onChange={event => {
              const selected = question.multiSelect ? event.target.checked ? [...value.selected, option.label] : value.selected.filter(label => label !== option.label) : [option.label];
              setValues(previous => ({ ...previous, [index]: { selected, custom: question.multiSelect ? value.custom : "" } }));
            }} />
            <span><strong>{option.label}</strong>{option.description && <small>{option.description}</small>}</span>
          </label>)}
          <label className="question-free"><span>{question.options.length && !question.isSecret ? "Autre réponse" : "Votre réponse"}</span>
            <input aria-label={`Réponse libre : ${question.question}`} type={question.isSecret ? "password" : "text"} autoComplete="off" maxLength={16000} value={value.custom} onChange={event => {
              setValues(previous => ({ ...previous, [index]: { selected: question.multiSelect && !question.isSecret ? value.selected : [], custom: event.target.value } }));
            }} />
          </label>
        </fieldset>;
      })}
      {size > 16384 && <p role="alert">Les réponses dépassent la limite de 16 Ko.</p>}
      {error && <p role="alert">{error}</p>}
      <div className="question-actions">
        <button type="button" disabled={locked} onClick={() => void send(true)}>Passer la demande</button>
        <button className="primary" type="submit" disabled={locked || !valid}>{pending ? "Envoi…" : submitted ? "Réponse envoyée" : "Envoyer les réponses"}</button>
      </div>
    </form>
  </section>;
}
