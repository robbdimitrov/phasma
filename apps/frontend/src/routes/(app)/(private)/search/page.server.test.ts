import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, search, getSuggestedUsers, getPopularPosts, followUser, unfollowUser } =
	vi.hoisted(() => ({
		apiClient: vi.fn(() => vi.fn()),
		search: vi.fn(),
		getSuggestedUsers: vi.fn(),
		getPopularPosts: vi.fn(),
		followUser: vi.fn(),
		unfollowUser: vi.fn()
	}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/search', () => ({ search }));
vi.mock('$lib/server/api/users', () => ({ getSuggestedUsers, followUser, unfollowUser }));
vi.mock('$lib/server/api/posts', () => ({ getPopularPosts }));

import { load, actions } from './+page.server';

function followAction() {
	const action = actions.follow;
	if (!action) throw new Error('follow action is not defined');
	return action;
}

function unfollowAction() {
	const action = actions.unfollow;
	if (!action) throw new Error('unfollow action is not defined');
	return action;
}

function actionEvent(formData: FormData) {
	return {
		cookies: {},
		fetch: vi.fn(),
		request: new Request('http://localhost/search', { method: 'POST', body: formData })
	} as unknown as Parameters<ReturnType<typeof followAction>>[0];
}

function usernameFormData(username?: string) {
	const data = new FormData();
	if (username !== undefined) data.set('username', username);
	return data;
}

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

	it('fetches a posts-only page for a bare query', async () => {
		search.mockResolvedValue({
			items: [{ id: 'p1' }],
			nextCursor: null
		});

		const result = await load(loadEvent('http://localhost/search?q=alice'));

		expect(search).toHaveBeenCalledTimes(1);
		expect(search).toHaveBeenCalledWith(expect.anything(), {
			q: 'alice',
			type: 'posts',
			limit: 5
		});
		expect(result).toEqual({
			q: 'alice',
			resultsQuery: 'alice',
			results: { items: [{ type: 'posts', item: { id: 'p1' } }], nextCursor: null },
			suggested: [],
			popular: []
		});
	});

	it('an @ prefix is stripped before searching posts, since users are handled by the typeahead', async () => {
		search.mockResolvedValue({ items: [{ id: 'p1' }], nextCursor: null });

		const result = await load(loadEvent('http://localhost/search?q=%40alice'));

		expect(search).toHaveBeenCalledTimes(1);
		expect(search).toHaveBeenCalledWith(expect.anything(), {
			q: 'alice',
			type: 'posts',
			limit: 5
		});
		expect(result).toEqual(
			expect.objectContaining({
				resultsQuery: 'alice',
				results: { items: [{ type: 'posts', item: { id: 'p1' } }], nextCursor: null }
			})
		);
	});

	it('a # prefix stays intact so the backend applies its exact-hashtag filter', async () => {
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
				results: { items: [{ type: 'posts', item: { id: 'p1' } }], nextCursor: null }
			})
		);
	});

	it('degrades a failing search to empty results instead of failing the page', async () => {
		search.mockRejectedValue(new Error('backend down'));

		const result = await load(loadEvent('http://localhost/search?q=alice'));

		expect(result).toEqual({
			q: 'alice',
			resultsQuery: 'alice',
			results: { items: [], nextCursor: null },
			suggested: [],
			popular: []
		});
	});
});

describe('search page follow action', () => {
	it('rejects a submission missing username before calling the backend', async () => {
		const result = await followAction()(actionEvent(usernameFormData()));

		expect(result).toMatchObject({ status: 400 });
		expect(followUser).not.toHaveBeenCalled();
	});

	it('reports a backend failure instead of silently succeeding', async () => {
		followUser.mockRejectedValue(new Error('backend down'));

		const result = await followAction()(actionEvent(usernameFormData('alice')));

		expect(result).toMatchObject({
			status: 503,
			data: { error: 'Could not update follow status.' }
		});
	});

	it('succeeds when the backend call succeeds', async () => {
		followUser.mockResolvedValue(null);

		const result = await followAction()(actionEvent(usernameFormData('alice')));

		expect(followUser).toHaveBeenCalledWith(expect.anything(), 'alice');
		expect(result).toEqual({ success: true });
	});
});

describe('search page unfollow action', () => {
	it('rejects a submission missing username before calling the backend', async () => {
		const result = await unfollowAction()(actionEvent(usernameFormData()));

		expect(result).toMatchObject({ status: 400 });
		expect(unfollowUser).not.toHaveBeenCalled();
	});

	it('reports a backend failure instead of silently succeeding', async () => {
		unfollowUser.mockRejectedValue(new Error('backend down'));

		const result = await unfollowAction()(actionEvent(usernameFormData('alice')));

		expect(result).toMatchObject({
			status: 503,
			data: { error: 'Could not update follow status.' }
		});
	});

	it('succeeds when the backend call succeeds', async () => {
		unfollowUser.mockResolvedValue(null);

		const result = await unfollowAction()(actionEvent(usernameFormData('alice')));

		expect(unfollowUser).toHaveBeenCalledWith(expect.anything(), 'alice');
		expect(result).toEqual({ success: true });
	});
});
