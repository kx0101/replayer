package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogging(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		status     int
		wantMethod string
		wantPath   string
	}{
		{
			name:       "logs GET request",
			method:     http.MethodGet,
			path:       "/api/v1/runs",
			status:     http.StatusOK,
			wantMethod: "GET",
			wantPath:   "/api/v1/runs",
		},
		{
			name:       "logs POST request with status",
			method:     http.MethodPost,
			path:       "/api/v1/runs",
			status:     http.StatusCreated,
			wantMethod: "POST",
			wantPath:   "/api/v1/runs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log.SetOutput(&buf)
			log.SetFlags(0)
			defer log.SetOutput(nil)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			})

			wrapped := Logging(handler)
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			logOutput := buf.String()
			if !strings.Contains(logOutput, tt.wantMethod) {
				t.Errorf("log output %q does not contain method %q", logOutput, tt.wantMethod)
			}
			if !strings.Contains(logOutput, tt.wantPath) {
				t.Errorf("log output %q does not contain path %q", logOutput, tt.wantPath)
			}
		})
	}
}
