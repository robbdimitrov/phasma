package database_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"phasma/backend/internal/auth"
	"phasma/backend/internal/comments"
	"phasma/backend/internal/httpx"
	"phasma/backend/internal/pagination"
	"phasma/backend/internal/posts"
	"phasma/backend/internal/search"
	"phasma/backend/internal/sessions"
	"phasma/backend/internal/store"
	"phasma/backend/internal/store/database"
	"phasma/backend/internal/uploads"
	"phasma/backend/internal/users"
)

const testSessionSecret = "integration-secret"

const (
	sessionTTL                     = 7 * 24 * time.Hour
	defaultSessionAbsoluteTTLHours = 720
	maxSessionsPerUser             = 100
)

type testPost struct {
	ID       int
	PublicID string
}

type testClient struct {
	*database.Client
}

func openTestClient(t *testing.T) *testClient {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	databaseURL := os.Getenv("PHASMA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PHASMA_TEST_DATABASE_URL is not set")
	}
	client, err := database.New(context.Background(), databaseURL, testSessionSecret)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(client.Close)
	_, err = client.DB().Pool().Exec(context.Background(),
		`TRUNCATE post_hashtags, hashtags, comments, likes, follows, posts,
		 uploads, sessions, users RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate error = %v", err)
	}
	return &testClient{Client: client}
}

func createTestUser(t *testing.T, client *testClient, suffix string) int {
	t.Helper()
	id, err := client.CreateUser(
		context.Background(),
		"User "+suffix,
		"user_"+suffix,
		suffix+"@example.com",
		"password-hash",
	)
	if err != nil {
		t.Fatalf("CreateUser(%q) error = %v", suffix, err)
	}
	return id
}

func insertTestPost(t *testing.T, client *testClient, userID int, filename string, created time.Time) testPost {
	t.Helper()
	var post testPost
	err := client.DB().Pool().QueryRow(
		context.Background(),
		`INSERT INTO posts (user_id, filename, created)
		VALUES ($1, $2, $3) RETURNING id, public_id`,
		userID, filename, created,
	).Scan(&post.ID, &post.PublicID)
	if err != nil {
		t.Fatalf("insert post %q error = %v", filename, err)
	}
	return post
}

func postIDs(items []posts.Post) []int {
	ids := make([]int, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func commentIDs(items []comments.Comment) []int {
	ids := make([]int, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func userIDs(items []users.User) []int {
	ids := make([]int, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func assertIDs(t *testing.T, got, want []int) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
}

func assertNoDuplicateIDs(t *testing.T, ids []int) {
	t.Helper()
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate ID %d in %v", id, ids)
		}
		seen[id] = struct{}{}
	}
}

func stringID(id int) string {
	return strconv.Itoa(id)
}

func TestDatabaseRepositoryRejectsDuplicateUsernameAndEmail(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	createTestUser(t, client, "unique")

	if _, err := client.CreateUser(ctx, "Duplicate", "user_unique", "other@example.com", "hash"); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate username error = %v, want conflict", err)
	}
	if _, err := client.CreateUser(ctx, "Duplicate", "other_user", "unique@example.com", "hash"); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate email error = %v, want conflict", err)
	}
}

func TestDatabaseRepositoryProfilePostCursorPagination(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	authorID := createTestUser(t, client, "profile_author")
	viewerID := createTestUser(t, client, "profile_viewer")
	base := time.Date(2026, 6, 15, 11, 0, 0, 0, time.UTC)

	var want []int
	for i := 0; i < 5; i++ {
		post := insertTestPost(t, client, authorID, fmt.Sprintf("profile-%d", i), base.Add(time.Duration(i)*time.Minute))
		want = append([]int{post.ID}, want...)
	}

	var got []int
	var cursor *pagination.Cursor
	for {
		items, next, err := client.GetPosts(ctx, "user_profile_author", cursor, 2, stringID(viewerID))
		if err != nil {
			t.Fatalf("GetPosts(cursor %+v) error = %v", cursor, err)
		}
		got = append(got, postIDs(items)...)
		if next == nil {
			break
		}
		cursor = next
	}
	assertIDs(t, got, want)
	assertNoDuplicateIDs(t, got)

	if _, _, err := client.GetPosts(ctx, "missing_user", nil, 2, stringID(viewerID)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetPosts(missing user) error = %v, want not found", err)
	}
}

func TestDatabaseRepositoryFollowListCursorPagination(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	profileID := createTestUser(t, client, "follow_profile")
	viewerID := createTestUser(t, client, "follow_viewer")
	followerAID := createTestUser(t, client, "follower_a")
	followerBID := createTestUser(t, client, "follower_b")
	followerCID := createTestUser(t, client, "follower_c")
	followedAID := createTestUser(t, client, "followed_a")
	followedBID := createTestUser(t, client, "followed_b")
	followedCID := createTestUser(t, client, "followed_c")
	base := time.Date(2026, 6, 15, 11, 30, 0, 0, time.UTC)

	follows := []struct {
		follower int
		followee int
		created  time.Time
	}{
		{followerAID, profileID, base},
		{followerBID, profileID, base.Add(time.Minute)},
		{followerCID, profileID, base.Add(2 * time.Minute)},
		{profileID, followedAID, base},
		{profileID, followedBID, base.Add(time.Minute)},
		{profileID, followedCID, base.Add(2 * time.Minute)},
		{viewerID, followerBID, base.Add(3 * time.Minute)},
		{viewerID, followedBID, base.Add(3 * time.Minute)},
	}
	for _, follow := range follows {
		if _, err := client.DB().Pool().Exec(
			ctx,
			`INSERT INTO follows (follower_id, followee_id, created) VALUES ($1, $2, $3)`,
			follow.follower, follow.followee, follow.created,
		); err != nil {
			t.Fatalf("insert follow error = %v", err)
		}
	}

	var followers []users.User
	var cursor *pagination.Cursor
	for {
		items, next, err := client.ListFollowers(ctx, "user_follow_profile", cursor, 2, stringID(viewerID))
		if err != nil {
			t.Fatalf("ListFollowers(cursor %+v) error = %v", cursor, err)
		}
		followers = append(followers, items...)
		if next == nil {
			break
		}
		cursor = next
	}
	assertIDs(t, userIDs(followers), []int{followerCID, followerBID, followerAID})
	if !followers[1].IsFollowing || followers[0].IsFollowing || followers[2].IsFollowing {
		t.Fatalf("follower isFollowing flags = %#v", followers)
	}

	var following []users.User
	cursor = nil
	for {
		items, next, err := client.ListFollowing(ctx, "user_follow_profile", cursor, 2, stringID(viewerID))
		if err != nil {
			t.Fatalf("ListFollowing(cursor %+v) error = %v", cursor, err)
		}
		following = append(following, items...)
		if next == nil {
			break
		}
		cursor = next
	}
	assertIDs(t, userIDs(following), []int{followedCID, followedBID, followedAID})
	if !following[1].IsFollowing || following[0].IsFollowing || following[2].IsFollowing {
		t.Fatalf("following isFollowing flags = %#v", following)
	}

	if _, _, err := client.ListFollowers(ctx, "missing_user", nil, 2, stringID(viewerID)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("ListFollowers(missing user) error = %v, want not found", err)
	}
}

func TestDatabaseRepositoryLikedPostCursorPagination(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	authorID := createTestUser(t, client, "liked_author")
	likerID := createTestUser(t, client, "liker")
	base := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	var want []int
	for i := 0; i < 5; i++ {
		post := insertTestPost(t, client, authorID, fmt.Sprintf("liked-%d", i), base)
		if _, err := client.DB().Pool().Exec(
			ctx,
			`INSERT INTO likes (post_id, user_id, created) VALUES ($1, $2, $3)`,
			post.ID, likerID, base.Add(time.Duration(i)*time.Minute),
		); err != nil {
			t.Fatalf("insert like error = %v", err)
		}
		want = append([]int{post.ID}, want...)
	}

	var got []int
	var cursor *pagination.Cursor
	for {
		items, next, err := client.GetLikedPosts(ctx, "user_liker", cursor, 2, stringID(likerID))
		if err != nil {
			t.Fatalf("GetLikedPosts(cursor %+v) error = %v", cursor, err)
		}
		got = append(got, postIDs(items)...)
		if next == nil {
			break
		}
		cursor = next
	}
	assertIDs(t, got, want)
	assertNoDuplicateIDs(t, got)

	if _, _, err := client.GetLikedPosts(ctx, "missing_user", nil, 2, stringID(likerID)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetLikedPosts(missing user) error = %v, want not found", err)
	}
}

func TestDatabaseRepositoryCommentCursorPagination(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	ownerID := createTestUser(t, client, "comment_page_owner")
	commenterID := createTestUser(t, client, "comment_page_author")
	post := insertTestPost(t, client, ownerID, "comment-page-post", time.Now())
	created := time.Date(2026, 6, 15, 13, 0, 0, 0, time.UTC)

	var want []int
	for i := 0; i < 5; i++ {
		var id int
		err := client.DB().Pool().QueryRow(
			ctx,
			`INSERT INTO comments (post_id, user_id, body, created)
			VALUES ($1, $2, $3, $4) RETURNING id`,
			post.ID, commenterID, fmt.Sprintf("comment-%d", i), created,
		).Scan(&id)
		if err != nil {
			t.Fatalf("insert comment error = %v", err)
		}
		want = append([]int{id}, want...)
	}

	var got []int
	var cursor *pagination.Cursor
	for {
		items, next, err := client.ListComments(ctx, post.PublicID, cursor, 2)
		if err != nil {
			t.Fatalf("ListComments(cursor %+v) error = %v", cursor, err)
		}
		got = append(got, commentIDs(items)...)
		if next == nil {
			break
		}
		cursor = next
	}
	assertIDs(t, got, want)
	assertNoDuplicateIDs(t, got)

	items, terminal, err := client.ListComments(
		ctx,
		post.PublicID,
		&pagination.Cursor{Created: created, ID: int64(want[len(want)-1])},
		2,
	)
	if err != nil {
		t.Fatalf("ListComments(boundary cursor) error = %v", err)
	}
	if len(items) != 0 || terminal != nil {
		t.Fatalf("ListComments(boundary cursor) = %v, %+v; want empty terminal page", commentIDs(items), terminal)
	}
}

func TestDatabaseRepositoryConsumesUploadAndRollsBackFailedPost(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	userID := createTestUser(t, client, "upload_owner")

	if created, err := client.CreateUpload(ctx, stringID(userID), "successful-upload"); err != nil || !created {
		t.Fatalf("CreateUpload(successful) = %v, %v", created, err)
	}
	publicID, created, err := client.CreatePost(ctx, stringID(userID), "successful-upload", nil)
	if err != nil || !created || publicID == "" {
		t.Fatalf("CreatePost(successful) = %q, %v, %v", publicID, created, err)
	}

	var uploadExists bool
	if err := client.DB().Pool().QueryRow(
		ctx,
		`SELECT EXISTS (SELECT 1 FROM uploads WHERE filename = 'successful-upload')`,
	).Scan(&uploadExists); err != nil {
		t.Fatalf("successful upload query error = %v", err)
	}
	if uploadExists {
		t.Fatal("successful upload was not consumed")
	}

	if created, err := client.CreateUpload(ctx, stringID(userID), "rollback-upload"); err != nil || !created {
		t.Fatalf("CreateUpload(rollback) = %v, %v", created, err)
	}
	tooLong := strings.Repeat("x", 1001)
	if _, _, err := client.CreatePost(ctx, stringID(userID), "rollback-upload", &tooLong); err == nil {
		t.Fatal("CreatePost(invalid description) error = nil, want constraint failure")
	}
	if err := client.DB().Pool().QueryRow(
		ctx,
		`SELECT EXISTS (SELECT 1 FROM uploads WHERE filename = 'rollback-upload')`,
	).Scan(&uploadExists); err != nil || !uploadExists {
		t.Fatalf("rollback upload exists = %v, error = %v", uploadExists, err)
	}

	if publicID, created, err := client.CreatePost(ctx, stringID(userID), "missing-upload", nil); err != nil || created || publicID != "" {
		t.Fatalf("CreatePost(missing upload) = %q, %v, %v", publicID, created, err)
	}
}

func TestDatabaseRepositoryPostHashtagOutboxUsesVisiblePostCount(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	userID := createTestUser(t, client, "hashtag_outbox_owner")
	description := "first #smoke"

	if created, err := client.CreateUpload(ctx, stringID(userID), "hashtag-outbox-upload"); err != nil || !created {
		t.Fatalf("CreateUpload() = %v, %v", created, err)
	}
	if publicID, created, err := client.CreatePost(ctx, stringID(userID), "hashtag-outbox-upload", &description, "smoke"); err != nil || !created || publicID == "" {
		t.Fatalf("CreatePost() = %q, %v, %v", publicID, created, err)
	}

	var payload []byte
	if err := client.DB().Pool().QueryRow(ctx,
		`SELECT payload
		FROM outbox
		WHERE topic = 'entity-changes'
		AND payload->>'table' = 'hashtags'
		AND payload->>'name' = 'smoke'
		ORDER BY id DESC LIMIT 1`,
	).Scan(&payload); err != nil {
		t.Fatalf("hashtag outbox payload query error = %v", err)
	}

	var decoded database.EntityHashtagUpsertPayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.PostCount != 1 {
		t.Fatalf("hashtag post_count = %d, want 1; payload = %s", decoded.PostCount, payload)
	}
}

func TestDatabaseRepositoryProfileAvatarOwnershipAndUnusedAvatar(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	userID := createTestUser(t, client, "profile_owner")
	otherID := createTestUser(t, client, "profile_other")

	if created, err := client.CreateUpload(ctx, stringID(otherID), "foreign-avatar"); err != nil || !created {
		t.Fatalf("CreateUpload(foreign) = %v, %v", created, err)
	}
	result, err := client.UpdateUser(
		ctx, stringID(userID), "Owner", "user_profile_owner",
		"profile_owner@example.com", "foreign-avatar", nil,
	)
	if err != nil {
		t.Fatalf("UpdateUser(foreign avatar) error = %v", err)
	}
	if result.Updated {
		t.Fatalf("UpdateUser(foreign avatar) = %+v, want not updated", result)
	}

	for _, avatar := range []string{"first-avatar", "second-avatar"} {
		if created, err := client.CreateUpload(ctx, stringID(userID), avatar); err != nil || !created {
			t.Fatalf("CreateUpload(%q) = %v, %v", avatar, created, err)
		}
	}
	result, err = client.UpdateUser(
		ctx, stringID(userID), "Owner", "user_profile_owner",
		"profile_owner@example.com", "first-avatar", nil,
	)
	if err != nil || !result.Updated || result.UnusedAvatar != "" {
		t.Fatalf("UpdateUser(first avatar) = %+v, %v", result, err)
	}
	result, err = client.UpdateUser(
		ctx, stringID(userID), "Owner", "user_profile_owner",
		"profile_owner@example.com", "second-avatar", nil,
	)
	if err != nil {
		t.Fatalf("UpdateUser(second avatar) error = %v", err)
	}
	if !result.Updated || result.UnusedAvatar != "first-avatar" {
		t.Fatalf("UpdateUser(second avatar) = %+v, want unused first-avatar", result)
	}

	var remaining int
	if err := client.DB().Pool().QueryRow(
		ctx,
		`SELECT count(*) FROM uploads WHERE user_id = $1 AND filename IN ('first-avatar', 'second-avatar')`,
		userID,
	).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("owned avatar uploads remaining = %d, error = %v", remaining, err)
	}
}

func TestDatabaseRepositoryFollowIsIdempotentAndRejectsMissingTarget(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	followerID := createTestUser(t, client, "follower")
	followeeID := createTestUser(t, client, "followee")

	if err := client.FollowUser(ctx, stringID(followerID), "999999"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("FollowUser(missing target) error = %v, want not found", err)
	}
	for i := 0; i < 2; i++ {
		if err := client.FollowUser(ctx, stringID(followerID), stringID(followeeID)); err != nil {
			t.Fatalf("FollowUser(attempt %d) error = %v", i+1, err)
		}
	}
	var count int
	if err := client.DB().Pool().QueryRow(
		ctx,
		`SELECT count(*) FROM follows WHERE follower_id = $1 AND followee_id = $2`,
		followerID, followeeID,
	).Scan(&count); err != nil || count != 1 {
		t.Fatalf("follow count = %d, error = %v", count, err)
	}
}

func TestDatabaseRepositoryCommentOwnershipAndNotFoundOutcomes(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	postOwnerID := createTestUser(t, client, "comment_post_owner")
	commentOwnerID := createTestUser(t, client, "comment_owner")
	otherID := createTestUser(t, client, "comment_other")
	post := insertTestPost(t, client, postOwnerID, "comment-workflow", time.Now())

	comment, err := client.CreateComment(ctx, post.PublicID, stringID(commentOwnerID), "hello")
	if err != nil {
		t.Fatalf("CreateComment() error = %v", err)
	}
	if comment.UserID != commentOwnerID || comment.Body != "hello" {
		t.Fatalf("CreateComment() = %+v", comment)
	}
	if _, err := client.CreateComment(ctx, "00000000-0000-0000-0000-000000000000", stringID(commentOwnerID), "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("CreateComment(missing post) error = %v, want not found", err)
	}

	deleted, err := client.DeleteComment(ctx, post.PublicID, comment.PublicID, stringID(otherID))
	if !deleted || !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("DeleteComment(non-owner) = %v, %v; want true, forbidden", deleted, err)
	}
	deleted, err = client.DeleteComment(ctx, post.PublicID, comment.PublicID, stringID(commentOwnerID))
	if err != nil || !deleted {
		t.Fatalf("DeleteComment(owner) = %v, %v", deleted, err)
	}
	deleted, err = client.DeleteComment(ctx, post.PublicID, comment.PublicID, stringID(commentOwnerID))
	if err != nil || deleted {
		t.Fatalf("DeleteComment(missing) = %v, %v; want false, nil", deleted, err)
	}
	deleted, err = client.DeleteComment(ctx, "00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000", stringID(commentOwnerID))
	if err != nil || deleted {
		t.Fatalf("DeleteComment(missing post) = %v, %v; want false, nil", deleted, err)
	}
}

func TestDatabaseRepositoryPostOwnerCanDeleteComments(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	postOwnerID := createTestUser(t, client, "mod_post_owner")
	commenterID := createTestUser(t, client, "mod_commenter")
	post := insertTestPost(t, client, postOwnerID, "moderated-post", time.Now())

	comment, err := client.CreateComment(ctx, post.PublicID, stringID(commenterID), "to be moderated")
	if err != nil {
		t.Fatalf("CreateComment() error = %v", err)
	}

	deleted, err := client.DeleteComment(ctx, post.PublicID, comment.PublicID, stringID(postOwnerID))
	if err != nil || !deleted {
		t.Fatalf("DeleteComment(post owner) = %v, %v; want true, nil", deleted, err)
	}
	var remaining int
	if err := client.DB().Pool().QueryRow(ctx,
		`SELECT count(*) FROM comments WHERE id = $1`, comment.ID).Scan(&remaining); err != nil {
		t.Fatalf("comment count query error = %v", err)
	}
	if remaining != 0 {
		t.Fatalf("comment still exists after post owner deletion")
	}
}

func TestDatabaseRepositorySelfLikeIsNoop(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	ownerID := createTestUser(t, client, "self_like_owner")
	post := insertTestPost(t, client, ownerID, "self-like-post", time.Now())

	if err := client.LikePost(ctx, post.PublicID, stringID(ownerID)); err != nil {
		t.Fatalf("LikePost(self) error = %v, want nil", err)
	}
	var count int
	if err := client.DB().Pool().QueryRow(ctx,
		`SELECT count(*) FROM likes WHERE post_id = $1 AND user_id = $2`,
		post.ID, ownerID).Scan(&count); err != nil {
		t.Fatalf("like count query error = %v", err)
	}
	if count != 0 {
		t.Fatalf("self-like inserted %d like rows, want 0", count)
	}
}

func TestDatabaseRepositorySessionHashExpirationAndConditionalRefresh(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	userID := createTestUser(t, client, "session_user")

	freshToken := "fresh-session-token"
	freshExpiry := time.Now().Add(6 * 24 * time.Hour).Truncate(time.Microsecond)
	if _, err := client.CreateSession(ctx, freshToken, userID, freshExpiry); err != nil {
		t.Fatalf("CreateSession(fresh) error = %v", err)
	}
	hashedFresh := auth.HashSessionToken(freshToken, testSessionSecret)
	var persistedID string
	if err := client.DB().Pool().QueryRow(
		ctx, `SELECT id FROM sessions WHERE user_id = $1`, userID,
	).Scan(&persistedID); err != nil {
		t.Fatalf("fresh session query error = %v", err)
	}
	if persistedID != hashedFresh || persistedID == freshToken {
		t.Fatalf("persisted session ID = %q, want hash %q", persistedID, hashedFresh)
	}

	session, err := client.RefreshSession(ctx, freshToken)
	if err != nil || session.UserID != stringID(userID) || session.ID != hashedFresh {
		t.Fatalf("RefreshSession(fresh) = %+v, %v", session, err)
	}
	var unchangedExpiry time.Time
	if err := client.DB().Pool().QueryRow(
		ctx, `SELECT expires_at FROM sessions WHERE id = $1`, hashedFresh,
	).Scan(&unchangedExpiry); err != nil {
		t.Fatalf("fresh expiry query error = %v", err)
	}
	if !unchangedExpiry.Equal(freshExpiry) {
		t.Fatalf("fresh expiry = %v, want unchanged %v", unchangedExpiry, freshExpiry)
	}

	staleToken := "stale-session-token"
	staleExpiry := time.Now().Add(24 * time.Hour)
	if _, err := client.CreateSession(ctx, staleToken, userID, staleExpiry); err != nil {
		t.Fatalf("CreateSession(stale) error = %v", err)
	}
	beforeRefresh := time.Now()
	session, err = client.RefreshSession(ctx, staleToken)
	if err != nil || session.UserID != stringID(userID) {
		t.Fatalf("RefreshSession(stale) = %+v, %v", session, err)
	}
	var refreshedExpiry time.Time
	if err := client.DB().Pool().QueryRow(
		ctx, `SELECT expires_at FROM sessions WHERE id = $1`,
		auth.HashSessionToken(staleToken, testSessionSecret),
	).Scan(&refreshedExpiry); err != nil {
		t.Fatalf("stale expiry query error = %v", err)
	}
	if !refreshedExpiry.After(beforeRefresh.Add(6 * 24 * time.Hour)) {
		t.Fatalf("stale expiry = %v, want approximately seven days", refreshedExpiry)
	}

	expiredToken := "expired-session-token"
	if _, err := client.CreateSession(ctx, expiredToken, userID, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("CreateSession(expired) error = %v", err)
	}
	session, err = client.RefreshSession(ctx, expiredToken)
	if err != nil || session.UserID != "" || session.ID != "" {
		t.Fatalf("RefreshSession(expired) = %+v, %v; want zero session", session, err)
	}

	t.Setenv("SESSION_ABSOLUTE_TTL_HOURS", "1")
	absoluteExpiredToken := "absolute-expired-session-token"
	absoluteExpiry := time.Now().Add(24 * time.Hour).Truncate(time.Microsecond)
	if _, err := client.CreateSession(ctx, absoluteExpiredToken, userID, absoluteExpiry); err != nil {
		t.Fatalf("CreateSession(absolute expired) error = %v", err)
	}
	hashedAbsoluteExpired := auth.HashSessionToken(absoluteExpiredToken, testSessionSecret)
	if _, err := client.DB().Pool().Exec(
		ctx,
		`UPDATE sessions SET created = now() - interval '2 hours' WHERE id = $1`,
		hashedAbsoluteExpired,
	); err != nil {
		t.Fatalf("age absolute-expired session error = %v", err)
	}
	session, err = client.RefreshSession(ctx, absoluteExpiredToken)
	if err != nil || session.UserID != "" || session.ID != "" {
		t.Fatalf("RefreshSession(absolute expired) = %+v, %v; want zero session", session, err)
	}
	var unchangedAbsoluteExpiry time.Time
	if err := client.DB().Pool().QueryRow(
		ctx, `SELECT expires_at FROM sessions WHERE id = $1`, hashedAbsoluteExpired,
	).Scan(&unchangedAbsoluteExpiry); err != nil {
		t.Fatalf("absolute-expired expiry query error = %v", err)
	}
	if !unchangedAbsoluteExpiry.Equal(absoluteExpiry) {
		t.Fatalf("absolute-expired expiry = %v, want unchanged %v", unchangedAbsoluteExpiry, absoluteExpiry)
	}
	if err := client.DeleteExpiredSessions(ctx); err != nil {
		t.Fatalf("DeleteExpiredSessions() error = %v", err)
	}
	var remaining int
	if err := client.DB().Pool().QueryRow(
		ctx, `SELECT count(*) FROM sessions WHERE id = $1`, hashedAbsoluteExpired,
	).Scan(&remaining); err != nil {
		t.Fatalf("absolute-expired session count error = %v", err)
	}
	if remaining != 0 {
		t.Fatalf("absolute-expired session count = %d, want 0", remaining)
	}
}

func TestDatabaseRepositoryListsAndRevokesActiveSessions(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	userID := createTestUser(t, client, "session_list_owner")
	otherUserID := createTestUser(t, client, "session_list_other")
	now := time.Now().UTC().Truncate(time.Microsecond)

	type seededSession struct {
		token    string
		publicID string
		created  time.Time
		expires  time.Time
		userID   int
	}
	seeded := []seededSession{
		{
			token:    "current-token",
			publicID: "10000000-0000-0000-0000-000000000002",
			created:  now.Add(-time.Hour),
			expires:  now.Add(24 * time.Hour),
			userID:   userID,
		},
		{
			token:    "remote-newer-public-id",
			publicID: "10000000-0000-0000-0000-000000000003",
			created:  now.Add(-2 * time.Hour),
			expires:  now.Add(24 * time.Hour),
			userID:   userID,
		},
		{
			token:    "remote-older-public-id",
			publicID: "10000000-0000-0000-0000-000000000001",
			created:  now.Add(-2 * time.Hour),
			expires:  now.Add(24 * time.Hour),
			userID:   userID,
		},
		{
			token:    "sliding-expired",
			publicID: "20000000-0000-0000-0000-000000000001",
			created:  now.Add(-3 * time.Hour),
			expires:  now.Add(-time.Minute),
			userID:   userID,
		},
		{
			token:    "absolute-expired",
			publicID: "20000000-0000-0000-0000-000000000002",
			created:  now.Add(-31 * 24 * time.Hour),
			expires:  now.Add(24 * time.Hour),
			userID:   userID,
		},
		{
			token:    "other-user",
			publicID: "30000000-0000-0000-0000-000000000001",
			created:  now,
			expires:  now.Add(24 * time.Hour),
			userID:   otherUserID,
		},
	}
	for _, session := range seeded {
		if _, err := client.CreateSession(ctx, session.token, session.userID, session.expires); err != nil {
			t.Fatalf("CreateSession(%q) error = %v", session.token, err)
		}
		if _, err := client.DB().Pool().Exec(
			ctx,
			`UPDATE sessions SET public_id = $1, created = $2 WHERE id = $3`,
			session.publicID,
			session.created,
			auth.HashSessionToken(session.token, testSessionSecret),
		); err != nil {
			t.Fatalf("seed session %q error = %v", session.token, err)
		}
	}

	items, err := client.ListActiveSessions(ctx, stringID(userID), "current-token")
	if err != nil {
		t.Fatalf("ListActiveSessions() error = %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("ListActiveSessions() = %+v, want 3 items", items)
	}
	wantIDs := []string{
		"10000000-0000-0000-0000-000000000002",
		"10000000-0000-0000-0000-000000000003",
		"10000000-0000-0000-0000-000000000001",
	}
	for i, wantID := range wantIDs {
		if items[i].ID != wantID {
			t.Fatalf("session[%d].ID = %q, want %q", i, items[i].ID, wantID)
		}
		if items[i].Current != (i == 0) {
			t.Fatalf("session[%d].Current = %v, want %v", i, items[i].Current, i == 0)
		}
	}
	if !items[0].Created.Equal(seeded[0].created) || !items[0].ExpiresAt.Equal(seeded[0].expires) {
		t.Fatalf("current session timestamps = %v, %v; want %v, %v",
			items[0].Created, items[0].ExpiresAt, seeded[0].created, seeded[0].expires)
	}

	outcome, err := client.DeleteSessionByID(
		ctx,
		"30000000-0000-0000-0000-000000000001",
		stringID(userID),
		"current-token",
	)
	if err != nil || outcome != sessions.DeleteSessionNotFound {
		t.Fatalf("DeleteSessionByID(other user) = %v, %v", outcome, err)
	}
	var otherUserSessionCount int
	if err := client.DB().Pool().QueryRow(
		ctx,
		`SELECT count(*) FROM sessions WHERE public_id = $1 AND user_id = $2`,
		"30000000-0000-0000-0000-000000000001",
		otherUserID,
	).Scan(&otherUserSessionCount); err != nil || otherUserSessionCount != 1 {
		t.Fatalf("other user's session count = %d, error = %v; want 1", otherUserSessionCount, err)
	}

	outcome, err = client.DeleteSessionByID(
		ctx,
		"10000000-0000-0000-0000-000000000002",
		stringID(userID),
		"current-token",
	)
	if err != nil || outcome != sessions.DeleteSessionCurrent {
		t.Fatalf("DeleteSessionByID(current) = %v, %v", outcome, err)
	}
	var currentSessionCount int
	if err := client.DB().Pool().QueryRow(
		ctx,
		`SELECT count(*) FROM sessions WHERE public_id = $1 AND id = $2`,
		"10000000-0000-0000-0000-000000000002",
		auth.HashSessionToken("current-token", testSessionSecret),
	).Scan(&currentSessionCount); err != nil || currentSessionCount != 1 {
		t.Fatalf("current session count = %d, error = %v; want 1", currentSessionCount, err)
	}

	outcome, err = client.DeleteSessionByID(
		ctx,
		"10000000-0000-0000-0000-000000000003",
		stringID(userID),
		"current-token",
	)
	if err != nil || outcome != sessions.DeleteSessionDeleted {
		t.Fatalf("DeleteSessionByID(remote) = %v, %v", outcome, err)
	}
	var remaining int
	if err := client.DB().Pool().QueryRow(
		ctx,
		`SELECT count(*) FROM sessions WHERE public_id = $1`,
		"10000000-0000-0000-0000-000000000003",
	).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("deleted session count = %d, error = %v", remaining, err)
	}
}

func TestDatabaseRepositoryConcurrentSessionRevocationHasOneWinner(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	userID := createTestUser(t, client, "session_revoke_concurrent")
	const (
		token    = "remote-session-token"
		publicID = "40000000-0000-0000-0000-000000000001"
	)
	if _, err := client.CreateSession(ctx, token, userID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if _, err := client.DB().Pool().Exec(
		ctx,
		`UPDATE sessions SET public_id = $1 WHERE id = $2`,
		publicID,
		auth.HashSessionToken(token, testSessionSecret),
	); err != nil {
		t.Fatalf("set public ID error = %v", err)
	}

	const callers = 16
	start := make(chan struct{})
	outcomes := make(chan sessions.DeleteSessionOutcome, callers)
	errs := make(chan error, callers)
	var group sync.WaitGroup
	for range callers {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			outcome, err := client.DeleteSessionByID(
				ctx,
				publicID,
				stringID(userID),
				"different-current-token",
			)
			outcomes <- outcome
			errs <- err
		}()
	}
	close(start)
	group.Wait()
	close(outcomes)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("DeleteSessionByID() error = %v", err)
		}
	}
	deleted, notFound := 0, 0
	for outcome := range outcomes {
		switch outcome {
		case sessions.DeleteSessionDeleted:
			deleted++
		case sessions.DeleteSessionNotFound:
			notFound++
		case sessions.DeleteSessionCurrent:
			t.Fatal("remote session was misclassified as current during concurrent revocation")
		default:
			t.Fatalf("unexpected deletion outcome %v", outcome)
		}
	}
	if deleted != 1 || notFound != callers-1 {
		t.Fatalf("outcomes = %d deleted, %d not found; want 1, %d", deleted, notFound, callers-1)
	}
}

func TestDatabaseRepositoryConcurrentRevocationPreservesCurrentSession(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	userID := createTestUser(t, client, "session_current_race")
	const (
		token    = "current-session-token"
		publicID = "50000000-0000-0000-0000-000000000001"
		callers  = 16
	)
	if _, err := client.CreateSession(ctx, token, userID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if _, err := client.DB().Pool().Exec(
		ctx,
		`UPDATE sessions SET public_id = $1 WHERE id = $2`,
		publicID,
		auth.HashSessionToken(token, testSessionSecret),
	); err != nil {
		t.Fatalf("set public ID error = %v", err)
	}

	start := make(chan struct{})
	outcomes := make(chan sessions.DeleteSessionOutcome, callers)
	errs := make(chan error, callers)
	var group sync.WaitGroup
	for range callers {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			outcome, err := client.DeleteSessionByID(ctx, publicID, stringID(userID), token)
			outcomes <- outcome
			errs <- err
		}()
	}
	close(start)
	group.Wait()
	close(outcomes)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("DeleteSessionByID() error = %v", err)
		}
	}
	for outcome := range outcomes {
		if outcome != sessions.DeleteSessionCurrent {
			t.Fatalf("DeleteSessionByID() outcome = %v, want current", outcome)
		}
	}
	var remaining int
	if err := client.DB().Pool().QueryRow(
		ctx,
		`SELECT count(*) FROM sessions WHERE public_id = $1 AND id = $2`,
		publicID,
		auth.HashSessionToken(token, testSessionSecret),
	).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatalf("current session count = %d, error = %v; want 1", remaining, err)
	}
}

func TestDatabaseSessionPublicIDGeneratedAndUnique(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	userID := createTestUser(t, client, "session_public_id")

	for _, token := range []string{"first-token", "second-token"} {
		if _, err := client.CreateSession(ctx, token, userID, time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("CreateSession(%q) error = %v", token, err)
		}
	}

	var count, distinct int
	if err := client.DB().Pool().QueryRow(
		ctx,
		`SELECT count(public_id), count(DISTINCT public_id) FROM sessions WHERE user_id = $1`,
		userID,
	).Scan(&count, &distinct); err != nil {
		t.Fatalf("public ID query error = %v", err)
	}
	if count != 2 || distinct != 2 {
		t.Fatalf("public IDs = count %d, distinct %d; want 2, 2", count, distinct)
	}

	var firstPublicID string
	if err := client.DB().Pool().QueryRow(
		ctx,
		`SELECT public_id FROM sessions WHERE id = $1`,
		auth.HashSessionToken("first-token", testSessionSecret),
	).Scan(&firstPublicID); err != nil {
		t.Fatalf("first public ID query error = %v", err)
	}
	if _, err := client.DB().Pool().Exec(
		ctx,
		`UPDATE sessions SET public_id = $1 WHERE id = $2`,
		firstPublicID,
		auth.HashSessionToken("second-token", testSessionSecret),
	); err == nil {
		t.Fatal("duplicate public ID update succeeded, want unique constraint violation")
	}
}

func TestDatabaseRepositoryCapsSessionsPerUser(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	userID := createTestUser(t, client, "session_limit")

	for i := 0; i < maxSessionsPerUser+1; i++ {
		token := fmt.Sprintf("bounded-session-%03d", i)
		if _, err := client.CreateSession(ctx, token, userID, time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("CreateSession(%d) error = %v", i, err)
		}
	}

	var count int
	if err := client.DB().Pool().QueryRow(
		ctx,
		`SELECT count(*) FROM sessions WHERE user_id = $1`,
		userID,
	).Scan(&count); err != nil {
		t.Fatalf("session count query error = %v", err)
	}
	if count != maxSessionsPerUser {
		t.Fatalf("session count = %d, want %d", count, maxSessionsPerUser)
	}
}

func TestDatabaseRepositoryConcurrentSessionCreationRetainsIssuedSessions(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	userID := createTestUser(t, client, "session_concurrent_limit")

	if _, err := client.DB().Pool().Exec(
		ctx,
		`INSERT INTO sessions (id, user_id, created, expires_at)
		SELECT 'existing-' || value, $1,
		  clock_timestamp() - (value * interval '1 minute'),
		  clock_timestamp() + interval '1 day'
		FROM generate_series(1, 99) AS series(value)`,
		userID,
	); err != nil {
		t.Fatalf("seed existing sessions error = %v", err)
	}

	const callers = 8
	start := make(chan struct{})
	errs := make(chan error, callers)
	var group sync.WaitGroup
	for i := 0; i < callers; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			token := fmt.Sprintf("concurrent-issued-%d", index)
			_, err := client.CreateSession(ctx, token, userID, time.Now().Add(time.Hour))
			errs <- err
		}(i)
	}
	close(start)
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent CreateSession() error = %v", err)
		}
	}

	var count int
	if err := client.DB().Pool().QueryRow(
		ctx,
		`SELECT count(*) FROM sessions WHERE user_id = $1`,
		userID,
	).Scan(&count); err != nil {
		t.Fatalf("session count query error = %v", err)
	}
	if count != maxSessionsPerUser {
		t.Fatalf("session count = %d, want %d", count, maxSessionsPerUser)
	}
	for i := 0; i < callers; i++ {
		token := fmt.Sprintf("concurrent-issued-%d", i)
		var exists bool
		if err := client.DB().Pool().QueryRow(
			ctx,
			`SELECT EXISTS (SELECT 1 FROM sessions WHERE id = $1)`,
			auth.HashSessionToken(token, testSessionSecret),
		).Scan(&exists); err != nil {
			t.Fatalf("issued session %d query error = %v", i, err)
		}
		if !exists {
			t.Fatalf("issued session %d was pruned", i)
		}
	}
}

func TestDatabaseRepositorySessionListHasHardLimit(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	userID := createTestUser(t, client, "session_list_limit")

	if _, err := client.DB().Pool().Exec(
		ctx,
		`INSERT INTO sessions (id, user_id, created, expires_at)
		SELECT 'direct-' || value, $1,
		  clock_timestamp() - (value * interval '1 minute'),
		  clock_timestamp() + interval '1 day'
		FROM generate_series(1, 101) AS series(value)`,
		userID,
	); err != nil {
		t.Fatalf("seed direct sessions error = %v", err)
	}

	items, err := client.ListActiveSessions(ctx, stringID(userID), "not-a-seeded-token")
	if err != nil {
		t.Fatalf("ListActiveSessions() error = %v", err)
	}
	if len(items) != maxSessionsPerUser {
		t.Fatalf("listed sessions = %d, want %d", len(items), maxSessionsPerUser)
	}
}

func TestDatabaseRepositoryListsEffectiveAbsoluteExpiry(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	userID := createTestUser(t, client, "session_effective_expiry")
	const token = "near-absolute-expiry"

	if _, err := client.CreateSession(ctx, token, userID, time.Now().Add(sessionTTL)); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	created := time.Now().UTC().Add(-29 * 24 * time.Hour).Truncate(time.Microsecond)
	if _, err := client.DB().Pool().Exec(
		ctx,
		`UPDATE sessions SET created = $1, expires_at = $2 WHERE id = $3`,
		created,
		time.Now().UTC().Add(sessionTTL),
		auth.HashSessionToken(token, testSessionSecret),
	); err != nil {
		t.Fatalf("set near-absolute expiry error = %v", err)
	}

	items, err := client.ListActiveSessions(ctx, stringID(userID), token)
	if err != nil {
		t.Fatalf("ListActiveSessions() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListActiveSessions() = %+v, want one session", items)
	}
	wantExpiry := created.Add(defaultSessionAbsoluteTTLHours * time.Hour)
	if !items[0].ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("effective expiry = %v, want %v", items[0].ExpiresAt, wantExpiry)
	}
}

func TestSearchRepositoryTypeaheadResultsAndLimit(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	repository := search.NewSearchRepository(client.Client)

	for i := 0; i < 10; i++ {
		userID := createTestUser(t, client, fmt.Sprintf("typeahead_%02d", i))
		if _, err := client.DB().Pool().Exec(
			ctx,
			`UPDATE users SET avatar = $1 WHERE id = $2`,
			fmt.Sprintf("avatar-%02d", i),
			userID,
		); err != nil {
			t.Fatalf("set avatar error = %v", err)
		}
		if _, err := client.DB().Pool().Exec(
			ctx,
			`INSERT INTO hashtags (name) VALUES ($1)`,
			fmt.Sprintf("typeahead_%02d", i),
		); err != nil {
			t.Fatalf("insert hashtag error = %v", err)
		}
	}

	userResults, err := repository.SearchUsers(ctx, "typeahead")
	if err != nil {
		t.Fatalf("SearchUsers() error = %v", err)
	}
	if len(userResults) != 8 {
		t.Fatalf("SearchUsers() result count = %d, want 8", len(userResults))
	}
	for _, result := range userResults {
		if result.Username == "" || result.Avatar == nil {
			t.Fatalf("SearchUsers() result = %+v, want username and avatar", result)
		}
	}

	hashtagResults, err := repository.SearchHashtags(ctx, "typeahead")
	if err != nil {
		t.Fatalf("SearchHashtags() error = %v", err)
	}
	if len(hashtagResults) != 8 {
		t.Fatalf("SearchHashtags() result count = %d, want 8", len(hashtagResults))
	}
	for _, result := range hashtagResults {
		if result.Name == "" || result.PostCount != 0 {
			t.Fatalf("SearchHashtags() result = %+v, want name and zero post count", result)
		}
	}
}

func TestDatabaseRepositoryChangePasswordRemovesOtherSessions(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	userID := createTestUser(t, client, "password_user")
	currentToken := "current-session-token"
	otherToken := "other-session-token"

	for _, token := range []string{currentToken, otherToken} {
		if _, err := client.CreateSession(ctx, token, userID, time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("CreateSession(%q) error = %v", token, err)
		}
	}
	if err := client.ChangePassword(ctx, stringID(userID), "new-password-hash", currentToken); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}

	var password string
	if err := client.DB().Pool().QueryRow(
		ctx, `SELECT password FROM users WHERE id = $1`, userID,
	).Scan(&password); err != nil {
		t.Fatalf("password query error = %v", err)
	}
	if password != "new-password-hash" {
		t.Fatalf("password = %q, want new-password-hash", password)
	}
	var sessionIDs []string
	rows, err := client.DB().Pool().Query(ctx, `SELECT id FROM sessions ORDER BY id`)
	if err != nil {
		t.Fatalf("session query error = %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("session scan error = %v", err)
		}
		sessionIDs = append(sessionIDs, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("session rows error = %v", err)
	}
	want := []string{auth.HashSessionToken(currentToken, testSessionSecret)}
	if !reflect.DeepEqual(sessionIDs, want) {
		t.Fatalf("sessions = %v, want %v", sessionIDs, want)
	}
}

func TestDatabaseRepositoryChangePasswordRollsBackWhenSessionRemovalFails(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	userID := createTestUser(t, client, "password_rollback")
	currentToken := "rollback-current-token"
	otherToken := "rollback-other-token"
	for _, token := range []string{currentToken, otherToken} {
		if _, err := client.CreateSession(ctx, token, userID, time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("CreateSession(%q) error = %v", token, err)
		}
	}

	_, err := client.DB().Pool().Exec(ctx, `
		CREATE FUNCTION fail_session_delete() RETURNS trigger
		LANGUAGE plpgsql AS $$
		BEGIN
		  RAISE EXCEPTION 'forced session delete failure';
		END;
		$$;
		CREATE TRIGGER fail_session_delete
		BEFORE DELETE ON sessions
		FOR EACH STATEMENT EXECUTE FUNCTION fail_session_delete();
	`)
	if err != nil {
		t.Fatalf("create failure trigger error = %v", err)
	}
	t.Cleanup(func() {
		_, _ = client.DB().Pool().Exec(
			context.Background(),
			`DROP TRIGGER IF EXISTS fail_session_delete ON sessions;
			DROP FUNCTION IF EXISTS fail_session_delete();`,
		)
	})

	if err := client.ChangePassword(ctx, stringID(userID), "should-roll-back", currentToken); err == nil {
		t.Fatal("ChangePassword() error = nil, want forced delete failure")
	}
	var password string
	if err := client.DB().Pool().QueryRow(
		ctx, `SELECT password FROM users WHERE id = $1`, userID,
	).Scan(&password); err != nil {
		t.Fatalf("password query error = %v", err)
	}
	if password != "password-hash" {
		t.Fatalf("password = %q, want original password-hash", password)
	}
	var sessionCount int
	if err := client.DB().Pool().QueryRow(
		ctx, `SELECT count(*) FROM sessions WHERE user_id = $1`, userID,
	).Scan(&sessionCount); err != nil || sessionCount != 2 {
		t.Fatalf("session count = %d, error = %v; want 2", sessionCount, err)
	}
}

func TestDatabaseRepositoryDeletePostCascadesLikesCommentsAndClearsAvatar(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	ownerID := createTestUser(t, client, "delete_owner")
	otherID := createTestUser(t, client, "delete_other")
	post := insertTestPost(t, client, ownerID, "shared-post-file", time.Now())

	if _, err := client.DB().Pool().Exec(
		ctx, `UPDATE users SET avatar = 'shared-post-file' WHERE id = $1`, ownerID,
	); err != nil {
		t.Fatalf("set avatar error = %v", err)
	}
	if err := client.LikePost(ctx, post.PublicID, stringID(otherID)); err != nil {
		t.Fatalf("LikePost() error = %v", err)
	}
	if _, err := client.CreateComment(ctx, post.PublicID, stringID(otherID), "delete me"); err != nil {
		t.Fatalf("CreateComment() error = %v", err)
	}

	filename, err := client.DeletePost(ctx, post.PublicID, stringID(ownerID))
	if err != nil || filename != "shared-post-file" {
		t.Fatalf("DeletePost() = %q, %v", filename, err)
	}
	var posts, likes, comments int
	if err := client.DB().Pool().QueryRow(
		ctx,
		`SELECT
		  (SELECT count(*) FROM posts WHERE id = $1),
		  (SELECT count(*) FROM likes WHERE post_id = $1),
		  (SELECT count(*) FROM comments WHERE post_id = $1)`,
		post.ID,
	).Scan(&posts, &likes, &comments); err != nil {
		t.Fatalf("cascade query error = %v", err)
	}
	if posts != 0 || likes != 0 || comments != 0 {
		t.Fatalf("remaining post data = posts %d, likes %d, comments %d", posts, likes, comments)
	}
	var avatar string
	if err := client.DB().Pool().QueryRow(
		ctx, `SELECT avatar FROM users WHERE id = $1`, ownerID,
	).Scan(&avatar); err != nil {
		t.Fatalf("avatar query error = %v", err)
	}
	if avatar != "" {
		t.Fatalf("avatar = %q, want cleared", avatar)
	}
}

// The following forwarding methods preserve the integration test API.
// They were extracted from client.go to dismantle the monolith.
func (c *testClient) RefreshSession(ctx context.Context, sessionID string) (httpx.Session, error) {
	return sessions.NewSessionRepository(c.Client).RefreshSession(ctx, sessionID)
}

// The forwarding methods below preserve the package's integration-test API.
// Application code uses the focused repositories directly.
func (c *testClient) CreateUser(ctx context.Context, name, username, email, passwordHash string) (int, error) {
	return users.NewUserRepository(c.Client).CreateUser(ctx, name, username, email, passwordHash)
}

func (c *testClient) GetUserWithEmail(ctx context.Context, email string) (sessions.UserCredentials, bool, error) {
	credentials, err := sessions.NewSessionRepository(c.Client).FindLoginCredentialsByEmail(ctx, email)
	if err != nil || credentials == nil {
		return sessions.UserCredentials{}, false, err
	}
	return *credentials, true, nil
}

func (c *testClient) GetUserWithID(ctx context.Context, userID string) (users.UserCredentials, bool, error) {
	return users.NewUserRepository(c.Client).GetUserWithID(ctx, userID)
}

func (c *testClient) GetUserByUsername(ctx context.Context, username, currentUserID string) (users.User, bool, error) {
	return users.NewUserRepository(c.Client).GetUserByUsername(ctx, username, currentUserID)
}

func (c *testClient) GetUserByID(ctx context.Context, userID, currentUserID string) (users.User, bool, error) {
	return users.NewUserRepository(c.Client).GetUserByID(ctx, userID, currentUserID)
}

func (c *testClient) ListFollowers(ctx context.Context, username string, cursor *pagination.Cursor, limit int, currentUserID string) ([]users.User, *pagination.Cursor, error) {
	return users.NewUserRepository(c.Client).ListFollowers(ctx, username, cursor, limit, currentUserID)
}

func (c *testClient) ListFollowing(ctx context.Context, username string, cursor *pagination.Cursor, limit int, currentUserID string) ([]users.User, *pagination.Cursor, error) {
	return users.NewUserRepository(c.Client).ListFollowing(ctx, username, cursor, limit, currentUserID)
}

func (c *testClient) FollowUser(ctx context.Context, followerID, followeeID string) error {
	return users.NewUserRepository(c.Client).FollowUser(ctx, followerID, followeeID)
}

func (c *testClient) UnfollowUser(ctx context.Context, followerID, followeeID string) error {
	return users.NewUserRepository(c.Client).UnfollowUser(ctx, followerID, followeeID)
}

func (c *testClient) UpdateUser(ctx context.Context, userID, name, username, email, avatar string, bio *string) (users.UpdateUserResult, error) {
	return users.NewUserRepository(c.Client).UpdateUser(ctx, userID, name, username, email, avatar, bio)
}

func (c *testClient) ChangePassword(ctx context.Context, userID, passwordHash, currentSessionID string) error {
	return users.NewUserRepository(c.Client).ChangePassword(ctx, userID, passwordHash, currentSessionID)
}

func (c *testClient) CreateUpload(ctx context.Context, userID, filename string) (bool, error) {
	created, _, err := uploads.NewUploadRepository(c.Client).CreateUpload(ctx, userID, filename)
	return created, err
}

func (c *testClient) CreatePost(ctx context.Context, userID, filename string, description *string, tags ...string) (string, bool, error) {
	return posts.NewPostRepository(c.Client).CreatePost(ctx, userID, filename, description, tags)
}

func (c *testClient) GetPosts(ctx context.Context, username string, cursor *pagination.Cursor, limit int, currentUserID string) ([]posts.Post, *pagination.Cursor, error) {
	return posts.NewPostRepository(c.Client).GetPosts(ctx, username, cursor, limit, currentUserID)
}

func (c *testClient) GetLikedPosts(ctx context.Context, username string, cursor *pagination.Cursor, limit int, currentUserID string) ([]posts.Post, *pagination.Cursor, error) {
	return posts.NewPostRepository(c.Client).GetLikedPosts(ctx, username, cursor, limit, currentUserID)
}

func (c *testClient) GetPost(ctx context.Context, postID, currentUserID string) (posts.Post, bool, error) {
	return posts.NewPostRepository(c.Client).GetPost(ctx, postID, currentUserID)
}

func (c *testClient) DeletePost(ctx context.Context, postID, userID string) (string, error) {
	return posts.NewPostRepository(c.Client).DeletePost(ctx, postID, userID)
}

func (c *testClient) LikePost(ctx context.Context, postID, userID string) error {
	return posts.NewPostRepository(c.Client).LikePost(ctx, postID, userID)
}

func (c *testClient) UnlikePost(ctx context.Context, postID, userID string) error {
	return posts.NewPostRepository(c.Client).UnlikePost(ctx, postID, userID)
}

func (c *testClient) CreateComment(ctx context.Context, postID, userID, body string) (comments.Comment, error) {
	return comments.NewCommentRepository(c.Client).CreateComment(ctx, postID, userID, body)
}

func (c *testClient) ListComments(ctx context.Context, postID string, cursor *pagination.Cursor, limit int) ([]comments.Comment, *pagination.Cursor, error) {
	return comments.NewCommentRepository(c.Client).ListComments(ctx, postID, cursor, limit)
}

func (c *testClient) DeleteComment(ctx context.Context, postID, commentID, userID string) (bool, error) {
	return comments.NewCommentRepository(c.Client).DeleteComment(ctx, postID, commentID, userID)
}

func (c *testClient) CreateSession(ctx context.Context, sessionID string, userID int, expiresAt time.Time) (sessions.CreatedSession, error) {
	return sessions.NewSessionRepository(c.Client).CreateSession(ctx, sessionID, userID, expiresAt)
}

func (c *testClient) DeleteExpiredSessions(ctx context.Context) error {
	return sessions.NewSessionRepository(c.Client).DeleteExpiredSessions(ctx)
}

func (c *testClient) DeleteSession(ctx context.Context, sessionID string) error {
	return sessions.NewSessionRepository(c.Client).DeleteSession(ctx, sessionID)
}

func (c *testClient) ListActiveSessions(ctx context.Context, userID, currentSessionToken string) ([]sessions.Session, error) {
	return sessions.NewSessionRepository(c.Client).ListActiveSessions(ctx, userID, currentSessionToken)
}

func (c *testClient) DeleteSessionByID(
	ctx context.Context,
	publicID, userID, currentSessionToken string,
) (sessions.DeleteSessionOutcome, error) {
	return sessions.NewSessionRepository(c.Client).DeleteSessionByID(ctx, publicID, userID, currentSessionToken)
}
