package app

import (
	"context"
	"net/http"

	"phasma/backend/internal/blobstore"
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

type Config struct {
	Blobs         blobstore.Store
	RateLimiter   httpx.RateLimiterStore
	LoginThrottle sessions.LoginThrottle
	Readiness     func(context.Context) error
	SearchClient  *search.SearchClient
}

type Repositories struct {
	SessionAuth   httpx.SessionStore
	Users         users.Repository
	Sessions      sessions.Repository
	Uploads       uploads.Repository
	Posts         posts.Repository
	Comments      comments.Repository
	Search        search.Repository
	Feed          feed.Repository
	Notifications notifications.Repository
}

func New(cfg Config, repositories Repositories) http.Handler {
	if cfg.Blobs == nil {
		cfg.Blobs = blobstore.NewMemoryStore()
	}
	if cfg.RateLimiter == nil {
		cfg.RateLimiter = httpx.NoopRateLimiterStore{}
	}
	if cfg.LoginThrottle == nil {
		cfg.LoginThrottle = sessions.NoopLoginThrottle{}
	}
	protected := http.NewServeMux()
	userHandler := users.NewHandler(users.NewService(repositories.Users, cfg.Blobs))
	sessionHandler := sessions.NewHandler(sessions.NewService(repositories.Sessions, cfg.LoginThrottle))
	uploadHandler := uploads.Handler{
		Service: uploads.NewService(repositories.Uploads), Store: cfg.Blobs,
	}
	postHandler := posts.Handler{Service: posts.NewService(
		repositories.Posts, uploads.Files{Store: cfg.Blobs},
	)}
	commentHandler := comments.Handler{Service: comments.NewService(repositories.Comments)}
	searchHandler := search.Handler{Service: search.NewService(repositories.Search), Client: cfg.SearchClient}
	feedHandler := feed.Handler{Service: feed.NewService(repositories.Feed)}
	notificationHandler := notifications.Handler{Service: notifications.NewService(repositories.Notifications)}

	public := http.NewServeMux()
	registerRoutes(
		routeMux{mux: public},
		routeMux{mux: protected, authenticated: true},
		handlers{
			users: userHandler, sessions: sessionHandler, uploads: uploadHandler,
			posts: postHandler, comments: commentHandler, search: searchHandler,
			feed: feedHandler, notifications: notificationHandler, readiness: cfg.Readiness,
		},
		httpx.RequireSession(repositories.SessionAuth),
		httpx.OptionalSession(repositories.SessionAuth),
	)
	protected.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteMessage(w, http.StatusNotFound, "Not Found")
	})

	public.Handle("/", httpx.RequireSession(repositories.SessionAuth)(protected))

	return httpx.Chain(
		public,
		httpx.RequestID,
		httpx.Logger,
		httpx.SecurityHeaders,
		httpx.OriginGuard,
		httpx.RateLimit(cfg.RateLimiter),
	)
}
