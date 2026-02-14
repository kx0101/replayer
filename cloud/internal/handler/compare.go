package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kx0101/replayer-cloud/internal/middleware"
	"github.com/kx0101/replayer-cloud/internal/models"
)

func (h *Handler) CompareRun(w http.ResponseWriter, r *http.Request) {
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

	baseline, err := h.store.GetBaselineForUser(r.Context(), userID, run.Environment)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get baseline")
		return
	}
	if baseline == nil {
		respondError(w, http.StatusNotFound, "no baseline set for environment")
		return
	}

	result := buildComparison(run, baseline)
	respondJSON(w, http.StatusOK, result)
}

func buildComparison(run, baseline *models.Run) models.ComparisonResult {
	result := models.ComparisonResult{
		RunID:      run.ID,
		BaselineID: baseline.ID,
		RunSummary: models.Summary{
			TotalRequests: run.TotalRequests,
			Succeeded:     run.Succeeded,
			Failed:        run.Failed,
			Latency:       run.LatencyStats,
			ByTarget:      run.ByTarget,
		},
		BaseSummary: models.Summary{
			TotalRequests: baseline.TotalRequests,
			Succeeded:     baseline.Succeeded,
			Failed:        baseline.Failed,
			Latency:       baseline.LatencyStats,
			ByTarget:      baseline.ByTarget,
		},
		LatencyDelta: make(map[string]models.LatencyDelta),
	}

	baselineByReqID := make(map[string]*models.MultiEnvResult)
	for i := range baseline.Results {
		baselineByReqID[baseline.Results[i].RequestID] = &baseline.Results[i]
	}
	for _, r := range run.Results {
		if r.Diff != nil && (r.Diff.StatusMismatch || r.Diff.BodyMismatch) {
			result.DiffCount++
			continue
		}
		if br, ok := baselineByReqID[r.RequestID]; ok {
			if hasDifferences(r, *br) {
				result.DiffCount++
			}
		}
	}

	for target, runStats := range run.ByTarget {
		if baseStats, ok := baseline.ByTarget[target]; ok {
			result.LatencyDelta[target] = models.LatencyDelta{
				Current:   runStats.Latency,
				Baseline:  baseStats.Latency,
				P50Change: pctChange(baseStats.Latency.P50, runStats.Latency.P50),
				P90Change: pctChange(baseStats.Latency.P90, runStats.Latency.P90),
				P95Change: pctChange(baseStats.Latency.P95, runStats.Latency.P95),
				P99Change: pctChange(baseStats.Latency.P99, runStats.Latency.P99),
				AvgChange: pctChange(baseStats.Latency.Avg, runStats.Latency.Avg),
			}
		}
	}

	return result
}

func hasDifferences(a, b models.MultiEnvResult) bool {
	for target, aResp := range a.Responses {
		if bResp, ok := b.Responses[target]; ok {
			if aResp.Status != nil && bResp.Status != nil && *aResp.Status != *bResp.Status {
				return true
			}
		}
	}
	return false
}

func pctChange(base, current int64) float64 {
	if base == 0 {
		return 0
	}
	return float64(current-base) / float64(base) * 100
}
