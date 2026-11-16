import { useEffect, useRef, useState } from "react";
import { ProviderPreparation } from "./ProviderPreparation";
import { ProviderPermissions } from "./ProviderPermissions";
import { ClaudeSandboxSettings } from "./ClaudeSandboxSettings";
import { ProviderMark } from "./ProviderMark";
import { WorkstationProfiles } from "./ProjectProfiles";
import { providerReadiness, type Readiness } from "../lib/workspace";
import { invoke } from "@tauri-apps/api/core";

type ProviderStatus = {
  installed: boolean;
  path?: string;
  version?: string;
  models?: string[];
  authentication: "connected" | "disconnected" | "unknown";
};

type EngineTab = "overview" | "claude" | "codex" | "agy";

export function ProviderSettings({
  projects = [],
  showProfiles = false,
}: {
  projects?: { id: string; name: string }[];
  showProfiles?: boolean;
}) {
  const [tab, setTab] = useState<EngineTab>("overview");
  const [statuses, setStatuses] = useState<Record<string, ProviderStatus>>({});
  const [statusErrors, setStatusErrors] = useState<Record<string, string>>({});
  const [readiness, setReadiness] = useState<Record<string, Readiness>>({});
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState(false);
  const [watching, setWatching] = useState(false);
  const [refreshCount, setRefreshCount] = useState(0);
  const mounted = useRef(false);
  const checking = useRef(false);
  const launching = useRef(false);

  async function refreshReadiness(provider: "claude" | "codex") {
    try {
      const data = await providerReadiness(provider);
      if (mounted.current) {
        setReadiness((prev) => ({ ...prev, [provider]: data }));
      }
    } catch {
      // Readiness is optional in overview tests when services are mocked.
    }
  }

  async function refresh() {
    if (checking.current) return;
    checking.current = true;
    setPending(true);
    try {
      const providers = ["claude", "codex"] as const;
      const results = await Promise.allSettled(
        providers.map((provider) =>
          invoke<ProviderStatus>("provider_status", { provider }),
        ),
      );
      if (mounted.current) {
        const nextStatuses: Record<string, ProviderStatus> = {};
        const nextErrors: Record<string, string> = {};
        results.forEach((result, index) => {
          const provider = providers[index];
          if (result.status === "fulfilled")
            nextStatuses[provider] = result.value;
          else nextErrors[provider] = String(result.reason);
        });
        setStatuses(nextStatuses);
        setStatusErrors(nextErrors);
        setRefreshCount((count) => count + 1);
        setError("");
      }
    } catch (cause) {
      if (mounted.current) setError(String(cause));
    } finally {
      checking.current = false;
      if (mounted.current) setPending(false);
    }
  }

  useEffect(() => {
    mounted.current = true;
    void refresh();
    return () => {
      mounted.current = false;
    };
  }, []);

  useEffect(() => {
    if (tab === "claude" || tab === "codex") {
      void refreshReadiness(tab);
    }
  }, [tab]);

  useEffect(() => {
    if (!watching) return;
    const poll = setInterval(() => void refresh(), 5000);
    const deadline = setTimeout(() => {
      setWatching(false);
      setNotice(
        "Automatic check finished. Refresh after signing in if needed.",
      );
    }, 120000);
    return () => {
      clearInterval(poll);
      clearTimeout(deadline);
    };
  }, [watching]);

  async function login(provider: string) {
    if (launching.current) return;
    launching.current = true;
    setPending(true);
    setError("");
    try {
      await invoke("provider_login", { provider });
      if (mounted.current) {
        setWatching(true);
        setNotice(
          "Finish signing in using the opened terminal and browser. Status is checked automatically for two minutes.",
        );
      }
    } catch (cause) {
      if (mounted.current) setError(String(cause));
    } finally {
      launching.current = false;
      setPending(false);
    }
  }

  return (
    <article className="provider-settings" aria-label="AI engines">
      <div className="provider-tabs" role="tablist" aria-label="AI engine sections">
        {(
          [
            ["overview", "Overview", null],
            ["claude", "Claude", "claude"],
            ["codex", "Codex", "codex"],
            ["agy", "Antigravity", "agy"],
          ] as const
        ).map(([id, label, mark]) => (
          <button
            key={id}
            type="button"
            role="tab"
            aria-selected={tab === id}
            className={`provider-tab ${tab === id ? "active" : ""}`}
            onClick={() => setTab(id)}
          >
            {mark && (
              <span className={`provider-tab-mark provider-symbol-${mark}`}>
                <ProviderMark provider={mark} />
              </span>
            )}
            <span>{label}</span>
          </button>
        ))}
      </div>

      {tab === "overview" && (
        <>
          <div className="provider-settings-toolbar">
            <p>
              Engines run on this workstation. Their accounts are separate from
              trackers.
            </p>
            <button disabled={pending} onClick={() => void refresh()}>
              Check connections
            </button>
          </div>
          {error && (
            <p role="alert" className="error">
              {error}
            </p>
          )}
          {notice && <p role="status">{notice}</p>}
          <div className="provider-grid">
            {(
              [
                ["claude", "Claude Code"],
                ["codex", "Codex"],
              ] as const
            ).map(([provider, name]) => {
              const status = statuses[provider];
              return (
                <section className="provider-card" key={provider}>
                  <div className="provider-card-main">
                    <div
                      className={`provider-symbol provider-symbol-${provider}`}
                    >
                      <ProviderMark provider={provider} />
                    </div>
                    <div className="provider-card-copy">
                      <div className="provider-card-heading">
                        <h3>{name}</h3>
                        {status?.version && (
                          <span className="provider-version-badge">
                            {status.version}
                          </span>
                        )}
                      </div>
                      <p>
                        {!status
                          ? statusErrors[provider]
                            ? "CLI status unavailable"
                            : "Detecting…"
                          : !status.installed
                          ? "CLI not detected"
                          : status.authentication === "connected"
                          ? "CLI detected · connected"
                          : status.authentication === "disconnected"
                          ? "CLI detected · sign-in required"
                          : "CLI detected · connection status unknown"}
                      </p>
                    </div>
                    <button
                      className={
                        status?.authentication === "connected" ? "" : "primary"
                      }
                      disabled={pending || !status?.installed}
                      onClick={() => void login(provider)}
                    >
                      {status?.authentication === "connected"
                        ? `Reconnect ${name}`
                        : `Sign in to ${name}`}
                    </button>
                  </div>
                  {status?.models && status.models.length > 0 && (
                    <div
                      className="provider-models"
                      aria-label={`Detected ${name} models`}
                    >
                      <span className="provider-models-label">
                        Detected models
                      </span>
                      <div className="provider-models-list">
                        {status.models.map((model) => (
                          <code key={model} className="provider-model-pill">
                            {model}
                          </code>
                        ))}
                      </div>
                    </div>
                  )}
                  <div className="provider-card-extra">
                    {statusErrors[provider] && (
                      <p role="alert" className="error">
                        {name}: {statusErrors[provider]}
                      </p>
                    )}
                    {status?.path && (
                      <details className="provider-details">
                        <summary>Installation details</summary>
                        <code>{status.path}</code>
                      </details>
                    )}
                    {status && !status.installed && (
                      <p>
                        Install the official CLI, then run detection again.{" "}
                        <code>
                          {provider === "claude"
                            ? "https://code.claude.com/docs/en/setup"
                            : "https://developers.openai.com/codex/cli"}
                        </code>
                      </p>
                    )}
                    <ProviderPreparation
                      provider={provider}
                      projects={projects}
                    />
                  </div>
                </section>
              );
            })}
          </div>
          {showProfiles && (
            <WorkstationProfiles
              key={refreshCount}
              projectId={projects[0]?.id ?? ""}
            />
          )}
        </>
      )}

      {tab === "claude" && (
        <div className="provider-engine-panel">
          <ProviderPermissions
            provider="claude"
            view={readiness.claude}
            onSaved={() => refreshReadiness("claude")}
            inline
          />
          <ClaudeSandboxSettings projects={projects} />
        </div>
      )}

      {tab === "codex" && (
        <div className="provider-engine-panel">
          <ProviderPermissions
            provider="codex"
            view={readiness.codex}
            onSaved={() => refreshReadiness("codex")}
            inline
          />
          <ProviderPreparation provider="codex" projects={projects} hidePermissions />
        </div>
      )}

      {tab === "agy" && (
        <div className="provider-engine-panel">
          <ProviderPermissions
            provider="agy"
            onSaved={async () => {}}
            inline
          />
        </div>
      )}
    </article>
  );
}
