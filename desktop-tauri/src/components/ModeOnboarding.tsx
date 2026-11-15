import { useState, type FormEvent } from "react";
import { Icon } from "./Icon";
import * as api from "../lib/local";

interface ModeOnboardingProps {
  onConfigured: () => void;
}

export function ModeOnboarding({ onConfigured }: ModeOnboardingProps) {
  const [pending, setPending] = useState(false);
  const [pendingMessage, setPendingMessage] = useState("");
  const [error, setError] = useState("");
  const [showFleetForm, setShowFleetForm] = useState(false);
  const [fleetUrl, setFleetUrl] = useState("");
  const [fleetCode, setFleetCode] = useState("");

  async function startLocal() {
    if (pending) return;
    setPending(true);
    setPendingMessage("Starting local companion…");
    setError("");
    try {
      await api.enableLocalRuntime();
      onConfigured();
    } catch (cause) {
      setError(String(cause));
    } finally {
      setPending(false);
      setPendingMessage("");
    }
  }

  async function handleFleetPair(e: FormEvent) {
    e.preventDefault();
    if (pending) return;
    setPending(true);
    setPendingMessage("Pairing with Fleet backend…");
    setError("");
    try {
      await api.pairFleetAgent(fleetUrl, fleetCode);
      setFleetCode("");
      onConfigured();
    } catch (cause) {
      setError(String(cause));
    } finally {
      setPending(false);
      setPendingMessage("");
    }
  }

  async function handleAttachFleet() {
    if (pending) return;
    setPending(true);
    setPendingMessage("Attaching existing Fleet agent…");
    setError("");
    try {
      await api.attachFleetAgent();
      onConfigured();
    } catch (cause) {
      setError(String(cause));
    } finally {
      setPending(false);
      setPendingMessage("");
    }
  }

  return (
    <div className="onboarding-view">
      <div className="onboarding-card-wrapper">
        <header className="onboarding-header">
          <span className="brand-mark">S</span>
          <h1>Welcome to Sectile Desktop</h1>
          <p className="onboarding-lead">
            Choose your operating mode to get started. You can switch modes at any time in settings.
          </p>
        </header>

        {error && (
          <div role="alert" className="error onboarding-error">
            {error}
          </div>
        )}

        {pending && (
          <div className="onboarding-pending" role="status">
            <span className="loading-spinner" aria-hidden="true" />
            <p>{pendingMessage}</p>
          </div>
        )}

        <div className="onboarding-grid">
          {/* Local Companion Card */}
          <section className="onboarding-option-card">
            <div className="onboarding-card-header">
              <div className="onboarding-icon-badge">
                <Icon name="monitor" />
              </div>
              <span className="onboarding-pill">100% Standalone</span>
            </div>
            <h2>Local Companion</h2>
            <p>
              Run tasks and manage tickets independently on your workstation.
              An embedded local SQLite server starts automatically, with no remote infrastructure needed.
            </p>
            <div className="onboarding-card-action">
              <button
                type="button"
                className="primary onboarding-submit"
                disabled={pending}
                onClick={() => void startLocal()}
              >
                Start in Companion mode
              </button>
            </div>
          </section>

          {/* Fleet Agent Card */}
          <section className="onboarding-option-card">
            <div className="onboarding-card-header">
              <div className="onboarding-icon-badge">
                <Icon name="link" />
              </div>
              <span className="onboarding-pill">Team & Centralized</span>
            </div>
            <h2>Fleet Agent</h2>
            <p>
              Connect this workstation to your team Sectile server.
              Projects and orchestration are centralized, while execution runs locally on your machine.
            </p>
            <div className="onboarding-card-action">
              {!showFleetForm ? (
                <button
                  type="button"
                  className="secondary onboarding-submit"
                  disabled={pending}
                  onClick={() => setShowFleetForm(true)}
                >
                  Configure Fleet mode
                </button>
              ) : (
                <form className="onboarding-fleet-form" onSubmit={e => void handleFleetPair(e)}>
                  <fieldset disabled={pending}>
                    <label>
                      Fleet server URL
                      <input
                        type="url"
                        required
                        placeholder="https://sectile.example.com"
                        value={fleetUrl}
                        onChange={e => setFleetUrl(e.target.value)}
                        autoComplete="url"
                      />
                    </label>
                    <label>
                      Pairing code
                      <input
                        type="password"
                        required
                        placeholder="••••••••"
                        autoComplete="off"
                        value={fleetCode}
                        onChange={e => setFleetCode(e.target.value)}
                      />
                    </label>
                    <div className="onboarding-form-actions">
                      <button className="primary" type="submit">
                        Pair with Fleet
                      </button>
                      <button
                        type="button"
                        disabled={pending}
                        onClick={() => {
                          setShowFleetForm(false);
                          setError("");
                        }}
                      >
                        Cancel
                      </button>
                    </div>
                  </fieldset>
                  <details className="onboarding-existing-agent">
                    <summary>I already have an agent on this workstation</summary>
                    <p className="muted">
                      Tauri can attach to this agent without modifying its original installation.
                    </p>
                    <button
                      type="button"
                      disabled={pending}
                      onClick={() => void handleAttachFleet()}
                    >
                      Attach existing Fleet agent
                    </button>
                  </details>
                </form>
              )}
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
