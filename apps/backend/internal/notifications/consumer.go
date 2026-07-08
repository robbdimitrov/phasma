package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"phasma/backend/internal/pipeline"
)

const (
	notificationsConsumerGroup = "notifications-consumer"
	topicEntityChanges         = "entity-changes"
	topicActivity              = "activity"
	maxRecordPanics            = 3
	pipelineName               = "notifications-consumer"
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
		kgo.ConsumerGroup(notificationsConsumerGroup),
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
					slog.Error("notifications consumer panicked", "panic", recovered)
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
					slog.Warn("notifications consumer: fetch error", "topic", topic, "partition", partition, "error", err)
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
					slog.Warn("notifications consumer: commit failed", "error", err)
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
			slog.Error("notifications consumer: skipping repeatedly panicking record", attrs...)
			c.monitor.Error(pipelineName, errPanic(recovered))
			return
		}
		slog.Warn("notifications consumer: skipping panicking record", attrs...)
		c.monitor.Error(pipelineName, errPanic(recovered))
	}()
	c.handle(ctx, record)
}

type entityChangesPayload struct {
	Table            string   `json:"table"`
	Op               string   `json:"op"`
	PostID           string   `json:"post_id"`
	CommentPublicIDs []string `json:"comment_public_ids"`
}

type activityPayload struct {
	Op          string `json:"op"`
	PostID      string `json:"post_id"`
	CommentID   string `json:"comment_id"`
	ActorID     string `json:"actor_id"`
	RecipientID string `json:"recipient_id"`
}

func (c *Consumer) handle(ctx context.Context, record *kgo.Record) {
	switch record.Topic {
	case topicEntityChanges:
		c.handleEntityChanges(ctx, record)
	case topicActivity:
		c.handleActivity(ctx, record)
	}
}

func (c *Consumer) handleEntityChanges(ctx context.Context, record *kgo.Record) {
	var payload entityChangesPayload
	if err := json.Unmarshal(record.Value, &payload); err != nil {
		slog.Warn("notifications consumer: failed to decode entity-changes payload", "error", err)
		return
	}
	if payload.Table != "posts" || payload.Op != "delete" {
		return
	}
	if payload.PostID == "" {
		return
	}
	if err := c.repo.DeleteByEntity(ctx, "post", payload.PostID); err != nil {
		slog.Warn("notifications consumer: failed to delete notifications for post", "post_id", payload.PostID, "error", err)
	}
	for _, cid := range payload.CommentPublicIDs {
		if err := c.repo.DeleteByEntity(ctx, "comment", cid); err != nil {
			slog.Warn("notifications consumer: failed to delete comment notification", "comment_id", cid, "error", err)
		}
	}
}

func (c *Consumer) handleActivity(ctx context.Context, record *kgo.Record) {
	var payload activityPayload
	if err := json.Unmarshal(record.Value, &payload); err != nil {
		slog.Warn("notifications consumer: failed to decode activity payload", "error", err)
		return
	}

	externalID := string(record.Key)
	if externalID == "" {
		externalID = fmt.Sprintf("%s-%d-%d", record.Topic, record.Partition, record.Offset)
	}

	switch payload.Op {
	case "like":
		c.handleLike(ctx, payload, externalID)
	case "unlike":
		c.handleUnlike(ctx, payload)
	case "comment":
		c.handleComment(ctx, payload, externalID)
	case "uncomment":
		c.handleUncomment(ctx, payload)
	case "follow":
		c.handleFollowNotification(ctx, payload, externalID)
	case "unfollow":
		c.handleUnfollowNotification(ctx, payload)
	}
}

func (c *Consumer) handleLike(ctx context.Context, payload activityPayload, externalID string) {
	actorID, recipientID, ok := parseActorRecipient(payload.ActorID, payload.RecipientID, "like")
	if !ok {
		return
	}
	if actorID == recipientID {
		return
	}
	if err := c.repo.CreateNotification(ctx, Notification{
		ExternalID: externalID,
		UserID:     recipientID,
		ActorID:    actorID,
		Type:       "like",
		EntityID:   payload.PostID,
	}); err != nil {
		slog.Warn("notifications consumer: failed to create like notification", "error", err)
	}
}

func (c *Consumer) handleUnlike(ctx context.Context, payload activityPayload) {
	actorID, recipientID, ok := parseActorRecipient(payload.ActorID, payload.RecipientID, "unlike")
	if !ok {
		return
	}
	if err := c.repo.DeleteByActorAndType(ctx, actorID, recipientID, "like", payload.PostID); err != nil {
		slog.Warn("notifications consumer: failed to delete like notification", "error", err)
	}
}

func (c *Consumer) handleComment(ctx context.Context, payload activityPayload, externalID string) {
	actorID, recipientID, ok := parseActorRecipient(payload.ActorID, payload.RecipientID, "comment")
	if !ok {
		return
	}
	if actorID == recipientID {
		return
	}
	if err := c.repo.CreateNotification(ctx, Notification{
		ExternalID: externalID,
		UserID:     recipientID,
		ActorID:    actorID,
		Type:       "comment",
		EntityID:   payload.CommentID,
	}); err != nil {
		slog.Warn("notifications consumer: failed to create comment notification", "error", err)
	}
}

func (c *Consumer) handleUncomment(ctx context.Context, payload activityPayload) {
	if err := c.repo.DeleteByEntity(ctx, "comment", payload.CommentID); err != nil {
		slog.Warn("notifications consumer: failed to delete comment notification", "error", err)
	}
}

func (c *Consumer) handleFollowNotification(ctx context.Context, payload activityPayload, externalID string) {
	actorID, recipientID, ok := parseActorRecipient(payload.ActorID, payload.RecipientID, "follow")
	if !ok {
		return
	}
	if actorID == recipientID {
		return
	}
	entityID := strconv.FormatInt(actorID, 10)
	if err := c.repo.CreateNotification(ctx, Notification{
		ExternalID: externalID,
		UserID:     recipientID,
		ActorID:    actorID,
		Type:       "follow",
		EntityID:   entityID,
	}); err != nil {
		slog.Warn("notifications consumer: failed to create follow notification", "error", err)
	}
}

func (c *Consumer) handleUnfollowNotification(ctx context.Context, payload activityPayload) {
	actorID, recipientID, ok := parseActorRecipient(payload.ActorID, payload.RecipientID, "unfollow")
	if !ok {
		return
	}
	entityID := strconv.FormatInt(actorID, 10)
	if err := c.repo.DeleteByActorAndType(ctx, actorID, recipientID, "follow", entityID); err != nil {
		slog.Warn("notifications consumer: failed to delete follow notification", "error", err)
	}
}

func parseActorRecipient(actorIDStr, recipientIDStr, op string) (int64, int64, bool) {
	actorID, err := strconv.ParseInt(actorIDStr, 10, 64)
	if err != nil {
		slog.Warn("notifications consumer: invalid actor_id", "op", op, "actor_id", actorIDStr)
		return 0, 0, false
	}
	recipientID, err := strconv.ParseInt(recipientIDStr, 10, 64)
	if err != nil {
		slog.Warn("notifications consumer: invalid recipient_id", "op", op, "recipient_id", recipientIDStr)
		return 0, 0, false
	}
	return actorID, recipientID, true
}
