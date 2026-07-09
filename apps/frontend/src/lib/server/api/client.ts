import type { RequestEvent } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';

/** A backend-bound fetch: a backend-relative path plus the standard `RequestInit`. */
export type ApiClient = (path: string, init?: RequestInit) => Promise<Response>;

const backendBase = (): string => env.BACKEND_URL ?? 'http://localhost:8080';

/** Per-request BFF client; forwards the session cookie and, optionally, the real client IP. */
export function apiClient(
	event: Pick<RequestEvent, 'fetch' | 'cookies'> & { getClientAddress?: RequestEvent['getClientAddress'] }
): ApiClient {
	return (path, init) => {
		const headers = new Headers(init?.headers);
		const session = event.cookies.get('session');
		if (session) headers.set('cookie', `session=${session}`);
		if (event.getClientAddress) {
			try {
				headers.set('x-forwarded-for', event.getClientAddress());
			} catch (err) {
				// Throws without a proxy setting ADDRESS_HEADER in front (e.g. local dev).
				console.error('[warn] getClientAddress() failed:', err instanceof Error ? err.message : err);
			}
		}
		// event.fetch always adds an Origin header, tripping the backend's CSRF guard.
		return fetch(new URL(path, backendBase()), { ...init, headers });
	};
}
