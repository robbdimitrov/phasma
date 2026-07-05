package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"phasma/backend/internal/app"
	"phasma/backend/internal/blobstore"
	"phasma/backend/internal/comments"
	"phasma/backend/internal/env"
	"phasma/backend/internal/feed"
	"phasma/backend/internal/httpx"
	"phasma/backend/internal/notifications"
	"phasma/backend/internal/pipeline"
	"phasma/backend/internal/posts"
	"phasma/backend/internal/search"
	"phasma/backend/internal/sessions"
	"phasma/backend/internal/store/database"
	"phasma/backend/internal/uploads"
	"phasma/backend/internal/users"
)

const defaultDatabaseURL = "postgres://postgres:postgres@localhost:5432/phasma?sslmode=disable"

func main() {
	setupLogger()

	port := env.String("PORT", "8080")
	databaseURL := env.String("DATABASE_URL", defaultDatabaseURL)

	repositories, databaseHandle, readiness, closeRepositories, err := openRepositories(databaseURL)
	if err != nil {
		slog.Error("failed to initialize repositories", "error", err)
		os.Exit(1)
	}
	defer closeRepositories()

	blobs, err := openBlobStore(context.Background())
	if err != nil {
		slog.Error("failed to initialize blob store", "error", err)
		os.Exit(1)
	}

	searchClient, err := openSearchClient(context.Background())
	if err != nil {
		slog.Error("failed to initialize search client", "error", err)
		os.Exit(1)
	}

	rateLimiter, err := openRateLimiter()
	if err != nil {
		slog.Error("failed to initialize rate limiter", "error", err)
		os.Exit(1)
	}

	loginThrottle, err := openLoginThrottle()
	if err != nil {
		slog.Error("failed to initialize login throttle", "error", err)
		os.Exit(1)
	}

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	sessionCleanupDone := startSessionCleanup(signalContext, repositories.Sessions)
	pipelineMonitor := pipeline.NewMonitor(time.Duration(env.Int("BACKGROUND_PIPELINE_STALE_SECONDS", 120)) * time.Second)
	backgroundReadiness := readiness

	var sweepOutboxDone <-chan struct{}
	if databaseHandle != nil {
		ch := make(chan struct{})
		sweepOutboxDone = ch
		go func() {
			defer close(ch)
			sweepOutboxPeriodically(signalContext, databaseHandle)
		}()
	}

	var reconcileFollowersDone <-chan struct{}
	if databaseHandle != nil {
		ch := make(chan struct{})
		reconcileFollowersDone = ch
		go func() {
			defer close(ch)
			reconcileFollowerCountsPeriodically(signalContext, databaseHandle)
		}()
	}

	var consumerWg sync.WaitGroup
	var outboxRelayDone <-chan struct{}
	if brokersEnv := os.Getenv("REDPANDA_BROKERS"); brokersEnv != "" {
		brokers := strings.Split(brokersEnv, ",")

		if databaseHandle != nil {
			var err error
			outboxRelayDone, err = startOutboxRelay(signalContext, databaseHandle, brokers, pipelineMonitor)
			if err != nil {
				slog.Error("failed to initialize outbox relay", "error", err)
				os.Exit(1)
			}
		}

		notifConsumer, err := notifications.NewConsumer(brokers, repositories.Notifications)
		if err != nil {
			slog.Error("failed to initialize notifications consumer", "error", err)
			os.Exit(1)
		}
		notifConsumer.SetMonitor(pipelineMonitor)
		defer notifConsumer.Close()
		consumerWg.Add(1)
		go func() {
			defer consumerWg.Done()
			notifConsumer.Run(signalContext)
		}()

		feedConsumer, err := feed.NewConsumer(brokers, repositories.Feed)
		if err != nil {
			slog.Error("failed to initialize feed consumer", "error", err)
			os.Exit(1)
		}
		feedConsumer.SetMonitor(pipelineMonitor)
		defer feedConsumer.Close()
		consumerWg.Add(1)
		go func() {
			defer consumerWg.Done()
			feedConsumer.Run(signalContext)
		}()

		backgroundReadiness = func(ctx context.Context) error {
			if err := readiness(ctx); err != nil {
				return err
			}
			return pipelineMonitor.Check(ctx)
		}
	}

	handler := app.New(app.Config{
		Blobs:         blobs,
		RateLimiter:   rateLimiter,
		LoginThrottle: loginThrottle,
		Readiness:     backgroundReadiness,
		Pipelines:     pipelineMonitor,
		SearchClient:  searchClient,
	}, repositories)

	addr := ":" + port
	slog.Info("starting backend", "addr", addr)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("backend stopped", "error", err)
			os.Exit(1)
		}
	case <-signalContext.Done():
		slog.Info("stopping backend")
	}
	stop()

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		slog.Error("failed to stop backend", "error", err)
		os.Exit(1)
	}
	<-sessionCleanupDone
	if sweepOutboxDone != nil {
		<-sweepOutboxDone
	}
	if reconcileFollowersDone != nil {
		<-reconcileFollowersDone
	}
	if outboxRelayDone != nil {
		<-outboxRelayDone
	}
	consumerWg.Wait()
}

func setupLogger() {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}

func openRepositories(databaseURL string) (app.Repositories, *database.DB, func(context.Context) error, func(), error) {
	sessionSecret := os.Getenv("SESSION_HASH_SECRET")
	if sessionSecret == "" {
		return app.Repositories{}, nil, nil, func() {}, errors.New("SESSION_HASH_SECRET is required")
	}

	client, err := database.New(context.Background(), databaseURL, sessionSecret)
	if err != nil {
		return app.Repositories{}, nil, nil, func() {}, err
	}

	return app.Repositories{
		SessionAuth:   sessions.NewSessionRepository(client),
		Users:         users.NewUserRepository(client),
		Sessions:      sessions.NewSessionRepository(client),
		Uploads:       uploads.NewUploadRepository(client),
		Posts:         posts.NewPostRepository(client),
		Comments:      comments.NewCommentRepository(client),
		Search:        search.NewSearchRepository(client),
		Feed:          feed.NewFeedRepository(client, feed.CelebThreshold),
		Notifications: notifications.NewNotificationRepository(client),
	}, client.DB(), client.Ping, client.Close, nil
}

func openSearchClient(ctx context.Context) (*search.SearchClient, error) {
	searchURL := env.String("MEILI_URL", "http://search:7700")
	masterKey := os.Getenv("MEILI_MASTER_KEY")
	return search.NewSearchClient(ctx, searchURL, masterKey)
}

func openBlobStore(ctx context.Context) (blobstore.Store, error) {
	return blobstore.NewStorageStore(
		ctx,
		env.String("S3_ENDPOINT", "http://storage:8333"),
		env.String("S3_BUCKET", "phasma"),
		env.String("S3_REGION", "us-east-1"),
		os.Getenv("S3_ACCESS_KEY"),
		os.Getenv("S3_SECRET_KEY"),
	)
}

func openRateLimiter() (httpx.RateLimiterStore, error) {
	cacheURL := os.Getenv("CACHE_URL")
	if cacheURL == "" {
		return httpx.NoopRateLimiterStore{}, nil
	}
	return httpx.NewCacheRateLimiterStore(cacheURL, os.Getenv("CACHE_PASSWORD"))
}

func openLoginThrottle() (sessions.LoginThrottle, error) {
	cacheURL := os.Getenv("CACHE_URL")
	if cacheURL == "" {
		return sessions.NoopLoginThrottle{}, nil
	}
	return sessions.NewCacheLoginThrottle(cacheURL, os.Getenv("CACHE_PASSWORD"))
}
