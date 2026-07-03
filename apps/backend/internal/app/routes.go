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
}

type routeMux struct {
	mux           *http.ServeMux
	authenticated bool
	routes        *[]Route
	// optionalSession, when set, wraps every handler registered through
	// HandleFunc so a valid session cookie populates the viewer id in
	// context without requiring one. Only set it on a routeMux used for
	// routes that actually read the viewer id (e.g. to personalize
	// "liked"/"isFollowing"): health checks, login, and static file serving
	// don't need it, and protected already requires and injects a session
	// for every request via RequireSession, so wrapping it there too would
	// refresh the session twice per request.
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

// handleAuthenticated registers a handler on m's underlying mux that requires
// a session, regardless of m's own authenticated field. It exists for routes
// that must live on the public mux to win Go's ServeMux precedence rules
// (see its call sites) while still being recorded as Authenticated in the
// route contract.
func (m routeMux) handleAuthenticated(pattern string, handler http.HandlerFunc, requireSession func(http.Handler) http.Handler) {
	m.register(pattern, handler, requireSession, true)
}

func registerRoutes(public, protected routeMux, h handlers, requireSession, optionalSession func(http.Handler) http.Handler) {
	public.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	public.HandleFunc("GET /ready", readinessHandler(h.readiness))

	// personalized carries the same underlying mux and route list as public,
	// but additionally resolves a signed-in viewer's id for handlers that
	// read one (liked/isFollowing/email visibility). Routes that don't need
	// it (above, and sessions/uploads below) stay on plain public so they
	// don't pay for a session lookup they never use.
	personalized := public
	personalized.optionalSession = optionalSession

	users.RegisterPublicRoutes(personalized, h.users)
	// GET /users/me, GET /users/suggested, and GET /users/search must be
	// registered directly on the public mux (not only on protected's nested
	// "/" catch-all): Go's ServeMux prefers the most specific pattern IT
	// knows about, and public's own "GET /users/{username}" wildcard would
	// otherwise shadow these literal paths for every request, authenticated
	// or not.
	public.handleAuthenticated("GET /users/me", h.users.GetCurrentUser, requireSession)
	public.handleAuthenticated("GET /users/suggested", h.users.ListSuggestedUsers, requireSession)
	public.handleAuthenticated("GET /users/search", h.search.SearchUsers, requireSession)
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
		w.WriteHeader(http.StatusOK)
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
