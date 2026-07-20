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

// Bounds a crafted request from fanning a single POST out into an unbounded
// number of backend PUT calls.
const MAX_MARK_READ_IDS = 100;

export const actions: Actions = {
	// Fired from mount/load-more (see +page.svelte), never a GET; takes explicit ids.
	markRead: async ({ request, fetch, cookies }) => {
		const client = apiClient({ fetch, cookies });
		const formData = await request.formData();
		const ids = formData
			.getAll('id')
			.slice(0, MAX_MARK_READ_IDS)
			.map((id) => id.toString());
		void Promise.allSettled(ids.map((id) => markNotificationRead(client, id)));
		return { success: true };
	}
};
