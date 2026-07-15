import { fail, isHttpError, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { apiClient } from '$lib/server/api/client';
import { deleteSession, getSessions } from '$lib/server/api/sessions';

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export const load: PageServerLoad = async ({ fetch, cookies }) => {
	try {
		return {
			sessions: await getSessions(apiClient({ fetch, cookies })),
			loadError: ''
		};
	} catch (cause) {
		if (isHttpError(cause) && cause.status === 401) {
			throw redirect(303, '/login');
		}
		return {
			sessions: [],
			loadError: 'Could not load active sessions. Please try again.'
		};
	}
};

export const actions: Actions = {
	revoke: async ({ fetch, cookies, request }) => {
		if (!cookies.get('session')) throw redirect(303, '/login');
		const data = await request.formData();
		const sessionID = String(data.get('sessionId') ?? '');
		if (!uuidPattern.test(sessionID)) {
			return fail(400, { error: 'Invalid session.', sessionID });
		}

		try {
			await deleteSession(apiClient({ fetch, cookies }), sessionID);
			return { revokedSessionID: sessionID };
		} catch (cause) {
			const status = isHttpError(cause) ? cause.status : 500;
			if (status === 401) {
				throw redirect(303, '/login');
			}
			const error =
				status === 404
					? 'That session is no longer active.'
					: status === 409
						? 'Your current session cannot be revoked here. Use Logout instead.'
						: 'Could not revoke the session. Please try again.';
			return fail(status === 404 || status === 409 ? status : 500, { error, sessionID });
		}
	}
};
