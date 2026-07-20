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

function actionEvent(ids: string[]): Parameters<ReturnType<typeof markReadAction>>[0] {
	const formData = new FormData();
	for (const id of ids) formData.append('id', id);
	return {
		cookies: {},
		fetch: vi.fn(),
		request: new Request('http://localhost/notifications?/markRead', {
			method: 'POST',
			body: formData
		})
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
	it('marks exactly the ids it was given as read, without re-fetching notifications', async () => {
		markNotificationRead.mockResolvedValue(null);

		await markReadAction()(actionEvent(['n1', 'n3']));
		await new Promise((resolve) => setImmediate(resolve));

		expect(getNotifications).not.toHaveBeenCalled();
		expect(markNotificationRead).toHaveBeenCalledTimes(2);
		expect(markNotificationRead).toHaveBeenCalledWith(expect.any(Function), 'n1');
		expect(markNotificationRead).toHaveBeenCalledWith(expect.any(Function), 'n3');
	});
});
