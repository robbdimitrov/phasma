import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiClient, recordRecentSearch } = vi.hoisted(() => ({
	apiClient: vi.fn(() => vi.fn()),
	recordRecentSearch: vi.fn()
}));

vi.mock('$lib/server/api/client', () => ({ apiClient }));
vi.mock('$lib/server/api/search', () => ({ recordRecentSearch }));

import { POST } from './+server';

function event(
	body: unknown,
	session: string | null = 'session-token'
): Parameters<typeof POST>[0] {
	return {
		cookies: { get: vi.fn(() => session ?? undefined) },
		fetch: vi.fn(),
		request: new Request('http://localhost/search/recent', {
			method: 'POST',
			body: JSON.stringify(body)
		})
	} as unknown as Parameters<typeof POST>[0];
}

beforeEach(() => {
	vi.clearAllMocks();
});

describe('recent search recording endpoint', () => {
	it('rejects anonymous requests', async () => {
		const response = await POST(event({ type: 'posts', reference: 'cats' }, null));

		expect(response.status).toBe(401);
		expect(recordRecentSearch).not.toHaveBeenCalled();
	});

	it('rejects an invalid type', async () => {
		await expect(POST(event({ type: 'bogus', reference: 'cats' }))).rejects.toMatchObject({
			status: 400
		});
		expect(recordRecentSearch).not.toHaveBeenCalled();
	});

	it('rejects a whitespace-only reference', async () => {
		await expect(POST(event({ type: 'posts', reference: '   ' }))).rejects.toMatchObject({
			status: 400
		});
		expect(recordRecentSearch).not.toHaveBeenCalled();
	});

	it('trims surrounding whitespace before forwarding to the backend', async () => {
		recordRecentSearch.mockResolvedValue(null);

		const response = await POST(event({ type: 'posts', reference: '  cats  ' }));

		expect(response.status).toBe(204);
		expect(recordRecentSearch).toHaveBeenCalledWith(expect.anything(), 'posts', 'cats');
	});
});
