import { fail, redirect } from '@sveltejs/kit';
import type { Actions } from './$types';
import { applySessionCookie, login } from '$lib/server/api/auth';
import { apiClient } from '$lib/server/api/client';

export const actions: Actions = {
	default: async ({ request, fetch, cookies }) => {
		const data = await request.formData();
		const email = ((data.get('email') as string) ?? '').trim();
		const password = (data.get('password') as string) ?? '';

		if (!email || !password) {
			return fail(400, { error: 'Email and password are required.' });
		}

		const res = await login(apiClient({ fetch, cookies }), email, password);

		if (!res.ok) {
			return fail(res.status, { error: 'Invalid email or password.' });
		}

		// The backend sets the session cookie on its own origin, so SvelteKit must re-emit it here.
		if (!applySessionCookie(res.headers, cookies)) {
			return fail(502, { error: 'Could not establish a session. Please try again.' });
		}

		throw redirect(303, '/feed');
	}
};
