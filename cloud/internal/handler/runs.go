package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kx0101/replayer-cloud/internal/middleware"
	"github.com/kx0101/replayer-cloud/internal/models"
	"github.com/kx0101/replayer-cloud/internal/store"
)

type createRunRequest struct {
	Environment string                  `json:"environment"`
	Targets     []string                `json:"targets"`
	Summary     models.Summary          `json:"summary"`
	Results     []models.MultiEnvResult `json:"results"`
	Labels      map[string]string       `json:"labels,omitempty"`
}

func (h *Handler) CreateRun(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req createRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Environment == "" {
		respondError(w, http.StatusBadRequest, "environment is required")
		return
	}
	if len(req.Targets) == 0 {
		respondError(w, http.StatusBadRequest, "targets is required")
		return
	}

	run := &models.Run{
		Environment:   req.Environment,
		Targets:       req.Targets,
		TotalRequests: req.Summary.TotalRequests,
		Succeeded:     req.Summary.Succeeded,
		Failed:        req.Summary.Failed,
		LatencyStats:  req.Summary.Latency,
		ByTarget:      req.Summary.ByTarget,
		Results:       req.Results,
		Labels:        req.Labels,
	}

	if run.ByTarget == nil {
		run.ByTarget = make(map[string]models.TargetStats)
	}
	if run.Results == nil {
		run.Results = make([]models.MultiEnvResult, 0)
	}
	if run.Labels == nil {
		run.Labels = make(map[string]string)
	}

	if err := h.store.CreateRunForUser(r.Context(), userID, run); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create run")
		return
	}

	respondJSON(w, http.StatusCreated, run)
}

func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run id")
		return
	}

	run, err := h.store.GetRunForUser(r.Context(), userID, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get run")
		return
	}
	if run == nil {
		respondError(w, http.StatusNotFound, "run not found")
		return
	}

	respondJSON(w, http.StatusOK, run)
}

func (h *Handler) ListRuns(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	filter := store.ListFilter{
		Environment: r.URL.Query().Get("environment"),
	}

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Offset = n
		}
	}
	if v := r.URL.Query().Get("after"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.After = &t
		}
	}
	if v := r.URL.Query().Get("before"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Before = &t
		}
	}

	items, total, err := h.store.ListRunsForUser(r.Context(), userID, filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list runs")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
	})
}
