import { fail } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import {
	search,
	type SearchUserItem,
	type SearchPostItem,
	type SearchAllItem,
	type SearchPage,
	type SearchType
} from '$lib/server/api/search';
import { getSuggestedUsers, followUser, unfollowUser } from '$lib/server/api/users';
import { getPopularPosts } from '$lib/server/api/posts';
import { apiClient } from '$lib/server/api/client';
import { SEARCH_PREVIEW_LIMIT, searchQueryPrefix, stripSearchQueryPrefix } from '$lib/utils/searchQuery';

const MAX_Q_LENGTH = 50;

const EMPTY_RESULTS: SearchPage<SearchAllItem> = { items: [], nextCursor: null };

// Tags each item of a single-type page with its entity type, so a @/#
// prefix-narrowed search still produces the same blended-item shape the full
// page renders — it just happens to contain one type.
function wrapItems<T extends SearchUserItem | SearchPostItem>(
	type: 'users' | 'posts',
	page: SearchPage<T>
): SearchPage<SearchAllItem> {
	return {
		items: page.items.map((item) => ({ type, item }) as SearchAllItem),
		nextCursor: page.nextCursor
	};
}

// A failing search (e.g. Meilisearch briefly unavailable, or an invalid
// #hashtag filter) must not take down the whole page.
function searchResults(promise: Promise<SearchPage<SearchAllItem>>): Promise<SearchPage<SearchAllItem>> {
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
			resultsType: 'all' as SearchType,
			results: EMPTY_RESULTS,
			suggested: suggested.items,
			popular: popular.items
		};
	}

	const api = apiClient(event);
	const prefix = searchQueryPrefix(q);
	// An explicit @/# prefix means the user picked a specific entity, so
	// narrow to just that type instead of blending. @ needs stripping since
	// it's only meaningful client-side; # stays in the query so the backend's
	// posts search can apply its exact-hashtag filter. resultsQuery/resultsType
	// are echoed back so the client's "Load more" continuation queries the
	// same type with the same (possibly stripped) query text.
	let results: Promise<SearchPage<SearchAllItem>>;
	let resultsQuery = q;
	let resultsType: SearchType = 'all';
	if (prefix === '@') {
		resultsQuery = stripSearchQueryPrefix(q, prefix);
		resultsType = 'users';
		results = searchResults(
			search<SearchUserItem>(api, { q: resultsQuery, type: 'users', limit: SEARCH_PREVIEW_LIMIT }).then(
				(page) => wrapItems('users', page)
			)
		);
	} else if (prefix === '#') {
		resultsType = 'posts';
		results = searchResults(
			search<SearchPostItem>(api, { q, type: 'posts', limit: SEARCH_PREVIEW_LIMIT }).then((page) =>
				wrapItems('posts', page)
			)
		);
	} else {
		results = searchResults(search<SearchAllItem>(api, { q, type: 'all', limit: SEARCH_PREVIEW_LIMIT }));
	}

	return { q, resultsQuery, resultsType, results: await results, suggested: [], popular: [] };
};

export const actions: Actions = {
	follow: async (event) => {
		const api = apiClient(event);
		const data = await event.request.formData();
		const username = (data.get('username') as string) ?? '';
		if (!username) return fail(400, { error: 'Missing username.' });
		try {
			await followUser(api, username);
		} catch {
			return fail(503, { error: 'Could not update follow status.' });
		}
		return { success: true };
	},
	unfollow: async (event) => {
		const api = apiClient(event);
		const data = await event.request.formData();
		const username = (data.get('username') as string) ?? '';
		if (!username) return fail(400, { error: 'Missing username.' });
		try {
			await unfollowUser(api, username);
		} catch {
			return fail(503, { error: 'Could not update follow status.' });
		}
		return { success: true };
	}
};
