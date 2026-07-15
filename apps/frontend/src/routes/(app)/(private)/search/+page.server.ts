import { fail, isHttpError, redirect } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import {
	search,
	type SearchPostItem,
	type SearchAllItem,
	type SearchPage
} from '$lib/server/api/search';
import { getSuggestedUsers, followUser, unfollowUser } from '$lib/server/api/users';
import { getPopularPosts } from '$lib/server/api/posts';
import { apiClient } from '$lib/server/api/client';
import {
	SEARCH_PREVIEW_LIMIT,
	searchQueryPrefix,
	stripSearchQueryPrefix
} from '$lib/utils/searchQuery';

const MAX_Q_LENGTH = 50;

const EMPTY_RESULTS: SearchPage<SearchAllItem> = { items: [], nextCursor: null };

// Tags each post with its entity type, so the results page and its "Load
// more" continuation share the same blended-item shape the typeahead uses.
function wrapPosts(page: SearchPage<SearchPostItem>): SearchPage<SearchAllItem> {
	return {
		items: page.items.map((item) => ({ type: 'posts', item }) as SearchAllItem),
		nextCursor: page.nextCursor
	};
}

// A failing search (e.g. Meilisearch briefly unavailable, or an invalid
// #hashtag filter) must not take down the whole page.
function searchResults(
	promise: Promise<SearchPage<SearchAllItem>>
): Promise<SearchPage<SearchAllItem>> {
	return promise.catch(() => EMPTY_RESULTS);
}

export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams.get('q') ?? '';

	if (!q || q.length > MAX_Q_LENGTH) {
		const api = apiClient(event);
		const [suggested, popular] = await Promise.all([getSuggestedUsers(api), getPopularPosts(api)]);
		return {
			q,
			resultsQuery: q,
			results: EMPTY_RESULTS,
			suggested: suggested.items,
			popular: popular.items
		};
	}

	const api = apiClient(event);
	const prefix = searchQueryPrefix(q);
	// Results are posts-only: typeahead handles users/hashtags. Strip @ for
	// posts search; keep # so the backend can apply exact-hashtag filtering.
	const resultsQuery = prefix === '@' ? stripSearchQueryPrefix(q, prefix) : q;
	const results = searchResults(
		search<SearchPostItem>(api, {
			q: resultsQuery,
			type: 'posts',
			limit: SEARCH_PREVIEW_LIMIT
		}).then(wrapPosts)
	);

	return { q, resultsQuery, results: await results, suggested: [], popular: [] };
};

export const actions: Actions = {
	follow: async (event) => {
		if (!event.cookies.get('session')) throw redirect(303, '/login');
		const api = apiClient(event);
		const data = await event.request.formData();
		const username = (data.get('username') as string) ?? '';
		if (!username) return fail(400, { error: 'Missing username.' });
		try {
			await followUser(api, username);
		} catch (cause) {
			if (isHttpError(cause) && cause.status === 401) throw redirect(303, '/login');
			return fail(503, { error: 'Could not update follow status.' });
		}
		return { success: true };
	},
	unfollow: async (event) => {
		if (!event.cookies.get('session')) throw redirect(303, '/login');
		const api = apiClient(event);
		const data = await event.request.formData();
		const username = (data.get('username') as string) ?? '';
		if (!username) return fail(400, { error: 'Missing username.' });
		try {
			await unfollowUser(api, username);
		} catch (cause) {
			if (isHttpError(cause) && cause.status === 401) throw redirect(303, '/login');
			return fail(503, { error: 'Could not update follow status.' });
		}
		return { success: true };
	}
};
