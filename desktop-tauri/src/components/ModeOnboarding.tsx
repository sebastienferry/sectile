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
    setPendingMessage("Démarrage du compagnon local…");
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
    setPendingMessage("Jumelage avec le backend Fleet…");
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
    setPendingMessage("Association de l’agent Fleet existant…");
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
          <h1>Bienvenue dans Sectile Desktop</h1>
          <p className="onboarding-lead">
            Choisissez votre mode de fonctionnement pour commencer. Vous pourrez en changer à tout moment dans les réglages.
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
              <span className="onboarding-pill">100% Autonome</span>
            </div>
            <h2>Compagnon Local</h2>
            <p>
              Exécutez vos tâches et gérez vos tickets en toute autonomie sur votre poste.
              Un serveur local SQLite embarqué est démarré automatiquement, sans besoin d’infrastructure distante.
            </p>
            <div className="onboarding-card-action">
              <button
                type="button"
                className="primary onboarding-submit"
                disabled={pending}
                onClick={() => void startLocal()}
              >
                Démarrer en mode Compagnon
              </button>
            </div>
          </section>

          {/* Fleet Agent Card */}
          <section className="onboarding-option-card">
            <div className="onboarding-card-header">
              <div className="onboarding-icon-badge">
                <Icon name="link" />
              </div>
              <span className="onboarding-pill">Équipe & Centralisé</span>
            </div>
            <h2>Agent Fleet</h2>
            <p>
              Connectez ce poste à votre serveur Sectile d’équipe.
              Les projets et l’orchestration sont centralisés, tandis que l’exécution s’effectue localement sur votre machine.
            </p>
            <div className="onboarding-card-action">
              {!showFleetForm ? (
                <button
                  type="button"
                  className="secondary onboarding-submit"
                  disabled={pending}
                  onClick={() => setShowFleetForm(true)}
                >
                  Configurer le mode Fleet
                </button>
              ) : (
                <form className="onboarding-fleet-form" onSubmit={e => void handleFleetPair(e)}>
                  <fieldset disabled={pending}>
                    <label>
                      Adresse du serveur Fleet
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
                      Code de jumelage
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
                        Associer à Fleet
                      </button>
                      <button
                        type="button"
                        disabled={pending}
                        onClick={() => {
                          setShowFleetForm(false);
                          setError("");
                        }}
                      >
                        Annuler
                      </button>
                    </div>
                  </fieldset>
                  <details className="onboarding-existing-agent">
                    <summary>J’ai déjà un agent sur ce poste</summary>
                    <p className="muted">
                      Tauri peut reprendre cet agent sans modifier son installation d’origine.
                    </p>
                    <button
                      type="button"
                      disabled={pending}
                      onClick={() => void handleAttachFleet()}
                    >
                      Associer l’agent Fleet existant
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
