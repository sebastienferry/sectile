package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"tasks/internal/models"
)

// HandleSkillMarketplaces serves the deployment-wide registry of skill sources.
//
//	GET    /api/skill-marketplaces              → the registry
//	POST   /api/skill-marketplaces              → register one (admin)
//	DELETE /api/skill-marketplaces/{name}       → unregister one (admin)
//	GET    /api/skill-marketplaces/{name}/catalog → its plugins
//
// The registry is a deployment setting, so writing it is an administrator's;
// reading it is open to anyone signed in, because every project owner picks
// from it.
func (h *Handler) HandleSkillMarketplaces(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/skill-marketplaces"), "/")
	parts := []string{}
	if rawPath != "" {
		parts = strings.Split(rawPath, "/")
	}

	name := ""
	if len(parts) > 0 {
		if decoded, err := url.PathUnescape(parts[0]); err == nil {
			name = decoded
		} else {
			name = parts[0]
		}
	}

	switch {
	case r.Method == http.MethodGet && name == "":
		entries, err := h.db.ListSkillMarketplaces()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, entries)

	case r.Method == http.MethodPost && name == "":
		if _, ok := h.requireAdmin(w, r); !ok {
			return
		}
		var payload models.SkillMarketplace
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "corps de requête JSON invalide: "+err.Error())
			return
		}
		entry, err := h.db.AddSkillMarketplace(payload)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, entry)

	case r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "catalog":
		catalog, err := h.db.MarketplaceCatalog(name)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, catalog)

	case r.Method == http.MethodGet && len(parts) == 1:
		entry, err := h.db.SkillMarketplace(name)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, entry)

	case r.Method == http.MethodDelete && len(parts) == 1:
		if _, ok := h.requireAdmin(w, r); !ok {
			return
		}
		pinned, err := h.db.RemoveSkillMarketplace(name)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		// The projects that pinned it keep the bodies they applied; naming them
		// is what turns a silent orphaning into something someone can act on.
		writeJSON(w, http.StatusOK, map[string]any{"removed": name, "pinnedBy": pinned})

	default:
		writeError(w, http.StatusMethodNotAllowed, "Action non supportée sur skill-marketplaces")
	}
}

// handleProjectSkillPack serves the per-project half: what the project pinned,
// what a pack would change, and the one call that applies it.
//
//	GET    /api/projects/{id}/skill-pack          → the pin, or null
//	POST   /api/projects/{id}/skill-pack/preview  → read-only diff
//	POST   /api/projects/{id}/skill-pack          → apply
//	DELETE /api/projects/{id}/skill-pack          → unpin
//
// Applying and unpinning follow the skill editor's rule, since both only change
// which baseline the project's skill bodies start from: any signed-in account
// that is not blocked, which RequireSession already enforces on every route.
// Choosing the registry the packs come from stays an admin's.
func (h *Handler) handleProjectSkillPack(w http.ResponseWriter, r *http.Request, projectID string, parts []string) {
	sub := ""
	if len(parts) >= 3 {
		sub = parts[2]
	}

	var payload struct {
		Marketplace string `json:"marketplace"`
		Plugin      string `json:"plugin"`
		Commit      string `json:"commit"`
	}
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "corps de requête JSON invalide: "+err.Error())
			return
		}
		if strings.TrimSpace(payload.Marketplace) == "" || strings.TrimSpace(payload.Plugin) == "" {
			writeError(w, http.StatusBadRequest, "marketplace et plugin sont obligatoires")
			return
		}
	}

	switch {
	case r.Method == http.MethodGet && sub == "":
		pin, err := h.db.SkillPackPin(projectID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, pin)

	case r.Method == http.MethodPost && sub == "preview":
		preview, err := h.db.PreviewSkillPack(projectID, payload.Marketplace, payload.Plugin, payload.Commit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, preview)

	case r.Method == http.MethodPost && sub == "":
		pin, err := h.db.ApplySkillPack(projectID, payload.Marketplace, payload.Plugin, payload.Commit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, pin)

	case r.Method == http.MethodDelete && sub == "":
		if err := h.db.UnpinSkillPack(projectID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"unpinned": true})

	default:
		writeError(w, http.StatusMethodNotAllowed, "Action non supportée sur skill-pack")
	}
}
