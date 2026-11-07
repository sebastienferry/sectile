import { useEffect, useId, useRef, useState } from "react";
import { providerModels, type ConversationMode, type ConversationSnapshot, type TurnOptions } from "../lib/conversation";
import { Icon } from "./Icon";
import { ProviderMark } from "./ProviderMark";

const EFFORT_LEVELS = ["Low", "Medium", "High", "Max"] as const;
type EffortLevel = (typeof EFFORT_LEVELS)[number];

export interface ComposerEngineChoice {
  key: string;
  label: string;
  provider: "claude" | "codex";
}

function conversationModeLabel(mode?: string): string {
  switch (mode) {
    case "plan":
      return "Plan";
    case "read-only":
      return "Lecture seule";
    case "acceptEdits":
      return "Accepter les modifications";
    case "auto":
      return "Auto (YOLO)";
    case "default":
    case "execute":
    case "workspace-write":
    default:
      return "Exécution";
  }
}

export function TurnSettings({
  provider,
  snapshot,
  defaultModel,
  value,
  onChange,
  disabled,
  compact = false,
  showProviderMark = false,
  engineChoices,
  selectedEngineKey,
  onSelectEngine,
}: {
  provider?: string;
  snapshot?: ConversationSnapshot;
  defaultModel?: string;
  value: TurnOptions;
  onChange: (options: TurnOptions) => void;
  disabled: boolean;
  compact?: boolean;
  showProviderMark?: boolean;
  engineChoices?: ComposerEngineChoice[];
  selectedEngineKey?: string;
  onSelectEngine?: (key: string) => void;
}) {
  const [catalogue, setCatalogue] = useState<string[]>([]);
  const [unavailable, setUnavailable] = useState(false);
  const [openMenu, setOpenMenu] = useState<"model" | "mode" | null>(null);
  const [effort, setEffort] = useState<EffortLevel>("Medium");
  const rootRef = useRef<HTMLFieldSetElement>(null);
  const id = useId();
  useEffect(() => {
    let mounted = true;
    setCatalogue([]); setUnavailable(false);
    if (provider === "claude" || provider === "codex") void Promise.resolve().then(() => providerModels(provider)).then(models => { if (mounted) setCatalogue(Array.isArray(models) ? models.filter(model => typeof model === "string") : []); }).catch(() => { if (mounted) setUnavailable(true); });
    return () => { mounted = false; };
  }, [provider]);
  useEffect(() => {
    if (!openMenu) return;
    function onPointerDown(event: MouseEvent) {
      if (rootRef.current && !rootRef.current.contains(event.target as Node)) setOpenMenu(null);
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setOpenMenu(null);
    }
    document.addEventListener("mousedown", onPointerDown);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [openMenu]);
  useEffect(() => {
    if (disabled) setOpenMenu(null);
  }, [disabled]);
  // Codex's live catalogue is scoped to the signed-in account. Once it is
  // available, workstation suggestions must not add models the CLI may reject.
  const snapshotModels = snapshot?.models?.map(model => model.model) ?? [];
  const suggestions = provider === "codex" && Array.isArray(snapshot?.models) ? snapshotModels : [...snapshotModels, ...catalogue];
  const models = [...new Set([...suggestions, ...(snapshot?.model ? [snapshot.model] : []), ...(defaultModel ? [defaultModel] : [])])];
  const currentMode = conversationModeLabel(snapshot?.mode);
  const activeProvider = (provider ?? snapshot?.provider) === "codex" ? "codex" : "claude";
  const effectiveMode = value.mode ?? (snapshot?.mode ? snapshot.mode : "execute");
  const isAlertMode = effectiveMode === "execute" || effectiveMode === "default" || effectiveMode === "acceptEdits" || effectiveMode === "auto" || effectiveMode === "workspace-write";
  const modelSelectValue = value.model === undefined ? "" : models.includes(value.model) ? value.model : "__custom__";
  const defaultModelLabel = snapshot?.model || defaultModel || (engineChoices?.find(item => item.key === selectedEngineKey)?.label) || (activeProvider === "codex" ? "Codex" : "Claude Code");
  const modelDisplayLabel = modelSelectValue === "__custom__" ? (value.model || "Personnalisé…") : (modelSelectValue || defaultModelLabel);
  const modeDisplayLabel = value.mode ? conversationModeLabel(value.mode) : (snapshot?.mode ? currentMode : "Mode auto");
  const modeOptions: { val: ConversationMode; label: string }[] = activeProvider === "codex"
    ? [
        { val: "plan", label: "Plan" },
        { val: "read-only", label: "Lecture seule" },
        { val: "execute", label: "Exécution" },
      ]
    : [
        { val: "plan", label: "Plan" },
        { val: "execute", label: "Exécution (demander)" },
        { val: "acceptEdits", label: "Accepter les modifications" },
        { val: "auto", label: "Auto (YOLO)" },
      ];
  if (!compact) {
    return <fieldset className="turn-settings" disabled={disabled}>
      <legend>Prochain message</legend>
      <label><span>Modèle</span><select aria-label="Modèle" title="Choisir le modèle du prochain message" value={modelSelectValue} onChange={event => onChange({ ...value, model: event.target.value === "__custom__" ? "" : event.target.value || undefined })}>
        <option value="">{snapshot?.model ? `Conserver : ${snapshot.model}` : "Configuration du projet / fournisseur"}</option>
        {models.map(model => <option value={model} key={model}>{model}</option>)}
        <option value="__custom__">Identifiant personnalisé…</option>
      </select></label>
      {value.model !== undefined && !models.includes(value.model) && <label htmlFor={id}>Identifiant du modèle<input id={id} required pattern={String.raw`[A-Za-z0-9][A-Za-z0-9._:@\/\-]*`} maxLength={256} value={value.model} placeholder="Identifiant accepté par votre CLI" onChange={event => onChange({ ...value, model: event.target.value })} /></label>}
      <label><span>Mode</span><select aria-label="Mode de conversation" title="Choisir le mode du prochain message" value={value.mode ?? ""} onChange={event => onChange({ ...value, mode: event.target.value ? event.target.value as ConversationMode : undefined })}>
        <option value="">{snapshot?.mode ? `Conserver : ${currentMode}` : "Configuration actuelle"}</option>
        {modeOptions.map(option => <option key={option.val} value={option.val}>{option.val === "execute" ? "Exécution" : option.label}</option>)}
      </select></label>
      <p className="muted turn-settings-hint">Choix appliqués à l’envoi. {value.mode === "execute" ? provider === "claude" ? "Claude demandera les autorisations nécessaires." : "Codex pourra modifier le dossier avec son sandbox et ses demandes d’autorisation." : value.mode === "auto" ? "Claude approuvera automatiquement les outils autorisés par son classificateur." : "Les permissions restent gérées par l’agent."}{unavailable && " Catalogue indisponible : vous pouvez saisir un identifiant ou conserver le modèle actuel."}</p>
    </fieldset>;
  }
  return <fieldset ref={rootRef} className="turn-settings compact" disabled={disabled}>
    <legend className="sr-only">Prochain message</legend>
    <div className="composer-chip-anchor composer-chip-anchor--model">
      <button
        type="button"
        className="composer-chip composer-chip-model"
        aria-expanded={openMenu === "model"}
        disabled={disabled}
        onClick={() => setOpenMenu(current => current === "model" ? null : "model")}
      >
        {(showProviderMark || engineChoices) && <span className={`composer-chip-icon composer-provider-icon composer-provider-icon-${activeProvider}`} aria-hidden="true"><ProviderMark provider={activeProvider} /></span>}
        <span className="composer-chip-primary">{modelDisplayLabel}</span>
        <span className="composer-chip-secondary">{effort}</span>
        <span className="composer-chip-chevron" aria-hidden="true"><Icon name="chevronDown" /></span>
      </button>
      <label className="sr-only">Modèle<select tabIndex={-1} aria-label="Modèle" title="Choisir le modèle du prochain message" value={modelSelectValue} onChange={event => onChange({ ...value, model: event.target.value === "__custom__" ? "" : event.target.value || undefined })}>
        <option value="">{snapshot?.model ? snapshot.model : "Modèle auto"}</option>
        {models.map(model => <option value={model} key={model}>{model}</option>)}
        <option value="__custom__">Identifiant personnalisé…</option>
      </select></label>
      {openMenu === "model" && (
        <div className="composer-popover composer-model-popover" role="dialog" aria-label="Modèle et effort">
          {engineChoices && engineChoices.length > 0 && onSelectEngine && (
            <div className="composer-popover-section">
              <div className="composer-popover-heading">Moteur</div>
              {engineChoices.map(choice => {
                const selected = choice.key === selectedEngineKey;
                return (
                  <button
                    type="button"
                    key={choice.key}
                    className={`composer-popover-option ${selected ? "is-selected" : ""}`}
                    onClick={() => onSelectEngine(choice.key)}
                  >
                    <span className={`composer-popover-option-icon composer-provider-icon-${choice.provider}`} aria-hidden="true">
                      <ProviderMark provider={choice.provider} />
                    </span>
                    <span className="composer-popover-option-label">{choice.label}</span>
                    {selected && <span className="composer-popover-option-check" aria-hidden="true"><Icon name="check" /></span>}
                  </button>
                );
              })}
            </div>
          )}
          <div className="composer-popover-section">
            {engineChoices && engineChoices.length > 0 && <div className="composer-popover-heading">Modèle</div>}
            <button
              type="button"
              className={`composer-popover-option ${modelSelectValue === "" ? "is-selected" : ""}`}
              onClick={() => { onChange({ ...value, model: undefined }); setOpenMenu(null); }}
            >
              <span className={`composer-popover-option-icon composer-provider-icon-${activeProvider}`} aria-hidden="true">
                <ProviderMark provider={activeProvider} />
              </span>
              <span className="composer-popover-option-label">{snapshot?.model || defaultModel || "Modèle auto"}</span>
              {modelSelectValue === "" && <span className="composer-popover-option-check" aria-hidden="true"><Icon name="check" /></span>}
            </button>
            {models.filter(model => model !== (snapshot?.model || defaultModel)).map(model => {
              const selected = modelSelectValue === model;
              return (
                <button
                  type="button"
                  key={model}
                  className={`composer-popover-option ${selected ? "is-selected" : ""}`}
                  onClick={() => { onChange({ ...value, model }); setOpenMenu(null); }}
                >
                  <span className={`composer-popover-option-icon composer-provider-icon-${activeProvider}`} aria-hidden="true">
                    <ProviderMark provider={activeProvider} />
                  </span>
                  <span className="composer-popover-option-label">{model}</span>
                  {selected && <span className="composer-popover-option-check" aria-hidden="true"><Icon name="check" /></span>}
                </button>
              );
            })}
            <button
              type="button"
              className={`composer-popover-option ${modelSelectValue === "__custom__" ? "is-selected" : ""}`}
              onClick={() => onChange({ ...value, model: modelSelectValue === "__custom__" ? value.model : "" })}
            >
              <span className="composer-popover-option-label">Identifiant personnalisé…</span>
              {modelSelectValue === "__custom__" && <span className="composer-popover-option-check" aria-hidden="true"><Icon name="check" /></span>}
            </button>
          </div>
          <div className="composer-popover-section composer-effort-section">
            <div className="composer-effort-header">
              <span>Effort</span>
              <strong>{effort}</strong>
            </div>
            <div className="composer-effort-Scale">
              <span>Faster</span>
              <span>Smarter</span>
            </div>
            <div className="composer-effort-track" role="radiogroup" aria-label="Effort">
              {EFFORT_LEVELS.map(level => {
                const active = effort === level;
                return (
                  <button
                    type="button"
                    key={level}
                    role="radio"
                    aria-checked={active}
                    aria-label={level}
                    className={`composer-effort-step ${active ? "is-active" : ""}`}
                    onClick={() => setEffort(level)}
                  >
                    <span className="composer-effort-dot" />
                  </button>
                );
              })}
            </div>
          </div>
        </div>
      )}
    </div>
    {value.model !== undefined && !models.includes(value.model) && <label htmlFor={id} className="turn-settings-custom">Identifiant du modèle<input id={id} required pattern={String.raw`[A-Za-z0-9][A-Za-z0-9._:@\/\-]*`} maxLength={256} value={value.model} placeholder="Identifiant accepté par votre CLI" onChange={event => onChange({ ...value, model: event.target.value })} /></label>}
    <div className="composer-chip-anchor composer-chip-anchor--mode">
      <button
        type="button"
        className={`composer-chip composer-chip-mode ${isAlertMode ? "composer-chip--alert" : "composer-chip--accent"}`}
        aria-expanded={openMenu === "mode"}
        disabled={disabled}
        onClick={() => setOpenMenu(current => current === "mode" ? null : "mode")}
      >
        <span className="composer-chip-icon" aria-hidden="true"><Icon name="shield" /></span>
        <span className="composer-chip-primary">{modeDisplayLabel}</span>
        <span className="composer-chip-chevron" aria-hidden="true"><Icon name="chevronDown" /></span>
      </button>
      <label className="sr-only">Mode<select tabIndex={-1} aria-label="Mode de conversation" title="Choisir le mode du prochain message" value={value.mode ?? ""} onChange={event => onChange({ ...value, mode: event.target.value ? event.target.value as ConversationMode : undefined })}>
        <option value="">{snapshot?.mode ? currentMode : "Mode auto"}</option>
        {modeOptions.map(option => <option key={option.val} value={option.val}>{option.val === "execute" ? "Exécution" : option.label}</option>)}
      </select></label>
      {openMenu === "mode" && (
        <div className="composer-popover composer-mode-popover" role="menu" aria-label="Mode de conversation">
          {[
            { val: "", label: snapshot?.mode ? `Conserver (${currentMode})` : "Mode du poste" },
            ...modeOptions,
          ].map(item => {
            const selected = (value.mode ?? "") === item.val;
            return (
              <button
                type="button"
                key={item.val || "default-mode"}
                role="menuitemradio"
                aria-checked={selected}
                className={`composer-popover-option ${selected ? "is-selected" : ""}`}
                onClick={() => {
                  onChange({ ...value, mode: item.val ? (item.val as ConversationMode) : undefined });
                  setOpenMenu(null);
                }}
              >
                <span className="composer-popover-option-label">{item.label}</span>
                {selected && <span className="composer-popover-option-check" aria-hidden="true"><Icon name="check" /></span>}
              </button>
            );
          })}
        </div>
      )}
    </div>
    <p className="muted turn-settings-hint" hidden={!unavailable}>Choix appliqués à l’envoi. {value.mode === "execute" ? provider === "claude" ? "Claude demandera les autorisations nécessaires." : "Codex pourra modifier le dossier avec son sandbox et ses demandes d’autorisation." : "Les permissions restent gérées par l’agent."}{unavailable && " Catalogue indisponible : vous pouvez saisir un identifiant ou conserver le modèle actuel."}</p>
  </fieldset>;
}
