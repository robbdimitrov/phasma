import { describe, expect, it } from 'vitest';
import { interleaveSuggestions, SUGGEST_DISPLAY_LIMIT } from './interleaveSuggestions';
import type { HashtagSuggestion, UserSuggestion } from '$lib/server/api/search';

function makeUsers(n: number): UserSuggestion[] {
	return Array.from({ length: n }, (_, i) => ({ username: `user${i}`, name: '', avatar: null }));
}

function makeHashtags(n: number): HashtagSuggestion[] {
	return Array.from({ length: n }, (_, i) => ({ name: `tag${i}`, postCount: i }));
}

describe('interleaveSuggestions', () => {
	it('alternates users and hashtags when both have enough items', () => {
		const items = interleaveSuggestions(
			makeUsers(SUGGEST_DISPLAY_LIMIT),
			makeHashtags(SUGGEST_DISPLAY_LIMIT)
		);

		const expectedTypes = Array.from({ length: SUGGEST_DISPLAY_LIMIT }, (_, i) =>
			i % 2 === 0 ? 'users' : 'hashtags'
		);
		expect(items.map((i) => i.type)).toEqual(expectedTypes);
	});

	it('caps the result at SUGGEST_DISPLAY_LIMIT', () => {
		const items = interleaveSuggestions(makeUsers(20), makeHashtags(20));

		expect(items).toHaveLength(SUGGEST_DISPLAY_LIMIT);
	});

	it('skips an exhausted queue without losing or duplicating items', () => {
		const items = interleaveSuggestions([], makeHashtags(3));

		expect(items.map((i) => i.type)).toEqual(['hashtags', 'hashtags', 'hashtags']);
		expect(items.map((i) => (i.type === 'hashtags' ? i.item.name : null))).toEqual([
			'tag0',
			'tag1',
			'tag2'
		]);
	});

	it('returns an empty list when both inputs are empty', () => {
		expect(interleaveSuggestions([], [])).toEqual([]);
	});
});
