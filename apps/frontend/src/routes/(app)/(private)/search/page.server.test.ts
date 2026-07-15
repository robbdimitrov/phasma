import { error, isRedirect } from '@sveltejs/kit';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const {
	apiClient,
	search,
	listRecentSearches,
	deleteRecentSearch,
	clearRecentSearches,
	getSuggestedUsers,
	getPopularPosts,
	followUser,
	unfollowUser
} = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	search: vi.fn(),
	listRecentSearches: vi.fn(),
	deleteRecentSearch: vi.fn(),
	clearRecentSearches: vi.fn(),
	getSuggestedUsers: vi.fn(),
	getPopularPosts: vi.fn(),
	followUser: vi.fn(),
	unfollowUser: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/search', () => ({
	search,
	listRecentSearches,
	deleteRecentSearch,
	clearRecentSearches
}));
vi.mock('$lib/server/api/users', () => ({ getSuggestedUsers, followUser, unfollowUser }));
vi.mock('$lib/server/api/posts', () => ({ getPopularPosts }));

import { load, actions } from './+page.server';
import { load as guardLoad } from '../+layout.server';

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

function actionEvent(formData: FormData, session: string | null = 'session-token') {
	return {
		cookies: { get: vi.fn(() => session ?? undefined) },
		fetch: vi.fn(),
		request: new Request('http://localhost/search', { method: 'POST', body: formData })
	} as unknown as Parameters<ReturnType<typeof followAction>>[0];
}

function usernameFormData(username?: string) {
	const data = new FormData();
	if (username !== undefined) data.set('username', username);
	return data;
}

function removeRecentAction() {
	const action = actions.removeRecent;
	if (!action) throw new Error('removeRecent action is not defined');
	return action;
}

function clearRecentAction() {
	const action = actions.clearRecent;
	if (!action) throw new Error('clearRecent action is not defined');
	return action;
}

function idFormData(id?: string) {
	const data = new FormData();
	if (id !== undefined) data.set('id', id);
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
	listRecentSearches.mockResolvedValue([]);
});

function backendError(status: number): unknown {
	try {
		error(status, 'backend details');
	} catch (cause) {
		return cause;
	}
}

describe('search page load', () => {
	it('redirects anonymous visitors through the private layout guard', async () => {
		const parent = vi.fn().mockResolvedValue({ currentUser: null });

		try {
			await guardLoad({ parent } as unknown as Parameters<typeof guardLoad>[0]);
			expect.unreachable('expected a redirect to be thrown');
		} catch (err) {
			if (!isRedirect(err)) throw err;
			expect(err.status).toBe(303);
			expect(err.location).toBe('/login');
		}
	});

	it('shows discovery content for an empty query', async () => {
		getSuggestedUsers.mockResolvedValue({ items: [{ username: 'alice' }] });
		getPopularPosts.mockResolvedValue({ items: [{ publicId: 'post-1' }] });
		listRecentSearches.mockResolvedValue([{ id: 'r1', type: 'posts', item: 'sunset' }]);

		const result = await load(loadEvent('http://localhost/search'));

		expect(result).toEqual({
			q: '',
			resultsQuery: '',
			results: { items: [], nextCursor: null },
			recent: [{ id: 'r1', type: 'posts', item: 'sunset' }],
			suggested: [{ username: 'alice' }],
			popular: [{ publicId: 'post-1' }]
		});
		expect(search).not.toHaveBeenCalled();
	});

	it('degrades a failing recent-searches fetch to an empty list instead of failing the page', async () => {
		getSuggestedUsers.mockResolvedValue({ items: [] });
		getPopularPosts.mockResolvedValue({ items: [] });
		listRecentSearches.mockRejectedValue(new Error('backend down'));

		const result = await load(loadEvent('http://localhost/search'));

		expect(result).toMatchObject({ recent: [] });
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
			recent: [],
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
			recent: [],
			suggested: [],
			popular: []
		});
	});
});

describe('search page follow action', () => {
	it('redirects anonymous submissions to /login', async () => {
		try {
			await followAction()(actionEvent(usernameFormData('alice'), null));
			expect.unreachable('expected a redirect to be thrown');
		} catch (err) {
			if (!isRedirect(err)) throw err;
			expect(err.status).toBe(303);
			expect(err.location).toBe('/login');
		}
		expect(followUser).not.toHaveBeenCalled();
	});

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

	it('redirects expired sessions to /login', async () => {
		followUser.mockRejectedValue(backendError(401));

		try {
			await followAction()(actionEvent(usernameFormData('alice')));
			expect.unreachable('expected a redirect to be thrown');
		} catch (err) {
			if (!isRedirect(err)) throw err;
			expect(err.status).toBe(303);
			expect(err.location).toBe('/login');
		}
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

describe('search page removeRecent action', () => {
	it('redirects anonymous submissions to /login', async () => {
		try {
			await removeRecentAction()(actionEvent(idFormData('r1'), null));
			expect.unreachable('expected a redirect to be thrown');
		} catch (err) {
			if (!isRedirect(err)) throw err;
			expect(err.status).toBe(303);
			expect(err.location).toBe('/login');
		}
		expect(deleteRecentSearch).not.toHaveBeenCalled();
	});

	it('rejects a submission missing id before calling the backend', async () => {
		const result = await removeRecentAction()(actionEvent(idFormData()));

		expect(result).toMatchObject({ status: 400 });
		expect(deleteRecentSearch).not.toHaveBeenCalled();
	});

	it('reports a backend failure instead of silently succeeding', async () => {
		deleteRecentSearch.mockRejectedValue(new Error('backend down'));

		const result = await removeRecentAction()(actionEvent(idFormData('r1')));

		expect(result).toMatchObject({
			status: 503,
			data: { error: 'Could not remove recent search.' }
		});
	});

	it('redirects expired sessions to /login', async () => {
		deleteRecentSearch.mockRejectedValue(backendError(401));

		try {
			await removeRecentAction()(actionEvent(idFormData('r1')));
			expect.unreachable('expected a redirect to be thrown');
		} catch (err) {
			if (!isRedirect(err)) throw err;
			expect(err.status).toBe(303);
			expect(err.location).toBe('/login');
		}
	});

	it('succeeds when the backend call succeeds', async () => {
		deleteRecentSearch.mockResolvedValue(null);

		const result = await removeRecentAction()(actionEvent(idFormData('r1')));

		expect(deleteRecentSearch).toHaveBeenCalledWith(expect.anything(), 'r1');
		expect(result).toEqual({ success: true });
	});
});

describe('search page clearRecent action', () => {
	it('redirects anonymous submissions to /login', async () => {
		try {
			await clearRecentAction()(actionEvent(new FormData(), null));
			expect.unreachable('expected a redirect to be thrown');
		} catch (err) {
			if (!isRedirect(err)) throw err;
			expect(err.status).toBe(303);
			expect(err.location).toBe('/login');
		}
		expect(clearRecentSearches).not.toHaveBeenCalled();
	});

	it('reports a backend failure instead of silently succeeding', async () => {
		clearRecentSearches.mockRejectedValue(new Error('backend down'));

		const result = await clearRecentAction()(actionEvent(new FormData()));

		expect(result).toMatchObject({
			status: 503,
			data: { error: 'Could not clear recent searches.' }
		});
	});

	it('succeeds when the backend call succeeds', async () => {
		clearRecentSearches.mockResolvedValue(null);

		const result = await clearRecentAction()(actionEvent(new FormData()));

		expect(clearRecentSearches).toHaveBeenCalledWith(expect.anything());
		expect(result).toEqual({ success: true });
	});
});
