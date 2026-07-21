import type { PageServerLoad } from './$types';
import { getNotifications } from '$lib/server/api/notifications';
import { apiClient } from '$lib/server/api/client';

export const load: PageServerLoad = async ({ fetch, cookies }) => {
	const client = apiClient({ fetch, cookies });
	const page = await getNotifications(client);

	return {
		notifications: page.items,
		nextCursor: page.nextCursor
	};
};
