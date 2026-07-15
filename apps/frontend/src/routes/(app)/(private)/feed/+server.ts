import type { RequestHandler } from './$types';
import { getFeed } from '$lib/server/api/posts';
import { cursorEndpoint } from '$lib/server/cursorEndpoint';

export const GET: RequestHandler = async (event) =>
	cursorEndpoint(event, (client, cursor) => getFeed(client, cursor));
