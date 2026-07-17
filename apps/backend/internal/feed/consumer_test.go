package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"phasma/backend/internal/pagination"
	"phasma/backend/internal/posts"
)

type fakeRepo struct {
	insertedEntries     [][]Entry
	insertErr           error
	followers           []int64
	followersErr        error
	recentEntries       []Entry
	recentEntriesErr    error
	isCelebrity         bool
	isCelebrityErr      error
	pruneFollowerID     int64
	pruneFolloweeID     int64
	pruneErr            error
	panicInsert         bool
	getFollowersCalled  bool
	getRecentCalled     bool
	getIsCelebrityCalls []int64
}

func (r *fakeRepo) ListFeed(_ context.Context, _ string, _ *pagination.Cursor, _ int) ([]posts.Post, *pagination.Cursor, error) {
	return nil, nil, nil
}

func (r *fakeRepo) InsertEntries(_ context.Context, entries []Entry) error {
	if r.panicInsert {
		panic("insert panic")
	}
	r.insertedEntries = append(r.insertedEntries, entries)
	return r.insertErr
}

func (r *fakeRepo) PruneByFollowee(_ context.Context, followerID, followeeID int64) error {
	r.pruneFollowerID = followerID
	r.pruneFolloweeID = followeeID
	return r.pruneErr
}

func (r *fakeRepo) GetFollowers(_ context.Context, _ int64) ([]int64, error) {
	r.getFollowersCalled = true
	return r.followers, r.followersErr
}

func (r *fakeRepo) GetRecentPostEntries(_ context.Context, _ int64, _ int) ([]Entry, error) {
	r.getRecentCalled = true
	return r.recentEntries, r.recentEntriesErr
}

func (r *fakeRepo) GetUserIsCelebrity(_ context.Context, userID int64) (bool, error) {
	r.getIsCelebrityCalls = append(r.getIsCelebrityCalls, userID)
	return r.isCelebrity, r.isCelebrityErr
}

func newConsumerWithRepo(repo Repository) *Consumer {
	return &Consumer{repo: repo}
}

// entityChangesMsg builds a payload for both old-format (followerCount) and
// new-format (isCelebrity) events so tests can exercise the backward-compat path.
func entityChangesMsg(authorID int64, postID int64, followerCount int64, isCelebrity bool) []byte {
	payload := entityChangesPayload{
		Table:         "posts",
		Op:            "upsert",
		ID:            postID,
		AuthorID:      fmt.Sprintf("%d", authorID),
		Created:       time.Now().UTC().Format(time.RFC3339Nano),
		IsCelebrity:   isCelebrity,
		FollowerCount: followerCount,
	}
	data, _ := json.Marshal(payload)
	return data
}

func TestHandleEntityChangesCelebSkipsFanout(t *testing.T) {
	repo := &fakeRepo{}
	c := newConsumerWithRepo(repo)

	// Old-format event: FollowerCount set, IsCelebrity absent (zero-valued false).
	c.handleEntityChanges(context.Background(), entityChangesMsg(42, 100, CelebThreshold+1, false))

	if repo.getFollowersCalled {
		t.Fatal("GetFollowers must not be called for celebrity authors")
	}
	if len(repo.insertedEntries) != 1 {
		t.Fatalf("InsertEntries called %d time(s), want 1", len(repo.insertedEntries))
	}
	batch := repo.insertedEntries[0]
	if len(batch) != 1 {
		t.Fatalf("inserted %d entries, want 1 (author only)", len(batch))
	}
	if batch[0].UserID != 42 || batch[0].PostID != 100 {
		t.Fatalf("entry = %+v, want userID=42 postID=100", batch[0])
	}
}

func TestHandleEntityChangesCelebIsCelebritySkipsFanout(t *testing.T) {
	repo := &fakeRepo{}
	c := newConsumerWithRepo(repo)

	// New-format event: IsCelebrity set, FollowerCount zero.
	c.handleEntityChanges(context.Background(), entityChangesMsg(42, 100, 0, true))

	if repo.getFollowersCalled {
		t.Fatal("GetFollowers must not be called when IsCelebrity is true")
	}
	if len(repo.insertedEntries) != 1 {
		t.Fatalf("InsertEntries called %d time(s), want 1", len(repo.insertedEntries))
	}
	batch := repo.insertedEntries[0]
	if len(batch) != 1 {
		t.Fatalf("inserted %d entries, want 1 (author only)", len(batch))
	}
	if batch[0].UserID != 42 || batch[0].PostID != 100 {
		t.Fatalf("entry = %+v, want userID=42 postID=100", batch[0])
	}
}

