import { fail, redirect } from '@sveltejs/kit';
import type { Actions } from './$types';
import { applySessionCookie, login } from '$lib/server/api/auth';
import { apiClient } from '$lib/server/api/client';
import { statusMessage } from '$lib/server/api/http';

export const actions: Actions = {
	default: async ({ request, fetch, cookies, getClientAddress }) => {
		const data = await request.formData();
		const email = ((data.get('email') as string) ?? '').trim();
		const password = (data.get('password') as string) ?? '';

		if (!email || !password) {
			return fail(400, { error: 'Email and password are required.' });
		}

		const res = await login(apiClient({ fetch, cookies, getClientAddress }), email, password);

		if (!res.ok) {
			const error = res.status === 401 ? 'Invalid email or password.' : statusMessage(res.status);
			return fail(res.status, { error });
		}

		// The backend sets the session cookie on its own origin, so SvelteKit must re-emit it here.
		if (!applySessionCookie(res.headers, cookies)) {
			return fail(502, { error: 'Could not establish a session. Please try again.' });
		}

		throw redirect(303, '/feed');
	}
};
