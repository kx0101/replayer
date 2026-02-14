package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kx0101/replayer-cloud/internal/middleware"
)

func (h *Handler) SetBaseline(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run id")
		return
	}

	if err := h.store.SetBaselineForUser(r.Context(), userID, id); err != nil {
		if err.Error() == "run not found" {
			respondError(w, http.StatusNotFound, "run not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to set baseline")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
