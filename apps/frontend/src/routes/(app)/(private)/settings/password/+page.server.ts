import { fail, redirect } from '@sveltejs/kit';
import type { Actions } from './$types';
import { getCurrent, changePassword } from '$lib/server/api/users';
import { apiClient } from '$lib/server/api/client';

export const actions: Actions = {
	default: async ({ fetch, cookies, request }) => {
		const api = apiClient({ fetch, cookies });
		const currentUser = await getCurrent(api);
		if (!currentUser) throw redirect(303, '/login');

		const data = await request.formData();
		const oldPassword = (data.get('oldPassword') as string) ?? '';
		const password = (data.get('password') as string) ?? '';

		if (!oldPassword || !password) {
			return fail(400, { error: 'Both passwords are required.' });
		}

		if (password.length < 8) {
			return fail(400, { error: 'New password must be at least 8 characters.' });
		}

		try {
			await changePassword(api, oldPassword, password);
		} catch (e) {
			const status = (e as { status?: number }).status;
			return fail(400, {
				error:
					status === 400 || status === 401
						? 'Current password is incorrect.'
						: 'Something went wrong. Please try again.'
			});
		}

		throw redirect(303, '/settings');
	}
};
