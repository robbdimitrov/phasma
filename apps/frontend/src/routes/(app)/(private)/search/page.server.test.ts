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
			users: { items: [], nextCursor: null },
			posts: { items: [], nextCursor: null },
			hashtags: { items: [], nextCursor: null },
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

	it('fetches all three categories in parallel with a preview limit', async () => {
		search.mockImplementation((_api, params: { type: string }) =>
			Promise.resolve({ items: [{ type: params.type }], nextCursor: null })
		);

		const result = await load(loadEvent('http://localhost/search?q=alice'));

		expect(search).toHaveBeenCalledTimes(3);
		expect(search).toHaveBeenCalledWith(expect.anything(), {
			q: 'alice',
			type: 'users',
			limit: 5
		});
		expect(search).toHaveBeenCalledWith(expect.anything(), {
			q: 'alice',
			type: 'posts',
			limit: 5
		});
		expect(search).toHaveBeenCalledWith(expect.anything(), {
			q: 'alice',
			type: 'hashtags',
			limit: 5
		});
		expect(result).toEqual({
			q: 'alice',
			users: { items: [{ type: 'users' }], nextCursor: null },
			posts: { items: [{ type: 'posts' }], nextCursor: null },
			hashtags: { items: [{ type: 'hashtags' }], nextCursor: null },
			suggested: [],
			popular: []
		});
	});

	it('an @ prefix narrows the search to users only', async () => {
		search.mockResolvedValue({ items: [], nextCursor: null });

		const result = await load(loadEvent('http://localhost/search?q=%40alice'));

		expect(search).toHaveBeenCalledTimes(1);
		expect(search).toHaveBeenCalledWith(expect.anything(), {
			q: 'alice',
			type: 'users',
			limit: 5
		});
		expect(result).toEqual(
			expect.objectContaining({
				posts: { items: [], nextCursor: null },
				hashtags: { items: [], nextCursor: null }
			})
		);
	});

	it('a # prefix narrows the search to posts only', async () => {
		search.mockResolvedValue({ items: [], nextCursor: null });

		const result = await load(loadEvent('http://localhost/search?q=%23vacation'));

		expect(search).toHaveBeenCalledTimes(1);
		expect(search).toHaveBeenCalledWith(expect.anything(), {
			q: '#vacation',
			type: 'posts',
			limit: 5
		});
		expect(result).toEqual(
			expect.objectContaining({
				users: { items: [], nextCursor: null },
				hashtags: { items: [], nextCursor: null }
			})
		);
	});

	it('degrades one failing category to empty instead of failing the whole page', async () => {
		search.mockImplementation((_api, params: { type: string }) => {
			if (params.type === 'hashtags') return Promise.reject(new Error('backend down'));
			return Promise.resolve({ items: [{ type: params.type }], nextCursor: null });
		});

		const result = await load(loadEvent('http://localhost/search?q=alice'));

		expect(result).toEqual({
			q: 'alice',
			users: { items: [{ type: 'users' }], nextCursor: null },
			posts: { items: [{ type: 'posts' }], nextCursor: null },
			hashtags: { items: [], nextCursor: null },
			suggested: [],
			popular: []
		});
	});
});
