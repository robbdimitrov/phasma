import type { RequestEvent } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';

/** A backend-bound fetch: a backend-relative path plus the standard `RequestInit`. */
export type ApiClient = (path: string, init?: RequestInit) => Promise<Response>;

const backendBase = (): string => env.BACKEND_URL ?? 'http://localhost:8080';

/**
 * Per-request BFF client that forwards the session cookie server-side; the
 * browser remains limited to this origin by CSP.
 *
 * `getClientAddress` is optional and only needed by callers hitting endpoints
 * the backend rate-limits by IP before a session exists (login, register):
 * without it, the backend's TRUST_PROXY-aware rate limiter sees every request
 * arriving from this BFF's own address, sharing one bucket across all users.
 */
export function apiClient(
	event: Pick<RequestEvent, 'fetch' | 'cookies'> & { getClientAddress?: RequestEvent['getClientAddress'] }
): ApiClient {
	return (path, init) => {
		const headers = new Headers(init?.headers);
		// Read per call to honour a session set earlier in the same request.
		const session = event.cookies.get('session');
		if (session) headers.set('cookie', `session=${session}`);
		// getClientAddress() throws when ADDRESS_HEADER is configured but the
		// configured header is absent from this request (e.g. a direct
		// kubectl port-forward with no reverse proxy in front locally) —
		// fall back to not forwarding rather than failing the request.
		if (event.getClientAddress) {
			try {
				headers.set('x-forwarded-for', event.getClientAddress());
			} catch {
				// no-op: backend falls back to seeing this BFF's own address
			}
		}
		// Deliberately the global fetch, not event.fetch: event.fetch always adds
		// an Origin header, which trips the backend's cross-site POST guard on
		// this trusted, server-to-server call.
		return fetch(new URL(path, backendBase()), { ...init, headers });
	};
}
