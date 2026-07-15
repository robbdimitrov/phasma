import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, search } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	search: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/search', () => ({ search }));

import { GET } from './+server';

function event(session?: string): Parameters<typeof GET>[0] {
	return {
		cookies: { get: vi.fn(() => session) },
		fetch: vi.fn(),
		url: new URL('http://localhost/search?q=photo&type=posts')
	} as unknown as Parameters<typeof GET>[0];
}

beforeEach(() => {
	vi.clearAllMocks();
});

describe('search pagination endpoint', () => {
	it('rejects anonymous requests', async () => {
		const response = await GET(event());

		expect(response.status).toBe(401);
		expect(search).not.toHaveBeenCalled();
	});
});
