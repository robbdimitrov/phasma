import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { getNotifications, markNotificationRead } from '$lib/server/api/notifications';
import { apiClient } from '$lib/server/api/client';

export const GET: RequestHandler = async ({ fetch, cookies, url }) => {
	if (!cookies.get('session')) return new Response(null, { status: 401 });
	const cursor = url.searchParams.get('cursor') ?? undefined;
	const client = apiClient({ fetch, cookies });
	const page = await getNotifications(client, cursor);

	return json(page);
};

// Bounds a crafted request from fanning a single POST out into an unbounded call count.
const MAX_MARK_READ_IDS = 100;

// Fired from mount/load-more (see +page.svelte), never a GET. A +page.server.ts
// action would be unreachable here since this file owns every HTTP method for the route.
export const POST: RequestHandler = async ({ request, fetch, cookies }) => {
	if (!cookies.get('session')) return new Response(null, { status: 401 });
	const body: { ids?: unknown } = await request.json().catch(() => ({}));
	const rawIds = Array.isArray(body.ids) ? body.ids : [];
	const ids = rawIds
		.filter((id): id is string => typeof id === 'string')
		.slice(0, MAX_MARK_READ_IDS);
	const client = apiClient({ fetch, cookies });
	void Promise.allSettled(ids.map((id) => markNotificationRead(client, id)));
	return json({ success: true });
};
