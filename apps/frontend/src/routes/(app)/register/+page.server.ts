import { fail, isHttpError, redirect } from '@sveltejs/kit';
import type { Actions } from './$types';
import { applySessionCookie, createUser, login } from '$lib/server/api/auth';
import { apiClient } from '$lib/server/api/client';
import { statusMessage } from '$lib/server/api/http';

export const actions: Actions = {
	default: async ({ request, fetch, cookies, getClientAddress }) => {
		const api = apiClient({ fetch, cookies, getClientAddress });
		const data = await request.formData();
		const name = ((data.get('name') as string) ?? '').trim();
		const username = ((data.get('username') as string) ?? '').trim().toLowerCase();
		const email = ((data.get('email') as string) ?? '').trim();
		const password = (data.get('password') as string) ?? '';

		if (!name || !username || !email || !password) {
			return fail(400, { error: 'All fields are required.' });
		}
		if (name.length > 100) {
			return fail(400, { error: 'Name must be 100 characters or fewer.' });
		}

		let created;
		try {
			created = await createUser(api, name, username, email, password);
		} catch (err) {
			if (isHttpError(err)) return fail(err.status, { error: statusMessage(err.status) });
			throw err;
		}
		if (!created) {
			return fail(400, { error: 'Could not create account. Please try again.' });
		}

		const res = await login(api, email, password);
		if (!res.ok || !applySessionCookie(res.headers, cookies)) {
			return fail(res.ok ? 502 : res.status, {
				error: 'Account created, but sign-in failed. Please log in.'
			});
		}

		throw redirect(303, '/feed');
	}
};
