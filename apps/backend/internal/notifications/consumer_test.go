package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"

	"phasma/backend/internal/pagination"
)

// fakeRepo implements Repository for consumer unit tests.
type fakeRepo struct {
	created            []Notification
	createErr          error
	deletedByEntity    []struct{ entityType, entityID string }
	deleteByEntityErr  error
	panicCreate        bool
	deletedByActorType []struct {
		actorID, recipientID int64
		notifType, entityID  string
	}
	deleteByActorErr error
}

func (r *fakeRepo) ListNotifications(_ context.Context, _ int64, _ *pagination.Cursor, _ int) ([]Notification, error) {
	return nil, nil
}
func (r *fakeRepo) MarkRead(_ context.Context, _ string, _ int64) error { return nil }
func (r *fakeRepo) UnreadCount(_ context.Context, _ int64) (int, error) { return 0, nil }

func (r *fakeRepo) DeleteByEntity(_ context.Context, entityType, entityID string) error {
	r.deletedByEntity = append(r.deletedByEntity, struct{ entityType, entityID string }{entityType, entityID})
	return r.deleteByEntityErr
}

func (r *fakeRepo) DeleteByActorAndType(_ context.Context, actorID, recipientID int64, notifType, entityID string) error {
	r.deletedByActorType = append(r.deletedByActorType, struct {
		actorID, recipientID int64
		notifType, entityID  string
	}{actorID, recipientID, notifType, entityID})
	return r.deleteByActorErr
}

func (r *fakeRepo) CreateNotification(_ context.Context, n Notification) error {
	if r.panicCreate {
		panic("create panic")
	}
	r.created = append(r.created, n)
	return r.createErr
}

var _ Repository = (*fakeRepo)(nil)

func newTestConsumer(repo *fakeRepo) *Consumer {
	return &Consumer{repo: repo}
}

func activityRecord(topic string, partition int32, offset int64, data []byte) *kgo.Record {
	return &kgo.Record{Topic: topic, Partition: partition, Offset: offset, Value: data}
}

func entityDeletePayload(postID string, commentPublicIDs []string) []byte {
	p := entityChangesPayload{Table: "posts", Op: "delete", PostID: postID, CommentPublicIDs: commentPublicIDs}
	b, _ := json.Marshal(p)
	return b
}

func makeExternalID(topic string, partition int32, offset int64) string {
	return fmt.Sprintf("%s-%d-%d", topic, partition, offset)
}

func TestHandleLikeCreatesNotification(t *testing.T) {
	repo := &fakeRepo{}
	c := newTestConsumer(repo)

	payload := activityPayload{Op: "like", PostID: "post-uuid", ActorID: "1", RecipientID: "2"}
	c.handleLike(context.Background(), payload, "ext-1")

	if len(repo.created) != 1 {
		t.Fatalf("created %d notifications, want 1", len(repo.created))
	}
	n := repo.created[0]
	if n.Type != "like" || n.ActorID != 1 || n.UserID != 2 || n.EntityID != "post-uuid" || n.ExternalID != "ext-1" {
		t.Fatalf("unexpected notification: %+v", n)
	}
}

func TestHandleLikeSelfSkipped(t *testing.T) {
	repo := &fakeRepo{}
	c := newTestConsumer(repo)

	payload := activityPayload{Op: "like", PostID: "post-uuid", ActorID: "5", RecipientID: "5"}
	c.handleLike(context.Background(), payload, "ext-1")

	if len(repo.created) != 0 {
		t.Fatalf("expected 0 notifications for self-like, got %d", len(repo.created))
	}
}

func TestHandleUnlikeDeletesByActorAndType(t *testing.T) {
	repo := &fakeRepo{}
	c := newTestConsumer(repo)

	payload := activityPayload{Op: "unlike", PostID: "post-uuid", ActorID: "1", RecipientID: "2"}
	c.handleUnlike(context.Background(), payload)

	if len(repo.deletedByActorType) != 1 {
		t.Fatalf("DeleteByActorAndType called %d times, want 1", len(repo.deletedByActorType))
	}
	d := repo.deletedByActorType[0]
	if d.actorID != 1 || d.recipientID != 2 || d.notifType != "like" || d.entityID != "post-uuid" {
		t.Fatalf("unexpected delete params: %+v", d)
	}
}

