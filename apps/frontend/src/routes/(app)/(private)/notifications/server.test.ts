import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, getNotifications, markNotificationRead } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	getNotifications: vi.fn(),
	markNotificationRead: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/notifications', () => ({ getNotifications, markNotificationRead }));

import { GET, POST } from './+server';

type PostEvent = Parameters<typeof POST>[0];

function postEvent(ids: unknown, session: string | null = 'token'): PostEvent {
	return {
		request: new Request('http://localhost/notifications', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ ids })
		}),
		cookies: { get: (name: string) => (name === 'session' ? session : null) },
		fetch: vi.fn()
	} as unknown as PostEvent;
}

beforeEach(() => {
	vi.clearAllMocks();
});

describe('GET /notifications', () => {
	it('requires a session cookie', async () => {
		const event = {
			cookies: { get: () => null },
			url: new URL('http://localhost/notifications'),
			fetch: vi.fn()
		} as unknown as Parameters<typeof GET>[0];

		const res = await GET(event);
		expect(res.status).toBe(401);
		expect(getNotifications).not.toHaveBeenCalled();
	});
});

describe('POST /notifications (mark read)', () => {
	it('marks exactly the given ids as read, without re-fetching notifications', async () => {
		markNotificationRead.mockResolvedValue(null);

		const res = await POST(postEvent(['n1', 'n3']));
		await new Promise((resolve) => setImmediate(resolve));

		expect(res.status).toBe(200);
		expect(getNotifications).not.toHaveBeenCalled();
		expect(markNotificationRead).toHaveBeenCalledTimes(2);
		expect(markNotificationRead).toHaveBeenCalledWith(expect.any(Function), 'n1');
		expect(markNotificationRead).toHaveBeenCalledWith(expect.any(Function), 'n3');
	});

	it('caps a request carrying an excessive number of ids', async () => {
		markNotificationRead.mockResolvedValue(null);
		const ids = Array.from({ length: 250 }, (_, i) => `n${i}`);

		await POST(postEvent(ids));
		await new Promise((resolve) => setImmediate(resolve));

		expect(markNotificationRead).toHaveBeenCalledTimes(100);
	});

	it('ignores non-string entries in the ids array', async () => {
		markNotificationRead.mockResolvedValue(null);

		await POST(postEvent(['n1', 42, null, 'n2']));
		await new Promise((resolve) => setImmediate(resolve));

		expect(markNotificationRead).toHaveBeenCalledTimes(2);
	});

	it('requires a session cookie', async () => {
		const res = await POST(postEvent(['n1'], null));
		expect(res.status).toBe(401);
		expect(markNotificationRead).not.toHaveBeenCalled();
	});
});
