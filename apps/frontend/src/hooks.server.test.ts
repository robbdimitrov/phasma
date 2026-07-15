import { describe, expect, it, vi } from 'vitest';

import { handle } from './hooks.server';

async function resolveURL(rawURL: string) {
	const response = new Response(null);
	const resolve = vi.fn().mockResolvedValue(response);
	const event = {
		url: new URL(rawURL),
		request: new Request(rawURL),
		cookies: { get: vi.fn().mockReturnValue(null) }
	};

	return handle({
		event,
		resolve
	} as unknown as Parameters<typeof handle>[0]);
}

describe('handle', () => {
	it('sets security headers', async () => {
		const response = await resolveURL('http://phasma.localhost/feed');

		expect(response.headers.get('X-Content-Type-Options')).toBe('nosniff');
		expect(response.headers.get('X-Frame-Options')).toBe('DENY');
		expect(response.headers.get('Referrer-Policy')).toBe('strict-origin-when-cross-origin');
		expect(response.headers.get('Cross-Origin-Opener-Policy')).toBe('same-origin');
		expect(response.headers.get('Permissions-Policy')).toBe(
			'camera=(), microphone=(), geolocation=(), payment=(), usb=(), bluetooth=()'
		);
	});

	it('omits Strict-Transport-Security: no TLS termination in this deployment', async () => {
		const response = await resolveURL('https://phasma.localhost/feed');

		expect(response.headers.has('Strict-Transport-Security')).toBe(false);
	});
});
