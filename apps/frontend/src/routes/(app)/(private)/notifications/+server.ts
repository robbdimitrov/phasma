import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { getNotifications } from '$lib/server/api/notifications';
import { apiClient } from '$lib/server/api/client';

export const GET: RequestHandler = async ({ fetch, cookies, url }) => {
	if (!cookies.get('session')) return new Response(null, { status: 401 });
	const cursor = url.searchParams.get('cursor') ?? undefined;
	const client = apiClient({ fetch, cookies });
	const page = await getNotifications(client, cursor);

	return json(page);
};
