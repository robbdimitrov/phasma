import type { SearchAllItem, UserSuggestion, HashtagSuggestion, SearchPostItem } from '$lib/server/api/search';

// Down from up to 21 rows shown today across three separate sections.
export const SUGGEST_DISPLAY_LIMIT = 8;

// Posts appear 3x as often as users or hashtags, mirroring the backend's
// blended full-page ratio (see computeBlendTargets in apps/backend).
const PATTERN: SearchAllItem['type'][] = ['users', 'posts', 'posts', 'posts', 'hashtags'];

// Merges the three already-fetched, already-ranked typeahead arrays into one
// blended, capped list. Unlike the backend's paginated blend, there's no
// cursor/lookahead here — this is a single one-shot fetch, so a pattern slot
// landing on an exhausted queue is simply skipped until the cap is reached or
// every queue is drained.
export function interleaveSuggestions(
	users: UserSuggestion[],
	posts: SearchPostItem[],
	hashtags: HashtagSuggestion[]
): SearchAllItem[] {
	const items: SearchAllItem[] = [];
	let iu = 0;
	let ip = 0;
	let ih = 0;
	for (
		let pi = 0;
		items.length < SUGGEST_DISPLAY_LIMIT && (iu < users.length || ip < posts.length || ih < hashtags.length);
		pi++
	) {
		switch (PATTERN[pi % PATTERN.length]) {
			case 'users': {
				const user = users[iu];
				if (user) {
					items.push({ type: 'users', item: user });
					iu++;
				}
				break;
			}
			case 'posts': {
				const post = posts[ip];
				if (post) {
					items.push({ type: 'posts', item: post });
					ip++;
				}
				break;
			}
			case 'hashtags': {
				const hashtag = hashtags[ih];
				if (hashtag) {
					items.push({ type: 'hashtags', item: hashtag });
					ih++;
				}
				break;
			}
		}
	}
	return items;
}
