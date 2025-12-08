package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(map[string]string{
			"message": "pong",
			"time":    time.Now().Format(time.RFC3339),
		})

		if err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		body, err := json.Marshal(map[string]any{
			"method": r.Method,
			"path":   r.URL.Path,
			"query":  r.URL.RawQuery,
		})
		if err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(body)
		if err != nil {
			http.Error(w, "Failed to write response", http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/random-delay", func(w http.ResponseWriter, r *http.Request) {
		delay := time.Duration(rand.Intn(400)+100) * time.Millisecond //#nosec G404
		time.Sleep(delay)

		w.Header().Set("X-Delay", delay.String())
		_, err := fmt.Fprintf(w, "Delayed %s", delay)
		if err != nil {
			http.Error(w, "Failed to write response", http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/big", func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 500*1024)
		for i := range buf {
			buf[i] = 'A'
		}
		_, err := w.Write(buf)
		if err != nil {
			http.Error(w, "Failed to write response", http.StatusInternalServerError)
		}
	})

	server := &http.Server{
		Addr:    ":8082",
		Handler: mux,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	log.Println("Mock server running on :8082")
	log.Fatal(server.ListenAndServe())
}
