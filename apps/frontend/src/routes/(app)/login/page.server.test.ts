import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, login, applySessionCookie } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	login: vi.fn(),
	applySessionCookie: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/auth', () => ({ login, applySessionCookie }));

import { actions } from './+page.server';

function loginAction() {
	const action = actions.default;
	if (!action) throw new Error('login action is not defined');
	return action;
}

function actionEvent(formData: FormData) {
	const getClientAddress = vi.fn().mockReturnValue('203.0.113.7');
	return {
		cookies: { set: vi.fn() },
		fetch: vi.fn(),
		getClientAddress,
		request: new Request('http://localhost/login', { method: 'POST', body: formData })
	} as unknown as Parameters<ReturnType<typeof loginAction>>[0];
}

function formData(email?: string, password?: string) {
	const data = new FormData();
	if (email !== undefined) data.set('email', email);
	if (password !== undefined) data.set('password', password);
	return data;
}

beforeEach(() => {
	vi.clearAllMocks();
});

describe('login action', () => {
	it('rejects a submission missing email or password before calling the backend', async () => {
		const result = await loginAction()(actionEvent(formData('', 'secret')));

		expect(result).toMatchObject({
			status: 400,
			data: { error: 'Email and password are required.' }
		});
		expect(login).not.toHaveBeenCalled();
	});

	it('forwards the real client address to apiClient', async () => {
		login.mockResolvedValue(new Response(null, { status: 401 }));

		await loginAction()(actionEvent(formData('user@example.com', 'secret')));

		expect(apiClient).toHaveBeenCalledWith(
			expect.objectContaining({ getClientAddress: expect.any(Function) })
		);
	});

	it('reports invalid credentials distinctly from rate limiting', async () => {
		login.mockResolvedValue(new Response(null, { status: 401 }));

		const result = await loginAction()(actionEvent(formData('user@example.com', 'wrong')));

		expect(result).toMatchObject({ status: 401, data: { error: 'Invalid email or password.' } });
	});

	it('reports rate limiting without implying the credentials were wrong', async () => {
		login.mockResolvedValue(new Response(null, { status: 429 }));

		const result = await loginAction()(actionEvent(formData('user@example.com', 'secret')));

		expect(result).toMatchObject({
			status: 429,
			data: { error: 'Too many requests. Please try again later.' }
		});
	});

	it('reports a backend outage without implying the credentials were wrong', async () => {
		login.mockResolvedValue(new Response(null, { status: 503 }));

		const result = await loginAction()(actionEvent(formData('user@example.com', 'secret')));

		expect(result).toMatchObject({
			status: 503,
			data: { error: 'The service is temporarily unavailable. Please try again later.' }
		});
	});

	it('fails safely when the session cookie cannot be established', async () => {
		login.mockResolvedValue(new Response(null, { status: 200 }));
		applySessionCookie.mockReturnValue(false);

		const result = await loginAction()(actionEvent(formData('user@example.com', 'secret')));

		expect(result).toMatchObject({
			status: 502,
			data: { error: 'Could not establish a session. Please try again.' }
		});
	});

	it('redirects to the feed on success', async () => {
		login.mockResolvedValue(new Response(null, { status: 200 }));
		applySessionCookie.mockReturnValue(true);

		await expect(
			loginAction()(actionEvent(formData('user@example.com', 'secret')))
		).rejects.toMatchObject({
			status: 303,
			location: '/feed'
		});
	});
});
