import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, searchUsers, searchHashtags } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	searchUsers: vi.fn(),
	searchHashtags: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/search', () => ({ searchUsers, searchHashtags }));

import { GET } from './+server';

function event(session?: string): Parameters<typeof GET>[0] {
	return {
		cookies: { get: vi.fn(() => session) },
		fetch: vi.fn(),
		url: new URL('http://localhost/suggest?q=alice&type=users')
	} as unknown as Parameters<typeof GET>[0];
}

beforeEach(() => {
	vi.clearAllMocks();
});

describe('suggest endpoint', () => {
	it('rejects anonymous requests', async () => {
		const response = await GET(event());

		expect(response.status).toBe(401);
		expect(searchUsers).not.toHaveBeenCalled();
		expect(searchHashtags).not.toHaveBeenCalled();
	});
});
