package proxy

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

type CaptureConfig struct {
	ListenAddr string
	Upstream   string
	OutputFile string
	Stream     bool
	TLSCert    string
	TLSKey     string
}

type CapturedEntry struct {
	Timestamp       time.Time   `json:"timestamp"`
	Method          string      `json:"method"`
	Path            string      `json:"path"`
	Headers         http.Header `json:"headers"`
	Body            string      `json:"body"`
	Status          int         `json:"status"`
	ResponseHeaders http.Header `json:"response_headers"`
	ResponseBody    string      `json:"response_body"`
	LatencyMs       int64       `json:"latency_ms"`
}

func StartReverseProxy(config *CaptureConfig) error {
	out, err := os.OpenFile(config.OutputFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err == nil {
		_ = os.Chmod(config.OutputFile, 0600)
	}

	writer := bufio.NewWriter(out)
	defer func() {
		err = writer.Flush()
		if err != nil {
			log.Printf("Error flushing writer: %v\n", err)
		}

		err = out.Close()
		if err != nil {
			log.Printf("Error closing output file: %v\n", err)
		}
	}()

	rawUp := strings.TrimSpace(config.Upstream)
	if rawUp == "" {
		return fmt.Errorf("upstream is empty")
	}

	upURL, err := url.Parse(rawUp)
	if err != nil {
		return fmt.Errorf("invalid upstream URL: %w", err)
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			if req.Body != nil {
				bodyBytes, _ := io.ReadAll(req.Body)
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

				req.Header.Set("X-Original-Body-Buffer", base64.StdEncoding.EncodeToString(bodyBytes))
			}

			req.URL.Scheme = upURL.Scheme
			req.URL.Host = upURL.Host
		},
		ModifyResponse: func(resp *http.Response) error {
			start := time.Now()

			var reqBody []byte
			if b64 := resp.Request.Header.Get("X-Original-Body-Buffer"); b64 != "" {
				reqBody, err = base64.StdEncoding.DecodeString(b64)
				if err != nil {
					return err
				}
			}

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}

			resp.Body = io.NopCloser(bytes.NewReader(respBody))

			entry := CapturedEntry{
				Timestamp:       start,
				Method:          resp.Request.Method,
				Path:            resp.Request.URL.Path,
				Headers:         resp.Request.Header,
				Body:            base64.StdEncoding.EncodeToString(reqBody),
				Status:          resp.StatusCode,
				ResponseHeaders: resp.Header,
				ResponseBody:    base64.StdEncoding.EncodeToString(respBody),
				LatencyMs:       time.Since(start).Milliseconds(),
			}

			data, _ := json.Marshal(entry)
			log.Println(string(data))

			_, err = writer.Write(data)
			if err != nil {
				return err
			}

			_, err = writer.Write([]byte("\n"))
			if err != nil {
				return err
			}

			err = writer.Flush()
			if err != nil {
				return err
			}

			return nil
		},
	}

	server := &http.Server{
		Addr:              config.ListenAddr,
		Handler:           proxy,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	log.Printf("Capture mode ON -- listening on %s --> %s\n", config.ListenAddr, upURL.Host) //#nosec G706 -- config values are from CLI flags, not user input

	if config.TLSCert != "" && config.TLSKey != "" {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		server.TLSConfig = tlsConfig
		return server.ListenAndServeTLS(config.TLSCert, config.TLSKey)
	}

	return server.ListenAndServe()
}
