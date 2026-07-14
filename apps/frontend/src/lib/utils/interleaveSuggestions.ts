import type { UserSuggestion, HashtagSuggestion } from '$lib/server/api/search';

// Keeps the combined dropdown short enough to scan while typing.
export const SUGGEST_DISPLAY_LIMIT = 8;

export type SuggestionItem =
	| { type: 'users'; item: UserSuggestion }
	| { type: 'hashtags'; item: HashtagSuggestion };

// Merges the two already-fetched, already-ranked typeahead arrays into one
// alternating, capped list. A slot landing on an exhausted queue is simply
// skipped until the cap is reached or both queues are drained.
export function interleaveSuggestions(
	users: UserSuggestion[],
	hashtags: HashtagSuggestion[]
): SuggestionItem[] {
	const items: SuggestionItem[] = [];
	let iu = 0;
	let ih = 0;
	while (items.length < SUGGEST_DISPLAY_LIMIT && (iu < users.length || ih < hashtags.length)) {
		const user = users[iu];
		if (user) {
			items.push({ type: 'users', item: user });
			iu++;
		}
		const hashtag = hashtags[ih];
		if (items.length < SUGGEST_DISPLAY_LIMIT && hashtag) {
			items.push({ type: 'hashtags', item: hashtag });
			ih++;
		}
	}
	return items;
}
