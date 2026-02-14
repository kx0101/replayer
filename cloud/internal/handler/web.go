package handler

import (
	"context"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kx0101/replayer-cloud/internal/middleware"
	"github.com/kx0101/replayer-cloud/internal/models"
	"github.com/kx0101/replayer-cloud/internal/store"
)

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	user := middleware.GetUser(r.Context())

	filter := store.ListFilter{
		Environment: r.URL.Query().Get("environment"),
		Limit:       20,
	}

	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 1 {
			filter.Offset = (n - 1) * filter.Limit
		}
	}

	runs, total, err := h.store.ListRunsForUser(r.Context(), userID, filter)
	if err != nil {
		log.Printf("error listing runs: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	environments, err := h.getDistinctEnvironments(r.Context(), userID)
	if err != nil {
		log.Printf("error getting environments: %v", err)
	}

	page := 1
	if filter.Offset > 0 {
		page = (filter.Offset / filter.Limit) + 1
	}
	totalPages := (total + filter.Limit - 1) / filter.Limit

	data := map[string]any{
		"User":         user,
		"ActiveNav":    "dashboard",
		"Runs":         runs,
		"Total":        total,
		"Page":         page,
		"TotalPages":   totalPages,
		"Limit":        filter.Limit,
		"Offset":       filter.Offset,
		"Environment":  filter.Environment,
		"Environments": environments,
	}

	if err := h.templates.Render(w, "pages/dashboard.html", data); err != nil {
		log.Printf("error rendering dashboard: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) getDistinctEnvironments(ctx context.Context, userID uuid.UUID) ([]string, error) {
	filter := store.ListFilter{Limit: 100}
	runs, _, err := h.store.ListRunsForUser(ctx, userID, filter)
	if err != nil {
		return nil, err
	}

	envMap := make(map[string]struct{})
	for _, r := range runs {
		envMap[r.Environment] = struct{}{}
	}

	envs := make([]string, 0, len(envMap))
	for env := range envMap {
		envs = append(envs, env)
	}
	return envs, nil
}

func (h *Handler) RunDetailPage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	user := middleware.GetUser(r.Context())

	idStr := chi.URLParam(r, "id")
	runID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid run ID", http.StatusBadRequest)
		return
	}

	run, err := h.store.GetRunForUser(r.Context(), userID, runID)
	if err != nil {
		log.Printf("error getting run: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if run == nil {
		http.Error(w, "Run not found", http.StatusNotFound)
		return
	}

	data := map[string]any{
		"User":      user,
		"ActiveNav": "dashboard",
		"Run":       run,
	}

	if err := h.templates.Render(w, "pages/run_detail.html", data); err != nil {
		log.Printf("error rendering run detail: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) CompareViewPage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	user := middleware.GetUser(r.Context())

	idStr := chi.URLParam(r, "id")
	runID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid run ID", http.StatusBadRequest)
		return
	}

	run, err := h.store.GetRunForUser(r.Context(), userID, runID)
	if err != nil {
		log.Printf("error getting run: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if run == nil {
		http.Error(w, "Run not found", http.StatusNotFound)
		return
	}

	baseline, err := h.store.GetBaselineForUser(r.Context(), userID, run.Environment)
	if err != nil {
		log.Printf("error getting baseline: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"User":       user,
		"ActiveNav":  "dashboard",
		"Run":        run,
		"NoBaseline": baseline == nil,
	}

	if baseline != nil {
		comparison := buildComparison(run, baseline)
		data["Baseline"] = baseline
		data["Comparison"] = comparison
	}

	if err := h.templates.Render(w, "pages/compare.html", data); err != nil {
		log.Printf("error rendering compare page: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) RunsListPartial(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	filter := store.ListFilter{
		Environment: r.URL.Query().Get("environment"),
		Limit:       20,
	}

	runs, total, err := h.store.ListRunsForUser(r.Context(), userID, filter)
	if err != nil {
		log.Printf("error listing runs: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Runs":  runs,
		"Total": total,
	}

	if err := h.templates.RenderPartial(w, "partials/runs_list.html", data); err != nil {
		log.Printf("error rendering runs list partial: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) SetBaselineHTMX(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	idStr := chi.URLParam(r, "id")
	runID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid run ID", http.StatusBadRequest)
		return
	}

	if err = h.store.SetBaselineForUser(r.Context(), userID, runID); err != nil {
		log.Printf("error setting baseline: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	run, err := h.store.GetRunForUser(r.Context(), userID, runID)
	if err != nil {
		log.Printf("error getting run: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.renderRunRow(w, run)
}

func (h *Handler) renderRunRow(w http.ResponseWriter, run *models.Run) {
	successRate := float64(run.Succeeded) / float64(run.TotalRequests) * 100

	html := `<tr>
		<td class="whitespace-nowrap py-4 pl-4 pr-3 text-sm font-medium text-gray-900 sm:pl-6">` + run.Environment + `</td>
		<td class="whitespace-nowrap px-3 py-4 text-sm text-gray-500">` + run.CreatedAt.Format("Jan 02, 15:04") + `</td>
		<td class="whitespace-nowrap px-3 py-4 text-sm text-gray-500">` + strconv.Itoa(run.TotalRequests) + `</td>
		<td class="whitespace-nowrap px-3 py-4 text-sm">`

	if run.Failed == 0 {
		html += `<span class="inline-flex items-center rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-800">100%</span>`
	} else {
		html += `<span class="inline-flex items-center rounded-full bg-yellow-100 px-2.5 py-0.5 text-xs font-medium text-yellow-800">` + strconv.FormatFloat(successRate, 'f', 1, 64) + `%</span>`
	}

	html += `</td>
		<td class="whitespace-nowrap px-3 py-4 text-sm text-gray-500">` + strconv.FormatInt(run.LatencyStats.P95, 10) + `ms</td>
		<td class="whitespace-nowrap px-3 py-4 text-sm">
			<span class="inline-flex items-center rounded-full bg-indigo-100 px-2.5 py-0.5 text-xs font-medium text-indigo-800">Baseline</span>
		</td>
		<td class="relative whitespace-nowrap py-4 pl-3 pr-4 text-right text-sm font-medium sm:pr-6">
			<a href="/runs/` + run.ID.String() + `" class="text-indigo-600 hover:text-indigo-900">View</a>
		</td>
	</tr>`

	_, err := w.Write([]byte(html))
	if err != nil {
		log.Printf("error writing response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
