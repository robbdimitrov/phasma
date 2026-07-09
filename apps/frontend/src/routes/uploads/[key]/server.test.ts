import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

vi.mock('$env/dynamic/private', () => ({ env: { BACKEND_URL: 'http://backend:8080' } }));
vi.mock('@sveltejs/kit', () => ({
	error: (status: number, message: string) => {
		const err = new Error(message) as Error & { status: number };
		err.status = status;
		throw err;
	}
}));

import { GET } from './+server';

function makeEvent(key: string) {
	return {
		params: { key },
		cookies: { get: vi.fn().mockReturnValue(undefined) }
	} as unknown as Parameters<typeof GET>[0];
}

describe('GET /uploads/[key]', () => {
	const validKey = 'a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4';
	let fetchMock: ReturnType<typeof vi.fn>;

	beforeEach(() => {
		fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('streams image bytes and forwards safe headers', async () => {
		fetchMock.mockResolvedValue(
			new Response('IMAGE-BYTES', {
				status: 200,
				headers: { 'content-type': 'image/jpeg', 'content-length': '11', etag: '"abc"' }
			})
		);

		const res = await GET(makeEvent(validKey));

		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(String(fetchMock.mock.calls[0]![0])).toBe(`http://backend:8080/uploads/${validKey}`);
		expect(res.status).toBe(200);
		expect(res.headers.get('content-type')).toBe('image/jpeg');
		expect(res.headers.get('etag')).toBe('"abc"');
		expect(await res.text()).toBe('IMAGE-BYTES');
	});

	it('maps a missing image to 404 without reaching the body', async () => {
		fetchMock.mockResolvedValue(new Response('not found', { status: 404 }));
		await expect(GET(makeEvent(validKey))).rejects.toMatchObject({ status: 404 });
	});

	it('maps other backend failures to 502', async () => {
		fetchMock.mockResolvedValue(new Response('boom', { status: 500 }));
		await expect(GET(makeEvent(validKey))).rejects.toMatchObject({ status: 502 });
	});

	it('rejects traversal and malformed keys before calling the backend', async () => {
		const badKeys = [
			'..',
			'photo.jpg',
			'with/slash',
			'bad%2F',
			'',
			'ABCDEF1234567890abcdef1234567890'
		];
		for (const key of badKeys) {
			await expect(GET(makeEvent(key))).rejects.toMatchObject({ status: 404 });
		}
		expect(fetchMock).not.toHaveBeenCalled();
	});
});
