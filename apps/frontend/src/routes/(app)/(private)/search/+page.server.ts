import type { PageServerLoad, Actions } from './$types';
import {
	search,
	type SearchUserItem,
	type SearchPostItem,
	type SearchHashtagItem,
	type SearchItem,
	type SearchPage
} from '$lib/server/api/search';
import { getSuggestedUsers, followUser, unfollowUser } from '$lib/server/api/users';
import { getPopularPosts } from '$lib/server/api/posts';
import { apiClient } from '$lib/server/api/client';
import { SEARCH_PREVIEW_LIMIT, searchQueryPrefix, stripSearchQueryPrefix } from '$lib/utils/searchQuery';

const MAX_Q_LENGTH = 50;

const EMPTY_SECTION = { items: [], nextCursor: null };

// A category search failing (e.g. Meilisearch briefly unavailable, or an
// invalid #hashtag filter) must not take down the other two sections.
function searchSection<T extends SearchItem>(promise: Promise<SearchPage<T>>): Promise<SearchPage<T>> {
	return promise.catch(() => EMPTY_SECTION);
}

export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams.get('q') ?? '';

	if (!q || q.length > MAX_Q_LENGTH) {
		const api = apiClient(event);
		const [suggested, popular] = await Promise.all([getSuggestedUsers(api), getPopularPosts(api)]);
		return {
			q,
			users: EMPTY_SECTION,
			posts: EMPTY_SECTION,
			hashtags: EMPTY_SECTION,
			suggested: suggested.items,
			popular: popular.items
		};
	}

	const api = apiClient(event);
	const prefix = searchQueryPrefix(q);
	// An explicit @/# prefix means the user picked a specific entity, so show
	// only the matching section instead of three mostly-empty ones. Hashtags
	// is therefore only ever queried without a prefix, so it never needs
	// stripping; the users index does, since @ is only meaningful client-side.
	const wantUsers = prefix !== '#';
	const wantPosts = prefix !== '@';
	const wantHashtags = prefix === null;
	const usersQuery = prefix === '@' ? stripSearchQueryPrefix(q, prefix) : q;
	const [users, posts, hashtags] = await Promise.all([
		wantUsers
			? searchSection(
					search<SearchUserItem>(api, { q: usersQuery, type: 'users', limit: SEARCH_PREVIEW_LIMIT })
				)
			: EMPTY_SECTION,
		wantPosts
			? searchSection(search<SearchPostItem>(api, { q, type: 'posts', limit: SEARCH_PREVIEW_LIMIT }))
			: EMPTY_SECTION,
		wantHashtags
			? searchSection(search<SearchHashtagItem>(api, { q, type: 'hashtags', limit: SEARCH_PREVIEW_LIMIT }))
			: EMPTY_SECTION
	]);

	return { q, users, posts, hashtags, suggested: [], popular: [] };
};

export const actions: Actions = {
	follow: async (event) => {
		const api = apiClient(event);
		const data = await event.request.formData();
		const username = (data.get('username') as string) ?? '';
		if (!username) return { success: false };
		await followUser(api, username);
		return { success: true };
	},
	unfollow: async (event) => {
		const api = apiClient(event);
		const data = await event.request.formData();
		const username = (data.get('username') as string) ?? '';
		if (!username) return { success: false };
		await unfollowUser(api, username);
		return { success: true };
	}
};
