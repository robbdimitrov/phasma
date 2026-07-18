import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, getCurrent, getUnreadCount } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	getCurrent: vi.fn(),
	getUnreadCount: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/users', () => ({ getCurrent }));
vi.mock('$lib/server/api/notifications', () => ({ getUnreadCount }));

import { load } from './+layout.server';

function loadEvent(session?: string): Parameters<typeof load>[0] {
	return {
		cookies: { get: vi.fn(() => session) },
		depends: vi.fn(),
		fetch: vi.fn()
	} as unknown as Parameters<typeof load>[0];
}

beforeEach(() => {
	vi.clearAllMocks();
	apiClient.mockReturnValue(vi.fn());
	getCurrent.mockResolvedValue(null);
	getUnreadCount.mockResolvedValue(0);
});

describe('(app) layout load', () => {
	it('does not call the backend when there is no session cookie', async () => {
		await expect(load(loadEvent())).resolves.toEqual({ currentUser: null, unreadCount: 0 });

		expect(apiClient).not.toHaveBeenCalled();
		expect(getCurrent).not.toHaveBeenCalled();
		expect(getUnreadCount).not.toHaveBeenCalled();
	});

	it('loads the current user and unread count when a session cookie exists', async () => {
		getCurrent.mockResolvedValue({
			id: '1',
			username: 'alice',
			name: 'Alice',
			email: 'alice@example.com'
		});
		getUnreadCount.mockResolvedValue(3);

		await expect(load(loadEvent('session-token'))).resolves.toEqual({
			currentUser: { id: '1', username: 'alice', name: 'Alice' },
			unreadCount: 3
		});

		expect(apiClient).toHaveBeenCalledWith(expect.objectContaining({ cookies: expect.anything() }));
		expect(getUnreadCount).toHaveBeenCalledWith(expect.any(Function));
	});

	it('degrades unread count failures to zero', async () => {
		getCurrent.mockResolvedValue({ id: '1', username: 'alice', email: 'alice@example.com' });
		getUnreadCount.mockRejectedValue(new Error('backend down'));

		await expect(load(loadEvent('session-token'))).resolves.toEqual({
			currentUser: { id: '1', username: 'alice' },
			unreadCount: 0
		});
	});

	it('does not load unread count when a session cookie is invalid', async () => {
		getCurrent.mockResolvedValue(null);

		await expect(load(loadEvent('expired-session-token'))).resolves.toEqual({
			currentUser: null,
			unreadCount: 0
		});

		expect(getUnreadCount).not.toHaveBeenCalled();
	});
});
