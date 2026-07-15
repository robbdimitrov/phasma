import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { getByUsername, getFollowers, getFollowing } from '$lib/server/api/users';
import { stripAt } from '$lib/server/username';
import { apiClient } from '$lib/server/api/client';

export const load: PageServerLoad = async ({ fetch, cookies, params }) => {
	const api = apiClient({ fetch, cookies });
	const username = stripAt(params.username);
	const mode = params.mode as 'followers' | 'following';

	const profileUser = await getByUsername(api, username);
	if (!profileUser) throw error(404, 'User not found');

	const page =
		mode === 'followers' ? await getFollowers(api, username) : await getFollowing(api, username);

	return {
		profileUser,
		mode,
		users: page.items,
		nextCursor: page.nextCursor
	};
};
