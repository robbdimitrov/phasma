import type { RequestHandler } from './$types';
import { getUserPosts, getLikedPosts } from '$lib/server/api/posts';
import { stripAt } from '$lib/server/username';
import { cursorEndpoint } from '$lib/server/cursorEndpoint';

export const GET: RequestHandler = async (event) =>
	cursorEndpoint(
		event,
		(client, cursor) => {
			const username = stripAt(event.params.username);
			return event.params.mode === 'likes'
				? getLikedPosts(client, username, cursor)
				: getUserPosts(client, username, cursor);
		},
		{ auth: 'public' }
	);
