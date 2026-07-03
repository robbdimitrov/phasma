import type { RequestHandler } from './$types';
import { getComments } from '$lib/server/api/posts';
import { cursorEndpoint } from '$lib/server/cursorEndpoint';

export const GET: RequestHandler = async (event) =>
	cursorEndpoint(
		event,
		(client, cursor) => getComments(client, event.params.publicId, cursor),
		{ auth: 'public' }
	);
