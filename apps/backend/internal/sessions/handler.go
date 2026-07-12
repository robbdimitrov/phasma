package sessions

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"phasma/backend/internal/auth"
	"phasma/backend/internal/httpx"
	"phasma/backend/internal/validation"
)

type HandlerService interface {
	Login(ctx context.Context, input LoginInput) (LoginOutput, error)
	Logout(ctx context.Context, sessionID string) error
	ListActive(ctx context.Context, userID, currentSessionToken string) ([]Session, error)
	Revoke(ctx context.Context, publicID, userID, currentSessionToken string) error
}

type Handler struct {
	Service HandlerService
}

func NewHandler(service HandlerService) Handler {
	return Handler{Service: service}
}

func (h Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}

	email := strings.ToLower(strings.TrimSpace(body.Email))
	if email == "" || body.Password == "" {
		httpx.WriteMessage(w, http.StatusBadRequest, "Email and password are required.")
		return
	}

	// No length policy check here: login must fail identically (401, generic
	// message) for any wrong credential so the response can't reveal why.
	output, err := h.Service.Login(r.Context(), LoginInput{
		Email:    email,
		Password: body.Password,
		ClientIP: httpx.ClientIP(r),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrLoginRateLimited):
			httpx.WriteMessage(w, http.StatusTooManyRequests, "Incorrect email or password.")
		case errors.Is(err, ErrInvalidCredentials):
			httpx.WriteMessage(w, http.StatusUnauthorized, "Incorrect email or password.")
		default:
			httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
		}
		return
	}

	httpx.SetSessionCookie(w, r, output.SessionID)
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"username": output.Username})
}

func (h Handler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := httpx.GetSessionCookie(r)
	if !ok || !auth.ValidSessionID(sessionID) {
		httpx.ClearSessionCookie(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.Service.Logout(r.Context(), sessionID); err != nil {
		httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	httpx.ClearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID, sessionID, ok := authenticatedSession(r)
	if !ok {
		httpx.WriteMessage(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	items, err := h.Service.ListActive(r.Context(), userID, sessionID)
	if err != nil {
		httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, struct {
		Sessions []Session `json:"sessions"`
	}{Sessions: items})
}

func (h Handler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	publicID := r.PathValue("sessionId")
	if !validation.ValidUUID(publicID) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Invalid session ID.")
		return
	}

	userID, sessionID, ok := authenticatedSession(r)
	if !ok {
		httpx.WriteMessage(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := h.Service.Revoke(r.Context(), publicID, userID, sessionID); err != nil {
		switch {
		case errors.Is(err, ErrSessionNotFound):
			httpx.WriteMessage(w, http.StatusNotFound, "Session not found.")
		case errors.Is(err, ErrCurrentSession):
			httpx.WriteMessage(w, http.StatusConflict, "The current session cannot be revoked.")
		default:
			httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func authenticatedSession(r *http.Request) (string, string, bool) {
	userID, ok := httpx.UserID(r)
	if !ok || userID == "" {
		return "", "", false
	}
	sessionID, ok := httpx.GetSessionCookie(r)
	if !ok || !auth.ValidSessionID(sessionID) {
		return "", "", false
	}
	return userID, sessionID, true
}