func TestHandleCommentCreatesNotification(t *testing.T) {
	repo := &fakeRepo{}
	c := newTestConsumer(repo)

	payload := activityPayload{Op: "comment", PostID: "post-uuid", CommentID: "550e8400-e29b-41d4-a716-446655440042", ActorID: "1", RecipientID: "2"}
	c.handleComment(context.Background(), payload, "ext-2")

	if len(repo.created) != 1 {
		t.Fatalf("created %d notifications, want 1", len(repo.created))
	}
	n := repo.created[0]
	if n.Type != "comment" || n.ActorID != 1 || n.UserID != 2 || n.EntityID != "550e8400-e29b-41d4-a716-446655440042" || n.ExternalID != "ext-2" {
		t.Fatalf("unexpected notification: %+v", n)
	}
}

func TestHandleCommentSelfSkipped(t *testing.T) {
	repo := &fakeRepo{}
	c := newTestConsumer(repo)

	payload := activityPayload{Op: "comment", CommentID: "550e8400-e29b-41d4-a716-446655440042", ActorID: "3", RecipientID: "3"}
	c.handleComment(context.Background(), payload, "ext-3")

	if len(repo.created) != 0 {
		t.Fatalf("expected 0 notifications for self-comment, got %d", len(repo.created))
	}
}

func TestHandleUncommentDeletesByEntity(t *testing.T) {
	repo := &fakeRepo{}
	c := newTestConsumer(repo)

	payload := activityPayload{Op: "uncomment", CommentID: "550e8400-e29b-41d4-a716-446655440099"}
	c.handleUncomment(context.Background(), payload)

	if len(repo.deletedByEntity) != 1 {
		t.Fatalf("DeleteByEntity called %d times, want 1", len(repo.deletedByEntity))
	}
	d := repo.deletedByEntity[0]
	if d.entityType != "comment" || d.entityID != "550e8400-e29b-41d4-a716-446655440099" {
		t.Fatalf("unexpected delete params: %+v", d)
	}
}

func TestHandleFollowNotificationCreatesEntry(t *testing.T) {
	repo := &fakeRepo{}
	c := newTestConsumer(repo)

	payload := activityPayload{Op: "follow", ActorID: "7", RecipientID: "8"}
	c.handleFollowNotification(context.Background(), payload, "ext-4")

	if len(repo.created) != 1 {
		t.Fatalf("created %d notifications, want 1", len(repo.created))
	}
	n := repo.created[0]
	if n.Type != "follow" || n.ActorID != 7 || n.UserID != 8 || n.EntityID != "7" || n.ExternalID != "ext-4" {
		t.Fatalf("unexpected notification: %+v", n)
	}
}

func TestHandleFollowSelfSkipped(t *testing.T) {
	repo := &fakeRepo{}
	c := newTestConsumer(repo)

	payload := activityPayload{Op: "follow", ActorID: "5", RecipientID: "5"}
	c.handleFollowNotification(context.Background(), payload, "ext-5")

	if len(repo.created) != 0 {
		t.Fatalf("expected 0 notifications for self-follow, got %d", len(repo.created))
	}
}

func TestHandleUnfollowDeletesEntry(t *testing.T) {
	repo := &fakeRepo{}
	c := newTestConsumer(repo)

	payload := activityPayload{Op: "unfollow", ActorID: "7", RecipientID: "8"}
	c.handleUnfollowNotification(context.Background(), payload)

	if len(repo.deletedByActorType) != 1 {
		t.Fatalf("DeleteByActorAndType called %d times, want 1", len(repo.deletedByActorType))
	}
	d := repo.deletedByActorType[0]
	if d.actorID != 7 || d.recipientID != 8 || d.notifType != "follow" || d.entityID != "7" {
		t.Fatalf("unexpected delete params: %+v", d)
	}
}

