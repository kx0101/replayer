package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type CheckoutRequest struct {
	UserID uint64   `json:"user_id"`
	Items  []uint64 `json:"items"`
}

type CheckoutResponse struct {
	OrderID   string    `json:"order_id"`
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Total     float64   `json:"total"`
	Tax       float64   `json:"tax"`
	CreatedAt time.Time `json:"created_at"`
	RequestID string    `json:"request_id"`
}

type StatusResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"request_id"`
}

type UserResponse struct {
	ID        uint64    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Version   string    `json:"version"`
	LastLogin time.Time `json:"last_login"`
	SessionID string    `json:"session_id"`
	RequestID string    `json:"request_id"`
}

func main() {
	port := flag.Int("port", 8081, "Port to run the mock server on")
	version := flag.String("version", "v2", "Server version identifier")
	flag.Parse()

	mux := http.NewServeMux()
	handlers := &Handlers{version: *version}

	mux.HandleFunc("/users/", handlers.getUserHandler)
	mux.HandleFunc("/checkout", handlers.checkoutHandler)
	mux.HandleFunc("/slow", handlers.slowHandler)
	mux.HandleFunc("/status", handlers.statusHandler)

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	fmt.Printf("Server v2 running on http://%s/\n", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

type Handlers struct {
	version string
}

func (h *Handlers) getUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/users/")
	id, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if id > 500 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	name := "Liakos koulaxis"
	if id%2 == 0 {
		name = "Liakos Koulaxis Jr."
	}

	response := UserResponse{
		ID:        id,
		Name:      name,
		Email:     fmt.Sprintf("user%d@example.com", id),
		Version:   h.version,
		LastLogin: time.Now(),
		SessionID: uuid.New().String(),
		RequestID: uuid.New().String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handlers) checkoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Items) > 10 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CheckoutResponse{
			Success:   false,
			Message:   "Too many items, maximum is 10",
			CreatedAt: time.Now(),
			RequestID: uuid.New().String(),
		})
		return
	}

	itemCount := len(req.Items)
	total := float64(itemCount) * 29.99
	tax := total * 0.10

	response := CheckoutResponse{
		OrderID:   fmt.Sprintf("ORD-%s", uuid.New().String()[:8]),
		Success:   true,
		Message:   fmt.Sprintf("Order confirmed for user %d | %d items | Tax calculated", req.UserID, itemCount), // DIFFERENT format
		Total:     total,
		Tax:       tax,
		CreatedAt: time.Now(),
		RequestID: uuid.New().String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handlers) statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := StatusResponse{
		Status:    "ok",
		Version:   h.version,
		Timestamp: time.Now(),
		RequestID: uuid.New().String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handlers) slowHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	time.Sleep(3 * time.Second)

	response := map[string]any{
		"status":     "completed",
		"message":    "done - v2 optimized",
		"duration":   3000,
		"timestamp":  time.Now(),
		"request_id": uuid.New().String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
