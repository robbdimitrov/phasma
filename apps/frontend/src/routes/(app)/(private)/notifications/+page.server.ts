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
	// Fired once the page has genuinely mounted or a new page is loaded (see
	// +page.svelte), never from a GET, so no speculative or passive fetch can
	// trigger it. Takes explicit ids since the client already knows which
	// notifications on the page(s) it holds are unread.
	markRead: async ({ request, fetch, cookies }) => {
		const client = apiClient({ fetch, cookies });
		const formData = await request.formData();
		const ids = formData.getAll('id').map((id) => id.toString());
		void Promise.allSettled(ids.map((id) => markNotificationRead(client, id)));
		return { success: true };
	}
};
