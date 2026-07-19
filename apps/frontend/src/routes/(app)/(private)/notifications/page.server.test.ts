import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, getNotifications, markNotificationRead } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	getNotifications: vi.fn(),
	markNotificationRead: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/notifications', () => ({ getNotifications, markNotificationRead }));

import { actions, load } from './+page.server';

function loadEvent(): Parameters<typeof load>[0] {
	return {
		cookies: {},
		fetch: vi.fn()
	} as unknown as Parameters<typeof load>[0];
}

function markReadAction() {
	const action = actions.markRead;
	if (!action) throw new Error('markRead action is not defined');
	return action;
}

function actionEvent(): Parameters<ReturnType<typeof markReadAction>>[0] {
	return {
		cookies: {},
		fetch: vi.fn(),
		request: new Request('http://localhost/notifications?/markRead', { method: 'POST' })
	} as unknown as Parameters<ReturnType<typeof markReadAction>>[0];
}

beforeEach(() => {
	vi.clearAllMocks();
});

describe('notifications page load', () => {
	it('returns notifications without marking any as read', async () => {
		const page = {
			items: [
				{ id: 'n1', read: false },
				{ id: 'n2', read: true }
			],
			nextCursor: 'cursor-1'
		};
		getNotifications.mockResolvedValue(page);

		await expect(load(loadEvent())).resolves.toEqual({
			notifications: page.items,
			nextCursor: page.nextCursor
		});

		expect(markNotificationRead).not.toHaveBeenCalled();
	});
});

describe('markRead action', () => {
	it('marks every currently-unread notification as read', async () => {
		const page = {
			items: [
				{ id: 'n1', read: false },
				{ id: 'n2', read: true },
				{ id: 'n3', read: false }
			],
			nextCursor: null
		};
		getNotifications.mockResolvedValue(page);
		markNotificationRead.mockResolvedValue(null);

		await markReadAction()(actionEvent());
		await new Promise((resolve) => setImmediate(resolve));

		expect(markNotificationRead).toHaveBeenCalledTimes(2);
		expect(markNotificationRead).toHaveBeenCalledWith(expect.any(Function), 'n1');
		expect(markNotificationRead).toHaveBeenCalledWith(expect.any(Function), 'n3');
	});
});
