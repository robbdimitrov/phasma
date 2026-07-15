import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, getComments } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	getComments: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/posts', () => ({ getComments }));

import { GET } from './+server';

function event(session?: string): Parameters<typeof GET>[0] {
	return {
		cookies: { get: vi.fn(() => session) },
		fetch: vi.fn(),
		params: { publicId: 'post-1' },
		url: new URL('http://localhost/posts/post-1/comments?cursor=next')
	} as unknown as Parameters<typeof GET>[0];
}

beforeEach(() => {
	vi.clearAllMocks();
});

describe('post comments pagination endpoint', () => {
	it('rejects anonymous requests', async () => {
		const response = await GET(event());

		expect(response.status).toBe(401);
		expect(getComments).not.toHaveBeenCalled();
	});

	it('loads comments for signed-in requests', async () => {
		getComments.mockResolvedValue({ items: [{ id: 'comment-1' }], nextCursor: null });

		const response = await GET(event('session-token'));

		expect(response.status).toBe(200);
		await expect(response.json()).resolves.toEqual({
			items: [{ id: 'comment-1' }],
			nextCursor: null
		});
		expect(getComments).toHaveBeenCalledWith(expect.any(Function), 'post-1', 'next');
	});
});
