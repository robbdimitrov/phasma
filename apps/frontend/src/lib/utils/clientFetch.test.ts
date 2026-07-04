import { describe, it, expect } from 'vitest';
import { fetchCursorPage, fetchJson } from '$lib/utils/clientFetch';

describe('fetchJson', () => {
	it('parses and returns JSON on ok response', async () => {
		const data = { items: [1, 2], nextCursor: 'abc' };
		const res = new Response(JSON.stringify(data), { status: 200 });
		const result = await fetchJson<typeof data>(res);
		expect(result).toEqual(data);
	});

	it('throws a sign-in message on 401', async () => {
		const res = new Response('Unauthorized', { status: 401 });
		await expect(fetchJson(res)).rejects.toThrow('Please sign in to continue.');
	});

	it('throws a rate-limit message on 429', async () => {
		const res = new Response('Too Many Requests', { status: 429 });
		await expect(fetchJson(res)).rejects.toThrow('Too many requests. Please try again later.');
	});

	it('throws a generic message on other non-ok responses', async () => {
		const res = new Response('Not found', { status: 404 });
		await expect(fetchJson(res)).rejects.toThrow('Could not load more items. Please try again.');
	});

	it('throws a generic message on server error', async () => {
		const res = new Response('Internal Server Error', { status: 500 });
		await expect(fetchJson(res)).rejects.toThrow('Could not load more items. Please try again.');
	});

	it('throws on empty body', async () => {
		const res = new Response('', { status: 200 });
		await expect(fetchJson(res)).rejects.toThrow('Empty response body');
	});
});

describe('fetchCursorPage', () => {
	it('appends an encoded cursor to paths without query params', async () => {
		const fetcher = async (input: RequestInfo | URL) => {
			expect(input).toBe('/feed?cursor=abc%3D%3D');
			return new Response(JSON.stringify({ items: [1], nextCursor: null }), { status: 200 });
		};

		await expect(fetchCursorPage<number>(fetcher, '/feed', 'abc==')).resolves.toEqual({
			items: [1],
			nextCursor: null
		});
	});

	it('appends an encoded cursor to paths with existing query params', async () => {
		const fetcher = async (input: RequestInfo | URL) => {
			expect(input).toBe('/search?q=cats&type=posts&cursor=next%2Fpage');
			return new Response(JSON.stringify({ items: [], nextCursor: 'later' }), { status: 200 });
		};

		await expect(
			fetchCursorPage<string>(fetcher, '/search?q=cats&type=posts', 'next/page')
		).resolves.toEqual({
			items: [],
			nextCursor: 'later'
		});
	});
});
