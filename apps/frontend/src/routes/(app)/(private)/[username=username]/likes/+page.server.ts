import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { getByUsername } from '$lib/server/api/users';
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
		mode: 'likes' as const,
		posts: postsPage.items,
		nextCursor: postsPage.nextCursor
	};
};
