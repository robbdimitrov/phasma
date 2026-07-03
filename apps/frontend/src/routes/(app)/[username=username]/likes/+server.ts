import type { RequestHandler } from './$types';
import { getLikedPosts } from '$lib/server/api/posts';
import { stripAt } from '$lib/server/username';
import { cursorEndpoint } from '$lib/server/cursorEndpoint';

export const GET: RequestHandler = async (event) =>
	cursorEndpoint(
		event,
		(client, cursor) => getLikedPosts(client, stripAt(event.params.username), cursor),
		{ auth: 'public' }
	);
