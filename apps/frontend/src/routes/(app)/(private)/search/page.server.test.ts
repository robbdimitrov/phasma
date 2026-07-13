import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, search, getSuggestedUsers, getPopularPosts } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	search: vi.fn(),
	getSuggestedUsers: vi.fn(),
	getPopularPosts: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/search', () => ({ search }));
vi.mock('$lib/server/api/users', () => ({
	getSuggestedUsers,
	followUser: vi.fn(),
	unfollowUser: vi.fn()
}));
vi.mock('$lib/server/api/posts', () => ({ getPopularPosts }));

import { load } from './+page.server';

function loadEvent(url: string): Parameters<typeof load>[0] {
	return {
		url: new URL(url),
		cookies: {},
		fetch: vi.fn()
	} as unknown as Parameters<typeof load>[0];
}

beforeEach(() => {
	vi.clearAllMocks();
});

describe('search page load', () => {
	it('shows discovery content for an empty query', async () => {
		getSuggestedUsers.mockResolvedValue({ items: [{ username: 'alice' }] });
		getPopularPosts.mockResolvedValue({ items: [{ publicId: 'post-1' }] });

		const result = await load(loadEvent('http://localhost/search'));

		expect(result).toEqual({
			q: '',
			resultsQuery: '',
			resultsType: 'all',
			results: { items: [], nextCursor: null },
			suggested: [{ username: 'alice' }],
			popular: [{ publicId: 'post-1' }]
		});
		expect(search).not.toHaveBeenCalled();
	});

	it('shows discovery content when the query exceeds the length limit', async () => {
		getSuggestedUsers.mockResolvedValue({ items: [] });
		getPopularPosts.mockResolvedValue({ items: [] });

		const tooLong = 'a'.repeat(51);
		await load(loadEvent(`http://localhost/search?q=${tooLong}`));

		expect(search).not.toHaveBeenCalled();
	});

	it('fetches one blended page for a bare query', async () => {
		search.mockResolvedValue({
			items: [{ type: 'users', item: {} }],
			nextCursor: null
		});

		const result = await load(loadEvent('http://localhost/search?q=alice'));

		expect(search).toHaveBeenCalledTimes(1);
		expect(search).toHaveBeenCalledWith(expect.anything(), {
			q: 'alice',
			type: 'all',
			limit: 5
		});
		expect(result).toEqual({
			q: 'alice',
			resultsQuery: 'alice',
			resultsType: 'all',
			results: { items: [{ type: 'users', item: {} }], nextCursor: null },
			suggested: [],
			popular: []
		});
	});

	it('an @ prefix narrows the search to users only, tagged as type users', async () => {
		search.mockResolvedValue({ items: [{ username: 'alice' }], nextCursor: null });

		const result = await load(loadEvent('http://localhost/search?q=%40alice'));

		expect(search).toHaveBeenCalledTimes(1);
		expect(search).toHaveBeenCalledWith(expect.anything(), {
			q: 'alice',
			type: 'users',
			limit: 5
		});
		expect(result).toEqual(
			expect.objectContaining({
				resultsQuery: 'alice',
				resultsType: 'users',
				results: { items: [{ type: 'users', item: { username: 'alice' } }], nextCursor: null }
			})
		);
	});

	it('a # prefix narrows the search to posts only, tagged as type posts', async () => {
		search.mockResolvedValue({ items: [{ id: 'p1' }], nextCursor: null });

		const result = await load(loadEvent('http://localhost/search?q=%23vacation'));

		expect(search).toHaveBeenCalledTimes(1);
		expect(search).toHaveBeenCalledWith(expect.anything(), {
			q: '#vacation',
			type: 'posts',
			limit: 5
		});
		expect(result).toEqual(
			expect.objectContaining({
				resultsQuery: '#vacation',
				resultsType: 'posts',
				results: { items: [{ type: 'posts', item: { id: 'p1' } }], nextCursor: null }
			})
		);
	});

	it('degrades a failing blended search to empty results instead of failing the page', async () => {
		search.mockRejectedValue(new Error('backend down'));

		const result = await load(loadEvent('http://localhost/search?q=alice'));

		expect(result).toEqual({
			q: 'alice',
			resultsQuery: 'alice',
			resultsType: 'all',
			results: { items: [], nextCursor: null },
			suggested: [],
			popular: []
		});
	});
});
