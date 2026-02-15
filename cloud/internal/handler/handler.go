package handler

import (
	"embed"

	"github.com/kx0101/replayer-cloud/internal/auth"
	"github.com/kx0101/replayer-cloud/internal/store"
)

type Handler struct {
	store          store.Store
	templates      *Templates
	sessionManager *auth.SessionManager
	emailSender    *auth.EmailSender
}

type HandlerOptions struct {
	Store          store.Store
	TemplatesFS    embed.FS
	SessionManager *auth.SessionManager
	EmailSender    *auth.EmailSender
}

func New(opts HandlerOptions) (*Handler, error) {
	templates, err := NewTemplates(opts.TemplatesFS)
	if err != nil {
		return nil, err
	}

	return &Handler{
		store:          opts.Store,
		templates:      templates,
		sessionManager: opts.SessionManager,
		emailSender:    opts.EmailSender,
	}, nil
}
