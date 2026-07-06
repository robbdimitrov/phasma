package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"phasma/backend/internal/auth"
)

type SessionStore interface {
	RefreshSession(ctx context.Context, sessionID string) (Session, error)
}

const sessionCookieName = "session"

func GetSessionCookie(r *http.Request) (string, bool) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		return cookie.Value, true
	}
	return "", false
}

func secureRequest(r *http.Request) bool {
	return r.TLS != nil || (trustProxy && r.Header.Get("X-Forwarded-Proto") == "https")
}

type Session struct {
	ID     string
	UserID string
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func Chain(handler http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 64 {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, WithRequestID(r, id))
	})
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		slog.Info("http request",
			"request_id", GetRequestID(r),
			"method", r.Method,
			"route", r.Pattern,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 0 disables the legacy XSS auditor; it has its own injection vectors
		// and is superseded by the Content-Security-Policy below.
		w.Header().Set("X-XSS-Protection", "0")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; font-src 'self' data:; connect-src 'self'")
		if secureRequest(r) {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func OriginGuard(next http.Handler) http.Handler {
	unsafeMethods := map[string]bool{
		http.MethodPost:   true,
		http.MethodPut:    true,
		http.MethodPatch:  true,
		http.MethodDelete: true,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !unsafeMethods[r.Method] {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			WriteMessage(w, http.StatusForbidden, "Forbidden")
			return
		}

		if parsed.Scheme+"://"+parsed.Host != expectedOrigin(r) {
			WriteMessage(w, http.StatusForbidden, "Forbidden")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func RequireSession(store SessionStore) func(http.Handler) http.Handler {
	allowed := map[string]bool{
		http.MethodPost + " /sessions": true,
		http.MethodPost + " /users":    true,
		http.MethodGet + " /health":    true,
		http.MethodOptions + " /":      true,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions || allowed[r.Method+" "+r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			sessionID, ok := GetSessionCookie(r)
			if !ok {
				WriteMessage(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			if !auth.ValidSessionID(sessionID) {
				ClearSessionCookie(w, r)
				WriteMessage(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			session, err := store.RefreshSession(r.Context(), sessionID)
			if err != nil {
				WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
				return
			}
			if session.UserID == "" {
				ClearSessionCookie(w, r)
				WriteMessage(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			SetSessionCookie(w, r, sessionID)
			next.ServeHTTP(w, WithUserID(r, session.UserID))
		})
	}
}

// OptionalSession populates viewer id for a valid cookie and otherwise
// continues anonymously. Use it for public routes with personalized fields.
func OptionalSession(store SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID, ok := GetSessionCookie(r)
			if !ok || !auth.ValidSessionID(sessionID) {
				next.ServeHTTP(w, r)
				return
			}

			session, err := store.RefreshSession(r.Context(), sessionID)
			if err != nil || session.UserID == "" {
				next.ServeHTTP(w, r)
				return
			}

			SetSessionCookie(w, r, sessionID)
			next.ServeHTTP(w, WithUserID(r, session.UserID))
		})
	}
}

func expectedOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if trustProxy {
		if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
			scheme = strings.Split(forwardedProto, ",")[0]
		}
	}
	return scheme + "://" + r.Host
}

func SetSessionCookie(w http.ResponseWriter, r *http.Request, sessionID string) {
	secure := secureRequest(r)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
	})
}

func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	secure := secureRequest(r)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
	})
}
