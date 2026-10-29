package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
)

const DesktopOAuthPath = "/api/desktop-oauth"
const DesktopOAuthStartPath = "/auth/desktop-oauth/start"

// The desktop exchanges its authenticated request for a one-use browser ticket.
// A separate cookie binds the callback without overwriting web sign-in cookies.
// Provider code exchange, state consumption and encrypted grants stay in DB.
type desktopOAuthFlow struct {
	ID, UserID, Provider, Ticket, Session, State, Status string
	Expires                                              time.Time
}
type desktopOAuthBroker struct {
	sync.Mutex
	flows map[string]*desktopOAuthFlow
}

func desktopProvider(provider string) bool {
	return provider == "github" || provider == "gitlab" || provider == "jira"
}
func (h *Handler) expireDesktopOAuthLocked() {
	for id, flow := range h.desktopOAuth.flows {
		if time.Now().After(flow.Expires) && flow.Status != "completing" {
			if flow.Session != "" {
				_ = h.db.RevokeWebSession(flow.Session)
			}
			delete(h.desktopOAuth.flows, id)
		}
	}
}
func (h *Handler) HandleDesktopOAuth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	caller := h.webPrincipal(r)
	if caller.Anonymous() {
		writeError(w, 401, "Connectez votre profil local avant de continuer.")
		return
	}
	h.desktopOAuth.Lock()
	defer h.desktopOAuth.Unlock()
	h.expireDesktopOAuthLocked()
	if h.desktopOAuth.flows == nil {
		h.desktopOAuth.flows = map[string]*desktopOAuthFlow{}
	}
	switch r.Method {
	case http.MethodPost:
		var input struct {
			Provider string `json:"provider"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&input) != nil || !desktopProvider(input.Provider) {
			writeError(w, 400, "Fournisseur inconnu")
			return
		}
		if !h.db.OAuthAppConfigured(input.Provider) {
			writeError(w, 409, "Configurez d’abord l’application OAuth de ce fournisseur.")
			return
		}
		for _, flow := range h.desktopOAuth.flows {
			if flow.UserID == caller.UserID && flow.Provider == input.Provider && (flow.Status == "waiting" || flow.Status == "completing") {
				writeError(w, 409, "Une connexion est déjà en cours pour ce fournisseur.")
				return
			}
		}
		if len(h.desktopOAuth.flows) >= 256 {
			writeError(w, 429, "Trop de connexions en attente")
			return
		}
		flow := &desktopOAuthFlow{ID: uuid.NewString(), UserID: caller.UserID, Provider: input.Provider, Ticket: uuid.NewString(), Status: "waiting", Expires: time.Now().Add(10 * time.Minute)}
		h.desktopOAuth.flows[flow.ID] = flow
		writeJSON(w, 201, map[string]any{"id": flow.ID, "startPath": DesktopOAuthStartPath + "?ticket=" + flow.Ticket, "expiresAt": flow.Expires})
	case http.MethodGet, http.MethodDelete:
		flow := h.desktopOAuth.flows[r.URL.Query().Get("id")]
		if flow == nil || flow.UserID != caller.UserID {
			writeJSON(w, 200, map[string]string{"status": "expired"})
			return
		}
		if r.Method == http.MethodDelete {
			if flow.Status == "completing" {
				writeError(w, 409, "La connexion est en cours de validation.")
				return
			}
			if flow.Status == "waiting" {
				flow.Status = "cancelled"
				flow.Ticket = ""
				if flow.Session != "" {
					_ = h.db.RevokeWebSession(flow.Session)
					flow.Session = ""
				}
			}
		}
		writeJSON(w, 200, map[string]string{"id": flow.ID, "provider": flow.Provider, "status": flow.Status})
	default:
		writeError(w, 405, "Méthode non autorisée")
	}
}
func (h *Handler) HandleDesktopOAuthStart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Method != http.MethodGet {
		http.Error(w, "Méthode non autorisée", 405)
		return
	}
	h.desktopOAuth.Lock()
	defer h.desktopOAuth.Unlock()
	h.expireDesktopOAuthLocked()
	var flow *desktopOAuthFlow
	ticket := r.URL.Query().Get("ticket")
	if ticket != "" {
		for _, candidate := range h.desktopOAuth.flows {
			if candidate.Ticket == ticket && candidate.Status == "waiting" {
				flow = candidate
				break
			}
		}
	}
	if flow == nil {
		http.Error(w, "Cette demande a expiré ou a déjà été utilisée. Revenez au desktop.", 410)
		return
	}
	flow.Ticket = ""
	session, _, err := h.db.CreateWebSession(flow.UserID)
	if err != nil {
		flow.Status = "failed"
		http.Error(w, "Connexion indisponible", 500)
		return
	}
	flow.Session = session
	var authorize string
	if flow.Provider == "jira" {
		authorize, err = h.db.JiraOAuthAuthorizeURL(flow.UserID, session)
	} else {
		authorize, err = h.db.ForgeOAuthAuthorizeURL(flow.Provider, flow.UserID, session)
	}
	if err != nil {
		flow.Status = "failed"
		_ = h.db.RevokeWebSession(session)
		flow.Session = ""
		http.Error(w, "Vérifiez l’application OAuth et les sources de tickets configurées dans le desktop.", 409)
		return
	}
	parsed, _ := url.Parse(authorize)
	flow.State = parsed.Query().Get("state")
	http.SetCookie(w, &http.Cookie{Name: "sectile_desktop_" + flow.Provider, Value: session, Path: "/auth/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil, MaxAge: 600})
	http.Redirect(w, r, authorize, http.StatusFound)
}

// finishDesktopOAuth returns false for ordinary web callbacks. A desktop cookie
// only enters its own flow; DB still checks state, user, session and provider.
func (h *Handler) finishDesktopOAuth(provider string, w http.ResponseWriter, r *http.Request) bool {
	cookie, err := r.Cookie("sectile_desktop_" + provider)
	if err != nil {
		return false
	}
	h.desktopOAuth.Lock()
	h.expireDesktopOAuthLocked()
	var flow *desktopOAuthFlow
	for _, candidate := range h.desktopOAuth.flows {
		if candidate.Provider == provider && candidate.Session == cookie.Value && candidate.State == r.URL.Query().Get("state") && candidate.Status == "waiting" {
			flow = candidate
			break
		}
	}
	if flow == nil {
		h.desktopOAuth.Unlock()
		// A web flow can coexist with a stale desktop cookie.
		if _, err := r.Cookie(sessionCookie); err == nil {
			return false
		}
		http.Error(w, "Connexion expirée ou invalide. Revenez au desktop.", http.StatusGone)
		return true
	}
	flow.Status = "completing"
	userID, session, id := flow.UserID, flow.Session, flow.ID
	h.desktopOAuth.Unlock()
	query := r.URL.Query()
	outcome := "invalid"
	if h.db.UserForWebSession(session) == userID {
		if provider == "jira" {
			outcome, err = h.db.CompleteJiraOAuth(r.Context(), userID, session, query.Get("state"), query.Get("code"), query.Get("error") == "access_denied")
		} else {
			outcome, err = h.db.CompleteForgeOAuth(r.Context(), provider, userID, session, query.Get("state"), query.Get("code"), query.Get("error") == "access_denied")
		}
	}
	if err != nil && provider == "jira" {
		// CompleteJiraOAuth errors exclude codes, tokens and client secrets.
		log.Printf("[DesktopOAuth] validation Jira : statut=%s, erreur=%v", outcome, err)
	}
	// Outcome values describe provider failures without exposing codes or tokens.
	if err != nil && outcome == "connected" {
		outcome = "failed"
	}
	h.desktopOAuth.Lock()
	if stored := h.desktopOAuth.flows[id]; stored != nil {
		stored.Status = outcome
		stored.Session = ""
	}
	h.desktopOAuth.Unlock()
	_ = h.db.RevokeWebSession(session)
	http.SetCookie(w, &http.Cookie{Name: cookie.Name, Path: "/auth/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	message := "La connexion n’a pas abouti. Consultez le résultat dans Sectile Desktop."
	if outcome == "connected" {
		message = "Connexion réussie. Vous pouvez fermer cette page et revenir à Sectile Desktop."
	}
	_, _ = w.Write([]byte("<!doctype html><html lang=\"fr\"><meta charset=\"utf-8\"><title>Sectile Desktop</title><h1>Sectile Desktop</h1><p>" + message + "</p></html>"))
	return true
}
