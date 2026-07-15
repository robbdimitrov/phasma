import { error } from '@sveltejs/kit';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, deleteSession, getSessions } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	deleteSession: vi.fn(),
	getSessions: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/sessions', () => ({ deleteSession, getSessions }));

import { actions, load } from './+page.server';

const sessionID = '01904d2e-7f4d-7c33-ae21-2f94737eaa10';

function loadEvent(): Parameters<typeof load>[0] {
	return {
		cookies: {},
		fetch: vi.fn()
	} as unknown as Parameters<typeof load>[0];
}

function actionEvent(
	formData: FormData,
	session: string | null = 'session-token'
): Parameters<ReturnType<typeof revokeAction>>[0] {
	return {
		cookies: { get: vi.fn(() => session ?? undefined) },
		fetch: vi.fn(),
		request: new Request('http://localhost/settings/sessions', {
			method: 'POST',
			body: formData
		})
	} as unknown as Parameters<ReturnType<typeof revokeAction>>[0];
}

function revokeAction() {
	const action = actions.revoke;
	if (!action) throw new Error('revoke action is not defined');
	return action;
}

function backendError(status: number): unknown {
	try {
		error(status, 'backend details');
	} catch (cause) {
		return cause;
	}
}

beforeEach(() => {
	vi.clearAllMocks();
});

describe('active sessions load', () => {
	it('returns sessions from the backend API', async () => {
		const sessions = [
			{
				id: sessionID,
				created: new Date('2026-06-22T12:00:00Z'),
				expiresAt: new Date('2026-06-29T12:00:00Z'),
				current: true
			}
		];
		getSessions.mockResolvedValue(sessions);

		await expect(load(loadEvent())).resolves.toEqual({ sessions, loadError: '' });
	});

	it('returns a safe load error when the backend fails', async () => {
		getSessions.mockRejectedValue(new Error('database details'));

		await expect(load(loadEvent())).resolves.toEqual({
			sessions: [],
			loadError: 'Could not load active sessions. Please try again.'
		});
	});

	it('redirects to login when authentication expires during loading', async () => {
		getSessions.mockRejectedValue(backendError(401));

		await expect(load(loadEvent())).rejects.toMatchObject({
			status: 303,
			location: '/login'
		});
	});
});

describe('revoke session action', () => {
	it('redirects anonymous submissions before validating the form', async () => {
		const data = new FormData();
		data.set('sessionId', 'not-a-uuid');

		await expect(revokeAction()(actionEvent(data, null))).rejects.toMatchObject({
			status: 303,
			location: '/login'
		});
		expect(deleteSession).not.toHaveBeenCalled();
	});

	it('rejects malformed session IDs before calling the backend', async () => {
		const data = new FormData();
		data.set('sessionId', 'not-a-uuid');

		const result = await revokeAction()(actionEvent(data));

		expect(result).toMatchObject({ status: 400, data: { error: 'Invalid session.' } });
		expect(deleteSession).not.toHaveBeenCalled();
	});

	it('revokes a valid remote session', async () => {
		const data = new FormData();
		data.set('sessionId', sessionID);
		deleteSession.mockResolvedValue(null);

		await expect(revokeAction()(actionEvent(data))).resolves.toEqual({
			revokedSessionID: sessionID
		});
		expect(deleteSession).toHaveBeenCalledWith(expect.any(Function), sessionID);
	});

	it.each([
		[404, 'That session is no longer active.'],
		[409, 'Your current session cannot be revoked here. Use Logout instead.']
	])('maps backend status %i to a safe action failure', async (status, message) => {
		const data = new FormData();
		data.set('sessionId', sessionID);
		deleteSession.mockRejectedValue(backendError(status));

		const result = await revokeAction()(actionEvent(data));

		expect(result).toMatchObject({ status, data: { error: message, sessionID } });
	});

	it('maps unexpected failures to a generic server error', async () => {
		const data = new FormData();
		data.set('sessionId', sessionID);
		deleteSession.mockRejectedValue(new Error('network details'));

		const result = await revokeAction()(actionEvent(data));

		expect(result).toMatchObject({
			status: 500,
			data: { error: 'Could not revoke the session. Please try again.', sessionID }
		});
	});

	it('redirects to login when authentication expires during revocation', async () => {
		const data = new FormData();
		data.set('sessionId', sessionID);
		deleteSession.mockRejectedValue(backendError(401));

		await expect(revokeAction()(actionEvent(data))).rejects.toMatchObject({
			status: 303,
			location: '/login'
		});
	});
});
