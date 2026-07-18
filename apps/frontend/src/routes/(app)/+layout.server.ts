import type { LayoutServerLoad } from './$types';
import { getCurrent } from '$lib/server/api/users';
import { getUnreadCount } from '$lib/server/api/notifications';
import { apiClient } from '$lib/server/api/client';

export const load: LayoutServerLoad = async ({ fetch, cookies, depends }) => {
	depends('app:unreadCount');
	if (!cookies.get('session')) return { currentUser: null, unreadCount: 0 };

	const client = apiClient({ fetch, cookies });
	const fullUser = await getCurrent(client);
	const currentUser = fullUser ? (({ email: _, ...rest }) => rest)(fullUser) : null;
	const unreadCount = currentUser ? await getUnreadCount(client).catch(() => 0) : 0;
	return { currentUser, unreadCount };
};
