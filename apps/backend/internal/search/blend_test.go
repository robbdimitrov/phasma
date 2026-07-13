package search

import (
	"fmt"
	"testing"
)

func TestComputeBlendTargets(t *testing.T) {
	tests := []struct {
		limit                  int
		users, posts, hashtags int
	}{
		{0, 0, 0, 0},
		{1, 0, 1, 0},
		{2, 1, 1, 0},
		{3, 1, 1, 1},
		{4, 1, 2, 1},
		{5, 1, 3, 1},
		{20, 4, 12, 4},
		{50, 10, 30, 10},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("limit=%d", tt.limit), func(t *testing.T) {
			users, posts, hashtags := computeBlendTargets(tt.limit)
			if users != tt.users || posts != tt.posts || hashtags != tt.hashtags {
				t.Fatalf("computeBlendTargets(%d) = (%d,%d,%d), want (%d,%d,%d)",
					tt.limit, users, posts, hashtags, tt.users, tt.posts, tt.hashtags)
			}
			if users < 0 || posts < 0 || hashtags < 0 {
				t.Fatalf("computeBlendTargets(%d) produced a negative target", tt.limit)
			}
			if tt.limit > 0 && users+posts+hashtags != tt.limit {
				t.Fatalf("computeBlendTargets(%d) sums to %d, want %d", tt.limit, users+posts+hashtags, tt.limit)
			}
		})
	}
}

func TestPlanBlendFullRound(t *testing.T) {
	// Every type returns its full target+1 lookahead: nothing is short, and
	// every type still has more beyond this page.
	plan := planBlend(4, 12, 4, 5, 13, 5)
	if plan.ConsumeUsers != 4 || plan.ConsumePosts != 12 || plan.ConsumeHashtags != 4 {
		t.Fatalf("consume = (%d,%d,%d), want (4,12,4)", plan.ConsumeUsers, plan.ConsumePosts, plan.ConsumeHashtags)
	}
	if !plan.HasMoreUsers || !plan.HasMorePosts || !plan.HasMoreHashtags {
		t.Fatalf("expected hasMore true for all types, got %+v", plan)
	}
}

func TestPlanBlendExhaustionConvergesToNoMore(t *testing.T) {
	// Every type returns exactly its target (not target+1): each is exhausted
	// right at the page boundary, so hasMore must be false for all three.
	plan := planBlend(4, 12, 4, 4, 12, 4)
	if plan.ConsumeUsers != 4 || plan.ConsumePosts != 12 || plan.ConsumeHashtags != 4 {
		t.Fatalf("consume = (%d,%d,%d), want (4,12,4)", plan.ConsumeUsers, plan.ConsumePosts, plan.ConsumeHashtags)
	}
	if plan.HasMoreUsers || plan.HasMorePosts || plan.HasMoreHashtags {
		t.Fatalf("expected hasMore false for all types at exact exhaustion, got %+v", plan)
	}
}

func TestPlanBlendBackfillsShortfallFromPostsSurplus(t *testing.T) {
	// Hashtags only has 2 available against a target of 4; posts has its one
	// surplus (lookahead) item to spare. Users lands exactly on its target
	// (no surplus of its own), isolating this as a posts-only backfill case.
	// The freed slot is backfilled from posts, and hashtags is correctly
	// marked exhausted.
	plan := planBlend(4, 12, 4, 4, 13, 2)
	if plan.ConsumeHashtags != 2 {
		t.Fatalf("ConsumeHashtags = %d, want 2 (only what was available)", plan.ConsumeHashtags)
	}
	if plan.ConsumePosts != 13 {
		t.Fatalf("ConsumePosts = %d, want 13 (target 12 + 1 backfilled)", plan.ConsumePosts)
	}
	if plan.ConsumeUsers != 4 {
		t.Fatalf("ConsumeUsers = %d, want 4 (unaffected)", plan.ConsumeUsers)
	}
	if plan.HasMoreHashtags {
		t.Fatal("expected HasMoreHashtags = false, hashtags returned fewer than requested")
	}
	if !plan.HasMorePosts {
		t.Fatal("expected HasMorePosts = true even though its lookahead item was donated as backfill")
	}
	total := plan.ConsumeUsers + plan.ConsumePosts + plan.ConsumeHashtags
	if total != 19 {
		t.Fatalf("total consumed = %d, want 19 (limit 20 minus the 1 unrecoverable hashtag shortfall)", total)
	}
}

