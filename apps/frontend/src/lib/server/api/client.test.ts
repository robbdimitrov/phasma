import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

vi.mock('$env/dynamic/private', () => ({ env: { BACKEND_URL: 'http://backend:8080' } }));

import { apiClient } from './client';

function makeEvent(session?: string, clientAddress?: string) {
	const cookies = { get: vi.fn().mockReturnValue(session) };
	const getClientAddress = clientAddress ? vi.fn().mockReturnValue(clientAddress) : undefined;
	return { cookies, getClientAddress } as unknown as Parameters<typeof apiClient>[0];
}

describe('apiClient', () => {
	let fetchMock: ReturnType<typeof vi.fn>;

	beforeEach(() => {
		fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
		vi.stubGlobal('fetch', fetchMock);
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('resolves backend-relative paths against BACKEND_URL', async () => {
		const event = makeEvent();
		await apiClient(event)('/users/me');
		const [url] = fetchMock.mock.calls[0]!;
		expect(String(url)).toBe('http://backend:8080/users/me');
	});

	it('preserves the query string of paginated requests', async () => {
		const event = makeEvent();
		await apiClient(event)('/posts?cursor=abc%3D%3D');
		const [url] = fetchMock.mock.calls[0]!;
		expect(String(url)).toBe('http://backend:8080/posts?cursor=abc%3D%3D');
	});

	it('forwards only the session cookie when present', async () => {
		const event = makeEvent('SECRET');
		await apiClient(event)('/posts');
		const [, init] = fetchMock.mock.calls[0]!;
		expect(new Headers(init.headers).get('cookie')).toBe('session=SECRET');
	});

	it('omits the cookie header for anonymous requests', async () => {
		const event = makeEvent(undefined);
		await apiClient(event)('/posts');
		const [, init] = fetchMock.mock.calls[0]!;
		expect(new Headers(init.headers).has('cookie')).toBe(false);
	});

	it('forwards the real client address when the caller supplies getClientAddress', async () => {
		const event = makeEvent(undefined, '203.0.113.7');
		await apiClient(event)('/sessions', { method: 'POST' });
		const [, init] = fetchMock.mock.calls[0]!;
		expect(new Headers(init.headers).get('x-forwarded-for')).toBe('203.0.113.7');
	});

	it('omits x-forwarded-for when the caller has no getClientAddress', async () => {
		const event = makeEvent();
		await apiClient(event)('/posts');
		const [, init] = fetchMock.mock.calls[0]!;
		expect(new Headers(init.headers).has('x-forwarded-for')).toBe(false);
	});

	it('logs and does not fail the request when getClientAddress throws', async () => {
		const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
		const event = {
			cookies: { get: vi.fn().mockReturnValue(undefined) },
			getClientAddress: vi.fn().mockImplementation(() => {
				throw new Error('ADDRESS_HEADER was specified but is absent from request');
			})
		} as unknown as Parameters<typeof apiClient>[0];

		const res = await apiClient(event)('/sessions', { method: 'POST' });

		expect(res.status).toBe(204);
		const [, init] = fetchMock.mock.calls[0]!;
		expect(new Headers(init.headers).has('x-forwarded-for')).toBe(false);
		expect(consoleError).toHaveBeenCalledTimes(1);
		consoleError.mockRestore();
	});

	it('preserves method, body, and caller headers', async () => {
		const event = makeEvent('SECRET');
		await apiClient(event)('/posts', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: '{}'
		});
		const [, init] = fetchMock.mock.calls[0]!;
		expect(init.method).toBe('POST');
		expect(init.body).toBe('{}');
		const headers = new Headers(init.headers);
		expect(headers.get('content-type')).toBe('application/json');
		expect(headers.get('cookie')).toBe('session=SECRET');
	});
});
