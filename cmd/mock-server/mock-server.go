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
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"request_id"`
}

type UserResponse struct {
	ID        uint64    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	LastLogin time.Time `json:"last_login"`
	SessionID string    `json:"session_id"`
	RequestID string    `json:"request_id"`
}

func main() {
	port := flag.Int("port", 8080, "Port to run the mock server on")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/users/", getUserHandler)
	mux.HandleFunc("/checkout", checkoutHandler)
	mux.HandleFunc("/slow", slowHandler)
	mux.HandleFunc("/status", statusHandler)

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	fmt.Printf("Server v1 running on http://%s/\n", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func getUserHandler(w http.ResponseWriter, r *http.Request) {
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

	response := UserResponse{
		ID:        id,
		Name:      "Liakos koulaxis",
		Email:     fmt.Sprintf("user%d@example.com", id),
		LastLogin: time.Now(),
		SessionID: uuid.New().String(),
		RequestID: uuid.New().String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func checkoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	itemCount := len(req.Items)
	total := float64(itemCount) * 29.99
	tax := total * 0.08

	response := CheckoutResponse{
		OrderID:   fmt.Sprintf("ORD-%s", uuid.New().String()[:8]),
		Success:   true,
		Message:   fmt.Sprintf("Checkout OK for user %d with %d items", req.UserID, itemCount),
		Total:     total,
		Tax:       tax,
		CreatedAt: time.Now(),
		RequestID: uuid.New().String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := StatusResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		RequestID: uuid.New().String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func slowHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	time.Sleep(2 * time.Second)

	response := map[string]any{
		"status":     "completed",
		"duration":   2000,
		"timestamp":  time.Now(),
		"request_id": uuid.New().String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