func TestPlanBlendShrinksPageWhenShortfallExceedsSpare(t *testing.T) {
	// Both users and hashtags are simultaneously scarce; posts has at most 1
	// spare item to offer, so the page legitimately comes back smaller than
	// the sum of targets rather than losing or duplicating anything.
	plan := planBlend(4, 12, 4, 1, 13, 0)
	if plan.ConsumeUsers != 1 || plan.ConsumeHashtags != 0 {
		t.Fatalf("consume users/hashtags = %d/%d, want 1/0 (only what was available)", plan.ConsumeUsers, plan.ConsumeHashtags)
	}
	if plan.ConsumePosts != 13 {
		t.Fatalf("ConsumePosts = %d, want 13 (target 12 + the 1 available spare)", plan.ConsumePosts)
	}
	total := plan.ConsumeUsers + plan.ConsumePosts + plan.ConsumeHashtags
	if total >= 20 {
		t.Fatalf("total consumed = %d, want < 20 (limit), page should shrink rather than fabricate results", total)
	}
	if plan.HasMoreUsers {
		t.Fatal("expected HasMoreUsers = false, users returned fewer than requested")
	}
}

func TestInterleaveBlendedOrdersByPattern(t *testing.T) {
	users := []UserResult{{Username: "u1"}}
	posts := []PostResult{{ID: "p1"}, {ID: "p2"}, {ID: "p3"}}
	hashtags := []HashtagResult{{Name: "h1"}}

	items := interleaveBlended(users, posts, hashtags)

	wantOrder := []string{"users", "posts", "posts", "posts", "hashtags"}
	if len(items) != len(wantOrder) {
		t.Fatalf("got %d items, want %d", len(items), len(wantOrder))
	}
	for i, want := range wantOrder {
		if items[i].Type != want {
			t.Fatalf("item %d has type %q, want %q", i, items[i].Type, want)
		}
	}
}

func TestInterleaveBlendedSkipsExhaustedQueuesWithoutLoss(t *testing.T) {
	// Users and hashtags are both empty; every item must still surface, in
	// order, with the pattern's users/hashtags slots simply skipped.
	posts := []PostResult{{ID: "p1"}, {ID: "p2"}}

	items := interleaveBlended(nil, posts, nil)

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	for i, item := range items {
		if item.Type != "posts" {
			t.Fatalf("item %d has type %q, want posts", i, item.Type)
		}
	}
}

// TestBlendPaginationNeverRepeatsOrSkips walks a fake, fixed-size index for
// each type across many pages, applying the same offset-advances-by-consumed
// invariant the handler uses, and asserts every id surfaces exactly once and
// pagination terminates with a nil cursor.
func TestBlendPaginationNeverRepeatsOrSkips(t *testing.T) {
	const limit = 10
	targetUsers, targetPosts, targetHashtags := computeBlendTargets(limit)

	allUsers := fakeIDs("user", 7)
	allPosts := fakeIDs("post", 41)
	allHashtags := fakeIDs("hashtag", 3)

	seen := map[string]int{}
	offsetUsers, offsetPosts, offsetHashtags := 0, 0, 0

	for range 50 {
		fetchedUsers := fakeFetch(allUsers, offsetUsers, targetUsers+1)
		fetchedPosts := fakeFetch(allPosts, offsetPosts, targetPosts+1)
		fetchedHashtags := fakeFetch(allHashtags, offsetHashtags, targetHashtags+1)

		plan := planBlend(targetUsers, targetPosts, targetHashtags,
			len(fetchedUsers), len(fetchedPosts), len(fetchedHashtags))

		for _, id := range fetchedUsers[:plan.ConsumeUsers] {
			seen[id]++
		}
		for _, id := range fetchedPosts[:plan.ConsumePosts] {
			seen[id]++
		}
		for _, id := range fetchedHashtags[:plan.ConsumeHashtags] {
			seen[id]++
		}

		offsetUsers += plan.ConsumeUsers
		offsetPosts += plan.ConsumePosts
		offsetHashtags += plan.ConsumeHashtags

		if !plan.HasMoreUsers && !plan.HasMorePosts && !plan.HasMoreHashtags {
			for _, id := range allUsers {
				if seen[id] != 1 {
					t.Fatalf("user id %q seen %d times, want 1", id, seen[id])
				}
			}
			for _, id := range allPosts {
				if seen[id] != 1 {
					t.Fatalf("post id %q seen %d times, want 1", id, seen[id])
				}
			}
			for _, id := range allHashtags {
				if seen[id] != 1 {
					t.Fatalf("hashtag id %q seen %d times, want 1", id, seen[id])
				}
			}
			return
		}
	}
	t.Fatal("pagination did not converge to a nil cursor within 50 pages")
}

func TestPartitionByFollowingPreservesRelativeOrder(t *testing.T) {
	users := []UserResult{
		{Username: "a"}, {Username: "b"}, {Username: "c"}, {Username: "d"},
	}
	following := map[string]bool{"b": true, "d": true}

	got := partitionByFollowing(users, following)

	want := []string{"b", "d", "a", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %d users, want %d", len(got), len(want))
	}
	for i, username := range want {
		if got[i].Username != username {
			t.Fatalf("position %d = %q, want %q", i, got[i].Username, username)
		}
	}
}

func fakeIDs(prefix string, n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("%s-%d", prefix, i)
	}
	return ids
}

func fakeFetch(all []string, offset, count int) []string {
	if offset >= len(all) {
		return nil
	}
	end := min(offset+count, len(all))
	return all[offset:end]
}
