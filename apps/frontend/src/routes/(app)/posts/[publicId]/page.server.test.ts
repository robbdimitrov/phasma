import { error } from '@sveltejs/kit';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, getPost, getComments, likePost } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	getPost: vi.fn(),
	getComments: vi.fn(),
	likePost: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/posts', () => ({
	getPost,
	getComments,
	likePost,
	unlikePost: vi.fn(),
	createComment: vi.fn(),
	deleteComment: vi.fn(),
	deletePost: vi.fn()
}));

import { actions, load } from './+page.server';

function loadEvent(currentUser: unknown = null): Parameters<typeof load>[0] {
	return {
		cookies: {},
		fetch: vi.fn(),
		parent: vi.fn().mockResolvedValue({ currentUser }),
		params: { publicId: 'post-1' }
	} as unknown as Parameters<typeof load>[0];
}

function likeAction() {
	const action = actions.like;
	if (!action) throw new Error('like action is not defined');
	return action;
}

function actionEvent(
	session: string | null = 'session-token'
): Parameters<ReturnType<typeof likeAction>>[0] {
	return {
		cookies: { get: vi.fn(() => session ?? undefined) },
		fetch: vi.fn(),
		params: { publicId: 'post-1' }
	} as unknown as Parameters<ReturnType<typeof likeAction>>[0];
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

describe('post page load', () => {
	it('does not load comment lists for anonymous direct post visitors', async () => {
		getPost.mockResolvedValue({ publicId: 'post-1' });

		await expect(load(loadEvent())).resolves.toEqual({
			post: { publicId: 'post-1' },
			comments: [],
			nextCommentsCursor: null
		});

		expect(getPost).toHaveBeenCalledWith(expect.any(Function), 'post-1');
		expect(getComments).not.toHaveBeenCalled();
	});

	it('loads comments for authenticated direct post visitors', async () => {
		getPost.mockResolvedValue({ publicId: 'post-1' });
		getComments.mockResolvedValue({ items: [{ id: 'comment-1' }], nextCursor: 'next' });

		await expect(load(loadEvent({ id: '1', username: 'alice' }))).resolves.toEqual({
			post: { publicId: 'post-1' },
			comments: [{ id: 'comment-1' }],
			nextCommentsCursor: 'next'
		});

		expect(getComments).toHaveBeenCalledWith(expect.any(Function), 'post-1');
	});
});

describe('post page actions', () => {
	it('redirects anonymous mutations to /login', async () => {
		await expect(likeAction()(actionEvent(null))).rejects.toMatchObject({
			status: 303,
			location: '/login'
		});
		expect(likePost).not.toHaveBeenCalled();
	});

	it('redirects expired sessions to /login', async () => {
		likePost.mockRejectedValue(backendError(401));

		await expect(likeAction()(actionEvent())).rejects.toMatchObject({
			status: 303,
			location: '/login'
		});
	});
});
