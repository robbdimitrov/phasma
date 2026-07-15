import { isRedirect } from '@sveltejs/kit';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, getByUsername, getFollowers, getFollowing } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	getByUsername: vi.fn(),
	getFollowers: vi.fn(),
	getFollowing: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/users', () => ({ getByUsername, getFollowers, getFollowing }));

import { load } from './+page.server';
import { load as guardLoad } from '../../+layout.server';

function loadEvent(mode: 'followers' | 'following'): Parameters<typeof load>[0] {
	return {
		cookies: {},
		fetch: vi.fn(),
		params: { username: '@alice', mode }
	} as unknown as Parameters<typeof load>[0];
}

beforeEach(() => {
	vi.clearAllMocks();
});

describe('profile connections page load', () => {
	it('redirects anonymous visitors through the private layout guard', async () => {
		const parent = vi.fn().mockResolvedValue({ currentUser: null });

		try {
			await guardLoad({ parent } as unknown as Parameters<typeof guardLoad>[0]);
			expect.unreachable('expected a redirect to be thrown');
		} catch (err) {
			if (!isRedirect(err)) throw err;
			expect(err.status).toBe(303);
			expect(err.location).toBe('/login');
		}
	});

	it('loads followers for authenticated visitors', async () => {
		getByUsername.mockResolvedValue({ username: 'alice' });
		getFollowers.mockResolvedValue({ items: [{ username: 'bob' }], nextCursor: 'next' });

		await expect(load(loadEvent('followers'))).resolves.toEqual({
			profileUser: { username: 'alice' },
			mode: 'followers',
			users: [{ username: 'bob' }],
			nextCursor: 'next'
		});

		expect(getFollowers).toHaveBeenCalledWith(expect.any(Function), 'alice');
		expect(getFollowing).not.toHaveBeenCalled();
	});
});
