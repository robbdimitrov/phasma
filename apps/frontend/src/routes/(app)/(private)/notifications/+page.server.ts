import type { PageServerLoad } from './$types';
import { getNotifications, markNotificationRead } from '$lib/server/api/notifications';
import { apiClient } from '$lib/server/api/client';

export const load: PageServerLoad = async ({ fetch, cookies }) => {
	const client = apiClient({ fetch, cookies });
	const page = await getNotifications(client);

	// Mark all unread notifications as read on page visit without blocking
	// navigation on it; failures are best-effort.
	const unreadIds = new Set(page.items.filter((n) => !n.read).map((n) => n.id));
	void Promise.allSettled([...unreadIds].map((id) => markNotificationRead(client, id)));

	return {
		notifications: page.items.map((n) => (unreadIds.has(n.id) ? { ...n, read: true } : n)),
		nextCursor: page.nextCursor
	};
};
