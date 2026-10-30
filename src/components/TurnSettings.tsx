import { useEffect, useId, useState } from "react";
import { providerModels, type ConversationSnapshot, type TurnOptions } from "../lib/conversation";
export function TurnSettings({ provider, snapshot, value, onChange, disabled, compact = false }: {
  provider?: string; snapshot?: ConversationSnapshot; value: TurnOptions; onChange: (options: TurnOptions) => void; disabled: boolean; compact?: boolean;
}) {
  const [catalogue, setCatalogue] = useState<string[]>([]);
  const [unavailable, setUnavailable] = useState(false);
  const id = useId();
  useEffect(() => {
    let mounted = true;
    setCatalogue([]); setUnavailable(false);
    if (provider === "claude" || provider === "codex") void providerModels(provider).then(models => { if (mounted) setCatalogue(Array.isArray(models) ? models.filter(model => typeof model === "string") : []); }).catch(() => { if (mounted) setUnavailable(true); });
    return () => { mounted = false; };
  }, [provider]);
  // Codex's live catalogue is scoped to the signed-in account. Once it is
  // available, workstation suggestions must not add models the CLI may reject.
  const snapshotModels = snapshot?.models?.map(model => model.model) ?? [];
  const suggestions = provider === "codex" && Array.isArray(snapshot?.models) ? snapshotModels : [...snapshotModels, ...catalogue];
  const models = [...new Set([...suggestions, ...(snapshot?.model ? [snapshot.model] : [])])];
  const currentMode = snapshot?.mode === "plan" ? "Plan" : snapshot?.mode === "read-only" ? "Lecture seule" : "Exécution";
  return <fieldset className={`turn-settings ${compact ? "compact" : ""}`} disabled={disabled}>
    <legend className={compact ? "sr-only" : ""}>Prochain message</legend>
    <label><span className={compact ? "composer-option-label" : ""}>Modèle</span><select aria-label="Modèle" value={value.model === undefined ? "" : models.includes(value.model) ? value.model : "__custom__"} onChange={event => onChange({ ...value, model: event.target.value === "__custom__" ? value.model ?? "" : event.target.value || undefined })}>
      <option value="">{snapshot?.model ? compact ? snapshot.model : `Conserver : ${snapshot.model}` : compact ? "Modèle du projet" : "Configuration du projet / fournisseur"}</option>
      {models.map(model => <option value={model} key={model}>{model}</option>)}
      <option value="__custom__">Identifiant personnalisé…</option>
    </select></label>
    {value.model !== undefined && !models.includes(value.model) && <label htmlFor={id}>Identifiant du modèle<input id={id} required pattern={String.raw`[A-Za-z0-9][A-Za-z0-9._:@\/\-]*`} maxLength={256} value={value.model} placeholder="Identifiant accepté par votre CLI" onChange={event => onChange({ ...value, model: event.target.value })} /></label>}
    <label><span className={compact ? "composer-option-label" : ""}>Mode</span><select aria-label="Mode de conversation" value={value.mode ?? ""} onChange={event => onChange({ ...value, mode: event.target.value ? event.target.value as "plan" | "execute" : undefined })}>
      <option value="">{snapshot?.mode ? compact ? currentMode : `Conserver : ${currentMode}` : compact ? "Mode du projet" : "Configuration actuelle"}</option>
      <option value="plan">Plan</option><option value="execute">Exécution</option>
    </select></label>
    <p className="muted turn-settings-hint" hidden={compact && !unavailable && !value.mode}>Choix appliqués à l’envoi. {value.mode === "execute" ? provider === "claude" ? "Claude demandera les autorisations nécessaires." : "Codex pourra modifier le dossier avec son sandbox et ses demandes d’autorisation." : "Les permissions restent gérées par l’agent."}{unavailable && " Catalogue indisponible : vous pouvez saisir un identifiant ou conserver le modèle actuel."}</p>
  </fieldset>;
}