func TestHandleEntityChangesPostDeleteCleansLikesAndComments(t *testing.T) {
	repo := &fakeRepo{}
	c := newTestConsumer(repo)

	cid1 := "550e8400-e29b-41d4-a716-446655440010"
	cid2 := "550e8400-e29b-41d4-a716-446655440011"
	rec := activityRecord(topicEntityChanges, 0, 0, entityDeletePayload("post-uuid", []string{cid1, cid2}))
	c.handleEntityChanges(context.Background(), rec)

	if len(repo.deletedByEntity) != 3 {
		t.Fatalf("DeleteByEntity called %d times, want 3", len(repo.deletedByEntity))
	}
	if repo.deletedByEntity[0].entityType != "post" || repo.deletedByEntity[0].entityID != "post-uuid" {
		t.Fatalf("first deletion: want post/post-uuid, got %+v", repo.deletedByEntity[0])
	}
	if repo.deletedByEntity[1].entityType != "comment" || repo.deletedByEntity[1].entityID != cid1 {
		t.Fatalf("second deletion: want comment/%s, got %+v", cid1, repo.deletedByEntity[1])
	}
	if repo.deletedByEntity[2].entityType != "comment" || repo.deletedByEntity[2].entityID != cid2 {
		t.Fatalf("third deletion: want comment/%s, got %+v", cid2, repo.deletedByEntity[2])
	}
}

func TestHandleEntityChangesIgnoresUpsertOp(t *testing.T) {
	repo := &fakeRepo{}
	c := newTestConsumer(repo)

	data, _ := json.Marshal(entityChangesPayload{Table: "posts", Op: "upsert", PostID: "post-uuid"})
	rec := activityRecord(topicEntityChanges, 0, 0, data)
	c.handleEntityChanges(context.Background(), rec)

	if len(repo.deletedByEntity) != 0 {
		t.Fatal("expected no deletions for upsert op")
	}
}

func TestHandleEntityChangesMissingPostIDIsNoOp(t *testing.T) {
	repo := &fakeRepo{}
	c := newTestConsumer(repo)

	data, _ := json.Marshal(entityChangesPayload{Table: "posts", Op: "delete", PostID: ""})
	rec := activityRecord(topicEntityChanges, 0, 0, data)
	c.handleEntityChanges(context.Background(), rec)

	if len(repo.deletedByEntity) != 0 {
		t.Fatal("expected no deletions when post_id is empty")
	}
}

func TestHandleActivityUsesRecordKeyForExternalID(t *testing.T) {
	repo := &fakeRepo{}
	c := newTestConsumer(repo)

	data, _ := json.Marshal(activityPayload{Op: "like", PostID: "p", ActorID: "1", RecipientID: "2"})
	rec := activityRecord(topicActivity, 3, 77, data)
	rec.Key = []byte("outbox-123")
	c.handleActivity(context.Background(), rec)

	if len(repo.created) != 1 || repo.created[0].ExternalID != "outbox-123" {
		t.Fatalf("external_id = %q, want %q", func() string {
			if len(repo.created) == 0 {
				return "<none>"
			}
			return repo.created[0].ExternalID
		}(), "outbox-123")
	}
}

func TestHandleActivityFallsBackToOffsetExternalID(t *testing.T) {
	repo := &fakeRepo{}
	c := newTestConsumer(repo)

	data, _ := json.Marshal(activityPayload{Op: "like", PostID: "p", ActorID: "1", RecipientID: "2"})
	rec := activityRecord(topicActivity, 3, 77, data)
	c.handleActivity(context.Background(), rec)

	wantExtID := makeExternalID(topicActivity, 3, 77)
	if len(repo.created) != 1 || repo.created[0].ExternalID != wantExtID {
		t.Fatalf("external_id = %q, want %q", func() string {
			if len(repo.created) == 0 {
				return "<none>"
			}
			return repo.created[0].ExternalID
		}(), wantExtID)
	}
}

func TestHandleRecordSafelyRecoversPanic(t *testing.T) {
	repo := &fakeRepo{panicCreate: true}
	c := newTestConsumer(repo)
	data, _ := json.Marshal(activityPayload{Op: "like", PostID: "p", ActorID: "1", RecipientID: "2"})
	record := activityRecord(topicActivity, 3, 77, data)

	c.handleRecordSafely(context.Background(), record)

	key := recordKey{topic: topicActivity, partition: 3, offset: 77}
	if c.panicCounts[key] != 1 {
		t.Fatalf("panic count = %d, want 1", c.panicCounts[key])
	}
}

func TestParseActorRecipientInvalidInputReturnsFalse(t *testing.T) {
	cases := []struct{ actor, recipient string }{
		{"", "2"},
		{"1", ""},
		{"abc", "2"},
		{"1", "xyz"},
	}
	for _, tc := range cases {
		_, _, ok := parseActorRecipient(tc.actor, tc.recipient, "test")
		if ok {
			t.Errorf("expected false for actor=%q recipient=%q", tc.actor, tc.recipient)
		}
	}
}
