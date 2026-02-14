package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kx0101/replayer-cloud/internal/auth"
	"github.com/kx0101/replayer-cloud/internal/config"
	"github.com/kx0101/replayer-cloud/internal/handler"
	"github.com/kx0101/replayer-cloud/internal/middleware"
	"github.com/kx0101/replayer-cloud/internal/store"
	"github.com/kx0101/replayer-cloud/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	if err = pool.Ping(ctx); err != nil {
		log.Fatalf("pinging database: %v", err)
	}

	pgStore := store.NewPostgresStore(pool)
	if err = pgStore.Migrate(ctx); err != nil {
		log.Fatalf("running migrations: %v", err)
	}

	sessionManager, err := auth.NewSessionManager(cfg.SessionSecret, cfg.SecureCookies)
	if err != nil {
		log.Fatalf("creating session manager: %v", err)
	}

	var emailSender *auth.EmailSender
	if cfg.SMTPHost != "" {
		emailSender = auth.NewEmailSender(
			cfg.SMTPHost,
			cfg.SMTPPort,
			cfg.SMTPUser,
			cfg.SMTPPassword,
			cfg.SMTPFrom,
			cfg.BaseURL,
		)
	}

	h, err := handler.New(handler.HandlerOptions{
		Store:          pgStore,
		TemplatesFS:    web.TemplatesFS,
		SessionManager: sessionManager,
		EmailSender:    emailSender,
	})
	if err != nil {
		log.Fatalf("creating handler: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logging)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.APIKeyAuth(pgStore))
		r.Post("/runs", h.CreateRun)
		r.Get("/runs", h.ListRuns)
		r.Get("/runs/{id}", h.GetRun)
		r.Post("/runs/{id}/baseline", h.SetBaseline)
		r.Get("/compare/{id}", h.CompareRun)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.OptionalSession(sessionManager))

		r.Get("/login", h.LoginPage)
		r.Post("/login", h.Login)
		r.Get("/register", h.RegisterPage)
		r.Post("/register", h.Register)
		r.Get("/verify", h.VerifyEmail)
		r.Get("/verify-pending", h.VerifyPendingPage)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireVerified(sessionManager, pgStore))

		r.Get("/", h.Dashboard)
		r.Post("/logout", h.Logout)
		r.Get("/runs/{id}", h.RunDetailPage)
		r.Get("/runs/{id}/compare", h.CompareViewPage)
		r.Get("/settings", h.SettingsPage)
		r.Post("/settings/api-keys", h.CreateAPIKey)
		r.Post("/settings/api-keys/{id}", func(w http.ResponseWriter, r *http.Request) {
			if r.FormValue("_method") == "DELETE" {
				h.DeleteAPIKey(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		})
		r.Delete("/settings/api-keys/{id}", h.DeleteAPIKey)

		r.Get("/htmx/runs", h.RunsListPartial)
		r.Post("/htmx/runs/{id}/baseline", h.SetBaselineHTMX)
	})

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("server listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-done
	log.Println("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown error: %v", err)
	}

	log.Println("server stopped")
}
