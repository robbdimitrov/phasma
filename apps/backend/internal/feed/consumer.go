package feed

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"phasma/backend/internal/pipeline"
)

const (
	consumerGroup       = "feed-consumer"
	topicEntityChanges  = "entity-changes"
	topicActivity       = "activity"
	followBackfillLimit = 50
	maxRecordPanics     = 3
	pipelineName        = "feed-consumer"
	// Bound offset commits; PollFetches stays unbounded since it drives group
	// rebalance and a short timeout there causes a rebalance livelock.
	commitTimeout = 10 * time.Second
)

type Consumer struct {
	client      *kgo.Client
	repo        Repository
	monitor     *pipeline.Monitor
	panicCounts map[recordKey]int
}

type recordKey struct {
	topic     string
	partition int32
	offset    int64
}

func NewConsumer(brokers []string, repo Repository) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(consumerGroup),
		kgo.ConsumeTopics(topicEntityChanges, topicActivity),
		kgo.FetchMaxWait(5*time.Second),
	)
	if err != nil {
		return nil, err
	}
	return &Consumer{client: client, repo: repo, monitor: pipeline.NewMonitor(0), panicCounts: map[recordKey]int{}}, nil
}

func (c *Consumer) SetMonitor(monitor *pipeline.Monitor) {
	if monitor == nil {
		monitor = pipeline.NewMonitor(0)
	}
	c.monitor = monitor
	c.monitor.Register(pipelineName)
}

func (c *Consumer) Close() {
	c.client.Close()
}

func (c *Consumer) Run(ctx context.Context) {
	if c.monitor == nil {
		c.monitor = pipeline.NewMonitor(0)
	}
	c.monitor.Start(pipelineName)
	defer c.monitor.Stop(pipelineName)
	for {
		closed := false
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					slog.Error("feed consumer panicked", "panic", recovered)
					c.monitor.Error(pipelineName, errPanic(recovered))
				}
			}()
			for {
				fetches := c.client.PollFetches(ctx)
				if ctx.Err() != nil {
					return
				}
				if fetches.IsClientClosed() {
					closed = true
					return
				}
				hadError := false
				fetches.EachError(func(topic string, partition int32, err error) {
					hadError = true
					slog.Warn("feed consumer: fetch error", "topic", topic, "partition", partition, "error", err)
					c.monitor.Error(pipelineName, err)
				})
				var records uint64
				fetches.EachRecord(func(record *kgo.Record) {
					c.handleRecordSafely(ctx, record)
					records++
				})
				if !hadError {
					c.monitor.Progress(pipelineName, records, "poll")
				}
				commitCtx, commitCancel := context.WithTimeout(ctx, commitTimeout)
				err := c.client.CommitUncommittedOffsets(commitCtx)
				commitCancel()
				if err != nil {
					slog.Warn("feed consumer: commit failed", "error", err)
					c.monitor.Error(pipelineName, err)
				}
			}
		}()
		if ctx.Err() != nil || closed {
			return
		}
	}
}

func (c *Consumer) handleRecordSafely(ctx context.Context, record *kgo.Record) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		key := recordKey{topic: record.Topic, partition: record.Partition, offset: record.Offset}
		if c.panicCounts == nil {
			c.panicCounts = map[recordKey]int{}
		}
		c.panicCounts[key]++
		attrs := []any{
			"topic", record.Topic,
			"partition", record.Partition,
			"offset", record.Offset,
			"panic", recovered,
			"failures", c.panicCounts[key],
		}
		if c.panicCounts[key] >= maxRecordPanics {
			slog.Error("feed consumer: skipping repeatedly panicking record", attrs...)
			c.monitor.Error(pipelineName, errPanic(recovered))
			return
		}
		slog.Warn("feed consumer: skipping panicking record", attrs...)
		c.monitor.Error(pipelineName, errPanic(recovered))
	}()
	c.handle(ctx, record)
}

type entityChangesPayload struct {
	Table         string `json:"table"`
	Op            string `json:"op"`
	ID            int64  `json:"id"`
	AuthorID      string `json:"author_id"`
	Created       string `json:"created"`
	IsCelebrity   bool   `json:"is_celebrity"`
	FollowerCount int64  `json:"follower_count"`
}

type activityPayload struct {
	Op          string `json:"op"`
	ActorID     string `json:"actor_id"`
	RecipientID string `json:"recipient_id"`
}

func (c *Consumer) handle(ctx context.Context, record *kgo.Record) {
	switch record.Topic {
	case topicEntityChanges:
		c.handleEntityChanges(ctx, record.Value)
	case topicActivity:
		c.handleActivity(ctx, record.Value)
	}
}

