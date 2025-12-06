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
)

type CheckoutRequest struct {
	UserID uint64   `json:"user_id"`
	Items  []uint64 `json:"items"`
}

type CheckoutResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type StatusResponse struct {
	Status string `json:"status"`
}

type UserResponse struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
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
	fmt.Printf("Server running on http://%s/\n", addr)

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
		ID:   id,
		Name: "Liakos koulaxis",
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

	response := CheckoutResponse{
		Success: true,
		Message: fmt.Sprintf("Checkout OK for user %d with %d items", req.UserID, len(req.Items)),
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
		Status: "ok",
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
	w.Write([]byte("done"))
}
