package search

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

const (
	// blendShareDivisor reserves roughly 1/5 of a page each for users and
	// hashtags; the remainder goes to posts, the dominant content type.
	blendShareDivisor = 5
)

// blendInterleavePattern controls the visual order of a blended page: posts
// appear 3x as often as users or hashtags, mirroring the ~60/20/20 split
// computeBlendTargets uses for page composition.
var blendInterleavePattern = []string{"users", "posts", "posts", "posts", "hashtags"}

// computeBlendTargets splits a page of limit results across the three entity
// types. Very small limits are handled as explicit cases since the general
// formula degenerates (a divisor-based share rounds to zero).
func computeBlendTargets(limit int) (users, posts, hashtags int) {
	switch {
	case limit <= 0:
		return 0, 0, 0
	case limit == 1:
		return 0, 1, 0
	case limit == 2:
		return 1, 1, 0
	}
	users = limit / blendShareDivisor
	if users == 0 {
		users = 1
	}
	hashtags = limit / blendShareDivisor
	if hashtags == 0 {
		hashtags = 1
	}
	posts = limit - users - hashtags
	return users, posts, hashtags
}

// blendCursor tracks the next Meilisearch offset for each of the three
// indexes independently, since a blended page draws from all of them.
type blendCursor struct {
	Users    int `json:"u"`
	Posts    int `json:"p"`
	Hashtags int `json:"h"`
}

func encodeBlendCursor(c blendCursor) string {
	b, err := json.Marshal(c)
	if err != nil {
		// c is a fixed struct of ints; marshaling cannot fail.
		panic(fmt.Sprintf("search: marshal blend cursor: %v", err))
	}
	return base64.StdEncoding.EncodeToString(b)
}

// An empty cursor returns the zero cursor (first page).
func decodeBlendCursor(cursor string) (blendCursor, error) {
	if cursor == "" {
		return blendCursor{}, nil
	}
	b, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return blendCursor{}, err
	}
	var c blendCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return blendCursor{}, err
	}
	if c.Users < 0 || c.Posts < 0 || c.Hashtags < 0 {
		return blendCursor{}, fmt.Errorf("invalid blend cursor")
	}
	return c, nil
}

// blendPlan says how many of each type's fetched hits belong in this page's
// output, and whether each type might have more beyond this page.
type blendPlan struct {
	ConsumeUsers    int
	ConsumePosts    int
	ConsumeHashtags int
	HasMoreUsers    bool
	HasMorePosts    bool
	HasMoreHashtags bool
}

// planBlend decides consumption counts for one page. Each fetchedX is the
// number of hits actually returned for a requested count of targetX+1 (a
// lookahead used only to detect more results, never shown). A type that
// comes back short of its target is backfilled from another type's
// already-fetched surplus this same round — never a new fetch — so a page
// can legitimately come back smaller than the sum of targets when two types
// are simultaneously scarce.
//
// hasMore is deliberately based on fetched == requested, not on how many
// hits were consumed: a type that donates its lookahead hit as backfill
// still has more beyond this page, even though nothing of it went unused.
func planBlend(targetUsers, targetPosts, targetHashtags, fetchedUsers, fetchedPosts, fetchedHashtags int) blendPlan {
	availUsers := min(fetchedUsers, targetUsers)
	availPosts := min(fetchedPosts, targetPosts)
	availHashtags := min(fetchedHashtags, targetHashtags)

	consumeUsers := availUsers
	consumePosts := availPosts
	consumeHashtags := availHashtags

	shortfall := (targetUsers - availUsers) + (targetHashtags - availHashtags)
	// Posts is the deepest, dominant category, so it donates first; a donor
	// that is itself short has zero spare capacity and is skipped naturally.
	donors := []*int{&consumePosts, &consumeUsers, &consumeHashtags}
	spare := []int{fetchedPosts - availPosts, fetchedUsers - availUsers, fetchedHashtags - availHashtags}
	for i := range donors {
		if shortfall <= 0 {
			break
		}
		take := min(shortfall, spare[i])
		*donors[i] += take
		shortfall -= take
	}

	return blendPlan{
		ConsumeUsers:    consumeUsers,
		ConsumePosts:    consumePosts,
		ConsumeHashtags: consumeHashtags,
		HasMoreUsers:    fetchedUsers == targetUsers+1,
		HasMorePosts:    fetchedPosts == targetPosts+1,
		HasMoreHashtags: fetchedHashtags == targetHashtags+1,
	}
}

// interleaveBlended orders already-sized (frozen-prefix) result slices into
// one page following blendInterleavePattern. A pattern slot landing on an
// exhausted queue is simply skipped that iteration; the loop only ends once
// every queue is drained, so every item is placed exactly once.
func interleaveBlended(users []UserResult, posts []PostResult, hashtags []HashtagResult) []BlendedItem {
	items := make([]BlendedItem, 0, len(users)+len(posts)+len(hashtags))
	iu, ip, ih := 0, 0, 0
	for pi := 0; iu < len(users) || ip < len(posts) || ih < len(hashtags); pi++ {
		switch blendInterleavePattern[pi%len(blendInterleavePattern)] {
		case "users":
			if iu < len(users) {
				items = append(items, BlendedItem{Type: "users", Item: users[iu]})
				iu++
			}
		case "posts":
			if ip < len(posts) {
				items = append(items, BlendedItem{Type: "posts", Item: posts[ip]})
				ip++
			}
		case "hashtags":
			if ih < len(hashtags) {
				items = append(items, BlendedItem{Type: "hashtags", Item: hashtags[ih]})
				ih++
			}
		}
	}
	return items
}

// BlendedItem tags a single result with its entity type for the blended
// (type=all) search response.
type BlendedItem struct {
	Type string `json:"type"`
	Item any    `json:"item"`
}

// partitionByFollowing stable-partitions users so ones the viewer follows
// come first, preserving their relative (Meilisearch relevance) order within
// each partition.
func partitionByFollowing(users []UserResult, following map[string]bool) []UserResult {
	result := make([]UserResult, 0, len(users))
	for _, u := range users {
		if following[u.Username] {
			result = append(result, u)
		}
	}
	for _, u := range users {
		if !following[u.Username] {
			result = append(result, u)
		}
	}
	return result
}
