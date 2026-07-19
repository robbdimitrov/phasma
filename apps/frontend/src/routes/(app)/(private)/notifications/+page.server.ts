import type { Actions, PageServerLoad } from './$types';
import { getNotifications, markNotificationRead } from '$lib/server/api/notifications';
import { apiClient } from '$lib/server/api/client';

export const load: PageServerLoad = async ({ fetch, cookies }) => {
	const client = apiClient({ fetch, cookies });
	const page = await getNotifications(client);

	return {
		notifications: page.items,
		nextCursor: page.nextCursor
	};
};

export const actions: Actions = {
	// Fired once the page has genuinely mounted (see +page.svelte), not from load,
	// so speculative preloads and other GET-triggered fetches can't mark read.
	markRead: async ({ fetch, cookies }) => {
		const client = apiClient({ fetch, cookies });
		const page = await getNotifications(client);
		const unreadIds = page.items.filter((n) => !n.read).map((n) => n.id);
		void Promise.allSettled(unreadIds.map((id) => markNotificationRead(client, id)));
		return { success: true };
	}
};
