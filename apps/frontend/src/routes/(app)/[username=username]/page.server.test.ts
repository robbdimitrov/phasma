import { error } from '@sveltejs/kit';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, getByUsername, getUserPosts, followUser, unfollowUser } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	getByUsername: vi.fn(),
	getUserPosts: vi.fn(),
	followUser: vi.fn(),
	unfollowUser: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/users', () => ({
	getByUsername,
	followUser,
	unfollowUser
}));
vi.mock('$lib/server/api/posts', () => ({ getUserPosts }));

import { actions, load } from './+page.server';

function loadEvent(username = '@alice'): Parameters<typeof load>[0] {
	return {
		cookies: {},
		fetch: vi.fn(),
		params: { username }
	} as unknown as Parameters<typeof load>[0];
}

function followAction() {
	const action = actions.follow;
	if (!action) throw new Error('follow action is not defined');
	return action;
}

function actionEvent(
	session: string | null = 'session-token'
): Parameters<ReturnType<typeof followAction>>[0] {
	return {
		cookies: { get: vi.fn(() => session ?? undefined) },
		fetch: vi.fn(),
		params: { username: '@alice' }
	} as unknown as Parameters<ReturnType<typeof followAction>>[0];
}

beforeEach(() => {
	vi.clearAllMocks();
});

function backendError(status: number): unknown {
	try {
		error(status, 'backend details');
	} catch (cause) {
		return cause;
	}
}

describe('profile page load', () => {
	it('loads public profile posts only', async () => {
		getByUsername.mockResolvedValue({ username: 'alice' });
		getUserPosts.mockResolvedValue({ items: [{ publicId: 'post-1' }], nextCursor: 'next' });

		await expect(load(loadEvent())).resolves.toEqual({
			profileUser: { username: 'alice' },
			mode: 'posts',
			posts: [{ publicId: 'post-1' }],
			nextCursor: 'next'
		});

		expect(getByUsername).toHaveBeenCalledWith(expect.any(Function), 'alice');
		expect(getUserPosts).toHaveBeenCalledWith(expect.any(Function), 'alice');
	});
});

describe('profile follow action', () => {
	it('redirects anonymous submissions to /login', async () => {
		await expect(followAction()(actionEvent(null))).rejects.toMatchObject({
			status: 303,
			location: '/login'
		});
		expect(followUser).not.toHaveBeenCalled();
	});

	it('follows for authenticated submissions', async () => {
		followUser.mockResolvedValue(null);

		await expect(followAction()(actionEvent())).resolves.toEqual({ success: true });

		expect(followUser).toHaveBeenCalledWith(expect.any(Function), 'alice');
	});

	it('redirects expired sessions to /login', async () => {
		followUser.mockRejectedValue(backendError(401));

		await expect(followAction()(actionEvent())).rejects.toMatchObject({
			status: 303,
			location: '/login'
		});
	});
});
