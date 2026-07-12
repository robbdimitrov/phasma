import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { getNotifications, markNotificationRead } from '$lib/server/api/notifications';
import { apiClient } from '$lib/server/api/client';

export const GET: RequestHandler = async ({ fetch, cookies, url }) => {
	if (!cookies.get('session')) return new Response(null, { status: 401 });
	const cursor = url.searchParams.get('cursor') ?? undefined;
	const client = apiClient({ fetch, cookies });
	const page = await getNotifications(client, cursor);

	// Mark newly loaded unread notifications as read without blocking the
	// response on it; failures are best-effort.
	const unreadIds = new Set(page.items.filter((n) => !n.read).map((n) => n.id));
	void Promise.allSettled([...unreadIds].map((id) => markNotificationRead(client, id)));

	return json({
		items: page.items.map((n) => (unreadIds.has(n.id) ? { ...n, read: true } : n)),
		nextCursor: page.nextCursor
	});
};
