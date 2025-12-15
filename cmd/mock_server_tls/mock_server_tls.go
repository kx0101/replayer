package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		log.Printf("[TLS MOCK] %s %s Body=%q\n", r.Method, r.URL.Path, string(body))

		w.Header().Set("Content-Type", "application/json")
		_, err := fmt.Fprintf(w, `{"ok":true,"from":"tls-mock","path":"%s"}`, r.URL.Path)
		if err != nil {
			http.Error(w, "error", http.StatusBadRequest)
		}
	})

	certFile := "proxy.crt"
	keyFile := "proxy.key"

	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	server := &http.Server{
		Addr:              ":8443",
		Handler:           mux,
		TLSConfig:         tlsCfg,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	log.Println("TLS mock upstream listening on https://localhost:8443/")
	err := server.ListenAndServeTLS(certFile, keyFile)
	if err != nil {
		log.Fatal(err)
	}
}
