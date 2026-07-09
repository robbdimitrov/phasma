import { error } from '@sveltejs/kit';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, createUser, login, applySessionCookie } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	createUser: vi.fn(),
	login: vi.fn(),
	applySessionCookie: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/auth', () => ({ createUser, login, applySessionCookie }));

import { actions } from './+page.server';

function registerAction() {
	const action = actions.default;
	if (!action) throw new Error('register action is not defined');
	return action;
}

function actionEvent(formData: FormData) {
	return {
		cookies: { set: vi.fn() },
		fetch: vi.fn(),
		getClientAddress: vi.fn().mockReturnValue('203.0.113.7'),
		request: new Request('http://localhost/register', { method: 'POST', body: formData })
	} as unknown as Parameters<ReturnType<typeof registerAction>>[0];
}

function formData() {
	const data = new FormData();
	data.set('name', 'Test User');
	data.set('username', 'testuser');
	data.set('email', 'user@example.com');
	data.set('password', 'secretpw');
	return data;
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

describe('register action', () => {
	it('reports rate limiting from account creation without a generic error page', async () => {
		createUser.mockRejectedValue(backendError(429));

		const result = await registerAction()(actionEvent(formData()));

		expect(result).toMatchObject({
			status: 429,
			data: { error: 'Too many requests. Please try again later.' }
		});
		expect(login).not.toHaveBeenCalled();
	});

	it('tells the user when the account was created but auto sign-in was rate limited', async () => {
		createUser.mockResolvedValue({ username: 'testuser' });
		login.mockResolvedValue(new Response(null, { status: 429 }));

		const result = await registerAction()(actionEvent(formData()));

		expect(result).toMatchObject({
			status: 429,
			data: { error: 'Account created, but sign-in failed. Please log in.' }
		});
	});

	it('redirects to the feed on success', async () => {
		createUser.mockResolvedValue({ username: 'testuser' });
		login.mockResolvedValue(new Response(null, { status: 200 }));
		applySessionCookie.mockReturnValue(true);

		await expect(registerAction()(actionEvent(formData()))).rejects.toMatchObject({
			status: 303,
			location: '/feed'
		});
	});
});
