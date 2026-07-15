import { beforeEach, describe, expect, it, vi } from 'vitest';
import { isRedirect } from '@sveltejs/kit';

const { apiClient, getFeed } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	getFeed: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/posts', () => ({ getFeed }));

import { load } from './+page.server';
import { load as guardLoad } from '../+layout.server';

function loadEvent(): Parameters<typeof load>[0] {
	return {
		cookies: {},
		fetch: vi.fn()
	} as unknown as Parameters<typeof load>[0];
}

beforeEach(() => {
	vi.clearAllMocks();
});

describe('feed page load', () => {
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

	it('loads the authenticated feed', async () => {
		const page = {
			items: [{ publicId: 'post-1' }],
			nextCursor: 'cursor-1'
		};
		getFeed.mockResolvedValue(page);

		await expect(load(loadEvent())).resolves.toEqual({
			posts: page.items,
			nextCursor: page.nextCursor
		});

		expect(getFeed).toHaveBeenCalledWith(expect.any(Function));
	});
});
