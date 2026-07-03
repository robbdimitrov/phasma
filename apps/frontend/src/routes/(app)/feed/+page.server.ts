import type { PageServerLoad } from './$types';
import { getFeed, getPopularPosts } from '$lib/server/api/posts';
import { apiClient } from '$lib/server/api/client';

export const load: PageServerLoad = async ({ fetch, cookies, parent }) => {
	const { currentUser } = await parent();
	const client = apiClient({ fetch, cookies });
	if (!currentUser) {
		const { items } = await getPopularPosts(client);
		return { posts: items, nextCursor: null };
	}
	const page = await getFeed(client);
	return { posts: page.items, nextCursor: page.nextCursor };
};
