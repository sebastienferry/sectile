import { useEffect, useRef, useState } from "react";
import { ProviderPreparation } from "./ProviderPreparation";
import { Icon } from "./Icon";
import { invoke } from "@tauri-apps/api/core";
type ProviderStatus = { installed: boolean; path?: string; authentication: "connected" | "disconnected" | "unknown" };
export function ProviderSettings({projects=[]}:{projects?:{id:string;name:string}[]}) {
  const [statuses, setStatuses] = useState<Record<string, ProviderStatus>>({});
  const [statusErrors, setStatusErrors] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState(false);
  const [watching, setWatching] = useState(false);
  const mounted = useRef(false);
  const checking = useRef(false);
  const launching = useRef(false);
  async function refresh() {
    if (checking.current) return;
    checking.current = true; setPending(true);
    try {
      const providers = ["claude", "codex"] as const;
      const results = await Promise.allSettled(providers.map(provider => invoke<ProviderStatus>("provider_status", { provider })));
      if (mounted.current) {
        const nextStatuses: Record<string, ProviderStatus> = {};
        const nextErrors: Record<string, string> = {};
        results.forEach((result, index) => {
          const provider = providers[index];
          if (result.status === "fulfilled") nextStatuses[provider] = result.value;
          else nextErrors[provider] = String(result.reason);
        });
        setStatuses(nextStatuses);
        setStatusErrors(nextErrors);
        setError("");
      }
    } catch (cause) { if (mounted.current) setError(String(cause)); }
    finally { checking.current = false; if (mounted.current) setPending(false); }
  }
  useEffect(() => { mounted.current = true; void refresh(); return () => { mounted.current = false; }; }, []);
  useEffect(() => {
    if (!watching) return;
    const poll = setInterval(() => void refresh(), 5000);
    const deadline = setTimeout(() => { setWatching(false); setNotice("Vérification automatique terminée. Actualisez après la connexion si nécessaire."); }, 120000);
    return () => { clearInterval(poll); clearTimeout(deadline); };
  }, [watching]);
  async function login(provider: string) {
    if (launching.current) return;
    launching.current = true; setPending(true); setError("");
    try {
      await invoke("provider_login", { provider });
      if (mounted.current) { setWatching(true); setNotice("Terminez la connexion dans le terminal et le navigateur ouverts. L’état est vérifié automatiquement pendant deux minutes."); }
    } catch (cause) { if (mounted.current) setError(String(cause)); }
    finally { launching.current = false; if (mounted.current) setPending(false); }
  }
  return <article className="provider-settings" aria-label="Moteurs IA">
    <div className="settings-card-heading"><h2>Vos assistants de développement</h2><span className="badge">Exécution locale</span></div>
    <p>Claude Code et Codex utilisent leur connexion officielle sur ce poste. Ces comptes sont distincts de vos trackers.</p>
    <button disabled={pending} onClick={() => void refresh()}>Détecter / vérifier les connexions</button>
    {error && <p role="alert" className="error">{error}</p>}
    {notice && <p role="status">{notice}</p>}
    <div className="provider-grid">{([ ["claude", "Claude Code"], ["codex", "Codex"] ] as const).map(([provider, name]) => {
      const status = statuses[provider];
      return <section className="provider-card" key={provider}><div className="provider-symbol"><Icon name={provider === "claude" ? "agent" : "terminal"} /></div><h3>{name}</h3>
        <p>{!status ? statusErrors[provider] ? "État de la CLI indisponible" : "Détection en cours…" : !status.installed ? "CLI non détectée" : status.authentication === "connected" ? "CLI détectée · connecté" : status.authentication === "disconnected" ? "CLI détectée · connexion requise" : "CLI détectée · état de connexion indéterminé"}</p>
        {statusErrors[provider] && <p role="alert" className="error">{name} : {statusErrors[provider]}</p>}
        {status?.path && <details className="provider-details"><summary>Détails de l’installation</summary><code>{status.path}</code></details>}
        {status && !status.installed && <p>Installez la CLI officielle, puis relancez la détection. <code>{provider === "claude" ? "https://code.claude.com/docs/en/setup" : "https://developers.openai.com/codex/cli"}</code></p>}
        <button className={status?.authentication === "connected" ? "" : "primary"} disabled={pending || !status?.installed} onClick={() => void login(provider)}>{status?.authentication === "connected" ? `Reconnecter ${name}` : `Se connecter à ${name}`}</button>
        <ProviderPreparation provider={provider} projects={projects}/>
      </section>;
    })}</div>
  </article>;
}
