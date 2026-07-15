import { isRedirect } from '@sveltejs/kit';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, getByUsername, getLikedPosts } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	getByUsername: vi.fn(),
	getLikedPosts: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/users', () => ({
	getByUsername,
	followUser: vi.fn(),
	unfollowUser: vi.fn()
}));
vi.mock('$lib/server/api/posts', () => ({ getLikedPosts }));

import { load } from './+page.server';
import { load as guardLoad } from '../../+layout.server';

function loadEvent(username = '@alice'): Parameters<typeof load>[0] {
	return {
		cookies: {},
		fetch: vi.fn(),
		params: { username }
	} as unknown as Parameters<typeof load>[0];
}

beforeEach(() => {
	vi.clearAllMocks();
});

describe('profile likes page load', () => {
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

	it('loads liked posts for authenticated visitors', async () => {
		getByUsername.mockResolvedValue({ username: 'alice' });
		getLikedPosts.mockResolvedValue({ items: [{ publicId: 'post-1' }], nextCursor: null });

		await expect(load(loadEvent())).resolves.toEqual({
			profileUser: { username: 'alice' },
			mode: 'likes',
			posts: [{ publicId: 'post-1' }],
			nextCursor: null
		});

		expect(getLikedPosts).toHaveBeenCalledWith(expect.any(Function), 'alice');
	});
});