func (c *Consumer) handleEntityChanges(ctx context.Context, data []byte) {
	var payload entityChangesPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		slog.Warn("feed consumer: failed to decode entity-changes payload", "error", err)
		return
	}
	if payload.Table != "posts" || payload.Op != "upsert" {
		return
	}

	authorID, err := strconv.ParseInt(payload.AuthorID, 10, 64)
	if err != nil {
		slog.Warn("feed consumer: invalid author_id", "author_id", payload.AuthorID)
		return
	}

	var created time.Time
	if payload.Created != "" {
		created, err = time.Parse(time.RFC3339Nano, payload.Created)
		if err != nil {
			created = time.Now().UTC()
		}
	} else {
		created = time.Now().UTC()
	}

	if payload.IsCelebrity || payload.FollowerCount > CelebThreshold {
		if err := c.repo.InsertEntries(ctx, []Entry{{
			UserID:  authorID,
			PostID:  payload.ID,
			Created: created,
		}}); err != nil {
			slog.Warn("feed consumer: failed to insert author feed entry", "post_id", payload.ID, "error", err)
		}
		return
	}

	followers, err := c.repo.GetFollowers(ctx, authorID)
	if err != nil {
		slog.Warn("feed consumer: failed to get followers", "author_id", authorID, "error", err)
		return
	}
	if int64(len(followers)) >= CelebThreshold {
		slog.Warn("feed consumer: follower list truncated at threshold; some followers may miss this post",
			"author_id", authorID, "post_id", payload.ID, "returned", len(followers))
	}

	entries := make([]Entry, 0, len(followers)+1)
	entries = append(entries, Entry{UserID: authorID, PostID: payload.ID, Created: created})
	for _, followerID := range followers {
		entries = append(entries, Entry{UserID: followerID, PostID: payload.ID, Created: created})
	}

	if err := c.repo.InsertEntries(ctx, entries); err != nil {
		slog.Warn("feed consumer: failed to insert feed entries", "post_id", payload.ID, "error", err)
	}
}

func (c *Consumer) handleActivity(ctx context.Context, data []byte) {
	var payload activityPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		slog.Warn("feed consumer: failed to decode activity payload", "error", err)
		return
	}

	switch payload.Op {
	case "follow":
		c.handleFollow(ctx, payload.ActorID, payload.RecipientID)
	case "unfollow":
		c.handleUnfollow(ctx, payload.ActorID, payload.RecipientID)
	}
}

func (c *Consumer) handleFollow(ctx context.Context, actorIDStr, recipientIDStr string) {
	actorID, err := strconv.ParseInt(actorIDStr, 10, 64)
	if err != nil {
		slog.Warn("feed consumer: invalid actor_id", "actor_id", actorIDStr)
		return
	}
	recipientID, err := strconv.ParseInt(recipientIDStr, 10, 64)
	if err != nil {
		slog.Warn("feed consumer: invalid recipient_id", "recipient_id", recipientIDStr)
		return
	}

	isCeleb, err := c.repo.GetUserIsCelebrity(ctx, recipientID)
	if err != nil {
		slog.Warn("feed consumer: failed to get celebrity status", "recipient_id", recipientID, "error", err)
		return
	}
	if isCeleb {
		return // celebrity posts are served on-read; no backfill
	}

	recentEntries, err := c.repo.GetRecentPostEntries(ctx, recipientID, followBackfillLimit)
	if err != nil {
		slog.Warn("feed consumer: failed to get recent posts for follow", "recipient_id", recipientID, "error", err)
		return
	}
	if len(recentEntries) == 0 {
		return
	}

	entries := make([]Entry, len(recentEntries))
	for i, e := range recentEntries {
		entries[i] = Entry{UserID: actorID, PostID: e.PostID, Created: e.Created}
	}

	if err := c.repo.InsertEntries(ctx, entries); err != nil {
		slog.Warn("feed consumer: failed to insert feed entries on follow", "actor_id", actorID, "error", err)
	}
}

func (c *Consumer) handleUnfollow(ctx context.Context, actorIDStr, recipientIDStr string) {
	actorID, err := strconv.ParseInt(actorIDStr, 10, 64)
	if err != nil {
		slog.Warn("feed consumer: invalid actor_id", "actor_id", actorIDStr)
		return
	}
	recipientID, err := strconv.ParseInt(recipientIDStr, 10, 64)
	if err != nil {
		slog.Warn("feed consumer: invalid recipient_id", "recipient_id", recipientIDStr)
		return
	}

	if err := c.repo.PruneByFollowee(ctx, actorID, recipientID); err != nil {
		slog.Warn("feed consumer: failed to prune feed on unfollow", "actor_id", actorID, "recipient_id", recipientID, "error", err)
	}
}
