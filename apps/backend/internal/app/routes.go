package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"phasma/backend/internal/comments"
	"phasma/backend/internal/feed"
	"phasma/backend/internal/httpx"
	"phasma/backend/internal/notifications"
	"phasma/backend/internal/pipeline"
	"phasma/backend/internal/posts"
	"phasma/backend/internal/search"
	"phasma/backend/internal/sessions"
	"phasma/backend/internal/uploads"
	"phasma/backend/internal/users"
)

type Route struct {
	Method        string
	Path          string
	Authenticated bool
}

type handlers struct {
	users         users.Handler
	sessions      sessions.Handler
	uploads       uploads.Handler
	posts         posts.Handler
	comments      comments.Handler
	search        search.Handler
	feed          feed.Handler
	notifications notifications.Handler
	readiness     func(context.Context) error
	pipelines     *pipeline.Monitor
}

type routeMux struct {
	mux           *http.ServeMux
	authenticated bool
	routes        *[]Route
	// optionalSession resolves a viewer id without requiring login. Use it
	// only for public routes that personalize responses.
	optionalSession func(http.Handler) http.Handler
}

// register wraps handler with wrap (if non-nil) and records it in m.routes
// (if tracked) with the given Authenticated flag.
func (m routeMux) register(pattern string, handler http.HandlerFunc, wrap func(http.Handler) http.Handler, authenticated bool) {
	var wrapped http.Handler = handler
	if wrap != nil {
		wrapped = wrap(wrapped)
	}
	m.mux.Handle(pattern, wrapped)
	if m.routes == nil {
		return
	}
	method, path, _ := strings.Cut(pattern, " ")
	*m.routes = append(*m.routes, Route{Method: method, Path: path, Authenticated: authenticated})
}

func (m routeMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.register(pattern, handler, m.optionalSession, m.authenticated)
}

// handleAuthenticated registers a session-required route on m's mux. It is
// used for public-mux literals that must outrank public wildcard routes.
func (m routeMux) handleAuthenticated(pattern string, handler http.HandlerFunc, requireSession func(http.Handler) http.Handler) {
	m.register(pattern, handler, requireSession, true)
}

func registerRoutes(public, protected routeMux, h handlers, requireSession, optionalSession func(http.Handler) http.Handler) {
	public.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	public.HandleFunc("GET /health/background", backgroundHealthHandler(h.pipelines))
	public.HandleFunc("GET /metrics", metricsHandler(h.pipelines))
	public.HandleFunc("GET /ready", readinessHandler(h.readiness))

	// personalized shares public's mux and route list, adding optional session
	// lookup only for routes that render viewer-specific fields.
	personalized := public
	personalized.optionalSession = optionalSession

	users.RegisterPublicRoutes(personalized, h.users)
	// These literals must live on public to outrank the public
	// "GET /users/{username}" wildcard before the protected mux is reached.
	public.handleAuthenticated("GET /users/me", h.users.GetCurrentUser, requireSession)
	public.handleAuthenticated("GET /users/suggested", h.users.ListSuggestedUsers, requireSession)
	public.handleAuthenticated("GET /users/search", h.search.SearchUsers, requireSession)
	public.handleAuthenticated("GET /users/{username}/followers", h.users.ListFollowers, requireSession)
	public.handleAuthenticated("GET /users/{username}/following", h.users.ListFollowing, requireSession)
	public.handleAuthenticated("GET /users/{username}/likes", h.posts.GetLikedPosts, requireSession)
	public.handleAuthenticated("GET /posts/popular", h.posts.ListPopularPosts, requireSession)
	sessions.RegisterPublicRoutes(public, h.sessions)
	uploads.RegisterPublicRoutes(public, h.uploads)
	posts.RegisterPublicRoutes(personalized, h.posts)
	comments.RegisterPublicRoutes(personalized, h.comments)

	users.RegisterProtectedRoutes(protected, h.users)
	sessions.RegisterProtectedRoutes(protected, h.sessions)
	uploads.RegisterProtectedRoutes(protected, h.uploads)
	posts.RegisterProtectedRoutes(protected, h.posts)
	comments.RegisterProtectedRoutes(protected, h.comments)
	search.RegisterRoutes(protected, h.search)
	feed.RegisterRoutes(protected, h.feed)
	notifications.RegisterRoutes(protected, h.notifications)
}

func readinessHandler(check func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if check != nil {
			if err := check(ctx); err != nil {
				httpx.WriteMessage(w, http.StatusServiceUnavailable, "Service unavailable")
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func backgroundHealthHandler(monitor *pipeline.Monitor) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"pipelines": monitor.Snapshot()})
	}
}

func metricsHandler(monitor *pipeline.Monitor) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		if monitor == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(monitor.Metrics("backend")))
	}
}

func Routes() []Route {
	public := http.NewServeMux()
	protected := http.NewServeMux()
	routes := []Route{}
	registerRoutes(
		routeMux{mux: public, routes: &routes},
		routeMux{mux: protected, authenticated: true, routes: &routes},
		handlers{},
		nil,
		nil,
	)
	return routes
}
