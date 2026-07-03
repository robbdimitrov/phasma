import type { PageServerLoad, Actions } from './$types';
import { search, type SearchType } from '$lib/server/api/search';
import { getSuggestedUsers, followUser, unfollowUser } from '$lib/server/api/users';
import { getPopularPosts } from '$lib/server/api/posts';
import { apiClient } from '$lib/server/api/client';

const MAX_Q_LENGTH = 50;

function resolveType(q: string, param: string | null): SearchType {
	if (param === 'users' || param === 'posts' || param === 'hashtags') return param;
	if (q.startsWith('@')) return 'users';
	if (q.startsWith('#')) return 'hashtags';
	return 'posts';
}

export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams.get('q') ?? '';
	const typeParam = event.url.searchParams.get('type');
	const type = resolveType(q, typeParam);

	if (!q || q.length > MAX_Q_LENGTH) {
		const api = apiClient(event);
		const [suggested, popular] = await Promise.all([getSuggestedUsers(api), getPopularPosts(api)]);
		return {
			q,
			type: 'posts' as SearchType,
			items: [],
			nextCursor: null,
			suggested: suggested.items,
			popular: popular.items
		};
	}

	const page = await search(apiClient(event), { q, type });
	return { q, type, items: page.items, nextCursor: page.nextCursor, suggested: [], popular: [] };
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
