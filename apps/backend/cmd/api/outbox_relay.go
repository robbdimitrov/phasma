package main

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"

	"phasma/backend/internal/pipeline"
	"phasma/backend/internal/store/database"
)

type outboxEvent struct {
	ID      int64
	Topic   string
	Payload []byte
}

const outboxRelayPipeline = "outbox-relay"

func startOutboxRelay(ctx context.Context, db *database.DB, brokers []string, monitor *pipeline.Monitor) (<-chan struct{}, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, err
	}

	done := make(chan struct{})
	monitor.Start(outboxRelayPipeline)
	go func() {
		defer close(done)
		defer client.Close()
		defer monitor.Stop(outboxRelayPipeline)
		relayOutboxPeriodically(ctx, db, client, monitor)
	}()
	return done, nil
}

func relayOutboxPeriodically(ctx context.Context, db *database.DB, client *kgo.Client, monitor *pipeline.Monitor) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			relayOutboxBatch(ctx, db, client, monitor)
		}
	}
}

func relayOutboxBatch(ctx context.Context, db *database.DB, client *kgo.Client, monitor *pipeline.Monitor) {
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error("outbox relay panicked", "panic", recovered)
			monitor.Error(outboxRelayPipeline, errPanic(recovered))
		}
	}()

	tx, err := db.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		slog.Warn("outbox relay: begin transaction failed", "error", err)
		monitor.Error(outboxRelayPipeline, err)
		return
	}
	defer database.Rollback(context.Background(), tx)

	rows, err := tx.Query(ctx, `
		SELECT id, topic, payload::text
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY id
		LIMIT 100
		FOR UPDATE SKIP LOCKED`)
	if err != nil {
		slog.Warn("outbox relay: query failed", "error", err)
		monitor.Error(outboxRelayPipeline, err)
		return
	}
	events, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (outboxEvent, error) {
		var event outboxEvent
		if err := row.Scan(&event.ID, &event.Topic, &event.Payload); err != nil {
			return outboxEvent{}, err
		}
		return event, nil
	})
	if err != nil {
		slog.Warn("outbox relay: scan failed", "error", err)
		monitor.Error(outboxRelayPipeline, err)
		return
	}
	if len(events) == 0 {
		monitor.Progress(outboxRelayPipeline, 0, "idle")
		return
	}

	records := make([]*kgo.Record, 0, len(events))
	ids := make([]int64, 0, len(events))
	for _, event := range events {
		records = append(records, &kgo.Record{
			Topic: event.Topic,
			Key:   []byte(strconv.FormatInt(event.ID, 10)),
			Value: event.Payload,
		})
		ids = append(ids, event.ID)
	}

	produceCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := client.ProduceSync(produceCtx, records...).FirstErr(); err != nil {
		slog.Warn("outbox relay: publish failed", "error", err)
		monitor.Error(outboxRelayPipeline, err)
		return
	}

	if _, err := tx.Exec(ctx, `UPDATE outbox SET published_at = now() WHERE id = ANY($1)`, ids); err != nil {
		slog.Warn("outbox relay: mark published failed", "error", err)
		monitor.Error(outboxRelayPipeline, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		slog.Warn("outbox relay: commit failed", "error", err)
		monitor.Error(outboxRelayPipeline, err)
		return
	}
	monitor.Progress(outboxRelayPipeline, uint64(len(events)), "published")
}
