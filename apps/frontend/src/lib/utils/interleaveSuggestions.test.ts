import { describe, expect, it } from 'vitest';
import { interleaveSuggestions, SUGGEST_DISPLAY_LIMIT } from './interleaveSuggestions';
import type { HashtagSuggestion, SearchPostItem, UserSuggestion } from '$lib/server/api/search';

function makeUsers(n: number): UserSuggestion[] {
	return Array.from({ length: n }, (_, i) => ({ username: `user${i}`, name: '', avatar: null }));
}

function makePosts(n: number): SearchPostItem[] {
	return Array.from({ length: n }, (_, i) => ({
		id: `post${i}`,
		username: 'u',
		description: '',
		filename: 'f.jpg'
	}));
}

function makeHashtags(n: number): HashtagSuggestion[] {
	return Array.from({ length: n }, (_, i) => ({ name: `tag${i}`, postCount: i }));
}

describe('interleaveSuggestions', () => {
	it('follows the users/posts/posts/posts/hashtags pattern when all three have enough items', () => {
		const items = interleaveSuggestions(makeUsers(5), makePosts(5), makeHashtags(5));

		expect(items.map((i) => i.type)).toEqual(['users', 'posts', 'posts', 'posts', 'hashtags', 'users', 'posts', 'posts']);
	});

	it('caps the result at SUGGEST_DISPLAY_LIMIT', () => {
		const items = interleaveSuggestions(makeUsers(20), makePosts(20), makeHashtags(20));

		expect(items).toHaveLength(SUGGEST_DISPLAY_LIMIT);
	});

	it('skips exhausted queues without losing or duplicating items', () => {
		const items = interleaveSuggestions([], makePosts(3), []);

		expect(items.map((i) => i.type)).toEqual(['posts', 'posts', 'posts']);
		expect(items.map((i) => (i.type === 'posts' ? i.item.id : null))).toEqual(['post0', 'post1', 'post2']);
	});

	it('returns an empty list when all three inputs are empty', () => {
		expect(interleaveSuggestions([], [], [])).toEqual([]);
	});
});
