import { redirect } from '@sveltejs/kit';
import type { Actions } from './$types';
import { logout } from '$lib/server/api/auth';
import { apiClient } from '$lib/server/api/client';

export const actions: Actions = {
	default: async ({ fetch, cookies }) => {
		await logout(apiClient({ fetch, cookies })).catch(() => {});
		cookies.delete('session', { path: '/' });
		throw redirect(303, '/login');
	}
};
