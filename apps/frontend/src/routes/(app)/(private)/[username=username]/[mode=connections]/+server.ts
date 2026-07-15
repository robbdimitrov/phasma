import type { RequestHandler } from './$types';
import { getFollowers, getFollowing } from '$lib/server/api/users';
import { stripAt } from '$lib/server/username';
import { cursorEndpoint } from '$lib/server/cursorEndpoint';

export const GET: RequestHandler = async (event) =>
	cursorEndpoint(event, (client, cursor) => {
		const username = stripAt(event.params.username);
		return event.params.mode === 'followers'
			? getFollowers(client, username, cursor)
			: getFollowing(client, username, cursor);
	});
