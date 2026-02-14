package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/kx0101/replayer-cloud/internal/auth"
	"github.com/kx0101/replayer-cloud/internal/models"
)

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	if err := h.templates.Render(w, "pages/login.html", nil); err != nil {
		log.Printf("error rendering login page: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderLoginError(w, "Invalid form data", "")
		return
	}

	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	password := r.FormValue("password")

	if email == "" || password == "" {
		h.renderLoginError(w, "Email and password are required", email)
		return
	}

	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		log.Printf("error getting user: %v", err)
		h.renderLoginError(w, "Invalid email or password", email)
		return
	}
	if user == nil {
		h.renderLoginError(w, "Invalid email or password", email)
		return
	}

	if !auth.CheckPassword(password, user.PasswordHash) {
		h.renderLoginError(w, "Invalid email or password", email)
		return
	}

	if err := h.sessionManager.CreateSession(w, user); err != nil {
		log.Printf("error creating session: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if user.VerifiedAt == nil {
		http.Redirect(w, r, "/verify-pending", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) renderLoginError(w http.ResponseWriter, errMsg, email string) {
	data := map[string]any{
		"Error": errMsg,
		"Email": email,
	}
	if err := h.templates.Render(w, "pages/login.html", data); err != nil {
		log.Printf("error rendering login page: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	if err := h.templates.Render(w, "pages/register.html", nil); err != nil {
		log.Printf("error rendering register page: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderRegisterError(w, "Invalid form data", "")
		return
	}

	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")

	if email == "" || password == "" {
		h.renderRegisterError(w, "Email and password are required", email)
		return
	}

	if len(password) < 8 {
		h.renderRegisterError(w, "Password must be at least 8 characters", email)
		return
	}

	if password != passwordConfirm {
		h.renderRegisterError(w, "Passwords do not match", email)
		return
	}

	existing, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		log.Printf("error checking existing user: %v", err)
		h.renderRegisterError(w, "An error occurred. Please try again.", email)
		return
	}
	if existing != nil {
		h.renderRegisterError(w, "An account with this email already exists", email)
		return
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		log.Printf("error hashing password: %v", err)
		h.renderRegisterError(w, "An error occurred. Please try again.", email)
		return
	}

	verifyToken, err := auth.GenerateVerifyToken()
	if err != nil {
		log.Printf("error generating verify token: %v", err)
		h.renderRegisterError(w, "An error occurred. Please try again.", email)
		return
	}

	user := &models.User{
		Email:        email,
		PasswordHash: passwordHash,
		VerifyToken:  &verifyToken,
	}

	if err := h.store.CreateUser(r.Context(), user); err != nil {
		log.Printf("error creating user: %v", err)
		h.renderRegisterError(w, "An error occurred. Please try again.", email)
		return
	}

	if h.emailSender != nil && h.emailSender.IsConfigured() {
		if err := h.emailSender.SendVerificationEmail(email, verifyToken); err != nil {
			log.Printf("error sending verification email: %v", err)
		}
	} else {
		log.Printf("verification token generated for new user (email sending disabled)")
	}

	if err := h.sessionManager.CreateSession(w, user); err != nil {
		log.Printf("error creating session: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/verify-pending", http.StatusSeeOther)
}

func (h *Handler) renderRegisterError(w http.ResponseWriter, errMsg, email string) {
	data := map[string]any{
		"Error": errMsg,
		"Email": email,
	}
	if err := h.templates.Render(w, "pages/register.html", data); err != nil {
		log.Printf("error rendering register page: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := h.store.GetUserByVerifyToken(r.Context(), token)
	if err != nil {
		log.Printf("error getting user by verify token: %v", err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := h.store.VerifyUser(r.Context(), user.ID); err != nil {
		log.Printf("error verifying user: %v", err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := h.sessionManager.CreateSession(w, user); err != nil {
		log.Printf("error creating session: %v", err)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) VerifyPendingPage(w http.ResponseWriter, r *http.Request) {
	if err := h.templates.Render(w, "pages/verify_pending.html", nil); err != nil {
		log.Printf("error rendering verify pending page: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.sessionManager.ClearSession(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
