package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kx0101/replayer-cloud/internal/auth"
	"github.com/kx0101/replayer-cloud/internal/middleware"
	"github.com/kx0101/replayer-cloud/internal/models"
)

func (h *Handler) SettingsPage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	user := middleware.GetUser(r.Context())

	apiKeys, err := h.store.ListAPIKeysForUser(r.Context(), userID)
	if err != nil {
		log.Printf("error listing api keys: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"User":      user,
		"ActiveNav": "settings",
		"APIKeys":   apiKeys,
	}

	if err := h.templates.Render(w, "pages/settings.html", data); err != nil {
		log.Printf("error rendering settings page: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	user := middleware.GetUser(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = "Default"
	}

	fullKey, keyHash, keyPrefix, err := auth.GenerateAPIKey()
	if err != nil {
		log.Printf("error generating api key: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	apiKey := &models.APIKey{
		UserID:    userID,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		Name:      name,
	}

	if err := h.store.CreateAPIKey(r.Context(), apiKey); err != nil {
		log.Printf("error creating api key: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	apiKeys, err := h.store.ListAPIKeysForUser(r.Context(), userID)
	if err != nil {
		log.Printf("error listing api keys: %v", err)
	}

	data := map[string]any{
		"User":      user,
		"ActiveNav": "settings",
		"APIKeys":   apiKeys,
		"NewKey":    fullKey,
	}

	if err := h.templates.Render(w, "pages/settings.html", data); err != nil {
		log.Printf("error rendering settings page: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	idStr := chi.URLParam(r, "id")
	keyID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid key ID", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteAPIKey(r.Context(), userID, keyID); err != nil {
		log.Printf("error deleting api key: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
