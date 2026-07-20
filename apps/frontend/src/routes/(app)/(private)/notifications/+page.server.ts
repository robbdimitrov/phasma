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
	// Fired from mount/load-more (see +page.svelte), never a GET; takes explicit ids.
	markRead: async ({ request, fetch, cookies }) => {
		const client = apiClient({ fetch, cookies });
		const formData = await request.formData();
		const ids = formData.getAll('id').map((id) => id.toString());
		void Promise.allSettled(ids.map((id) => markNotificationRead(client, id)));
		return { success: true };
	}
};
