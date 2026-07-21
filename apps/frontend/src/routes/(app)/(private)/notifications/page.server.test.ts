import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, getNotifications, markNotificationRead } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	getNotifications: vi.fn(),
	markNotificationRead: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/notifications', () => ({ getNotifications, markNotificationRead }));

import { load } from './+page.server';

function loadEvent(): Parameters<typeof load>[0] {
	return {
		cookies: {},
		fetch: vi.fn()
	} as unknown as Parameters<typeof load>[0];
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