func TestHandleEntityChangesNormalFansOut(t *testing.T) {
	repo := &fakeRepo{
		followers: []int64{10, 11, 12},
	}
	c := newConsumerWithRepo(repo)

	c.handleEntityChanges(context.Background(), entityChangesMsg(42, 100, CelebThreshold, false))

	if !repo.getFollowersCalled {
		t.Fatal("GetFollowers must be called on the normal path")
	}
	if len(repo.insertedEntries) != 1 {
		t.Fatalf("InsertEntries called %d time(s), want 1", len(repo.insertedEntries))
	}
	// author + 3 followers = 4 entries
	batch := repo.insertedEntries[0]
	if len(batch) != 4 {
		t.Fatalf("inserted %d entries, want 4 (author + followers)", len(batch))
	}
	if batch[0].UserID != 42 {
		t.Fatalf("first entry userID = %d, want 42 (author)", batch[0].UserID)
	}
}

func TestHandleRecordSafelyRecoversPanic(t *testing.T) {
	repo := &fakeRepo{panicInsert: true}
	c := newConsumerWithRepo(repo)
	record := &kgo.Record{
		Topic:     topicEntityChanges,
		Partition: 1,
		Offset:    7,
		Value:     entityChangesMsg(42, 100, 0, false),
	}

	c.handleRecordSafely(context.Background(), record)

	key := recordKey{topic: topicEntityChanges, partition: 1, offset: 7}
	if c.panicCounts[key] != 1 {
		t.Fatalf("panic count = %d, want 1", c.panicCounts[key])
	}
}

func TestHandleFollowCelebSkipsBackfill(t *testing.T) {
	repo := &fakeRepo{isCelebrity: true}
	c := newConsumerWithRepo(repo)

	c.handleFollow(context.Background(), "1", "999")

	if repo.getRecentCalled {
		t.Fatal("GetRecentPostEntries must not be called when recipient is a celebrity")
	}
	if len(repo.insertedEntries) != 0 {
		t.Fatal("InsertEntries must not be called when recipient is a celebrity")
	}
	if len(repo.getIsCelebrityCalls) != 1 || repo.getIsCelebrityCalls[0] != 999 {
		t.Fatalf("GetUserIsCelebrity calls = %v, want [999]", repo.getIsCelebrityCalls)
	}
}

func TestHandleFollowNormalBackfills(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeRepo{
		isCelebrity: false,
		recentEntries: []Entry{
			{UserID: 999, PostID: 201, Created: now},
			{UserID: 999, PostID: 202, Created: now},
		},
	}
	c := newConsumerWithRepo(repo)

	c.handleFollow(context.Background(), "1", "999")

	if !repo.getRecentCalled {
		t.Fatal("GetRecentPostEntries must be called on the normal follow path")
	}
	if len(repo.insertedEntries) != 1 {
		t.Fatalf("InsertEntries called %d time(s), want 1", len(repo.insertedEntries))
	}
	batch := repo.insertedEntries[0]
	if len(batch) != 2 {
		t.Fatalf("inserted %d backfill entries, want 2", len(batch))
	}
	// All entries must be attributed to the actor (follower), not the recipient.
	for _, e := range batch {
		if e.UserID != 1 {
			t.Fatalf("backfill entry userID = %d, want 1 (actor)", e.UserID)
		}
	}
}

func TestHandleEntityChangesFollowerListAtThresholdWarns(t *testing.T) {
	followers := make([]int64, CelebThreshold)
	for i := range followers {
		followers[i] = int64(1000 + i)
	}
	repo := &fakeRepo{followers: followers}
	c := newConsumerWithRepo(repo)

	c.handleEntityChanges(context.Background(), entityChangesMsg(42, 100, 0, false))

	if len(repo.insertedEntries) != 1 {
		t.Fatalf("InsertEntries called %d time(s), want 1", len(repo.insertedEntries))
	}
	batch := repo.insertedEntries[0]
	if len(batch) != int(CelebThreshold)+1 {
		t.Fatalf("inserted %d entries, want %d (author + all followers)", len(batch), int(CelebThreshold)+1)
	}
	if batch[0].UserID != 42 {
		t.Fatalf("first entry userID = %d, want 42 (author)", batch[0].UserID)
	}
}
