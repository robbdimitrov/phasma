import { error, fail } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import { getByUsername, followUser, unfollowUser } from '$lib/server/api/users';
import { getLikedPosts } from '$lib/server/api/posts';
import { stripAt } from '$lib/server/username';
import { apiClient } from '$lib/server/api/client';

export const load: PageServerLoad = async ({ fetch, cookies, params }) => {
	const api = apiClient({ fetch, cookies });
	const username = stripAt(params.username);
	const [profileUser, postsPage] = await Promise.all([
		getByUsername(api, username),
		getLikedPosts(api, username)
	]);
	if (!profileUser) throw error(404, 'User not found');
	return {
		profileUser,
		posts: postsPage.items,
		nextCursor: postsPage.nextCursor
	};
};

export const actions: Actions = {
	follow: async ({ fetch, cookies, params }) => {
		const api = apiClient({ fetch, cookies });
		const username = stripAt(params.username);
		try {
			await followUser(api, username);
		} catch {
			return fail(503, { error: 'Could not update follow status.' });
		}
		return { success: true };
	},
	unfollow: async ({ fetch, cookies, params }) => {
		const api = apiClient({ fetch, cookies });
		const username = stripAt(params.username);
		try {
			await unfollowUser(api, username);
		} catch {
			return fail(503, { error: 'Could not update follow status.' });
		}
		return { success: true };
	}
};
